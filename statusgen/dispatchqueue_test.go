package main

import (
	"fmt"
	"testing"
)

// TestDispatchViewSurfacesTodoAndInProgressUnclaimed pins the --next-up selection
// semantics (#321): todo + in-progress that are eligible and UNCLAIMED surface;
// a CLAIMED todo, and every implemented/verified brief, are absent. This is the
// exact population distinction from --gate-scores (awaiting-verification), and the
// reason `deskboard dispatch` is a different verb from `deskboard awaiting`.
func TestDispatchViewSurfacesTodoAndInProgressUnclaimed(t *testing.T) {
	s := mkStream("alpha", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},        // surfaces
		Brief{Num: "02", Wave: 0, Status: "in-progress"}, // surfaces
		Brief{Num: "03", Wave: 0, Status: "todo"},        // CLAIMED → excluded
		Brief{Num: "04", Wave: 0, Status: "implemented"}, // awaiting-verification → excluded
		Brief{Num: "05", Wave: 0, Status: "verified"},    // awaiting-verification → excluded
	)
	s.LastTouch = day(0)
	claims := KnownClaims(map[string]bool{"alpha/03": true})
	nu := nextUp([]*Stream{s}, claims, nil)
	view := buildDispatchView(nu, []*Stream{s}, "medici-finance/example", ClaimSource{Known: true})

	got := map[string]string{} // brief -> status
	for _, r := range view.Rows {
		got[r.Brief] = r.Status
		if r.Repo != "medici-finance/example" {
			t.Errorf("row %s: repo = %q, want the view repo", r.Brief, r.Repo)
		}
	}
	if len(got) != 2 {
		t.Fatalf("dispatch view surfaced %d rows, want 2 (alpha/01 todo + alpha/02 in-progress); got %v", len(got), got)
	}
	if got["alpha/01"] != "todo" {
		t.Errorf("alpha/01 not surfaced as todo: %v", got)
	}
	if got["alpha/02"] != "in-progress" {
		t.Errorf("alpha/02 not surfaced as in-progress: %v", got)
	}
	for _, excluded := range []string{"alpha/03", "alpha/04", "alpha/05"} {
		if _, ok := got[excluded]; ok {
			t.Errorf("%s must NOT be in the dispatch queue (claimed or awaiting-verification): %v", excluded, got)
		}
	}
	if !view.ClaimsKnown {
		t.Error("ClaimsKnown should be true when the claim read succeeded")
	}
	if view.ClaimsReason != "" {
		t.Errorf("a known-claims view must carry no reason, got %q", view.ClaimsReason)
	}
}

// TestDispatchViewHeldBackDecompositionAndDegraded pins requirement 3: an empty or
// short queue is distinguishable from a throttled one. A per-stream cap holding
// briefs back is counted in HeldByStreamCap (not lumped into the span cap), and a
// failed claim read surfaces as ClaimsKnown=false WITH a reason (an unfiltered
// superset a dispatcher must not act on).
func TestDispatchViewHeldBackDecompositionAndDegraded(t *testing.T) {
	briefs := make([]Brief, 0, perStreamCap+2)
	for i := 1; i <= perStreamCap+2; i++ {
		briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", i), Wave: 0, Status: "todo"})
	}
	s := mkStream("hot", "active", "P0", briefs...)
	s.LastTouch = day(0)

	nu := nextUp([]*Stream{s}, KnownClaims(map[string]bool{}), nil)
	view := buildDispatchView(nu, []*Stream{s}, "r", ClaimSource{Known: true})
	if view.Shown != perStreamCap {
		t.Fatalf("shown = %d, want perStreamCap %d", view.Shown, perStreamCap)
	}
	if view.Eligible != perStreamCap+2 {
		t.Errorf("eligible = %d, want %d", view.Eligible, perStreamCap+2)
	}
	if view.HeldByStreamCap != 2 {
		t.Errorf("heldByStreamCap = %d, want 2 (the two briefs over the per-stream cap)", view.HeldByStreamCap)
	}
	if view.HeldBySpan != 0 {
		t.Errorf("heldBySpan = %d, want 0 (the span cap did not fire)", view.HeldBySpan)
	}

	// Degraded claim read: ClaimsKnown=false with a reason — the rows are an
	// unfiltered superset.
	deg := buildDispatchView(nextUp([]*Stream{s}, ClaimView{}, nil), []*Stream{s}, "r",
		ClaimSource{Known: false, Reason: "remote unreachable"})
	if deg.ClaimsKnown {
		t.Error("ClaimsKnown should be false for a failed claim read")
	}
	if deg.ClaimsReason == "" {
		t.Error("a degraded view must carry a claims reason so a dispatcher knows not to act on it")
	}
}
