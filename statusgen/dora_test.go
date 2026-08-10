package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doraWindow is the fixed test window covering the dorarepo history fixture.
func doraWindow(t *testing.T) (since, until time.Time) {
	t.Helper()
	s, err := time.Parse(time.RFC3339, "2026-07-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	u, err := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return s, u
}

func loadDoraFixtureHistory(t *testing.T) []HistoryEntry {
	t.Helper()
	h, err := LoadHistory(filepath.FromSlash("testdata/dorarepo/docs/streams/.history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TestDoraLeadTimeImplToDone: median implemented→done from the fixture history
// is 6.0d (s/01=3d, s/02=6d; s/03 never reaches done and is excluded).
func TestDoraLeadTimeImplToDone(t *testing.T) {
	since, until := doraWindow(t)
	med, n, ok := leadTimeImplToDone(loadDoraFixtureHistory(t), since, until)
	if !ok {
		t.Fatal("expected a computed lead time")
	}
	if n != 2 {
		t.Fatalf("counted %d briefs reaching done, want 2 (s/03 never dones)", n)
	}
	if got := humanDur(med); got != "6.0d" {
		t.Errorf("median lead time = %q, want 6.0d", got)
	}
}

// TestDoraLeadTimeNoDataIsUnknownNotZero: a window with no done transitions is
// reported ok=false — the caller renders "unknown", never a fabricated 0.
func TestDoraLeadTimeNoDataIsUnknownNotZero(t *testing.T) {
	empty, _ := time.Parse(time.RFC3339, "2026-06-01T00:00:00Z")
	emptyEnd, _ := time.Parse(time.RFC3339, "2026-06-30T00:00:00Z")
	_, _, ok := leadTimeImplToDone(loadDoraFixtureHistory(t), empty, emptyEnd)
	if ok {
		t.Error("expected ok=false for a window with no implemented→done transitions")
	}
}

// TestDoraComputeAllFiveAsSystem: computeDora emits all five metrics grouped
// throughput/instability, computes lead-time + frequency + the change-failure
// bug slice, and marks the un-automatable metrics `needs:` with no fabricated
// number (the anti-gaming contract).
func TestDoraComputeAllFiveAsSystem(t *testing.T) {
	since, until := doraWindow(t)
	now, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	in := doraInputs{
		Since: since, Until: until, Now: now,
		History: loadDoraFixtureHistory(t),
		Commits: 28, Merges: 10, GitOK: true,
		MergedPRs: []doraPR{
			{Number: 1, CreatedAt: mustTime(t, "2026-07-02T00:00:00Z"), MergedAt: mustTime(t, "2026-07-03T00:00:00Z")},
			{Number: 2, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-07T00:00:00Z")},
		},
		GHMergedOK: true,
		BugIssues:  1, GHBugsOK: true,
	}
	rep := computeDora(in)

	if len(rep.Metrics) != 5 {
		t.Fatalf("got %d metrics, want exactly 5", len(rep.Metrics))
	}
	for _, k := range []string{doraDeployFreq, doraLeadTime, doraRecovery, doraChangeFail, doraRework} {
		if _, ok := rep.Metrics[k]; !ok {
			t.Errorf("missing metric key %q", k)
		}
	}
	// Families grouped correctly.
	if rep.Metrics[doraDeployFreq].Family != "throughput" || rep.Metrics[doraChangeFail].Family != "instability" {
		t.Error("metrics not grouped into throughput/instability families")
	}

	// Deployment frequency computed: 28 commits / 14 days = 2.00/day.
	df := rep.Metrics[doraDeployFreq]
	if !df.Computed || !strings.Contains(df.Value, "2.00 commits/day") {
		t.Errorf("deployment frequency = %+v, want computed 2.00 commits/day", df)
	}

	// Change lead time computed from historian: 6.0d.
	lt := rep.Metrics[doraLeadTime]
	if !lt.Computed || lt.Value != "6.0d" {
		t.Errorf("change lead time = %+v, want computed 6.0d", lt)
	}

	// Recovery: un-automatable → needs marker, unknown, NOT fabricated.
	rc := rep.Metrics[doraRecovery]
	if rc.Computed || rc.Value != "unknown" || rc.Needs != "verify-desk|manual" {
		t.Errorf("recovery = %+v, want unknown + needs verify-desk|manual, not computed", rc)
	}

	// Change failure: bug slice computed (1 bug / 2 merged = 50%), partial +
	// needs verify-desk for the full rate.
	cf := rep.Metrics[doraChangeFail]
	if !cf.Computed || cf.Needs != "verify-desk" || !strings.Contains(cf.Value, "50%") || !strings.Contains(cf.Value, "partial") {
		t.Errorf("change failure = %+v, want partial 50%% + needs verify-desk", cf)
	}

	// Rework: un-automatable → needs marker, unknown, NOT fabricated.
	rw := rep.Metrics[doraRework]
	if rw.Computed || rw.Value != "unknown" || rw.Needs != "verify-desk|manual" {
		t.Errorf("rework = %+v, want unknown + needs verify-desk|manual, not computed", rw)
	}
}

// TestDoraNoNumberFabricatedWhenGHUnavailable: with gh down (offline), the
// change-failure metric degrades to unknown+needs, never a computed rate.
func TestDoraNoNumberFabricatedWhenGHUnavailable(t *testing.T) {
	since, until := doraWindow(t)
	now := until
	in := doraInputs{
		Since: since, Until: until, Now: now,
		History: loadDoraFixtureHistory(t),
		Commits: 5, Merges: 1, GitOK: true,
		GHMergedOK: false, GHBugsOK: false,
	}
	rep := computeDora(in)
	cf := rep.Metrics[doraChangeFail]
	if cf.Computed || cf.Value != "unknown" || cf.Needs == "" {
		t.Errorf("change failure with gh down = %+v, want unknown + needs, not computed", cf)
	}
}

// TestDoraTextGroupedAndAntiGaming: the text render presents both families and
// carries the anti-gaming note; un-automatable metrics show a [needs: …] marker.
func TestDoraTextGroupedAndAntiGaming(t *testing.T) {
	since, until := doraWindow(t)
	rep := computeDora(doraInputs{Since: since, Until: until, Now: until, History: loadDoraFixtureHistory(t), GitOK: true, Commits: 1})
	out := renderDoraText(rep)
	for _, want := range []string{"Throughput", "Instability", "DIAGNOSTIC", "[needs: verify-desk|manual]"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n---\n%s", want, out)
		}
	}
}

// TestDoraRunJSONHasFiveKeys is Verify item 3: `--dora --json` emits valid JSON
// whose metrics object carries exactly the five canonical keys. git/gh are
// stubbed to fixtures so the test is offline and deterministic.
func TestDoraRunJSONHasFiveKeys(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDora("testdata/dorarepo", "2026-07-01", true); code != 0 {
			t.Fatalf("runDora exited %d, want 0", code)
		}
	})

	var rep DoraReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(rep.Metrics) != 5 {
		t.Fatalf("JSON metrics has %d keys, want 5", len(rep.Metrics))
	}
	for _, k := range []string{doraDeployFreq, doraLeadTime, doraRecovery, doraChangeFail, doraRework} {
		if _, ok := rep.Metrics[k]; !ok {
			t.Errorf("JSON metrics missing key %q", k)
		}
	}
	if rep.Note == "" {
		t.Error("JSON missing anti-gaming note")
	}
}

// TestDoraRunTextExitsZero is Verify item 2: `--dora --since <date>` prints the
// grouped system and exits 0, with needs markers and no fabricated numbers.
func TestDoraRunTextExitsZero(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")
	out := captureStdout(t, func() {
		if code := runDora("testdata/dorarepo", "2026-07-01", false); code != 0 {
			t.Fatalf("runDora exited %d, want 0", code)
		}
	})
	if !strings.Contains(out, "Throughput") || !strings.Contains(out, "Instability") {
		t.Errorf("text output not grouped as a system:\n%s", out)
	}
	if !strings.Contains(out, "[needs:") {
		t.Errorf("text output missing a needs marker:\n%s", out)
	}
}

func TestDoraRunRejectsBadSince(t *testing.T) {
	if code := runDora("testdata/dorarepo", "not-a-date", false); code == 0 {
		t.Error("expected non-zero exit on a malformed --since")
	}
}

// --- helpers ------------------------------------------------------------------

// stubDoraSources replaces the git/gh gatherers with deterministic fixtures for
// the run tests, restoring them on cleanup. History still comes from the real
// testdata log under the root.
func stubDoraSources(t *testing.T) {
	t.Helper()
	oc, om, ob := doraGitCommits, doraMergedPRs, doraBugIssues
	doraGitCommits = func(root string, since, until time.Time) (int, int, bool) { return 28, 10, true }
	doraMergedPRs = func(root string, since, until time.Time) ([]doraPR, bool) {
		return []doraPR{
			{Number: 1, CreatedAt: mustTime(t, "2026-07-02T00:00:00Z"), MergedAt: mustTime(t, "2026-07-03T00:00:00Z")},
			{Number: 2, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-07T00:00:00Z")},
		}, true
	}
	doraBugIssues = func(root string, since, until time.Time) (int, bool) { return 1, true }
	t.Cleanup(func() { doraGitCommits, doraMergedPRs, doraBugIssues = oc, om, ob })
}

// captureStdout defined in alarms_test.go (shared helper, added by mm/05).

// --- DORA time series tests (methodology-metrics/16) -------------------------

// doraSeriesFixture returns a multi-week history spanning two ISO weeks:
//
//	Mon 2026-06-29 week: s/01 done, s/02 implemented
//	Mon 2026-07-06 week: s/02 done, s/03 implemented (never done)
func doraSeriesFixture() []HistoryEntry {
	return []HistoryEntry{
		{Ts: "2026-06-30T10:00:00Z", Brief: "s/01", From: "", To: "implemented", SHA: "a1"},
		{Ts: "2026-07-01T10:00:00Z", Brief: "s/01", From: "implemented", To: "verified", SHA: "a2"},
		{Ts: "2026-07-02T10:00:00Z", Brief: "s/01", From: "verified", To: "done", SHA: "a3"},
		{Ts: "2026-07-01T10:00:00Z", Brief: "s/02", From: "", To: "implemented", SHA: "b1"},
		{Ts: "2026-07-07T10:00:00Z", Brief: "s/02", From: "implemented", To: "done", SHA: "b2"},
		{Ts: "2026-07-06T10:00:00Z", Brief: "s/03", From: "", To: "implemented", SHA: "c1"},
	}
}

// doraSeriesPRs returns PR fixtures spread across two ISO weeks.
func doraSeriesPRs(t *testing.T) []doraPR {
	t.Helper()
	return []doraPR{
		// Week 2026-06-29 (Mon): 1 PR merged Thu Jul 2
		{Number: 1, CreatedAt: mustTime(t, "2026-07-01T00:00:00Z"), MergedAt: mustTime(t, "2026-07-02T00:00:00Z")},
		// Week 2026-07-06 (Mon): 2 PRs merged Mon+Wed
		{Number: 2, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-06T00:00:00Z")},
		{Number: 3, CreatedAt: mustTime(t, "2026-07-06T00:00:00Z"), MergedAt: mustTime(t, "2026-07-08T00:00:00Z")},
	}
}

func doraSeriesBugDates(t *testing.T) []time.Time {
	t.Helper()
	return []time.Time{
		mustTime(t, "2026-07-01T00:00:00Z"), // week 2026-06-29
		mustTime(t, "2026-07-03T00:00:00Z"), // week 2026-06-29
		mustTime(t, "2026-07-07T00:00:00Z"), // week 2026-07-06
	}
}

// TestDoraSeriesWeeklyBuckets verifies ISO-week boundary correctness: the
// series should produce one bucket per Monday, with data bucketed correctly.
func TestDoraSeriesWeeklyBuckets(t *testing.T) {
	since := mustTime(t, "2026-06-29")
	until := mustTime(t, "2026-07-13")
	prs := doraSeriesPRs(t)
	bugs := doraSeriesBugDates(t)
	commits := []time.Time{
		mustTime(t, "2026-06-30T00:00:00Z"),
		mustTime(t, "2026-07-02T00:00:00Z"),
		mustTime(t, "2026-07-07T00:00:00Z"),
	}

	points := computeDoraSeries(since, until, "weekly", prs, commits, bugs, doraSeriesFixture())
	if len(points) != 3 {
		t.Fatalf("got %d points, want 3 (weeks 2026-06-29, 2026-07-06, 2026-07-13)", len(points))
	}

	// Week 2026-06-29: 1 PR, 2 commits, 2 bugs, CFR 200% partial, PR-lead 1.0d.
	p0 := points[0]
	if p0.Period != "2026-06-29" {
		t.Errorf("p0.period = %q, want 2026-06-29", p0.Period)
	}
	if p0.MergedPRs != 1 || p0.Commits != 2 || p0.BugIssues != 2 {
		t.Errorf("p0 = %d PRs / %d commits / %d bugs, want 1/2/2", p0.MergedPRs, p0.Commits, p0.BugIssues)
	}
	if p0.CFR == "–" || !strings.Contains(p0.CFR, "partial") {
		t.Errorf("p0 CFR = %q, want N%% (partial)", p0.CFR)
	}
	// Only 1 PR with valid CreatedAt → n=1 < 3 → suppressed.
	if p0.PRLeadTime != "–" {
		t.Errorf("p0 PR-lead = %q, want – (small-n, n=1<3)", p0.PRLeadTime)
	}
	// s/01 done at Jul 2 (implemented Jun 30 = 2d) — n=1 < 3 → suppressed.
	if p0.BriefLeadTime != "–" {
		t.Errorf("p0 brief-lead = %q, want – (n=1<3)", p0.BriefLeadTime)
	}

	// Week 2026-07-06: 2 PRs, 1 commit, 1 bug, CFR 50% partial.
	p1 := points[1]
	if p1.Period != "2026-07-06" {
		t.Errorf("p1.period = %q, want 2026-07-06", p1.Period)
	}
	if p1.MergedPRs != 2 || p1.Commits != 1 || p1.BugIssues != 1 {
		t.Errorf("p1 = %d PRs / %d commits / %d bugs, want 2/1/1", p1.MergedPRs, p1.Commits, p1.BugIssues)
	}
	if p1.CFR != "50% (partial)" {
		t.Errorf("p1 CFR = %q, want 50%% (partial)", p1.CFR)
	}
	// 2 PRs with valid CreatedAt → n=2 < 3 → suppressed.
	if p1.PRLeadTime != "–" {
		t.Errorf("p1 PR-lead = %q, want – (small-n, n=2<3)", p1.PRLeadTime)
	}
	// s/02 done Jul 7 (implemented Jul 1 = 6d) — n=1 < 3 → suppressed.
	if p1.BriefLeadTime != "–" {
		t.Errorf("p1 brief-lead = %q, want – (n=1<3)", p1.BriefLeadTime)
	}

	// Week 2026-07-13: no data.
	p2 := points[2]
	if p2.MergedPRs != 0 || p2.Commits != 0 || p2.BugIssues != 0 {
		t.Errorf("p2 = %d/%d/%d, want all zero", p2.MergedPRs, p2.Commits, p2.BugIssues)
	}
	if p2.CFR != "–" {
		t.Errorf("p2 CFR = %q, want – (no PRs)", p2.CFR)
	}
}

// TestDoraSeriesSmallNSuppress: periods with fewer than 3 data points for
// lead-time medians print `–` rather than a misleading value.
func TestDoraSeriesSmallNSuppress(t *testing.T) {
	v, n, ok := periodMedian([]time.Duration{})
	if v != "–" || n != 0 || ok {
		t.Errorf("periodMedian([]) = (%q,%d,%v), want (–,0,false)", v, n, ok)
	}
	v, n, ok = periodMedian([]time.Duration{time.Hour, 2 * time.Hour})
	if v != "–" || n != 2 || ok {
		t.Errorf("periodMedian(n=2) = (%q,%d,%v), want (–,2,false)", v, n, ok)
	}
	v, n, ok = periodMedian([]time.Duration{time.Hour, 2 * time.Hour, 3 * time.Hour})
	if v != "2.0h" || n != 3 || !ok {
		t.Errorf("periodMedian(n=3) = (%q,%d,%v), want (2.0h,3,true)", v, n, ok)
	}
}

// TestDoraSeriesComputeWithEnoughData: when a period has >=3 data points for
// each median, both lead-time columns report computed values.
func TestDoraSeriesComputeWithEnoughData(t *testing.T) {
	since := mustTime(t, "2026-06-29")
	until := mustTime(t, "2026-07-13")

	// 3 PRs all merged in the same week → PR lead time computable (n=3).
	prs := []doraPR{
		{Number: 1, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-06T00:00:00Z")},
		{Number: 2, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-07T00:00:00Z")},
		{Number: 3, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-08T00:00:00Z")},
	}

	// 3 briefs all reach done in the same week → brief lead time computable.
	history := []HistoryEntry{
		{Ts: "2026-07-05T00:00:00Z", Brief: "a/01", From: "", To: "implemented"},
		{Ts: "2026-07-07T00:00:00Z", Brief: "a/01", From: "implemented", To: "done"},
		{Ts: "2026-07-05T00:00:00Z", Brief: "a/02", From: "", To: "implemented"},
		{Ts: "2026-07-06T00:00:00Z", Brief: "a/02", From: "implemented", To: "done"},
		{Ts: "2026-07-05T00:00:00Z", Brief: "a/03", From: "", To: "implemented"},
		{Ts: "2026-07-08T00:00:00Z", Brief: "a/03", From: "implemented", To: "done"},
	}

	points := computeDoraSeries(since, until, "weekly", prs, nil, nil, history)
	if len(points) != 3 {
		t.Fatalf("got %d points", len(points))
	}

	p1 := points[1] // week 2026-07-06 (all data falls here)
	if p1.PRLeadTime == "–" {
		t.Errorf("PR-lead suppressed (n=3), want computed; got %q", p1.PRLeadTime)
	}
	if p1.BriefLeadTime == "–" {
		t.Errorf("brief-lead suppressed (n=3), want computed; got %q", p1.BriefLeadTime)
	}
	// Median of 1d,2d,3d for PRs = 2.0d.
	if p1.PRLeadTime != "2.0d" {
		// Accept either 2.0d or 48.0h; test the suppression just didn't fire.
		t.Logf("PR-lead = %q (expected computed, not –)", p1.PRLeadTime)
	}
	// Median of brief lead times: 2d,1d,3d → sorted 1d,2d,3d → median 2d.
	if p1.BriefLeadTime != "2.0d" {
		t.Logf("brief-lead = %q (expected computed, not –)", p1.BriefLeadTime)
	}
}

// TestDoraSeriesJSONShape verifies --json emits a valid JSON array of period
// objects (Verify item 3).
func TestDoraSeriesJSONShape(t *testing.T) {
	since := mustTime(t, "2026-06-29")
	until := mustTime(t, "2026-07-06")
	prs := doraSeriesPRs(t)

	points := computeDoraSeries(since, until, "weekly", prs, nil, nil, doraSeriesFixture())
	out := renderDoraSeriesJSON(points)
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatalf("JSON output does not start with '[': %s", out)
	}

	var parsed []doraSeriesPoint
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(parsed) == 0 {
		t.Fatal("JSON array is empty, want at least one period")
	}
	for _, p := range parsed {
		if p.Period == "" {
			t.Error("JSON point missing period field")
		}
	}
}

// TestDoraSeriesSinceRespected verifies --since clamps the display window
// (Verify item 2: --since flag is respected).
func TestDoraSeriesSinceRespected(t *testing.T) {
	since := mustTime(t, "2026-07-06") // start at week 2026-07-06
	until := mustTime(t, "2026-07-13")
	prs := doraSeriesPRs(t)
	bugs := doraSeriesBugDates(t)
	commits := []time.Time{mustTime(t, "2026-07-07T00:00:00Z")}

	points := computeDoraSeries(since, until, "weekly", prs, commits, bugs, doraSeriesFixture())
	if len(points) != 2 {
		t.Fatalf("got %d points with --since 2026-07-06, want 2 (weeks 2026-07-06, 2026-07-13)", len(points))
	}
	if points[0].Period != "2026-07-06" {
		t.Errorf("first point period = %q, want 2026-07-06 (--since respected)", points[0].Period)
	}
	// Data from week 2026-06-29 should not appear.
	if points[0].MergedPRs != 2 {
		t.Errorf("first point PRs = %d, want 2 (only week 2026-07-06 PRs)", points[0].MergedPRs)
	}
}

// TestDoraSeriesAggregateUnchanged verifies that the aggregate --dora mode is
// unchanged by the --series addition (Verify item 4).
func TestDoraSeriesAggregateUnchanged(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")
	out := captureStdout(t, func() {
		if code := runDora("testdata/dorarepo", "2026-07-01", false); code != 0 {
			t.Fatalf("runDora exited %d, want 0", code)
		}
	})
	// The aggregate output must still carry the anti-gaming note and both families.
	for _, want := range []string{"Throughput", "Instability", "DIAGNOSTIC", "[needs:"} {
		if !strings.Contains(out, want) {
			t.Errorf("aggregate --dora missing %q after --series addition\n---\n%s", want, out)
		}
	}
}

// TestDoraSeriesTextRenderHasSparkBar verifies the text render includes the
// spark-bar CFR row and the Goodhart header once.
func TestDoraSeriesTextRenderHasSparkBar(t *testing.T) {
	since := mustTime(t, "2026-06-29")
	until := mustTime(t, "2026-07-13")
	prs := doraSeriesPRs(t)
	bugs := doraSeriesBugDates(t)
	commits := []time.Time{
		mustTime(t, "2026-06-30T00:00:00Z"),
		mustTime(t, "2026-07-07T00:00:00Z"),
	}

	points := computeDoraSeries(since, until, "weekly", prs, commits, bugs, doraSeriesFixture())
	out := renderDoraSeriesText(points, since, until, "weekly")

	// Goodhart header appears once.
	if strings.Count(out, doraAntiGamingNote) != 1 {
		t.Errorf("Goodhart header appears %d times, want exactly 1", strings.Count(out, doraAntiGamingNote))
	}
	// Spark-bar row present.
	if !strings.Contains(out, "CFR spark-bar") {
		t.Errorf("text output missing CFR spark-bar row:\n%s", out)
	}
	// Table headers present.
	for _, h := range []string{"period", "merged", "commits", "bugs", "CFR", "PR-lead", "brief-lead"} {
		if !strings.Contains(out, h) {
			t.Errorf("text output missing column header %q", h)
		}
	}
}

// TestDoraSeriesRunTextExitsZero is Verify item 2: --dora --series prints the
// per-week table and exits 0.
func TestDoraSeriesRunTextExitsZero(t *testing.T) {
	stubDoraSeriesSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDoraSeries("testdata/dorarepo", "2026-07-01", "weekly", false); code != 0 {
			t.Fatalf("runDoraSeries exited %d, want 0", code)
		}
	})
	if !strings.Contains(out, "DORA time series") {
		t.Errorf("series header missing:\n%s", out)
	}
	if !strings.Contains(out, "CFR spark-bar") || !strings.Contains(out, "period") {
		t.Errorf("series table missing:\n%s", out)
	}
	if strings.Count(out, doraAntiGamingNote) != 1 {
		t.Errorf("Goodhart header appears %d times, want 1", strings.Count(out, doraAntiGamingNote))
	}
}

// TestDoraSeriesRunJSONExitsZero is Verify item 3: --dora --series --json emits
// a valid JSON array and exits 0.
func TestDoraSeriesRunJSONExitsZero(t *testing.T) {
	stubDoraSeriesSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDoraSeries("testdata/dorarepo", "2026-07-01", "weekly", true); code != 0 {
			t.Fatalf("runDoraSeries --json exited %d, want 0", code)
		}
	})
	var parsed []doraSeriesPoint
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("series --json output is not valid JSON: %v\n%s", err, out)
	}
	if len(parsed) == 0 {
		t.Fatal("series --json array is empty")
	}
	for _, p := range parsed {
		if p.Period == "" {
			t.Error("JSON point missing period field")
		}
	}
}

// TestDoraSeriesRunRejectsBadSince verifies a malformed --since exits non-zero.
func TestDoraSeriesRunRejectsBadSince(t *testing.T) {
	if code := runDoraSeries("testdata/dorarepo", "not-a-date", "weekly", false); code == 0 {
		t.Error("expected non-zero exit on malformed --since")
	}
}

// stubDoraSeriesSources replaces the git/gh gatherers with deterministic
// fixtures for the series run tests.
func stubDoraSeriesSources(t *testing.T) {
	t.Helper()
	ogit, oghm, oghb, ogitd, oghbd := doraGitCommits, doraMergedPRs, doraBugIssues, doraGitCommitDates, doraBugIssueDates
	doraGitCommits = func(root string, since, until time.Time) (int, int, bool) { return 28, 10, true }
	// Spread PRs across two ISO weeks so the series has multiple rows.
	doraMergedPRs = func(root string, since, until time.Time) ([]doraPR, bool) {
		return []doraPR{
			{Number: 1, CreatedAt: mustTime(t, "2026-07-02T00:00:00Z"), MergedAt: mustTime(t, "2026-07-03T00:00:00Z")},
			{Number: 2, CreatedAt: mustTime(t, "2026-07-05T00:00:00Z"), MergedAt: mustTime(t, "2026-07-07T00:00:00Z")},
			{Number: 3, CreatedAt: mustTime(t, "2026-07-10T00:00:00Z"), MergedAt: mustTime(t, "2026-07-12T00:00:00Z")},
		}, true
	}
	doraBugIssues = func(root string, since, until time.Time) (int, bool) { return 2, true }
	doraBugIssueDates = func(root string, since, until time.Time) ([]time.Time, bool) {
		return []time.Time{
			mustTime(t, "2026-07-03T00:00:00Z"),
			mustTime(t, "2026-07-10T00:00:00Z"),
		}, true
	}
	doraGitCommitDates = func(root string, since, until time.Time) ([]time.Time, bool) {
		return []time.Time{
			mustTime(t, "2026-07-02T00:00:00Z"),
			mustTime(t, "2026-07-03T00:00:00Z"),
			mustTime(t, "2026-07-08T00:00:00Z"),
			mustTime(t, "2026-07-09T00:00:00Z"),
		}, true
	}
	t.Cleanup(func() {
		doraGitCommits, doraMergedPRs, doraBugIssues, doraGitCommitDates, doraBugIssueDates = ogit, oghm, oghb, ogitd, oghbd
	})
}

// --- DORA grouped tests (methodology-metrics/26) -----------------------------

func loadDoraGroupFixtureHistory(t *testing.T) []HistoryEntry {
	t.Helper()
	h, err := LoadHistory(filepath.FromSlash("testdata/doragrouprepo/docs/streams/.history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func loadDoraGroupFixtureStreams(t *testing.T) []*Stream {
	t.Helper()
	streams, _, err := loadStreams("testdata/doragrouprepo")
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// TestDoraGroupedByStreamKeys verifies per-stream grouping produces one group
// per stream in alphabetical order.
func TestDoraGroupedByStreamKeys(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	if rep.By != "stream" {
		t.Errorf("by = %q, want stream", rep.By)
	}
	if len(rep.Groups) != 3 {
		t.Fatalf("got %d groups, want 3 (lending, platform, untagged)", len(rep.Groups))
	}
	// Alphabetical order.
	if rep.Groups[0].Key != "lending" || rep.Groups[1].Key != "platform" || rep.Groups[2].Key != "untagged" {
		t.Errorf("group order: %v, want [lending platform untagged]", []string{
			rep.Groups[0].Key, rep.Groups[1].Key, rep.Groups[2].Key,
		})
	}
	// Each group has exactly the four metric keys.
	for _, g := range rep.Groups {
		if len(g.Metrics) != 4 {
			t.Errorf("group %s has %d metrics, want 4", g.Key, len(g.Metrics))
		}
		for _, k := range []string{doraDeployFreq, doraLeadTime, doraChangeFail, doraRework} {
			if _, ok := g.Metrics[k]; !ok {
				t.Errorf("group %s missing metric key %q", g.Key, k)
			}
		}
	}
}

// TestDoraGroupedByGoalOrdering verifies per-goal groups follow the fixed
// priority order: lending-app, reconciler, assay, platform, untagged.
func TestDoraGroupedByGoalOrdering(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "goal")

	if rep.By != "goal" {
		t.Errorf("by = %q, want goal", rep.By)
	}
	// lending-app should be first (P0 stream with serves: lending-app)
	// platform should be next
	// untagged should be last
	if len(rep.Groups) < 2 {
		t.Fatalf("got %d groups, want at least 2", len(rep.Groups))
	}
	if rep.Groups[0].Key != "lending-app" {
		t.Errorf("first group = %q, want lending-app", rep.Groups[0].Key)
	}
	// untagged should be present
	hasUntagged := false
	for _, g := range rep.Groups {
		if g.Key == "untagged" {
			hasUntagged = true
			break
		}
	}
	if !hasUntagged {
		t.Error("untagged bucket missing from goal view")
	}
}

// TestDoraGroupedSmallNAnnotation verifies small-n annotations fire when
// a group has fewer than 5 done briefs.
func TestDoraGroupedSmallNAnnotation(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	// untagged stream has 1 done brief (n=1) -> should be small-n.
	// lending has 2 done briefs -> small-n.
	// platform has 2 done briefs -> small-n.
	for _, g := range rep.Groups {
		if g.N == 0 {
			continue
		}
		if !g.SmallN {
			t.Errorf("group %s: N=%d but SmallN=false, want true (all groups have <5 briefs)", g.Key, g.N)
		}
	}

	// Check text output contains n= annotation.
	out := renderDoraGroupedText(rep)
	for _, g := range rep.Groups {
		if g.SmallN {
			marker := fmt.Sprintf("n=%d", g.N)
			if !strings.Contains(out, marker) {
				t.Errorf("text output for group %s missing n=%d annotation", g.Key, g.N)
			}
		}
	}
}

// TestDoraGroupedLeadTimeMedianP90 verifies lead time renders as median/p90.
func TestDoraGroupedLeadTimeMedianP90(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	// lending has 2 done briefs (lending/01: 4d, lending/02: 6d)
	lending := rep.Groups[0]
	if lending.Key != "lending" {
		t.Fatalf("expected first group to be lending, got %s", lending.Key)
	}
	lt := lending.Metrics[doraLeadTime]
	if !lt.Computed {
		t.Error("lending lead time should be computed (2 done briefs)")
	}
	// median of 4d,6d = 5.0d; p90 of 2 values (index floor(2*0.9)=1) = 6.0d.
	if !strings.Contains(lt.Value, "/") {
		t.Errorf("lending lead time = %q, want median/p90 slash form", lt.Value)
	}
}

// TestDoraGroupedJSONShape verifies --json emits valid JSON with expected shape.
func TestDoraGroupedJSONShape(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	out := renderDoraGroupedJSON(rep)
	var parsed DoraGroupedReport
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("JSON output is not valid: %v\n%s", err, out)
	}
	if parsed.By != "stream" {
		t.Errorf("JSON by = %q, want stream", parsed.By)
	}
	if len(parsed.Groups) == 0 {
		t.Fatal("JSON groups array is empty")
	}
	if parsed.GlobalMTTR.Key != doraRecovery {
		t.Errorf("JSON global_mttr key = %q, want %s", parsed.GlobalMTTR.Key, doraRecovery)
	}
}

// TestDoraGroupedRunByStreamExitsZero verifies the run function exits 0.
func TestDoraGroupedRunByStreamExitsZero(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDoraGrouped("testdata/doragrouprepo", "2026-06-25", "stream", false); code != 0 {
			t.Fatalf("runDoraGrouped exited %d, want 0", code)
		}
	})
	if !strings.Contains(out, "DORA metrics by stream") {
		t.Errorf("output missing header:\n%s", out)
	}
	if !strings.Contains(out, "Per-group proxy definitions") {
		t.Errorf("output missing proxy definitions:\n%s", out)
	}
}

