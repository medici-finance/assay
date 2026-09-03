package main

import (
	"math"
	"testing"
)

// --- Golden: the fixed formula, dereferenced against known inputs -----------

// TestAssayScore_GoldenComposite pins the composite against a HAND-COMPUTED
// value from known sub-scores, driven end-to-end through the normalization:
//
//	Flow:    FE = 0.50            -> 0.50 × 100                     = 50
//	Quality: FPY = 1.00 (10/10)   -> 1.00 × 100                     = 100
//	Speed:   L = 4d, band [2,12]  -> 100 × (12−4)/(12−2) = 100×0.8  = 80
//	Value:   V = 3, band [1,6]    -> 100 × (3−1)/(6−1)   = 100×0.4  = 40
//
//	AssayScore = (80 · 50 · 100 · 40)^(1/4) = (16,000,000)^(1/4) = 63.2
//
// Hand check: 80·50=4000; ·100=400000; ·40=16,000,000; sqrt=4000; sqrt=63.2455…
func TestAssayScore_GoldenComposite(t *testing.T) {
	flow := boundedSubScore(0.50, "ok", "flow")
	quality := boundedSubScore(float64(10)/float64(10), "ok", "quality")
	speed := speedSubScore(4, "ok", assayBand{Lo: 2, Hi: 12, N: 20, State: "ok"})
	value := valueSubScore(3, "ok", assayBand{Lo: 1, Hi: 6, N: 13, State: "ok"})

	assertSub(t, "flow", flow, 50)
	assertSub(t, "quality", quality, 100)
	assertSub(t, "speed", speed, 80)
	assertSub(t, "value", value, 40)

	rep := computeAssayScore(speed, flow, quality, value, assayInputs{}, assayBaselineWindow{})
	if rep.Incomplete {
		t.Fatalf("all four dimensions available: incomplete must be false, got true (missing=%v)", rep.Missing)
	}
	if rep.State != "ok" {
		t.Fatalf("state = %q, want ok", rep.State)
	}
	if rep.Score == nil {
		t.Fatal("score is nil, want 63.2")
	}
	if *rep.Score != 63.2 {
		t.Fatalf("AssayScore = %v, want 63.2 (geometric mean of 80,50,100,40)", *rep.Score)
	}
}

// TestAssayScore_IncompleteExcludesNeverZeroes is the could-not-check case: one
// dimension is could-not-check. It MUST be excluded from the geometric mean —
// NOT coerced to 0 (a zero factor would zero the whole score and lie).
//
//	available = {Speed 80, Flow 50, Value 40}  (Quality could-not-check)
//	AssayScore = (80 · 50 · 40)^(1/3) = 160000^(1/3) = 54.3
//
// If Quality were wrongly coerced to 0, the product would be 0 and the score 0
// — so an asserted 54.3 proves exclusion, not zeroing.
func TestAssayScore_IncompleteExcludesNeverZeroes(t *testing.T) {
	speed := speedSubScore(4, "ok", assayBand{Lo: 2, Hi: 12, N: 20, State: "ok"})
	flow := boundedSubScore(0.50, "ok", "flow")
	quality := assayCNC("quality first-pass-yield could-not-check")
	value := valueSubScore(3, "ok", assayBand{Lo: 1, Hi: 6, N: 13, State: "ok"})

	rep := computeAssayScore(speed, flow, quality, value, assayInputs{}, assayBaselineWindow{})

	if !rep.Incomplete {
		t.Fatal("one dimension is could-not-check: incomplete must be true")
	}
	if len(rep.Missing) != 1 || rep.Missing[0] != "quality" {
		t.Fatalf("missing = %v, want [quality]", rep.Missing)
	}
	if rep.Subscores["quality"].State != "could-not-check" || rep.Subscores["quality"].Score != nil {
		t.Fatalf("quality subscore = %+v, want could-not-check with null score (never a fabricated 0)", rep.Subscores["quality"])
	}
	if rep.Score == nil {
		t.Fatal("score is nil, want 54.3 (geometric mean over the 3 available dims)")
	}
	if *rep.Score != 54.3 {
		t.Fatalf("AssayScore = %v, want 54.3 — the could-not-check dim was NOT excluded (a 0 coercion would give 0.0)", *rep.Score)
	}
}

// TestAssayScore_AllCouldNotCheck: with no dimension available the composite is
// itself could-not-check with a nil score (never 0), incomplete true.
func TestAssayScore_AllCouldNotCheck(t *testing.T) {
	cnc := assayCNC("x")
	rep := computeAssayScore(cnc, cnc, cnc, cnc, assayInputs{}, assayBaselineWindow{})
	if rep.State != "could-not-check" {
		t.Fatalf("state = %q, want could-not-check", rep.State)
	}
	if rep.Score != nil {
		t.Fatalf("score = %v, want nil (no dimension available — never a fabricated 0)", *rep.Score)
	}
	if !rep.Incomplete || len(rep.Missing) != 4 {
		t.Fatalf("incomplete=%v missing=%v, want true / all four", rep.Incomplete, rep.Missing)
	}
}

