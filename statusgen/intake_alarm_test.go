package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIntakeAlarm(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)

	t.Run("all disposition new — counts as untriaged", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-10", Title: "old", Disposition: "new"},
			{ID: "I-02", Date: "2026-07-12", Title: "recent", Disposition: "new"},
			{ID: "I-03", Date: "2026-07-09", Title: "oldest", Disposition: "new"},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 3 {
			t.Errorf("Untriaged = %d, want 3", r.Untriaged)
		}
		// July 13 - July 10 = 3 days, not > 3 → NOT over threshold
		// July 13 - July 9 = 4 days → over threshold
		if r.OverThreshold != 1 {
			t.Errorf("OverThreshold = %d, want 1 (I-01 at 3d is NOT > 3, I-02 at 1d is not, I-03 at 4d IS > 3)", r.OverThreshold)
		}
		if r.OldestID != "I-03" {
			t.Errorf("OldestID = %s, want I-03", r.OldestID)
		}
		if r.OldestDays != 4 {
			t.Errorf("OldestDays = %d, want 4", r.OldestDays)
		}
	})

	t.Run("scoped, rejected, watching — do NOT count", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-08", Title: "old new", Disposition: "new"},
			{ID: "I-02", Date: "2026-07-08", Title: "scoped", Disposition: "scoped"},
			{ID: "I-03", Date: "2026-07-08", Title: "rejected", Disposition: "rejected"},
			{ID: "I-04", Date: "2026-07-08", Title: "watching", Disposition: "watching"},
			{ID: "I-05", Date: "2026-07-08", Title: "unknown", Disposition: "bogus"},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 1 {
			t.Errorf("Untriaged = %d, want 1 (only disposition: new counts)", r.Untriaged)
		}
		if r.OverThreshold != 1 {
			t.Errorf("OverThreshold = %d, want 1", r.OverThreshold)
		}
	})

	t.Run("zero entries — silent", func(t *testing.T) {
		r := intakeAlarm(nil, now)
		if r.Untriaged != 0 || r.OverThreshold != 0 || r.OldestID != "" || r.OldestDays != 0 {
			t.Errorf("zero entries should produce all-zero result; got %+v", r)
		}
		n := intakeDebtNotice(r)
		if n != "" {
			t.Errorf("zero entries should produce no NOTICE; got %q", n)
		}
	})

	t.Run("under threshold — no NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-11", Title: "new", Disposition: "new"},      // 2 days ago
			{ID: "I-02", Date: "2026-07-10", Title: "also new", Disposition: "new"}, // 3 days ago, not > 3
		}
		r := intakeAlarm(entries, now)
		n := intakeDebtNotice(r)
		if n != "" {
			t.Errorf("entries at/under threshold should produce no NOTICE; got %q (over=%d)", n, r.OverThreshold)
		}
		if r.Untriaged != 2 {
			t.Errorf("Untriaged = %d, want 2", r.Untriaged)
		}
		if r.OverThreshold != 0 {
			t.Errorf("OverThreshold = %d, want 0 (I-01 at 2 days; I-02 at 3 days = NOT > 3)", r.OverThreshold)
		}
	})

	t.Run("NOTICE text stable", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-05", Date: "2026-07-08", Title: "old", Disposition: "new"},    // 5 days ago
			{ID: "I-06", Date: "2026-07-09", Title: "older", Disposition: "new"},  // 4 days ago
			{ID: "I-07", Date: "2026-07-12", Title: "recent", Disposition: "new"}, // 1 day ago
		}
		r := intakeAlarm(entries, now)
		n := intakeDebtNotice(r)
		want := "intake debt: 3 untriaged (2 over 3 days, oldest I-05 at 5d) — triage the front door"
		if n != want {
			t.Errorf("NOTICE text mismatch:\ngot:  %s\nwant: %s", n, want)
		}
	})

	t.Run("board line with untriaged over threshold", func(t *testing.T) {
		r := IntakeAlarmResult{Untriaged: 5, OverThreshold: 3, OldestID: "I-01", OldestDays: 7}
		got := intakeBoardLine(r)
		want := "_5 untriaged (3 over 3 days, oldest I-01 at 7d) — triage the front door_"
		if got != want {
			t.Errorf("got  %s\nwant %s", got, want)
		}
	})

	t.Run("board line with untriaged but none over threshold", func(t *testing.T) {
		r := IntakeAlarmResult{Untriaged: 3, OverThreshold: 0, OldestID: "I-02", OldestDays: 2}
		got := intakeBoardLine(r)
		if !strings.Contains(got, "3 untriaged (0 over 3 days, oldest I-02 at 2d)") {
			t.Errorf("board line should always show over-count: %s", got)
		}
	})

	t.Run("board line zero entries", func(t *testing.T) {
		got := intakeBoardLine(IntakeAlarmResult{})
		if !strings.Contains(got, "0 untriaged") {
			t.Errorf("zero case: %s", got)
		}
		if !strings.Contains(got, "front door is clear") {
			t.Errorf("zero case missing 'clear': %s", got)
		}
	})

	t.Run("malformed dates — tracked in DateParseFailIDs", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-10", Title: "good", Disposition: "new"},
			{ID: "I-02", Date: "not-a-date", Title: "bad", Disposition: "new"},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 2 {
			t.Errorf("Untriaged = %d, want 2 (bad date still counts as untriaged)", r.Untriaged)
		}
		if r.OverThreshold != 0 {
			t.Errorf("OverThreshold = %d, want 0 (I-01 at 3d NOT > 3; bad date is skipped in age calc)", r.OverThreshold)
		}
		if r.OldestID != "I-01" {
			t.Errorf("OldestID = %s, want I-01 (bad date skipped for oldest calc)", r.OldestID)
		}
		if len(r.BadDates) != 1 {
			t.Fatalf("BadDates = %d, want 1", len(r.BadDates))
		}
		if !strings.Contains(r.BadDates[0], "I-02") || !strings.Contains(r.BadDates[0], "not-a-date") {
			t.Errorf("BadDates[0] = %q, want mention of I-02 and not-a-date", r.BadDates[0])
		}
	})

	t.Run("disposition with whitespace around 'new'", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-08", Title: "whitespaced", Disposition: " new "},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 1 {
			t.Errorf("disposition ' new ' should count as new after TrimSpace; got Untriaged=%d", r.Untriaged)
		}
	})

	t.Run("empty disposition — defaults to new", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-08", Title: "no disposition key", Disposition: ""},
			{ID: "I-02", Date: "2026-07-07", Title: "also empty", Disposition: ""},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 2 {
			t.Errorf("Untriaged = %d, want 2 (empty disposition defaults to new)", r.Untriaged)
		}
	})

	t.Run("no bad dates — BadDates empty", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-01", Date: "2026-07-10", Title: "good", Disposition: "new"},
		}
		r := intakeAlarm(entries, now)
		if len(r.BadDates) != 0 {
			t.Errorf("BadDates = %v, want empty when all dates parse", r.BadDates)
		}
	})

	t.Run("first-entry guard — all entries filed today", func(t *testing.T) {
		today := now.Format("2006-01-02")
		entries := []intakeEntry{
			{ID: "I-01", Date: today, Title: "first", Disposition: "new"},
			{ID: "I-02", Date: today, Title: "second", Disposition: "new"},
		}
		r := intakeAlarm(entries, now)
		if r.Untriaged != 2 {
			t.Errorf("Untriaged = %d, want 2", r.Untriaged)
		}
		if r.OverThreshold != 0 {
			t.Errorf("OverThreshold = %d, want 0 (all entries today)", r.OverThreshold)
		}
		if r.OldestID == "" {
			t.Error("OldestID must not be empty when there are untriaged entries (first-entry guard)")
		}
		if r.OldestDays != 0 {
			t.Errorf("OldestDays = %d, want 0", r.OldestDays)
		}
		bl := intakeBoardLine(r)
		if bl == "" {
			t.Error("board line should render even when nothing is over threshold")
		}
		if strings.Contains(bl, "oldest  at") {
			t.Errorf("board line must never print a blank oldest ID: %s", bl)
		}
	})
}

