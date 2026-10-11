package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// exitevent_test.go — the intake-exit record's per-event key, and what a record failure may and may
// not cost: never an audit line, never another batch member's record.

// plantRecord writes one record straight into the record file, as an earlier session would have.
func plantRecord(t *testing.T, r IntakeExitRecord) {
	t.Helper()
	if _, err := appendExitRecord(r, nil); err != nil {
		t.Fatalf("planting a record: %v", err)
	}
}

func judgmentFixture(item, opened string, exit Exit) IntakeExitRecord {
	return IntakeExitRecord{
		Schema:      intakeExitSchema,
		Source:      "issue",
		Item:        item,
		Repo:        repoOfItemID(item),
		Exit:        exit,
		Artifact:    "#900",
		DecidedBy:   decidedJudgment,
		TriagerRole: "intake-desk",
		TriagerTier: tierStrong,
		Opened:      opened,
		Triaged:     "2026-08-24T11:59:00Z",
	}
}

// twoMemberPass drains #170 and #171 as new issues through ONE batched scan dispatch.
func twoMemberPass(t *testing.T) (*ScanLoop, error, string) {
	t.Helper()
	loop := batchLoop(t,
		"INBOUND: medici-finance/assay#170 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#171 2026-08-24T11:51:00Z\n", newFakeGit())
	var sb strings.Builder
	err := drainPass(passConfig(t), loop, &sb)
	return loop, err, sb.String()
}

// TestExitEvent_RecordFailureNeverCostsAuditOrBatch — Land writes every member's ledger entry and audit
// `land` line first; a refused or unwritable intake-exit record costs that member its record only.
//
// regression: F-audit-line-gated-on-record — the record was written BEFORE the audit line and its
// error returned, so a refused record suppressed the audit line for an exit that had happened.
// regression: C-land-batch-abort — that return also aborted the loop, so one member's refusal cost
// every later batch member its ledger entry, audit line and record.
func TestExitEvent_RecordFailureNeverCostsAuditOrBatch(t *testing.T) {
	t.Run("a conflicting earlier record for the first member", func(t *testing.T) {
		stateDir := isolatedDeskHome(t)
		// The same EVENT (#170 opened 11:50) already left by bug: the placeholder record is refused.
		plantRecord(t, judgmentFixture("medici-finance/assay#170", "2026-08-24T11:50:00Z", ExitBug))
		loop, err, out := twoMemberPass(t)
		if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (refused, reported after the batch landed): %v\n%s", code, deskkit.ExitRefused, err, out)
		}
		if !strings.Contains(err.Error(), "medici-finance/assay#170") {
			t.Fatalf("the error does not name the refused member: %v", err)
		}
		if n := len(loop.Ledger().Records()); n != 2 {
			t.Fatalf("ledger has %d entries, want both members landed", n)
		}
		if n := auditLandLines(t, stateDir); n != 2 {
			t.Fatalf("%d audit land lines, want one per landed member (2)", n)
		}
		lines := readRecordLines(t, stateDir)
		if len(lines) != 2 {
			t.Fatalf("record file has %d lines, want the planted one plus #171's:\n%s", len(lines), strings.Join(lines, "\n"))
		}
		if r, _ := decodeRecord(t, lines[1]); r.Item != "medici-finance/assay#171" || r.Exit != ExitPlaceholder {
			t.Fatalf("the second member's record = %+v", r)
		}
	})

	t.Run("an unreadable record file", func(t *testing.T) {
		stateDir := isolatedDeskHome(t)
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, intakeExitsFile), []byte("not json\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		loop, err, out := twoMemberPass(t)
		if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
			t.Fatalf("exit = %d, want %d (could-not-check): %v\n%s", code, deskkit.ExitUnverifiable, err, out)
		}
		if n := len(loop.Ledger().Records()); n != 2 {
			t.Fatalf("ledger has %d entries, want both members landed", n)
		}
		if n := auditLandLines(t, stateDir); n != 2 {
			t.Fatalf("%d audit land lines, want one per landed member (2)", n)
		}
	})

	t.Run("an earlier record for a different event of the same item", func(t *testing.T) {
		stateDir := isolatedDeskHome(t)
		// #170 left by judgment at an EARLIER event, and once at an unknown one: neither is this
		// pass's event, so neither blocks this pass's record.
		plantRecord(t, judgmentFixture("medici-finance/assay#170", "2026-08-20T09:00:00Z", ExitBug))
		plantRecord(t, judgmentFixture("medici-finance/assay#170", "", ExitBug))
		_, err, out := twoMemberPass(t)
		if err != nil {
			t.Fatalf("pass errored: %v\n%s", err, out)
		}
		if n := len(readRecordLines(t, stateDir)); n != 4 {
			t.Fatalf("record file has %d lines, want the two planted plus both members' (4)", n)
		}
		if n := auditLandLines(t, stateDir); n != 2 {
			t.Fatalf("%d audit land lines, want 2", n)
		}
	})
}

