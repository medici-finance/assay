package deskkit

// perfread.go — reading the local perf history back. LoadPerfDays is the
// only I/O; SummarisePerf, SummarisePerfV2 and SummariseBootSteps are PURE functions over a
// record slice, so every aggregate the deskperf verb prints is testable from a fixture
// directory alone.
//
// The history holds two schemas and the readers never merge them. perf-v1 records keep their
// original meaning (wall from the wrapper's own start; cpuMs a process-wide total) and are
// summarised exactly as before by SummarisePerf; perf-v2 records (wall and CPU over one
// declared boundary, counters with coverage) are summarised by SummarisePerfV2 as a separate
// group. Step records (verb "step:<name>", perf-v1) subdivide an invocation that already has
// its own record, so they appear only in SummariseBootSteps — never in an invocation total.
// No summariser adds a record's subprocessMs or phase times to a wall total: both are
// INCLUSIVE durations already inside that record's wall.

import (
	"bufio"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// PerfMaxDays is the longest window a reader may ask for: there is no eighth day to read.
const PerfMaxDays = perfRetentionDays

// PerfHistory is what LoadPerfDays found: the records, the unparseable lines it skipped, and
// how many day files the window held (0 = no history at all, as distinct from "files, but
// nothing in them").
type PerfHistory struct {
	Records  []PerfRecord
	Skipped  int
	DayFiles int
}

// LoadPerfDays reads the day files for the last `days` UTC days (clamped to 1..7), oldest
// first. It returns the records, the count of lines it could not parse as a perf-v1 or
// perf-v2 record (a truncated final line, a foreign schema, a perf-v2 line that fails
// validation), and an error ONLY for a genuinely unreadable
// directory — a missing directory is an empty history, not an error. It never creates
// anything.
func LoadPerfDays(days int) ([]PerfRecord, int, error) {
	h, err := LoadPerfHistory(days)
	return h.Records, h.Skipped, err
}

// LoadPerfHistory is LoadPerfDays plus the day-file count the empty-history refusal needs.
func LoadPerfHistory(days int) (PerfHistory, error) {
	var h PerfHistory
	dir, err := perfDir()
	if err != nil {
		return h, err
	}
	return loadPerfHistoryFrom(dir, days)
}

func loadPerfHistoryFrom(dir string, days int) (PerfHistory, error) {
	var h PerfHistory
	if days < 1 {
		days = 1
	}
	if days > PerfMaxDays {
		days = PerfMaxDays
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return h, nil
		}
		return h, err
	}
	now := perfNow().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	oldest := today.AddDate(0, 0, -(days - 1))
	var names []string
	for _, e := range entries {
		name := e.Name()
		if !e.Type().IsRegular() {
			continue
		}
		day, ok := perfDayOf(name)
		if !ok || day.Before(oldest) || day.After(today) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	h.DayFiles = len(names)
	for _, name := range names {
		recs, skipped, rerr := readPerfDayFile(filepath.Join(dir, name))
		if rerr != nil {
			// A day file that vanished between the listing and the open (a concurrent prune)
			// is not an unreadable directory; anything else is counted as skipped, never dropped.
			if !errors.Is(rerr, os.ErrNotExist) {
				h.Skipped++
			}
			continue
		}
		h.Records = append(h.Records, recs...)
		h.Skipped += skipped
	}
	return h, nil
}

func readPerfDayFile(path string) ([]PerfRecord, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	var recs []PerfRecord
	skipped := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r PerfRecord
		if json.Unmarshal([]byte(line), &r) != nil || (r.Schema != PerfSchema && r.Schema != PerfSchemaV2 && r.Schema != perfrec.SchemaV3 && r.Schema != perfrec.SchemaV4) {
			skipped++
			continue
		}
		recs = append(recs, r)
	}
	if sc.Err() != nil {
		skipped++
	}
	return recs, skipped, nil
}