// TestAssayScore_ZeroSubscoreIsIncludedNotExcluded: a LEGITIMATE zero (Speed at
// the slow end of its band) is a real factor that zeroes the geometric mean —
// that is the anti-gaming behaviour, distinct from a could-not-check exclusion.
func TestAssayScore_ZeroSubscoreIsIncludedNotExcluded(t *testing.T) {
	// L=20 with band [2,12]: (12−20)/10 = −0.8 → clamp 0 → Speed=0.
	speed := speedSubScore(20, "ok", assayBand{Lo: 2, Hi: 12, N: 20, State: "ok"})
	assertSub(t, "speed", speed, 0)
	flow := boundedSubScore(0.50, "ok", "flow")
	quality := boundedSubScore(1.0, "ok", "quality")
	value := valueSubScore(3, "ok", assayBand{Lo: 1, Hi: 6, N: 13, State: "ok"})

	rep := computeAssayScore(speed, flow, quality, value, assayInputs{}, assayBaselineWindow{})
	if rep.Incomplete {
		t.Fatal("a zero sub-score is available, not missing — incomplete must be false")
	}
	if rep.Score == nil || *rep.Score != 0 {
		t.Fatalf("score = %v, want 0 (a zero factor drags the geometric mean to 0 — anti-gaming)", rep.Score)
	}
}

// --- Normalization + clamp edges --------------------------------------------

func TestAssayScore_SpeedInverseNormalizationAndClamp(t *testing.T) {
	band := assayBand{Lo: 2, Hi: 12, N: 20, State: "ok"}
	// fast end and beyond → clamps to 100
	assertSub(t, "speed@fast", speedSubScore(2, "ok", band), 100)
	assertSub(t, "speed@faster-than-band", speedSubScore(0, "ok", band), 100)
	// slow end and beyond → clamps to 0
	assertSub(t, "speed@slow", speedSubScore(12, "ok", band), 0)
	assertSub(t, "speed@slower-than-band", speedSubScore(20, "ok", band), 0)
	// mid
	assertSub(t, "speed@mid", speedSubScore(7, "ok", band), 50)
}

func TestAssayScore_ValueForwardNormalizationAndClamp(t *testing.T) {
	band := assayBand{Lo: 1, Hi: 6, N: 13, State: "ok"}
	assertSub(t, "value@low", valueSubScore(1, "ok", band), 0)
	assertSub(t, "value@lower-than-band", valueSubScore(0, "ok", band), 0)
	assertSub(t, "value@high", valueSubScore(6, "ok", band), 100)
	assertSub(t, "value@higher-than-band", valueSubScore(9, "ok", band), 100)
	assertSub(t, "value@mid", valueSubScore(3.5, "ok", band), 50)
}

func TestAssayScore_DegenerateBandIsCouldNotCheck(t *testing.T) {
	band := assayBand{Lo: 5, Hi: 5, N: 20, State: "ok"} // p10==p90
	if got := speedSubScore(5, "ok", band); got.State != "could-not-check" {
		t.Errorf("degenerate speed band: state = %q, want could-not-check", got.State)
	}
	if got := valueSubScore(5, "ok", band); got.State != "could-not-check" {
		t.Errorf("degenerate value band: state = %q, want could-not-check", got.State)
	}
}

// --- Baseline guard ----------------------------------------------------------

func TestAssayScore_BaselineGuardBelowMinObs(t *testing.T) {
	four := []float64{1, 2, 3, 4}
	b := computeAssayBand(four)
	if b.State != "could-not-check" || b.N != 4 {
		t.Fatalf("band over 4 obs = %+v, want could-not-check/n=4 (below the %d floor)", b, assayBaselineMinObs)
	}
	// a dimension whose band tripped the guard is could-not-check
	if s := speedSubScore(3, "ok", b); s.State != "could-not-check" {
		t.Errorf("speed with a guard-tripped band: state = %q, want could-not-check", s.State)
	}
}

func TestAssayScore_BaselineGuardAtMinObs(t *testing.T) {
	// exactly 5 observations clears the guard; p10/p90 nearest-rank.
	obs := []float64{1, 2, 3, 4, 5}
	b := computeAssayBand(obs)
	if b.State != "ok" || b.N != 5 {
		t.Fatalf("band over 5 obs = %+v, want ok/n=5", b)
	}
	if b.Lo != 1 || b.Hi != 5 {
		t.Errorf("band = [%v,%v], want [1,5] (p10=idx0=1, p90=idx4=5)", b.Lo, b.Hi)
	}
}

// --- Bounded primitives could-not-check propagation --------------------------

func TestAssayScore_BoundedPropagatesCouldNotCheck(t *testing.T) {
	if s := boundedSubScore(0, "could-not-check", "flow"); s.State != "could-not-check" || s.Score != nil {
		t.Errorf("flow cnc primitive = %+v, want could-not-check/nil", s)
	}
	if s := boundedSubScore(0, "could-not-check", "quality"); s.State != "could-not-check" || s.Score != nil {
		t.Errorf("quality cnc primitive = %+v, want could-not-check/nil", s)
	}
}

// --- helpers -----------------------------------------------------------------

func assertSub(t *testing.T, name string, got assaySubScore, want float64) {
	t.Helper()
	if got.State != "ok" {
		t.Fatalf("%s: state = %q (%s), want ok", name, got.State, got.Reason)
	}
	if got.Score == nil {
		t.Fatalf("%s: score is nil, want %v", name, want)
	}
	if math.Abs(*got.Score-want) > 1e-9 {
		t.Fatalf("%s: score = %v, want %v", name, *got.Score, want)
	}
}
