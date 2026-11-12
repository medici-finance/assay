package main

// The post-flip backstop (medici-finance/assay#6242): right after the ready mutation the verb
// reads the forge's merge state and, when the forge reports BLOCKED or DIRTY, returns the
// change to draft with one comment and a non-zero exit. Ruling:
// https://github.com/medici-finance/assay/issues/6242#issuecomment-6812384155

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeBackstopClock swaps the backstop's clock and sleep for a fake one that advances only
// when the backstop sleeps, so a 30s bound runs instantly. It returns the number of sleeps.
func fakeBackstopClock(t *testing.T) *int {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sleeps := 0
	oldNow, oldSleep := backstopNow, backstopSleep
	backstopNow = func() time.Time { return now }
	backstopSleep = func(d time.Duration) { sleeps++; now = now.Add(d) }
	t.Cleanup(func() { backstopNow, backstopSleep = oldNow, oldSleep })
	return &sleeps
}

// flipWith runs a green flip whose post-flip reads serve states, returning the exit and stderr.
func flipWith(t *testing.T, s *stub, states ...string) (int, string) {
	t.Helper()
	fakeBackstopClock(t)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.mergeStates = states
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	return rc, out
}

func (s *stub) converts() int {
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodPost && strings.Contains(r.Body, "convertPullRequestToDraft") {
			n++
		}
	}
	return n
}

// assertReverted is the BLOCKED/DIRTY outcome: back to draft once, one comment naming the
// state and the full head, exit 5, a line saying the forge disagrees, and no label swap.
func assertReverted(t *testing.T, s *stub, rc int, out, state string) {
	t.Helper()
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused: the flip was reverted)", rc, deskkit.ExitRefused)
	}
	if !s.flipped() || s.converts() != 1 {
		t.Fatalf("flipped=%v converts=%d, want the flip then exactly ONE convertPullRequestToDraft",
			s.flipped(), s.converts())
	}
	if len(s.comments) != 1 || !strings.Contains(s.comments[0], state) || !strings.Contains(s.comments[0], headSHA) {
		t.Errorf("comments = %q, want ONE naming %s and the head %s", s.comments, state, headSHA)
	}
	if !strings.Contains(out, "reverted because the forge disagrees") {
		t.Errorf("stderr does not say the flip was reverted because the forge disagrees:\n%s", out)
	}
	if s.saw(http.MethodDelete, "/labels/"+labelBeforeFlip) {
		t.Errorf("the queue label moved on a reverted flip: %v", s.requests)
	}
}

// regression: #6242
func TestBackstopBlockedReverts(t *testing.T) {
	s := newStub()
	rc, out := flipWith(t, s, "blocked")
	assertReverted(t, s, rc, out, "BLOCKED")
}

// regression: #6242
func TestBackstopDirtyReverts(t *testing.T) {
	s := newStub()
	rc, out := flipWith(t, s, "dirty")
	assertReverted(t, s, rc, out, "DIRTY")
}

// regression: #6242
func TestBackstopCleanStaysReady(t *testing.T) {
	s := newStub()
	rc, out := flipWith(t, s, "clean")
	if rc != deskkit.ExitOK || s.converts() != 0 || len(s.comments) != 0 {
		t.Fatalf("rc=%d converts=%d comments=%q, want 0/0/none\n%s", rc, s.converts(), s.comments, out)
	}
	if s.mergeReads != 1 {
		t.Errorf("the merge state was read %d time(s) after the flip, want 1", s.mergeReads)
	}
}

// Every pass state leaves the change ready: the ruling names only BLOCKED and DIRTY.
func TestBackstopPassStatesStay(t *testing.T) {
	for _, st := range []string{"has_hooks", "unstable", "behind"} {
		t.Run(st, func(t *testing.T) {
			s := newStub()
			rc, out := flipWith(t, s, st)
			if rc != deskkit.ExitOK || s.converts() != 0 || len(s.comments) != 0 {
				t.Fatalf("rc=%d converts=%d comments=%q, want 0/0/none\n%s", rc, s.converts(), s.comments, out)
			}
		})
	}
}

// regression: #6242
func TestBackstopUnknownTimesOut(t *testing.T) {
	s := newStub()
	rc, out := flipWith(t, s, "unknown")
	if rc != deskkit.ExitOK || s.converts() != 0 {
		t.Fatalf("rc=%d converts=%d, want 0/0: UNKNOWN at the deadline leaves the change ready\n%s",
			rc, s.converts(), out)
	}
	if len(s.comments) != 1 || !strings.Contains(s.comments[0], "Could not check") {
		t.Errorf("comments = %q, want ONE could-not-check comment", s.comments)
	}
	if !strings.Contains(out, "could-not-verify") {
		t.Errorf("no could-not-verify notice on stderr:\n%s", out)
	}
	// The bound is 30s at a 3s interval: reads at 0,3,...,30, and none after the bound.
	if s.mergeReads != 11 {
		t.Errorf("merge state read %d times inside the 30s bound, want 11", s.mergeReads)
	}
}

// regression: #6242
func TestBackstopUnknownThenClean(t *testing.T) {
	s := newStub()
	rc, out := flipWith(t, s, "unknown", "unknown", "clean")
	if rc != deskkit.ExitOK || s.converts() != 0 || len(s.comments) != 0 {
		t.Fatalf("rc=%d converts=%d comments=%q, want 0/0/none\n%s", rc, s.converts(), s.comments, out)
	}
	if s.mergeReads != 3 {
		t.Errorf("merge state read %d times, want 3 (re-read while UNKNOWN)", s.mergeReads)
	}
	if !s.saw(http.MethodDelete, "/labels/"+labelBeforeFlip) {
		t.Errorf("the queue label did not move on a flip that stays ready: %v", s.requests)
	}
}

// regression: #6242
func TestBackstopConvertFails(t *testing.T) {
	s := newStub()
	s.convertFail = true
	rc, out := flipWith(t, s, "blocked")
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want %d: a failed revert is loud and non-zero\n%s", rc, deskkit.ExitUnverifiable, out)
	}
	if s.converts() != 1 {
		t.Errorf("convertPullRequestToDraft sent %d time(s), want exactly 1 — never retried", s.converts())
	}
	if !strings.Contains(out, "NOT reverted") {
		t.Errorf("stderr does not say the flip was NOT reverted:\n%s", out)
	}
}
