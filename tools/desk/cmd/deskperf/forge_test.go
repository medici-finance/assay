package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgeRecords is one day of history for the forge section: a perf-v1 invocation and perf-v2
// invocations across two release builds, a measured ZERO, two runs whose http counter is
// unavailable (one of them the tool's only run), a statusgen perf-v1 record whose 0 means "not
// counted", and a perf-v1 step inside an invocation. Every figure is hand-checked in the test below.
func forgeRecords(t *testing.T) []perfrec.Record {
	t.Helper()
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	at := func(hm string) string { return day + "T" + hm + ":00Z" }
	v1 := func(tool, verb, hm string, forge int64, sha string) perfrec.Record {
		return perfrec.Record{Schema: perfrec.Schema, TS: at(hm), Tool: tool, Verb: verb, Role: "worker-desk", WallMs: 100,
			ForgeCalls: forge, Result: perfrec.ResultForExit(0), Install: strings.Repeat("a", 32), SourceSHA: sha}
	}
	v2 := func(tool, hm string, http *int64, sha string) perfrec.Record {
		counters := map[perfrec.Counter]int64{}
		var un []perfrec.Counter
		for _, c := range perfrec.Counters() {
			if c == perfrec.CounterHTTP && http != nil {
				counters[c] = *http
			} else {
				un = append(un, c)
			}
		}
		return perfrec.Record{Schema: perfrec.SchemaV2, TS: at(hm), Tool: tool, Verb: "other", Role: "worker-desk",
			Boundary: perfrec.BoundaryProcess, WallMs: 100, CPU: perfrec.CPUUnsupported, Result: perfrec.ResultForExit(0),
			Install: strings.Repeat("b", 32), SourceSHA: sha, Counters: counters, Unavailable: un, Phases: []perfrec.PhaseStat{}}
	}
	n := func(v int64) *int64 { return &v }
	return []perfrec.Record{
		v1("deskpr", "create", "10:05", 4, "aaaa111"),
		// A step inside an invocation: its calls are already the invocation's, never a run.
		v1("deskboot", "step:token-mint", "10:40", 9, "aaaa111"),
		v2("deskpr", "10:20", n(6), "bbbb222"),
		v2("deskpr", "11:10", n(0), "bbbb222"), // measured zero: a run that made no forge call
		v2("deskboard", "11:15", nil, "bbbb222"),
		v2("deskboard", "11:20", n(11), "bbbb222"),
		v2("deskclose", "11:30", nil, "bbbb222"),        // the tool's only run was not observed
		v1("statusgen", "regen", "11:50", 0, "aaaa111"), // perf-v1 0 documented as "not counted"
	}
}

