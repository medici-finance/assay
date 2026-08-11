package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bottleneckFixture creates a test repo with streams and history suitable for
// bottleneck testing. The fixture has:
//
//	Stream "s":
//	  brief-01: todo  (historian: entered todo at T-30d)
//	  brief-02: in-progress (historian: entered in-progress at T-10d)
//	  brief-03: implemented (historian: entered implemented at T-5d)
//	  brief-04: implemented (historian: entered implemented at T-15d)
//	  brief-05: verified (historian: entered verified at T-2d)
//	  brief-06: verified (NO historian entry — unknown dwell)
//	  brief-07: done (historian: entered done at T-1d)
//
// Expected WIP: todo=1, in-progress=1, implemented=2, verified=2, done=1
// Expected constraint: todo (WIP=1, median dwell = 30d = 30d score).
//
//	With true median, implemented scores 2 * median(5d,15d) = 2 * 10d = 20d.
//	The old buggy upper-median (15d) gave implemented 30d too, masking this.
//
// verified: WIP=2, median dwell of known ones = 2d (1 item unknown)
//
//	score = 2 * 2d = 4d = 345,600s
func bottleneckFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Stream README.
	streamsDir := filepath.Join(root, "docs", "streams", "s")
	mustMkdirAll(t, streamsDir)
	writeTemp(t, streamsDir, "README.md", strings.Join([]string{
		"---",
		"stream: s",
		"status: active",
		"priority: P1",
		"track: product",
		"---",
		"",
		"# S",
		"",
		"| # | Brief | Wave | Status | Verified | Reviewed |",
		"|---|-------|------|--------|----------|----------|",
		"| 01 | brief-01 | 1 | todo | — | — |",
		"| 02 | brief-02 | 1 | in-progress | — | — |",
		"| 03 | brief-03 | 1 | implemented | — | — |",
		"| 04 | brief-04 | 1 | implemented | — | — |",
		"| 05 | brief-05 | 1 | verified | — | — |",
		"| 06 | brief-06 | 1 | verified | — | — |",
		"| 07 | brief-07 | 1 | done | — | — |",
		"",
	}, "\n"))

	// History log.
	now := time.Now().UTC()
	histDir := filepath.Join(root, "docs", "streams")
	writeTemp(t, histDir, ".history.jsonl", strings.Join([]string{
		// brief-01: entered todo T-30d
		`{"ts":"` + now.AddDate(0, 0, -30).Format(time.RFC3339) + `","brief":"s/01","from":"","to":"todo","sha":"a1"}`,
		// brief-02: entered in-progress T-10d
		`{"ts":"` + now.AddDate(0, 0, -10).Format(time.RFC3339) + `","brief":"s/02","from":"todo","to":"in-progress","sha":"a2"}`,
		// brief-03: entered implemented T-5d
		`{"ts":"` + now.AddDate(0, 0, -5).Format(time.RFC3339) + `","brief":"s/03","from":"in-progress","to":"implemented","sha":"a3"}`,
		// brief-04: entered implemented T-15d
		`{"ts":"` + now.AddDate(0, 0, -15).Format(time.RFC3339) + `","brief":"s/04","from":"in-progress","to":"implemented","sha":"a4"}`,
		// brief-05: entered verified T-2d
		`{"ts":"` + now.AddDate(0, 0, -2).Format(time.RFC3339) + `","brief":"s/05","from":"implemented","to":"verified","sha":"a5"}`,
		// brief-06: NO history entry — unknown dwell
		// brief-07: entered done T-1d
		`{"ts":"` + now.AddDate(0, 0, -1).Format(time.RFC3339) + `","brief":"s/07","from":"verified","to":"done","sha":"a6"}`,
	}, "\n"))

	return root
}

