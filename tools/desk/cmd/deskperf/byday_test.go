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

// byDayRecords is a two-day history for the --by-day timeseries and the per-tool perf-v3
// request rollup: perf-v1, perf-v2 and perf-v3 invocations of two tools across two UTC
// days, a step inside an invocation (never an invocation), and one record whose timestamp
// does not parse (counted, never day-assigned). Every figure is hand-checked in the tests
// below. The v3 records are session-built so their request groups are writer-valid; their
// durations and points are chosen so the histogram estimates land on hand-computable
// sub-interval midpoints.
func byDayRecords(t *testing.T, day0, day1 string) []perfrec.Record {
	t.Helper()
	at := func(day, hm string) string { return day + "T" + hm + ":00Z" }
	v1 := func(tool, verb, ts string, wall, forge int64) perfrec.Record {
		return perfrec.Record{Schema: perfrec.Schema, TS: ts, Tool: tool, Verb: verb, Role: "worker-desk", WallMs: wall,
			ForgeCalls: forge, Result: perfrec.ResultForExit(0), Install: strings.Repeat("a", 32), SourceSHA: "unpinned"}
	}
	v2 := func(tool, ts string, wall int64, http *int64) perfrec.Record {
		counters := map[perfrec.Counter]int64{}
		var un []perfrec.Counter
		for _, c := range perfrec.Counters() {
			if c == perfrec.CounterHTTP && http != nil {
				counters[c] = *http
			} else {
				un = append(un, c)
			}
		}
		return perfrec.Record{Schema: perfrec.SchemaV2, TS: ts, Tool: tool, Verb: "other", Role: "worker-desk",
			Boundary: perfrec.BoundaryProcess, WallMs: wall, CPU: perfrec.CPUUnsupported, Result: perfrec.ResultForExit(0),
			Install: strings.Repeat("b", 32), SourceSHA: "unpinned", Counters: counters, Unavailable: un, Phases: []perfrec.PhaseStat{}}
	}
	// v3: measured points when pts != nil, unknown coverage otherwise.
	v3 := func(ts string, wall time.Duration, http int64, durs []int64, pts *int64) perfrec.Record {
		sp := v3Spec{ts: ts, wall: wall, http: http, durs: durs}
		if pts != nil {
			sp.points = perfrec.Points{Coverage: perfrec.CoverageMeasured, Unit: perfrec.UnitGitHubPoints, Value: pts}
		}
		return buildV3(t, sp)
	}
	n := func(v int64) *int64 { return &v }
	return []perfrec.Record{
		// Day 0, deskboard: two perf-v1 invocations (200 + 1000 ms, 6 forge calls), a step
		// inside one of them, and two perf-v3 records (walls 300 + 300, http 5 + 1).
		v1("deskboard", "actions", at(day0, "10:00"), 1000, 4),
		v1("deskboard", "query", at(day0, "10:05"), 200, 2),
		v1("deskboard", "step:token-mint", at(day0, "10:06"), 50, 9),
		v3(at(day0, "12:00"), 300*time.Millisecond, 5, []int64{5, 5, 100}, n(2)),
		v3(at(day0, "12:30"), 300*time.Millisecond, 1, []int64{30}, nil),
		// Day 0, deskpr: two perf-v2 invocations (500 + 900 ms, http 2 + 3).
		v2("deskpr", at(day0, "11:00"), 500, n(2)),
		v2("deskpr", at(day0, "11:10"), 900, n(3)),
		// Day 1: one perf-v1 deskboard, one perf-v2 deskpr with an unavailable http
		// counter, one perf-v3 deskboard with an unavailable http counter.
		v1("deskboard", "actions", at(day1, "09:00"), 400, 1),
		v2("deskpr", at(day1, "09:30"), 100, nil),
		v3(at(day1, "09:45"), 200*time.Millisecond, -1, []int64{7}, n(3)),
		// Undated: a timestamp that does not parse — counted, never assigned a day.
		v1("deskwt", "other", "not-a-timestamp", 100, 0),
	}
}