// TestForgeSectionFigures pins the forge section of `deskperf report` over a hand-built
// history: perf-v1 forgeCalls and perf-v2 http pooled per tool and per release build, the
// calls-per-run distribution, the share, and the busiest hour and p90 run against the 5000/hour
// App budget. An unavailable http counter, or statusgen's uncounted perf-v1 0, is a run with no
// figure — never a zero call — and a step or perf-v3 record adds nothing; reading either the naive way moves a reported figure.
// regression: deskperf-forge-report
func TestForgeSectionFigures(t *testing.T) {
	_, perfDir := seedHome(t)
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	recs := forgeRecords(t)
	var lines []byte
	for _, r := range recs {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("fixture record does not encode: %+v: %v", r, err)
		}
		lines = append(append(lines, b...), '\n')
	}
	day1 := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	if err := os.WriteFile(filepath.Join(perfDir, day1+".jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}

	// The summariser holds the boundary itself, fed the WHOLE history plus a perf-v3 record
	// carrying an http counter: neither the step's 9 nor the v3 record's 50 is read.
	v3 := deskkit.PerfRecord{Schema: perfrec.SchemaV3, TS: day1 + "T11:40:00Z", Tool: "deskpr", SourceSHA: "bbbb222",
		Counters: map[perfrec.Counter]int64{perfrec.CounterHTTP: 50}}
	whole := deskkit.SummarisePerfForge(append(append([]deskkit.PerfRecord{}, recs...), v3))
	if whole.Calls != 21 || whole.Runs != 7 || whole.PeakHour != day1+"T11:00Z" || whole.PeakHourCalls != 11 {
		t.Errorf("SummarisePerfForge over the whole history = %d call(s), %d run(s), peak %s %d; want 21, 7, %sT11:00Z 11",
			whole.Calls, whole.Runs, whole.PeakHour, whole.PeakHourCalls, day1)
	}
	// Two hours with the same calls: the busiest hour is the earlier one, every run.
	early := recs[0]
	early.TS = day1 + "T09:05:00Z"
	if tie := deskkit.SummarisePerfForge([]deskkit.PerfRecord{recs[0], early}); tie.PeakHour != day1+"T09:00Z" || tie.PeakHourCalls != 4 {
		t.Errorf("tied hours: peak %s %d; want the earlier, %sT09:00Z 4", tie.PeakHour, tie.PeakHourCalls, day1)
	}

	rc, out, errOut := runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("report --json unparseable: %v\n%s", err, out)
	}
	f := rep.Forge
	if f == nil {
		t.Fatalf("no forge section in:\n%s", out)
	}
	if f.BudgetPerHour != 5000 || f.Runs != 7 || f.Measured != 4 || f.Unavailable != 3 || f.FromV1 != 1 || f.FromV2 != 3 ||
		f.Calls != 21 || f.Unhoured != 0 {
		t.Errorf("forge totals = %+v; want budget 5000, 7 runs, 4 measured (1 v1 + 3 v2), 3 unavailable, 21 calls", *f)
	}
	if f.PeakHour != day1+"T11:00Z" || f.PeakHourCalls != 11 || f.PeakHourBudgetPct != 0.22 {
		t.Errorf("busiest hour = %s %d (%.2f%%); want %sT11:00Z 11 (0.22%%)", f.PeakHour, f.PeakHourCalls, f.PeakHourBudgetPct, day1)
	}
	want := map[string]deskkit.PerfForgeStats{
		"deskboard": {Key: "deskboard", Runs: 2, Measured: 1, Unavailable: 1, Calls: 11, P50PerRun: 11, P90PerRun: 11, MaxPerRun: 11,
			SharePct: 52.4, PeakHour: day1 + "T11:00Z", PeakHourCalls: 11, PeakHourBudgetPct: 0.22, P90RunBudgetPct: 0.22},
		"deskpr": {Key: "deskpr", Runs: 3, Measured: 3, Calls: 10, P50PerRun: 4, P90PerRun: 6, MaxPerRun: 6,
			SharePct: 47.6, PeakHour: day1 + "T10:00Z", PeakHourCalls: 10, PeakHourBudgetPct: 0.2, P90RunBudgetPct: 0.12},
		"deskclose": {Key: "deskclose", Runs: 1, Unavailable: 1},
		"statusgen": {Key: "statusgen", Runs: 1, Unavailable: 1},
		"bbbb222": {Key: "bbbb222", Runs: 5, Measured: 3, Unavailable: 2, Calls: 17, P50PerRun: 6, P90PerRun: 11, MaxPerRun: 11,
			SharePct: 81, PeakHour: day1 + "T11:00Z", PeakHourCalls: 11, PeakHourBudgetPct: 0.22, P90RunBudgetPct: 0.22},
		"aaaa111": {Key: "aaaa111", Runs: 2, Measured: 1, Unavailable: 1, Calls: 4, P50PerRun: 4, P90PerRun: 4, MaxPerRun: 4,
			SharePct: 19, PeakHour: day1 + "T10:00Z", PeakHourCalls: 4, PeakHourBudgetPct: 0.08, P90RunBudgetPct: 0.08},
	}
	var order []string
	for _, rows := range [][]deskkit.PerfForgeStats{f.Tools, f.Releases} {
		for _, r := range rows {
			order = append(order, r.Key)
			if r != want[r.Key] {
				t.Errorf("forge row %s = %+v\nwant %+v", r.Key, r, want[r.Key])
			}
		}
	}
	if got := strings.Join(order, " "); got != "deskboard deskpr deskclose statusgen bbbb222 aaaa111" {
		t.Errorf("forge row order = %q; want tools then releases, each by calls descending", got)
	}

	// The table: the section header, and the unobserved tool printed as "-", never as 0.
	rc, out, errOut = runDeskperf("report")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	for _, line := range []string{
		"forge calls — 21 call(s) over 7 run(s) (legacy forgeCalls 1, perf-v2 http 3, unavailable 3); busiest UTC hour " +
			day1 + "T11:00Z 11 call(s), 0.22% of the 5000/hour App budget floor",
		"deskboard 2 1 11 11 11 11 52.4 11 0.22 0.22",
		"deskpr 3 0 10 4 6 6 47.6 10 0.20 0.12",
		"deskclose 1 1 - - - - - - - -",
		"statusgen 1 1 - - - - - - - -",
		"bbbb222 5 2 17 6 11 11 81.0 11 0.22 0.22",
	} {
		if !strings.Contains(squash(out), line) {
			t.Errorf("report table lacks %q:\n%s", line, out)
		}
	}
	if i, j := strings.Index(out, "perf-v2 —"), strings.Index(out, "forge calls —"); i < 0 || j < i {
		t.Errorf("the forge section must follow the perf-v2 group:\n%s", out)
	}
}

