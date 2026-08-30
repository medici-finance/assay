package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- resolveBFWindow ---------------------------------------------------------

func TestResolveBFWindow_Defaults(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since, until, err := resolveBFWindow("", "", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !until.Equal(now) {
		t.Errorf("until = %v, want now (%v)", until, now)
	}
	wantSince := now.AddDate(0, 0, -defaultDoraWindowDays)
	if !since.Equal(wantSince) {
		t.Errorf("since = %v, want %v (default window)", since, wantSince)
	}
}

func TestResolveBFWindow_Explicit(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since, until, err := resolveBFWindow("2026-08-01", "2026-08-20", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !since.Equal(mustTime(t, "2026-08-01T00:00:00Z")) || !until.Equal(mustTime(t, "2026-08-20T00:00:00Z")) {
		t.Errorf("got since=%v until=%v", since, until)
	}
}

func TestResolveBFWindow_SinceAfterUntil_Errors(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	if _, _, err := resolveBFWindow("2026-08-20", "2026-08-01", now); err == nil {
		t.Fatal("expected an error when --since is after --until")
	}
}

func TestResolveBFWindow_BadDateErrors(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	if _, _, err := resolveBFWindow("not-a-date", "", now); err == nil {
		t.Fatal("expected an error for a malformed --since")
	}
	if _, _, err := resolveBFWindow("", "not-a-date", now); err == nil {
		t.Fatal("expected an error for a malformed --until")
	}
}

// --- metric 1: weighted throughput ------------------------------------------

func bfStream(name, status string, briefs ...Brief) *Stream {
	return &Stream{Name: name, Status: status, Briefs: briefs}
}

func TestCompute_ThroughputWeightsAnd_Segments(t *testing.T) {
	streams := []*Stream{
		bfStream("s", "active",
			Brief{Num: "01", Effort: "S", Status: "done"},
			Brief{Num: "02", Effort: "M", Status: "done"},
			Brief{Num: "03", Effort: "L", Status: "done"},
		),
		bfStream(scanStreamName, "active",
			Brief{Num: "issue-100", Effort: "M", Status: "done", Schema: "placeholder-v1"},
		),
	}
	history := []HistoryEntry{
		{Brief: "s/01", From: "implemented", To: "done", Ts: "2026-08-10T00:00:00Z"},
		{Brief: "s/02", From: "implemented", To: "done", Ts: "2026-08-12T00:00:00Z"},
		{Brief: "s/03", From: "implemented", To: "done", Ts: "2026-08-15T00:00:00Z"},
		{Brief: scanStreamName + "/issue-100", From: "implemented", To: "done", Ts: "2026-08-14T00:00:00Z"},
		// outside the window — must not be counted.
		{Brief: "s/01", From: "implemented", To: "done", Ts: "2026-01-01T00:00:00Z"},
	}
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-20T00:00:00Z")

	rep := computeThroughput(streams, history, since, until)

	// Correctness core: 1 + 3 + 8 = 12 authored points; issue-loop = 3 (one M).
	if rep.Authored.Points != 12 {
		t.Errorf("authored points = %v, want 12 (S=1+M=3+L=8)", rep.Authored.Points)
	}
	if rep.Authored.Count != 3 {
		t.Errorf("authored count = %d, want 3", rep.Authored.Count)
	}
	if rep.IssueLoop.Points != 3 {
		t.Errorf("issue-loop points = %v, want 3 (one M)", rep.IssueLoop.Points)
	}
	if rep.IssueLoop.Count != 1 {
		t.Errorf("issue-loop count = %d, want 1", rep.IssueLoop.Count)
	}
	if rep.Total.Points != 15 {
		t.Errorf("total points = %v, want 15", rep.Total.Points)
	}
	if rep.State != "ok" {
		t.Errorf("state = %q, want ok", rep.State)
	}
}

func TestCompute_ThroughputUnknown_EffortNever_FabricatesAZero(t *testing.T) {
	// Brief "s/09" completed but no longer exists on the board (removed after
	// archival) — its effort cannot be resolved. It must be counted as
	// unknown, NOT silently scored as 0 points.
	streams := []*Stream{bfStream("s", "active", Brief{Num: "01", Effort: "S", Status: "done"})}
	history := []HistoryEntry{
		{Brief: "s/01", To: "done", Ts: "2026-08-10T00:00:00Z"},
		{Brief: "s/09", To: "done", Ts: "2026-08-11T00:00:00Z"},
	}
	rep := computeThroughput(streams, history, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.Authored.UnknownEffort != 1 {
		t.Errorf("unknown_effort = %d, want 1", rep.Authored.UnknownEffort)
	}
	if rep.Authored.Points != 1 {
		t.Errorf("points = %v, want 1 (only s/01 counted)", rep.Authored.Points)
	}
}

// A window with no `to:"done"` events at all must render could-not-check, not
// a fabricated zero — the three-state discipline brief-07's facts require.
func TestCompute_ThroughputEmpty_WindowIsCouldNot_Check(t *testing.T) {
	streams := []*Stream{bfStream("s", "active", Brief{Num: "01", Effort: "S", Status: "todo"})}
	rep := computeThroughput(streams, nil, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check", rep.State)
	}
}

// --- metric 2: lead time by size --------------------------------------------

func TestParseAuthored_Date(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"2026-08-26", "2026-08-26", true},
		{"2026-08-26 (re-authored clean for the statusgen board)", "2026-08-26", true},
		{"", "", false},
		{"not a date", "", false},
	}
	for _, c := range cases {
		got, ok := parseAuthoredDate(c.raw)
		if ok != c.ok {
			t.Errorf("parseAuthoredDate(%q) ok=%v, want %v", c.raw, ok, c.ok)
			continue
		}
		if ok && got.Format("2006-01-02") != c.want {
			t.Errorf("parseAuthoredDate(%q) = %v, want %s", c.raw, got, c.want)
		}
	}
}

func TestPctlDaysNearest_Rank(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5}
	// 0.85 * 5 = 4.25 -> truncated index 4 -> value 5 (nearest-rank convention,
	// same indexing doratiming.go's pctlHours uses).
	if got := pctlDays(vals, 0.85); got != 5 {
		t.Errorf("p85 = %v, want 5", got)
	}
}

