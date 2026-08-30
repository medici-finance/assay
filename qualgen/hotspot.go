package main

import (
	"math"
	"sort"
	"time"
)

// DefaultHotspotHalfLifeDays is the default exponential-decay half-life for
// change-frequency (spec §4.3): a commit this many days old contributes half
// the weight of one landing today. Configurable per HotspotParams.
const DefaultHotspotHalfLifeDays = 90.0

// HotspotParams configures the hotspot computation (spec §4.3).
type HotspotParams struct {
	// HalfLifeDays is the exponential-decay half-life for change-frequency.
	// Zero (or negative) defaults to DefaultHotspotHalfLifeDays.
	HalfLifeDays float64
	// Now is the reference time decay is measured from. Zero defaults to
	// time.Now().UTC() at call time; tests inject a fixed value so decay math
	// stays deterministic.
	Now time.Time
}

// HotspotRecord is one per-file hotspot row appended to metrics.jsonl (spec
// §9.4, §4.3). Hotspot is the PRODUCT of ChangeFrequency and ComplexityProxy —
// the product predicts defects, not either factor alone (Verify #2).
type HotspotRecord struct {
	Metric          string           `json:"metric"` // "hotspot"
	Path            string           `json:"path"`
	ChangeFrequency Measure[float64] `json:"change_frequency"`
	ComplexityProxy Measure[float64] `json:"complexity_proxy"`
	Hotspot         Measure[float64] `json:"hotspot"`
	HalfLifeDays    float64          `json:"half_life_days"`
	MinedAt         time.Time        `json:"mined_at"`
}

// ComputeHotspots computes the per-file hotspot family (spec §4.3) from the
// mined commit and diff tables.
//
// allPaths is the full set of paths present in the mining window, INCLUDING
// files never touched by any diff (the measured-zero change-frequency case,
// spec §4.3/Verify #6). When nil, it is derived from the paths actually seen
// in diffs, which under-reports files that were never touched at all — a
// caller with access to the live repo tree (mine.go's wiring) SHOULD supply
// the tip tree's full path list instead.
func ComputeHotspots(commits []Commit, diffs []FileDiff, allPaths []string, params HotspotParams) []HotspotRecord {
	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	halfLife := params.HalfLifeDays
	if halfLife <= 0 {
		halfLife = DefaultHotspotHalfLifeDays
	}

	bySHA := make(map[string]Commit, len(commits))
	for _, c := range commits {
		bySHA[c.SHA] = c
	}

	touchTimes := map[string][]time.Time{}
	addLines := map[string][]string{}
	sawMeasured := map[string]bool{}
	couldNotReason := map[string]string{}
	sawAny := map[string]bool{}

	for _, fd := range diffs {
		c, ok := bySHA[fd.CommitSHA]
		if !ok {
			// The commit this diff belongs to is not in the table we were
			// handed (e.g. a partial/filtered read) — nothing to attribute a
			// touch time to, so skip rather than guess a date.
			continue
		}
		path := fd.NewPath
		if path == "" {
			path = fd.OldPath
		}
		if path == "" {
			continue
		}
		sawAny[path] = true
		touchTimes[path] = append(touchTimes[path], c.AuthorWhen)

		switch fd.Lines.State {
		case StateMeasured:
			sawMeasured[path] = true
			for _, hunk := range fd.Lines.Value {
				for _, lc := range hunk.Lines {
					if lc.Op == OpAdd {
						addLines[path] = append(addLines[path], lc.Content)
					}
				}
			}
		case StateCouldNotMeasure:
			if _, have := couldNotReason[path]; !have {
				couldNotReason[path] = fd.Lines.Reason
			}
		}
	}

	paths := allPaths
	if paths == nil {
		paths = make([]string, 0, len(sawAny))
		for p := range sawAny {
			paths = append(paths, p)
		}
		sort.Strings(paths)
	}

	out := make([]HotspotRecord, 0, len(paths))
	for _, path := range paths {
		freq := changeFrequency(touchTimes[path], now, halfLife)
		complexity := complexityForPath(addLines[path], sawMeasured[path], couldNotReason[path])
		hotspot := combineHotspot(freq, complexity)

		out = append(out, HotspotRecord{
			Metric:          "hotspot",
			Path:            path,
			ChangeFrequency: freq,
			ComplexityProxy: complexity,
			Hotspot:         hotspot,
			HalfLifeDays:    halfLife,
			MinedAt:         now,
		})
	}
	return out
}

