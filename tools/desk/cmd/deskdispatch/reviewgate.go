package main

// reviewgate.go — the pre-dispatch gate for review dispatches (#2444).
//
// A reviewer is not sent to a head while, on a fresh read, one of four things is true of it:
//
//	required-check-red    a check the base branch requires has a completed, failed latest run
//	description-stale     the description's weight section declares a head that is not this one
//	decision-not-ruled    the change carries the decision label and no ruling is recorded since
//	lane-awaits-ruling    this lane's verdict at this head blocked only on unrecorded rulings
//
// WHAT A HOLD IS. A hold is a refusal (exit 5) returned before the claim. It delays a review
// and does nothing else: this file reads the forge through reviewRoundForge, which has no
// write method, so a hold cannot post a verdict, stand in for one, mark a head reviewed or
// feed a ready-flip. The held head is named in the refusal's first line with every reason,
// and that line is what the dispatch audit log records.
//
// A READ THAT FAILS DOES NOT HOLD. Each condition holds only on a positive, complete read.
// Anything short of that is returned as a note and the reviewer is dispatched.
//
// THE REVIEWER STILL CHECKS THE DESCRIPTION. description-stale is one mechanical test. The
// review kit's own description check is unchanged and still runs on every dispatched round.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionLabel is the label that marks an item as waiting on a human ruling. It is the one
// the review loop already holds a change on.
const decisionLabel = "needs-decision"

// heldMarker opens every hold's first line.
const heldMarker = "review dispatch HELD"

type gateReason string

const (
	holdRequiredCheckRed gateReason = "required-check-red"
	holdStaleDescription gateReason = "description-stale"
	holdDecisionNotRuled gateReason = "decision-not-ruled"
	holdLaneAwaitsRuling gateReason = "lane-awaits-ruling"
)

// gateHold is one reason a head is held, with a sentence saying what was read.
type gateHold struct {
	reason gateReason
	detail string
}

// reviewGateRatifierFn answers who records a ruling: the roster's blessing authority, the
// identity the trust gate already treats as the human who blesses. ok is false when no
// authority is configured, and then no decision condition can hold. A seam for tests.
var reviewGateRatifierFn = func() (is func(login string, id int64) bool, ok bool) {
	return deskkit.IsBlessAuthorityID, deskkit.BlessAuthorityLogin() != ""
}

// reviewGate evaluates the four hold conditions for one change at its current head.
// verdicts are the dispatched lane's own verdicts, oldest first (nil when not known). It
// returns every condition that holds and a note for every read that could not be completed.
func reviewGate(fg reviewRoundForge, fr deskkit.ForgeRepo, number int, pr *deskkit.PullRequest, verdicts []laneVerdict) (holds []gateHold, notes []string) {
	add := func(h *gateHold, note string) {
		if h != nil {
			holds = append(holds, *h)
		}
		if note != "" {
			notes = append(notes, note)
		}
	}
	add(requiredCheckHold(fg, fr, pr))
	add(staleDescriptionHold(pr), "")
	add(decisionHold(fg, fr, number, pr))
	add(laneRulingHold(fg, fr, pr, verdicts))
	return holds, notes
}

// holdRefusal is the refusal a held dispatch returns. Its first line names the change, the
// head and every reason; nothing after it is needed to act on it.
func holdRefusal(repo string, number int, head string, holds []gateHold) error {
	names := make([]string, 0, len(holds))
	var details []string
	for _, h := range holds {
		names = append(names, string(h.reason))
		details = append(details, "  - "+string(h.reason)+": "+h.detail)
	}
	if !isCommitID(head) {
		head = "unknown"
	}
	return deskkit.Refused(fmt.Sprintf("%s — %s#%d at head %s: %s. No reviewer was dispatched, no claim was "+
		"taken and nothing is recorded as reviewed; this is a delay, not a verdict.\n%s\n"+
		"Dispatch again once the reason clears: each condition is read fresh every time.",
		heldMarker, repo, number, head, strings.Join(names, ", "), strings.Join(details, "\n")))
}

// --- required-check-red ---

func requiredCheckHold(fg reviewRoundForge, fr deskkit.ForgeRepo, pr *deskkit.PullRequest) (*gateHold, string) {
	if strings.TrimSpace(pr.BaseRef) == "" || !isCommitID(strings.TrimSpace(pr.HeadSHA)) {
		return nil, "the change's base branch or head could not be read, so its required checks could not be read"
	}
	required, err := fg.RequiredStatusChecks(fr, pr.BaseRef)
	if err != nil {
		return nil, "could not read the base branch's required checks"
	}
	if len(required) == 0 {
		return nil, ""
	}
	checks, err := fg.ChecksAtHead(fr, strings.TrimSpace(pr.HeadSHA))
	if err != nil || checks == nil {
		return nil, "could not read the checks at the head"
	}
	if checks.CheckRunsTotalCount > len(checks.CheckRuns) || checks.StatusTotalCount > len(checks.Statuses) {
		return nil, "the forge served only part of the checks at the head"
	}
	red := redRequiredChecks(required, checks)
	if len(red) == 0 {
		return nil, ""
	}
	return &gateHold{holdRequiredCheckRed, fmt.Sprintf("%d required check(s) failed on their latest completed run: %s",
		len(red), strings.Join(red, ", "))}, ""
}

