package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setup points deskkit at a fresh (non-existent) state dir under the test's
// temp dir — so the first Log/appendEntry is what CREATES it (0700), exactly as in
// production — and neutralises the ambient kill-switch / session env so tests are
// deterministic even when the process runs under DESK_TOOLS_DISABLED=1 (the Verify
// step run under that env). Each test/subtest calls it with its own *testing.T.
func setup(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "assay")
	old := dirOverride
	dirOverride = dir
	t.Cleanup(func() { dirOverride = old })
	t.Setenv("DESK_TOOLS_DISABLED", "") // "" is not "1" → not armed
	// Neutralise the harness's real session var so the fixture value below is what
	// SessionTag() returns — otherwise the ambient $CLAUDE_CODE_SESSION_ID (present in
	// every Claude Code session) wins the precedence and the tag is non-deterministic.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	return dir
}

func appendLine(t *testing.T, dir, line string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir desk dir: %v", err)
	}
	p := filepath.Join(dir, "audit.jsonl")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		t.Fatalf("append line: %v", err)
	}
}

func appendEntry(t *testing.T, dir string, e Entry) {
	t.Helper()
	if e.Result == "" {
		e.Result = ResultOK
	}
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	appendLine(t, dir, string(b))
}

func iptr(i int) *int       { return &i }
func sptr(s string) *string { return &s }

// equalStrings lives HERE, in a file that ships, and not in citrigger_test.go
// where it started: citrigger_test.go is withheld from the public tree
// (docs/publication-manifest.yaml), and modprefix_test.go — which DOES ship —
// calls this helper in executable code. A shared test helper defined on the
// withheld side of the publication boundary is a build break the house tree
// cannot see, because the house tree has both files. Keep helpers that shipping
// tests call in shipping files.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