// v3Spec describes one fixture perf-v3 deskboard service-client record: its stamp, its wall
// (exactly `wall` — the probe ticks the measurement interval once), its http counter (< 0
// leaves it unavailable), and repeated request observations of ONE group, one per duration,
// carrying `points` (the zero value means unknown coverage). A non-empty session makes it a
// workflow-profile record attributed to that session.
type v3Spec struct {
	ts      string
	wall    time.Duration
	http    int64
	durs    []int64
	points  perfrec.Points
	session string
}

func buildV3(t *testing.T, sp v3Spec) perfrec.Record {
	t.Helper()
	var tick int64
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p := perfrec.Probe{
		Now:   func() time.Time { tick++; return base.Add(time.Duration(tick) * sp.wall) },
		Stamp: func() time.Time { t, _ := time.Parse(time.RFC3339, sp.ts); return t },
	}
	in := perfrec.ContextInput{Profile: perfrec.Value{Value: "tool", Present: true},
		LookupEnv: func(k string) (string, bool) {
			if k == perfrec.EnvVar {
				return "1", true
			}
			return "", false
		}}
	if sp.session != "" {
		in.Profile = perfrec.Value{Value: "workflow", Present: true}
		in.SessionID = perfrec.Value{Value: sp.session, Present: true}
	}
	s := perfrec.BeginObservation(p, nil, perfrec.ResolveContext(in), perfrec.ObservationClient)
	if sp.http >= 0 {
		s.Add(perfrec.CounterHTTP, sp.http)
	}
	pts := sp.points
	if pts.Coverage == "" {
		pts = perfrec.Points{Coverage: perfrec.CoverageUnknown, Unit: perfrec.UnitGitHubPoints}
	}
	for _, d := range sp.durs {
		ms := d
		obs := perfrec.RequestObservation{Class: perfrec.RequestProvider, Origin: perfrec.OriginDirect,
			Protocol: "graphql", Resource: "graphql", Outcome: "2xx",
			DurationMs: &ms, DurationCoverage: perfrec.CoverageMeasured, Points: pts}
		if !s.ObserveRequest(obs) {
			t.Fatalf("fixture request refused: %+v", obs)
		}
	}
	s.ObserveRequests()
	r := s.Finish("deskboard", "query", "", 0, "")
	r.Install = strings.Repeat("c", 32)
	if sp.session != "" && (r.V3 == nil || r.V3.Attribution.SessionID != sp.session) {
		t.Fatalf("fixture record lost its session attribution: %+v", r.V3)
	}
	return r
}

