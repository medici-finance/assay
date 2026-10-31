package deskkit

// securitybodycr.go is the SECURITY-LANE twin of bodyeditcr.go: the one decision for whether a
// `Security-Review: fail` that blocks only on the PR BODY has been cleared at the unchanged
// head. deskflip's reviewer-approved gate and deskboard's review reduction both call
// EvaluateSecurityBodyClearance, as they both call EvaluateBodyEditReverification for the
// correctness lane, so the gate and the board cannot disagree about it (#5092).
//
// THE CASE. A security reviewer fails a PR because its description claims a control that is
// not there, the author corrects the description, and the reviewer re-reads it and passes at
// the same head. The pass is posted as a COMMENTED review, which can never supersede the
// fail's CHANGES_REQUESTED, so before this decision the only ways out were a human dismissal
// or a no-op push.
//
// THE MARKER is the correctness lane's own, unchanged:
//
//	on the `Security-Review: fail`:
//	  Blocked-On-Body: <finding-id> <body-digest-at-the-fail>
//	on the later `Security-Review: pass` that answers it:
//	  Body-Reread-Digest: <body-digest-re-read>
//
// and the declaration is parsed by the SAME reader (ParseBodyEditDeclaration), so a malformed,
// ambiguous, mixed or co-declared marker refuses here for exactly the reasons it refuses there.
// An UNMARKED security fail never reaches a clearance: it keeps blocking as it always did.
//
// WHAT CLEARS IT. All of these, each in the fail-closed direction:
//
//   - a later `Security-Review: pass` from the same reviewer stands AT THE SAME HEAD after
//     the last fail. The newest such standing pass is the one read; a later pass in a state
//     that does not stand does not count and does not displace it (TestSecBodyLaterDismissedPass);
//   - every security fail between the marked fail and that pass is itself marked and parses
//     cleanly. A later UNMARKED fail blocks, whatever passes follow it at this head, because
//     its basis may be code;
//   - that pass was posted in a review STATE that carries a standing pass
//     (SecurityPassStateStands: APPROVED or COMMENTED, the same two states `deskpost ready`'s
//     own list holds). A DISMISSED, PENDING or CHANGES_REQUESTED review carrying the marker
//     does not stand, so it clears nothing;
//   - neither the fail, nor a later fail in its chain, nor any pass at head after the last
//     fail carries a correctness verdict line (HasCorrectnessVerdictLine, the reading
//     `deskpost ready`'s lane classifier uses). Such a body speaks for both lanes, and the
//     gate's correctness rule drops every security-marked body, so clearing it would drop
//     its correctness half unread;
//   - the pass documents `Body-Reread-Digest:` equal to the digest of the live body the
//     caller read at the gate;
//   - the forge's own record of the body's last edit is LATER than the last fail in that
//     chain. That, not the digests, establishes that the body changed after the block.
//
// HOW IT DIFFERS FROM THE CORRECTNESS LANE (EvaluateBodyEditReverification), on purpose:
//
//   - THE CLEARING REVIEW. The correctness lane clears with an APPROVED; this lane clears with
//     a `Security-Review: pass`, which GitHub records as COMMENTED. The two are never
//     interchangeable: a correctness APPROVE never clears a security fail and a security pass
//     never clears a correctness CR. Callers pass only security-marked reviews here and only
//     correctness-lane approves there.
//   - ONLY THE STANDING PASS COUNTS. The correctness lane admits any after-CR APPROVE that
//     documents the clearance, because rule 1 separately requires an APPROVED to govern at
//     head. A security pass IS the lane's verdict, so the newest standing pass after the last
//     fail is read, and a later fail (marked or not) undoes an earlier pass.
//   - NO `Resolved-Body-Finding:` / `CI-Green-At:` LINES. A pass is a whole-lane verdict: it
//     says the security lane is clean at head, which already covers the one finding the fail
//     declared, and CI stays the gate's own checks-green condition in both lanes. The pass
//     documents only the re-read that this clearance turns on.
//   - A CHAIN, NOT ONE CR. A fail can be answered by a second marked fail before a pass; the
//     edit and the re-read must then postdate, and differ from, every fail in the chain.
//
// WHAT THE GATE CANNOT CHECK. Whether the marker is TRUE — that the fail's whole basis really
// was the PR body and no code at head would change — is the reviewer's claim. The later pass
// and the forge's body-edit record back it up; nothing here can verify it.

import (
	"strconv"
	"strings"
	"time"
)

