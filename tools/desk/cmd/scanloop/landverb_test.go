package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// auditLandLines counts the audit log's `land` lines for the scanloop tool.
func auditLandLines(t *testing.T, stateDir string) int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(stateDir, "audit.jsonl"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, `"verb":"land"`) && strings.Contains(line, `"tool":"scanloop"`) {
			n++
		}
	}
	return n
}

func recordFileBytes(t *testing.T, stateDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(stateDir, intakeExitsFile))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(raw)
}

// TestLandVerb_OneExitPerItem — the verb writes one record and one audit `land` line; the same exit
// again is a no-op; a different exit, `unrouted`, a role outside the closed set, a free-text kind
// and an unparseable dispatch_ref are each refused (exit 5) before the record file changes.
func TestLandVerb_OneExitPerItem(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	t.Setenv("DESK_LOOP", "intake-desk")
	base := []string{"--item", "medici-finance/assay#77", "--artifact", "medici-finance/assay#901", "--tier", "strong"}
	land := func(extra ...string) error {
		return cmdLand(append(append([]string(nil), base...), extra...), &strings.Builder{})
	}

	if err := land("--exit", "needs-decision"); err != nil {
		t.Fatalf("first land: %v", err)
	}
	lines := readRecordLines(t, stateDir)
	if len(lines) != 1 {
		t.Fatalf("first land wrote %d record lines, want 1", len(lines))
	}
	if n := auditLandLines(t, stateDir); n != 1 {
		t.Fatalf("first land wrote %d audit land lines, want 1", n)
	}
	r, _ := decodeRecord(t, lines[0])
	if r.DecidedBy != decidedJudgment || r.TriagerRole != "intake-desk" || r.TriagerTier != tierStrong ||
		r.Exit != ExitNeedsDecision || r.Repo != "medici-finance/assay" {
		t.Fatalf("record = %+v", r)
	}
	before := recordFileBytes(t, stateDir)

	if err := land("--exit", "needs-decision"); err != nil {
		t.Fatalf("same exit again must be a no-op (exit 0): %v", err)
	}
	if recordFileBytes(t, stateDir) != before || auditLandLines(t, stateDir) != 1 {
		t.Fatal("same exit again wrote a line")
	}

	refusals := []struct {
		name string
		env  string
		args []string
	}{
		{"different exit", "", []string{"--exit", "bug"}},
		{"unrouted", "", []string{"--item", "medici-finance/assay#78", "--exit", "unrouted"}},
		{"role alice-dev", "alice-dev", []string{"--item", "medici-finance/assay#79", "--exit", "bug"}},
		{"role unset", "-", []string{"--item", "medici-finance/assay#79", "--exit", "bug"}},
		{"free-text kind", "", []string{"--item", "medici-finance/assay#80", "--exit", "bug", "--kind", "looks like a bug"}},
		{"bad dispatch-ref", "", []string{"--item", "medici-finance/assay#81", "--exit", "bug", "--dispatch-ref", "assay--x@yesterday"}},
		{"vendor tier", "", []string{"--item", "medici-finance/assay#82", "--exit", "bug", "--tier", "claude-opus"}},
		{"free-text artifact", "", []string{"--item", "medici-finance/assay#83", "--exit", "bug", "--artifact", "a new issue"}},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.env {
			case "":
			case "-":
				t.Setenv("DESK_LOOP", "")
			default:
				t.Setenv("DESK_LOOP", tc.env)
			}
			err := land(tc.args...)
			if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (refused): %v", code, deskkit.ExitRefused, err)
			}
			if recordFileBytes(t, stateDir) != before {
				t.Fatal("a refused land changed the record file")
			}
			if auditLandLines(t, stateDir) != 1 {
				t.Fatal("a refused land wrote an audit land line")
			}
		})
	}
}

// TestLandVerb_ClosedSetAccepted — every desk loop in the closed set may land; `driver` may not,
// because the verb's role is the running loop's and a human stamps the register by hand.
func TestLandVerb_ClosedSetAccepted(t *testing.T) {
	for i, role := range intakeExitRoles {
		t.Run(role, func(t *testing.T) {
			isolatedDeskHome(t)
			t.Setenv("DESK_LOOP", role)
			err := cmdLand([]string{"--item", "medici-finance/assay#" + string(rune('1'+i)), "--exit", "bug",
				"--artifact", "#5", "--tier", "any"}, &strings.Builder{})
			if role == "driver" {
				if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
					t.Fatalf("DESK_LOOP=driver was accepted: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("closed-set loop %q refused: %v", role, err)
			}
		})
	}
}