// redRequiredChecks names the required contexts that are RED at the head. A required context
// is red only when something reported under its name and every latest report under it is a
// failure: a check run that COMPLETED with failure, timed_out or startup_failure, or a commit
// status of failure or error. A context nothing has reported under, a run still queued or in
// progress, a pending status, and a cancelled, skipped, stale or action-required run are not
// red. Names are returned quoted, in the required list's order.
func redRequiredChecks(required []string, checks *deskkit.ChecksAtHead) []string {
	runs := deskkit.LatestRunPerName(checks.CheckRuns,
		func(r deskkit.CheckRun) string { return strings.ToLower(strings.TrimSpace(r.Name)) },
		func(r deskkit.CheckRun) string {
			if r.CompletedAt != "" {
				return r.CompletedAt
			}
			return r.StartedAt
		})
	statuses := deskkit.LatestRunPerName(checks.Statuses,
		func(s deskkit.StatusContext) string { return strings.ToLower(strings.TrimSpace(s.Context)) },
		func(s deskkit.StatusContext) string { return s.CreatedAt })
	var red []string
	seen := map[string]bool{}
	for _, name := range required {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		reported, failed := 0, 0
		for _, r := range runs {
			if strings.ToLower(strings.TrimSpace(r.Name)) != key {
				continue
			}
			reported++
			if strings.EqualFold(strings.TrimSpace(r.Status), "completed") && failedConclusion(r.Conclusion) {
				failed++
			}
		}
		for _, s := range statuses {
			if strings.ToLower(strings.TrimSpace(s.Context)) != key {
				continue
			}
			reported++
			switch strings.ToLower(strings.TrimSpace(s.State)) {
			case "failure", "error":
				failed++
			}
		}
		if reported > 0 && failed == reported {
			red = append(red, strconv.Quote(strings.TrimSpace(name)))
		}
	}
	return red
}

func failedConclusion(c string) bool {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "failure", "timed_out", "startup_failure":
		return true
	}
	return false
}

// --- description-stale ---

// weightHeadingRe finds the description's weight section; the next anyHeadingRe match ends it.
var (
	weightHeadingRe = regexp.MustCompile(`(?mi)^#{1,6}[ \t]+weight\b.*$`)
	anyHeadingRe    = regexp.MustCompile(`(?m)^#{1,6}[ \t]+\S`)
	// declaredHeadRe is a line that declares the measured head: after list, quote and table
	// markers, the word "head", then a commit id of 7 to 40 hex digits (in backticks or not).
	declaredHeadRe = regexp.MustCompile("(?mi)^[ \\t>|*+-]*head[ \\t]+`?([0-9a-f]{7,40})\\b")
)

// declaredHeads returns the heads a description's weight section declares, lower-cased, in
// order. A word that is all letters (such as "deadbeef") is not taken for a commit id.
func declaredHeads(body string) []string {
	loc := weightHeadingRe.FindStringIndex(body)
	if loc == nil {
		return nil
	}
	section := body[loc[1]:]
	if next := anyHeadingRe.FindStringIndex(section); next != nil {
		section = section[:next[0]]
	}
	var out []string
	for _, m := range declaredHeadRe.FindAllStringSubmatch(section, -1) {
		if strings.ContainsAny(m[1], "0123456789") {
			out = append(out, strings.ToLower(m[1]))
		}
	}
	return out
}

// staleDescriptionHold holds when the description declares at least one head and none of
// them is the head being dispatched. A description that declares no head is never held.
func staleDescriptionHold(pr *deskkit.PullRequest) *gateHold {
	head := strings.ToLower(strings.TrimSpace(pr.HeadSHA))
	declared := declaredHeads(pr.Body)
	if len(declared) == 0 || !isCommitID(head) {
		return nil
	}
	for _, d := range declared {
		if strings.HasPrefix(head, d) {
			return nil
		}
	}
	return &gateHold{holdStaleDescription, fmt.Sprintf("the description's weight section declares head %s, which is not "+
		"the head on the forge — the description was not updated for this head", strings.Join(declared, ", "))}
}

// --- decision-not-ruled ---

// rulingState is whether a ruling is recorded for an item carrying the decision label.
type rulingState int

const (
	rulingUnknown rulingState = iota
	rulingRecorded
	rulingMissing
)

// rulingRecordedSince decides whether the ratifying identity has written on an item since
// the decision label was last applied to it. events is the item's label history in order;
// trust is its comments and reviews. Recorded: a comment or review by the ratifying identity
// created after that application. Missing: the history is complete and holds none. Unknown:
// the application cannot be dated, or the history is incomplete and holds none.
func rulingRecordedSince(events []deskkit.LabelEvent, trust *deskkit.TrustPayload, isRatifier func(string, int64) bool) rulingState {
	var applied time.Time
	standing := false
	for _, e := range events {
		if !strings.EqualFold(strings.TrimSpace(e.Name), decisionLabel) {
			continue
		}
		standing = !e.Removed
		if standing {
			t, err := time.Parse(time.RFC3339, e.CreatedAt)
			if err != nil {
				return rulingUnknown
			}
			applied = t
		}
	}
	if !standing || applied.IsZero() || trust == nil {
		return rulingUnknown
	}
	for _, ev := range trust.Events {
		if isRatifier(ev.Author, ev.AuthorID) && ev.CreatedAt.After(applied) {
			return rulingRecorded
		}
	}
	if !trust.Complete {
		return rulingUnknown
	}
	return rulingMissing
}

