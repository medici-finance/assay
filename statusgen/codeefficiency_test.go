package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// --- test fixtures ----------------------------------------------------------

// codeFixtureCommits returns a fixture of known commits for deterministic testing.
func codeFixtureCommits() []codeCommit {
	return []codeCommit{
		{Date: mustTimeT("2026-07-08T10:00:00Z"), Added: 100, Removed: 20, Files: 3, FileList: []string{"a.go", "b.go", "c.go"}},
		{Date: mustTimeT("2026-07-09T10:00:00Z"), Added: 50, Removed: 30, Files: 2, FileList: []string{"a.go", "d.go"}},
		{Date: mustTimeT("2026-07-10T10:00:00Z"), Added: 80, Removed: 10, Files: 4, FileList: []string{"e.go", "f.go", "g.go", "h.go"}},
		{Date: mustTimeT("2026-07-11T10:00:00Z"), Added: 200, Removed: 50, Files: 5, FileList: []string{"a.go", "i.go", "j.go", "k.go", "l.go"}},
		{Date: mustTimeT("2026-07-12T10:00:00Z"), Added: 30, Removed: 40, Files: 1, FileList: []string{"e.go"}},
	}
}

// codeFixturePRs returns fixture merged PRs.
func codeFixturePRs(t *testing.T) []codePR {
	t.Helper()
	return []codePR{
		{Number: 1, MergedAt: mustTimeT("2026-07-09T00:00:00Z"), ReviewComments: 5},
		{Number: 2, MergedAt: mustTimeT("2026-07-10T00:00:00Z"), ReviewComments: 3},
		{Number: 3, MergedAt: mustTimeT("2026-07-11T00:00:00Z"), ReviewComments: 12},
		{Number: 4, MergedAt: mustTimeT("2026-07-12T00:00:00Z"), ReviewComments: 0},
	}
}

// mustTimeT parses a RFC3339 timestamp for tests.
func mustTimeT(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try date-only
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			panic(err)
		}
	}
	return t
}

// --- SLOC delta tests -------------------------------------------------------

// TestCodeSLOCDelta: verify SLOC delta computes added/removed/net per day.
func TestCodeSLOCDelta(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13") // 5 days inclusive
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:   since,
		Until:   until,
		Now:     now,
		Commits: commits,
		GitOK:   true,
	}
	rep := computeCodeEfficiency(in)

	sd := rep.Metrics[codeSLOCDelta]
	if !sd.Computed {
		t.Fatalf("SLOC delta not computed: %+v", sd)
	}
	// Total: 100+50+80+200+30 = 460 added, 20+30+10+50+40 = 150 removed
	// net = 310 over 5 days = +92.0 added, -30.0 removed, net +62.0 per day
	if !strings.Contains(sd.Value, "+92.0") || !strings.Contains(sd.Value, "-30.0") {
		t.Errorf("SLOC delta value = %q, want +92.0 added / -30.0 removed / net +62.0 per day", sd.Value)
	}
}

// --- Churn ratio tests ------------------------------------------------------

// TestCodeChurnRatio: re-touched files within 7 days produce churn.
// Commits: a.go touched at day 0 (Jul 8), day 1 (Jul 9), day 3 (Jul 11) → day 1 and day 3 both within 7 days of day 0,
// so day 1's added lines (50) and day 3's added lines (200) are churn.
// e.go touched at day 2 (Jul 10), day 4 (Jul 12) → within 7d, day 4's added lines (30) are churn.
// Total churn = 50 + 200 + 30 = 280, total added = 460, ratio = 280/460 ≈ 60.9% → 61%
func TestCodeChurnRatio(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:     since,
		Until:     until,
		Now:       now,
		Commits:   commits,
		GitOK:     true,
		ChurnDays: 7,
	}
	rep := computeCodeEfficiency(in)

	cr := rep.Metrics[codeChurnRatio]
	if !cr.Computed {
		t.Fatalf("churn ratio not computed: %+v", cr)
	}
	if !strings.Contains(cr.Value, "61%") {
		t.Errorf("churn ratio = %q, want 61%% (280/460)", cr.Value)
	}
}

