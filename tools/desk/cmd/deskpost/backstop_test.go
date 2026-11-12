package main

// `deskpost ready`'s post-flip backstop (medici-finance/assay#6242): right after the flip it
// reads the forge's merge state and, when the forge reports BLOCKED or DIRTY, returns the PR
// to draft with one comment and a non-zero exit. Ruling:
// https://github.com/medici-finance/assay/issues/6242#issuecomment-6812384155

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeBackstopClock swaps the backstop's clock and sleep for a fake one that advances only
// when the backstop sleeps, so the 30s bound runs instantly.
func fakeBackstopClock(t *testing.T) {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	oldNow, oldSleep := backstopNow, backstopSleep
	backstopNow = func() time.Time { return now }
	backstopSleep = func(d time.Duration) { now = now.Add(d) }
	t.Cleanup(func() { backstopNow, backstopSleep = oldNow, oldSleep })
}

// readyWith runs a green `ready` whose post-flip reads serve states.
func readyWith(t *testing.T, convertFail bool, states ...string) (*fakeGH, int, *bytes.Buffer) {
	t.Helper()
	fakeBackstopClock(t)
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.mergeStates = states
	f.convertFail = convertFail
	return f, run(readyArgs(exampleRepo)), errBuf
}

func bodies(f *fakeGH) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.issueComments {
		b, _ := c["body"].(string)
		out = append(out, b)
	}
	return out
}

func assertReadyReverted(t *testing.T, f *fakeGH, code int, errBuf *bytes.Buffer, state string) {
	t.Helper()
	if code != deskkit.ExitRefused {
		t.Fatalf("ready exit = %d, want %d (the flip was reverted)\n%s", code, deskkit.ExitRefused, errBuf)
	}
	if f.flips != 1 || f.converts != 1 {
		t.Fatalf("flips=%d converts=%d, want the flip then exactly ONE convertPullRequestToDraft", f.flips, f.converts)
	}
	cs := bodies(f)
	if len(cs) != 1 || !strings.Contains(cs[0], state) || !strings.Contains(cs[0], testHead) {
		t.Errorf("comments = %q, want ONE naming %s and the head %s", cs, state, testHead)
	}
	if !strings.Contains(errBuf.String(), "reverted because the forge disagrees") {
		t.Errorf("stderr does not say the flip was reverted because the forge disagrees:\n%s", errBuf)
	}
	if e := lastAudit(t); e.Result != deskkit.ResultRefused {
		t.Errorf("audit result = %q, want %q", e.Result, deskkit.ResultRefused)
	}
}

// regression: #6242
func TestReadyBackstopBlocked(t *testing.T) {
	f, code, errBuf := readyWith(t, false, "blocked")
	assertReadyReverted(t, f, code, errBuf, "BLOCKED")
}

// regression: #6242
func TestReadyBackstopDirty(t *testing.T) {
	f, code, errBuf := readyWith(t, false, "dirty")
	assertReadyReverted(t, f, code, errBuf, "DIRTY")
}

// regression: #6242
func TestReadyBackstopClean(t *testing.T) {
	f, code, errBuf := readyWith(t, false, "clean")
	if code != 0 || f.converts != 0 || len(bodies(f)) != 0 {
		t.Fatalf("exit=%d converts=%d comments=%q, want 0/0/none\n%s", code, f.converts, bodies(f), errBuf)
	}
	if f.mergeReads != 1 {
		t.Errorf("merge state read %d time(s) after the flip, want 1", f.mergeReads)
	}
}

// Every pass state leaves the PR ready: the ruling names only BLOCKED and DIRTY.
func TestReadyBackstopPassStates(t *testing.T) {
	for _, st := range []string{"has_hooks", "unstable", "behind"} {
		t.Run(st, func(t *testing.T) {
			f, code, errBuf := readyWith(t, false, st)
			if code != 0 || f.converts != 0 || len(bodies(f)) != 0 {
				t.Fatalf("exit=%d converts=%d comments=%q, want 0/0/none\n%s", code, f.converts, bodies(f), errBuf)
			}
		})
	}
}

// regression: #6242
func TestReadyBackstopUnknown(t *testing.T) {
	f, code, errBuf := readyWith(t, false, "unknown")
	if code != 0 || f.converts != 0 {
		t.Fatalf("exit=%d converts=%d, want 0/0: UNKNOWN at the deadline leaves the PR ready\n%s",
			code, f.converts, errBuf)
	}
	cs := bodies(f)
	if len(cs) != 1 || !strings.Contains(cs[0], "Could not check") {
		t.Errorf("comments = %q, want ONE could-not-check comment", cs)
	}
	if !strings.Contains(errBuf.String(), "could-not-verify") {
		t.Errorf("no could-not-verify notice on stderr:\n%s", errBuf)
	}
	if f.mergeReads != 11 {
		t.Errorf("merge state read %d times inside the 30s bound, want 11", f.mergeReads)
	}
}

// regression: #6242
func TestReadyBackstopUnknownClean(t *testing.T) {
	f, code, errBuf := readyWith(t, false, "unknown", "unknown", "clean")
	if code != 0 || f.converts != 0 || len(bodies(f)) != 0 {
		t.Fatalf("exit=%d converts=%d comments=%q, want 0/0/none\n%s", code, f.converts, bodies(f), errBuf)
	}
	if f.mergeReads != 3 {
		t.Errorf("merge state read %d times, want 3 (re-read while UNKNOWN)", f.mergeReads)
	}
}

// regression: #6242
func TestReadyBackstopConvertFail(t *testing.T) {
	f, code, errBuf := readyWith(t, true, "blocked")
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("ready exit = %d, want %d: a failed revert is loud and non-zero\n%s",
			code, deskkit.ExitUnverifiable, errBuf)
	}
	if f.converts != 1 {
		t.Errorf("convertPullRequestToDraft sent %d time(s), want exactly 1 — never retried", f.converts)
	}
	if !strings.Contains(errBuf.String(), "NOT reverted") {
		t.Errorf("stderr does not say the flip was NOT reverted:\n%s", errBuf)
	}
}
