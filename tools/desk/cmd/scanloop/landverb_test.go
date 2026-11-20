package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskcore/issueflow"
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
		return cmdLand(append(append([]string(nil), base...), extra...), os.Getenv("DESK_LOOP"), &strings.Builder{})
	}

	ok := t.Run("first land writes one record and one audit line", func(t *testing.T) {
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
	})
	if !ok {
		t.FailNow()
	}
	before := recordFileBytes(t, stateDir)

	t.Run("same exit again is a no-op", func(t *testing.T) {
		if err := land("--exit", "needs-decision"); err != nil {
			t.Fatalf("same exit again must be a no-op (exit 0): %v", err)
		}
		if recordFileBytes(t, stateDir) != before || auditLandLines(t, stateDir) != 1 {
			t.Fatal("same exit again wrote a line")
		}
	})

	refusals := []struct {
		name string
		env  string
		args []string
	}{
		{"different exit", "", []string{"--exit", "finding"}},
		{"unrouted", "", []string{"--item", "medici-finance/assay#78", "--exit", "unrouted"}},
		{"role alice-dev", "alice-dev", []string{"--item", "medici-finance/assay#79", "--exit", "finding"}},
		{"role unset", "-", []string{"--item", "medici-finance/assay#79", "--exit", "finding"}},
		{"free-text kind", "", []string{"--item", "medici-finance/assay#80", "--exit", "finding", "--kind", "looks like a bug"}},
		{"bad dispatch-ref", "", []string{"--item", "medici-finance/assay#81", "--exit", "finding", "--dispatch-ref", "assay--x@yesterday"}},
		{"vendor tier", "", []string{"--item", "medici-finance/assay#82", "--exit", "finding", "--tier", "claude-opus"}},
		{"free-text artifact", "", []string{"--item", "medici-finance/assay#83", "--exit", "finding", "--artifact", "a new issue"}},
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
			err := cmdLand([]string{"--item", "medici-finance/assay#" + string(rune('1'+i)), "--exit", "finding",
				"--artifact", "#5", "--tier", "any"}, role, &strings.Builder{})
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

// TestLandRefusesBareJudgedExit — a bug or a placeholder recorded with no --handoff is refused
// (exit 5) before anything is written; the three exits that discharge without a companion, and a
// call that supplies one, still land.
//
// regression: F-judged-exit-no-handoff — land recorded `bug` and `placeholder` with no judgment, so
// the issue was parked with a placeholder the board could not assess and nothing published one.
func TestLandRefusesBareJudgedExit(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	t.Setenv("DESK_LOOP", "intake-desk")
	land := func(n int, extra ...string) error {
		args := append([]string{"--item", fmt.Sprintf("medici-finance/assay#%d", n),
			"--opened", "2026-08-24T11:50:00Z", "--artifact", "medici-finance/assay#901", "--tier", "strong"}, extra...)
		return cmdLand(args, "intake-desk", &strings.Builder{})
	}

	for i, exit := range []string{"bug", "placeholder"} {
		t.Run("refused "+exit, func(t *testing.T) {
			err := land(300+i, "--exit", exit)
			if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (refused): %v", code, deskkit.ExitRefused, err)
			}
			for _, want := range []string{"--handoff", "--handoff-reader"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("the refusal does not name %s: %v", want, err)
				}
			}
			if recordFileBytes(t, stateDir) != "" || auditLandLines(t, stateDir) != 0 {
				t.Fatal("a refused land wrote a record or an audit line")
			}
		})
	}

	for i, exit := range []string{"finding", "needs-decision", "rejected-watching"} {
		t.Run("exempt "+exit, func(t *testing.T) {
			extra := []string{"--exit", exit}
			if exit == "rejected-watching" {
				extra = append(extra, "--detail", "watching")
			}
			if err := land(310+i, extra...); err != nil {
				t.Fatalf("%s was refused: %v", exit, err)
			}
		})
	}
	if n := len(readRecordLines(t, stateDir)); n != 3 {
		t.Fatalf("record lines = %d after the three exempt exits, want 3", n)
	}

	t.Run("a bug with a handoff lands", func(t *testing.T) {
		root := t.TempDir()
		oldRoot := handoffRoot
		handoffRoot = func() (string, error) { return root, nil }
		t.Cleanup(func() { handoffRoot = oldRoot })
		briefDir := filepath.Join(root, "docs", "streams", "repair")
		if err := os.MkdirAll(briefDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(briefDir, "brief-01-author.md"), []byte("---\nbrief: repair/01\nexec-tier: strong\n---\nAuthor the scoped work.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		h := issueflow.IntakeHandoff{Schema: issueflow.IntakeHandoffSchema, Item: "medici-finance/assay#320", Opened: "2026-08-24T11:50:00Z",
			Revision: 1, DecidingRole: "intake-desk", Disposition: issueflow.Repair, Route: "repair", Successor: "repair/01", Evidence: "medici-finance/assay#320"}
		raw, err := json.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		companion := filepath.Join(t.TempDir(), "handoff.json")
		if err := os.WriteFile(companion, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if err := land(320, "--exit", "bug", "--handoff", companion, "--handoff-reader", issueflow.IntakeHandoffSchema); err != nil {
			t.Fatalf("a bug with a handoff was refused: %v", err)
		}
	})
}