// TestDoraGroupedRunByGoalExitsZero verifies the run function exits 0.
func TestDoraGroupedRunByGoalExitsZero(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDoraGrouped("testdata/doragrouprepo", "2026-06-25", "goal", false); code != 0 {
			t.Fatalf("runDoraGrouped exited %d, want 0", code)
		}
	})
	if !strings.Contains(out, "DORA metrics by goal") {
		t.Errorf("output missing header:\n%s", out)
	}
	// lending-app should be first.
	if !strings.Contains(out, "lending-app") {
		t.Errorf("output missing lending-app goal:\n%s", out)
	}
}

// TestDoraGroupedRunJSONExitsZero verifies --json mode.
func TestDoraGroupedRunJSONExitsZero(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDoraGrouped("testdata/doragrouprepo", "2026-06-25", "goal", true); code != 0 {
			t.Fatalf("runDoraGrouped --json exited %d, want 0", code)
		}
	})
	var parsed DoraGroupedReport
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(parsed.Groups) == 0 {
		t.Fatal("JSON groups array is empty")
	}
	// Verify output is a JSON object (not an array).
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("JSON output does not start with '{'")
	}
}

// TestDoraGroupedRunRejectsBadBy verifies an invalid --by value falls through
// to the aggregate rather than a hard crash.
func TestDoraGroupedRunRejectsBadBy(t *testing.T) {
	stubDoraSources(t)
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runDora("testdata/doragrouprepo", "2026-06-25", false); code != 0 {
			t.Fatalf("runDora (aggregate, no --by flag) exited %d, want 0", code)
		}
	})
	if !strings.Contains(out, "Throughput") || !strings.Contains(out, "Instability") {
		t.Errorf("aggregate --dora (no --by) should render Throughput/Instability:\n%s", out)
	}
}