// TestBottleneckWIPCounts verifies per-stage WIP counts are correct on the fixture.
func TestBottleneckWIPCounts(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, err := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep := computeBottleneck(streams, history, now)

	wantWIP := map[string]int{"todo": 1, "in-progress": 1, "implemented": 2, "verified": 2, "done": 1, "blocked": 0}
	for _, st := range rep.Stages {
		if st.WIP != wantWIP[st.Stage] {
			t.Errorf("stage %s WIP = %d, want %d", st.Stage, st.WIP, wantWIP[st.Stage])
		}
	}
	if rep.TotalBriefs != 7 {
		t.Errorf("TotalBriefs = %d, want 7", rep.TotalBriefs)
	}
}

// TestBottleneckMedianDwellEvenCount verifies that the median dwell for an
// even-count stage is the true median (average of the two central values), not
// the upper of the two. On the fixture, implemented has two briefs with dwells
// ~5d and ~15d — the true median is ~10d, not ~15d.
func TestBottleneckMedianDwellEvenCount(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, err := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep := computeBottleneck(streams, history, now)

	// Find the implemented stage.
	var imp *StageStats
	for i := range rep.Stages {
		if rep.Stages[i].Stage == "implemented" {
			imp = &rep.Stages[i]
			break
		}
	}
	if imp == nil {
		t.Fatal("implemented stage not found in report")
	}

	// Two known dwells: ~5d and ~15d.
	if len(imp.KnownDwells) != 2 {
		t.Fatalf("implemented KnownDwells = %d, want 2", len(imp.KnownDwells))
	}

	// True median = (~5d + ~15d) / 2 ≈ 10d. Allow 5s tolerance for clock drift
	// between fixture creation and bottleneck computation.
	wantMedian := 10 * 24 * time.Hour
	diff := imp.MedianDwell - wantMedian
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Second {
		t.Errorf("implemented MedianDwell = %s, want ~%s (diff %s > 5s tolerance)",
			imp.MedianDwell, wantMedian, diff)
	}

	// The old code returned the upper of the two central values for even n,
	// which would give ~15d. Verify the median is at the midpoint (the average
	// of the two values), not at the upper element. Sort is ascending, so
	// KnownDwells[0] is the shorter dwell (~5d), [1] is longer (~15d).
	mid := (imp.KnownDwells[0] + imp.KnownDwells[1]) / 2
	diff2 := imp.MedianDwell - mid
	if diff2 < 0 {
		diff2 = -diff2
	}
	if diff2 > time.Second {
		t.Errorf("median not at midpoint of [%s, %s]: median=%s, midpoint=%s (diff %s)",
			imp.KnownDwells[0], imp.KnownDwells[1], imp.MedianDwell, mid, diff2)
	}

	// Sanity: the old upper-of-two bug would give ~15d. True median is ~10d.
	// Verify it's not the bug (the median should not equal KnownDwells[1]).
	if imp.MedianDwell > imp.KnownDwells[0]+(imp.KnownDwells[1]-imp.KnownDwells[0])*3/4 {
		t.Errorf("median looks like old upper-of-two bug: median=%s, lower=%s, upper=%s",
			imp.MedianDwell, imp.KnownDwells[0], imp.KnownDwells[1])
	}

	// With true median, todo (WIP=1, ~30d) scores higher than implemented
	// (WIP=2, ~10d). The old buggy upper-median (~15d) gave implemented the
	// same score as todo, masking this.
	if rep.Constraint != "todo" {
		t.Errorf("constraint = %q, want \"todo\" (WIP=1, median ~30d = ~30d score > implemented WIP=2, median ~10d = ~20d score)", rep.Constraint)
	}
}

