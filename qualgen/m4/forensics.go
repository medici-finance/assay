// Package m4 implements quality's M4 reflexivity layer over the M1/M2
// corpus (spec §7): a JOIN, not new mining — no history-walking code lives
// here (spec §7 preamble, quality/13 facts).
//
// This file is quality/13's session-forensics join (spec §7.3): for each PR
// the M1/M2 corpus has an outcome for, pull that PR's harness telemetry
// through the pluggable telemetry.TelemetrySource seam and join it to the
// PR's M1 churn outcome and M2 defect outcome, then emit per-harness-behavior
// correlation output with three-state coverage reported beside every number
// (honest-claims discipline, spec §10).
//
// OSS boundary: this package depends on the qualgen/telemetry INTERFACE
// only, never a concrete telemetry source — the reference file-based
// adapter that package ships is wired in by a caller, not named here.
package m4

import "github.com/medici-finance/assay/qualgen/telemetry"

// ChurnOutcome is one PR's M1 churn/rework outcome (quality/02, spec §4.2),
// the shape this join consumes as input. It is deliberately a minimal,
// package-local record rather than a shared M1 artifact type: this join
// takes it as a parameter (no Store read, no new mining), so whatever
// eventually assembles a live M1 corpus into this shape — a future qualgen
// mode, or a fixture in a test — is free to do so without this package
// depending on qualgen's internal mining types.
type ChurnOutcome struct {
	Key telemetry.PRKey
	// ChurnRate is this PR's churned/new-lines rate (spec §4.2). Three-state:
	// could-not-measure when the PR landed no measurable lines.
	ChurnRate telemetry.Measure[float64]
}

// DefectOutcome is one PR's M2 traced defect outcome (quality/07, spec
// §5.2-5.3): whether this PR was traced as an INDUCING commit/PR for a later
// fix. Same "join input, not a shared artifact type" rationale as
// ChurnOutcome.
type DefectOutcome struct {
	Key telemetry.PRKey
	// DefectInducing is Measured(true) when the B-SZZ trace named this PR an
	// inducing commit, Measured(false) when the trace ran and cleared it,
	// and could-not-measure when no trace reached this PR at all (spec
	// §3.2 — never a silent "not a defect").
	DefectInducing telemetry.Measure[bool]
}

// JoinedPR is one PR's session-forensics join row: its telemetry alongside
// its M1 churn and M2 defect outcomes, each carrying its own three-state
// Measure independently — a row is never dropped for a partially-missing
// family, so a reader can tell "this PR had no traced defect outcome yet"
// apart from "this PR had no telemetry."
type JoinedPR struct {
	Key       telemetry.PRKey
	Telemetry telemetry.Measure[telemetry.TelemetryRecord]
	Churn     telemetry.Measure[float64]
	Defect    telemetry.Measure[bool]
}

// Joiner runs the session-forensics join against an injected
// telemetry.TelemetrySource (quality/13 Task item 4: "the forensics package
// compiles against the INTERFACE only"). house or other adapters are
// configuration passed to NewJoiner, never a fork of this package.
type Joiner struct {
	source telemetry.TelemetrySource
}

// NewJoiner builds a Joiner over source.
func NewJoiner(source telemetry.TelemetrySource) *Joiner {
	return &Joiner{source: source}
}

// Join pulls telemetry (via the injected source) for every PR appearing in
// EITHER churn or defects — the union of the two, not the intersection — and
// joins it to that PR's churn/defect outcome. A PR present in one family but
// not the other still gets a row, with the missing family reported
// could-not-measure rather than the PR being silently dropped from the
// corpus (spec §3.2).
func (j *Joiner) Join(churn []ChurnOutcome, defects []DefectOutcome) []JoinedPR {
	churnByKey := make(map[telemetry.PRKey]telemetry.Measure[float64], len(churn))
	for _, c := range churn {
		churnByKey[c.Key] = c.ChurnRate
	}
	defectByKey := make(map[telemetry.PRKey]telemetry.Measure[bool], len(defects))
	for _, d := range defects {
		defectByKey[d.Key] = d.DefectInducing
	}

	seen := make(map[telemetry.PRKey]bool, len(churn)+len(defects))
	keys := make([]telemetry.PRKey, 0, len(churn)+len(defects))
	for _, c := range churn {
		if !seen[c.Key] {
			seen[c.Key] = true
			keys = append(keys, c.Key)
		}
	}
	for _, d := range defects {
		if !seen[d.Key] {
			seen[d.Key] = true
			keys = append(keys, d.Key)
		}
	}

	rows := make([]JoinedPR, 0, len(keys))
	for _, k := range keys {
		row := JoinedPR{Key: k, Telemetry: j.source.Telemetry(k)}
		if cr, ok := churnByKey[k]; ok {
			row.Churn = cr
		} else {
			row.Churn = telemetry.CouldNotMeasure[float64]("no M1 churn outcome recorded for this PR")
		}
		if dr, ok := defectByKey[k]; ok {
			row.Defect = dr
		} else {
			row.Defect = telemetry.CouldNotMeasure[bool]("no M2 defect outcome recorded for this PR")
		}
		rows = append(rows, row)
	}
	return rows
}

