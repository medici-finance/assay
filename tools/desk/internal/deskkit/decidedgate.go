package deskkit

// decidedgate.go — the desk-decided READY-FLIP CONDITION (attention-budget/19, option A per
// the driver's ruling on #1677: refuse the flip ONLY on a finding), shared by BOTH verbs that
// perform the ready-flip: `deskflip` and `deskpost ready`.
//
// WHY IT LIVES HERE. The condition first landed inside deskflip alone (#1687), and
// `deskpost ready` — the other live path to the identical markPullRequestReadyForReview
// mutation — never ran it (#1694). A condition on one of two equivalent flip verbs is only as
// strong as the convention that picks the verb: a PR deskflip refused on desk-decided could
// simply be flipped with `deskpost ready`. Both verbs now call this ONE function, so the two
// cannot drift into disagreeing about the same PR (the same reason decided.go is one shared
// parser rather than two).
//
// It is PURE: the caller reads the PR's labels, body and reviews (and fails closed when it
// cannot), then hands them here. It returns the refusal REASON, or "" when the condition
// holds; each verb wraps the reason in its own refusal shape (deskflip: deskkit.Refused;
// deskpost: its audited `refused` writeResult). Both map to ExitRefused.

import (
	"fmt"
	"strings"
)

// DeskDecidedCondition is the condition's name, as both flip verbs report it in a refusal.
const DeskDecidedCondition = "desk-decided"

// DeskDecidedReview is the slice of one forge review the desk-decided condition reads. Each
// verb lifts its own review struct into this, so the reduction never depends on either
// verb's wire shape.
type DeskDecidedReview struct {
	Login    string // the review author, in either the gh-CLI or REST rendering (SameActor folds both)
	State    string // APPROVED | CHANGES_REQUESTED | COMMENTED | ...
	CommitID string
	Body     string
}

// DeskDecidedRefusal evaluates the desk-decided condition. Two things are MECHANICAL,
// evaluated in this order:
//
//  1. the `## Desk-decided` block, when present, must PARSE — a malformed block (the marker
//     missing, an empty list, an item missing a field) refuses, since a block nobody can
//     read is not a declaration;
//  2. the desk-decided LABEL and the block must AGREE — a label with no block, or a block
//     with no label, refuses, because either shape lets a human trust a signal (the label,
//     glanced at) that the other surface (the block, actually read) contradicts.
//
// The one thing that is NOT mechanical — whether a PR that declares nothing in fact contains
// an undeclared desk decision — is the REVIEWER's call: this reads the latest verdict from
// the bound reviewer role AT THE CURRENT HEAD and refuses while it carries the fixed line
// `Undeclared-desk-decision: <one line>`. Absence of a block, by itself, with no such finding
// and no label/block disagreement, is NEVER refused (Verify row 6) — that is option 2 of the
// human decision, not built without a ruling naming it.
//
// pr is the PR number (message text only); labels are the label NAMES on the PR; body is the
// PR description; reviews arrive in ascending submitted order; reviewerLogin is the bound
// reviewer role's login, compared with SameActor; head is the PR's current head sha.
func DeskDecidedRefusal(pr int, labels []string, body string, reviews []DeskDecidedReview, reviewerLogin, head string) string {
	labelled := hasLabelFold(labels, DeskDecidedLabel)
	_, blocked, perr := ParseDeskDecidedBlockInBody(body)
	if blocked && perr != nil {
		return fmt.Sprintf(
			"condition %s: PR #%d's `%s` section does not parse: %v — fix the block "+
				"(`deskpr edit --decided`) before the flip.",
			DeskDecidedCondition, pr, DeskDecidedHeading, perr)
	}
	if labelled != blocked {
		var detail string
		if labelled {
			detail = fmt.Sprintf("carries the %s label but its body has no `%s` section",
				DeskDecidedLabel, DeskDecidedHeading)
		} else {
			detail = fmt.Sprintf("body carries a `%s` section but not the %s label",
				DeskDecidedHeading, DeskDecidedLabel)
		}
		return fmt.Sprintf(
			"condition %s: PR #%d %s — the label and the block must agree. Re-run `deskpr edit --decided` "+
				"(it applies both together), or drop whichever one is stale.", DeskDecidedCondition, pr, detail)
	}

	// The reviewer's finding, at the CURRENT head only, reduced PER LANE (standingUndeclared-
	// DeskDecision): the correctness and security verdicts are posted by the SAME reviewer App,
	// in parallel, so a verdict in one lane must never clear the other lane's finding.
	if line, lane := standingUndeclaredDeskDecision(reviews, reviewerLogin, head); line != "" {
		return fmt.Sprintf(
			"condition %s: %s's %s review at head %s names an undeclared desk decision: %q — declare it "+
				"(`deskpr edit --decided`), then a fresh %s verdict at this head that omits the line clears it.",
			DeskDecidedCondition, reviewerLogin, lane, shortHead(head), line, lane)
	}
	return ""
}

// standingUndeclaredDeskDecision reduces the reviewer App's `Undeclared-desk-decision:` lines
// at the CURRENT head to the one that still STANDS, if any, and names the lane that raised it.
//
// PER LANE, because the correctness verdict and the security verdict are posted by the same
// App (review findings SEC-1 / F1 on attention-budget/19's PR): a "last reviewer-App review at
// head" reduction let a `Security-Review: pass` — a review that never looked at the question —
// become the governing review and silently clear a correctness finding, so the answer depended
// on which parallel lane happened to post last. A body carrying a security marker (pass or
// fail) is the security lane; every other body is the correctness lane.
//
// Within a lane, walked in ascending submitted order:
//   - ANY review carrying the line RAISES (or re-raises) the finding — a BLOCK-direction
//     marker is read in every state, fenced or not, so it cannot be hidden;
//   - only a later DECISIVE verdict in the SAME lane that omits the line CLEARS it: in the
//     correctness lane an APPROVED or CHANGES_REQUESTED review; in the security lane any
//     review (every security-lane body is by definition a pass/fail verdict). A COMMENTED
//     correctness-lane note that omits the line is not a fresh verdict and clears nothing.
//
// A finding in EITHER lane stands and refuses; the correctness lane is reported first.
func standingUndeclaredDeskDecision(reviews []DeskDecidedReview, reviewerLogin, head string) (line, lane string) {
	for _, security := range []bool{false, true} {
		standing := ""
		for _, r := range reviews {
			if !SameActor(r.Login, reviewerLogin) || r.CommitID != head {
				continue
			}
			if hasSecurityVerdictMarker(r.Body) != security {
				continue
			}
			if lines := UndeclaredDeskDecisionLines(r.Body); len(lines) > 0 {
				standing = lines[0]
				continue
			}
			if security || r.State == "APPROVED" || r.State == "CHANGES_REQUESTED" {
				standing = ""
			}
		}
		if standing != "" {
			if security {
				return standing, "security"
			}
			return standing, "correctness"
		}
	}
	return "", ""
}

// hasSecurityVerdictMarker reports whether a review body carries either security verdict
// marker — the lane test standingUndeclaredDeskDecision splits on.
func hasSecurityVerdictMarker(body string) bool {
	return HasSecurityReviewPass(body) || HasSecurityReviewFail(body)
}

// hasLabelFold reports whether want is among labels, case-insensitively (label names are
// case-insensitive on both forges).
func hasLabelFold(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, want) {
			return true
		}
	}
	return false
}

// shortHead renders a sha prefix for refusal text.
func shortHead(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