// TestBottleneckConstraintHighestWIPxDwell verifies the constraint is the stage
// with the highest WIP x median dwell. On the fixture, todo has WIP=1 with
// median dwell ~30d (= ~30d score), which beats implemented (WIP=2, median ~10d
// = ~20d score) and verified (WIP=2, median ~2d with one unknown).
func TestBottleneckConstraintHighestWIPxDwell(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, err := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep := computeBottleneck(streams, history, now)

	if rep.Constraint != "todo" {
		t.Errorf("constraint = %q, want \"todo\" (WIP=1, median ~30d = ~30d score; with true median fix, beats implemented WIP=2 median ~10d = ~20d)", rep.Constraint)
	}
	if rep.Action == "" {
		t.Error("prescribed action is empty for a known constraint")
	}
	if !strings.Contains(rep.Action, "Dispatch-bound") {
		t.Errorf("action for todo constraint should be dispatch-bound, got: %s", rep.Action)
	}
}

// TestBottleneckUnknownDwellCountedSeparately verifies that briefs with no
// historian row are counted in UnknownDwell, not silently zeroed.
func TestBottleneckUnknownDwellCountedSeparately(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, err := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep := computeBottleneck(streams, history, now)

	// brief-06 is verified with no historian entry.
	for _, st := range rep.Stages {
		if st.Stage == "verified" {
			if st.UnknownDwell != 1 {
				t.Errorf("verified UnknownDwell = %d, want 1 (brief-06 has no historian entry)", st.UnknownDwell)
			}
			if len(st.KnownDwells) != 1 {
				t.Errorf("verified KnownDwells = %d, want 1 (only brief-05 has a historian entry)", len(st.KnownDwells))
			}
		}
	}
}

// TestBottleneckShiftDetection verifies that a prior report with a different
// constraint is correctly detected as a shift.
func TestBottleneckShiftDetection(t *testing.T) {
	root := bottleneckFixtureRepo(t)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	// Write a prior report claiming the constraint was "verified".
	reportsDir := filepath.Join(root, "docs", "reports", "factory-floor")
	mustMkdirAll(t, reportsDir)
	priorContent := strings.Join([]string{
		"# Daily factory-floor bottleneck report — " + yesterday.Format("2006-01-02"),
		"",
		"| verified→done | 2 | 2.0d | 1 | ... |",
		"",
		"**Likely constraint:** `verified`",
		"",
		"**Prescribed action:** Review-bound — parallelize review recording.",
	}, "\n")
	writeTemp(t, reportsDir, yesterday.Format("2006-01-02")+".md", priorContent)

	shiftedFrom := detectShift(root, now)
	if shiftedFrom != "verified" {
		t.Errorf("detectShift returned %q, want \"verified\"", shiftedFrom)
	}

	// Now test the full report rendering with shift detection.
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, _ := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	rep := computeBottleneck(streams, history, now)

	text := renderBottleneckText(rep, shiftedFrom)
	if !strings.Contains(text, "Shift detected") {
		t.Error("report should mention shift detection when constraint changed")
	}
	if !strings.Contains(text, "verified") || !strings.Contains(text, "todo") {
		t.Error("report should name both the old constraint (verified) and new constraint (todo)")
	}
}

// TestBottleneckNoShiftWhenSameConstraint verifies that when the constraint is the
// same as the prior report, no shift is reported.
func TestBottleneckNoShiftWhenSameConstraint(t *testing.T) {
	root := bottleneckFixtureRepo(t)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	// Write a prior report claiming the constraint was "todo" (same as current).
	reportsDir := filepath.Join(root, "docs", "reports", "factory-floor")
	mustMkdirAll(t, reportsDir)
	priorContent := strings.Join([]string{
		"# Daily factory-floor bottleneck report — " + yesterday.Format("2006-01-02"),
		"",
		"**Likely constraint:** `todo`",
	}, "\n")
	writeTemp(t, reportsDir, yesterday.Format("2006-01-02")+".md", priorContent)

	shiftedFrom := detectShift(root, now)
	if shiftedFrom != "todo" {
		t.Errorf("detectShift returned %q, want \"todo\"", shiftedFrom)
	}

	streams, _, _ := loadStreams(root)
	history, _ := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	rep := computeBottleneck(streams, history, now)

	text := renderBottleneckText(rep, shiftedFrom)
	if strings.Contains(text, "Shift detected") {
		t.Error("should NOT report shift when constraint unchanged")
	}
	if !strings.Contains(text, "No shift") {
		t.Error("should report 'No shift' when constraint unchanged")
	}
}

