package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// trendFixture is a synthetic multi-week history exercising the rollup:
//
//	week A (starts Mon 2026-06-29): a/01 seeded implemented, a/02 seeded todo
//	week B (starts Mon 2026-07-06): a/01 implemented→verified, a/02 todo→done
//	week C (starts Mon 2026-07-13): a/01 verified→done, a/03 seeded implemented
func trendFixture() []HistoryEntry {
	return []HistoryEntry{
		{Ts: "2026-06-30T10:00:00Z", Brief: "a/01", From: "", To: "implemented", SHA: "s1"},
		{Ts: "2026-06-30T10:00:00Z", Brief: "a/02", From: "", To: "todo", SHA: "s1"},
		{Ts: "2026-07-07T10:00:00Z", Brief: "a/01", From: "implemented", To: "verified", SHA: "s2"},
		{Ts: "2026-07-07T10:00:00Z", Brief: "a/02", From: "todo", To: "done", SHA: "s2"},
		{Ts: "2026-07-14T10:00:00Z", Brief: "a/01", From: "verified", To: "done", SHA: "s3"},
		{Ts: "2026-07-14T10:00:00Z", Brief: "a/03", From: "", To: "implemented", SHA: "s3"},
	}
}

func TestTrendWeeklyRollup(t *testing.T) {
	buckets, ok, err := buildTrend(trendFixture(), time.Time{}, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected sufficient history")
	}
	if len(buckets) != 3 {
		t.Fatalf("got %d weekly buckets, want 3", len(buckets))
	}

	// Bucket boundaries are Mondays.
	wantStarts := []string{"2026-06-29", "2026-07-06", "2026-07-13"}
	for i, w := range wantStarts {
		if got := buckets[i].start.Format("2006-01-02"); got != w {
			t.Errorf("bucket[%d].start = %s, want %s", i, got, w)
		}
	}

	// Week A: a/01 implemented, a/02 todo; 0 done this week; backlog(impl+verif)=1.
	if buckets[0].throughput != 0 || buckets[0].backlog != 1 {
		t.Errorf("week A throughput=%d backlog=%d, want 0/1", buckets[0].throughput, buckets[0].backlog)
	}
	if buckets[0].counts["implemented"] != 1 || buckets[0].counts["todo"] != 1 {
		t.Errorf("week A counts = %+v, want implemented=1 todo=1", buckets[0].counts)
	}

	// Week B: a/02 done (throughput 1); state a/01 verified, a/02 done; backlog=1.
	if buckets[1].throughput != 1 || buckets[1].backlog != 1 {
		t.Errorf("week B throughput=%d backlog=%d, want 1/1", buckets[1].throughput, buckets[1].backlog)
	}
	if buckets[1].counts["verified"] != 1 || buckets[1].counts["done"] != 1 {
		t.Errorf("week B counts = %+v, want verified=1 done=1", buckets[1].counts)
	}

	// Week C: a/01 done (throughput 1); state a/01+a/02 done, a/03 implemented; backlog=1.
	if buckets[2].throughput != 1 || buckets[2].backlog != 1 {
		t.Errorf("week C throughput=%d backlog=%d, want 1/1", buckets[2].throughput, buckets[2].backlog)
	}
	if buckets[2].counts["done"] != 2 || buckets[2].counts["implemented"] != 1 {
		t.Errorf("week C counts = %+v, want done=2 implemented=1", buckets[2].counts)
	}
}

func TestTrendSinceClampsDisplayButNotState(t *testing.T) {
	since, _ := parseSinceDate("2026-07-06")
	buckets, ok, err := buildTrend(trendFixture(), since, "weekly")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2 (weeks B and C only)", len(buckets))
	}
	if got := buckets[0].start.Format("2006-01-02"); got != "2026-07-06" {
		t.Errorf("first displayed bucket = %s, want 2026-07-06", got)
	}
	// State was pre-rolled from week A even though it isn't displayed: week B
	// still shows a/01 verified + a/02 done (backlog 1), not a fresh start.
	if buckets[0].counts["verified"] != 1 || buckets[0].counts["done"] != 1 || buckets[0].backlog != 1 {
		t.Errorf("week B (clamped) counts=%+v backlog=%d, want verified=1 done=1 backlog=1",
			buckets[0].counts, buckets[0].backlog)
	}
}

func TestTrendDailyRollup(t *testing.T) {
	buckets, ok, err := buildTrend(trendFixture(), time.Time{}, "daily")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// Daily spans 2026-06-30 .. 2026-07-14 inclusive = 15 day buckets.
	if len(buckets) != 15 {
		t.Fatalf("got %d daily buckets, want 15", len(buckets))
	}
	if got := buckets[0].start.Format("2006-01-02"); got != "2026-06-30" {
		t.Errorf("first daily bucket = %s, want 2026-06-30", got)
	}
	if got := buckets[len(buckets)-1].start.Format("2006-01-02"); got != "2026-07-14" {
		t.Errorf("last daily bucket = %s, want 2026-07-14", got)
	}
}

func TestTrendInsufficientHistory(t *testing.T) {
	// Empty and single-entry histories both degrade gracefully.
	for _, entries := range [][]HistoryEntry{
		nil,
		{{Ts: "2026-07-09T00:00:00Z", Brief: "a/01", From: "", To: "todo"}},
	} {
		_, ok, err := buildTrend(entries, time.Time{}, "weekly")
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Errorf("expected insufficient history for %d entries", len(entries))
		}
	}
}

func TestTrendSinceBeyondAllData(t *testing.T) {
	since, _ := parseSinceDate("2027-01-01")
	_, ok, err := buildTrend(trendFixture(), since, "weekly")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected insufficient history when --since is past every transition")
	}
}

func TestTrendMalformedTimestampErrors(t *testing.T) {
	entries := []HistoryEntry{
		{Ts: "2026-07-09T00:00:00Z", Brief: "a/01", From: "", To: "todo"},
		{Ts: "not-a-date", Brief: "a/02", From: "", To: "todo"},
	}
	if _, _, err := buildTrend(entries, time.Time{}, "weekly"); err == nil {
		t.Error("expected an error on a malformed timestamp")
	}
}

func TestTrendRenderContainsSignals(t *testing.T) {
	buckets, _, _ := buildTrend(trendFixture(), time.Time{}, "weekly")
	out := renderTrend(buckets, "weekly", time.Time{}, 6)
	for _, want := range []string{"historian view", "period", "done/p", "backlog", "throughput", "lead-time debt"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q\n%s", want, out)
		}
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

// TestTrendRunInsufficientFixture is Verify item 3: --trend against a 1-entry
// history file prints "insufficient history" and exits 0.
func TestTrendRunInsufficientFixture(t *testing.T) {
	if code := runTrend(".", "testdata/trend/onerow.jsonl", "", "weekly"); code != 0 {
		t.Fatalf("runTrend on 1-entry fixture exited %d, want 0", code)
	}
}

// TestTrendRunMultiWeek exercises the full runTrend path against a written
// multi-week log (default history-path resolution under --root).
func TestTrendRunMultiWeek(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(historyRelPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendHistory(path, trendFixture()); err != nil {
		t.Fatal(err)
	}
	if code := runTrend(root, "", "2026-07-01", "weekly"); code != 0 {
		t.Fatalf("runTrend exited %d, want 0", code)
	}
}
