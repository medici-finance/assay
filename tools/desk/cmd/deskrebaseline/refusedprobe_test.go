package main

import (
	"strings"
	"testing"
)

// TestRefusedRenameNeverRuns — gatherRowFacts has two execution paths, the
// rename-proof re-run (#1953) and the behaviour probe, and both sit behind one
// refusal gate. A cell statusgen refuses that is not a comment (a no-op, marked
// or not, or a prose mention of the pinned file) is never executed: not as
// rename proof against a missing pinned path with a single rename hop, and not
// as a behaviour probe against a path that exists. A row built without a cell
// is judged on its lift. Each refused row must stay off safe:rename.
// regression: #3147
func TestRefusedRenameNeverRuns(t *testing.T) {
	root := renameFixture(t, "// needle\n")
	orig := runCommand
	var probed []string
	runCommand = func(_ string, command string) (bool, int, string) {
		probed = append(probed, command)
		return true, 0, ""
	}
	t.Cleanup(func() { runCommand = orig })

	refused := []string{
		"`: test -f %s`",
		"`: test -f %s`: confirm it",
		"`cmd: : test -f %s`",
		"`REM test -f %s`",
		"`:: test -f %s`",
		"`%s` holds the needle",
	}
	for _, path := range []string{"tools/old/w.go", "tools/new/w.go"} {
		for _, shape := range refused {
			cell := strings.ReplaceAll(shape, "%s", path)
			row := parseVerifyRows(verifyHead + "| 1 | check | " + cell + " | exit 0 |\n")[0]
			if row.Refused == "" {
				t.Fatalf("positive control: %q must be refused, lift %q", cell, row.Command)
			}
			f := gatherRowFacts(root, row, false, "")
			checkUnprobed(t, cell, path, f, &probed)
		}
		// A caller constructing a row directly is judged on the lift itself.
		for _, cmd := range []string{": test -f " + path, "REM test -f " + path, "   "} {
			f := gatherRowFacts(root, verifyRow{Num: 1, Command: cmd, ExpectCell: "exit 0"}, false, "")
			checkUnprobed(t, "direct "+cmd, path, f, &probed)
		}
	}

	// Positive controls: a real command at each site still runs, and an escaped
	// pipe after a colon is a pipeline into a real command, not a no-op.
	row := parseVerifyRows(verifyHead + "| 1 | check | `: \\| test -f tools/old/w.go` | exit 0 |\n")[0]
	if got := Classify(gatherRowFacts(root, row, false, "")); len(probed) != 1 ||
		probed[0] != ": | test -f tools/new/w.go" || got.Verdict != SafeRename {
		t.Fatalf("positive control: rename re-run probed=%q verdict=%s", probed, got.Verdict)
	}
	row = parseVerifyRows(verifyHead + "| 1 | check | `test -f tools/new/w.go` | exit 0 |\n")[0]
	if f := gatherRowFacts(root, row, false, ""); !f.CommandProbed || len(probed) != 2 {
		t.Fatalf("positive control: behaviour probe probed=%q facts=%+v", probed, f)
	}
}

// checkUnprobed fails when a refused row reached either probe. Against the
// missing path it also checks the rename hop was found, so the row really
// reached the rename-proof site and was stopped there.
func checkUnprobed(t *testing.T, label, path string, f RowFacts, probed *[]string) {
	t.Helper()
	if path == "tools/old/w.go" && strings.Contains(f.Command, path) && f.RenameHop != "tools/new/w.go" {
		t.Fatalf("positive control: %s: rename hop = %q, want tools/new/w.go", label, f.RenameHop)
	}
	if path == "tools/new/w.go" && strings.Contains(f.Command, path) && !f.PathExists {
		t.Fatalf("positive control: %s: pinned path should exist, facts %+v", label, f)
	}
	if got := Classify(f); len(*probed) != 0 || f.CommandProbed || f.RenameProbe != "" || got.Verdict == SafeRename {
		t.Errorf("%s: a refused row was executed: probed=%q facts=%+v verdict=%s", label, *probed, f, got.Verdict)
	}
	*probed = nil
}
