// aggregate.go — the reducer's MATH: per-arm means (over the runs that carried
// each metric) and the with-vs-without delta.
//
// The three-state contract survives every step. An arm's metric is measured
// only when at least one run carried it; otherwise it is could-not-check with
// its count of contributing runs still shown. A delta exists only when BOTH
// arms measured the metric — a difference taken across a metric one arm never
// measured would be movement in the instrument, not in the world.
package main

import "fmt"

// armMetric is one arm's aggregate of one metric.
type armMetric struct {
	State cellState
	Mean  float64 // valid only when State == cellMeasured
	N     int     // runs that contributed a measured value
	Total int     // total runs in the arm
	Note  string  // why, when could-not-check
}

// deltaMetric is the with-minus-without comparison of one metric.
type deltaMetric struct {
	State    cellState // measured only when both arms measured the metric
	Abs      float64   // with.Mean - without.Mean
	PctState cellState // measured only when the baseline (without) mean is nonzero
	Pct      float64   // relative change vs the without-overlay baseline, %
	Note     string
}

// aggregateArm reduces one arm's runs into a per-metric aggregate.
func aggregateArm(a *arm) map[string]armMetric {
	res := map[string]armMetric{}
	for _, meta := range metricOrder {
		var sum float64
		var n int
		for _, r := range a.Runs {
			if m := r.Metrics[meta.Key]; m.State == cellMeasured {
				sum += m.Value
				n++
			}
		}
		am := armMetric{N: n, Total: len(a.Runs)}
		if n == 0 {
			am.State = cellCouldNotCheck
			switch {
			case !a.Present:
				am.Note = a.Note
			case len(a.Runs) == 0:
				am.Note = "arm has no runs"
			default:
				am.Note = "no run in this arm carried this metric"
			}
		} else {
			am.State = cellMeasured
			am.Mean = sum / float64(n)
		}
		res[meta.Key] = am
	}
	return res
}

// computeDelta compares the two arms' aggregates of one metric.
func computeDelta(with, without armMetric) deltaMetric {
	if with.State != cellMeasured || without.State != cellMeasured {
		return deltaMetric{
			State:    cellCouldNotCheck,
			PctState: cellCouldNotCheck,
			Note:     "a delta needs a measured mean in BOTH arms",
		}
	}
	d := deltaMetric{State: cellMeasured, Abs: with.Mean - without.Mean}
	if without.Mean != 0 {
		d.PctState = cellMeasured
		d.Pct = (with.Mean - without.Mean) / without.Mean * 100
	} else {
		d.PctState = cellCouldNotCheck
	}
	return d
}

// metricReport is one metric's line of the report: both arms and their delta.
type metricReport struct {
	Meta    metricMeta
	With    armMetric
	Without armMetric
	Delta   deltaMetric
}

// report is the whole reduced result the renderer turns into markdown.
type report struct {
	OverlaySlug string
	Date        string
	ArmRuns     map[string]int
	ArmPresent  map[string]bool
	ArmNote     map[string]string
	Metrics     []metricReport

	// Safety floor: did the task-check pass rate hold with the overlay on?
	SafetyState cellState
	SafetyNote  string
}

// reduce turns the loaded arms into the report.
func reduce(arms map[string]*arm, slug, date string) report {
	with := aggregateArm(arms[armWith])
	without := aggregateArm(arms[armWithout])

	rep := report{
		OverlaySlug: slug,
		Date:        date,
		ArmRuns:     map[string]int{},
		ArmPresent:  map[string]bool{},
		ArmNote:     map[string]string{},
	}
	for _, name := range armOrder {
		rep.ArmRuns[name] = len(arms[name].Runs)
		rep.ArmPresent[name] = arms[name].Present
		rep.ArmNote[name] = arms[name].Note
	}
	for _, meta := range metricOrder {
		rep.Metrics = append(rep.Metrics, metricReport{
			Meta:    meta,
			With:    with[meta.Key],
			Without: without[meta.Key],
			Delta:   computeDelta(with[meta.Key], without[meta.Key]),
		})
	}

	// Verdict input: the safety floor is the task-check pass rate. It is a
	// stated observation, not an adoption decision — that belongs to 02.
	wc, oc := with[mCheck], without[mCheck]
	if wc.State == cellMeasured && oc.State == cellMeasured {
		rep.SafetyState = cellMeasured
		if wc.Mean+1e-9 >= oc.Mean {
			rep.SafetyNote = fmt.Sprintf(
				"held — with-overlay pass rate %.0f%% >= without-overlay %.0f%%",
				wc.Mean*100, oc.Mean*100)
		} else {
			rep.SafetyNote = fmt.Sprintf(
				"NOT held — with-overlay pass rate %.0f%% < without-overlay %.0f%%",
				wc.Mean*100, oc.Mean*100)
		}
	} else {
		rep.SafetyState = cellCouldNotCheck
		rep.SafetyNote = "could-not-check — a task-check pass rate is not measured in both arms"
	}

	return rep
}
