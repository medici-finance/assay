package main

// The re-flip latch (#7096): a revert recording its cause holds the head only while a fresh
// merge-requirement read shows that cause; a transient mergeable=UNKNOWN never writes one.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// causeMarker is the backstop's revert marker recording causes, spelled out so the wire format
// is pinned.
func causeMarker(head, causes string) string {
	return "<!-- assay:ready-backstop-revert head=" + head + " cause=" + causes + " -->"
}

func metReq(t *testing.T, review string) string {
	return mergeReqJSON(t, headSHA, "MERGEABLE", review, [2]string{"build-test", "SUCCESS"})
}

// A revert recorded for a review decision that a fresh read shows APPROVED no longer holds the
// head: it flips, and says what cleared (a review re-approved at the same head).
//
// regression: #7096
func TestReflipRecordedCauseCleared(t *testing.T) {
	s := newStub()
	s.priorComments = [][2]string{{reviewerBot(t), "Returned to draft by the post-flip check.\n\n" + causeMarker(headSHA, "review")}}
	s.mergeReqJSON = metReq(t, "APPROVED")
	rc, out := flipWith(t, s, "clean")
	if rc != deskkit.ExitOK || !s.flipped() {
		t.Fatalf("rc=%d flipped=%v, want a flip: the recorded cause cleared\n%s", rc, s.flipped(), out)
	}
}

// The same revert while the review decision still reads CHANGES_REQUESTED keeps refusing.
//
// regression: #7096
func TestReflipRecordedCauseHolds(t *testing.T) {
	s := newStub()
	s.priorComments = [][2]string{{reviewerBot(t), "Returned to draft by the post-flip check.\n\n" + causeMarker(headSHA, "review")}}
	s.mergeReqJSON = metReq(t, "CHANGES_REQUESTED")
	rc, out := flipWith(t, s, "clean")
	if rc != deskkit.ExitUnverifiable || s.flipped() {
		t.Fatalf("rc=%d flipped=%v, want %d and no flip: the cause still holds\n%s", rc, s.flipped(), deskkit.ExitUnverifiable, out)
	}
	if !strings.Contains(out, "not-reverted") || !strings.Contains(out, "CHANGES_REQUESTED") {
		t.Errorf("the refusal does not name the condition and the holding cause:\n%s", out)
	}
}

// A marker written before causes were recorded clears once a fresh read shows every requirement
// met; with one unmet, or the read failing (TestReflipSameHeadRefused: it answers 502), it holds.
//
// regression: #7096
func TestReflipLegacyMarker(t *testing.T) {
	s := newStub()
	s.priorComments = [][2]string{{reviewerBot(t), "Returned to draft by the post-flip check.\n\n" + revertMarker(headSHA)}}
	s.mergeReqJSON = metReq(t, "APPROVED")
	if rc, out := flipWith(t, s, "clean"); rc != deskkit.ExitOK || !s.flipped() {
		t.Fatalf("rc=%d flipped=%v, want a flip: every requirement now reads met\n%s", rc, s.flipped(), out)
	}
	d := newStub()
	d.priorComments = s.priorComments
	d.mergeReqJSON = mergeReqJSON(t, headSHA, "MERGEABLE", "APPROVED", [2]string{"build-test", "FAILURE"})
	if rc, out := flipWith(t, d, "clean"); rc != deskkit.ExitUnverifiable || d.flipped() {
		t.Fatalf("rc=%d flipped=%v, want %d: a legacy marker holds while any requirement is unmet\n%s",
			rc, d.flipped(), deskkit.ExitUnverifiable, out)
	}
}

// A post-flip BLOCKED read whose mergeable verdict stays UNKNOWN through the bound leaves the
// change ready with a could-not-check comment and writes no revert marker.
//
// regression: #7096
func TestBackstopMergeableUnknownNoLatch(t *testing.T) {
	s := newStub()
	s.mergeReqJSON = mergeReqJSON(t, headSHA, "UNKNOWN", "APPROVED", [2]string{"build-test", "SUCCESS"})
	rc, out := flipWith(t, s, "blocked")
	if rc != deskkit.ExitOK || s.converts() != 0 {
		t.Fatalf("rc=%d converts=%d, want 0/0: an unsettled verdict is could-not-check\n%s", rc, s.converts(), out)
	}
	if len(s.comments) != 1 || strings.Contains(s.comments[0], "assay:ready-backstop-revert") ||
		!strings.Contains(s.comments[0], "mergeable UNKNOWN") {
		t.Errorf("comments = %q, want ONE could-not-check comment and no revert marker", s.comments)
	}
}
