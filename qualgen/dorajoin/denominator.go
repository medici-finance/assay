package dorajoin

import "strings"

// denominator.go — the quality denominator (spec §8): durable-change volume,
// the DORA join's contribution to the delivery board's denominator side. A
// flow board measures delivery; this reports whether the volume it shipped
// was durable rather than immediately reworked or duplicated.

// DurableVolumeInput is one grain's raw M1 inputs the quality denominator is
// computed from — brief-02's line-operation taxonomy (copied lines) and churn
// (landed lines, churn within the configured window, 14 days the comparable
// default). The caller translates its own qualgen M1 aggregates into this
// shape (package doc); this package never reads a metrics table directly.
type DurableVolumeInput struct {
	Window        string
	Stream        string
	IdentityClass string

	// LandedLines is brief-02's new/updated landed line count for this grain
	// (churn.go's ChurnCounts.NewLines).
	LandedLines Measure[float64]
	// Churn is brief-02's churned-line count within the configured window for
	// this grain (churn.go's ChurnCounts.ChurnedLines).
	Churn Measure[float64]
	// Copied is brief-02's classified-copied-line count for this grain
	// (taxonomy.go's CommitTaxonomy.LineClasses[ClassCopied]).
	Copied Measure[float64]
}

// DurableVolume is the computed quality denominator for one grain.
type DurableVolume struct {
	Window        string           `json:"window"`
	Stream        string           `json:"stream,omitempty"`
	IdentityClass string           `json:"identity_class,omitempty"`
	Value         Measure[float64] `json:"value"`
}

// ComputeDurableVolume computes durable-change volume for one grain:
//
//	landed_lines - churn_14d - copied
//
// (spec §8). Any component that is could-not-measure makes the WHOLE
// denominator could-not-measure, naming every missing component — never a
// subtraction against an assumed-zero, and never a partial number that reads
// like a complete one.
func ComputeDurableVolume(in DurableVolumeInput) DurableVolume {
	out := DurableVolume{Window: in.Window, Stream: in.Stream, IdentityClass: in.IdentityClass}

	var missing []string
	if !in.LandedLines.IsMeasured() {
		missing = append(missing, "landed_lines")
	}
	if !in.Churn.IsMeasured() {
		missing = append(missing, "churn_14d")
	}
	if !in.Copied.IsMeasured() {
		missing = append(missing, "copied")
	}
	if len(missing) > 0 {
		out.Value = CouldNotMeasure[float64]("could not measure: " + strings.Join(missing, ", "))
		return out
	}

	v := in.LandedLines.Value - in.Churn.Value - in.Copied.Value
	if v == 0 {
		out.Value = MeasuredZero[float64]()
		return out
	}
	out.Value = Measured(v)
	return out
}

// ComputeDurableVolumes computes the quality denominator over every supplied
// grain, preserving input order (deterministic, diffable output).
func ComputeDurableVolumes(inputs []DurableVolumeInput) []DurableVolume {
	out := make([]DurableVolume, 0, len(inputs))
	for _, in := range inputs {
		out = append(out, ComputeDurableVolume(in))
	}
	return out
}