// IsPerfStep reports whether a record is a boot- or launch-step record: a legacy (non-perf-v2)
// record with verb "step:<name>". perf-v2 records are invocations, never steps.
func IsPerfStep(r PerfRecord) bool {
	return r.Schema != PerfSchemaV2 && r.Schema != perfrec.SchemaV3 && r.Schema != perfrec.SchemaV4 && strings.HasPrefix(r.Verb, perfStepPfx)
}

// IsPerfLegacyInvocation reports whether a record belongs to the legacy perf-v1 invocation
// group SummarisePerf reads: anything that is neither perf-v2 nor a step. (LoadPerfDays only
// ever returns the three named schemas; a hand-built record without one reads as legacy.)
func IsPerfLegacyInvocation(r PerfRecord) bool {
	return r.Schema != PerfSchemaV2 && r.Schema != perfrec.SchemaV3 && r.Schema != perfrec.SchemaV4 && !IsPerfStep(r)
}

// IsPerfV2 reports whether a record belongs to the perf-v2 invocation group SummarisePerfV2
// reads.
func IsPerfV2(r PerfRecord) bool { return r.Schema == PerfSchemaV2 }

// FilterPerf keeps matching tools and legacy roles. A legacy role filter excludes v3,
// whose assigned role has different meaning and requires the workflow-aware ReduceV3 filter.
func FilterPerf(recs []PerfRecord, tool, role string) []PerfRecord {
	var out []PerfRecord
	for _, r := range recs {
		if tool != "" && r.Tool != tool {
			continue
		}
		if role != "" && (r.Schema == perfrec.SchemaV3 || r.Schema == perfrec.SchemaV4 || r.Role != role) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// PerfToolStats is one tool's row of `deskperf report`.
type PerfToolStats struct {
	Tool         string  `json:"tool"`
	Count        int     `json:"count"`
	P50Ms        int64   `json:"p50Ms"`
	P90Ms        int64   `json:"p90Ms"`
	MaxMs        int64   `json:"maxMs"`
	TotalMs      int64   `json:"totalMs"`
	SharePct     float64 `json:"sharePct"`
	Refusals     int     `json:"refusals"`
	RefusalRatio float64 `json:"refusalRatio"`
}

// PerfSlow is one of the top-N slowest invocations: ts / tool / verb / wallMs and nothing else.
type PerfSlow struct {
	TS     string `json:"ts"`
	Tool   string `json:"tool"`
	Verb   string `json:"verb"`
	WallMs int64  `json:"wallMs"`
}

// PerfSummary is the whole `deskperf report` aggregate.
type PerfSummary struct {
	Records int             `json:"records"`
	TotalMs int64           `json:"totalMs"`
	Tools   []PerfToolStats `json:"tools"`
	Slowest []PerfSlow      `json:"slowest"`
}

// SummarisePerf aggregates the LEGACY perf-v1 verb-level records — perf-v2 records and
// step records are excluded (a step subdivides a run already counted once; a perf-v2 wall is
// measured over a different boundary) — into per-tool p50/p90/max/total wall ms,
// each tool's share of total wall, its refusal ratio (refused + unwritten + unverifiable over
// count), and the top-N slowest invocations. Tools sort by total wall descending, then name.
func SummarisePerf(recs []PerfRecord, top int) PerfSummary {
	var s PerfSummary
	byTool := map[string][]PerfRecord{}
	for _, r := range recs {
		if !IsPerfLegacyInvocation(r) {
			continue
		}
		byTool[r.Tool] = append(byTool[r.Tool], r)
		s.Records++
		s.TotalMs += r.WallMs
	}
	var all []PerfRecord
	for tool, rs := range byTool {
		walls := make([]int64, len(rs))
		st := PerfToolStats{Tool: tool, Count: len(rs)}
		for i, r := range rs {
			walls[i] = r.WallMs
			st.TotalMs += r.WallMs
			switch r.Result {
			case ResultRefused, ResultUnwritten, ResultUnverifiable:
				st.Refusals++
			}
		}
		sort.Slice(walls, func(i, j int) bool { return walls[i] < walls[j] })
		st.P50Ms = nearestRank(walls, 50)
		st.P90Ms = nearestRank(walls, 90)
		st.MaxMs = walls[len(walls)-1]
		if s.TotalMs > 0 {
			st.SharePct = round1(100 * float64(st.TotalMs) / float64(s.TotalMs))
		}
		st.RefusalRatio = round3(float64(st.Refusals) / float64(st.Count))
		s.Tools = append(s.Tools, st)
		all = append(all, rs...)
	}
	sort.Slice(s.Tools, func(i, j int) bool {
		if s.Tools[i].TotalMs != s.Tools[j].TotalMs {
			return s.Tools[i].TotalMs > s.Tools[j].TotalMs
		}
		return s.Tools[i].Tool < s.Tools[j].Tool
	})
	s.Slowest = slowest(all, top)
	return s
}

// slowest is the top-N records by wall descending (ties: ts, then tool).
func slowest(all []PerfRecord, top int) []PerfSlow {
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].WallMs != all[j].WallMs {
			return all[i].WallMs > all[j].WallMs
		}
		if all[i].TS != all[j].TS {
			return all[i].TS < all[j].TS
		}
		return all[i].Tool < all[j].Tool
	})
	if top < 0 {
		top = 0
	}
	var out []PerfSlow
	for i := 0; i < len(all) && i < top; i++ {
		out = append(out, PerfSlow{TS: all[i].TS, Tool: all[i].Tool, Verb: all[i].Verb, WallMs: all[i].WallMs})
	}
	return out
}

