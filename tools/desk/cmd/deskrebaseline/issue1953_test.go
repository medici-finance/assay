package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Fail-first tests for #1953. Each one goes through parseVerifyRows (and, for the probe tests,
// gatherRowFacts and Classify) so it compiles against the pre-fix package and reds there.

const verifyHead = "# B\n\n## Verify\n\n| # | Class | Command | Expect |\n|---|---|---|---|\n"

// TestEscapedPipeStaysInCommand — defect 1. A `\|` in the Command cell is cell content, so
// the probe runs the whole pipeline, not the text before the first escaped pipe.
//
// regression: #1953
func TestEscapedPipeStaysInCommand(t *testing.T) {
	rows := parseVerifyRows(verifyHead +
		"| 1 | check | `grep -c a f.txt \\| wc -l \\|\\| true` | exit 0 |\n")
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	if want := "grep -c a f.txt | wc -l || true"; rows[0].Command != want {
		t.Fatalf("Command = %q, want %q", rows[0].Command, want)
	}
	if rows[0].Expect != "exit 0" {
		t.Fatalf("Expect = %q, want the cell after the escaped pipes", rows[0].Expect)
	}
}

// TestRenameRerunsAgainstTarget — defect 2. The pinned file moved by one rename hop, but the
// renamed file no longer carries the row's pattern: re-run against the hop, the row fails, so
// it is refused and never re-baselined safe:rename.
//
// regression: #1953
func TestRenameRerunsAgainstTarget(t *testing.T) {
	root := gitRepo(t)
	body := strings.Repeat("// filler line for rename detection\n", 30)
	writeFile(t, root, "tools/old/w.go", "package old\n// needle\n"+body)
	gitCommit(t, root, "add w")
	if err := os.MkdirAll(filepath.Join(root, "tools/new"), 0o755); err != nil {
		t.Fatal(err)
	}
	mv := exec.Command("git", "mv", "tools/old/w.go", "tools/new/w.go")
	mv.Dir = root
	if out, err := mv.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
	writeFile(t, root, "tools/new/w.go", "package old\n"+body) // the pattern is gone
	gitCommit(t, root, "move w, drop needle")

	rows := parseVerifyRows(verifyHead + "| 1 | check | `grep -q needle tools/old/w.go` | exit 0 |\n")
	f := gatherRowFacts(root, rows[0], false, "")
	if f.RenameHop != "tools/new/w.go" {
		t.Fatalf("positive control: rename hop = %q, want tools/new/w.go (facts %+v)", f.RenameHop, f)
	}
	if got := Classify(f); got.Verdict != RefusedBehaviourChanged {
		t.Fatalf("renamed file lacks the pattern: got %s (%s), want %s", got.Verdict, got.Reason, RefusedBehaviourChanged)
	}
}

// TestExpectExit1NotChanged — defect 3. A row whose Expect IS `exit 1` and whose command still
// exits 1 passes; the probe must judge rc against Expect, not read any non-zero as changed.
//
// regression: #1953
func TestExpectExit1NotChanged(t *testing.T) {
	root := gitRepo(t)
	writeFile(t, root, "pkg/x.txt", "one\n")
	gitCommit(t, root, "add x")

	rows := parseVerifyRows(verifyHead + "| 1 | check | `test -f pkg/x.txt && exit 1` | exit 1 (refuses) |\n")
	f := gatherRowFacts(root, rows[0], false, "")
	if !f.CommandProbed || !f.CommandRan {
		t.Fatalf("positive control: the row should be probed and run: %+v", f)
	}
	if got := Classify(f); got.Verdict == RefusedBehaviourChanged {
		t.Fatalf("rc 1 matches Expect `exit 1`: got %s (%s)", got.Verdict, got.Reason)
	}
	// Perturb: the same row now exits 0, which its Expect does not allow.
	rows = parseVerifyRows(verifyHead + "| 1 | check | `test -f pkg/x.txt` | exit 1 (refuses) |\n")
	if got := Classify(gatherRowFacts(root, rows[0], false, "")); got.Verdict != RefusedBehaviourChanged {
		t.Fatalf("rc 0 against Expect `exit 1`: got %s, want %s", got.Verdict, RefusedBehaviourChanged)
	}
}

