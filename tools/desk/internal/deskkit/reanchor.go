package deskkit

// reanchor.go — the stale-approval test (#3093).
//
// GitHub RE-ANCHORS a review's commit_id: a review submitted while commit A was the head can
// later be reported at a NEWER head B (observed on a merge commit whose committer date was
// seven hours after the review's submitted_at). Every reader that decides "approved at head"
// compares the review's commit_id with the head sha, and on that comparison alone a
// re-anchored APPROVED looks like a fresh verdict at B — worse, under the #37 reduction it
// looks like a GENUINE PUSH after a standing CHANGES_REQUESTED at A, so it clears the block.
//
// The second, independent signal is time: a review cannot have verified a commit that did
// not exist yet. A grant (an APPROVED, or a security pass) pinned at head whose submitted_at
// is BEFORE the head commit's committer date is stale — it is not approval at that head and
// not a re-verification that clears anything. Readers apply this ONLY where a review GRANTS;
// a CHANGES_REQUESTED, a failing marker or a finding is never dropped by it (that would
// loosen the gate rather than tighten it).
//
// Residual, stated so the control's reach is honest: the committer date is set by whoever
// made the commit. A FUTURE-dated committer date makes a genuine approval read stale — the
// over-refusal direction, fail closed. A BACKDATED committer date can make a re-anchored
// approval read current; this test does not defend against a pusher who forges commit
// dates, only against the forge moving a review's anchor on its own.

import (
	"strings"
	"time"
)

// ReviewPredatesHead reports whether a review that a reader has ALREADY established is
// pinned at the head (its commit_id equals the head sha) was submitted BEFORE the head
// commit's committer date — i.e. it cannot be a verdict on that commit and must not count as
// one. At-or-after is a verdict at head; strictly before is stale.
//
// Fail closed on every absence: a zero headCommittedAt (the head commit's date could not be
// read) and a submittedAt that is empty or not RFC3339 both report true — "could not show
// the review postdates the head" is not "the review postdates the head".
func ReviewPredatesHead(submittedAt string, headCommittedAt time.Time) bool {
	if headCommittedAt.IsZero() {
		return true
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(submittedAt))
	if err != nil {
		return true
	}
	return at.Before(headCommittedAt)
}

// ParseCommitDate parses a forge-reported commit date (RFC3339). An empty or unparseable
// value is the zero time, which ReviewPredatesHead reads as could-not-check (stale).
func ParseCommitDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}
