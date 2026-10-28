package deskkit

// perfday.go — the per-day timeseries view of the perf history (`deskperf report --by-day`)
// and the per-tool perf-v3 request rollup: PURE functions over a record slice, like the
// other summarisers in this package.
//
// A record's day is its timestamp's UTC calendar day; a record whose timestamp does not
// parse is counted (Undated) and never assigned one. Wall figures keep the report's schema
// separation — perf-v1, perf-v2 and perf-v3 walls are measured over different boundaries,
// so a day row carries one distribution per schema group and never mixes them — while the
// forge count pools perf-v1 forgeCalls and the perf-v2/perf-v3 http counter as ONE budget:
// SummarisePerfForge's pooling, extended to perf-v3 records here (a forge call spends the
// same budget whichever schema recorded it; an unavailable counter stays not-observed,
// never zero).
//
// Every perf-v3 figure — the row count, the perf-v3 wall, the forge count and the request
// rollup — is taken from ONE set: perfrec.SelectV3 over the WHOLE window (identity dedup
// before filters, conflicting identities excluded, the workflow filter applied), the set
// perfrec.ReduceV3 aggregates. Only then are the kept records bucketed by tool — or by
// (day, tool) — and each bucket's requests merged by ReduceV3. Selecting before bucketing
// means a replayed record is counted once and a conflict whose copies fall on different
// days is still excluded. A workflow filter matches perf-v3 attribution only; perf-v1 and
// perf-v2 records carry none, so a filtered view leaves them out (counted as Unattributed)
// rather than add them to the filtered rows. Request-latency p50/p90 are ESTIMATES from the merged duration
// histogram (perfrec.EstimateDurationPercentile), never observed durations; rate-limit
// points are summed only where their coverage is measured.