func writePerfDay(t *testing.T, perfDir, day string, recs []perfrec.Record) {
	t.Helper()
	var lines []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("fixture record does not encode: %+v: %v", r, err)
		}
		lines = append(append(lines, b...), '\n')
	}
	if err := os.WriteFile(filepath.Join(perfDir, day+".jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
}

func seedByDay(t *testing.T) (day0, day1 string) {
	t.Helper()
	_, perfDir := seedHome(t)
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	day0 = now.Format("2006-01-02")
	day1 = now.AddDate(0, 0, -1).Format("2006-01-02")
	recs := byDayRecords(t, day0, day1)
	var d0, d1 []perfrec.Record
	for _, r := range recs {
		if strings.HasPrefix(r.TS, day1) {
			d1 = append(d1, r)
		} else {
			d0 = append(d0, r)
		}
	}
	writePerfDay(t, perfDir, day0, d0)
	writePerfDay(t, perfDir, day1, d1)
	return day0, day1
}

// TestByDaySummariserFigures feeds the WHOLE mixed history to the deskkit summarisers and
// hand-checks the by-day rows and the pooled per-tool rollup: schema-separated walls, the
// forge pool extended to perf-v3, the undated count, and the histogram percentile
// estimates against the exact same durations.
// regression: deskperf-by-day
func TestByDaySummariserFigures(t *testing.T) {
	now := time.Now().UTC()
	day0 := now.Format("2006-01-02")
	day1 := now.AddDate(0, 0, -1).Format("2006-01-02")
	recs := byDayRecords(t, day0, day1)

	s, err := deskkit.SummarisePerfDays(recs, "", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Records != 10 || s.Undated != 1 || s.V3Records != 3 || len(s.Rows) != 4 {
		t.Fatalf("summary = %d records, %d undated, %d v3, %d rows; want 10, 1, 3, 4",
			s.Records, s.Undated, s.V3Records, len(s.Rows))
	}
	rows := map[string]deskkit.PerfDayRow{}
	for _, r := range s.Rows {
		rows[r.Day+"/"+r.Tool] = r
	}
	// Rows sort by day, then tool.
	if s.Rows[0].Day != day1 || s.Rows[3].Day != day0 || s.Rows[2].Tool != "deskboard" || s.Rows[3].Tool != "deskpr" {
		t.Errorf("rows out of order: %+v", s.Rows)
	}

	d0b := rows[day0+"/"+"deskboard"]
	if d0b.Count != 4 {
		t.Errorf("day0 deskboard count = %d, want 4 (2 perf-v1 + 2 perf-v3; the step is no invocation)", d0b.Count)
	}
	if w := d0b.PerfV1; w == nil || w.Records != 2 || w.P50Ms != 200 || w.P90Ms != 1000 || w.MaxMs != 1000 || w.SumMs != 1200 {
		t.Errorf("day0 deskboard perf-v1 wall = %+v, want 2 records p50 200 p90 1000 max 1000 sum 1200", w)
	}
	if w := d0b.PerfV3; w == nil || w.Records != 2 || w.P50Ms != 300 || w.P90Ms != 300 || w.SumMs != 600 {
		t.Errorf("day0 deskboard perf-v3 wall = %+v, want 2 records of exactly 300 ms", w)
	}
	if d0b.PerfV2 != nil {
		t.Errorf("day0 deskboard has no perf-v2 record: %+v", d0b.PerfV2)
	}
	// Forge pool: perf-v1 4+2, perf-v3 http 5+1 — one budget, 12 calls over 4 measured runs.
	if d0b.ForgeCalls != 12 || d0b.ForgeMeasured != 4 || d0b.ForgeUnavailable != 0 {
		t.Errorf("day0 deskboard forge = %d calls, %d measured, %d unavailable; want 12, 4, 0",
			d0b.ForgeCalls, d0b.ForgeMeasured, d0b.ForgeUnavailable)
	}
	// The day's request rollup: two groups, kept apart by points coverage. The measured
	// group: 3 attempts, durations 5/5/100 -> bins {3:2, 7:1}; p50 rank 2 sits in bin 3
	// ([4,8)) at 4 + 4*1.5/2 = 7; p90 rank 3 sits in bin 7 ([64,128)) at 64 + 64*0.5 = 96.
	if len(d0b.Requests) != 2 {
		t.Fatalf("day0 deskboard requests = %+v, want 2 groups", d0b.Requests)
	}
	var measured, unknown *deskkit.PerfRequestRollup
	for i := range d0b.Requests {
		switch d0b.Requests[i].PointsCoverage {
		case "measured":
			measured = &d0b.Requests[i]
		case "unknown":
			unknown = &d0b.Requests[i]
		}
	}
	if measured == nil || unknown == nil {
		t.Fatalf("request groups lost their points coverage: %+v", d0b.Requests)
	}
	if measured.Attempts != 3 || measured.DurationMeasured != 3 || measured.DurationSumMs != 110 || measured.MaxMs != 100 ||
		measured.P50Ms == nil || *measured.P50Ms != 7 || measured.P90Ms == nil || *measured.P90Ms != 96 {
		t.Errorf("measured request group = %+v, want attempts 3, sum 110, max 100, p50~ 7, p90~ 96", measured)
	}
	if measured.Points == nil || *measured.Points != 6 || measured.PointsUnit != "github-points" {
		t.Errorf("measured points = %+v, want 6 github-points summed", measured.Points)
	}
	if unknown.Attempts != 1 || unknown.P50Ms == nil || *unknown.P50Ms != 24 || unknown.Points != nil {
		t.Errorf("unknown-coverage group = %+v, want 1 attempt, p50~ 24 (bin [16,32)), NO points sum", unknown)
	}

	d1b := rows[day1+"/"+"deskboard"]
	if d1b.Count != 2 || d1b.PerfV1 == nil || d1b.PerfV1.P50Ms != 400 || d1b.PerfV3 == nil || d1b.PerfV3.P50Ms != 200 {
		t.Errorf("day1 deskboard = %+v, want count 2, perf-v1 p50 400, perf-v3 p50 200", d1b)
	}
	// The perf-v3 http counter is unavailable: not observed, never zero — 1 measured run
	// (the perf-v1 record's 1 call) and 1 unavailable.
	if d1b.ForgeCalls != 1 || d1b.ForgeMeasured != 1 || d1b.ForgeUnavailable != 1 {
		t.Errorf("day1 deskboard forge = %d/%d/%d, want 1 call, 1 measured, 1 unavailable",
			d1b.ForgeCalls, d1b.ForgeMeasured, d1b.ForgeUnavailable)
	}
	if len(d1b.Requests) != 1 || d1b.Requests[0].P50Ms == nil || *d1b.Requests[0].P50Ms != 6 {
		t.Errorf("day1 deskboard requests = %+v, want 1 group of 1 attempt at p50~ 6 (bin [4,8))", d1b.Requests)
	}

	d0p := rows[day0+"/"+"deskpr"]
	if d0p.Count != 2 || d0p.PerfV2 == nil || d0p.PerfV2.P50Ms != 500 || d0p.PerfV2.P90Ms != 900 || d0p.PerfV2.SumMs != 1400 {
		t.Errorf("day0 deskpr = %+v, want 2 perf-v2 records p50 500 p90 900 sum 1400", d0p)
	}
	if d0p.ForgeCalls != 5 || d0p.ForgeMeasured != 2 {
		t.Errorf("day0 deskpr forge = %d/%d, want 5 calls over 2 measured runs", d0p.ForgeCalls, d0p.ForgeMeasured)
	}
	d1p := rows[day1+"/"+"deskpr"]
	if d1p.ForgeMeasured != 0 || d1p.ForgeUnavailable != 1 || d1p.ForgeCalls != 0 {
		t.Errorf("day1 deskpr forge = %d/%d/%d, want 0 measured, 1 unavailable, no calls",
			d1p.ForgeCalls, d1p.ForgeMeasured, d1p.ForgeUnavailable)
	}

	// The pooled per-tool rollup merges across days: deskboard's measured group is one
	// row of 4 attempts (bins {3:3, 7:1}; p50 rank 2 -> 4 + 4*1.5/3 = 6, p90 rank 4 ->
	// bin 7's 96), points 6+3 = 9; deskpr, carrying no perf-v3 record, is absent.
	byTool, err := deskkit.SummarisePerfV3RequestsByTool(recs, "", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(byTool) != 1 || byTool[0].Tool != "deskboard" || byTool[0].Records != 3 || len(byTool[0].Requests) != 2 {
		t.Fatalf("pooled rollup = %+v, want deskboard only, 3 records, 2 groups", byTool)
	}
	var pool *deskkit.PerfRequestRollup
	for i := range byTool[0].Requests {
		if byTool[0].Requests[i].PointsCoverage == "measured" {
			pool = &byTool[0].Requests[i]
		}
	}
	if pool == nil || pool.Attempts != 4 || pool.P50Ms == nil || *pool.P50Ms != 6 || pool.P90Ms == nil || *pool.P90Ms != 96 ||
		pool.Points == nil || *pool.Points != 9 || pool.DurationSumMs != 117 {
		t.Errorf("pooled measured group = %+v, want attempts 4, p50~ 6, p90~ 96, points 9, sum 117", pool)
	}
}

// TestByDayReportRender renders the two-day fixture through `deskperf report --by-day`,
// text and JSON, and pins the pooled report's per-tool request section and the workflow
// filter's refusal in by-day mode.
func TestByDayReportRender(t *testing.T) {
	day0, day1 := seedByDay(t)

	rc, out, errOut := runDeskperf("report", "--by-day")
	if rc != deskkit.ExitOK {
		t.Fatalf("report --by-day rc %d stderr %q", rc, errOut)
	}
	for _, want := range []string{
		"deskperf report --by-day — 10 invocation record(s) over the last 7 UTC day(s), 1 undated",
		"DAY", "SCHEMA",
		"forge calls by UTC day",
		"perf-v3 requests:",
		"estimated from the durationBins histogram",
		day0, day1,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--by-day table lacks %q:\n%s", want, out)
		}
	}
	// One whitespace-normalised line per tool × day × schema group.
	lines := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		lines[strings.Join(strings.Fields(line), " ")] = true
	}
	for _, want := range []string{
		day0 + " deskboard perf-v1 2 200 1000 1000 1200",
		day0 + " deskboard perf-v3 2 300 300 300 600",
		day0 + " deskpr perf-v2 2 500 900 900 1400",
		day1 + " deskboard perf-v1 1 400 400 400 400",
		day0 + " deskboard 12 4 0",
		day1 + " deskpr - 0 1",
	} {
		if !lines[want] {
			t.Errorf("--by-day table lacks row %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "deskwt") {
		t.Errorf("the undated record was assigned a day:\n%s", out)
	}

	rc, out, errOut = runDeskperf("report", "--by-day", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report --by-day --json rc %d stderr %q", rc, errOut)
	}
	var rep reportDayJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("--by-day --json unparseable: %v\n%s", err, out)
	}
	if rep.Days != 7 || rep.ByDay.Records != 10 || rep.ByDay.Undated != 1 || len(rep.ByDay.Rows) != 4 {
		t.Errorf("--by-day --json drifted: %+v", rep.ByDay)
	}
	var d0b *deskkit.PerfDayRow
	for i := range rep.ByDay.Rows {
		if rep.ByDay.Rows[i].Day == day0 && rep.ByDay.Rows[i].Tool == "deskboard" {
			d0b = &rep.ByDay.Rows[i]
		}
	}
	if d0b == nil || d0b.ForgeCalls != 12 || len(d0b.Requests) != 2 ||
		d0b.Requests[0].P50Ms == nil || d0b.Requests[1].P50Ms == nil {
		t.Errorf("--by-day --json day0 deskboard row drifted: %+v", d0b)
	}

	// The pooled report gains the per-tool request rollup; the existing groups do not move.
	rc, out, errOut = runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report --json rc %d stderr %q", rc, errOut)
	}
	var pooled reportJSON
	if err := json.Unmarshal([]byte(out), &pooled); err != nil {
		t.Fatalf("report --json unparseable: %v\n%s", err, out)
	}
	if len(pooled.RequestsByTool) != 1 || pooled.RequestsByTool[0].Tool != "deskboard" ||
		len(pooled.RequestsByTool[0].Requests) != 2 {
		t.Errorf("pooled requestsByTool drifted: %+v", pooled.RequestsByTool)
	}
	if pooled.Summary.TotalMs != 1700 || pooled.PerfV2 == nil || pooled.PerfV2.SumWallMs != 1500 {
		t.Errorf("the existing groups moved: legacy %d ms, perf-v2 %+v", pooled.Summary.TotalMs, pooled.PerfV2)
	}
	rc, out, errOut = runDeskperf("report")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	if !strings.Contains(out, "perf-v3 requests by tool") || !strings.Contains(out, "deskboard (3 records)") {
		t.Errorf("pooled text report lacks the per-tool request rollup:\n%s", out)
	}

	// A workflow filter that matches no v3 record refuses in by-day mode, stdout empty.
	rc, out, errOut = runDeskperf("report", "--by-day", "--perf-profile", "workflow",
		"--session-id", strings.Repeat("d", 32))
	if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "no v3 record matches the workflow filter") {
		t.Errorf("--by-day with an unmatched workflow filter: rc %d stdout %q stderr %q", rc, out, errOut)
	}
	// A tool filter that matches nothing refuses as in the pooled report.
	rc, out, errOut = runDeskperf("report", "--by-day", "--tool", "deskclose")
	if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "no record matches --tool deskclose") {
		t.Errorf("--by-day --tool deskclose: rc %d stdout %q stderr %q", rc, out, errOut)
	}
}