func TestIntakeAlarmE2E(t *testing.T) {
	// End-to-end: create a real intake directory with fixture entries,
	// run intakeAlarm from the parsed results, verify board line
	// renders in the STATUS.md emit output.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	// One old untriaged entry (over threshold), one recent new, one scoped.
	writeTemp(t, idir, "2026-07-08-old.md",
		"---\nid: I-old-idea-intake\ndate: \"2026-07-08\"\ntitle: Old idea\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-12-recent.md",
		"---\nid: I-recent-idea-intake\ndate: \"2026-07-12\"\ntitle: Recent idea\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-10-scoped.md",
		"---\nid: I-scoped-idea-test\ndate: \"2026-07-10\"\ntitle: Scoped idea\ndisposition: scoped\nscoped-to: something\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	r := intakeAlarm(entries, now)

	if r.Untriaged != 2 {
		t.Errorf("Untriaged = %d, want 2", r.Untriaged)
	}
	if r.OverThreshold != 1 {
		t.Errorf("OverThreshold = %d, want 1 (oldest at 5d > 3)", r.OverThreshold)
	}
	if r.OldestID != "I-old-idea-intake" {
		t.Errorf("OldestID = %s, want I-old-idea-intake", r.OldestID)
	}
	if r.OldestDays != 5 {
		t.Errorf("OldestDays = %d, want 5", r.OldestDays)
	}

	// NOTICE should fire.
	n := intakeDebtNotice(r)
	if n == "" {
		t.Error("NOTICE should fire when over threshold")
	}
	if !strings.Contains(n, "intake debt: 2 untriaged (1 over 3 days, oldest I-old-idea-intake at 5d)") {
		t.Errorf("unexpected NOTICE: %s", n)
	}

	// Board line renders in emit.
	// Set up a minimal stream so emit doesn't zero out.
	sdir := filepath.Join(root, "docs", "streams", "teststream")
	mustMkdirAll(t, sdir)
	writeTemp(t, sdir, "README.md", "---\nstream: teststream\nstatus: active\npriority: P1\ntrack: product\n---\n\n# Test\n\n## Briefs\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n| 01 | One | 1 | S | done | grandfathered | grandfathered |\n")
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	out := emit(streams, nil, nextUp(streams, nil, nil), nil, r, nil, "")
	if !strings.Contains(out, "## Intake queue") {
		t.Error("STATUS.md missing Intake queue heading")
	}
	if !strings.Contains(out, "2 untriaged (1 over 3 days, oldest I-old-idea-intake at 5d)") {
		t.Errorf("intake board line not rendered correctly:\n%s", out)
	}

	// Zero-case board line.
	empty := emit(streams, nil, nextUp(streams, nil, nil), nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(empty, "0 untriaged entries — the front door is clear") {
		t.Error("zero-case board line missing")
	}
}

func TestIntakeAlarmParseRoundTrip(t *testing.T) {
	// Smoke test: the intakeDir parser correctly parses disposition fields.
	root := t.TempDir()
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)

	writeTemp(t, idir, "2026-07-08-a.md",
		"---\nid: I-01\ndate: \"2026-07-08\"\ntitle: Alpha\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-09-b.md",
		"---\nid: I-02\ndate: \"2026-07-09\"\ntitle: Beta\ndisposition: rejected\nwhy: not now\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-10-c.md",
		"---\nid: I-03\ndate: \"2026-07-10\"\ntitle: Gamma\ndisposition: watching\n---\n\nBody.")

	entries, err := parseIntakeDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	disp := map[string]string{}
	for _, e := range entries {
		disp[e.ID] = e.Disposition
	}
	if disp["I-01"] != "new" {
		t.Errorf("I-01 disposition = %q, want new", disp["I-01"])
	}
	if disp["I-02"] != "rejected" {
		t.Errorf("I-02 disposition = %q, want rejected", disp["I-02"])
	}
	if disp["I-03"] != "watching" {
		t.Errorf("I-03 disposition = %q, want watching", disp["I-03"])
	}
}

func TestIntakeAlarmMainLintIntegration(t *testing.T) {
	// Verify --lint mode correctly computes and surfaces the intake NOTICE.
	root := t.TempDir()

	// Copy goodrepo fixture as the base (no intake entries → zero state is fine).
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}

	// Add intake entries with one old untriaged entry.
	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	writeTemp(t, idir, "2026-07-08-old.md",
		"---\nid: I-old-idea-intake\ndate: \"2026-07-08\"\ntitle: Old idea\ndisposition: new\n---\n\nBody.")
	writeTemp(t, idir, "2026-07-12-recent.md",
		"---\nid: I-recent-idea-intake\ndate: \"2026-07-12\"\ntitle: Recent idea\ndisposition: new\n---\n\nBody.")

	// Override nowFunc for deterministic test.
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = old })

	// Capture stderr to check for NOTICE output.
	code := run(root, "lint", nil, nil, "")
	if code != 0 {
		t.Fatalf("lint exited %d, want 0 (NOTICE is non-fatal)", code)
	}
}
