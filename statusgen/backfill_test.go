package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// streamREADME renders a minimal stream README with one brief row at the given
// status — the shape parseBriefTable/parseStreamSnapshot read.
func streamREADME(stream, status string) string {
	return "---\nstream: " + stream + "\nstatus: active\npriority: P1\n---\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | Do the thing | 0 | M | " + status + " | — | — |\n"
}

// initBackfillRepo builds a git repo with a stream README committed at three
// successive dates, walking status todo → implemented → done. Returns the repo
// root. Each commit's committer date is pinned so the reconstructed ts is
// machine-independent.
func initBackfillRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	rel := filepath.Join("docs", "streams", "demo", "README.md")
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	commit := func(status, date string) {
		if err := os.WriteFile(abs, []byte(streamREADME("demo", status)), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", rel)
		runGitEnv(t, dir, []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date},
			"commit", "-q", "-m", status)
	}
	commit("todo", "2026-07-01T10:00:00Z")
	commit("implemented", "2026-07-08T10:00:00Z")
	commit("done", "2026-07-15T10:00:00Z")
	return dir
}

func TestReconstructBackfillWalksTransitions(t *testing.T) {
	dir := initBackfillRepo(t)
	entries, couldNot, streams, err := reconstructBackfill(dir)
	if err != nil {
		t.Fatalf("reconstructBackfill: %v", err)
	}
	if couldNot != 0 {
		t.Errorf("couldNotParse = %d, want 0", couldNot)
	}
	if streams != 1 {
		t.Errorf("streamsSeen = %d, want 1", streams)
	}
	// Expect three rows for demo/01: seed(""→todo), todo→implemented,
	// implemented→done, in commit-date order, each source:"backfill".
	want := []struct{ from, to, ts string }{
		{"", "todo", "2026-07-01T10:00:00Z"},
		{"todo", "implemented", "2026-07-08T10:00:00Z"},
		{"implemented", "done", "2026-07-15T10:00:00Z"},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for i, w := range want {
		e := entries[i]
		if e.Brief != "demo/01" || e.From != w.from || e.To != w.to || e.Ts != w.ts {
			t.Errorf("entry %d = %+v, want brief=demo/01 from=%q to=%q ts=%q", i, e, w.from, w.to, w.ts)
		}
		if e.Source != "backfill" {
			t.Errorf("entry %d source = %q, want backfill", i, e.Source)
		}
		if e.SHA == "" {
			t.Errorf("entry %d has empty sha", i)
		}
	}
}

func TestRunBackfillIdempotentAndPrepends(t *testing.T) {
	dir := initBackfillRepo(t)
	path := filepath.Join(dir, filepath.FromSlash(historyRelPath))

	// Seed a LIVE row that must survive byte-for-byte and stay LAST.
	live := `{"ts":"2026-08-15T00:00:00Z","brief":"demo/01","from":"","to":"done","sha":"live0000"}` + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(live), 0o644); err != nil {
		t.Fatal(err)
	}

	if rc := runBackfill([]string{"--root", dir}, os.Stdout, os.Stderr); rc != backfillOK {
		t.Fatalf("first run rc = %d, want %d", rc, backfillOK)
	}
	after1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(after1), "\n"), "\n")
	// 3 backfilled + 1 live.
	if len(lines) != 4 {
		t.Fatalf("after run 1: %d lines, want 4:\n%s", len(lines), after1)
	}
	// The live row is preserved byte-for-byte AND last.
	if lines[len(lines)-1] != strings.TrimRight(live, "\n") {
		t.Errorf("live row not preserved-last; got %q", lines[len(lines)-1])
	}
	// Every prepended row carries source:backfill; the live row does not.
	for _, l := range lines[:3] {
		if !strings.Contains(l, `"source":"backfill"`) {
			t.Errorf("prepended row missing source:backfill: %q", l)
		}
	}
	if strings.Contains(lines[3], `"source"`) {
		t.Errorf("live row gained a source key: %q", lines[3])
	}

	// Idempotency: a second run leaves the file byte-identical.
	if rc := runBackfill([]string{"--root", dir}, os.Stdout, os.Stderr); rc != backfillOK {
		t.Fatalf("second run rc = %d, want %d", rc, backfillOK)
	}
	after2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after1) != string(after2) {
		t.Errorf("second run mutated the log (not idempotent):\n--- after1 ---\n%s\n--- after2 ---\n%s", after1, after2)
	}
}

func TestParseStreamSnapshotSkipsUnparseable(t *testing.T) {
	// No frontmatter → not a stream README at that commit → skip, not crash.
	if _, _, ok := parseStreamSnapshot("just some prose, no fences", "demo"); ok {
		t.Error("parseStreamSnapshot accepted content with no frontmatter")
	}
	// Frontmatter but no brief table → skip.
	if _, _, ok := parseStreamSnapshot("---\nstream: demo\n---\n\nno table here\n", "demo"); ok {
		t.Error("parseStreamSnapshot accepted content with no brief table")
	}
	// Valid → stream name from frontmatter.
	name, briefs, ok := parseStreamSnapshot(streamREADME("realname", "todo"), "dirfallback")
	if !ok || name != "realname" || len(briefs) != 1 {
		t.Errorf("valid snapshot: ok=%v name=%q briefs=%d", ok, name, len(briefs))
	}
	// Missing stream: field → fall back to the directory name.
	noName := "---\nstatus: active\n---\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | X | 0 | M | todo | — | — |\n"
	if name, _, ok := parseStreamSnapshot(noName, "dirfallback"); !ok || name != "dirfallback" {
		t.Errorf("fallback name: ok=%v name=%q, want dirfallback", ok, name)
	}
}

func TestRunBackfillNoGitDir(t *testing.T) {
	// A non-git directory is could-not-check, never a false clean pass.
	dir := t.TempDir()
	if rc := runBackfill([]string{"--root", dir}, os.Stdout, os.Stderr); rc != backfillCouldNot {
		t.Errorf("non-git root rc = %d, want %d", rc, backfillCouldNot)
	}
}
