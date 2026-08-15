package main

// trend.go — the 7-day comparison, and the one thing it must never do.
//
// A trend line over a heuristic is only meaningful while the heuristic holds still.
// So the trend block reads each prior day-file's classifierVersion and EXCLUDES any
// file written by a different one, reporting how many it excluded
// (prior_files_other_classifier). Without that, the day someone tightens a cue regex
// the trend shows a "behaviour change" that is entirely an artefact of the ruler.
// This is the whole reason ClassifierVersion exists; the trend is where it earns it.

import (
	"encoding/json"
	"os"
	"time"
)

// priorDay is the subset of a prior day-file this comparison needs.
type priorDay struct {
	ClassifierVersion string   `json:"classifierVersion"`
	RelayRatio        *float64 `json:"relay_ratio"`
	Intervention      struct {
		MessagesPerMergedPR *float64 `json:"messages_per_merged_pr"`
	} `json:"intervention"`
}

// BuildTrend reads up to window prior day-files under root and returns the deltas of
// today's headline numbers against the mean of the comparable ones.
//
// "Comparable" means same classifierVersion. A day-file that cannot be read or parsed
// is skipped silently — it is a prior artefact, not this run's input, and a
// half-written file from last week must not make today's collection fail. What is NOT
// silent is the count: prior_files_found says how much evidence the delta rests on,
// and a delta over one prior day is a very different claim from a delta over seven.
func BuildTrend(root, date string, window int, todayRelay, todayIntervention *float64) TrendBlock {
	tb := TrendBlock{Status: StatusNoPriorData, Reason: ReasonNoPriorDayFiles, WindowDays: window}
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return tb
	}

	var relays, interventions []float64
	for i := 1; i <= window; i++ {
		d := day.AddDate(0, 0, -i).Format("2006-01-02")
		raw, rerr := os.ReadFile(DayFilePath(root, d))
		if rerr != nil {
			continue
		}
		var p priorDay
		if jerr := json.Unmarshal(raw, &p); jerr != nil {
			continue
		}
		if p.ClassifierVersion != ClassifierVersion {
			tb.ClassifierMismatch++
			continue
		}
		tb.PriorFilesFound++
		if p.RelayRatio != nil {
			relays = append(relays, *p.RelayRatio)
		}
		if p.Intervention.MessagesPerMergedPR != nil {
			interventions = append(interventions, *p.Intervention.MessagesPerMergedPR)
		}
	}

	if tb.PriorFilesFound == 0 {
		return tb
	}
	tb.Status = StatusOK
	tb.Reason = ReasonNone
	tb.RelayRatio = delta(relays, todayRelay)
	tb.MessagesPerMergedPR = delta(interventions, todayIntervention)
	return tb
}

// delta returns the prior mean and today minus that mean. Both are null when there is
// nothing to compare — a missing comparison is reported as missing, not as zero
// movement, which would read as "steady".
func delta(prior []float64, today *float64) TrendDelta {
	if len(prior) == 0 {
		return TrendDelta{}
	}
	sum := 0.0
	for _, v := range prior {
		sum += v
	}
	mean := sum / float64(len(prior))
	out := TrendDelta{PriorMean: fptr(round4(mean))}
	if today != nil {
		out.Delta = fptr(round4(*today - mean))
	}
	return out
}
