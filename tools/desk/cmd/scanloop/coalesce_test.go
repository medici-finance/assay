package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var coalesceNow = time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

func openPR(ageFromNow time.Duration) *OpenScanPR {
	return &OpenScanPR{Number: 42, Branch: "chore/intake-scan-2026-08-24-1140", CreatedAt: coalesceNow.Add(-ageFromNow)}
}

// TestCoalesce_InsideTheWindowAbsorbs is the positive control.
func TestCoalesce_InsideTheWindowAbsorbs(t *testing.T) {
	d, why := CoalescePolicy{}.Decide(openPR(5*time.Minute), coalesceNow)
	if d != CoalesceInto {
		t.Fatalf("decision = %s (%s), want COALESCE", d, why)
	}
	if !strings.Contains(why, "regenerate") {
		t.Fatalf("the coalesce reason does not state the body regeneration: %s", why)
	}
}

// TestCoalesce_AtTheWindowSealsThePR is THE property of this file. The unbounded rule this replaces
// let every new inbound append to the same PR forever, so it never reached a stable head and could
// not land however green it was. At the boundary the PR must be left alone.
func TestCoalesce_AtTheWindowSealsThePR(t *testing.T) {
	for _, age := range []time.Duration{DefaultCoalesceWindow, DefaultCoalesceWindow + time.Second, 4 * time.Hour} {
		d, why := CoalescePolicy{}.Decide(openPR(age), coalesceNow)
		if d != CoalesceFresh {
			t.Fatalf("age %s: decision = %s, want FRESH-PR — a PR at or past the window must be sealed at a stable head", age, d)
		}
		if !strings.Contains(why, "sealed") {
			t.Fatalf("age %s: reason does not say the PR is sealed: %s", age, why)
		}
	}
}

// TestCoalesce_ExactBoundaryIsNotInclusive pins the comparison itself: `<` not `<=`. A one-second
// drift here is the difference between a bounded window and the unbounded rule.
func TestCoalesce_ExactBoundaryIsNotInclusive(t *testing.T) {
	if d, _ := (CoalescePolicy{}).Decide(openPR(DefaultCoalesceWindow-time.Second), coalesceNow); d != CoalesceInto {
		t.Fatalf("one second INSIDE the window = %s, want COALESCE", d)
	}
	if d, _ := (CoalescePolicy{}).Decide(openPR(DefaultCoalesceWindow), coalesceNow); d != CoalesceFresh {
		t.Fatalf("exactly AT the window = %s, want FRESH-PR", d)
	}
}

func TestCoalesce_NoOpenPRCutsFresh(t *testing.T) {
	if d, _ := (CoalescePolicy{}).Decide(nil, coalesceNow); d != CoalesceFresh {
		t.Fatalf("decision = %s, want FRESH-PR", d)
	}
}

// TestCoalesce_UnreadableAgeNeverCoalesces — could-not-check takes the BOUNDED direction. An extra
// PR costs a review slot; a wrong coalesce re-opens the failure the window exists to close.
func TestCoalesce_UnreadableAgeNeverCoalesces(t *testing.T) {
	pr := &OpenScanPR{Number: 42, Branch: "b"} // CreatedAt unread
	d, why := CoalescePolicy{}.Decide(pr, coalesceNow)
	if d != CoalesceCouldNotCheck {
		t.Fatalf("decision = %s, want COULD-NOT-CHECK", d)
	}
	if d.Act() != CoalesceFresh {
		t.Fatalf("could-not-check resolved to %s, want the bounded FRESH-PR action", d.Act())
	}
	if !strings.Contains(why, "could not be read") {
		t.Fatalf("reason = %s", why)
	}
}

// TestCoalesce_FutureCreatedAtIsCouldNotCheck — clock skew is not a negative age, it is an
// unmeasurable one.
func TestCoalesce_FutureCreatedAtIsCouldNotCheck(t *testing.T) {
	if d, _ := (CoalescePolicy{}).Decide(openPR(-time.Hour), coalesceNow); d != CoalesceCouldNotCheck {
		t.Fatalf("decision = %s, want COULD-NOT-CHECK", d)
	}
}

// TestCoalesce_NegativeWindowDisables — "never coalesce" is expressible because it is the safe
// posture; "always coalesce" deliberately is not.
func TestCoalesce_NegativeWindowDisables(t *testing.T) {
	if d, _ := (CoalescePolicy{Window: -1}).Decide(openPR(time.Second), coalesceNow); d != CoalesceFresh {
		t.Fatalf("decision = %s, want FRESH-PR with coalescing disabled", d)
	}
}

// TestPushAndRegenerate_BothHalvesRun — the regeneration is STRUCTURAL, not a remembered step.
func TestPushAndRegenerate_BothHalvesRun(t *testing.T) {
	var pushed, regenerated bool
	if err := PushAndRegenerate(
		func() error { pushed = true; return nil },
		func() error { regenerated = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !pushed || !regenerated {
		t.Fatalf("pushed=%v regenerated=%v — both halves must run on every push", pushed, regenerated)
	}
}

// TestPushAndRegenerate_MissingRegenerationIsRefused — a push wired without a regeneration is the
// bug, so it is not expressible.
func TestPushAndRegenerate_MissingRegenerationIsRefused(t *testing.T) {
	if err := PushAndRegenerate(func() error { return nil }, nil); err == nil {
		t.Fatal("a push with no regeneration succeeded — the PR would keep stating the previous push's counts")
	}
}

// TestPushAndRegenerate_RegenerationFailureIsLoud — the dangerous state is a push that landed and a
// body that did not move. It must never be swallowed.
func TestPushAndRegenerate_RegenerationFailureIsLoud(t *testing.T) {
	err := PushAndRegenerate(func() error { return nil }, func() error { return errors.New("drift") })
	if err == nil {
		t.Fatal("a failed regeneration after a successful push returned nil")
	}
	if !strings.Contains(err.Error(), "NOT regenerated") {
		t.Fatalf("the error does not name the dangerous state: %v", err)
	}
}