// TestCodeChurnRatioNoReTouch: when no files are re-touched, churn is 0%.
func TestCodeChurnRatioNoReTouch(t *testing.T) {
	commits := []codeCommit{
		{Date: mustTimeT("2026-07-08T10:00:00Z"), Added: 100, Removed: 10, Files: 1, FileList: []string{"a.go"}},
		{Date: mustTimeT("2026-07-09T10:00:00Z"), Added: 50, Removed: 5, Files: 1, FileList: []string{"b.go"}},
		{Date: mustTimeT("2026-07-10T10:00:00Z"), Added: 30, Removed: 3, Files: 1, FileList: []string{"c.go"}},
	}
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-11")
	now := mustTimeT("2026-07-11T00:00:00Z")

	in := codeInputs{
		Since:     since,
		Until:     until,
		Now:       now,
		Commits:   commits,
		GitOK:     true,
		ChurnDays: 7,
	}
	rep := computeCodeEfficiency(in)

	cr := rep.Metrics[codeChurnRatio]
	if !cr.Computed {
		t.Fatalf("churn ratio not computed: %+v", cr)
	}
	if !strings.Contains(cr.Value, "0%") {
		t.Errorf("churn ratio = %q, want 0%% (no re-touch)", cr.Value)
	}
}

// --- Defect density tests ---------------------------------------------------

// TestCodeDefectDensity: bugs / KSLOC-changed.
// total changed = 460+150 = 610 lines = 0.61 KSLOC, 3 bugs → 3/0.61 ≈ 4.92 bugs/KSLOC
func TestCodeDefectDensity(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:     since,
		Until:     until,
		Now:       now,
		Commits:   commits,
		GitOK:     true,
		BugIssues: 3,
		GHBugsOK:  true,
	}
	rep := computeCodeEfficiency(in)

	dd := rep.Metrics[codeDefectDensity]
	if !dd.Computed {
		t.Fatalf("defect density not computed: %+v", dd)
	}
	if !strings.Contains(dd.Value, "4.92") {
		t.Errorf("defect density = %q, want 4.92 bugs/KSLOC", dd.Value)
	}
}

// --- Change spread tests ----------------------------------------------------

// TestCodeChangeSpread: median files-per-change.
// Files per commit: 3, 2, 4, 5, 1 → sorted: 1,2,3,4,5 → median 3
func TestCodeChangeSpread(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:   since,
		Until:   until,
		Now:     now,
		Commits: commits,
		GitOK:   true,
	}
	rep := computeCodeEfficiency(in)

	cs := rep.Metrics[codeChangeSpread]
	if !cs.Computed {
		t.Fatalf("change spread not computed: %+v", cs)
	}
	if !strings.Contains(cs.Value, "3 files/change") {
		t.Errorf("change spread = %q, want 3 files/change (median)", cs.Value)
	}
}

// --- Review depth tests -----------------------------------------------------

// TestCodeReviewDepth: review comments ÷ merged PRs.
// 5+3+12+0 = 20 comments / 4 PRs = 5.0 comments/PR
func TestCodeReviewDepth(t *testing.T) {
	commits := codeFixtureCommits()
	prs := codeFixturePRs(t)
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:      since,
		Until:      until,
		Now:        now,
		Commits:    commits,
		GitOK:      true,
		MergedPRs:  prs,
		GHMergedOK: true,
	}
	rep := computeCodeEfficiency(in)

	rd := rep.Metrics[codeReviewDepth]
	if !rd.Computed {
		t.Fatalf("review depth not computed: %+v", rd)
	}
	if !strings.Contains(rd.Value, "5.0") {
		t.Errorf("review depth = %q, want 5.0 comments/PR", rd.Value)
	}
}

// --- Small-n suppression tests ----------------------------------------------