// TestExitEvent_UndeclaredLoopRefused — through the real command path, a session that declared no
// loop is refused: the drain's own DESK_LOOP default is not a statement about who decided an exit.
//
// regression: F-land-undeclared-loop-default — run() filled DESK_LOOP with intake-desk before the
// land verb read it, so an undeclared session wrote triager_role=intake-desk.
func TestExitEvent_UndeclaredLoopRefused(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	t.Setenv("DESK_LOOP", "") // restored after the test; run() sets it in-process
	code := run([]string{"land", "--item", "medici-finance/assay#77", "--opened", "2026-08-24T11:50:00Z",
		"--exit", "bug", "--artifact", "#5", "--tier", "strong"})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused) for an undeclared loop", code, deskkit.ExitRefused)
	}
	if recordFileBytes(t, stateDir) != "" || auditLandLines(t, stateDir) != 0 {
		t.Fatal("an undeclared land wrote a record or an audit line")
	}

	t.Setenv("DESK_LOOP", "verify-desk")
	if code := run([]string{"land", "--item", "medici-finance/assay#77", "--opened", "2026-08-24T11:50:00Z",
		"--exit", "bug", "--artifact", "#5", "--tier", "strong"}); code != deskkit.ExitOK {
		t.Fatalf("a declared loop was refused: exit %d", code)
	}
	lines := readRecordLines(t, stateDir)
	if len(lines) != 1 {
		t.Fatalf("record lines = %d, want 1", len(lines))
	}
	if r, _ := decodeRecord(t, lines[0]); r.TriagerRole != "verify-desk" {
		t.Fatalf("triager_role = %q, want the declared verify-desk", r.TriagerRole)
	}
}

// TestExitEvent_AuditFailureRetryWritesBoth — the audit line is written before the record, under the
// same lock, so a failed audit write leaves no record and the retry writes both.
//
// regression: C-land-retry-audit-gap — the record was written first, so after a failed audit write
// the retry saw the same exit already recorded, returned a no-op, and the audit line never existed.
func TestExitEvent_AuditFailureRetryWritesBoth(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	auditPath := filepath.Join(stateDir, "audit.jsonl")
	// A directory where the audit log goes makes the audit append fail.
	if err := os.Mkdir(auditPath, 0o700); err != nil {
		t.Fatal(err)
	}
	args := []string{"--item", "medici-finance/assay#77", "--opened", "2026-08-24T11:50:00Z",
		"--exit", "bug", "--artifact", "#5", "--tier", "strong"}
	if err := cmdLand(args, "intake-desk", &strings.Builder{}); err == nil {
		t.Fatal("land succeeded with an unwritable audit log")
	}
	if got := recordFileBytes(t, stateDir); got != "" {
		t.Fatalf("a land whose audit line failed left a record:\n%s", got)
	}
	if err := os.Remove(auditPath); err != nil {
		t.Fatal(err)
	}
	if err := cmdLand(args, "intake-desk", &strings.Builder{}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if n := len(readRecordLines(t, stateDir)); n != 1 {
		t.Fatalf("after the retry the record file has %d lines, want 1", n)
	}
	if n := auditLandLines(t, stateDir); n != 1 {
		t.Fatalf("after the retry there are %d audit land lines, want 1", n)
	}
}

// TestExitEvent_Key — the key is item + opened, the item compared without regard to case; an
// --opened that does not parse, a stray argument and a bad flag are refused with clipped echoes.
//
// regression: C-item-case-identity — the duplicate check compared items byte for byte, so the same
// issue spelt with different case could leave by a second exit.
func TestExitEvent_Key(t *testing.T) {
	stateDir := isolatedDeskHome(t)
	land := func(args ...string) error {
		return cmdLand(append([]string{"--artifact", "#5", "--tier", "strong"}, args...), "intake-desk", &strings.Builder{})
	}
	if err := land("--item", "medici-finance/assay#77", "--opened", "2026-08-24T11:50:00Z", "--exit", "bug"); err != nil {
		t.Fatalf("first land: %v", err)
	}
	before := recordFileBytes(t, stateDir)

	refusals := []struct {
		name string
		args []string
	}{
		{"same event, different case, different exit",
			[]string{"--item", "Medici-Finance/Assay#77", "--opened", "2026-08-24T11:50:00Z", "--exit", "needs-decision"}},
		{"same event, offset spelling of the same instant, different exit",
			[]string{"--item", "medici-finance/assay#77", "--opened", "2026-08-24T13:50:00+02:00", "--exit", "needs-decision"}},
		{"opened does not parse", []string{"--item", "medici-finance/assay#78", "--opened", "yesterday", "--exit", "bug"}},
		{"stray argument", []string{"--item", "medici-finance/assay#78", "--exit", "bug", strings.Repeat("x", 200)}},
		{"unknown flag", []string{"--item", "medici-finance/assay#78", "--exit", "bug", "--" + strings.Repeat("y", 200)}},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			err := land(tc.args...)
			if code := deskkit.ExitCodeOf(err); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d (refused): %v", code, deskkit.ExitRefused, err)
			}
			if len(err.Error()) > 200 {
				t.Fatalf("the refusal echoes %d bytes of input; it must be clipped: %v", len(err.Error()), err)
			}
			if recordFileBytes(t, stateDir) != before {
				t.Fatal("a refused land changed the record file")
			}
		})
	}

	t.Run("a later event of the same item takes its own exit", func(t *testing.T) {
		if err := land("--item", "medici-finance/assay#77", "--opened", "2026-08-25T08:00:00Z", "--exit", "needs-decision"); err != nil {
			t.Fatalf("a later event was refused: %v", err)
		}
		if n := len(readRecordLines(t, stateDir)); n != 2 {
			t.Fatalf("record lines = %d, want 2", n)
		}
	})
}
