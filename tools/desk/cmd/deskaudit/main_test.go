package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// seedHome points the desk state dir at a temp HOME and returns the audit.jsonl path.
func seedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	return filepath.Join(dir, "audit.jsonl")
}

// TestRecoverVerbQuarantinesAndCarries runs the actual `deskaudit recover` path (Guard +
// RecoverCorruptAudit) against a corrupt log and asserts it exits OK, carries the good
// entries, and quarantines the bad line.
func TestRecoverVerbQuarantinesAndCarries(t *testing.T) {
	audit := seedHome(t)
	good := `{"ts":"2026-01-01T00:00:00Z","tool":"deskpost","verb":"comment","result":"ok","pr":1,"headSHA":null}`
	bad := `{"ts":"2026-01-01T00:00:01Z","tool":"deskpost","verb":"comm`
	if err := os.WriteFile(audit, []byte(good+"\n"+bad+"\n"), 0o600); err != nil {
		t.Fatalf("seed audit: %v", err)
	}

	var out, errb bytes.Buffer
	if code := run([]string{"recover"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("recover exit=%d (want 0); stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "carried 1") || !strings.Contains(out.String(), "quarantined 1") {
		t.Errorf("recover summary unexpected: %q", out.String())
	}

	// The live log now parses cleanly with only the good line.
	b, err := os.ReadFile(audit)
	if err != nil {
		t.Fatalf("read recovered audit: %v", err)
	}
	if strings.Contains(string(b), `"verb":"comm"`) || strings.Count(string(b), "\n") != 1 {
		t.Errorf("recovered log still holds the bad line or wrong count: %q", string(b))
	}
	// A corrupt-* sidecar exists.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(audit), "audit.jsonl.corrupt-*"))
	if len(matches) == 0 {
		t.Errorf("no quarantine sidecar written")
	}
}

// TestRecoverVerbCleanLogIsNoOp — a clean log reports "already clean" and exits OK.
func TestRecoverVerbCleanLogIsNoOp(t *testing.T) {
	audit := seedHome(t)
	good := `{"ts":"2026-01-01T00:00:00Z","tool":"deskpost","verb":"comment","result":"ok","pr":1,"headSHA":null}`
	if err := os.WriteFile(audit, []byte(good+"\n"), 0o600); err != nil {
		t.Fatalf("seed audit: %v", err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"recover"}, &out, &errb); code != deskkit.ExitOK {
		t.Fatalf("recover exit=%d (want 0); stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "already clean") {
		t.Errorf("clean-log summary unexpected: %q", out.String())
	}
}

// TestUnknownVerbRefused — an unknown verb is a refusal (exit 5), not a silent success.
func TestUnknownVerbRefused(t *testing.T) {
	seedHome(t)
	var out, errb bytes.Buffer
	if code := run([]string{"frobnicate"}, &out, &errb); code != deskkit.ExitRefused {
		t.Fatalf("unknown verb exit=%d (want %d)", code, deskkit.ExitRefused)
	}
}
