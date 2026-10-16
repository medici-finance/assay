package deskkit

import (
	"testing"
	"time"
)

// regression: #2163
// TestForgeReplayKeepsResolved: a restart replays the last forge observation (the repair PR is
// still merged). That replay must not reopen a resolved obligation into a second owed
// reverification — resolution is terminal, and an observation only ever advances state.
func TestForgeReplayKeepsResolved(t *testing.T) {
	now := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	o := NewRepairObligation("medici-finance/assay", "example-stream/17", "receipt/r9", []int{2},
		BlockerImplementation, "repro", "expected", "", now.Format(time.RFC3339))
	o = o.Claim("session-A", now)

	merged := RepairForgeObservation{Merged: true, MergedSHA: "cafef00d",
		RepairPR: "medici-finance/assay#5", RepairedBy: "worker-a"}
	for _, step := range []RepairForgeObservation{
		{ReviewOpened: true, RepairPR: "medici-finance/assay#5", RepairedBy: "worker-a"},
		{Approved: true},
		merged,
	} {
		var changed bool
		if o, changed = ApplyForgeObservation(o, step); !changed {
			t.Fatalf("observation %+v did not advance the obligation (state %s)", step, o.State)
		}
	}
	if o.State != RepairAwaitingReverification {
		t.Fatalf("after merge: state %s, want %s", o.State, RepairAwaitingReverification)
	}
	resolved, ok, reason := o.Resolve("verifier-b", "cafef00d")
	if !ok {
		t.Fatalf("independent pass must resolve: %s", reason)
	}

	// Replay every observation in the order a restart could deliver it.
	for _, step := range []RepairForgeObservation{merged, {Approved: true}, {ReviewOpened: true}} {
		got, changed := ApplyForgeObservation(resolved, step)
		if changed || got.State != RepairResolved {
			t.Fatalf("replaying %+v on a resolved obligation moved it to %s (changed=%v) — a resolved "+
				"obligation must stay resolved", step, got.State, changed)
		}
	}

	// And replaying the merge on the still-open obligation is a no-op, never a second wake.
	if again, changed := ApplyForgeObservation(o, merged); changed || again.State != RepairAwaitingReverification {
		t.Fatalf("replaying the merge must not change an awaiting-reverification obligation: %s changed=%v",
			again.State, changed)
	}
}
