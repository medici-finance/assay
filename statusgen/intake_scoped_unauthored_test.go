package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what it
// wrote. NOTICE lines (emitNotices) go to stderr, so the lint-integration test
// needs this to observe them (the stdout helper lives in alarms_test.go).
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()
	fn()
	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestScopedToStream(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"observability", "observability"},
		{"  observability  ", "observability"},
		{"agent-flow-observability (new stream — silent-failure alerting)", "agent-flow-observability"},
		{"rwa (new stream — real-world assets)", "rwa"},
	}
	for _, c := range cases {
		if got := scopedToStream(c.in); got != c.want {
			t.Errorf("scopedToStream(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIntakeScopedUnauthoredNotices(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	streams := []*Stream{
		{Name: "observability"},
		{Name: "reconciler-spinout"},
	}

	t.Run("aged scoped entry with unauthored stream — NOTICE fires", func(t *testing.T) {
		// I-03 shape: scoped 2026-07-08 (16 days before now) → a stream that
		// does not exist in the known set.
		entries := []intakeEntry{
			{ID: "I-03", Date: "2026-07-08", Title: "agent-flow observability",
				Disposition: "scoped", ScopedTo: "agent-flow-observability (new stream — silent-failure alerting)"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 1 {
			t.Fatalf("got %d notices, want 1: %v", len(got), got)
		}
		want := "scoped-but-unauthored: intake I-03 scoped → agent-flow-observability for 16d with no docs/streams/agent-flow-observability/README.md — author the stream/brief or re-triage (issue-loop/08)"
		if got[0] != want {
			t.Errorf("NOTICE mismatch:\ngot:  %s\nwant: %s", got[0], want)
		}
	})

	t.Run("scoped to an existing (authored) stream — no NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-10", Date: "2026-07-01", Title: "obs idea",
				Disposition: "scoped", ScopedTo: "observability"},
			{ID: "I-11", Date: "2026-07-01", Title: "recon idea",
				Disposition: "scoped", ScopedTo: "reconciler-spinout (existing stream)"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 0 {
			t.Errorf("scoped-to an authored stream must not fire; got %v", got)
		}
	})

	t.Run("unauthored but under threshold — no NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			// 7 days old exactly — NOT > 7.
			{ID: "I-20", Date: "2026-07-17", Title: "young", Disposition: "scoped", ScopedTo: "brand-new-stream"},
			// 5 days old.
			{ID: "I-21", Date: "2026-07-19", Title: "younger", Disposition: "scoped", ScopedTo: "another-new-stream"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 0 {
			t.Errorf("entries at/under threshold must not fire; got %v", got)
		}
	})

	t.Run("just over threshold — NOTICE fires", func(t *testing.T) {
		// 8 days old — > 7.
		entries := []intakeEntry{
			{ID: "I-22", Date: "2026-07-16", Title: "aged", Disposition: "scoped", ScopedTo: "brand-new-stream"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 1 {
			t.Fatalf("8-day-old unauthored entry must fire; got %v", got)
		}
		if !strings.Contains(got[0], "I-22 scoped → brand-new-stream for 8d") {
			t.Errorf("unexpected NOTICE: %s", got[0])
		}
	})

	t.Run("non-scoped dispositions — ignored", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-30", Date: "2026-07-01", Title: "new", Disposition: "new"},
			{ID: "I-31", Date: "2026-07-01", Title: "rejected", Disposition: "rejected", ScopedTo: "nope"},
			{ID: "I-32", Date: "2026-07-01", Title: "watching", Disposition: "watching"},
			{ID: "I-33", Date: "2026-07-01", Title: "decision", Disposition: "decision-needed"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 0 {
			t.Errorf("only disposition: scoped counts; got %v", got)
		}
	})

	t.Run("scoped with empty scoped-to — ignored", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-40", Date: "2026-07-01", Title: "no target", Disposition: "scoped", ScopedTo: ""},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 0 {
			t.Errorf("empty scoped-to has no resolvable stream; got %v", got)
		}
	})

	t.Run("scoped disposition with surrounding whitespace — still counts", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-50", Date: "2026-07-08", Title: "spaced", Disposition: " scoped ", ScopedTo: "ghost-stream"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 1 {
			t.Fatalf("' scoped ' should count after TrimSpace; got %v", got)
		}
	})

	t.Run("unparseable date — per-entry bad-date NOTICE", func(t *testing.T) {
		entries := []intakeEntry{
			{ID: "I-60", Date: "not-a-date", Title: "bad date", Disposition: "scoped", ScopedTo: "ghost-stream"},
		}
		got := intakeScopedUnauthoredNotices(entries, streams, now)
		if len(got) != 1 {
			t.Fatalf("bad date must produce a diagnostic NOTICE; got %v", got)
		}
		if !strings.Contains(got[0], "I-60") || !strings.Contains(got[0], "not-a-date") {
			t.Errorf("bad-date NOTICE must name ID and offending value: %s", got[0])
		}
		if !strings.Contains(got[0], "(issue-loop/08)") {
			t.Errorf("bad-date NOTICE missing brief reference: %s", got[0])
		}
	})

	t.Run("zero entries — silent", func(t *testing.T) {
		if got := intakeScopedUnauthoredNotices(nil, streams, now); len(got) != 0 {
			t.Errorf("no entries should be silent; got %v", got)
		}
	})
}

// TestIntakeScopedUnauthoredLintIntegration exercises the NOTICE end-to-end through
// run() in lint mode: it must surface on stderr AND stay non-fatal (exit 0).
func TestIntakeScopedUnauthoredLintIntegration(t *testing.T) {
	root := t.TempDir()

	// goodrepo has stream "alpha" and no intake dir — a valid base.
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}

	idir := filepath.Join(root, "docs", "streams", "intake")
	mustMkdirAll(t, idir)
	// Scoped → a stream that does NOT exist (only "alpha" exists), aged well past 7d.
	writeTemp(t, idir, "2026-07-08-dangling.md",
		"---\nid: I-dangling-scoped\ndate: \"2026-07-08\"\ntitle: Dangling scoped entry\ndisposition: scoped\nscoped-to: ghost-stream (new stream — never authored)\n---\n\nBody.")
	// Scoped → the existing "alpha" stream: must NOT fire even though aged.
	writeTemp(t, idir, "2026-07-08-drained.md",
		"---\nid: I-drained-scoped\ndate: \"2026-07-08\"\ntitle: Drained scoped entry\ndisposition: scoped\nscoped-to: alpha\n---\n\nBody.")

	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	old := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = old })

	var code int
	stderr := captureStderr(t, func() {
		code = run(root, "lint", nil, nil, "")
	})
	if code != 0 {
		t.Fatalf("lint exited %d, want 0 (the scoped-unauthored NOTICE is advisory, non-fatal)", code)
	}
	if !strings.Contains(stderr, "scoped-but-unauthored: intake I-dangling-scoped scoped → ghost-stream") {
		t.Errorf("expected scoped-unauthored NOTICE on stderr; got:\n%s", stderr)
	}
	if strings.Contains(stderr, "I-drained-scoped") {
		t.Errorf("scoped→alpha (an authored stream) must not fire; stderr:\n%s", stderr)
	}
}