// TestCodeSmallNSuppression: fewer than 3 commits suppresses SLOC delta, churn, etc.
func TestCodeSmallNSuppression(t *testing.T) {
	commits := []codeCommit{
		{Date: mustTimeT("2026-07-08T10:00:00Z"), Added: 100, Removed: 20, Files: 3, FileList: []string{"a.go"}},
		{Date: mustTimeT("2026-07-09T10:00:00Z"), Added: 50, Removed: 10, Files: 2, FileList: []string{"b.go"}},
	}
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:     since,
		Until:     until,
		Now:       now,
		Commits:   commits,
		GitOK:     true,
		BugIssues: 1,
		GHBugsOK:  true,
	}
	rep := computeCodeEfficiency(in)

	// SLOC delta suppressed
	sd := rep.Metrics[codeSLOCDelta]
	if sd.Computed {
		t.Errorf("SLOC delta should be suppressed (n=2 < 3), got computed: %q", sd.Value)
	}
	if !strings.Contains(sd.Value, "–") {
		t.Errorf("SLOC delta should show '–' for small-n, got %q", sd.Value)
	}

	// Churn ratio suppressed (only 2 commits)
	cr := rep.Metrics[codeChurnRatio]
	if cr.Computed {
		t.Errorf("churn ratio should be suppressed (n=2 < 3), got computed: %q", cr.Value)
	}

	// Defect density suppressed
	dd := rep.Metrics[codeDefectDensity]
	if dd.Computed {
		t.Errorf("defect density should be suppressed (n=2 < 3), got computed: %q", dd.Value)
	}

	// Change spread suppressed
	cs := rep.Metrics[codeChangeSpread]
	if cs.Computed {
		t.Errorf("change spread should be suppressed (n=2 < 3), got computed: %q", cs.Value)
	}
}

// TestCodeSmallNSuppressForReviewDepth: fewer than 3 merged PRs suppresses review depth.
func TestCodeSmallNSuppressForReviewDepth(t *testing.T) {
	commits := codeFixtureCommits()
	prs := []codePR{
		{Number: 1, MergedAt: mustTimeT("2026-07-09T00:00:00Z"), ReviewComments: 5},
		{Number: 2, MergedAt: mustTimeT("2026-07-10T00:00:00Z"), ReviewComments: 3},
	}
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:      since,
		Until:      until,
		Now:        now,
		Commits:    commits,
		GitOK:      true,
		MergedPRs:  prs,
		GHMergedOK: true,
	}
	rep := computeCodeEfficiency(in)

	rd := rep.Metrics[codeReviewDepth]
	if rd.Computed {
		t.Errorf("review depth should be suppressed (nPRs=2 < 3), got computed: %q", rd.Value)
	}
	if !strings.Contains(rd.Value, "–") {
		t.Errorf("review depth should show '–' for small-n, got %q", rd.Value)
	}
}

// --- F-12 posture and no-forbidden-class tests ------------------------------

// TestCodeF12HeaderPresent: every output carries the F-12 ledger-sourced posture.
func TestCodeF12HeaderPresent(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:   since,
		Until:   until,
		Now:     now,
		Commits: commits,
		GitOK:   true,
	}
	rep := computeCodeEfficiency(in)

	if rep.F12Posture == "" {
		t.Error("F12Posture field is empty — must state ledger-sourced provenance")
	}
	if !strings.Contains(rep.F12Posture, "F-12") {
		t.Errorf("F-12 posture does not mention F-12: %q", rep.F12Posture)
	}
	if !strings.Contains(rep.F12Posture, "ledger") {
		t.Errorf("F-12 posture does not mention 'ledger': %q", rep.F12Posture)
	}
	if !strings.Contains(rep.F12Posture, "leverage") {
		t.Errorf("F-12 posture does not disclaim leverage: %q", rep.F12Posture)
	}

	// Text output also includes F-12 header
	out := renderCodeText(rep)
	if !strings.Contains(out, "F-12") {
		t.Errorf("text output missing F-12 posture:\n%s", out)
	}
}