// PerfV2ToolStats is one tool's row of the perf-v2 group. Wall figures are per-invocation
// walls over each record's own boundary; SumWallMs is their SUM — summed invocation time,
// which overlapping invocations can make exceed the elapsed time of the window. The CPU
// figures are a distribution over the CPUMeasured records only (CPUCount of them); a record
// whose CPU was unsupported, failed or missing contributes nothing, never a zero.
type PerfV2ToolStats struct {
	Tool         string  `json:"tool"`
	Count        int     `json:"count"`
	P50Ms        int64   `json:"p50Ms"`
	P90Ms        int64   `json:"p90Ms"`
	MaxMs        int64   `json:"maxMs"`
	SumWallMs    int64   `json:"sumWallMs"`
	SharePct     float64 `json:"sharePct"`
	Refusals     int     `json:"refusals"`
	RefusalRatio float64 `json:"refusalRatio"`
	CPUCount     int     `json:"cpuCount"`
	CPUP50Ms     int64   `json:"cpuP50Ms"`
	CPUP90Ms     int64   `json:"cpuP90Ms"`
	CPUMaxMs     int64   `json:"cpuMaxMs"`
}

// PerfCounterCoverage is one counter's coverage over the perf-v2 group: how many records
// measured it, how many recorded it unavailable, and the total over the MEASURED records only
// (an unavailable counter is unknown, not zero). subprocessMs is an inclusive child wall:
// its total is reported here and never added to a wall total.
type PerfCounterCoverage struct {
	Counter     string `json:"counter"`
	Measured    int    `json:"measured"`
	Unavailable int    `json:"unavailable"`
	Total       int64  `json:"total"`
}

// PerfPhaseStats is one tool phase's distribution over the perf-v2 records that timed it:
// Records is how many records carry the phase, N the phase instances across them, and the
// percentiles are over each record's ms for that phase. Phase time is inside the record's
// wall, so it is never summed with it.
type PerfPhaseStats struct {
	Tool    string `json:"tool"`
	Phase   string `json:"phase"`
	Records int    `json:"records"`
	N       int64  `json:"n"`
	P50Ms   int64  `json:"p50Ms"`
	P90Ms   int64  `json:"p90Ms"`
	MaxMs   int64  `json:"maxMs"`
}

