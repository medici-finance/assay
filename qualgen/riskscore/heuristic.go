// Package riskscore is the qualgen PR risk-score layer (spec
// docs/streams/quality/spec.md §9.1). It has two halves that never come apart:
//
//   - the HAND-WEIGHTED heuristic features (this file) — the §9.1 fallback and
//     the human-readable explanation layer, usable standalone; and
//   - the LEARNED JIT defect-prediction model (learned.go, features.go) that
//     graduates from those features once the repo has traced enough of its own
//     defects, and that MUST emit its score alongside the heuristic decomposition
//     it was built from.
//
// The package is a subpackage of the qualgen command, which keeps the frozen
// three-state instrument (spec §3.2) in `package main`. A subpackage cannot
// import package main, so the three-state contract is mirrored here as State +
// Feature. The string values are kept byte-identical to the main package's
// on-disk contract so the two never diverge.
package riskscore

import "math"

// State is the three-state instrument enum (spec §3.2), mirrored from qualgen's
// frozen `Measure` contract. Every risk feature and every score distinguishes
// measured, measured-zero, and could-not-measure — a feature that could not be
// read is NEVER silently a zero.
type State string

const (
	// StateMeasured: a real value was read; Value is meaningful.
	StateMeasured State = "measured"
	// StateMeasuredZero: the instrument ran and the genuine answer is zero.
	StateMeasuredZero State = "measured-zero"
	// StateCouldNotMeasure: the value could not be read; Reason names why and
	// Value is meaningless.
	StateCouldNotMeasure State = "could-not-measure"
)