// TestCodeNoForbiddenClassEmitted: no leverage, person-day, or tier-mix figure
// appears in metric names, values, or details. The F-12 posture header may
// mention these terms as explicit disclaimers — that is its purpose.
func TestCodeNoForbiddenClassEmitted(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:   since,
		Until:   until,
		Now:     now,
		Commits: commits,
		GitOK:   true,
	}
	rep := computeCodeEfficiency(in)

	// Check each metric's value and detail for forbidden-class terms.
	// The F-12 posture field is the disclaimer — it may name them.
	forbidden := []string{"leverage", "person-day", "person_day", "tier-mix", "tier_mix"}
	for _, m := range rep.Metrics {
		combined := strings.ToLower(m.Name + " " + m.Value + " " + m.Detail)
		for _, f := range forbidden {
			if strings.Contains(combined, f) {
				t.Errorf("metric %q contains forbidden-class term %q: name=%q value=%q detail=%q",
					m.Key, f, m.Name, m.Value, m.Detail)
			}
		}
	}

	// Also verify note field doesn't carry forbidden terms (it's the Goodhart header)
	noteLower := strings.ToLower(rep.Note)
	for _, f := range forbidden {
		if strings.Contains(noteLower, f) {
			t.Errorf("note contains forbidden-class term %q: %q", f, rep.Note)
		}
	}
}

// --- JSON output tests ------------------------------------------------------

// TestCodeJSONHasFiveKeys: verify --json emits valid JSON with exactly 5 metrics.
func TestCodeJSONHasFiveKeys(t *testing.T) {
	commits := codeFixtureCommits()
	prs := codeFixturePRs(t)
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:      since,
		Until:      until,
		Now:        now,
		Commits:    commits,
		GitOK:      true,
		BugIssues:  3,
		GHBugsOK:   true,
		MergedPRs:  prs,
		GHMergedOK: true,
	}
	rep := computeCodeEfficiency(in)

	jsonBytes, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	var parsed CodeReport
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, string(jsonBytes))
	}

	if len(parsed.Metrics) != 5 {
		t.Fatalf("got %d metrics, want exactly 5", len(parsed.Metrics))
	}

	for _, k := range codeMetricKeys {
		if _, ok := parsed.Metrics[k]; !ok {
			t.Errorf("missing metric key %q", k)
		}
	}

	if parsed.Note == "" {
		t.Error("JSON missing Goodhart/anti-gaming note")
	}
	if parsed.F12Posture == "" {
		t.Error("JSON missing F-12 posture")
	}
}

// --- Text rendering tests ---------------------------------------------------

// TestCodeTextHasHeaderAndAllFive: text output includes header, F-12, 5 metrics.
func TestCodeTextHasHeaderAndAllFive(t *testing.T) {
	commits := codeFixtureCommits()
	prs := codeFixturePRs(t)
	since := mustTimeT("2026-07-08")
	until := mustTimeT("2026-07-13")
	now := mustTimeT("2026-07-13T00:00:00Z")

	in := codeInputs{
		Since:      since,
		Until:      until,
		Now:        now,
		Commits:    commits,
		GitOK:      true,
		BugIssues:  3,
		GHBugsOK:   true,
		MergedPRs:  prs,
		GHMergedOK: true,
	}
	rep := computeCodeEfficiency(in)
	out := renderCodeText(rep)

	for _, want := range []string{
		"Code-efficiency metrics",
		"F-12 posture",
		"DIAGNOSTIC",
		"SLOC delta/day",
		"Churn ratio",
		"Defect density",
		"Change spread",
		"Review depth",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q:\n%s", want, out)
		}
	}
}

// --- Series tests -----------------------------------------------------------

