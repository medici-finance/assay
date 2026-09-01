package dorajoin

import "fmt"

// cfr.go — the traced-CFR refinement (spec §8, §5.3, §10). The traced
// defect-inducing rate brief-07 computes (TracedCFRInput) is reported
// ALONGSIDE the delivery-metrics feed's incident-based change-failure rate,
// never merged into or replacing it: an incident-based CFR misses defects
// that never paged anyone, and a traced CFR only sees what blame could
// reattribute, so the two answer different questions and both travel
// together.

// TierComposition is the tier-1/2/3 evidence-tier split (spec §5.1) carried
// beside every traced number. Mirrors qualgen's fixlinkage.go TierComposition
// field-for-field (package-boundary reasons — see measure.go's package doc);
// Tier3Count is reported SEPARATELY and must never be folded into the
// tier-1/2 count, the same discipline qualgen's own type enforces.
type TierComposition struct {
	Tier1Count int `json:"tier1_count"`
	Tier2Count int `json:"tier2_count"`
	Tier3Count int `json:"tier3_count"`
}

// Tier1And2Count is the combined strong-evidence count (tiers 1-2),
// deliberately excluding Tier3Count.
func (tc TierComposition) Tier1And2Count() int { return tc.Tier1Count + tc.Tier2Count }

// CFRInput is the traced side of the refinement — brief-07's per-window
// output (szz.go's TracedCFRInput / MetricTracedCFRInput), translated by the
// caller into this shape: the traced defect-inducing rate itself, the
// trace-rate it was computed at, and the evidence-tier composition behind it.
type CFRInput struct {
	Window string

	// TracedRate is the traced defect-inducing rate (brief-07's
	// TracedCFRInput): distinct traced-defect-inducing changes / merged PRs.
	TracedRate Measure[float64]
	// TraceRate is the run's trace-rate (traced / total identified fixes) the
	// TracedRate above was computed at (spec §10 honest-claims: never a bare
	// rate).
	TraceRate Measure[float64]
	// Tiers is the evidence-tier composition behind the traced fixes.
	Tiers TierComposition
}

// CFRRecord is the joined output: incident-based CFR and traced CFR reported
// side by side for one window, the traced number carrying its trace-rate and
// tier composition (spec §10).
type CFRRecord struct {
	Window string `json:"window"`

	// IncidentCFR is the delivery-metrics feed's own incident-based
	// change-failure rate — untouched, never replaced by the traced number.
	IncidentCFR Measure[float64] `json:"incident_cfr"`

	// TracedCFR, TraceRate and TierComposition travel together (spec §10):
	// NewCFRRecord refuses to construct a record carrying a reported
	// TracedCFR with no TraceRate attached.
	TracedCFR       Measure[float64] `json:"traced_cfr"`
	TraceRate       Measure[float64] `json:"trace_rate"`
	TierComposition TierComposition  `json:"tier_composition"`
}

// NewCFRRecord joins one window's incident-based CFR (from the pluggable
// delivery-metrics source) with brief-07's traced-CFR refinement, and
// enforces the honest-claims emit guard (spec §10): a TracedCFR that carries
// any reported state (measured or measured-zero — a "no traced defects this
// window" answer counts as reported) MUST carry a TraceRate beside it. A
// caller that assembled a CFRInput without setting TraceRate — the
// zero-value Measure, State=="" — gets an error here rather than a record
// that silently ships a bare traced number.
//
// A could-not-measure TracedRate is not "reported" in this sense (there is no
// number to be bare) and is allowed through with an unset TraceRate, though
// in practice brief-07 always attaches one.
func NewCFRRecord(window string, incidentCFR Measure[float64], traced CFRInput) (CFRRecord, error) {
	if traced.Window != "" && traced.Window != window {
		return CFRRecord{}, fmt.Errorf("dorajoin: CFR window mismatch: incident-side %q vs traced-side %q", window, traced.Window)
	}
	tracedReported := traced.TracedRate.State == StateMeasured || traced.TracedRate.State == StateMeasuredZero
	traceRateAttached := traced.TraceRate.State != ""
	if tracedReported && !traceRateAttached {
		return CFRRecord{}, fmt.Errorf("dorajoin: refusing to emit a bare traced CFR for window %q: no trace-rate attached", window)
	}
	return CFRRecord{
		Window:          window,
		IncidentCFR:     incidentCFR,
		TracedCFR:       traced.TracedRate,
		TraceRate:       traced.TraceRate,
		TierComposition: traced.Tiers,
	}, nil
}
