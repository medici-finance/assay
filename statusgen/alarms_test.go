package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mustTime parses a YYYY-MM-DD test date as UTC midnight.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	// Try date-only first (alarms), then RFC3339 (DORA). Shared by multiple
	// test files post-merge — do not duplicate.
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("mustTime: %v", err)
		}
	}
	return parsed
}

// finding is a small constructor for a register entry in alarm tests.
func finding(id, date, title string, resolved bool) Finding {
	return Finding{ID: id, Date: date, Title: title, Resolved: resolved}
}

func TestAlarmRateAgeFloodBasics(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	findings := []Finding{
		finding("F-01", "2026-07-01", "old resolved", true),
		finding("F-02", "2026-07-28", "recent open", false),
		finding("F-03", "2026-07-29", "recent open too", false),
	}
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: defaultFloodThreshold}
	rep := computeAlarms(findings, cfg, now)

	if rep.OpenedTotal != 3 {
		t.Errorf("OpenedTotal = %d, want 3", rep.OpenedTotal)
	}
	if rep.RecentOpened != 2 { // F-02, F-03 within the last 7 days
		t.Errorf("RecentOpened = %d, want 2", rep.RecentOpened)
	}
	if rep.ActiveCount != 2 { // F-02, F-03 unresolved
		t.Errorf("ActiveCount = %d, want 2", rep.ActiveCount)
	}
	if rep.WindowDays != 29 { // 2026-07-01 → 2026-07-30
		t.Errorf("WindowDays = %d, want 29", rep.WindowDays)
	}
	if rep.Flood {
		t.Errorf("Flood = true, want false (2 active ≤ %d)", cfg.FloodThreshold)
	}
	if rep.PastThreshold != 0 { // both < 7 days old
		t.Errorf("PastThreshold = %d, want 0", rep.PastThreshold)
	}
	if rep.AvgPerWeek <= 0 {
		t.Errorf("AvgPerWeek = %v, want > 0", rep.AvgPerWeek)
	}
}

func TestAlarmStandingAgePastThreshold(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	// Opened 30 days ago, unresolved → a standing alarm past the 7-day threshold.
	findings := []Finding{finding("F-07", "2026-06-30", "stale unresolved finding", false)}
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: defaultFloodThreshold}
	rep := computeAlarms(findings, cfg, now)

	if len(rep.Standing) != 1 {
		t.Fatalf("Standing = %d entries, want 1", len(rep.Standing))
	}
	a := rep.Standing[0]
	if a.AgeDays != 30 {
		t.Errorf("AgeDays = %d, want 30", a.AgeDays)
	}
	if !a.Over {
		t.Errorf("Over = false, want true (30 > %d)", cfg.StandingAgeDays)
	}
	if rep.PastThreshold != 1 {
		t.Errorf("PastThreshold = %d, want 1", rep.PastThreshold)
	}

	// The --lint NOTICE must name it as a standing alarm past threshold.
	notices := standingAlarmNotices(findings, cfg, now)
	if len(notices) != 1 || !strings.Contains(notices[0], "F-07") || !strings.Contains(notices[0], "standing alarm") {
		t.Errorf("standingAlarmNotices = %v, want one naming F-07", notices)
	}
}

func TestAlarmFloodCondition(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: 3}
	// 4 active unresolved findings, threshold 3 → flood.
	var findings []Finding
	for i, id := range []string{"F-01", "F-02", "F-03", "F-04"} {
		_ = i
		findings = append(findings, finding(id, "2026-07-29", "active", false))
	}
	rep := computeAlarms(findings, cfg, now)
	if rep.ActiveCount != 4 {
		t.Fatalf("ActiveCount = %d, want 4", rep.ActiveCount)
	}
	if !rep.Flood {
		t.Errorf("Flood = false, want true (4 > 3)")
	}
	notices := standingAlarmNotices(findings, cfg, now)
	if !containsSubstr(notices, "alarm flood") {
		t.Errorf("expected a flood NOTICE, got %v", notices)
	}

	// At the threshold (not over) → no flood.
	repAt := computeAlarms(findings[:3], cfg, now)
	if repAt.Flood {
		t.Errorf("Flood = true at exactly the threshold, want false")
	}
}