// TestForgeSectionFixture renders the forge section over the committed perf-v1
// fixture (the same files the deskkit aggregate test reads): the 0-call deskboot run is a
// measured zero, the steps add nothing, and the out-of-window day never renders.
// regression: deskperf-forge-report
func TestForgeSectionFixture(t *testing.T) {
	_, perfDir := seedHome(t)
	seedFixture(t, perfDir)
	rc, out, errOut := runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Forge == nil {
		t.Fatalf("report --json unparseable or without a forge section: %v\n%s", err, out)
	}
	f := rep.Forge
	if f.Runs != 8 || f.Measured != 8 || f.FromV1 != 8 || f.Calls != 27 || f.PeakHour != "2026-10-11T09:00Z" ||
		f.PeakHourCalls != 21 || f.PeakHourBudgetPct != 0.42 {
		t.Errorf("fixture forge totals = %+v; want 8 v1 runs, 27 calls, peak 2026-10-11T09:00Z 21 (0.42%%)", *f)
	}
	rows := map[string]deskkit.PerfForgeStats{}
	for _, r := range append(append([]deskkit.PerfForgeStats{}, f.Tools...), f.Releases...) {
		rows[r.Key] = r
	}
	if r := rows["deskpr"]; r.Calls != 20 || r.P50PerRun != 3 || r.P90PerRun != 12 || r.SharePct != 74.1 || r.PeakHourCalls != 20 ||
		r.PeakHourBudgetPct != 0.4 || r.P90RunBudgetPct != 0.24 {
		t.Errorf("fixture deskpr forge row = %+v", r)
	}
	if r := rows["deskpost"]; r.Calls != 7 || r.P50PerRun != 2 || r.P90PerRun != 4 || r.SharePct != 25.9 || r.PeakHourCalls != 6 {
		t.Errorf("fixture deskpost forge row = %+v", r)
	}
	if r := rows["deskboot"]; r.Runs != 1 || r.Measured != 1 || r.Calls != 0 {
		t.Errorf("fixture deskboot forge row = %+v; want one measured run of 0 calls (no step)", r)
	}
	if r := rows["abc1234"]; r.Runs != 8 || r.Calls != 27 || r.P50PerRun != 2 || r.P90PerRun != 12 || r.SharePct != 100 {
		t.Errorf("fixture release forge row = %+v", r)
	}
	rc, out, _ = runDeskperf("report")
	if rc != deskkit.ExitOK || !strings.Contains(squash(out), "deskpr 4 0 20 3 12 12 74.1 20 0.40 0.24") {
		t.Errorf("fixture report table lacks the deskpr forge row:\n%s", out)
	}
}