// medianDays is the TRUE median (averages the two central values on an even
// count) — deliberately NOT pctlDays(0.5)'s nearest-rank index. bottleneck.go
// made the identical distinction after a real bug (its own comment: "the old
// buggy upper-median... masking this").
func TestMedianDaysTrue_MedianEvenCount(t *testing.T) {
	if got := medianDays([]float64{5, 10}); got != 7.5 {
		t.Errorf("median of [5,10] = %v, want 7.5 (average of the two central values)", got)
	}
	if got := medianDays([]float64{1, 2, 3, 4, 5}); got != 3 {
		t.Errorf("median of [1..5] = %v, want 3", got)
	}
}

// authoredFixtureRepo builds an on-disk repo with one authored stream carrying
// real brief-v1 files (loadAuthoredBriefInfo reads Authored directly off the
// file — it is never hydrated onto the Brief README row) plus the historian
// log lead time reads against.
func authoredFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "s")
	mustMkdirAll(t, dir)
	writeTemp(t, dir, "README.md", strings.Join([]string{
		"---", "stream: s", "status: active", "priority: P1", "track: product", "---", "",
		"# S", "",
		"| # | Brief | Wave | Status | Verified | Reviewed |",
		"|---|-------|------|--------|----------|----------|",
		"| 01 | brief-01 | 1 | done | — | — |",
		"| 02 | brief-02 | 1 | done | — | — |",
		"",
	}, "\n"))
	briefFM := func(id, effort, authored string) string {
		return strings.Join([]string{
			"---",
			"brief: " + id,
			"title: T",
			"wave: 1",
			"depends: []",
			"unblocks: []",
			"effort: " + effort,
			"gate: model",
			"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}",
			"issues: []",
			"schema: brief-v1",
			"authored: " + authored,
			"sources: []",
			"---", "", "# " + id, "",
		}, "\n")
	}
	// Trailing prose after the date is deliberate, matching every real
	// authored: line in this tree (0 of them are a bare date): a bare
	// YYYY-MM-DD scalar decodes to a YAML timestamp, not a string, and
	// brieffile.go's parseBriefFile only assigns bf.Authored from a string
	// value — a bare date would silently leave Authored empty.
	writeTemp(t, dir, "brief-01-a.md", briefFM("s/01", "S", "2026-08-01 (scoping session)"))
	writeTemp(t, dir, "brief-02-b.md", briefFM("s/02", "S", "2026-08-05 (re-authored)"))
	return root
}