// Feature is one three-state risk feature: a numeric Value guarded by a State.
// Value is meaningful only when State is StateMeasured (a measured-zero carries
// an implicit 0; a could-not-measure carries no value, only a Reason).
type Feature struct {
	State  State   `json:"state"`
	Value  float64 `json:"value,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

// Measured wraps a real, read feature value.
func Measured(v float64) Feature { return Feature{State: StateMeasured, Value: v} }

// MeasuredZero records a genuine zero — the instrument ran, the answer is zero.
func MeasuredZero() Feature { return Feature{State: StateMeasuredZero} }

// CouldNotMeasure records that the feature could not be read, with a required
// non-empty reason.
func CouldNotMeasure(reason string) Feature {
	return Feature{State: StateCouldNotMeasure, Reason: reason}
}

// number returns the feature's numeric contribution and whether it is usable
// (measured or measured-zero). A could-not-measure feature is not usable and is
// dropped from a weighted sum rather than counted as zero.
func (f Feature) number() (float64, bool) {
	switch f.State {
	case StateMeasured:
		return f.Value, true
	case StateMeasuredZero:
		return 0, true
	default:
		return 0, false
	}
}

// HeuristicFeatures is the §9.1 hand-weighted feature set for one change (per
// touched file, already reduced to the change grain by the caller). Each is a
// three-state Feature so a feature the upstream M1/M2 corpus could not compute
// is reported as could-not-measure, not as a false zero.
//
// This struct is BOTH the input to the heuristic score and the explanation layer
// the learned model must carry: it is the human-readable decomposition beside
// any learned number.
type HeuristicFeatures struct {
	// HotspotPercentile: the touched files' churn×complexity hotspot rank as a
	// percentile in [0,1] (spec §4.3) — higher is more defect-prone history.
	HotspotPercentile Feature `json:"hotspot_percentile"`
	// TracedDefectDensity: SZZ-traced defects per KLoC over the touched files
	// (spec §5.3) — the corpus's own defect history for this surface.
	TracedDefectDensity Feature `json:"traced_defect_density"`
	// TopIdentityOwnershipShare: the largest single author-identity's share of
	// surviving lines in the touched files (spec §4.4) — a SPOF/bus-factor
	// signal; concentration in one identity is brittle.
	TopIdentityOwnershipShare Feature `json:"top_identity_ownership_share"`
	// MissingCouplingPartners: count of historical change-coupling partners
	// (spec §4.5) the change did NOT also touch — the strongest cheap
	// brittleness predictor. Higher is riskier.
	MissingCouplingPartners Feature `json:"missing_coupling_partners"`
}

// HeuristicWeights are the hand-chosen weights of §9.1. They are a FIXED guess at
// what predicts defects — the very guess the learned model graduates past — kept
// here as the transparent default. A consumer MAY override them (spec §9.1:
// "consumers weight them; thresholds live in the consumer's config").
type HeuristicWeights struct {
	HotspotPercentile         float64
	TracedDefectDensity       float64
	TopIdentityOwnershipShare float64
	// MissingCouplingPartnersUnit is the per-missing-partner weight; the raw
	// count is squashed through a saturating transform so one design does not
	// dominate the score.
	MissingCouplingPartners float64
}

// DefaultHeuristicWeights is the transparent §9.1 default weighting. These are a
// documented starting point, deliberately not tuned to any one repo — that
// tuning is exactly what learned.go does from traced history.
func DefaultHeuristicWeights() HeuristicWeights {
	return HeuristicWeights{
		HotspotPercentile:         0.40,
		TracedDefectDensity:       0.30,
		TopIdentityOwnershipShare: 0.15,
		MissingCouplingPartners:   0.15,
	}
}

// HeuristicScore is the standalone §9.1 score together with the feature
// decomposition that explains it. Score is meaningful only when State is
// StateMeasured; when every feature is could-not-measure the whole score is
// could-not-measure (never a fabricated zero).
type HeuristicScore struct {
	State State `json:"state"`
	// Score is the weighted risk in [0,1], meaningful only when measured.
	Score float64 `json:"score,omitempty"`
	// Reason is set only when State is could-not-measure.
	Reason string `json:"reason,omitempty"`
	// Features is the decomposition — the explanation layer. It is ALWAYS
	// carried so the score can always show its reasoning.
	Features HeuristicFeatures `json:"features"`
	// UsedFeatures is how many of the four features were usable (measured or
	// measured-zero); a partially-measured score is still honest about how much
	// it saw.
	UsedFeatures int `json:"used_features"`
}

// Heuristic computes the standalone §9.1 hand-weighted score from a feature set.
// Could-not-measure features are DROPPED (not counted as zero) and the remaining
// weights are renormalized, so a partial read yields a partial-but-honest score
// rather than a silent under-count. If NO feature is usable the result is
// could-not-measure. The full feature set is always returned as the explanation.
func Heuristic(f HeuristicFeatures, w HeuristicWeights) HeuristicScore {
	// Each term contributes weight × normalized-feature-value in [0,1].
	type term struct {
		weight float64
		norm   float64 // feature value normalized to [0,1]
		f      Feature
	}
	terms := []term{
		{w.HotspotPercentile, clamp01Feature(f.HotspotPercentile), f.HotspotPercentile},
		{w.TracedDefectDensity, normDensity(f.TracedDefectDensity), f.TracedDefectDensity},
		{w.TopIdentityOwnershipShare, clamp01Feature(f.TopIdentityOwnershipShare), f.TopIdentityOwnershipShare},
		{w.MissingCouplingPartners, saturate(f.MissingCouplingPartners), f.MissingCouplingPartners},
	}

	var weighted, totalWeight float64
	used := 0
	for _, t := range terms {
		if _, ok := t.f.number(); !ok {
			continue // could-not-measure: dropped, never zero-counted
		}
		weighted += t.weight * t.norm
		totalWeight += t.weight
		used++
	}

	out := HeuristicScore{Features: f, UsedFeatures: used}
	if used == 0 || totalWeight == 0 {
		out.State = StateCouldNotMeasure
		out.Reason = "no §9.1 heuristic feature was measurable for this change"
		return out
	}
	out.Score = weighted / totalWeight // renormalize over the usable weights
	if out.Score == 0 {
		out.State = StateMeasuredZero
		return out
	}
	out.State = StateMeasured
	return out
}

// clamp01Feature reads a feature already in [0,1] and clamps it defensively.
func clamp01Feature(f Feature) float64 {
	v, ok := f.number()
	if !ok {
		return 0
	}
	return clamp01(v)
}

// normDensity squashes a per-KLoC defect density (≥0, unbounded) into [0,1] with
// a saturating transform so a very dense file cannot dominate linearly.
func normDensity(f Feature) float64 {
	v, ok := f.number()
	if !ok || v <= 0 {
		return 0
	}
	// 1 - e^{-v/k}: k chosen so ~5 defects/KLoC reads as high risk (~0.6).
	const k = 5.0
	return 1 - math.Exp(-v/k)
}

// saturate squashes a non-negative count (missing coupling partners) into [0,1].
func saturate(f Feature) float64 {
	v, ok := f.number()
	if !ok || v <= 0 {
		return 0
	}
	// 1 - e^{-v/k}: k chosen so ~3 missing partners reads as high risk.
	const k = 3.0
	return 1 - math.Exp(-v/k)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
