package reflex

import (
	"encoding/json"
	"fmt"
)

// stratify.go implements task item 3: the observational-validity guard.
// Ritual effectiveness is a NATURAL EXPERIMENT, not a controlled one (spec
// §7.2) — a fleet's harder code draws BOTH a stronger execution tier AND
// more churn, so a raw tier-vs-outcome gap is not evidence the tier caused
// the outcome. Brittleness-band stratification is the MINIMUM control, and a
// confounders block travels with every readout (spec §10 honest-claims).
//
// This file is the brief's single point of failure, by design: EmitRitual is
// the ONE serialization/emit gate both gateyield.go and ritual.go route
// their output through (EmitGateYield below is the sibling path for
// gate-yield's own output, kept on the SAME choke point rather than a
// separate ad hoc json.Marshal call). Because the confounders block is
// attached as DATA on the readout itself, and EmitRitual is the only path
// that produces bytes, a naive ritual-effectiveness number cannot reach an
// artifact even if a caller forgets to ask for stratification.

// BrittlenessBand buckets a continuous brittleness/hotspot signal (e.g. the
// PR risk-score layer's HotspotPercentile, spec §9.1) into the coarse strata
// a ritual-effectiveness join stratifies by. The raw percentile is never
// used directly as a join key: a continuous key invites a spurious precision
// no natural experiment supports, and a coarse, named band is what "the
// minimum control" (spec §7.2) means in practice.
type BrittlenessBand string

const (
	BandLow    BrittlenessBand = "low"
	BandMedium BrittlenessBand = "medium"
	BandHigh   BrittlenessBand = "high"
	// BandUnknown records that the input brittleness signal was
	// could-not-measure. It is a DISTINCT band, never silently folded into
	// "low" — and EmitRitual treats it exactly like an empty band: refused,
	// never emitted, because a stratum this package cannot name is not a
	// control.
	BandUnknown BrittlenessBand = "unknown"
)

// Brittleness-band split points. The advisory hotspot threshold qualgen
// already uses elsewhere (check.go's hotspotAdvisoryPercentile = 0.5) is a
// two-way split; ritual joins need three strata (a stronger tier is expected
// to concentrate on the HIGH band, so collapsing medium into either low or
// high would hide exactly the comparison §7.2 asks for), so the unit
// interval is split into equal thirds.
const (
	bandSplitLow  = 1.0 / 3.0
	bandSplitHigh = 2.0 / 3.0
)

// BrittlenessBandOf buckets a hotspot/brittleness percentile Measure into a
// band. A could-not-measure percentile bands as BandUnknown — never a
// guessed band (spec §3.2).
func BrittlenessBandOf(percentile Measure[float64]) BrittlenessBand {
	if !percentile.IsMeasured() {
		return BandUnknown
	}
	switch {
	case percentile.Value < bandSplitLow:
		return BandLow
	case percentile.Value < bandSplitHigh:
		return BandMedium
	default:
		return BandHigh
	}
}

// Confounders is the mandatory acknowledgement block a ritual-effectiveness
// readout carries (spec §7.2, §10 honest-claims): the natural-experiment
// caveats a reader must see beside the number, never a separate document a
// bare number could be copied away from. Statements is never empty on an
// emitted ritual readout — EmitRitual enforces this.
type Confounders struct {
	Statements []string `json:"statements"`
}

// DefaultConfounders is the spec §7.2 confounder acknowledgement every
// ritual-effectiveness readout in this stream carries verbatim, so a caller
// never hand-authors (and risks omitting or watering down) the required
// language. A caller with additional, readout-specific confounders should
// append to this slice rather than replace it.
func DefaultConfounders() Confounders {
	return Confounders{Statements: []string{
		"this is an observational natural experiment, not a controlled trial: authoring attributes (model tier, Verify-depth, lane coverage) were not randomly assigned",
		"harder code draws BOTH a stronger execution tier AND more churn — tier/depth and difficulty are confounded, so a raw gap is not evidence the ritual caused the outcome",
		"brittleness-band stratification is the minimum control applied here; residual confounding within a band (task complexity, author experience, review-lane assignment) is not ruled out",
	}}
}

// RitualReadout is the emit-time envelope every ritual-effectiveness number
// (cost per durable KLOC by tier × band, Verify-depth vs escape rate) is
// wrapped in before serialization. Metric names the readout, Band and
// Confounders carry the mandatory stratification and acknowledgement, and
// Payload is the actual computed value(s) (a *CostPerKLOCReadout,
// []VerifyDepthEscapeReadout, or similar) — kept generic so this envelope
// does not need to change shape as ritual.go's own readout types evolve.
type RitualReadout struct {
	Metric      string          `json:"metric"`
	Band        BrittlenessBand `json:"brittleness_band"`
	Confounders Confounders     `json:"confounders"`
	Payload     any             `json:"payload"`
}

// EmitRitual is the mandatory emit gate for a ritual-effectiveness number
// (ground rule: "NEVER emit a ritual-effectiveness number that is not
// brittleness-band stratified and carrying its confounders block — the
// guard is mandatory, not advisory"). It REFUSES — returns a non-nil error
// and produces NO bytes — a readout whose Band is empty or BandUnknown, or
// whose Confounders carries no statements. This is the honest-claims
// enforcement point brief-12's single-point-of-failure design names: no
// other path in this package serializes a RitualReadout.
func EmitRitual(r RitualReadout) ([]byte, error) {
	if r.Band == "" || r.Band == BandUnknown {
		return nil, fmt.Errorf("reflex: refusing to emit ritual-effectiveness metric %q un-stratified (band=%q) — brittleness-band stratification is the minimum control (spec §7.2)", r.Metric, r.Band)
	}
	if len(r.Confounders.Statements) == 0 {
		return nil, fmt.Errorf("reflex: refusing to emit ritual-effectiveness metric %q without its confounders block (spec §7.2, §10 honest-claims)", r.Metric)
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reflex: marshal ritual readout %q: %w", r.Metric, err)
	}
	return b, nil
}

// EmitGateYield is the sibling emit path for gate-yield accounting output
// (task item 3: "the emit gate BOTH gateyield and ritual outputs pass
// through"). Gate-yield's catch/escape rates are direct measurements read
// off the M3 review-escape overlay, not a confounded causal claim, so this
// path carries no stratification requirement — but keeping it on the SAME
// choke point (rather than an ad hoc json.Marshal at each call site) means a
// future ritual-shaped addition to gate-yield cannot bypass the guard by
// accident: it would have to be routed through EmitRitual explicitly.
func EmitGateYield(lanes []LaneYield) ([]byte, error) {
	b, err := json.MarshalIndent(lanes, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reflex: marshal gate-yield readout: %w", err)
	}
	return b, nil
}