func hasDecisionLabel(labels []string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), decisionLabel) {
			return true
		}
	}
	return false
}

// itemRuling reads one item's label history and comments and decides its ruling state. A
// read that fails is rulingUnknown.
func itemRuling(fg reviewRoundForge, fr deskkit.ForgeRepo, number int, isChange bool) rulingState {
	is, ok := reviewGateRatifierFn()
	if !ok || is == nil {
		return rulingUnknown
	}
	var (
		events []deskkit.LabelEvent
		trust  *deskkit.TrustPayload
		err    error
	)
	if isChange {
		events, err = fg.ListLabelEvents(fr, number)
	} else {
		events, err = fg.ListIssueLabelEvents(fr, number)
	}
	if err != nil {
		return rulingUnknown
	}
	if isChange {
		trust, err = fg.PRTrustEvents(fr, number)
	} else {
		trust, err = fg.IssueTrustEvents(fr, number)
	}
	if err != nil {
		return rulingUnknown
	}
	return rulingRecordedSince(events, trust, is)
}

func decisionHold(fg reviewRoundForge, fr deskkit.ForgeRepo, number int, pr *deskkit.PullRequest) (*gateHold, string) {
	if !hasDecisionLabel(pr.Labels) {
		return nil, ""
	}
	switch itemRuling(fg, fr, number, true) {
	case rulingMissing:
		return &gateHold{holdDecisionNotRuled, "the change carries `" + decisionLabel + "` and the ratifying identity has " +
			"written nothing on it since that label was applied"}, ""
	case rulingUnknown:
		return nil, "the change carries `" + decisionLabel + "` but whether a ruling is recorded could not be read"
	}
	return nil, ""
}

// --- lane-awaits-ruling ---

// itemRefRe is an item reference a blocked-on-ruling finding may name: "#N" or
// "<owner>/<repo>#N".
var itemRefRe = regexp.MustCompile(`^(?:([A-Za-z0-9._-]+/[A-Za-z0-9._-]+))?#([0-9]{1,9})$`)

// laneRulingHold holds a lane whose own latest verdict, AT THE HEAD BEING DISPATCHED, blocked
// only on rulings that are still not recorded. Read off the verdict's own typed declaration:
// the external-prerequisite marker line, no standing content blocker, and one or more
// prerequisite findings. It holds only when EVERY prerequisite is an open item of this
// repository carrying the decision label and at least one of them has no ruling recorded. A
// verdict at an earlier head never holds here: the head moved, so there is a delta to review.
func laneRulingHold(fg reviewRoundForge, fr deskkit.ForgeRepo, pr *deskkit.PullRequest, verdicts []laneVerdict) (*gateHold, string) {
	if len(verdicts) == 0 {
		return nil, ""
	}
	last := verdicts[len(verdicts)-1]
	head := strings.TrimSpace(pr.HeadSHA)
	if !last.blocking || !isCommitID(head) || !strings.EqualFold(last.head, head) {
		return nil, ""
	}
	decl := deskkit.ParsePrereqDeclaration(last.body, last.head)
	if !decl.Declared || decl.Err != nil || decl.HasContentBlocker || len(decl.Conditions) == 0 {
		return nil, ""
	}
	var waiting []string
	for _, c := range decl.Conditions {
		m := itemRefRe.FindStringSubmatch(strings.TrimSpace(c.Object))
		if m == nil || (m[1] != "" && !strings.EqualFold(m[1], fr.Slug())) {
			return nil, "" // not an item of this repository: not a ruling this gate can read
		}
		n, _ := strconv.Atoi(m[2])
		item, err := fg.GetIssue(fr, n)
		if err != nil || item == nil {
			return nil, "this lane's verdict at this head blocks on #" + m[2] + ", which could not be read"
		}
		if !strings.EqualFold(strings.TrimSpace(item.State), "open") || !hasDecisionLabel(item.Labels) {
			return nil, "" // not an open decision: the lane is not waiting on a ruling there
		}
		switch itemRuling(fg, fr, n, item.IsPullRequest) {
		case rulingUnknown:
			return nil, "this lane's verdict at this head blocks on #" + m[2] + ", whose ruling state could not be read"
		case rulingMissing:
			waiting = append(waiting, "#"+m[2])
		}
	}
	if len(waiting) == 0 {
		return nil, ""
	}
	sort.Strings(waiting)
	return &gateHold{holdLaneAwaitsRuling, fmt.Sprintf("this lane's review %d at this head blocked only on rulings, and "+
		"no ruling is recorded yet on %s", last.id, strings.Join(waiting, ", "))}, ""
}