// dayRow finds one tool × day row of a by-day summary.
func dayRow(s deskkit.PerfDaySummary, day, tool string) (deskkit.PerfDayRow, bool) {
	for _, r := range s.Rows {
		if r.Day == day && r.Tool == tool {
			return r, true
		}
	}
	return deskkit.PerfDayRow{}, false
}

// TestByDayV3Duplicate replays one perf-v3 record byte-identically. ReduceV3 counts it
// once, so EVERY figure of its day row — the invocation count, the perf-v3 wall and the
// forge pool, not just the request rollup — must count it once.
// regression: #3959
func TestByDayV3Duplicate(t *testing.T) {
	day0, day1 := "2026-01-05", "2026-01-04"
	recs := byDayRecords(t, day0, day1)
	recs = append(recs, recs[3]) // the first day-0 deskboard perf-v3 record, replayed
	s, err := deskkit.SummarisePerfDays(recs, "", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Records != 10 || s.V3Records != 3 || s.V3Duplicates != 1 {
		t.Errorf("summary = %d records, %d v3, %d duplicates; want 10, 3, 1", s.Records, s.V3Records, s.V3Duplicates)
	}
	d0b, _ := dayRow(s, day0, "deskboard")
	if d0b.Count != 4 {
		t.Errorf("day0 deskboard count = %d, want 4: the replay is one invocation", d0b.Count)
	}
	if w := d0b.PerfV3; w == nil || w.Records != 2 || w.SumMs != 600 {
		t.Errorf("day0 deskboard perf-v3 wall = %+v, want 2 records summing 600 ms", w)
	}
	if d0b.ForgeCalls != 12 || d0b.ForgeMeasured != 4 {
		t.Errorf("day0 deskboard forge = %d calls over %d runs, want 12 over 4", d0b.ForgeCalls, d0b.ForgeMeasured)
	}
}

// TestByDayV3Conflict moves a copy of the day-1 perf-v3 record to day 0: one identity, two
// different records. ReduceV3 excludes a conflicting identity completely, and the by-day
// view must too — even though the two copies land in different day buckets.
// regression: #3959
func TestByDayV3Conflict(t *testing.T) {
	day0, day1 := "2026-01-05", "2026-01-04"
	recs := byDayRecords(t, day0, day1)
	moved := recs[9] // the day-1 deskboard perf-v3 record
	moved.TS = day0 + "T13:00:00Z"
	recs = append(recs, moved)
	s, err := deskkit.SummarisePerfDays(recs, "", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Records != 9 || s.V3Records != 2 || s.V3Conflicts != 1 {
		t.Errorf("summary = %d records, %d v3, %d conflicts; want 9, 2, 1", s.Records, s.V3Records, s.V3Conflicts)
	}
	d0b, _ := dayRow(s, day0, "deskboard")
	if d0b.Count != 4 || d0b.PerfV3 == nil || d0b.PerfV3.Records != 2 || d0b.ForgeCalls != 12 {
		t.Errorf("day0 deskboard = %+v, want count 4, 2 perf-v3 walls, 12 forge calls", d0b)
	}
	d1b, _ := dayRow(s, day1, "deskboard")
	if d1b.Count != 1 || d1b.PerfV3 != nil || d1b.ForgeUnavailable != 0 || len(d1b.Requests) != 0 {
		t.Errorf("day1 deskboard = %+v, want count 1 (perf-v1 only): the conflicting identity is excluded", d1b)
	}
}

// TestByToolCrossConflict: the pooled per-tool rollup selects window-wide too. One
// invocation identity recorded under two tools with different contents is a conflict, so
// neither tool may report its requests.
// regression: #3959
func TestByToolCrossConflict(t *testing.T) {
	r := buildV3(t, v3Spec{ts: "2026-01-05T10:00:00Z", wall: time.Millisecond, http: 1, durs: []int64{10}})
	other := r
	other.Tool = "deskpr"
	out, err := deskkit.SummarisePerfV3RequestsByTool([]perfrec.Record{r, other}, "", perfrec.V3Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("per-tool rollup = %+v, want none: the conflicting identity is excluded", out)
	}
}

// TestByDaySessionFilter filters to one workflow session. Session B's wall, forge calls and
// requests must leave every figure of the row, not just the request rollup; a perf-v1
// record, which carries no session, cannot be attributed to session A either.
// regression: #3959
func TestByDaySessionFilter(t *testing.T) {
	day := "2026-01-05"
	a, b := strings.Repeat("1", 32), strings.Repeat("2", 32)
	recs := []perfrec.Record{
		buildV3(t, v3Spec{ts: day + "T10:00:00Z", wall: 100 * time.Millisecond, http: 1, durs: []int64{10}, session: a}),
		buildV3(t, v3Spec{ts: day + "T11:00:00Z", wall: 900 * time.Millisecond, http: 50, durs: []int64{20, 20}, session: b}),
		{Schema: perfrec.Schema, TS: day + "T12:00:00Z", Tool: "deskboard", Verb: "query", Role: "worker-desk", WallMs: 400,
			ForgeCalls: 2, Result: perfrec.ResultForExit(0), Install: strings.Repeat("a", 32), SourceSHA: "unpinned"},
	}
	s, err := deskkit.SummarisePerfDays(recs, perfrec.ProfileWorkflow, perfrec.V3Filter{SessionID: a})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Rows) != 1 {
		t.Fatalf("rows = %+v, want one deskboard row", s.Rows)
	}
	if s.Records != 1 || s.V3Records != 1 || s.Unattributed != 1 {
		t.Errorf("summary = %d records, %d v3, %d unattributed; want 1, 1, 1", s.Records, s.V3Records, s.Unattributed)
	}
	r := s.Rows[0]
	if r.Count != 1 || r.PerfV1 != nil {
		t.Errorf("row count %d, perf-v1 %+v; want 1 and none: only session A's record is in view", r.Count, r.PerfV1)
	}
	if w := r.PerfV3; w == nil || w.Records != 1 || w.SumMs != 100 || w.MaxMs != 100 {
		t.Errorf("perf-v3 wall = %+v, want session A's one 100 ms record", w)
	}
	if r.ForgeCalls != 1 || r.ForgeMeasured != 1 {
		t.Errorf("forge = %d calls over %d runs, want session A's 1 over 1", r.ForgeCalls, r.ForgeMeasured)
	}
	if len(r.Requests) != 1 || r.Requests[0].Attempts != 1 {
		t.Errorf("requests = %+v, want session A's 1 attempt", r.Requests)
	}
}

// TestByDayEstimatedPoints pins "points are summed only where measured": an
// estimated-coverage group CARRIES a points value, and neither rollup may report it as a
// measured sum, while the measured group beside it is summed.
// regression: #3959
func TestByDayEstimatedPoints(t *testing.T) {
	day := "2026-01-05"
	two, five := int64(2), int64(5)
	recs := []perfrec.Record{
		buildV3(t, v3Spec{ts: day + "T10:00:00Z", wall: time.Millisecond, http: 1, durs: []int64{10},
			points: perfrec.Points{Coverage: perfrec.CoverageMeasured, Unit: perfrec.UnitGitHubPoints, Value: &two}}),
		buildV3(t, v3Spec{ts: day + "T11:00:00Z", wall: time.Millisecond, http: 1, durs: []int64{10},
			points: perfrec.Points{Coverage: perfrec.CoverageEstimated, Unit: perfrec.UnitGitHubPoints, Value: &five,
				Method: "query-cost", Version: "v1"}}),
	}
	check := func(where string, rows []deskkit.PerfRequestRollup) {
		t.Helper()
		var measured, estimated *deskkit.PerfRequestRollup
		for i := range rows {
			switch rows[i].PointsCoverage {
			case "measured":
				measured = &rows[i]
			case "estimated":
				estimated = &rows[i]
			}
		}
		if measured == nil || estimated == nil {
			t.Fatalf("%s: want one measured and one estimated group: %+v", where, rows)
		}
		if measured.Points == nil || *measured.Points != 2 {
			t.Errorf("%s: measured points = %v, want 2", where, measured.Points)
		}
		if estimated.Points != nil || estimated.PointsUnit != "" {
			t.Errorf("%s: estimated points read as a measured sum: %d %s", where, *estimated.Points, estimated.PointsUnit)
		}
	}
	s, err := deskkit.SummarisePerfDays(recs, "", perfrec.V3Filter{})
	if err != nil || len(s.Rows) != 1 {
		t.Fatalf("by-day = %+v, %v; want one row", s, err)
	}
	check("by-day", s.Rows[0].Requests)
	byTool, err := deskkit.SummarisePerfV3RequestsByTool(recs, "", perfrec.V3Filter{})
	if err != nil || len(byTool) != 1 {
		t.Fatalf("per-tool = %+v, %v; want one tool", byTool, err)
	}
	check("per-tool", byTool[0].Requests)
}

// TestByDayGroupByRefused: --by-day always groups by tool, so another --group-by refuses
// rather than exiting 0 with tool rows.
// regression: #3959
func TestByDayGroupByRefused(t *testing.T) {
	seedByDay(t)
	rc, out, errOut := runDeskperf("report", "--by-day", "--perf-profile", "workflow", "--group-by", "session")
	if rc != deskkit.ExitRefused || out != "" || !strings.Contains(errOut, "--group-by session") {
		t.Errorf("--by-day --group-by session: rc %d stdout %q stderr %q", rc, out, errOut)
	}
	rc, _, errOut = runDeskperf("report", "--by-day", "--group-by", "tool")
	if rc != deskkit.ExitOK {
		t.Errorf("--by-day --group-by tool (the default, spelled out): rc %d stderr %q", rc, errOut)
	}
}

// TestPooledForgeV3Note: the pooled forge section reads perf-v1 and perf-v2 only, while the
// by-day forge column also pools perf-v3 http. Both outputs say so when the window holds a
// perf-v3 record, so their totals never silently disagree.
// regression: #3959
func TestPooledForgeV3Note(t *testing.T) {
	seedByDay(t)
	for _, args := range [][]string{{"report"}, {"report", "--by-day"}} {
		rc, out, errOut := runDeskperf(args...)
		if rc != deskkit.ExitOK {
			t.Fatalf("%v: rc %d stderr %q", args, rc, errOut)
		}
		if !strings.Contains(out, "the pooled forge section above leaves out perf-v3") &&
			!strings.Contains(out, "the pooled report's forge section leaves out perf-v3") {
			t.Errorf("%v: no note on how the two forge totals differ:\n%s", args, out)
		}
	}
}