// renameFixture commits tools/old/w.go, then moves it in one hop to tools/new/w.go whose
// content is next — the rename re-run runs against that file.
func renameFixture(t *testing.T, next string) string {
	t.Helper()
	root := gitRepo(t)
	body := strings.Repeat("// filler line for rename detection\n", 30)
	writeFile(t, root, "tools/old/w.go", "package old\n"+body)
	gitCommit(t, root, "add w")
	if err := os.MkdirAll(filepath.Join(root, "tools/new"), 0o755); err != nil {
		t.Fatal(err)
	}
	mv := exec.Command("git", "mv", "tools/old/w.go", "tools/new/w.go")
	mv.Dir = root
	if out, err := mv.CombinedOutput(); err != nil {
		t.Fatalf("git mv: %v\n%s", err, out)
	}
	writeFile(t, root, "tools/new/w.go", "package old\n"+next+body)
	gitCommit(t, root, "move w")
	return root
}

// TestRenameUndecidableExpectRefused — a rename re-run whose Expect the tool cannot decide
// is not proof: prose, a "non-zero" exit, and an exit-1 guard row (a control row that
// prints its exit as a token) all exit 0 here against a renamed file that now carries the
// forbidden marker. Each must be refused:unclassified, never safe:rename.
//
// regression: #1953
func TestRenameUndecidableExpectRefused(t *testing.T) {
	for _, tc := range []struct{ name, command, expect string }{
		{"prose", "grep -q forbidden tools/old/w.go", "the marker is absent from the widget"},
		{"non-zero", "grep -q forbidden tools/old/w.go", "exit non-zero (the marker is absent)"},
		{"exit-1 guard", "grep -q forbidden tools/old/w.go; echo exit=$?", "`exit=1`; output names the absent marker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := renameFixture(t, "// forbidden\n")
			rows := parseVerifyRows(verifyHead + "| 1 | check | `" + tc.command + "` | " + tc.expect + " |\n")
			f := gatherRowFacts(root, rows[0], false, "")
			if f.RenameHop != "tools/new/w.go" {
				t.Fatalf("positive control: rename hop = %q, want tools/new/w.go (facts %+v)", f.RenameHop, f)
			}
			if got := Classify(f); got.Verdict != RefusedUnclassified {
				t.Fatalf("Expect %q cannot be decided: got %s (%s), want %s", tc.expect, got.Verdict, got.Reason, RefusedUnclassified)
			}
		})
	}
}

// TestRenameJudgesRenamedExpect — the rename re-run judges the renamed command against the
// renamed Expect. Here the Expect names the pinned path, so judging against the old Expect
// would fail a row that is intact.
func TestRenameJudgesRenamedExpect(t *testing.T) {
	root := renameFixture(t, "// needle\n")
	rows := parseVerifyRows(verifyHead + "| 1 | check | `grep -l needle tools/old/w.go` | output is `tools/old/w.go` |\n")
	if got := Classify(gatherRowFacts(root, rows[0], false, "")); got.Verdict != SafeRename {
		t.Fatalf("got %s (%s), want %s", got.Verdict, got.Reason, SafeRename)
	}
}

// TestProbeCouldNotRunIsNotChanged — a probe that ran but produced no verdict (a count floor
// with no number in the output) is not a behaviour change.
func TestProbeCouldNotRunIsNotChanged(t *testing.T) {
	root := gitRepo(t)
	rows := parseVerifyRows(verifyHead + "| 1 | check | `echo none` | exit 0; ≥ `1` |\n")
	f := gatherRowFacts(root, rows[0], false, "")
	if !f.CommandProbed || !f.CommandRan || f.RCDiffers {
		t.Fatalf("want a probed run with no verdict, got %+v", f)
	}
	if got := Classify(f); got.Verdict != RefusedUnclassified {
		t.Fatalf("got %s, want %s", got.Verdict, RefusedUnclassified)
	}
}