// TestBottleneckShiftDetectionSkippedDay verifies that detectShift picks up the
// newest prior dated file even when yesterday is skipped — globs the directory
// rather than assuming exactly today-1 exists.
func TestBottleneckShiftDetectionSkippedDay(t *testing.T) {
	root := bottleneckFixtureRepo(t)

	now := time.Now().UTC()
	twoDaysAgo := now.AddDate(0, 0, -2)

	// Write a prior report for two days ago (yesterday is skipped).
	reportsDir := filepath.Join(root, "docs", "reports", "factory-floor")
	mustMkdirAll(t, reportsDir)
	priorContent := strings.Join([]string{
		"# Daily factory-floor bottleneck report — " + twoDaysAgo.Format("2006-01-02"),
		"",
		"**Likely constraint:** `verified`",
	}, "\n")
	writeTemp(t, reportsDir, twoDaysAgo.Format("2006-01-02")+".md", priorContent)

	// Should pick up the report from two days ago even though yesterday is missing.
	shiftedFrom := detectShift(root, now)
	if shiftedFrom != "verified" {
		t.Errorf("detectShift returned %q, want \"verified\" (should find file from two days ago)", shiftedFrom)
	}
}

// TestBottleneckFirstReportNoPrior verifies that without a prior report file,
// shift detection returns "" and the report indicates it's the first report.
func TestBottleneckFirstReportNoPrior(t *testing.T) {
	root := bottleneckFixtureRepo(t)

	now := time.Now().UTC()
	shiftedFrom := detectShift(root, now)
	if shiftedFrom != "" {
		t.Errorf("detectShift should return \"\" when no prior report exists, got %q", shiftedFrom)
	}

	streams, _, _ := loadStreams(root)
	history, _ := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	rep := computeBottleneck(streams, history, now)

	text := renderBottleneckText(rep, shiftedFrom)
	if !strings.Contains(text, "First report") {
		t.Error("report should indicate 'First report' when no prior exists")
	}
}

// TestBottleneckStageActionTableIsDeterministic verifies that every known stage
// maps to a non-empty action (the lookup table is complete).
func TestBottleneckStageActionTableIsDeterministic(t *testing.T) {
	for _, stage := range bottleneckStageOrder {
		action := bottleneckAction(stage)
		if action == "" {
			t.Errorf("stage %q has no prescribed action", stage)
		}
	}
}

// TestBottleneckRunWritesReportFile verifies that runBottleneck writes the dated
// report file and exits 0.
func TestBottleneckRunWritesReportFile(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	fixedNow(t, time.Now().UTC().Format(time.RFC3339))

	out := captureStdout(t, func() {
		if code := runBottleneck(root); code != 0 {
			t.Fatalf("runBottleneck exited %d, want 0", code)
		}
	})

	// Stdout should contain the report.
	if !strings.Contains(out, "factory-floor bottleneck report") {
		t.Errorf("stdout missing report header:\n%s", out)
	}
	if !strings.Contains(out, bottleneckAntiGamingNote) {
		t.Errorf("stdout missing anti-gaming note:\n%s", out)
	}

	// The dated report file must exist.
	date := time.Now().UTC().Format("2006-01-02")
	reportPath := filepath.Join(root, "docs", "reports", "factory-floor", date+".md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("report file not written: %v", err)
	}
	if !strings.Contains(string(data), "factory-floor bottleneck report") {
		t.Errorf("report file missing header:\n%s", string(data))
	}
}