// TestCodeSeriesWeeklyBuckets: verify per-period bucketing correctness.
func TestCodeSeriesWeeklyBuckets(t *testing.T) {
	// Two weeks of commits
	commits := []codeCommit{
		// Week 2026-07-06 (Mon): 2 commits
		{Date: mustTimeT("2026-07-07T10:00:00Z"), Added: 100, Removed: 20, Files: 3, FileList: []string{"a.go", "b.go"}},
		{Date: mustTimeT("2026-07-08T10:00:00Z"), Added: 50, Removed: 10, Files: 2, FileList: []string{"c.go"}},
		// Week 2026-07-13 (Mon): 1 commit
		{Date: mustTimeT("2026-07-14T10:00:00Z"), Added: 200, Removed: 30, Files: 5, FileList: []string{"d.go", "e.go", "f.go"}},
	}

	bugDates := []time.Time{
		mustTimeT("2026-07-08T00:00:00Z"), // week 2026-07-06
		mustTimeT("2026-07-15T00:00:00Z"), // week 2026-07-13
	}

	prs := []codePR{
		{Number: 1, MergedAt: mustTimeT("2026-07-08T00:00:00Z"), ReviewComments: 5},
		{Number: 2, MergedAt: mustTimeT("2026-07-09T00:00:00Z"), ReviewComments: 3},
		{Number: 3, MergedAt: mustTimeT("2026-07-14T00:00:00Z"), ReviewComments: 10},
	}

	since := mustTimeT("2026-07-06")
	until := mustTimeT("2026-07-20")
	points := computeCodeSeries(since, until, "weekly", 7, commits, bugDates, prs)

	if len(points) != 3 {
		t.Fatalf("got %d points, want 3 (weeks 2026-07-06, 2026-07-13, 2026-07-20)", len(points))
	}

	// Week 2026-07-06: 2 commits, 150 added, 30 removed, net +120, 5 files, 1 bug
	p0 := points[0]
	if p0.Period != "2026-07-06" {
		t.Errorf("p0.period = %q, want 2026-07-06", p0.Period)
	}
	if p0.Added != 150 || p0.Removed != 30 || p0.Net != 120 {
		t.Errorf("p0 SLOC = %+d/+%d/%d, want 150/30/120", p0.Added, p0.Removed, p0.Net)
	}
	if p0.BugIssues != 1 {
		t.Errorf("p0 bugs = %d, want 1", p0.BugIssues)
	}
	if p0.NumCommits != 2 {
		t.Errorf("p0 commits = %d, want 2", p0.NumCommits)
	}
	if p0.NumPRs != 2 {
		t.Errorf("p0 prs = %d, want 2", p0.NumPRs)
	}
	// Churn: n=2 < 3 → suppressed
	if p0.ChurnRatio != "–" {
		t.Errorf("p0 churn = %q, want – (n=2<3)", p0.ChurnRatio)
	}
	// Defect density: n=2 < 3 → suppressed
	if p0.DefectDensity != "–" {
		t.Errorf("p0 defect density = %q, want – (n=2<3)", p0.DefectDensity)
	}
	// Review depth: n=2 < 3 PRs → suppressed
	if p0.ReviewDepth != "–" {
		t.Errorf("p0 review depth = %q, want – (n=2<3)", p0.ReviewDepth)
	}

	// Week 2026-07-13: 1 commit, 200 added, 30 removed, net +170, 5 files, 1 bug
	p1 := points[1]
	if p1.Period != "2026-07-13" {
		t.Errorf("p1.period = %q, want 2026-07-13", p1.Period)
	}
	if p1.Added != 200 || p1.Removed != 30 || p1.Net != 170 {
		t.Errorf("p1 SLOC = %+d/+%d/%d, want 200/30/170", p1.Added, p1.Removed, p1.Net)
	}
	if p1.BugIssues != 1 {
		t.Errorf("p1 bugs = %d, want 1", p1.BugIssues)
	}
	if p1.NumCommits != 1 || p1.NumPRs != 1 {
		t.Errorf("p1 commits/PRs = %d/%d, want 1/1", p1.NumCommits, p1.NumPRs)
	}

	// Week 2026-07-20: empty
	p2 := points[2]
	if p2.Added != 0 || p2.Removed != 0 || p2.Net != 0 {
		t.Errorf("p2 SLOC = %+d/+%d/%d, want all zero", p2.Added, p2.Removed, p2.Net)
	}
}

