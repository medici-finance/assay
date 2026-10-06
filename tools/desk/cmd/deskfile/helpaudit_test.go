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

// TestHelpWritesNoAuditRow is the runtime half of the help retrofit for deskfile: a help
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
		{"new --help", []string{"new", "--help"}},
		{"new -h", []string{"new", "-h"}},
		{"attach --help", []string{"attach", "--help"}},
		{"check --help", []string{"check", "--help"}},
		// Tier two: help after another flag reaches the subcommand's parser as
		// flag.ErrHelp, inside the audited verb.
		{"new -R repo --help", []string{"new", "-R", allowedRepo, "--help"}},
		{"attach -R repo -h", []string{"attach", "-R", allowedRepo, "-h"}},
		{"check -R repo --help", []string{"check", "-R", allowedRepo, "--help"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t)

			before := auditBytes(t)
			rc, out := runCapture(c.args)

			if rc != deskkit.ExitOK {
				t.Errorf("%s exited %d, want %d — a help screen is a successful read, not a refusal\noutput: %s",
					c.name, rc, deskkit.ExitOK, out)
			}
			// Output ends with the usage screen, and nothing ahead of it is a refusal (the
			// pre-retrofit `refused: bad flags: flag: help requested`). The only other text
			// allowed is the unpinned-build warning a tier-two run prints under `go test`.
			trimmed := strings.TrimSpace(out)
			if !strings.HasSuffix(trimmed, strings.TrimSpace(usage)) {
				t.Errorf("%s did not end with the usage screen; output:\n%s", c.name, out)
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
		withEnv(t)

		before := auditBytes(t)
		rc, out := runCapture([]string{"new", "--no-such-flag"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("an unknown flag exited %d, want %d\noutput: %s", rc, deskkit.ExitRefused, out)
		}
		after := auditBytes(t)
		if len(after) <= len(before) {
			t.Errorf("an unknown flag wrote no audit row — real misuse must stay recorded "+
				"(before %d bytes, after %d)", len(before), len(after))
		}
	})
}