// Band is one bucket of a per-behavior correlation (spec §7.3): the PRs
// whose harness-behavior count fell in this band, the average outcome
// across those with a MEASURED outcome, and the three-state coverage
// (measured-outcome PRs / band PRs) that must ship beside it — honest-claims
// discipline (spec §10) forbids reporting the average alone.
type Band struct {
	// Label is the behavior-count bucket ("0", "1-2", "3-5", "6+").
	Label string
	// PRCount is the number of PRs with MEASURED telemetry whose behavior
	// count fell in this band (regardless of whether their outcome is
	// itself measured).
	PRCount int
	// OutcomeAvg is the mean outcome value (churn rate, or defect-inducing
	// rate expressed as a 0/1 average) over this band's PRs that have a
	// measured outcome. could-not-measure when no PR in the band does.
	OutcomeAvg telemetry.Measure[float64]
	// Coverage is (PRs in this band with a measured outcome) / PRCount.
	// could-not-measure when PRCount itself is zero (nothing to cover).
	Coverage telemetry.Measure[float64]
}

// ForensicsReport is the full session-forensics output for one M1/M2
// corpus: the per-PR join rows plus the two named correlations quality/13's
// Task item 3 calls out — retries-band × churn-rate and refusal-count ×
// defect-inducing rate.
type ForensicsReport struct {
	Rows           []JoinedPR
	RetriesChurn   []Band
	RefusalsDefect []Band
}

// Forensics runs the join and both correlations in one call — the entry
// point quality/13 Task item 3 describes.
func (j *Joiner) Forensics(churn []ChurnOutcome, defects []DefectOutcome) ForensicsReport {
	rows := j.Join(churn, defects)
	return ForensicsReport{
		Rows:           rows,
		RetriesChurn:   bands(rows, retriesOf, churnOutcomeOf),
		RefusalsDefect: bands(rows, refusalsOf, defectOutcomeOf),
	}
}

// behaviorLabel buckets a non-negative harness-behavior count into the
// bands this package reports. Bucket boundaries are a documented, coarse
// starting point (not a tuned threshold) — quality/12's later ritual work is
// where band width gets revisited against a seasoned corpus.
func behaviorLabel(v int) string {
	switch {
	case v <= 0:
		return "0"
	case v <= 2:
		return "1-2"
	case v <= 5:
		return "3-5"
	default:
		return "6+"
	}
}

var bandOrder = []string{"0", "1-2", "3-5", "6+"}

// retriesOf and refusalsOf extract the two named harness behaviors from a
// join row. ok is false when the row's telemetry is not measured at all —
// such a row cannot be banded by ANY behavior and is excluded from every
// band's PRCount, not silently folded into band "0".
func retriesOf(r JoinedPR) (int, bool) {
	if r.Telemetry.State == telemetry.StateCouldNotMeasure {
		return 0, false
	}
	return r.Telemetry.Value.Retries, true
}

func refusalsOf(r JoinedPR) (int, bool) {
	if r.Telemetry.State == telemetry.StateCouldNotMeasure {
		return 0, false
	}
	return r.Telemetry.Value.Refusals, true
}

// churnOutcomeOf and defectOutcomeOf extract a band-averageable float64
// outcome from a row's M1/M2 measure. defectOutcomeOf maps the boolean
// defect-inducing flag to 0/1 so OutcomeAvg reads as the band's
// defect-inducing RATE, matching churn's own rate framing.
func churnOutcomeOf(r JoinedPR) (float64, bool) {
	switch r.Churn.State {
	case telemetry.StateMeasured, telemetry.StateMeasuredZero:
		return r.Churn.Value, true
	default:
		return 0, false
	}
}

func defectOutcomeOf(r JoinedPR) (float64, bool) {
	switch r.Defect.State {
	case telemetry.StateMeasured:
		if r.Defect.Value {
			return 1, true
		}
		return 0, true
	case telemetry.StateMeasuredZero:
		return 0, true
	default:
		return 0, false
	}
}

// bands computes one correlation's bands over rows, given the behavior and
// outcome extractors.
func bands(rows []JoinedPR, behaviorOf func(JoinedPR) (int, bool), outcomeOf func(JoinedPR) (float64, bool)) []Band {
	total := map[string]int{}
	withOutcome := map[string]int{}
	sum := map[string]float64{}

	for _, r := range rows {
		v, ok := behaviorOf(r)
		if !ok {
			continue
		}
		label := behaviorLabel(v)
		total[label]++
		if ov, ok := outcomeOf(r); ok {
			withOutcome[label]++
			sum[label] += ov
		}
	}

	out := make([]Band, 0, len(bandOrder))
	for _, label := range bandOrder {
		t := total[label]
		b := Band{Label: label, PRCount: t}
		if t == 0 {
			b.OutcomeAvg = telemetry.CouldNotMeasure[float64]("no PR with measured telemetry fell into this band")
			b.Coverage = telemetry.CouldNotMeasure[float64]("no PR with measured telemetry fell into this band")
			out = append(out, b)
			continue
		}
		wo := withOutcome[label]
		if wo == 0 {
			b.OutcomeAvg = telemetry.CouldNotMeasure[float64]("no PR in this band has a measured outcome")
			b.Coverage = telemetry.MeasuredZero[float64]()
		} else {
			b.OutcomeAvg = telemetry.Measured(sum[label] / float64(wo))
			b.Coverage = telemetry.Measured(float64(wo) / float64(t))
		}
		out = append(out, b)
	}
	return out
}
