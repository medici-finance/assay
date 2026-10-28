package deskkit

// perfforge.go — the forge-call view of the perf history: how many forge HTTP attempts each
// tool and each release build made, and what that costs against the App rate-limit budget.
// Like the other summarisers it is a PURE function over a record slice.
//
// A forge count is read from perf-v1 `forgeCalls` and from the perf-v2 `http` counter. Both
// count attempts through the same counting transport with the same lower-bound meaning
// (docs/telemetry.md), so unlike wall time they are added together: the budget is spent by a
// call whichever schema recorded it. A perf-v2 record whose `http` counter is UNAVAILABLE was
// not observed and is never read as zero; nor is the perf-v1 0 of a tool documented as never
// counting (perfForgeUncounted), nor a negative count, which no counter can produce.
// Step records sit inside an invocation that has its own record, and perf-v3 records are
// reported in their own section, so neither is read here.
//
// The perf-v1 and perf-v2 readers do not re-check a record's tool or build stamp, so this view
// keys its rows on perfrec.Label and perfrec.SourceSHA, as the perf-v3 reader does: a value
// without a roster-shaped label's form (a shape check, not roster membership) or a hex stamp's
// groups under "other" or "unpinned", so no path, URL or control byte reaches this section's
// rows. Every sum saturates at the int64 maximum rather than wrapping.