// SecurityVerdictReview is one security-marked review by the fail's author, submitted after
// the fail. The CALLER supplies only reviews from the reviewer identity that carry a
// `Security-Review:` line, in the forge's ascending submission order.
type SecurityVerdictReview struct {
	Body     string
	CommitID string
	// State is the forge's review state (APPROVED, COMMENTED, CHANGES_REQUESTED, DISMISSED,
	// PENDING). A pass counts only in a state SecurityPassStateStands admits; a fail counts
	// in any state.
	State       string
	SubmittedAt string // RFC3339
}

// SecurityPassStateStands reports whether a `Security-Review: pass` posted in review STATE
// state can stand as the security lane's pass. deskflip's rule-2 clearance and deskboard's
// lift read it. `deskpost ready` keeps its own list (securityPassStates in
// cmd/deskpost/ready.go). The two lists must hold the same states; a test in cmd/deskpost
// pins them to each other and to APPROVED and COMMENTED.
//
// It is an allow-list, so a state the forge adds later grants nothing until someone decides
// it should. APPROVED is the legacy shape (`deskpost review --verdict approve` with a
// security body); COMMENTED is the shape `deskpost security-review --verdict pass` posts.
// PENDING (an unsubmitted draft, visible only to its author) and DISMISSED (an approval a
// maintainer dismissed) are excluded, and so is CHANGES_REQUESTED. The match is exact, as it
// is in `deskpost ready`, whose list carries the history of why each state is in or out. A
// FAIL is never filtered by state: a retraction counts from any state.
func SecurityPassStateStands(state string) bool {
	switch state {
	case "APPROVED", "COMMENTED":
		return true
	}
	return false
}

// twoLaneFail is the refusal for a security fail whose body also carries a correctness verdict
// line. The gate's correctness rule drops every security-marked body, so a clearance here would
// drop the correctness half with it (#5092).
const twoLaneFail = "`Security-Review: fail` also carries a correctness verdict line — a body that speaks for " +
	"both lanes is not cleared by a security pass, because its correctness half would go unread"

// SecurityBodyInput is everything EvaluateSecurityBodyClearance needs.
type SecurityBodyInput struct {
	// FailBody / FailSubmittedAt are the standing `Security-Review: fail` at Head.
	FailBody        string
	FailSubmittedAt string // RFC3339
	Head            string
	// Later is the same reviewer's security-marked reviews after the fail (see
	// SecurityVerdictReview). Reviews at other commits are read too: a later fail at any
	// commit stands (#361), and a pass at another commit clears nothing here.
	Later []SecurityVerdictReview
	// LiveBody and BodyEditedAt are read by the caller at the gate, exactly as for
	// BodyEditInput: "" BodyEditedAt means the forge reports no edit, which refuses.
	LiveBody     string
	BodyEditedAt string
}