func TestComputeLeadTime_BySizeMedianAndN(t *testing.T) {
	root := authoredFixtureRepo(t)
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatalf("loadHydratedStreams: %v", err)
	}
	history := []HistoryEntry{
		{Brief: "s/01", To: "done", Ts: "2026-08-06T00:00:00Z"}, // authored 08-01 -> 5 days
		{Brief: "s/02", To: "done", Ts: "2026-08-15T00:00:00Z"}, // authored 08-05 -> 10 days
	}
	rep := computeLeadTimeBySize(streams, history, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	s := rep.BySize["S"]
	if s.N != 2 {
		t.Fatalf("S.n = %d, want 2", s.N)
	}
	if s.MedianDays != 7.5 {
		t.Errorf("S.median_days = %v, want 7.5", s.MedianDays)
	}
	if rep.State != "ok" {
		t.Errorf("state = %q, want ok", rep.State)
	}
	// Buckets with no data render could-not-check, never a fabricated n=0 pass.
	if m := rep.BySize["M"]; m.State != "could-not-check" || m.N != 0 {
		t.Errorf("M bucket = %+v, want could-not-check/n=0", m)
	}
}

func TestComputeLeadTime_BySizeNoDoneEventsIs_CouldNotCheck(t *testing.T) {
	root := authoredFixtureRepo(t)
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatalf("loadHydratedStreams: %v", err)
	}
	rep := computeLeadTimeBySize(streams, nil, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check", rep.State)
	}
	for _, size := range []string{"S", "M", "L"} {
		if rep.BySize[size].State != "could-not-check" {
			t.Errorf("%s bucket state = %q, want could-not-check", size, rep.BySize[size].State)
		}
	}
}

// --- metric 7: per-stream net flow + stall ----------------------------------

func TestComputeNetFlow_ArrivalsCompletions_AndStall(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	streams := []*Stream{
		bfStream("active-stalled", "active",
			Brief{Num: "01", Status: "todo"},
		),
		bfStream("active-fresh", "active",
			Brief{Num: "01", Status: "in-progress"},
		),
		bfStream("done-stream", "active"), // no backlog -> never stalled
	}
	history := []HistoryEntry{
		// active-stalled: last transition 20 days ago -> stale, backlog>0, active -> stalled.
		{Brief: "active-stalled/01", From: "", To: "todo", Ts: "2026-08-06T00:00:00Z"},
		// active-fresh: transition 1 day ago -> not stale.
		{Brief: "active-fresh/01", From: "", To: "todo", Ts: "2026-08-06T00:00:00Z"},
		{Brief: "active-fresh/01", From: "todo", To: "in-progress", Ts: "2026-08-25T00:00:00Z"},
	}
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-26T00:00:00Z")
	rep := computeNetFlow(streams, history, since, until, now)

	byName := map[string]bfStreamFlow{}
	for _, r := range rep.Streams {
		byName[r.Stream] = r
	}
	as := byName["active-stalled"]
	if as.Arrivals != 1 || as.Completions != 0 || as.NetFlow != 1 {
		t.Errorf("active-stalled flow = %+v", as)
	}
	if !as.Stalled {
		t.Error("active-stalled should be stalled (no transition in >=14d, active, backlog>0)")
	}
	af := byName["active-fresh"]
	if af.Stalled {
		t.Error("active-fresh should NOT be stalled (transition 1 day ago)")
	}
	ds := byName["done-stream"]
	if ds.Stalled {
		t.Error("done-stream has no backlog and must never be stalled")
	}
}

