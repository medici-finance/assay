package main

// cancelledrun_test.go — the checks-green gate on a rollup holding CANCELLED runs (#6124).
//
// The gate judges each check NAME by its LATEST run at the head (latestPerRollupName), so a
// cancelled run that a later run of the same name superseded does not count, and a cancelled
// run that IS the latest run of its name still refuses. Every refusal names the check and the
// state the forge reported for it, from the same classifier that decided the verdict.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Three stamps in chronological (and lexicographic) order, for the twin fixtures below.
const (
	tRun1 = "2026-01-01T00:00:10Z"
	tRun2 = "2026-01-01T00:00:20Z"
	tRun3 = "2026-01-01T00:00:30Z"
)

// regression: #6124
// The issue's own shape: a concurrency group cancelled two `prompt` runs at the head, and a
// later `prompt` run at the same head passed. Every other check is green. The PR flips.
func TestCancelledTwinsFlip(t *testing.T) {
	s := newStub()
	s.rollup = append(greenRollup(),
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tRun1, CompletedAt: tRun1},
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tRun1, CompletedAt: tRun2},
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tRun2, CompletedAt: tRun3},
	)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("cancelled runs superseded by a later pass jammed the flip: rc = %d, want 0\n%s", rc, out)
	}
	if !s.flipped() {
		t.Errorf("the ready mutation never ran: %v", s.requests)
	}
}

// regression: #6124
// A cancelled run with no later passing run of its name still refuses, and the refusal names
// the check and says `cancelled` — never "not completed", since the run did complete.
func TestLoneCancelledRefuses(t *testing.T) {
	s := newStub()
	s.rollup = append(greenRollup(),
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tRun1, CompletedAt: tRun2},
	)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertCancelledRefusal(t, s)
}

// regression: #6124
// The LATER run of the name is the cancelled one: an earlier pass does not supersede a later
// cancellation, so the flip refuses and names the cancelled check.
func TestLaterCancelledRefuses(t *testing.T) {
	s := newStub()
	s.rollup = append(greenRollup(),
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tRun1, CompletedAt: tRun2},
		rollupEntry{Name: "prompt", Status: "COMPLETED", Conclusion: "CANCELLED", StartedAt: tRun2, CompletedAt: tRun3},
	)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	assertCancelledRefusal(t, s)
}

func assertCancelledRefusal(t *testing.T, s *stub) {
	t.Helper()
	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("a cancelled latest run did not refuse: rc = %d, want %d\n%s", rc, deskkit.ExitRefused, out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over a cancelled latest run: %v", m)
	}
	if !strings.Contains(out, "prompt (cancelled)") {
		t.Errorf("the refusal does not name the check and its conclusion %q: %s", "prompt (cancelled)", out)
	}
	if strings.Contains(out, "not completed") || strings.Contains(out, "has not completed") {
		t.Errorf("the refusal calls a completed, cancelled run not completed: %s", out)
	}
}

// regression: #6124
// A not-yet-green rollup names each blocking check and the state the forge reported for it.
func TestPendingNamesTheCheck(t *testing.T) {
	s := newStub()
	s.rollup = append(greenRollup(),
		rollupEntry{Name: "prompt", Status: "QUEUED"},
		rollupEntry{Context: "leak-sweep", State: "PENDING", CreatedAt: tRun1},
	)
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("a pending rollup rc = %d, want %d\n%s", rc, deskkit.ExitUnverifiable, out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutations over a pending rollup: %v", m)
	}
	for _, want := range []string{"prompt (status queued)", "leak-sweep (state pending)"} {
		if !strings.Contains(out, want) {
			t.Errorf("the refusal does not name %q: %s", want, out)
		}
	}
}

// regression: #6124
// judgeEntry is the one classifier behind both the verdict and the refusal text. Each shape
// below gets the verdict the gate has always given it, and a reason that says what the forge
// reported — so a COMPLETED run with no conclusion is never described as "not completed".
func TestJudgeEntryReasons(t *testing.T) {
	cases := []struct {
		e      rollupEntry
		state  ciState
		reason string
	}{
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "SUCCESS"}, ciGreen, "success"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "SKIPPED"}, ciGreen, "skipped"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "NEUTRAL"}, ciGreen, "neutral"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "CANCELLED"}, ciFail, "cancelled"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "FAILURE"}, ciFail, "failure"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "TIMED_OUT"}, ciFail, "timed_out"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "ACTION_REQUIRED"}, ciFail, "action_required"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "STALE"}, ciFail, "stale"},
		{rollupEntry{Name: "a", Status: "COMPLETED", Conclusion: "SOMETHING_NEW"}, ciFail, "something_new"},
		{rollupEntry{Name: "a", Status: "QUEUED"}, ciPending, "status queued"},
		{rollupEntry{Name: "a", Status: "IN_PROGRESS"}, ciPending, "status in_progress"},
		{rollupEntry{Name: "a", Status: "COMPLETED"}, ciPending, "completed with no conclusion"},
		{rollupEntry{Name: "a"}, ciPending, "no status reported"},
		{rollupEntry{Context: "c", State: "SUCCESS"}, ciGreen, "state success"},
		{rollupEntry{Context: "c", State: "PENDING"}, ciPending, "state pending"},
		{rollupEntry{Context: "c", State: "EXPECTED"}, ciPending, "state expected"},
		{rollupEntry{Context: "c", State: "ERROR"}, ciFail, "state error"},
		{rollupEntry{Context: "c", State: "FAILURE"}, ciFail, "state failure"},
		{rollupEntry{Context: "c", State: "WHATEVER"}, ciFail, "state whatever"},
	}
	for _, c := range cases {
		got := judgeEntry(c.e)
		if got.state != c.state || got.reason != c.reason {
			t.Errorf("judgeEntry(%+v) = (%v, %q), want (%v, %q)", c.e, got.state, got.reason, c.state, c.reason)
		}
	}
}

// regression: #6124
// An earlier pass never makes a non-green LATEST run green: whatever the latest run of a
// name reports — pending, queued, failure, timed_out, action_required, cancelled — the
// rollup is not green.
func TestLatestNonGreenNeverGreen(t *testing.T) {
	latest := []rollupEntry{
		{Name: "x", Status: "QUEUED", StartedAt: tRun3},
		{Name: "x", Status: "IN_PROGRESS", StartedAt: tRun3},
		{Name: "x", Status: "COMPLETED", Conclusion: "FAILURE", CompletedAt: tRun3},
		{Name: "x", Status: "COMPLETED", Conclusion: "TIMED_OUT", CompletedAt: tRun3},
		{Name: "x", Status: "COMPLETED", Conclusion: "ACTION_REQUIRED", CompletedAt: tRun3},
		{Name: "x", Status: "COMPLETED", Conclusion: "CANCELLED", CompletedAt: tRun3},
		{Context: "x", State: "PENDING", CreatedAt: tRun3},
	}
	for _, l := range latest {
		earlier := rollupEntry{Name: "x", Status: "COMPLETED", Conclusion: "SUCCESS", StartedAt: tRun1, CompletedAt: tRun2}
		if l.Context != "" {
			earlier = rollupEntry{Context: "x", State: "SUCCESS", CreatedAt: tRun2}
		}
		reduced := latestPerRollupName([]rollupEntry{earlier, l})
		if st := evalRollup(reduced); st == ciGreen || st == ciEmpty {
			t.Errorf("latest run %+v after an earlier pass reduced to %v — a non-green latest run is never green", l, st)
		}
	}
}