import (
	"sort"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// perfDayLayout is the UTC calendar day a by-day row is keyed on.
const perfDayLayout = "2006-01-02"

// PerfDayWall is one schema group's invocation-wall distribution inside a day row: a
// nearest-rank percentile set over that group's walls only (groups never mix: perf-v1,
// perf-v2 and perf-v3 measure over different boundaries). SumMs is summed invocation wall,
// which overlapping invocations can make exceed the day's elapsed time.
type PerfDayWall struct {
	Records int   `json:"records"`
	P50Ms   int64 `json:"p50Ms"`
	P90Ms   int64 `json:"p90Ms"`
	MaxMs   int64 `json:"maxMs"`
	SumMs   int64 `json:"sumMs"`
}

// PerfRequestRollup is one merged perf-v3 request class row, inside a day row or the pooled
// per-tool rollup. P50Ms/P90Ms are estimated from the merged durationBins histogram and are
// absent when no duration was measured; Points is the summed rate-limit charge, present
// only where its coverage is measured (an estimated or unavailable charge is never read as
// a measured sum).
type PerfRequestRollup struct {
	Kind             string `json:"observationKind"`
	Class            string `json:"class"`
	Origin           string `json:"origin"`
	Protocol         string `json:"protocol"`
	Resource         string `json:"resource"`
	Outcome          string `json:"outcome"`
	Attempts         int64  `json:"attempts"`
	DurationMeasured int64  `json:"durationMeasured"`
	DurationSumMs    int64  `json:"durationSumMs"`
	MaxMs            int64  `json:"maxMs"`
	P50Ms            *int64 `json:"p50Ms,omitempty"`
	P90Ms            *int64 `json:"p90Ms,omitempty"`
	Points           *int64 `json:"points,omitempty"`
	PointsUnit       string `json:"pointsUnit,omitempty"`
	PointsCoverage   string `json:"pointsCoverage"`
	PointSamples     int64  `json:"pointSamples"`
}

// PerfDayRow is one tool × UTC day row of `deskperf report --by-day`: invocation count
// across every schema, one wall distribution per schema group present, the pooled forge
// count, and the day's perf-v3 request rollup for the tool.
type PerfDayRow struct {
	Day              string              `json:"day"`
	Tool             string              `json:"tool"`
	Count            int                 `json:"count"`
	PerfV1           *PerfDayWall        `json:"perfV1,omitempty"`
	PerfV2           *PerfDayWall        `json:"perfV2,omitempty"`
	PerfV3           *PerfDayWall        `json:"perfV3,omitempty"`
	ForgeCalls       int64               `json:"forgeCalls"`
	ForgeMeasured    int                 `json:"forgeMeasured"`
	ForgeUnavailable int                 `json:"forgeUnavailable"`
	Requests         []PerfRequestRollup `json:"requests,omitempty"`
}

// PerfDaySummary is the whole `deskperf report --by-day` aggregate. Records counts the
// invocation records the rows read (perf-v1/v2, and the perf-v3 records perfrec.SelectV3
// keeps; never steps); Undated the ones whose timestamp did not parse (counted, never
// day-assigned). V3Records is the kept perf-v3 count; V3Duplicates, V3Conflicts and
// V3Invalid are the window-wide selection's exclusions. Unattributed counts the perf-v1/v2
// records a workflow filter left out (they carry no attribution to match). Rows sort by day,
// then tool.
type PerfDaySummary struct {
	Records      int          `json:"records"`
	Undated      int          `json:"undated"`
	Unattributed int          `json:"unattributed"`
	V3Records    int          `json:"v3Records"`
	V3Duplicates int          `json:"v3Duplicates"`
	V3Conflicts  int          `json:"v3Conflicts"`
	V3Invalid    int          `json:"v3Invalid"`
	Rows         []PerfDayRow `json:"rows"`
}

// PerfToolRequests is one tool's pooled perf-v3 request rollup: the v3 records that
// contributed and the merged request rows.
type PerfToolRequests struct {
	Tool     string              `json:"tool"`
	Records  int                 `json:"records"`
	Requests []PerfRequestRollup `json:"requests"`
}

// forgeCountDay reads one record's forge count for the by-day view: forgeCount's pooling,
// extended to perf-v3 records, whose http counter spends the same budget.
func forgeCountDay(r PerfRecord) (calls int64, measured, ok bool) {
	if r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 {
		v, has := r.Counters[perfrec.CounterHTTP]
		return v, has && v >= 0, true
	}
	return forgeCount(r)
}

// requestRollup renders ReduceV3's merged request rows, adding the histogram percentile
// estimates and keeping the measured-points sum apart from every other coverage.
func requestRollup(rows []perfrec.V3RequestStats) []PerfRequestRollup {
	var out []PerfRequestRollup
	for _, x := range rows {
		r := PerfRequestRollup{
			Kind:             string(x.Kind),
			Class:            string(x.Class),
			Origin:           string(x.Origin),
			Protocol:         x.Protocol,
			Resource:         x.Resource,
			Outcome:          x.Outcome,
			Attempts:         x.Attempts,
			DurationMeasured: x.DurationCoverage[perfrec.CoverageMeasured],
			DurationSumMs:    x.DurationSumMs,
			MaxMs:            x.DurationMaxMs,
			PointsCoverage:   string(x.Points.Coverage),
			PointSamples:     x.PointSamples,
		}
		if p, ok := perfrec.EstimateDurationPercentile(x.DurationBins, 50); ok {
			r.P50Ms = &p
		}
		if p, ok := perfrec.EstimateDurationPercentile(x.DurationBins, 90); ok {
			r.P90Ms = &p
		}
		if x.Points.Coverage == perfrec.CoverageMeasured && x.Points.Value != nil {
			v := *x.Points.Value
			r.Points = &v
			r.PointsUnit = string(x.Points.Unit)
		}
		out = append(out, r)
	}
	return out
}

// SummarisePerfDays aggregates invocation records into one row per tool × UTC day. Step
// records subdivide an invocation that has its own record and are never read. The perf-v3
// records are first selected window-wide by perfrec.SelectV3 under the report's permission
// and workflow filter, and every per-day figure is then derived from that selection, so a
// workflow filter means here exactly what it means in the pooled report.
func SummarisePerfDays(recs []PerfRecord, permission perfrec.Profile, filter perfrec.V3Filter) (PerfDaySummary, error) {
	var s PerfDaySummary
	kept, sel, err := perfrec.SelectV3(recs, permission, filter)
	if err != nil {
		return s, err
	}
	s.V3Records = len(kept)
	s.V3Duplicates, s.V3Conflicts, s.V3Invalid = sel.Duplicates, sel.Conflicts, sel.Invalid
	selected := append([]PerfRecord(nil), kept...)
	for _, r := range recs {
		if IsPerfStep(r) || r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 {
			continue
		}
		if filter.Used() {
			s.Unattributed++
			continue
		}
		selected = append(selected, r)
	}
	type key struct{ day, tool string }
	buckets := map[key][]PerfRecord{}
	for _, r := range selected {
		s.Records++
		t, err := time.Parse(time.RFC3339, r.TS)
		if err != nil {
			s.Undated++
			continue
		}
		k := key{t.UTC().Format(perfDayLayout), r.Tool}
		buckets[k] = append(buckets[k], r)
	}
	keys := make([]key, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].day != keys[j].day {
			return keys[i].day < keys[j].day
		}
		return keys[i].tool < keys[j].tool
	})
	walls := func(rs []PerfRecord, keep func(PerfRecord) bool) *PerfDayWall {
		var xs []int64
		for _, r := range rs {
			if keep(r) {
				xs = append(xs, r.WallMs)
			}
		}
		if len(xs) == 0 {
			return nil
		}
		sortInt64(xs)
		var sum int64
		for _, x := range xs {
			sum = satAdd(sum, x)
		}
		return &PerfDayWall{Records: len(xs), P50Ms: nearestRank(xs, 50), P90Ms: nearestRank(xs, 90), MaxMs: xs[len(xs)-1], SumMs: sum}
	}
	for _, k := range keys {
		rs := buckets[k]
		row := PerfDayRow{Day: k.day, Tool: k.tool, Count: len(rs)}
		row.PerfV1 = walls(rs, IsPerfLegacyInvocation)
		row.PerfV2 = walls(rs, IsPerfV2)
		row.PerfV3 = walls(rs, func(r PerfRecord) bool { return r.Schema == perfrec.SchemaV3 })
		for _, r := range rs {
			calls, measured, ok := forgeCountDay(r)
			if !ok {
				continue
			}
			if measured {
				row.ForgeMeasured++
				row.ForgeCalls = satAdd(row.ForgeCalls, calls)
			} else {
				row.ForgeUnavailable++
			}
		}
		// rs holds only selected perf-v3 records, so this reduce re-applies a selection they
		// already pass: it merges the requests and excludes nothing.
		v3, err := perfrec.ReduceV3(rs, permission, "tool", filter)
		if err != nil {
			return s, err
		}
		row.Requests = requestRollup(v3.Requests)
		s.Rows = append(s.Rows, row)
	}
	return s, nil
}