import (
	"math"
	"sort"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// PerfForgeBudgetPerHour is the floor of a GitHub App installation's REST rate limit, in
// requests per hour; a larger installation is granted more. A forge count is HTTP attempts —
// REST, GraphQL (which spends its own points budget), conditional 304s and other forges alike —
// and the busiest hour pools every tool, whichever installation it ran as. So a percentage
// against this floor is a rough gauge of pressure, not any one installation's measured draw.
const PerfForgeBudgetPerHour = 5000

// PerfForgeStats is one row of the forge section: a tool, or a release build stamp. Calls and
// every per-run and peak figure are over the Measured records only; Unavailable records were
// not observed and add nothing. PeakHourCalls is the most calls this row made in one UTC clock
// hour (PeakHour, the earliest on a tie); the two budget percentages divide the peak hour and
// the p90 run by PerfForgeBudgetPerHour.
type PerfForgeStats struct {
	Key               string  `json:"key"`
	Runs              int     `json:"runs"`
	Measured          int     `json:"measured"`
	Unavailable       int     `json:"unavailable"`
	Calls             int64   `json:"calls"`
	P50PerRun         int64   `json:"p50PerRun"`
	P90PerRun         int64   `json:"p90PerRun"`
	MaxPerRun         int64   `json:"maxPerRun"`
	SharePct          float64 `json:"sharePct"`
	PeakHour          string  `json:"peakHour,omitempty"`
	PeakHourCalls     int64   `json:"peakHourCalls"`
	PeakHourBudgetPct float64 `json:"peakHourBudgetPct"`
	P90RunBudgetPct   float64 `json:"p90RunBudgetPct"`
}

// PerfForgeSummary is the forge section of `deskperf report`. Runs counts the perf-v1 and
// perf-v2 invocation records read; FromV1 and FromV2 split the measured ones by source;
// Unhoured counts measured records whose timestamp did not parse (in every total, but in no
// hour). Tools sort by calls descending, then name; Releases the same, by build stamp.
type PerfForgeSummary struct {
	BudgetPerHour     int              `json:"budgetPerHour"`
	Runs              int              `json:"runs"`
	Measured          int              `json:"measured"`
	Unavailable       int              `json:"unavailable"`
	FromV1            int              `json:"fromV1"`
	FromV2            int              `json:"fromV2"`
	Calls             int64            `json:"calls"`
	PeakHour          string           `json:"peakHour,omitempty"`
	PeakHourCalls     int64            `json:"peakHourCalls"`
	PeakHourBudgetPct float64          `json:"peakHourBudgetPct"`
	Unhoured          int              `json:"unhoured"`
	Tools             []PerfForgeStats `json:"tools"`
	Releases          []PerfForgeStats `json:"releases"`
}

// perfForgeHour is the UTC clock-hour bucket a peak is reported in.
const perfForgeHour = "2006-01-02T15:00Z"

// perfForgeUncounted names the tools whose perf-v1 record writes `forgeCalls` 0 as "not
// counted" rather than "none made" (docs/telemetry.md): perf-v1 has no unavailable state, so
// this view supplies it rather than read that 0 as a measured zero.
var perfForgeUncounted = map[string]bool{"statusgen": true}

// forgeCount reads one record's forge count. ok is false for a record this view does not
// read (a step, perf-v3, an unknown schema); measured is false for a perf-v2 record whose
// `http` counter is unavailable, for a perf-v1 record from a tool that never counts, and for a
// negative count of either schema.
func forgeCount(r PerfRecord) (calls int64, measured, ok bool) {
	switch {
	case IsPerfLegacyInvocation(r) && r.Schema == PerfSchema:
		if perfForgeUncounted[r.Tool] || r.ForgeCalls < 0 {
			return 0, false, true
		}
		return r.ForgeCalls, true, true
	case IsPerfV2(r):
		v, has := r.Counters[perfrec.CounterHTTP]
		return v, has && v >= 0, true
	}
	return 0, false, false
}

// SummarisePerfForge aggregates forge calls per tool and per release build (sourceSHA) over
// the perf-v1 and perf-v2 invocation records: total calls, the calls-per-run distribution
// (nearest rank), each row's share of all calls, and the rate-limit cost — the busiest UTC
// hour and the p90 run as a share of PerfForgeBudgetPerHour. Runs == 0 means the window held
// no record this view reads.
func SummarisePerfForge(recs []PerfRecord) PerfForgeSummary {
	s := PerfForgeSummary{BudgetPerHour: PerfForgeBudgetPerHour}
	type obs struct {
		calls    int64
		measured bool
		hour     string
	}
	byTool := map[string][]obs{}
	byRelease := map[string][]obs{}
	hours := map[string]int64{}
	for _, r := range recs {
		calls, measured, ok := forgeCount(r)
		if !ok {
			continue
		}
		s.Runs++
		o := obs{calls: calls, measured: measured}
		if measured {
			s.Measured++
			s.Calls = satAdd(s.Calls, calls)
			if r.Schema == PerfSchemaV2 {
				s.FromV2++
			} else {
				s.FromV1++
			}
			if t, err := time.Parse(time.RFC3339, r.TS); err == nil {
				o.hour = t.UTC().Format(perfForgeHour)
				hours[o.hour] = satAdd(hours[o.hour], calls)
			} else {
				s.Unhoured++
			}
		} else {
			s.Unavailable++
		}
		tool, release := perfrec.Label(r.Tool), perfrec.SourceSHA(r.SourceSHA)
		byTool[tool] = append(byTool[tool], o)
		byRelease[release] = append(byRelease[release], o)
	}
	s.PeakHour, s.PeakHourCalls = peakHour(hours)
	s.PeakHourBudgetPct = budgetPct(s.PeakHourCalls)
	rows := func(groups map[string][]obs) []PerfForgeStats {
		var out []PerfForgeStats
		for key, os := range groups {
			st := PerfForgeStats{Key: key, Runs: len(os)}
			var per []int64
			hrs := map[string]int64{}
			for _, o := range os {
				if !o.measured {
					st.Unavailable++
					continue
				}
				st.Measured++
				st.Calls = satAdd(st.Calls, o.calls)
				per = append(per, o.calls)
				if o.hour != "" {
					hrs[o.hour] = satAdd(hrs[o.hour], o.calls)
				}
			}
			if len(per) > 0 {
				sortInt64(per)
				st.P50PerRun, st.P90PerRun, st.MaxPerRun = nearestRank(per, 50), nearestRank(per, 90), per[len(per)-1]
				st.P90RunBudgetPct = budgetPct(st.P90PerRun)
			}
			if s.Calls > 0 {
				st.SharePct = round1(100 * float64(st.Calls) / float64(s.Calls))
			}
			st.PeakHour, st.PeakHourCalls = peakHour(hrs)
			st.PeakHourBudgetPct = budgetPct(st.PeakHourCalls)
			out = append(out, st)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Calls != out[j].Calls {
				return out[i].Calls > out[j].Calls
			}
			return out[i].Key < out[j].Key
		})
		return out
	}
	s.Tools, s.Releases = rows(byTool), rows(byRelease)
	return s
}

// peakHour is the hour with the most calls, the earliest on a tie; "" when there is none.
func peakHour(hours map[string]int64) (string, int64) {
	var best string
	var most int64
	for h, n := range hours {
		if best == "" || n > most || (n == most && h < best) {
			best, most = h, n
		}
	}
	return best, most
}

// satAdd adds two non-negative counts, holding at the int64 maximum instead of wrapping.
func satAdd(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}

func budgetPct(calls int64) float64 {
	return round2(100 * float64(calls) / PerfForgeBudgetPerHour)
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
