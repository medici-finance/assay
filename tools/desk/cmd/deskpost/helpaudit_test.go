package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// helpAuditBytes returns the exact contents of this fixture HOME's audit ledger, or nil when
// it does not exist ("absent" and "empty" are different facts).
func helpAuditBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read audit: %v", err)
	}
	return b
}

// TestTierTwoHelpNoRow: `deskpost <verb> <owner/repo> <n> --help` is a MULTI-token help
// request, so tier one does not catch it and it reaches the verb's own flag parse, where it
// used to return exit 2 (a usage error) like any bad flag. It must print the help screen,
// exit 0 and leave the audit ledger untouched, for EVERY verb that parses flags. The control
// keeps a genuinely bad flag a usage error (exit 2) that posts nothing.
func TestTierTwoHelpNoRow(t *testing.T) {
	for _, args := range [][]string{
		{"review", "example-org/tracker", "1", "--help"},
		{"security-review", "example-org/tracker", "1", "-h"},
		{"comment", "example-org/tracker", "1", "--body-file", "b.md", "--help"},
		{"ready", "example-org/tracker", "1", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f, errBuf := setupFake(t)
			before := helpAuditBytes(t)
			if code := run(args); code != deskkit.ExitOK {
				t.Errorf("exit %d, want %d — a help screen is a successful read, not a usage error", code, deskkit.ExitOK)
			}
			if !strings.Contains(errBuf.String(), "deskpost") {
				t.Errorf("no help screen on stderr: %q", errBuf.String())
			}
			if after := helpAuditBytes(t); string(after) != string(before) {
				t.Errorf("help appended to the audit ledger: added %q", strings.TrimPrefix(string(after), string(before)))
			}
			if len(f.hits) != 0 {
				t.Errorf("a help request touched the forge: %v", f.hits)
			}
		})
	}

	t.Run("a bad flag is still a usage error", func(t *testing.T) {
		f, _ := setupFake(t)
		if code := run([]string{"ready", "example-org/tracker", "1", "--no-such-flag"}); code != 2 {
			t.Fatalf("an unknown flag exited %d, want 2", code)
		}
		if len(f.hits) != 0 {
			t.Errorf("a usage error touched the forge: %v", f.hits)
		}
	})
}
