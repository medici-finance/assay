package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// watchdogTestClock pins the watchdog's `now` so freshness verdicts are exact.
var watchdogTestClock = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

// freshStatusTree writes a STATUS.md with the given mtime into a temp root.
// The root is NOT a git repo, so boardFreshness falls back to the file mtime —
// the deterministic, content-free freshness source the watchdog reads.
func freshStatusTree(t *testing.T, mtime time.Time, content string) string {
	t.Helper()
	dir := t.TempDir()
	statusPath := filepath.Join(dir, "STATUS.md")
	if err := os.WriteFile(statusPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(statusPath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return dir
}

// pinNow pins nowFunc for the duration of the test.
func pinNow(t *testing.T, now time.Time) {
	t.Helper()
	saved := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = saved })
}

// TestBoardFreezeWatchdog is Verify row 8: the alarm fires (rc≠0 + one
// board-freeze issue payload) when the heartbeat age exceeds 2× the regen
// cadence, and stays SILENT (rc 0, no issue) within cadence.
func TestBoardFreezeWatchdog(t *testing.T) {
	pinNow(t, watchdogTestClock)
	dir := freshStatusTree(t, watchdogTestClock.Add(-1*time.Hour),
		"a board generated within cadence")

	if code := runWatchdog(dir); code != 0 {
		t.Fatalf("fresh board: runWatchdog = %d, want 0 (silent within cadence)", code)
	}

	// One missed regen of slack: exactly 2× cadence is NOT frozen.
	dir = freshStatusTree(t, watchdogTestClock.Add(-2*regenCadence),
		"exactly at the threshold")
	if code := runWatchdog(dir); code != 0 {
		t.Fatalf("board at exactly 2× cadence: runWatchdog = %d, want 0", code)
	}

	// Past 2× cadence: the alarm trips.
	age := 49 * time.Hour
	dir = freshStatusTree(t, watchdogTestClock.Add(-age), "stale board content")
	if code := runWatchdog(dir); code != 1 {
		t.Fatalf("stale board: runWatchdog = %d, want 1 (BOARD FROZEN)", code)
	}

	// The payload is one board-freeze issue carrying the idempotency marker.
	payload := boardFreezePayload(dir, age)
	if len(payload) != 1 {
		t.Fatalf("boardFreezePayload len = %d, want 1", len(payload))
	}
	if payload[0].Marker != boardFreezeMarker() {
		t.Errorf("marker = %q, want %q", payload[0].Marker, boardFreezeMarker())
	}
	if len(payload[0].Labels) != 1 || payload[0].Labels[0] != boardFreezeLabel {
		t.Errorf("labels = %v, want [%s]", payload[0].Labels, boardFreezeLabel)
	}
	if !strings.Contains(payload[0].Title, "BOARD FROZEN") {
		t.Errorf("title %q lacks BOARD FROZEN", payload[0].Title)
	}
	if !strings.Contains(payload[0].Body, boardFreezeMarker()) {
		t.Errorf("body lacks the idempotency marker")
	}
}

// TestWatchdogIndependentOfBoardBuild is Verify row I: --watchdog does NO board
// build and reaches its freshness verdict even when a full board build would
// PROBLEM-abort — a frozen board cannot suppress its own freeze alarm.
func TestWatchdogIndependentOfBoardBuild(t *testing.T) {
	pinNow(t, watchdogTestClock)
	// A root whose stream sources would fail any board build.
	dir := freshStatusTree(t, watchdogTestClock.Add(-49*time.Hour), "stale")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "streams", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "streams", "broken", "README.md"),
		[]byte("not a valid stream at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The watchdog never loads streams — it reads freshness only.
	if code := runWatchdog(dir); code != 1 {
		t.Fatalf("watchdog on an unbuildable board = %d, want 1 (frozen verdict despite build-breaking sources)", code)
	}
	// And with a fresh board the SAME unbuildable sources do not trip it.
	dir = freshStatusTree(t, watchdogTestClock.Add(-1*time.Hour), "fresh")
	if err := os.MkdirAll(filepath.Join(dir, "docs", "streams", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "streams", "broken", "README.md"),
		[]byte("not a valid stream at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runWatchdog(dir); code != 0 {
		t.Fatalf("watchdog on a fresh unbuildable board = %d, want 0", code)
	}
}

// TestWatchdogChecksMtimeNotContent is Verify row C: the verdict is a pure
// function of freshness — a stale board whose frozen content reads healthy
// TRIPS; a fresh board whose content contains alarm-like words does NOT.
func TestWatchdogChecksMtimeNotContent(t *testing.T) {
	pinNow(t, watchdogTestClock)

	// Stale + healthy-looking content ⇒ TRIPS.
	dir := freshStatusTree(t, watchdogTestClock.Add(-50*time.Hour),
		"All streams healthy. Nothing to see. Everything is fine.")
	if code := runWatchdog(dir); code != 1 {
		t.Fatalf("stale board with healthy content = %d, want 1", code)
	}

	// Fresh + alarm-word content ⇒ does NOT trip.
	dir = freshStatusTree(t, watchdogTestClock.Add(-30*time.Minute),
		"BOARD FROZEN is a string that appears here, in a fresh board's content")
	if code := runWatchdog(dir); code != 0 {
		t.Fatalf("fresh board with alarm-like content = %d, want 0", code)
	}
}