func TestAlarmStandingSortedOldestFirst(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	findings := []Finding{
		finding("F-01", "2026-07-29", "youngest", false),
		finding("F-02", "2026-07-10", "oldest", false),
		finding("F-03", "2026-07-25", "middle", false),
	}
	rep := computeAlarms(findings, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now)
	got := []string{rep.Standing[0].ID, rep.Standing[1].ID, rep.Standing[2].ID}
	want := []string{"F-02", "F-03", "F-01"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Standing order = %v, want %v", got, want)
			break
		}
	}
}

// TestRunAlarmsViewReportsStanding drives the full --alarms sub-command against a
// temp docs/streams/findings/ per-entry fixture (the source of truth — #297)
// and asserts the rendered view flags the standing alarm.
func TestRunAlarmsViewFixture(t *testing.T) {
	root := t.TempDir()
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-06-30-stale.md", "---\nid: F-01\ndate: \"2026-06-30\"\ntitle: stale unresolved finding\nresolved: false\n---\n\nBody.\n")
	writeTemp(t, fdir, "2026-07-29-fresh.md", "---\nid: F-02\ndate: \"2026-07-29\"\ntitle: fresh finding\nresolved: false\n---\n\nBody.\n")

	fixedNow(t, "2026-07-30T00:00:00Z")
	restore := setAlarmConfig(defaultStandingAgeDays, defaultFloodThreshold)
	defer restore()

	out := captureStdout(t, func() {
		if code := runAlarms(root); code != 0 {
			t.Fatalf("runAlarms exit = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "F-01") || !strings.Contains(out, "⚠") {
		t.Errorf("view missing flagged standing alarm F-01:\n%s", out)
	}
	if !strings.Contains(out, "Standing alarms") || !strings.Contains(out, "Alarm rate") {
		t.Errorf("view missing expected sections:\n%s", out)
	}
}

// TestRunAlarmsIgnoresLegacyFindingsFile proves the #297 fix: --alarms reads
// docs/streams/findings/ (per-entry, the source of truth on a branch — branches
// never regenerate FINDINGS.md, that's main-CI-only) and no longer routes
// through the legacy single-file FINDINGS.md reader. A finding added only under
// the per-entry dir, with a stale FINDINGS.md that doesn't mention it, must
// still surface — the same state --lint's standingAlarmNotices already sees
// (load.go's loadStreams uses parseFindings(root) too).
func TestRunAlarmsIgnoresLegacyFindingsFile(t *testing.T) {
	root := t.TempDir()
	sdir := filepath.Join(root, "docs", "streams")
	mustMkdirAll(t, sdir)
	// Stale legacy file: if runAlarms still read this, the new finding below
	// (added only under docs/streams/findings/) would be invisible.
	writeTemp(t, sdir, "FINDINGS.md", "# Findings\n\n_None._\n")

	fdir := filepath.Join(sdir, "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-10-new.md", "---\nid: F-09\ndate: \"2026-07-10\"\ntitle: new finding not in FINDINGS.md\nresolved: false\n---\n\nBody.\n")

	fixedNow(t, "2026-07-16T00:00:00Z")
	restore := setAlarmConfig(defaultStandingAgeDays, defaultFloodThreshold)
	defer restore()

	out := captureStdout(t, func() {
		if code := runAlarms(root); code != 0 {
			t.Fatalf("runAlarms exit = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "F-09") {
		t.Errorf("--alarms did not see per-entry finding F-09 (still reading legacy FINDINGS.md?):\n%s", out)
	}
}

// setAlarmConfig temporarily overrides the wired-in alarm thresholds; the returned
// func restores the previous values.
func setAlarmConfig(age, flood int) func() {
	prevAge, prevFlood := standingAgeDays, floodThreshold
	standingAgeDays, floodThreshold = age, flood
	return func() { standingAgeDays, floodThreshold = prevAge, prevFlood }
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
