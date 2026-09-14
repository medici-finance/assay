package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// auditBytes returns the exact contents of this fixture HOME's audit ledger, or nil when it
// does not exist. "Does not exist" and "exists and is empty" are different facts and the
// assertions below keep them apart.
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

// TestHelpWritesNoAuditRow.
//
// A SUBCOMMAND help request used to be reported as `refused: bad flags: flag: help requested`
// — exit 5, and one row appended to a ledger that ratelimit.go counts per tool for the write
// budget and the circuit breaker and that audit.go never rotates. Measured on one operating
// desk host over 32 days: 1,043 such rows, 538 of them this verb's.
//
// The assertion is BYTE equality of the ledger across the run, not merely "no new deskpr
// row": a help screen must leave the file exactly as it found it.
//
// The bad-flag control in the same position is what stops this from being a regression in the
// other direction. Silencing genuine misuse would be a worse defect than the one being fixed,
// and the two differ only in the token typed.
func TestHelpWritesNoAuditRow(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
	}{
		{"create --help", []string{"create", "--help"}},
		{"create -h", []string{"create", "-h"}},
		{"update --help", []string{"update", "--help"}},
		{"edit --help", []string{"edit", "--help"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			work := t.TempDir()
			withEnv(t, work)

			before := auditBytes(t)
			rc := run(c.args)

			if rc != deskkit.ExitOK {
				t.Errorf("%s exited %d, want %d — a help screen is a successful read, not a refusal",
					c.name, rc, deskkit.ExitOK)
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
	t.Run("a bad flag in the same position still refuses and still audits", func(t *testing.T) {
		work := t.TempDir()
		withEnv(t, work)

		before := auditBytes(t)
		if rc := run([]string{"create", "--no-such-flag"}); rc != deskkit.ExitRefused {
			t.Fatalf("an unknown flag exited %d, want %d", rc, deskkit.ExitRefused)
		}
		after := auditBytes(t)
		if len(after) <= len(before) {
			t.Errorf("an unknown flag wrote no audit row — real misuse must stay recorded "+
				"(before %d bytes, after %d)", len(before), len(after))
		}
	})
}

// Tier one returns BEFORE the kill-switch gate, so it must not become a way past it — and
// equally it must not be so eager that it swallows a real invocation. This asserts the one
// shape that would be catastrophic: a `--help` that is another flag's VALUE must NOT be
// treated as a help request, so the run proceeds into the verb's own parsing and reaches a
// real outcome rather than silently printing usage and exiting 0.
func TestHelpAsAFlagValueIsNotAHelpScreen(t *testing.T) {
	work := t.TempDir()
	withEnv(t, work)

	// `--title --help` makes `--help` the title's VALUE. Whatever this run does, it must not
	// be "print usage and exit 0 having done nothing" — that would silently swallow a real
	// create with an odd title.
	rc := run([]string{"create", "--title", "--help"})
	if rc == deskkit.ExitOK {
		t.Errorf("`create --title --help` exited %d — a --help that is a flag's VALUE was treated "+
			"as a help request, which silently swallows a real invocation", rc)
	}
}