// PerfV2Summary is the perf-v2 group of `deskperf report`.
type PerfV2Summary struct {
	Records      int                   `json:"records"`
	SumWallMs    int64                 `json:"sumWallMs"`
	Boundaries   map[string]int        `json:"boundaries"`
	CPU          map[string]int        `json:"cpu"`
	PhaseOverlap int                   `json:"phaseOverlap"`
	Tools        []PerfV2ToolStats     `json:"tools"`
	Counters     []PerfCounterCoverage `json:"counters"`
	Phases       []PerfPhaseStats      `json:"phases"`
	Slowest      []PerfSlow            `json:"slowest"`
}

// SummarisePerfV2 aggregates ONLY the perf-v2 records: per-tool wall distribution and summed
// invocation wall, the CPU distribution over measured records, coverage (boundary, CPU state,
// per-counter measured/unavailable, phase-overlap records), per-phase distributions and the
// top-N slowest. Counter totals cover measured records only; no inclusive duration
// (subprocessMs, a phase) is ever added to a wall figure. Tools sort by summed wall
// descending, then name; counters keep perfrec's roster order; phases sort by tool, phase.
func SummarisePerfV2(recs []PerfRecord, top int) PerfV2Summary {
	s := PerfV2Summary{Boundaries: map[string]int{}, CPU: map[string]int{}}
	byTool := map[string][]PerfRecord{}
	type phaseKey struct{ tool, phase string }
	phaseMs := map[phaseKey][]int64{}
	phaseN := map[phaseKey]int64{}
	measured := map[string]int{}
	unavailable := map[string]int{}
	totals := map[string]int64{}
	for _, r := range recs {
		if !IsPerfV2(r) {
			continue
		}
		s.Records++
		s.SumWallMs += r.WallMs
		s.Boundaries[r.Boundary]++
		s.CPU[r.CPU]++
		if r.PhaseOverlap {
			s.PhaseOverlap++
		}
		byTool[r.Tool] = append(byTool[r.Tool], r)
		for c, v := range r.Counters {
			measured[string(c)]++
			totals[string(c)] += v
		}
		for _, c := range r.Unavailable {
			unavailable[string(c)]++
		}
		for _, p := range r.Phases {
			k := phaseKey{r.Tool, p.Name}
			phaseMs[k] = append(phaseMs[k], p.Ms)
			phaseN[k] += p.N
		}
	}
	var all []PerfRecord
	for tool, rs := range byTool {
		walls := make([]int64, len(rs))
		var cpus []int64
		st := PerfV2ToolStats{Tool: tool, Count: len(rs)}
		for i, r := range rs {
			walls[i] = r.WallMs
			st.SumWallMs += r.WallMs
			if r.CPU == perfrec.CPUMeasured && r.CPUMs != nil {
				cpus = append(cpus, *r.CPUMs)
			}
			switch r.Result {
			case ResultRefused, ResultUnwritten, ResultUnverifiable:
				st.Refusals++
			}
		}
		sortInt64(walls)
		st.P50Ms, st.P90Ms, st.MaxMs = nearestRank(walls, 50), nearestRank(walls, 90), walls[len(walls)-1]
		if s.SumWallMs > 0 {
			st.SharePct = round1(100 * float64(st.SumWallMs) / float64(s.SumWallMs))
		}
		st.RefusalRatio = round3(float64(st.Refusals) / float64(st.Count))
		if st.CPUCount = len(cpus); st.CPUCount > 0 {
			sortInt64(cpus)
			st.CPUP50Ms, st.CPUP90Ms, st.CPUMaxMs = nearestRank(cpus, 50), nearestRank(cpus, 90), cpus[len(cpus)-1]
		}
		s.Tools = append(s.Tools, st)
		all = append(all, rs...)
	}
	sort.Slice(s.Tools, func(i, j int) bool {
		if s.Tools[i].SumWallMs != s.Tools[j].SumWallMs {
			return s.Tools[i].SumWallMs > s.Tools[j].SumWallMs
		}
		return s.Tools[i].Tool < s.Tools[j].Tool
	})
	if s.Records > 0 {
		for _, c := range perfrec.Counters() {
			name := string(c)
			s.Counters = append(s.Counters, PerfCounterCoverage{
				Counter: name, Measured: measured[name], Unavailable: unavailable[name], Total: totals[name],
			})
		}
	}
	for k, ms := range phaseMs {
		sortInt64(ms)
		s.Phases = append(s.Phases, PerfPhaseStats{
			Tool: k.tool, Phase: k.phase, Records: len(ms), N: phaseN[k],
			P50Ms: nearestRank(ms, 50), P90Ms: nearestRank(ms, 90), MaxMs: ms[len(ms)-1],
		})
	}
	sort.Slice(s.Phases, func(i, j int) bool {
		if s.Phases[i].Tool != s.Phases[j].Tool {
			return s.Phases[i].Tool < s.Phases[j].Tool
		}
		return s.Phases[i].Phase < s.Phases[j].Phase
	})
	s.Slowest = slowest(all, top)
	return s
}