// TestCodeSeriesJSONShape: --series --json emits valid JSON array.
func TestCodeSeriesJSONShape(t *testing.T) {
	commits := codeFixtureCommits()
	bugDates := []time.Time{mustTimeT("2026-07-10T00:00:00Z")}
	prs := []codePR{
		{Number: 1, MergedAt: mustTimeT("2026-07-09T00:00:00Z"), ReviewComments: 5},
		{Number: 2, MergedAt: mustTimeT("2026-07-10T00:00:00Z"), ReviewComments: 3},
		{Number: 3, MergedAt: mustTimeT("2026-07-11T00:00:00Z"), ReviewComments: 12},
		{Number: 4, MergedAt: mustTimeT("2026-07-12T00:00:00Z"), ReviewComments: 0},
	}

	since := mustTimeT("2026-07-06")
	until := mustTimeT("2026-07-14")
	points := computeCodeSeries(since, until, "weekly", 7, commits, bugDates, prs)

	out := renderCodeSeriesJSON(points)
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatalf("JSON output does not start with '[': %s", out)
	}

	var parsed []CodeSeriesPoint
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(parsed) == 0 {
		t.Fatal("JSON array empty, want at least one period")
	}
	for _, p := range parsed {
		if p.Period == "" {
			t.Error("JSON point missing period field")
		}
	}
}

// TestCodeSeriesTextRender: text series output has header and table.
func TestCodeSeriesTextRender(t *testing.T) {
	commits := codeFixtureCommits()
	since := mustTimeT("2026-07-06")
	until := mustTimeT("2026-07-14")

	points := computeCodeSeries(since, until, "weekly", 7, commits, nil, nil)
	out := renderCodeSeriesText(points, since, until, "weekly")

	if !strings.Contains(out, "Code-efficiency time series") {
		t.Errorf("series text missing header:\n%s", out)
	}
	if !strings.Contains(out, "F-12 posture") {
		t.Errorf("series text missing F-12 posture:\n%s", out)
	}
	for _, h := range []string{"period", "added", "removed", "net", "files", "churn", "defects"} {
		if !strings.Contains(out, h) {
			t.Errorf("series text missing column header %q", h)
		}
	}
}

// TestCodeRunRejectsBadSince: malformed --since exits non-zero.
func TestCodeRunRejectsBadSince(t *testing.T) {
	if code := runCode("testdata/dorarepo", "not-a-date", false, false); code == 0 {
		t.Error("expected non-zero exit on malformed --since")
	}
}

// TestCodeRunRejectsFutureSince: future --since exits non-zero.
func TestCodeRunRejectsFutureSince(t *testing.T) {
	if code := runCode("testdata/dorarepo", "2099-01-01", false, false); code == 0 {
		t.Error("expected non-zero exit on future --since")
	}
}

// --- Churn boundary test ----------------------------------------------------

// TestCodeChurnDaysBoundary: churn at exactly N days counts, N+1 days does not.
func TestCodeChurnDaysBoundary(t *testing.T) {
	// Commits exactly 7 days apart (re-touch within 7d window)
	commits := []codeCommit{
		{Date: mustTimeT("2026-07-01T10:00:00Z"), Added: 100, Removed: 10, Files: 1, FileList: []string{"a.go"}},
		{Date: mustTimeT("2026-07-08T10:00:00Z"), Added: 50, Removed: 5, Files: 1, FileList: []string{"a.go"}}, // exactly 7 days → churn
		{Date: mustTimeT("2026-07-16T10:00:00Z"), Added: 30, Removed: 3, Files: 1, FileList: []string{"a.go"}}, // 8 days after Jul 8 → NOT churn (in 7d window)
	}

	since := mustTimeT("2026-07-01")
	until := mustTimeT("2026-07-20")
	now := mustTimeT("2026-07-20T00:00:00Z")

	in := codeInputs{
		Since:     since,
		Until:     until,
		Now:       now,
		Commits:   commits,
		GitOK:     true,
		ChurnDays: 7,
	}
	rep := computeCodeEfficiency(in)

	cr := rep.Metrics[codeChurnRatio]
	if !cr.Computed {
		t.Fatalf("churn ratio not computed: %+v", cr)
	}
	// Jul 8 commit (50 added) re-touches a.go from Jul 1 → 50 churn
	// Jul 16 commit (30 added) re-touches a.go from Jul 8, but that's 8 days > 7 → NOT churn
	// Total churn = 50, total added = 180, ratio = 50/180 ≈ 28%
	if !strings.Contains(cr.Value, "28%") {
		t.Errorf("churn ratio = %q, want 28%% (50 churn / 180 total)", cr.Value)
	}
}
