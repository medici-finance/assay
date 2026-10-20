package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mixedRecords is one day of history holding BOTH schemas and both kinds of step: legacy
// perf-v1 invocations, perf-v1 deskboot boot steps and cellctl launch steps (each inside an
// invocation that has its own record), and perf-v2 invocations whose subprocessMs and phase
// times are INCLUSIVE — already inside their own wall. Each correct figure is hand-checked
// below; each naive sum over the same lines is a different number.
func mixedRecords(t *testing.T) []perfrec.Record {
	t.Helper()
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	ms := func(v int64) *int64 { return &v }
	v1 := func(tool, verb, role string, wall int64, cpu *int64, exit int) perfrec.Record {
		return perfrec.Record{Schema: perfrec.Schema, TS: ts, Tool: tool, Verb: verb, Role: role, WallMs: wall,
			CPUMs: cpu, ChildProcs: 2, ChildWallMs: 700, ForgeCalls: 3, Exit: exit, Result: perfrec.ResultForExit(exit),
			Install: strings.Repeat("a", 32), SourceSHA: "unpinned"}
	}
	v2 := func(tool, verb, role, boundary string, wall int64, cpu string, cpuMs *int64, exit int,
		counters map[perfrec.Counter]int64, phases []perfrec.PhaseStat, overlap bool) perfrec.Record {
		var un []perfrec.Counter
		for _, c := range perfrec.Counters() {
			if _, ok := counters[c]; !ok {
				un = append(un, c)
			}
		}
		return perfrec.Record{Schema: perfrec.SchemaV2, TS: ts, Tool: tool, Verb: verb, Role: role, Boundary: boundary,
			WallMs: wall, CPU: cpu, CPUMs: cpuMs, Exit: exit, Result: perfrec.ResultForExit(exit),
			Install: strings.Repeat("b", 32), SourceSHA: "unpinned",
			Counters: counters, Unavailable: un, Phases: phases, PhaseOverlap: overlap}
	}
	return []perfrec.Record{
		// Legacy perf-v1 invocations: 1000 + 200 + 900 = 2100 ms.
		v1("deskpr", "create", "worker-desk", 1000, ms(900), 0),
		v1("deskpost", "other", "worker-desk", 200, nil, 0),
		v1("deskboot", "other", "the-desk", 900, ms(100), 0),
		// Steps (perf-v1): inside the deskboot run and a cellctl launch — never invocation time.
		v1("deskboot", "step:token-mint", "the-desk", 300, nil, 0),
		v1("cellctl", "step:launch-worktree", "none", 400, nil, 0),
		// perf-v2 invocations: 2000 + 1000 + 600 = 3600 ms summed invocation wall.
		v2("deskpr", "create", "worker-desk", perfrec.BoundaryProcess, 2000, perfrec.CPUMeasured, ms(500), 0,
			map[perfrec.Counter]int64{perfrec.CounterSubprocess: 2, perfrec.CounterSubprocessMs: 1200, perfrec.CounterHTTP: 3},
			[]perfrec.PhaseStat{{Name: "fetch", N: 1, Ms: 800}, {Name: "render", N: 1, Ms: 300}}, false),
		v2("deskpr", "update", "worker-desk", perfrec.BoundaryProcess, 1000, perfrec.CPUUnsupported, nil, deskkit.ExitRefused,
			map[perfrec.Counter]int64{perfrec.CounterSubprocess: 0, perfrec.CounterSubprocessMs: 0, perfrec.CounterHTTP: 1},
			[]perfrec.PhaseStat{}, false),
		v2("deskboard", "other", "the-desk", perfrec.BoundaryCaller, 600, perfrec.CPUFailed, nil, 0,
			map[perfrec.Counter]int64{perfrec.CounterHTTP: 2, perfrec.CounterCacheLookup: 4, perfrec.CounterCacheHit: 3,
				perfrec.CounterCacheMiss: 1, perfrec.CounterCacheRefresh: 0, perfrec.CounterWaitShared: 0,
				perfrec.CounterWaitFailed: 0, perfrec.CounterWaitAbandon: 0},
			[]perfrec.PhaseStat{{Name: perfrec.Other, N: 2, Ms: 700}}, true),
	}
}

