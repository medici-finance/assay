package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLogAppendOnly proves the audit file is opened O_APPEND: write two lines and
// assert BOTH survive in order (the tools never truncate/rewrite).
func TestLogAppendOnly(t *testing.T) {
	setup(t)

	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "first"}); err != nil {
		t.Fatalf("Log #1: %v", err)
	}
	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultNoop, Detail: "second"}); err != nil {
		t.Fatalf("Log #2: %v", err)
	}

	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (append must not overwrite)", len(entries))
	}
	if entries[0].Detail != "first" || entries[1].Detail != "second" {
		t.Fatalf("append order wrong: %q then %q", entries[0].Detail, entries[1].Detail)
	}
	// Defaults filled in.
	if entries[0].TS == "" || entries[0].SessionTag != "test-session" {
		t.Fatalf("defaults not filled: ts=%q session=%q", entries[0].TS, entries[0].SessionTag)
	}
	if entries[0].SourceSHA == "" || entries[0].BuiltAt == "" {
		t.Fatalf("version stamp not filled: %q %q", entries[0].SourceSHA, entries[0].BuiltAt)
	}
}

func TestLogCreatesDirAndFilePerms(t *testing.T) {
	dir := setup(t)
	if err := Log(Entry{Tool: "deskwt", Verb: "guard", Result: ResultOK}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
	fi, err := os.Stat(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestLogRejectsMissingResult(t *testing.T) {
	setup(t)
	if err := Log(Entry{Tool: "deskpost", Verb: "comment"}); !IsUnverifiable(err) {
		t.Fatalf("Log with empty result = %v, want Unverifiable", err)
	}
}

// TestLoadEntriesMissingIsEmpty — a missing audit file is empty history, not an error
// (bootstrap).
func TestLoadEntriesMissingIsEmpty(t *testing.T) {
	setup(t)
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries on missing file = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

// TestLoadEntriesCorruptLineUnverifiable — a malformed line is REFUSED (exit 6), not
// skipped. This is the core negative-path property downstream lookups
// inherit.
func TestLoadEntriesCorruptLineUnverifiable(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK})
	appendLine(t, dir, `{"ts": this is not valid json`)

	entries, err := LoadEntries()
	if entries != nil {
		t.Fatalf("LoadEntries returned entries despite corruption: %v", entries)
	}
	if !IsUnverifiable(err) {
		t.Fatalf("LoadEntries on corrupt line = %v, want Unverifiable (exit 6)", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitUnverifiable)
	}
}

func TestFirstTS(t *testing.T) {
	dir := setup(t)

	if ts, err := FirstTS(); err != nil || ts != "" {
		t.Fatalf("FirstTS on empty = (%q,%v), want (\"\",nil)", ts, err)
	}
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, TS: "2020-01-01T00:00:00Z"})
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, TS: "2021-01-01T00:00:00Z"})
	ts, err := FirstTS()
	if err != nil {
		t.Fatalf("FirstTS: %v", err)
	}
	if ts != "2020-01-01T00:00:00Z" {
		t.Fatalf("FirstTS = %q, want the earliest-appended ts", ts)
	}
}

func TestArgsDigestStable(t *testing.T) {
	a := ArgsDigest([]string{"review", "--head", "abc"})
	b := ArgsDigest([]string{"review", "--head", "abc"})
	if a != b {
		t.Fatalf("ArgsDigest not stable: %q vs %q", a, b)
	}
	if a == ArgsDigest([]string{"review", "--head", "def"}) {
		t.Fatalf("ArgsDigest collided across different args")
	}
	if len(a) != 64 {
		t.Fatalf("ArgsDigest length = %d, want 64 (sha256 hex)", len(a))
	}
}

func TestSessionTag(t *testing.T) {
	// Primary: the variable the Claude Code harness actually exports wins.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "code-sess")
	t.Setenv("CLAUDE_SESSION_ID", "legacy-sess")
	if got := SessionTag(); got != "code-sess" {
		t.Fatalf("SessionTag = %q, want code-sess (CLAUDE_CODE_SESSION_ID takes precedence)", got)
	}
	// Legacy fallback: when the primary is unset, the old variable is still honoured.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	if got := SessionTag(); got != "legacy-sess" {
		t.Fatalf("SessionTag = %q, want legacy-sess (legacy fallback)", got)
	}
	// Neither set → "unknown".
	t.Setenv("CLAUDE_SESSION_ID", "")
	if got := SessionTag(); got != "unknown" {
		t.Fatalf("SessionTag = %q, want unknown", got)
	}
}
