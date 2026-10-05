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

// TestTierTwoHelpNoRow: `desktoken <role> --repo <r> --help` (and the coverage form) is a
// MULTI-token help request, so tier one does not catch it and it reaches the subcommand's own
// parse, where parseInterspersed returns flag.ErrHelp. It must print the help screen, exit 0
// and leave the audit ledger byte-for-byte untouched — until this it was recorded as
// `refused: bad flags: flag: help requested`, exit 5, one row. The control keeps genuine
// misuse refused AND recorded.
func TestTierTwoHelpNoRow(t *testing.T) {
	for _, args := range [][]string{
		{"reviewer", "--repo", "example-org/tracker", "--help"},
		{"reviewer", "-h"},
		{"--repo", "example-org/tracker", "reviewer", "--help"},
		{"coverage", "reviewer", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			setupTest(t)
			before := helpAuditBytes(t)
			rc, _, stderr := runCap(t, args)
			if rc != deskkit.ExitOK {
				t.Errorf("exit %d, want %d — a help screen is a successful read, not a refusal", rc, deskkit.ExitOK)
			}
			if !strings.Contains(stderr, "desktoken") {
				t.Errorf("no help screen on stderr: %q", stderr)
			}
			if after := helpAuditBytes(t); string(after) != string(before) {
				t.Errorf("help appended to the audit ledger: added %q", strings.TrimPrefix(string(after), string(before)))
			}
		})
	}

	t.Run("a bad flag still refuses and still audits", func(t *testing.T) {
		setupTest(t)
		before := helpAuditBytes(t)
		if rc, _, _ := runCap(t, []string{"reviewer", "--repo", "example-org/tracker", "--no-such-flag"}); rc != deskkit.ExitRefused {
			t.Fatalf("an unknown flag exited %d, want %d", rc, deskkit.ExitRefused)
		}
		if after := helpAuditBytes(t); len(after) <= len(before) {
			t.Errorf("an unknown flag wrote no audit row — real misuse must stay recorded")
		}
	})
}
