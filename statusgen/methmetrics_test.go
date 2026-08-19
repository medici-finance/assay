package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// backlogFixture is a synthetic multi-week history exercising the rollup:
//
//	week A (starts Mon 2026-06-29): a/01 seeded implemented, a/02 seeded todo
//	week B (starts Mon 2026-07-06): a/01 implemented→verified, a/02 todo→done
//	week C (starts Mon 2026-07-13): a/01 verified→done, a/03 seeded implemented
//
// The awaiting-verification backlog (impl+verif) is 1 at the end of every week:
// A={a/01 impl}, B={a/01 verif}, C={a/03 impl}.
func backlogFixture() []HistoryEntry {
	return []HistoryEntry{
		{Ts: "2026-06-30T10:00:00Z", Brief: "a/01", From: "", To: "implemented", SHA: "s1"},
		{Ts: "2026-06-30T10:00:00Z", Brief: "a/02", From: "", To: "todo", SHA: "s1"},
		{Ts: "2026-07-07T10:00:00Z", Brief: "a/01", From: "implemented", To: "verified", SHA: "s2"},
		{Ts: "2026-07-07T10:00:00Z", Brief: "a/02", From: "todo", To: "done", SHA: "s2"},
		{Ts: "2026-07-14T10:00:00Z", Brief: "a/01", From: "verified", To: "done", SHA: "s3"},
		{Ts: "2026-07-14T10:00:00Z", Brief: "a/03", From: "", To: "implemented", SHA: "s3"},
	}
}

func TestVerifBacklogWeeklyRollup(t *testing.T) {
	points, ok, err := buildVerifBacklog(backlogFixture(), time.Time{}, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected sufficient history")
	}
	if len(points) != 3 {
		t.Fatalf("got %d weekly points, want 3", len(points))
	}
	wantStarts := []string{"2026-06-29", "2026-07-06", "2026-07-13"}
	for i, w := range wantStarts {
		if got := points[i].start.Format("2006-01-02"); got != w {
			t.Errorf("point[%d].start = %s, want %s", i, got, w)
		}
		if points[i].backlog != 1 {
			t.Errorf("point[%d].backlog = %d, want 1", i, points[i].backlog)
		}
	}
}

func TestVerifBacklogSinceClampsDisplayButNotState(t *testing.T) {
	since, _ := parseSinceDate("2026-07-06")
	points, ok, err := buildVerifBacklog(backlogFixture(), since, "weekly")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(points) != 2 {
		t.Fatalf("got %d points, want 2 (weeks B and C only)", len(points))
	}
	if got := points[0].start.Format("2006-01-02"); got != "2026-07-06" {
		t.Errorf("first displayed point = %s, want 2026-07-06", got)
	}
	// State was pre-rolled from week A even though it isn't displayed: week B
	// still shows a/01 verified (backlog 1), not a fresh start.
	if points[0].backlog != 1 {
		t.Errorf("week B (clamped) backlog=%d, want 1", points[0].backlog)
	}
}

func TestVerifBacklogDailyRollup(t *testing.T) {
	points, ok, err := buildVerifBacklog(backlogFixture(), time.Time{}, "daily")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// Daily spans 2026-06-30 .. 2026-07-14 inclusive = 15 day buckets.
	if len(points) != 15 {
		t.Fatalf("got %d daily points, want 15", len(points))
	}
	if got := points[0].start.Format("2006-01-02"); got != "2026-06-30" {
		t.Errorf("first daily point = %s, want 2026-06-30", got)
	}
	if got := points[len(points)-1].start.Format("2006-01-02"); got != "2026-07-14" {
		t.Errorf("last daily point = %s, want 2026-07-14", got)
	}
}

func TestVerifBacklogInsufficientHistory(t *testing.T) {
	for _, entries := range [][]HistoryEntry{
		nil,
		{{Ts: "2026-07-09T00:00:00Z", Brief: "a/01", From: "", To: "todo"}},
	} {
		_, ok, err := buildVerifBacklog(entries, time.Time{}, "weekly")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Errorf("expected insufficient history for %d entries", len(entries))
		}
	}
}

func TestVerifBacklogSinceBeyondAllData(t *testing.T) {
	since, _ := parseSinceDate("2027-01-01")
	_, ok, err := buildVerifBacklog(backlogFixture(), since, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected insufficient history when --since is past every transition")
	}
}

func TestVerifBacklogMalformedTimestampErrors(t *testing.T) {
	entries := []HistoryEntry{
		{Ts: "2026-07-09T00:00:00Z", Brief: "a/01", From: "", To: "todo"},
		{Ts: "not-a-date", Brief: "a/02", From: "", To: "todo"},
	}
	if _, _, err := buildVerifBacklog(entries, time.Time{}, "weekly"); err == nil {
		t.Error("expected an error on a malformed timestamp")
	}
}

func TestVerifBacklogRenderContainsSignals(t *testing.T) {
	points, _, _ := buildVerifBacklog(backlogFixture(), time.Time{}, "weekly")
	out := renderVerifBacklog(points, "weekly", time.Time{}, 6)
	for _, want := range []string{"awaiting-verification backlog", "period", "backlog", "lead-time debt", "methodology metric"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n%s", want, out)
		}
	}
}

// TestVerifBacklogRunInsufficientFixture: --verif-backlog against a 1-entry
// history file prints "insufficient history" and exits 0.
func TestVerifBacklogRunInsufficientFixture(t *testing.T) {
	if code := runVerifBacklog(".", "testdata/trend/onerow.jsonl", "", "weekly"); code != 0 {
		t.Fatalf("runVerifBacklog on 1-entry fixture exited %d, want 0", code)
	}
}

// TestVerifBacklogRunMultiWeek exercises the full runVerifBacklog path against a
// written multi-week log (default history-path resolution under --root).
func TestVerifBacklogRunMultiWeek(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(historyRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendHistory(path, backlogFixture()); err != nil {
		t.Fatal(err)
	}
	if code := runVerifBacklog(root, "", "2026-07-01", "weekly"); code != 0 {
		t.Fatalf("runVerifBacklog exited %d, want 0", code)
	}
}

func TestSparkline(t *testing.T) {
	if got := sparkline([]int{0, 4, 7}); got != "▁▅█" {
		t.Errorf("sparkline([0 4 7]) = %q, want ▁▅█", got)
	}
	// Flat series renders a mid-level bar, not misleading full/empty blocks.
	if got := sparkline([]int{3, 3, 3}); got != "▅▅▅" {
		t.Errorf("sparkline(flat) = %q, want ▅▅▅", got)
	}
	if got := sparkline(nil); got != "" {
		t.Errorf("sparkline(nil) = %q, want empty", got)
	}
}

func TestHumanDur(t *testing.T) {
	cases := map[time.Duration]string{
		6 * 24 * time.Hour:              "6.0d",
		36 * time.Hour:                  "1.5d",
		4*time.Hour + 30*time.Minute:    "4.5h",
		-(4*time.Hour + 30*time.Minute): "4.5h",
	}
	for d, want := range cases {
		if got := humanDur(d); got != want {
			t.Errorf("humanDur(%v) = %q, want %q", d, got, want)
		}
	}
}
