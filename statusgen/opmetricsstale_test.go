package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOpmetricsStaleNotice is Verify row 9a: a 5-day-old day-file fires the
// NOTICE, a fresh one and an absent one do not.
func TestOpmetricsStaleNotice(t *testing.T) {
	writeDayFile := func(root, date string) {
		dir := filepath.Join(root, "docs", "reports", "daily", date)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"schema":"opmetrics/2","date":"` + date + `","classifierVersion":"opmetrics-relay/2"}`
		if err := os.WriteFile(filepath.Join(dir, "opmetrics.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	// (a) stale: newest day-file is 5 days old → exactly one NOTICE naming the token.
	stale := t.TempDir()
	writeDayFile(stale, "2026-07-15")
	got := opmetricsStaleNotices(stale, now)
	if len(got) != 1 {
		t.Fatalf("stale root: got %d notices, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0], "opmetrics-stale") {
		t.Fatalf("stale notice does not carry the token: %q", got[0])
	}

	// (b) fresh: a day-file dated today → silent (a newer file next to the stale
	// one also clears it — the check reads the NEWEST).
	fresh := t.TempDir()
	writeDayFile(fresh, "2026-07-15")
	writeDayFile(fresh, "2026-07-20")
	if got := opmetricsStaleNotices(fresh, now); len(got) != 0 {
		t.Fatalf("fresh root: got %d notices, want 0: %v", len(got), got)
	}

	// (c) boundary: exactly 3 days old is within the window → silent.
	edge := t.TempDir()
	writeDayFile(edge, "2026-07-17")
	if got := opmetricsStaleNotices(edge, now); len(got) != 0 {
		t.Fatalf("3-day-old root: got %d notices, want 0 (within the window): %v", len(got), got)
	}

	// (d) absent: a root that has never carried a day-file → silent, not nagged.
	if got := opmetricsStaleNotices(t.TempDir(), now); len(got) != 0 {
		t.Fatalf("root with no day-file: got %d notices, want 0: %v", len(got), got)
	}

	// (e) a daily dir that exists but holds NO opmetrics.json is still absent —
	// another tool's day-file must not be mistaken for the collector's.
	noOpm := t.TempDir()
	if err := os.MkdirAll(filepath.Join(noOpm, "docs", "reports", "daily", "2026-07-15"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := opmetricsStaleNotices(noOpm, now); len(got) != 0 {
		t.Fatalf("root with a daily dir but no opmetrics.json: got %d notices, want 0: %v", len(got), got)
	}
}
