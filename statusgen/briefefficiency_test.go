package main

import "testing"

func TestCompleted_TransitionEdgesOnly_ClosedIntervals(t *testing.T) {
	history := []HistoryEntry{
		{Brief: "s/01", From: "", To: "todo", Ts: "2026-08-01T00:00:00Z"},
		{Brief: "s/01", From: "todo", To: "in-progress", Ts: "2026-08-03T00:00:00Z"},        // closes a 2d todo dwell
		{Brief: "s/01", From: "in-progress", To: "implemented", Ts: "2026-08-05T00:00:00Z"}, // closes a 2d in-progress dwell
		// s/01's "implemented" stage is still open (no later record) — must NOT appear.
		{Brief: "s/02", From: "", To: "todo", Ts: "2026-08-10T00:00:00Z"}, // no later record at all — open, excluded
	}
	edges := completedTransitionEdges(history)
	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2 (only completed dwells): %+v", len(edges), edges)
	}
	var sawTodo, sawInProgress bool
	for _, e := range edges {
		switch e.Status {
		case "todo":
			sawTodo = true
			if got := e.End.Sub(e.Start).Hours(); got != 48 {
				t.Errorf("todo dwell = %vh, want 48h", got)
			}
		case "in-progress":
			sawInProgress = true
		case "implemented":
			t.Error("s/01's still-open implemented stage must not produce an edge")
		}
	}
	if !sawTodo || !sawInProgress {
		t.Errorf("missing expected edges: todo=%v in-progress=%v", sawTodo, sawInProgress)
	}
}

// The correctness core: touch counts ONLY in-progress dwell; wait accumulates
// todo/implemented/verified dwell. This is what would silently break if the
// stage classification were wrong (e.g. counting "implemented" as touch would
// make every brief look far more efficient than it is).
func TestComputeFlow_EfficiencyTouchVs_WaitSplit(t *testing.T) {
	history := []HistoryEntry{}
	// Build gtSmallN+1 completed brief lifecycles so the result clears the
	// thin-data threshold: each contributes a 1h todo (wait) + 3h in-progress
	// (touch) interval, both closing inside the window.
	n := gtSmallN + 1
	for i := 0; i < n; i++ {
		id := "s/0" + string(rune('1'+i))
		history = append(history,
			HistoryEntry{Brief: id, From: "", To: "todo", Ts: "2026-08-10T00:00:00Z"},
			HistoryEntry{Brief: id, From: "todo", To: "in-progress", Ts: "2026-08-10T01:00:00Z"},
			HistoryEntry{Brief: id, From: "in-progress", To: "implemented", Ts: "2026-08-10T04:00:00Z"},
		)
	}
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-20T00:00:00Z")
	rep := computeFlowEfficiency(history, since, until)
	if rep.State != "ok" {
		t.Fatalf("state = %q, want ok (n=%d completed intervals)", rep.State, rep.TouchIntervals+rep.WaitIntervals)
	}
	wantTouch := int64(3 * 3600 * n)
	wantWait := int64(1 * 3600 * n)
	if rep.TouchSeconds != wantTouch {
		t.Errorf("touch_seconds = %d, want %d", rep.TouchSeconds, wantTouch)
	}
	if rep.WaitSeconds != wantWait {
		t.Errorf("wait_seconds = %d, want %d", rep.WaitSeconds, wantWait)
	}
	wantEff := float64(wantTouch) / float64(wantTouch+wantWait)
	if diff := rep.Efficiency - wantEff; diff > 0.001 || diff < -0.001 {
		t.Errorf("efficiency = %v, want %v", rep.Efficiency, wantEff)
	}
}

// Thin data (fewer than gtSmallN completed intervals in the window) must
// render could-not-check, never a headline ratio computed from 1-2 samples.
func TestComputeFlow_EfficiencyThinDataIs_CouldNotCheck(t *testing.T) {
	history := []HistoryEntry{
		{Brief: "s/01", From: "", To: "todo", Ts: "2026-08-10T00:00:00Z"},
		{Brief: "s/01", From: "todo", To: "in-progress", Ts: "2026-08-10T01:00:00Z"},
	}
	rep := computeFlowEfficiency(history, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check (only 1 completed interval, below gtSmallN=%d)", rep.State, gtSmallN)
	}
}

// A completed dwell whose terminal instant falls OUTSIDE the window must be
// excluded, mirroring computeDoraTiming's windowing-on-terminal-instant
// convention (doratiming.go).
func TestComputeFlow_EfficiencyWindowsOn_TerminalInstant(t *testing.T) {
	history := []HistoryEntry{
		{Brief: "s/01", From: "", To: "todo", Ts: "2025-01-01T00:00:00Z"},
		{Brief: "s/01", From: "todo", To: "in-progress", Ts: "2025-01-02T00:00:00Z"}, // ends in 2025 — outside the 2026 window
	}
	rep := computeFlowEfficiency(history, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.TouchIntervals != 0 || rep.WaitIntervals != 0 {
		t.Errorf("an out-of-window dwell must be excluded, got touch=%d wait=%d", rep.TouchIntervals, rep.WaitIntervals)
	}
}