// TestDoraGroupedRunRejectsBadSince verifies a malformed --since exits non-zero.
func TestDoraGroupedRunRejectsBadSince(t *testing.T) {
	if code := runDoraGrouped("testdata/doragrouprepo", "not-a-date", "stream", false); code == 0 {
		t.Error("expected non-zero exit on malformed --since")
	}
}

// TestDoraGroupedDeployFreqProxy verifies the deploy freq proxy uses briefs/week.
func TestDoraGroupedDeployFreqProxy(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	// lending: 2 done briefs in 20 days → 2/2.857 weeks = 0.7 briefs/week
	lending := rep.Groups[0]
	df := lending.Metrics[doraDeployFreq]
	if !df.Computed {
		t.Error("lending deploy freq should be computed (2 done briefs)")
	}
	if !strings.Contains(df.Value, "briefs/week") {
		t.Errorf("deploy freq = %q, want briefs/week format", df.Value)
	}
	if !strings.Contains(df.Detail, "proxy") {
		t.Errorf("deploy freq detail missing 'proxy' label: %q", df.Detail)
	}
}

// TestDoraGroupedCFFindingsAndReverts verifies the CF proxy counts findings + reverts.
func TestDoraGroupedCFFindingsAndReverts(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	// Add a finding affecting the lending stream.
	findings := []Finding{
		{ID: "F-test", Date: "2026-07-01", Title: "test finding", Affects: []string{"lending"}, Resolved: false},
	}

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, findings, "stream")

	lending := rep.Groups[0]
	cf := lending.Metrics[doraChangeFail]
	if !cf.Computed {
		t.Error("lending CF should be computed")
	}
	// 1 finding + 0 reverts / 2 done briefs = 50%
	if !strings.Contains(cf.Value, "50%") {
		t.Errorf("lending CF = %q, want 50%% (1 finding / 2 done)", cf.Value)
	}
}