// SummarisePerfV3RequestsByTool is the pooled per-tool perf-v3 request rollup of
// `deskperf report`: the perf-v3 records perfrec.SelectV3 keeps over the whole window
// (under the report's permission and workflow filter), bucketed by tool and merged by
// perfrec.ReduceV3. Tools sort by name; a tool whose records carry no request rows (or none
// the filter keeps) is absent.
func SummarisePerfV3RequestsByTool(recs []PerfRecord, permission perfrec.Profile, filter perfrec.V3Filter) ([]PerfToolRequests, error) {
	kept, _, err := perfrec.SelectV3(recs, permission, filter)
	if err != nil {
		return nil, err
	}
	byTool := map[string][]PerfRecord{}
	for _, r := range kept {
		byTool[r.Tool] = append(byTool[r.Tool], r)
	}
	tools := make([]string, 0, len(byTool))
	for tool := range byTool {
		tools = append(tools, tool)
	}
	sort.Strings(tools)
	var out []PerfToolRequests
	for _, tool := range tools {
		v3, err := perfrec.ReduceV3(byTool[tool], permission, "tool", filter)
		if err != nil {
			return nil, err
		}
		rows := requestRollup(v3.Requests)
		if len(rows) == 0 {
			continue
		}
		out = append(out, PerfToolRequests{Tool: tool, Records: v3.Records, Requests: rows})
	}
	return out, nil
}