func sortInt64(xs []int64) { sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] }) }

// PerfStepStats is one boot step's row within a role.
type PerfStepStats struct {
	Step  string `json:"step"`
	Count int    `json:"count"`
	P50Ms int64  `json:"p50Ms"`
	P90Ms int64  `json:"p90Ms"`
	MaxMs int64  `json:"maxMs"`
}

// PerfBootRole is one role's boot breakdown: per-step stats and the summed boot cost (the sum
// of every step record's wall time for that role).
type PerfBootRole struct {
	Role    string          `json:"role"`
	Steps   []PerfStepStats `json:"steps"`
	TotalMs int64           `json:"totalMs"`
}

// PerfBootSummary is the whole `deskperf boot` aggregate.
type PerfBootSummary struct {
	Records int            `json:"records"`
	Roles   []PerfBootRole `json:"roles"`
}

// SummariseBootSteps aggregates ONLY the perf-v1 step:* records, per role then per step. Steps keep
// the boot's own order (the PerfBootSteps roster, then the PerfLaunchSteps roster), unknown steps last by name; roles sort by
// name.
func SummariseBootSteps(recs []PerfRecord) PerfBootSummary {
	var s PerfBootSummary
	byRole := map[string]map[string][]int64{}
	totals := map[string]int64{}
	for _, r := range recs {
		if !IsPerfStep(r) {
			continue
		}
		s.Records++
		step := strings.TrimPrefix(r.Verb, perfStepPfx)
		if byRole[r.Role] == nil {
			byRole[r.Role] = map[string][]int64{}
		}
		byRole[r.Role][step] = append(byRole[r.Role][step], r.WallMs)
		totals[r.Role] += r.WallMs
	}
	order := map[string]int{}
	known := append(append([]string(nil), perfBootSteps...), perfLaunchSteps...)
	for i, st := range known {
		order[st] = i
	}
	rank := func(step string) int {
		if i, ok := order[step]; ok {
			return i
		}
		return len(known)
	}
	for role, steps := range byRole {
		br := PerfBootRole{Role: role, TotalMs: totals[role]}
		for step, walls := range steps {
			sort.Slice(walls, func(i, j int) bool { return walls[i] < walls[j] })
			br.Steps = append(br.Steps, PerfStepStats{
				Step: step, Count: len(walls),
				P50Ms: nearestRank(walls, 50), P90Ms: nearestRank(walls, 90), MaxMs: walls[len(walls)-1],
			})
		}
		sort.Slice(br.Steps, func(i, j int) bool {
			ri, rj := rank(br.Steps[i].Step), rank(br.Steps[j].Step)
			if ri != rj {
				return ri < rj
			}
			return br.Steps[i].Step < br.Steps[j].Step
		})
		s.Roles = append(s.Roles, br)
	}
	sort.Slice(s.Roles, func(i, j int) bool { return s.Roles[i].Role < s.Roles[j].Role })
	return s
}

// nearestRank is NEAREST-RANK on an ascending slice: P_k is element ceil(k/100 * n) — the
// method cmd/opmetrics' latency percentiles use, chosen for the same reason: it always names
// a real observation rather than interpolating one no invocation ever had.
func nearestRank(sorted []int64, k float64) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(k / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }
func round3(f float64) float64 { return math.Round(f*1000) / 1000 }
