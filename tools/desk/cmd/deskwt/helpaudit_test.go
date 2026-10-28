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
// request — `<verb> --help` alone, and help after a positional or another flag — exits 0,
// prints that command's own help on stdout, and leaves the audit ledger byte-identical. The
// controls in the same position must still behave: a malformed command line is refused with
// the usage code before any effect (no audit row, as with every parse error since the Cobra
// port), and a real domain refusal still refuses AND still appends its row, so going quiet on
// misuse fails the test too.
func TestHelpWritesNoAuditRow(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string // a line of the command's own help
	}{
		{"add --help", []string{"add", "--help"}, "deskwt add <name>"},
		{"add -h", []string{"add", "-h"}, "deskwt add <name>"},
		{"remove --help", []string{"remove", "--help"}, "deskwt remove <path>"},
		{"prune --help", []string{"prune", "--help"}, "deskwt prune"},
		{"maintain --help", []string{"maintain", "--help"}, "deskwt maintain"},
		{"role-init --help", []string{"role-init", "--help"}, "deskwt role-init"},
		{"role-clean --help", []string{"role-clean", "--help"}, "deskwt role-clean"},
		{"add name --help", []string{"add", "wt-name", "--help"}, "deskwt add <name>"},
		{"add --dry-run -h", []string{"add", "--dry-run", "-h"}, "deskwt add <name>"},
		{"remove name --help", []string{"remove", "wt-name", "--help"}, "deskwt remove <path>"},
		{"maintain --apply --help", []string{"maintain", "--apply", "--help"}, "deskwt maintain"},
		{"maintain objects --help", []string{"maintain", "objects", "--help"}, "deskwt maintain objects"},
		{"maintain objects --apply --help", []string{"maintain", "objects", "--apply", "--help"}, "deskwt maintain objects"},
		{"help add", []string{"help", "add"}, "deskwt add <name>"},
	} {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, t.TempDir())

			before := auditBytes(t)
			rc, stdout, stderr := runCapBoth(t, c.args)

			if rc != deskkit.ExitOK {
				t.Errorf("%s exited %d, want %d — a help screen is a successful read, not a refusal\nstderr: %s",
					c.name, rc, deskkit.ExitOK, stderr)
			}
			if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, c.want) {
				t.Errorf("%s did not print its own help (want %q); stdout:\n%s", c.name, c.want, stdout)
			}
			if strings.Contains(stderr, "refused") || strings.Contains(stderr, "help requested") {
				t.Errorf("%s printed a refusal: %q", c.name, stderr)
			}
			after := auditBytes(t)
			if string(after) != string(before) {
				t.Errorf("%s appended to the audit ledger:\nbefore %d bytes, after %d bytes\nadded: %q",
					c.name, len(before), len(after), strings.TrimPrefix(string(after), string(before)))
			}
		})
	}

	// CONTROL ONE. A bad flag is a malformed command line: the usage code, refused before
	// the kill switch, the roster or the audit ledger is touched.
	t.Run("bad flag is a usage error with no effect", func(t *testing.T) {
		withEnv(t, t.TempDir())

		before := auditBytes(t)
		rc, stderr := runCapErr(t, []string{"add", "--no-such-flag"})
		if rc != 2 {
			t.Fatalf("an unknown flag exited %d, want 2 (usage)\nstderr: %s", rc, stderr)
		}
		if !strings.Contains(stderr, "unknown flag: --no-such-flag") {
			t.Errorf("an unknown flag did not name itself; stderr:\n%s", stderr)
		}
		if after := auditBytes(t); string(after) != string(before) {
			t.Errorf("a parse error reached the audit ledger (before %d bytes, after %d)", len(before), len(after))
		}
	})

	// CONTROL TWO. A well-formed verb the domain refuses still refuses AND still audits.
	t.Run("domain refusal still refuses and audits", func(t *testing.T) {
		withEnv(t, newRepo(t))

		before := auditBytes(t)
		rc, stderr := runCapErr(t, []string{"add", "../escape"})
		if rc != deskkit.ExitRefused {
			t.Fatalf("an escaping name exited %d, want %d\nstderr: %s", rc, deskkit.ExitRefused, stderr)
		}
		after := auditBytes(t)
		if len(after) <= len(before) {
			t.Errorf("a domain refusal wrote no audit row — real misuse must stay recorded "+
				"(before %d bytes, after %d)", len(before), len(after))
		}
	})
}
