package reflex

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Verify #4 (DEREFERENCING, NEGATIVE-PATH): TestRitualNumberRefusedWithout-
// Stratification — attempting to emit a cost-per-durable-KLOC number WITHOUT
// brittleness-band stratification returns an error and produces NO
// serialized readout.
// ---------------------------------------------------------------------------

func TestRitualNumberRefusedWithoutStratification(t *testing.T) {
	readout := CostPerKLOCReadout{
		ModelTier:   "strong",
		Band:        "", // no stratification
		CostPerKLOC: Measured(42.0),
		SampleSize:  10,
	}

	b, err := EmitRitual(RitualReadout{
		Metric:      "cost_per_durable_kloc",
		Band:        readout.Band,
		Confounders: DefaultConfounders(),
		Payload:     readout,
	})
	if err == nil {
		t.Fatalf("expected an error refusing an un-stratified ritual-effectiveness number, got nil (bytes=%q)", b)
	}
	if b != nil {
		t.Fatalf("expected NO serialized readout on refusal, got %q", b)
	}
	if !strings.Contains(err.Error(), "un-stratified") {
		t.Fatalf("error should name the un-stratified refusal, got %q", err.Error())
	}

	// The explicit "unknown" band (a could-not-measure brittleness signal)
	// must be refused identically — it is not a stratum this package can
	// name, so it is not a control.
	b2, err2 := EmitRitual(RitualReadout{
		Metric:      "cost_per_durable_kloc",
		Band:        BandUnknown,
		Confounders: DefaultConfounders(),
		Payload:     readout,
	})
	if err2 == nil || b2 != nil {
		t.Fatalf("BandUnknown must also be refused: err=%v bytes=%q", err2, b2)
	}
}

func TestRitualNumberRefusedWithoutConfounders(t *testing.T) {
	b, err := EmitRitual(RitualReadout{
		Metric:      "cost_per_durable_kloc",
		Band:        BandHigh,
		Confounders: Confounders{}, // no statements
		Payload:     CostPerKLOCReadout{CostPerKLOC: Measured(1.0)},
	})
	if err == nil || b != nil {
		t.Fatalf("a readout with no confounders statements must be refused: err=%v bytes=%q", err, b)
	}
	if !strings.Contains(err.Error(), "confounders") {
		t.Fatalf("error should name the missing confounders block, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Verify #5: TestRitualReadoutCarriesConfounders — a properly stratified
// readout serializes WITH a non-empty confounders block attached.
// ---------------------------------------------------------------------------

func TestRitualReadoutCarriesConfounders(t *testing.T) {
	readout := CostPerKLOCReadout{
		ModelTier:   "strong",
		Band:        BandHigh,
		CostPerKLOC: Measured(123.45),
		SampleSize:  7,
	}
	b, err := EmitRitual(RitualReadout{
		Metric:      "cost_per_durable_kloc",
		Band:        BandHigh,
		Confounders: DefaultConfounders(),
		Payload:     readout,
	})
	if err != nil {
		t.Fatalf("a properly stratified readout with confounders must emit cleanly, got err=%v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty serialized bytes")
	}
	if !strings.Contains(string(b), "confounders") {
		t.Fatalf("serialized readout must carry the confounders key, got %s", b)
	}
	if !strings.Contains(string(b), "observational natural experiment") {
		t.Fatalf("serialized readout must carry the actual confounder statements, got %s", b)
	}
	if !strings.Contains(string(b), "brittleness_band") {
		t.Fatalf("serialized readout must carry the brittleness_band field, got %s", b)
	}
}

func TestBrittlenessBandOf(t *testing.T) {
	cases := []struct {
		name string
		in   Measure[float64]
		want BrittlenessBand
	}{
		{"could-not-measure", CouldNotMeasure[float64]("no history"), BandUnknown},
		{"low", Measured(0.1), BandLow},
		{"low boundary just under", Measured(bandSplitLow - 0.001), BandLow},
		{"medium at low boundary", Measured(bandSplitLow), BandMedium},
		{"medium", Measured(0.5), BandMedium},
		{"high at high boundary", Measured(bandSplitHigh), BandHigh},
		{"high", Measured(0.95), BandHigh},
		{"measured zero", MeasuredZero[float64](), BandLow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BrittlenessBandOf(c.in); got != c.want {
				t.Errorf("BrittlenessBandOf(%+v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestEmitGateYieldNeverRefuses(t *testing.T) {
	// Gate-yield output carries no stratification requirement — it is a
	// direct measurement, not a causal ritual-effectiveness claim.
	lanes := []LaneYield{{
		Lane:        "security",
		Catches:     8,
		Escapes:     2,
		CatchRate:   Measured(0.8),
		EscapeRate:  Measured(0.2),
		LatencyCost: CouldNotMeasure[float64]("no latency samples recorded for lane security"),
	}}
	b, err := EmitGateYield(lanes)
	if err != nil {
		t.Fatalf("EmitGateYield must not refuse a normal lane readout, got err=%v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty serialized bytes")
	}
}
