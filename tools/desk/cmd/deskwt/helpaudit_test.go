package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// auditBytes returns the exact contents of this fixture HOME's audit ledger, or nil when it
// does not exist. "Does not exist" and "exists and is empty" are different facts; the
// assertions below compare bytes, so they keep them apart.
func auditBytes(t *testing.T) []byte {
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

// TestHelpWritesNoAuditRow is the runtime half of the help retrofit for deskwt: a help
// request — the single-token tier-one shape AND the wider tier-two shapes that reach the
// subcommand's own flag parse — exits 0, prints the usage screen, and leaves the audit
// ledger byte-identical. The bad-flag control in the same position must still refuse
// (exit 5) AND still append its row, so going quiet on real misuse fails the test too.
func TestHelpWritesNoAuditRow(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
	}{
		// Tier one: `<sub> --help` alone, answered before Guard.
		{"add --help", []string{"add", "--help"}},
		{"add -h", []string{"add", "-h"}},
		{"remove --help", []string{"remove", "--help"}},
		{"prune --help", []string{"prune", "--help"}},
		{"role-init --help", []string{"role-init", "--help"}},
		{"role-clean --help", []string{"role-clean", "--help"}},
		// Tier two: help after a positional or another flag reaches the subcommand's
		// parser as flag.ErrHelp, inside the audited verb.
		{"add name --help", []string{"add", "wt-name", "--help"}},
		{"add --dry-run -h", []string{"add", "--dry-run", "-h"}},
		{"remove name --help", []string{"remove", "wt-name", "--help"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, t.TempDir())

			before := auditBytes(t)
			rc, stderr := runCapErr(t, c.args)

			if rc != deskkit.ExitOK {
				t.Errorf("%s exited %d, want %d — a help screen is a successful read, not a refusal\nstderr: %s",
					c.name, rc, deskkit.ExitOK, stderr)
			}
			// stderr ends with the usage screen and carries no refusal line (the pre-retrofit
			// `refused: bad flags: flag: help requested`). The only other line allowed is the
			// unpinned-build warning a tier-two run prints after Guard under `go test`.
			trimmed := strings.TrimSpace(stderr)
			if !strings.HasSuffix(trimmed, strings.TrimSpace(usage)) {
				t.Errorf("%s did not end with the usage screen; stderr:\n%s", c.name, stderr)
			} else if lead := strings.TrimSuffix(trimmed, strings.TrimSpace(usage)); strings.Contains(lead, "refused") ||
				strings.Contains(lead, "help requested") {
				t.Errorf("%s printed a refusal ahead of the usage screen: %q", c.name, lead)
			}
			after := auditBytes(t)
			if string(after) != string(before) {
				t.Errorf("%s appended to the audit ledger:\nbefore %d bytes, after %d bytes\nadded: %q",
					c.name, len(before), len(after), strings.TrimPrefix(string(after), string(before)))
			}
		})
	}

	// THE CONTROL. A genuinely bad flag in the same position must still refuse AND still
	// write its row: the fix is about help screens, not about going quiet on misuse.
	t.Run("bad flag still refuses and audits", func(t *testing.T) {
		withEnv(t, t.TempDir())

		before := auditBytes(t)
		rc, stderr := runCapErr(t, []string{"add", "--no-such-flag"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("an unknown flag exited %d, want %d\nstderr: %s", rc, deskkit.ExitRefused, stderr)
		}
		after := auditBytes(t)
		if len(after) <= len(before) {
			t.Errorf("an unknown flag wrote no audit row — real misuse must stay recorded "+
				"(before %d bytes, after %d)", len(before), len(after))
		}
	})
}