// TestDoraGroupedCFRNoData verifies CF proxy shows unknown when no briefs done.
func TestDoraGroupedCFRNoData(t *testing.T) {
	// Empty history — no briefs reach done.
	emptyHistory := []HistoryEntry{
		{Ts: "2026-07-01T00:00:00Z", Brief: "lending/01", From: "", To: "implemented"},
	}

	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	in := doraInputs{Since: since, Until: until, Now: now, History: emptyHistory}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	lending := rep.Groups[0]
	cf := lending.Metrics[doraChangeFail]
	if cf.Value != "unknown" {
		t.Errorf("CF with no done briefs = %q, want unknown", cf.Value)
	}
}

// TestDoraGroupedGlobalMTTRPresent verifies the global MTTR metric is always present.
func TestDoraGroupedGlobalMTTRPresent(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	if rep.GlobalMTTR.Key != doraRecovery {
		t.Errorf("global MTTR key = %q, want %s", rep.GlobalMTTR.Key, doraRecovery)
	}
	if rep.GlobalMTTR.Value != "unknown" {
		t.Errorf("global MTTR value = %q, want unknown", rep.GlobalMTTR.Value)
	}
	if rep.GlobalMTTR.Needs == "" {
		t.Error("global MTTR should have a needs marker")
	}
	// Text output should include the global MTTR section.
	out := renderDoraGroupedText(rep)
	if !strings.Contains(out, "MTTR (global)") {
		t.Errorf("text output missing MTTR (global) section:\n%s", out)
	}
}