// EvaluateSecurityBodyClearance decides whether a standing `Security-Review: fail` at the
// unchanged head is cleared under the marked body-only class. PURE: same inputs, same
// decision. Reason is always populated, so a refusal names its clause.
func EvaluateSecurityBodyClearance(in SecurityBodyInput) BodyEditDecision {
	if !HasSecurityReviewFail(in.FailBody) {
		return BodyEditDecision{Reason: "the review carries no `Security-Review: fail` line — this is the security " +
			"lane's decision and does not read correctness reviews"}
	}
	d := ParseBodyEditDeclaration(in.FailBody, in.Head)
	dec := BodyEditDecision{Declared: d.Declared}
	if !d.Declared {
		dec.Reason = "the `Security-Review: fail` carries no `Blocked-On-Body:` line, so its basis may be code — " +
			"an unmarked security fail standing at head is not cleared at this head; deskflip needs a new commit"
		return dec
	}
	if d.Problem != "" {
		dec.Reason = d.Problem
		return dec
	}
	if HasCorrectnessVerdictLine(in.FailBody) {
		dec.Reason = twoLaneFail
		return dec
	}
	head := strings.TrimSpace(in.Head)
	if head == "" {
		dec.Reason = "the head is unknown — a same-head clearance cannot be established against no head"
		return dec
	}
	lastFailAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.FailSubmittedAt))
	if err != nil {
		dec.Reason = "the security fail's submission time " + in.FailSubmittedAt + " is not readable as RFC3339 — " +
			"whether the pass came after it cannot be established"
		return dec
	}
	chain := []BodyEditDeclaration{d}
	lastFailRaw := strings.TrimSpace(in.FailSubmittedAt)
	var pass *SecurityVerdictReview
	// notStanding names the last head pass after the fail whose review state does not stand,
	// so the refusal says why that pass did not count.
	notStanding, sawNotStanding := "", false
	for i := range in.Later {
		r := in.Later[i]
		switch {
		case HasSecurityReviewFail(r.Body):
			// A later fail undoes any pass before it, and must itself be clearable.
			pass, notStanding, sawNotStanding = nil, "", false
			at, perr := time.Parse(time.RFC3339, strings.TrimSpace(r.SubmittedAt))
			if perr != nil {
				dec.Reason = "a later `Security-Review: fail` has an unreadable submission time (" + r.SubmittedAt +
					") — its place in the order cannot be established"
				return dec
			}
			if strings.TrimSpace(r.CommitID) != head {
				dec.Reason = "a later `Security-Review: fail` stands at " + short12(r.CommitID) + ", not at head " +
					short12(head) + " — a fail stands across a head move, and this clearance applies only at its own head"
				return dec
			}
			ld := ParseBodyEditDeclaration(r.Body, head)
			if !ld.Declared {
				dec.Reason = "a later `Security-Review: fail` (submitted " + strings.TrimSpace(r.SubmittedAt) +
					") carries no `Blocked-On-Body:` line — a later unmarked security fail blocks"
				return dec
			}
			if ld.Problem != "" {
				dec.Reason = "a later `Security-Review: fail`: " + ld.Problem
				return dec
			}
			if HasCorrectnessVerdictLine(r.Body) {
				dec.Reason = "a later " + twoLaneFail
				return dec
			}
			chain = append(chain, ld)
			lastFailAt, lastFailRaw = at, strings.TrimSpace(r.SubmittedAt)
		case HasSecurityReviewPass(r.Body):
			if strings.TrimSpace(r.CommitID) != head {
				continue // a pass at another commit neither grants nor clears at this head
			}
			at, perr := time.Parse(time.RFC3339, strings.TrimSpace(r.SubmittedAt))
			if perr != nil || !at.After(lastFailAt) {
				continue
			}
			if HasCorrectnessVerdictLine(r.Body) {
				dec.Reason = "a `Security-Review: pass` at head " + short12(head) + " (submitted " +
					strings.TrimSpace(r.SubmittedAt) + ") also carries a correctness verdict line — a body that " +
					"speaks for both lanes never clears a security fail, because its correctness half would go unread"
				return dec
			}
			if !SecurityPassStateStands(r.State) {
				notStanding, sawNotStanding = r.State, true
				continue // a dismissed, pending or rejecting review is not a standing pass
			}
			pass = &in.Later[i]
		}
	}
	if pass == nil {
		dec.Reason = "no `Security-Review: pass` from the reviewer at head " + short12(head) +
			" stands after the last security fail (" + lastFailRaw + ")"
		if sawNotStanding {
			dec.Reason += " — a pass at head in review state " + strconv.Quote(notStanding) + " does not stand; " +
				"only APPROVED or COMMENTED carries a standing pass"
		}
		return dec
	}
	editedRaw := strings.TrimSpace(in.BodyEditedAt)
	if editedRaw == "" {
		dec.Reason = "the forge reports no edit of the PR body (no last-edited time) — that the body was edited " +
			"after the security fail cannot be established, and a recorded digest alone never establishes it"
		return dec
	}
	editedAt, err := time.Parse(time.RFC3339, editedRaw)
	if err != nil {
		dec.Reason = "the forge's PR-body last-edited time " + editedRaw + " is not readable as RFC3339 — that the " +
			"body was edited after the security fail cannot be established"
		return dec
	}
	if !editedAt.After(lastFailAt) {
		dec.Reason = "the forge records the PR body's last edit at " + editedRaw + ", not after the security fail (" +
			lastFailRaw + ") — the body was not edited after the block, so there is nothing new to verify"
		return dec
	}
	live := PRBodyDigest(in.LiveBody)
	dg := strings.ToLower(SoleVerdictMarkerValue(pass.Body, bodyRereadDigest, SkipFenced))
	if dg == "" {
		dec.Reason = "the standing `Security-Review: pass` does not document a live-API re-read of the PR body — " +
			"it must carry `Body-Reread-Digest: <64-hex>`"
		return dec
	}
	if dg != live {
		dec.Reason = "the standing pass's `Body-Reread-Digest` (" + dg[:12] + ") is not the digest of the live PR " +
			"body (" + live[:12] + ") — the body it re-read is not the body that is there now"
		return dec
	}
	for _, link := range chain {
		if dg == link.CRDigest {
			dec.Reason = "the pass's re-read digest equals the digest a security fail recorded — the body the " +
				"reviewer failed on is the body there now, so there is nothing new to verify"
			return dec
		}
	}
	dec.Cleared = true
	dec.Reason = "marked security fail cleared: finding " + d.FindingID + ", a later `Security-Review: pass` at " +
		short12(head) + " re-read the live body (digest " + live[:12] + "), and the forge records a body edit at " +
		editedRaw + ", after the last fail"
	return dec
}
