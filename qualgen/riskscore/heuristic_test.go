package riskscore

import "testing"

// TestHeuristic_StandaloneScore checks the §9.1 hand-weighted layer produces a
// usable standalone score in [0,1] from a fully-measured feature set, and that
// the score rises with each risk feature — the fallback must be a real ranker,
// not a constant.
func TestHeuristic_StandaloneScore(t *testing.T) {
	low := HeuristicFeatures{
		HotspotPercentile:         Measured(0.1),
		TracedDefectDensity:       Measured(0.2),
		TopIdentityOwnershipShare: Measured(0.1),
		MissingCouplingPartners:   MeasuredZero(),
	}
	high := HeuristicFeatures{
		HotspotPercentile:         Measured(0.95),
		TracedDefectDensity:       Measured(8.0),
		TopIdentityOwnershipShare: Measured(0.9),
		MissingCouplingPartners:   Measured(5),
	}
	w := DefaultHeuristicWeights()
	ls := Heuristic(low, w)
	hs := Heuristic(high, w)
	if ls.State != StateMeasured && ls.State != StateMeasuredZero {
		t.Fatalf("low score should be measurable, got %q", ls.State)
	}
	if hs.State != StateMeasured {
		t.Fatalf("high score should be measured, got %q", hs.State)
	}
	if !(hs.Score > ls.Score) {
		t.Errorf("high-risk score %.3f should exceed low-risk score %.3f", hs.Score, ls.Score)
	}
	if hs.Score < 0 || hs.Score > 1 {
		t.Errorf("score out of [0,1]: %.3f", hs.Score)
	}
	if hs.UsedFeatures != 4 {
		t.Errorf("expected 4 usable features, got %d", hs.UsedFeatures)
	}
}

// TestHeuristic_ThreeStateFeatureDropped proves a could-not-measure feature is
// DROPPED from the weighted sum (never counted as a zero) and the remaining
// weights renormalize — a partial read yields a partial-but-honest score.
func TestHeuristic_ThreeStateFeatureDropped(t *testing.T) {
	w := DefaultHeuristicWeights()
	full := HeuristicFeatures{
		HotspotPercentile:         Measured(0.9),
		TracedDefectDensity:       Measured(6),
		TopIdentityOwnershipShare: Measured(0.8),
		MissingCouplingPartners:   Measured(4),
	}
	// Same features but the hotspot could not be read.
	partial := full
	partial.HotspotPercentile = CouldNotMeasure("blame failed on the hotspot file")

	fs := Heuristic(full, w)
	ps := Heuristic(partial, w)
	if ps.UsedFeatures != 3 {
		t.Fatalf("expected 3 usable features after a could-not-measure, got %d", ps.UsedFeatures)
	}
	if fs.UsedFeatures != 4 {
		t.Fatalf("expected 4 usable features when all measured, got %d", fs.UsedFeatures)
	}
	// The dropped feature was high; had it been counted as zero, the partial
	// score would have COLLAPSED far below the renormalized one. Renormalization
	// keeps the partial score close to the full score, not near zero.
	if ps.Score < 0.5 {
		t.Errorf("renormalized partial score %.3f should not collapse toward a zero-filled value", ps.Score)
	}
	// The explanation must still carry the could-not-measure feature verbatim.
	if ps.Features.HotspotPercentile.State != StateCouldNotMeasure {
		t.Errorf("explanation lost the could-not-measure feature: %+v", ps.Features.HotspotPercentile)
	}
}

// TestHeuristic_AllUnmeasuredIsCouldNotMeasure proves that when NO feature is
// readable the whole score is could-not-measure — never a fabricated zero.
func TestHeuristic_AllUnmeasuredIsCouldNotMeasure(t *testing.T) {
	none := HeuristicFeatures{
		HotspotPercentile:         CouldNotMeasure("no history"),
		TracedDefectDensity:       CouldNotMeasure("no traces"),
		TopIdentityOwnershipShare: CouldNotMeasure("no blame"),
		MissingCouplingPartners:   CouldNotMeasure("no coupling data"),
	}
	s := Heuristic(none, DefaultHeuristicWeights())
	if s.State != StateCouldNotMeasure {
		t.Fatalf("all-unmeasured must be could-not-measure, got %q (score %.3f)", s.State, s.Score)
	}
	if s.Reason == "" {
		t.Error("could-not-measure score must carry a reason")
	}
	if s.Score != 0 {
		// Score is meaningless here; it must be the zero value, not a computed number.
		t.Errorf("could-not-measure score must not carry a computed value, got %.3f", s.Score)
	}
}