// TestForgeHostileLabels plants perf-v1 records whose tool and sourceSHA are not roster
// labels or build stamps — a URL with a query, a file path, and tab, newline and
// terminal-escape bytes — and checks the forge section, text and JSON, prints none of it: the
// rows fall under the "other" and "unpinned" placeholders the perf-v3 reader already uses.
// regression: deskperf-forge-labels
func TestForgeHostileLabels(t *testing.T) {
	_, perfDir := seedHome(t)
	if err := os.MkdirAll(perfDir, 0o700); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	planted := []string{"PLANTED", "planted", "forge.example", "forged", "\x1b", "\t"}
	rec := func(tool, sha string, forge int64) perfrec.Record {
		return perfrec.Record{Schema: perfrec.Schema, TS: day + "T10:00:00Z", Tool: tool, Verb: "create", Role: "worker-desk",
			WallMs: 100, ForgeCalls: forge, Result: perfrec.ResultForExit(0), Install: strings.Repeat("a", 32), SourceSHA: sha}
	}
	var lines []byte
	for _, r := range []perfrec.Record{
		rec("deskpr", "https://forge.example/build?sig=PLANTED", 3),
		rec("deskpr", "../planted/src/build", 2),
		rec("deskpr\tforged\n\x1b[31mPLANTED", "aaaa111", 4),
		rec("deskpr", "aaaa111\tforged\nPLANTED", 1),
	} {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(append(lines, b...), '\n')
	}
	if err := os.WriteFile(filepath.Join(perfDir, day+".jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	rc, out, errOut := runDeskperf("report", "--json")
	if rc != deskkit.ExitOK {
		t.Fatalf("report rc %d stderr %q", rc, errOut)
	}
	var rep reportJSON
	if err := json.Unmarshal([]byte(out), &rep); err != nil || rep.Forge == nil {
		t.Fatalf("report --json unparseable or without a forge section: %v\n%s", err, out)
	}
	forgeJSON, _ := json.Marshal(rep.Forge)
	var keys []string
	for _, r := range append(append([]deskkit.PerfForgeStats{}, rep.Forge.Tools...), rep.Forge.Releases...) {
		keys = append(keys, r.Key)
	}
	if got := strings.Join(keys, " "); got != "deskpr other unpinned aaaa111" {
		t.Errorf("forge row keys = %q; want deskpr other unpinned aaaa111", got)
	}
	rc, out, _ = runDeskperf("report")
	section := out[max(strings.Index(out, "forge calls —"), 0):]
	for _, bad := range planted {
		if strings.Contains(section, bad) || strings.Contains(string(forgeJSON), bad) {
			t.Errorf("the forge section echoes a planted %q:\n%s\n%s", bad, section, forgeJSON)
		}
	}
	if rc != deskkit.ExitOK || !strings.Contains(squash(section), "unpinned 3 0 6 2 3 3 60.0 6 0.12 0.06") {
		t.Errorf("report rc %d; forge section lacks the unpinned row:\n%s", rc, section)
	}
}

// TestForgeDistributionEdges pins what the hand-built histories above cannot: a row with
// eleven measured runs, where p90 is not the max; a record whose timestamp does not parse,
// counted in the totals and in no hour; an all-zero history, whose shares stay 0, never NaN;
// a negative count of either schema, read as unavailable; and two counts whose sums would overflow.
// regression: deskperf-forge-edges
func TestForgeDistributionEdges(t *testing.T) {
	rec := func(tool, ts string, http int64) deskkit.PerfRecord {
		return deskkit.PerfRecord{Schema: perfrec.SchemaV2, TS: ts, Tool: tool, SourceSHA: "bbbb222",
			Counters: map[perfrec.Counter]int64{perfrec.CounterHTTP: http}}
	}
	var recs []deskkit.PerfRecord
	for i := int64(1); i <= 11; i++ {
		recs = append(recs, rec("deskpr", "2026-10-11T"+[]string{"00", "01", "02", "03", "04", "05", "06", "07", "08", "09", "10"}[i-1]+":05:00Z", i))
	}
	recs = append(recs, rec("deskboard", "not-a-time", 100))
	s := deskkit.SummarisePerfForge(recs)
	rows := map[string]deskkit.PerfForgeStats{}
	for _, r := range s.Tools {
		rows[r.Key] = r
	}
	if r := rows["deskpr"]; r.P50PerRun != 6 || r.P90PerRun != 10 || r.MaxPerRun != 11 || r.P90RunBudgetPct != 0.2 {
		t.Errorf("eleven-run row = %+v; want p50 6, p90 10, max 11, p90 run 0.20%%", r)
	}
	if r := rows["deskboard"]; s.Unhoured != 1 || s.Calls != 166 || r.Calls != 100 || r.PeakHour != "" || r.PeakHourCalls != 0 {
		t.Errorf("unparseable timestamp: unhoured %d, calls %d, row %+v; want 1, 166, 100 calls in no hour", s.Unhoured, s.Calls, r)
	}

	zero := deskkit.SummarisePerfForge([]deskkit.PerfRecord{rec("deskpr", "2026-10-11T09:00:00Z", 0)})
	if _, err := json.Marshal(zero); err != nil || zero.Tools[0].SharePct != 0 {
		t.Errorf("all-zero history: share %v, JSON error %v; want 0 and a clean encode", zero.Tools[0].SharePct, err)
	}

	v1 := deskkit.PerfRecord{Schema: perfrec.Schema, TS: "2026-10-11T09:00:00Z", Tool: "deskpr", Verb: "create", SourceSHA: "aaaa111", ForgeCalls: -7}
	if neg := deskkit.SummarisePerfForge([]deskkit.PerfRecord{v1, rec("deskpr", "2026-10-11T09:00:00Z", -3)}); neg.Measured != 0 ||
		neg.Unavailable != 2 || neg.Calls != 0 {
		t.Errorf("negative perf-v1 and perf-v2 counts = %+v; want two unavailable runs, no calls", neg)
	}
	big := rec("deskpr", "2026-10-11T09:00:00Z", math.MaxInt64)
	if o := deskkit.SummarisePerfForge([]deskkit.PerfRecord{big, big}); o.Calls != math.MaxInt64 || o.PeakHourCalls != math.MaxInt64 ||
		o.Tools[0].Calls != math.MaxInt64 || o.Releases[0].Calls != math.MaxInt64 || o.Tools[0].PeakHourCalls != math.MaxInt64 {
		t.Errorf("overflowing sums = %d total, %d peak, %d tool, %d release, %d tool peak; want each held at the int64 maximum",
			o.Calls, o.PeakHourCalls, o.Tools[0].Calls, o.Releases[0].Calls, o.Tools[0].PeakHourCalls)
	}
}

// squash collapses every run of spaces, so a tabwriter row reads as single-spaced fields.
func squash(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(strings.Join(strings.Fields(line), " "))
		b.WriteByte('\n')
	}
	return b.String()
}