func TestComputeNetFlowNo_StreamsIsCouldNot_Check(t *testing.T) {
	rep := computeNetFlow(nil, nil, mustTime(t, "2026-08-01T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"), mustTime(t, "2026-08-20T00:00:00Z"))
	if rep.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check", rep.State)
	}
}

// --- no-default-probe: the gh-reading modes announce their target ------------

// countingPRSource is a bfPRSource that reaches no network — it records that it
// was called so a test can assert the announcement came FIRST.
type countingPRSource struct{ calls int }

func (s *countingPRSource) MergedPRs(repo string, since time.Time) ([]bfPRRecord, error) {
	s.calls++
	return nil, nil
}
func (s *countingPRSource) Reviews(repo string, pr int) ([]ghReview, error) { return nil, nil }

// TestBFResolveTarget_Announces: bfResolveTarget must name the repo it resolved,
// on STDERR, before the caller makes contact. These modes resolve their target
// implicitly ($GITHUB_REPOSITORY, git remote, gh default), so an opt-in flag on
// its own still leaves an operator run from the wrong directory unable to tell
// WHICH repo's queue was read. Stderr, not stdout, because each mode also has a
// --json shape an announcement would corrupt.
func TestBFResolveTarget_Announces(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "example-org/example-repo")
	var got string
	errOut := captureStderr(t, func() { got = bfResolveTarget("decision-latency", t.TempDir()) })
	if got != "example-org/example-repo" {
		t.Fatalf("resolved repo = %q, want the env target", got)
	}
	if !strings.Contains(errOut, "decision-latency") {
		t.Errorf("announcement must name the MODE that reached the network:\n%s", errOut)
	}
	if !strings.Contains(errOut, "example-org/example-repo") {
		t.Errorf("announcement must name the resolved TARGET:\n%s", errOut)
	}
}

// TestBFResolveTarget_Unresolved: an unresolvable target is announced too. The
// caller turns it into could-not-check, and a SILENT could-not-check reads as
// "nothing found" rather than "nowhere to look".
func TestBFResolveTarget_Unresolved(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	// A bare temp dir has no git remote, and an emptied PATH means the
	// `gh repo view` fallback cannot resolve one either — so the no-target
	// branch is reachable in every environment, not just an unauthenticated one.
	t.Setenv("PATH", "")
	var got string
	errOut := captureStderr(t, func() { got = bfResolveTarget("review-rework", t.TempDir()) })
	if got != "" {
		t.Fatalf("resolved repo = %q, want empty (no target resolvable)", got)
	}
	if !strings.Contains(errOut, "review-rework") || !strings.Contains(errOut, "could-not-check") {
		t.Errorf("an unresolved target must be announced as could-not-check:\n%s", errOut)
	}
}

// TestRunModesAnnounce_BeforeContact: the gh-reading run functions print their
// target BEFORE the first source call — the disclosure is at contact time, not
// merely somewhere in the run. --first-pass-yield shares the same helper but
// needs a hydrated root, so it is covered by the helper tests above.
func TestRunModesAnnounce_BeforeContact(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "example-org/example-repo")
	dir := t.TempDir()

	dq := &countingDecisionSource{}
	out := captureStderr(t, func() { runDecisionLatency(dir, "2026-08-01", "2026-08-20", true, dq) })
	if !strings.Contains(out, "decision-latency: querying example-org/example-repo") {
		t.Errorf("decision-latency did not announce its target on stderr:\n%s", out)
	}
	if dq.calls == 0 {
		t.Error("decision-latency never reached its source — the announcement was not at contact time")
	}

	pr := &countingPRSource{}
	out = captureStderr(t, func() { runReviewRework(dir, "2026-08-01", "2026-08-20", true, pr) })
	if !strings.Contains(out, "review-rework: querying example-org/example-repo") {
		t.Errorf("review-rework did not announce its target on stderr:\n%s", out)
	}
	if pr.calls == 0 {
		t.Error("review-rework never reached its source — the announcement was not at contact time")
	}
}
