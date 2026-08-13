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
