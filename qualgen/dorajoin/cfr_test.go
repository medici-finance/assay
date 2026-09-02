package dorajoin

import "testing"

// TestTracedCFRCarriesTierAndRate (Verify item 4): the emitted traced-CFR
// record carries a non-empty trace-rate and a tier-1/2/3 split; a
// serialization with a bare rate and no trace-rate FAILS the emit guard.
func TestTracedCFRCarriesTierAndRate(t *testing.T) {
	tiers := TierComposition{Tier1Count: 3, Tier2Count: 2, Tier3Count: 1}
	rec, err := NewCFRRecord("2026-08", Measured(0.12), CFRInput{
		Window:     "2026-08",
		TracedRate: Measured(0.08),
		TraceRate:  Measured(0.64),
		Tiers:      tiers,
	})
	if err != nil {
		t.Fatalf("NewCFRRecord: unexpected error: %v", err)
	}
	if rec.TraceRate.State != StateMeasured || rec.TraceRate.Value == 0 {
		t.Fatalf("expected a non-empty trace-rate, got %+v", rec.TraceRate)
	}
	if rec.TierComposition != tiers {
		t.Fatalf("tier composition not carried through: got %+v want %+v", rec.TierComposition, tiers)
	}
	if rec.TierComposition.Tier1And2Count() != 5 {
		t.Fatalf("Tier1And2Count = %d, want 5 (tier3 excluded)", rec.TierComposition.Tier1And2Count())
	}

	// A bare traced rate with no trace-rate attached FAILS the emit guard.
	_, err = NewCFRRecord("2026-08", Measured(0.12), CFRInput{
		Window:     "2026-08",
		TracedRate: Measured(0.08),
		// TraceRate deliberately left as the zero-value Measure (State=="").
		Tiers: tiers,
	})
	if err == nil {
		t.Fatalf("expected the emit guard to refuse a bare traced CFR with no trace-rate, got nil error")
	}
}

// TestTracedCFRMeasuredZeroStillGuarded: a "no traced defects this window"
// measured-zero answer is still a REPORTED number, so it still requires its
// trace-rate.
func TestTracedCFRMeasuredZeroStillGuarded(t *testing.T) {
	_, err := NewCFRRecord("2026-08", Measured(0.0), CFRInput{
		Window:     "2026-08",
		TracedRate: MeasuredZero[float64](),
	})
	if err == nil {
		t.Fatalf("expected the emit guard to refuse a measured-zero traced CFR with no trace-rate attached")
	}
}

// TestTracedCFRCouldNotMeasurePassesWithoutTraceRate: a could-not-measure
// traced rate carries no number to be bare, so it is not blocked by the
// trace-rate guard.
func TestTracedCFRCouldNotMeasurePassesWithoutTraceRate(t *testing.T) {
	rec, err := NewCFRRecord("2026-08", Measured(0.0), CFRInput{
		Window:     "2026-08",
		TracedRate: CouldNotMeasure[float64]("no merged-PR denominator for this window"),
	})
	if err != nil {
		t.Fatalf("unexpected error for a could-not-measure traced rate: %v", err)
	}
	if rec.TracedCFR.State != StateCouldNotMeasure {
		t.Fatalf("expected the could-not-measure state to pass through, got %q", rec.TracedCFR.State)
	}
}

func TestTracedCFRWindowMismatchRejected(t *testing.T) {
	_, err := NewCFRRecord("2026-08", Measured(0.1), CFRInput{
		Window:     "2026-09",
		TracedRate: Measured(0.05),
		TraceRate:  Measured(0.5),
	})
	if err == nil {
		t.Fatalf("expected a window mismatch between incident-side and traced-side to be rejected")
	}
}

// TestIncidentCFRNeverReplacedByTraced: the incident-based CFR travels
// through NewCFRRecord untouched, alongside the traced number — never merged
// or overwritten.
func TestIncidentCFRNeverReplacedByTraced(t *testing.T) {
	rec, err := NewCFRRecord("2026-08", Measured(0.20), CFRInput{
		Window:     "2026-08",
		TracedRate: Measured(0.05),
		TraceRate:  Measured(0.9),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.IncidentCFR.Value != 0.20 {
		t.Fatalf("incident CFR = %v, want the untouched 0.20 input", rec.IncidentCFR.Value)
	}
	if rec.TracedCFR.Value != 0.05 {
		t.Fatalf("traced CFR = %v, want 0.05", rec.TracedCFR.Value)
	}
}