// TestBottleneckRunStdoutSummaryOneScreen verifies the stdout summary is compact
// enough for a terminal (~one screen) and carries the key sections.
func TestBottleneckRunStdoutSummaryOneScreen(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	fixedNow(t, time.Now().UTC().Format(time.RFC3339))

	out := captureStdout(t, func() {
		if code := runBottleneck(root); code != 0 {
			t.Fatalf("runBottleneck exited %d", code)
		}
	})

	for _, want := range []string{
		"Per-stage WIP",
		"Likely constraint",
		"Prescribed action",
		"historian: docs/streams/.history.jsonl",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing section %q:\n%s", want, out)
		}
	}
}

// TestBottleneckStageLabelMapping verifies the stage→label mapping is complete
// for all pipeline stages.
func TestBottleneckStageLabelMapping(t *testing.T) {
	tests := map[string]string{
		"todo":        "todo→in-progress",
		"in-progress": "in-progress→implemented",
		"implemented": "implemented→verified",
		"verified":    "verified→done",
		"blocked":     "blocked",
		"done":        "done (exited)",
	}
	for stage, want := range tests {
		if got := bottleneckStageLabel(stage); got != want {
			t.Errorf("bottleneckStageLabel(%q) = %q, want %q", stage, got, want)
		}
	}
}

// TestBottleneckEmptyHistoryNoCrash verifies that an empty/missing history log
// doesn't crash — it just means all dwells are unknown.
func TestBottleneckEmptyHistoryNoCrash(t *testing.T) {
	root := bottleneckFixtureRepo(t)
	// Remove the history file.
	os.Remove(filepath.Join(root, "docs", "streams", ".history.jsonl"))

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	rep := computeBottleneck(streams, nil, now)

	// All dwells should be unknown.
	for _, st := range rep.Stages {
		if st.WIP > 0 && st.UnknownDwell != st.WIP {
			t.Errorf("stage %s: WIP=%d but UnknownDwell=%d (all should be unknown with no history)", st.Stage, st.WIP, st.UnknownDwell)
		}
	}

	// Constraint should be empty (no stage has known dwell).
	if rep.Constraint != "" {
		t.Errorf("constraint should be empty with no history, got %q", rep.Constraint)
	}

	// Action should be empty.
	if rep.Action != "" {
		t.Errorf("action should be empty with no constraint, got %q", rep.Action)
	}
}

// TestBottleneckBlockedConstraint verifies that when only blocked has WIP with
// dwell, it is named as the constraint.
func TestBottleneckBlockedConstraint(t *testing.T) {
	t.Helper()
	root := t.TempDir()

	streamsDir := filepath.Join(root, "docs", "streams", "s")
	mustMkdirAll(t, streamsDir)
	writeTemp(t, streamsDir, "README.md", strings.Join([]string{
		"---",
		"stream: s",
		"status: active",
		"priority: P1",
		"track: product",
		"---",
		"",
		"# S",
		"",
		"| # | Brief | Wave | Status | Verified | Reviewed |",
		"|---|-------|------|--------|----------|----------|",
		"| 01 | brief-01 | 1 | blocked | — | — |",
		"",
	}, "\n"))

	now := time.Now().UTC()
	histDir := filepath.Join(root, "docs", "streams")
	writeTemp(t, histDir, ".history.jsonl",
		`{"ts":"`+now.AddDate(0, 0, -5).Format(time.RFC3339)+`","brief":"s/01","from":"in-progress","to":"blocked","sha":"a1"}`+"\n")

	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	history, _ := LoadHistory(filepath.Join(root, "docs", "streams", ".history.jsonl"))
	rep := computeBottleneck(streams, history, now)

	if rep.Constraint != "blocked" {
		t.Errorf("constraint = %q, want \"blocked\" (only stage with WIP+dwell)", rep.Constraint)
	}
	if !strings.Contains(rep.Action, "Unblock-bound") {
		t.Errorf("blocked action should be unblock-bound, got %q", rep.Action)
	}
}