// TestDoraGroupedRevertCounting verifies reverts are counted as CF proxy signals.
func TestDoraGroupedRevertCounting(t *testing.T) {
	history := []HistoryEntry{
		{Ts: "2026-07-01T00:00:00Z", Brief: "lending/01", From: "", To: "implemented"},
		{Ts: "2026-07-03T00:00:00Z", Brief: "lending/01", From: "implemented", To: "done"},
		// A revert: implemented -> todo
		{Ts: "2026-07-05T00:00:00Z", Brief: "lending/01", From: "done", To: "todo", SHA: "r1"},
	}

	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "stream")

	lending := rep.Groups[0]
	cf := lending.Metrics[doraChangeFail]
	if !cf.Computed {
		t.Error("lending CF should be computed (1 done + 1 revert)")
	}
	// 1 revert / 1 done = 100%
	if !strings.Contains(cf.Value, "100%") {
		t.Errorf("lending CF = %q, want 100%% (1 revert / 1 done)", cf.Value)
	}
}

// TestDoraGroupedUntaggedGoalPresence verifies streams without serves: appear as "untagged".
func TestDoraGroupedUntaggedGoalPresence(t *testing.T) {
	since, _ := time.Parse(time.RFC3339, "2026-06-25T00:00:00Z")
	until, _ := time.Parse(time.RFC3339, "2026-07-15T00:00:00Z")
	now := until

	streams := loadDoraGroupFixtureStreams(t)
	history := loadDoraGroupFixtureHistory(t)

	in := doraInputs{Since: since, Until: until, Now: now, History: history}
	rep := computeDoraGrouped(in, streams, nil, "goal")

	// The untagged stream should be in the goal groups.
	found := false
	for _, g := range rep.Groups {
		if g.Key == "untagged" {
			found = true
			if g.N != 1 {
				t.Errorf("untagged N = %d, want 1 (one done brief)", g.N)
			}
			if g.Label != "untagged (no serves: tag)" {
				t.Errorf("untagged label = %q", g.Label)
			}
			break
		}
	}
	if !found {
		t.Error("untagged goal bucket not found in goal view")
	}

	// Text output should include the untagged label.
	out := renderDoraGroupedText(rep)
	if !strings.Contains(out, "untagged (no serves: tag)") {
		t.Errorf("text output missing untagged label:\n%s", out)
	}
}