// TestMixedSchemaReportBoundaries reads one history holding perf-v1 and perf-v2 invocations
// plus boot and launch step records, and pins that `deskperf report` keeps the legacy group
// exactly as before, reports the perf-v2 group separately with its coverage (boundary, CPU
// state, counter measured/unavailable, phase overlap) and distributions, that `deskperf boot`
// sees only the steps — and that every naive summed total over the same lines (all walls; v2
// wall plus its inclusive subprocessMs and phase times; v2 records folded into the legacy
// total; unsupported CPU read as zero) is a DIFFERENT number from what is reported.
// regression: desktools-v2/61
func TestMixedSchemaReportBoundaries(t *testing.T) {
	_, perfDir := seedHome(t)
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	recs := mixedRecords(t)
	var lines []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("fixture record does not encode: %+v: %v", r, err)
		}
		lines = append(append(lines, b...), '\n')
	}
	day := filepath.Join(perfDir, time.Now().UTC().Format("2006-01-02")+".jsonl")
	if err := os.WriteFile(day, lines, 0o600); err != nil {
		t.Fatal(err)
	}

	// The naive figures a reader that ignored schema, steps and inclusiveness would print.
	var allWalls, v2Inclusive, v1AndV2 int64
	for _, r := range recs {
		allWalls += r.WallMs
		if r.Schema == perfrec.SchemaV2 {
			v2Inclusive += r.WallMs + r.Counters[perfrec.CounterSubprocessMs]
			for _, p := range r.Phases {
				v2Inclusive += p.Ms
			}
		}
		if !strings.HasPrefix(r.Verb, "step:") {
			v1AndV2 += r.WallMs
		}
	}
	const legacyTotal, v2Sum, bootTotal = 2100, 3600, 700
	for name, naive := range map[string]int64{"all walls": allWalls, "v2 + inclusive": v2Inclusive, "v1+v2 invocations": v1AndV2} {
		if naive == legacyTotal || naive == v2Sum || naive == bootTotal {
			t.Fatalf("fixture cannot tell the naive %q sum (%d) from a correct total", name, naive)
		}
	}

	// The summarisers hold the boundary themselves, not only behind the report's step filter:
	// each is fed the WHOLE mixed history.
	if got := deskkit.SummarisePerf(recs, 0); got.Records != 3 || got.TotalMs != legacyTotal {
		t.Errorf("SummarisePerf over the mixed history = %d record(s), %d ms; want 3, %d", got.Records, got.TotalMs, legacyTotal)
	}
	if got := deskkit.SummarisePerfV2(recs, 0); got.Records != 3 || got.SumWallMs != v2Sum {
		t.Errorf("SummarisePerfV2 over the mixed history = %d record(s), %d ms; want 3, %d", got.Records, got.SumWallMs, v2Sum)
	}
	if got := deskkit.SummariseBootSteps(recs); got.Records != 2 {
		t.Errorf("SummariseBootSteps over the mixed history read %d record(s), want the 2 steps", got.Records)
	}

	rc, out, errOut := runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report --json unparseable: %v\n%s", err, out)
	}
	if rep.Skipped != 0 {
		t.Fatalf("the reader skipped %d valid line(s)", rep.Skipped)
	}
	// Legacy group: perf-v1 invocations only, figures as perf-v1 always meant them.
	if rep.Summary.Records != 3 || rep.Summary.TotalMs != legacyTotal {
		t.Errorf("legacy group = %d record(s), %d ms; want 3, %d (no step, no perf-v2 record)",
			rep.Summary.Records, rep.Summary.TotalMs, legacyTotal)
	}
	v := rep.PerfV2
	if v == nil {
		t.Fatalf("no perf-v2 group in:\n%s", out)
	}
	for name, naive := range map[string]int64{"all walls": allWalls, "v2 + inclusive": v2Inclusive, "v1+v2 invocations": v1AndV2} {
		if rep.Summary.TotalMs == naive || v.SumWallMs == naive {
			t.Errorf("a reported total equals the naive %q sum %d", name, naive)
		}
	}
	if v.Records != 3 || v.SumWallMs != v2Sum {
		t.Errorf("perf-v2 group = %d record(s), %d ms; want 3, %d (no inclusive child or phase time added)", v.Records, v.SumWallMs, v2Sum)
	}
	if v.Boundaries[perfrec.BoundaryProcess] != 2 || v.Boundaries[perfrec.BoundaryCaller] != 1 {
		t.Errorf("boundary coverage = %v", v.Boundaries)
	}
	if v.CPU[perfrec.CPUMeasured] != 1 || v.CPU[perfrec.CPUUnsupported] != 1 || v.CPU[perfrec.CPUFailed] != 1 || v.PhaseOverlap != 1 {
		t.Errorf("cpu coverage = %v, phase overlap %d", v.CPU, v.PhaseOverlap)
	}
	tools := map[string]deskkit.PerfV2ToolStats{}
	for _, ts := range v.Tools {
		tools[ts.Tool] = ts
	}
	pr := tools["deskpr"]
	if pr.Count != 2 || pr.SumWallMs != 3000 || pr.P50Ms != 1000 || pr.MaxMs != 2000 || pr.RefusalRatio != 0.5 {
		t.Errorf("deskpr perf-v2 wall row = %+v", pr)
	}
	// CPU distribution over MEASURED records only: the unsupported record is not a zero.
	if pr.CPUCount != 1 || pr.CPUP50Ms != 500 || pr.CPUMaxMs != 500 {
		t.Errorf("deskpr CPU distribution = n %d p50 %d max %d; want 1 measured record of 500 ms", pr.CPUCount, pr.CPUP50Ms, pr.CPUMaxMs)
	}
	if b := tools["deskboard"]; b.Count != 1 || b.SumWallMs != 600 || b.CPUCount != 0 {
		t.Errorf("deskboard perf-v2 row = %+v (failed CPU must not count)", b)
	}
	cov := map[string]deskkit.PerfCounterCoverage{}
	for _, c := range v.Counters {
		cov[c.Counter] = c
	}
	if len(v.Counters) != len(perfrec.Counters()) {
		t.Errorf("counter coverage lists %d counters, want the whole roster", len(v.Counters))
	}
	for name, want := range map[string]deskkit.PerfCounterCoverage{
		"http":         {Counter: "http", Measured: 3, Unavailable: 0, Total: 6},
		"subprocess":   {Counter: "subprocess", Measured: 2, Unavailable: 1, Total: 2},
		"subprocessMs": {Counter: "subprocessMs", Measured: 2, Unavailable: 1, Total: 1200},
		"cacheLookup":  {Counter: "cacheLookup", Measured: 1, Unavailable: 2, Total: 4},
		"waitAbandon":  {Counter: "waitAbandon", Measured: 1, Unavailable: 2, Total: 0},
	} {
		if cov[name] != want {
			t.Errorf("counter %s coverage = %+v, want %+v", name, cov[name], want)
		}
	}
	phases := map[string]deskkit.PerfPhaseStats{}
	for _, p := range v.Phases {
		phases[p.Tool+"/"+p.Phase] = p
	}
	if p := phases["deskpr/fetch"]; p.Records != 1 || p.N != 1 || p.P50Ms != 800 {
		t.Errorf("deskpr/fetch phase = %+v", p)
	}
	if p := phases["deskboard/other"]; p.Records != 1 || p.N != 2 || p.MaxMs != 700 {
		t.Errorf("deskboard/other phase = %+v", p)
	}
	if len(v.Slowest) == 0 || v.Slowest[0].WallMs != 2000 || rep.Summary.Slowest[0].WallMs != 1000 {
		t.Errorf("slowest lists crossed groups: legacy %+v v2 %+v", rep.Summary.Slowest, v.Slowest)
	}

	// The table: the legacy header unchanged, then the separately labelled perf-v2 group.
	rc, out, errOut = runDeskperf("report")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	for _, want := range []string{
		"deskperf report — 3 record(s) over the last 7 UTC day(s), total wall 2100 ms",
		"perf-v2 — 3 record(s) over the last 7 UTC day(s), summed invocation wall 3600 ms (a sum of per-invocation walls, not elapsed time",
		"boundary: process-init 2, caller 1 · cpu: measured 1, unsupported 1, failed 1, missing 0 · phase overlap: 1 record(s)",
		"unavailable = not observed, never zero",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report table lacks %q:\n%s", want, out)
		}
	}
	if i, j := strings.Index(out, "deskperf report —"), strings.Index(out, "perf-v2 —"); i < 0 || j < i {
		t.Errorf("legacy group must render first, perf-v2 after:\n%s", out)
	}

	// Steps stay in the boot view only, with the launch step alongside the boot step.
	rc, out, _ = runDeskperf("boot", "--json")
	var boot bootJSON
	if rc != deskkit.ExitOK || json.Unmarshal([]byte(out), &boot) != nil {
		t.Fatalf("boot --json rc %d / unparseable: %s", rc, out)
	}
	var steps int64
	for _, r := range boot.Summary.Roles {
		steps += r.TotalMs
	}
	if boot.Summary.Records != 2 || steps != bootTotal {
		t.Errorf("boot view = %d step(s), %d ms; want 2, %d (no invocation record)", boot.Summary.Records, steps, bootTotal)
	}

	// A filter applies to both groups and keeps them apart.
	rc, out, _ = runDeskperf("report", "--json", "--tool", "deskpr")
	rep = reportJSON{}
	if rc != deskkit.ExitOK || json.Unmarshal([]byte(out), &rep) != nil || rep.PerfV2 == nil {
		t.Fatalf("report --tool deskpr rc %d: %s", rc, out)
	}
	if rep.Summary.TotalMs != 1000 || rep.PerfV2.SumWallMs != 3000 {
		t.Errorf("--tool deskpr: legacy %d ms, perf-v2 %d ms; want 1000 and 3000", rep.Summary.TotalMs, rep.PerfV2.SumWallMs)
	}
}