// changeFrequency is the exponentially-decayed commit-touch count (spec
// §4.3): each touch contributes weight 2^(-ageDays/halfLifeDays), so recent
// touches weigh more. A path touched zero times in the window is a genuine
// measured-zero — the instrument ran and the honest answer is zero, never
// conflated with a value that could not be computed at all.
func changeFrequency(touchTimes []time.Time, now time.Time, halfLifeDays float64) Measure[float64] {
	if len(touchTimes) == 0 {
		return MeasuredZero[float64]()
	}
	total := 0.0
	for _, t := range touchTimes {
		ageDays := now.Sub(t).Hours() / 24
		if ageDays < 0 {
			ageDays = 0 // a touch "in the future" relative to now contributes full weight, never negative decay
		}
		total += math.Pow(2, -ageDays/halfLifeDays)
	}
	return Measured(total)
}

// complexityForPath decides the three-state complexity proxy for one file
// from what the diff table could recover about it:
//   - added-line content exists → the indentation proxy over those lines.
//   - a measured diff existed but contributed no added lines (pure deletions)
//     → could-not-measure: no content basis to assess nesting.
//   - only could-not-measure diffs were ever seen (binary/unreadable) →
//     could-not-measure, propagating the diff table's own reason.
//   - never touched at all → could-not-measure: the diff-only table carries
//     no snapshot of an untouched file's content.
func complexityForPath(addLines []string, sawMeasured bool, couldNotReason string) Measure[float64] {
	switch {
	case len(addLines) > 0:
		return indentationComplexity(addLines)
	case sawMeasured:
		return CouldNotMeasure[float64]("no added-line content recorded for this file to compute an indentation proxy from")
	case couldNotReason != "":
		return CouldNotMeasure[float64](couldNotReason)
	default:
		return CouldNotMeasure[float64]("file was never touched in the mined window; the diff-only table carries no content to derive an indentation proxy from")
	}
}

// combineHotspot is the product step (spec §4.3). Zero change-frequency
// floors the product at a genuine zero regardless of complexity (a file that
// never changed cannot be hot); anything else that cannot be measured
// propagates as could-not-measure rather than a silent number.
func combineHotspot(freq, complexity Measure[float64]) Measure[float64] {
	switch {
	case freq.State == StateCouldNotMeasure:
		return CouldNotMeasure[float64]("change-frequency could not be measured: " + freq.Reason)
	case freq.State == StateMeasuredZero:
		return MeasuredZero[float64]()
	case complexity.State != StateMeasured:
		return CouldNotMeasure[float64]("complexity proxy could not be measured: " + complexity.Reason)
	default:
		return Measured(freq.Value * complexity.Value)
	}
}

// indentationComplexity is the language-agnostic complexity proxy (spec
// §4.3): the average leading-whitespace depth of the given line contents, in
// 4-column "levels" (a tab counts as 4 columns). Deeper, more-nested code
// yields a strictly higher proxy on the same line count (Verify #3). It is
// deliberately cheap — the PRODUCT with change frequency is what predicts
// defects, not this proxy alone.
func indentationComplexity(lines []string) Measure[float64] {
	if len(lines) == 0 {
		return CouldNotMeasure[float64]("no line content available to compute an indentation proxy")
	}
	total := 0.0
	for _, l := range lines {
		total += leadingWhitespaceUnits(l) / 4.0
	}
	return Measured(total / float64(len(lines)))
}

// leadingWhitespaceUnits counts leading whitespace columns: a space is one
// column, a tab is four. It stops at the first non-whitespace rune (or the
// end of the string), so trailing/embedded whitespace never inflates it.
func leadingWhitespaceUnits(s string) float64 {
	units := 0.0
	for _, r := range s {
		switch r {
		case ' ':
			units++
		case '\t':
			units += 4
		default:
			return units
		}
	}
	return units
}
