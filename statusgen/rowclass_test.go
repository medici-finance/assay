package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// All tests in this file share the `RowClass` substring so a single
// `go test -run RowClass` (the brief's verify.d/brief-02/row-1.sh) exercises the
// whole parser+class+lint+routing surface at once.

// briefV1WithVerify writes a minimal, structurally-valid brief-v1 file whose
// `## Verify` section is `verifyBody`, and returns its path.
func briefV1WithVerify(t *testing.T, dir, num, verifyBody string) string {
	t.Helper()
	fm := "---\n" +
		"brief: t/" + num + "\n" +
		"title: fixture\n" +
		"wave: 0\n" +
		"depends: []\n" +
		"unblocks: []\n" +
		"effort: S\n" +
		"gate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" +
		"issues: []\n" +
		"authored: 2026-08-17 by test\n" +
		"sources: [\"fixture\"]\n" +
		"schema: brief-v1\n" +
		"---\n\n# Fixture\n\n## Verify\n" + verifyBody + "\n\n## Evidence\n\n## Review\nGate: model.\n"
	path := filepath.Join(dir, "brief-"+num+".md")
	if err := os.WriteFile(path, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRowClass_Resolve(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		known bool
	}{
		{"", legacyRowClass, true},           // empty → legacy default check
		{"check:ci", classCheckCI, true},     //
		{"`check:ci`", classCheckCI, true},   // markdown code span stripped
		{"CHECK", classCheck, true},          // case-folded
		{"gate:model", classGateModel, true}, //
		{"gate:human", classGateHuman, true}, //
		{"florp", "florp", false},            // unknown → returned verbatim, not known
		{"check-ci", "check-ci", false},      // near-miss is still unknown
	}
	for _, c := range cases {
		got, known := resolveRowClass(c.in)
		if got != c.want || known != c.known {
			t.Errorf("resolveRowClass(%q) = (%q, %v), want (%q, %v)", c.in, got, known, c.want, c.known)
		}
	}
}

func TestRowClass_TableColumnParsed(t *testing.T) {
	// A table WITH a Class column: each row carries its declared class and Classed=true.
	section := strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci | `true` | exit 0 |",
		"| 2 | check | `true` | exit 0 |",
		"| 3 | gate:human | prose | a human reads it |",
	}, "\n")
	var got []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 3 {
		t.Fatalf("parsed %d rows, want 3", len(got))
	}
	want := []string{classCheckCI, classCheck, classGateHuman}
	for i, r := range got {
		if !r.Classed {
			t.Errorf("row %d: Classed=false, want true (table has a Class column)", i+1)
		}
		if r.class() != want[i] {
			t.Errorf("row %d: class()=%q, want %q", i+1, r.class(), want[i])
		}
	}
}

func TestRowClass_LegacyTableDefaultsToCheck(t *testing.T) {
	// A table with NO Class column: every row is legacy check, Classed=false — the
	// compatibility hinge that keeps the inherited corpus behaving as before.
	section := strings.Join([]string{
		"| # | Command | Expect |",
		"|---|---------|--------|",
		"| 1 | `true` | exit 0 |",
	}, "\n")
	var got []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 1 {
		t.Fatalf("parsed %d rows, want 1", len(got))
	}
	if got[0].Classed {
		t.Error("legacy table: Classed=true, want false")
	}
	if got[0].class() != classCheck {
		t.Errorf("legacy row class()=%q, want %q", got[0].class(), classCheck)
	}
}

// TestRowClassLegacyDefaultUnchanged is the regression control (Verify row 7):
// a fixture table copied byte-for-byte from the inherited corpus — no Class
// column — must resolve exactly as it did before the obligation encoding
// existed. Every row is legacy `check`, Classed=false, and carries no obligation.
// The obligation axis is additive precisely because this stays true.
func TestRowClassLegacyDefaultUnchanged(t *testing.T) {
	// Copied from an inherited brief's Verify table shape (three-column, no Class).
	section := strings.Join([]string{
		"| # | Command | Expect |",
		"|---|---------|--------|",
		"| 1 | `git grep -n foo -- statusgen/` | exit 0 |",
		"| 2 | `go test ./statusgen/ -run Bar -count=1` | exit 0 |",
		"| 3 | prose — a human reads it | a human decides |",
	}, "\n")
	var got []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 3 {
		t.Fatalf("parsed %d rows, want 3", len(got))
	}
	for i, r := range got {
		if r.Classed {
			t.Errorf("row %d: Classed=true, want false (legacy table has no Class column)", i+1)
		}
		if r.class() != legacyRowClass {
			t.Errorf("row %d: class()=%q, want legacy default %q", i+1, r.class(), legacyRowClass)
		}
		if obs := r.obligations(); len(obs) != 0 {
			t.Errorf("row %d: obligations()=%v, want none (a legacy row carries no obligation)", i+1, obs)
		}
	}
}

func TestRowClass_CompoundCellSplit(t *testing.T) {
	cases := []struct {
		in       string
		wantExec string
		wantObl  []string
	}{
		{"check", "check", nil},                                                // execution-only, unchanged
		{"check:ci", "check:ci", nil},                                          // execution-only, unchanged
		{"`check:ci`", "check:ci", nil},                                        // code span stripped, unchanged
		{"check +mutation", "check", []string{"mutation"}},                     // one obligation
		{"check:ci +mutation +flow", "check:ci", []string{"mutation", "flow"}}, // two
		{"gate:model +dereference", "gate:model", []string{"dereference"}},
		{"+mutation", "", []string{"mutation"}},            // no exec → legacy default, obligation carried
		{"CHECK +Mutation", "check", []string{"mutation"}}, // case-folded on both axes
		{"", "", nil}, // empty → legacy
	}
	for _, c := range cases {
		exec, obl := splitRowClassCell(c.in)
		if exec != c.wantExec {
			t.Errorf("splitRowClassCell(%q) exec=%q, want %q", c.in, exec, c.wantExec)
		}
		if strings.Join(obl, ",") != strings.Join(c.wantObl, ",") {
			t.Errorf("splitRowClassCell(%q) obl=%v, want %v", c.in, obl, c.wantObl)
		}
	}
}

// TestRowClass_ObligationValuesKnownAndRouting proves the four new obligation
// values are a KNOWN closed set, that a compound cell routes on its EXECUTION
// token only (so every execution consumer is untouched), and that the obligation
// tokens are readable as a second axis.
func TestRowClass_ObligationValuesKnown(t *testing.T) {
	for _, ob := range []string{classMutation, classFlow, classDereference, classNeighbour} {
		if !knownObligations[ob] {
			t.Errorf("obligation %q not in knownObligations", ob)
		}
		// An obligation value must NOT be an execution class — the two axes are
		// separate, and adding it to knownRowClasses is exactly what task 1 forbids.
		if knownRowClasses[ob] {
			t.Errorf("obligation %q leaked into the execution class set knownRowClasses", ob)
		}
	}
	section := strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci +mutation | `true` | exit 0 |",
		"| 2 | gate:model +dereference | prose | resolves |",
	}, "\n")
	var got []verifyRowCells
	verifyRowTable(section, func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 2 {
		t.Fatalf("parsed %d rows, want 2", len(got))
	}
	// Routing is on the execution token; the obligation does not change it.
	if got[0].class() != classCheckCI {
		t.Errorf("row 1 class()=%q, want %q (routing ignores the obligation)", got[0].class(), classCheckCI)
	}
	if got[1].class() != classGateModel {
		t.Errorf("row 2 class()=%q, want %q", got[1].class(), classGateModel)
	}
	if obs := got[0].obligations(); len(obs) != 1 || obs[0] != classMutation {
		t.Errorf("row 1 obligations()=%v, want [mutation]", obs)
	}
	if obs := got[1].obligations(); len(obs) != 1 || obs[0] != classDereference {
		t.Errorf("row 2 obligations()=%v, want [dereference]", obs)
	}
}

// TestRowClass_UnknownObligationIsProblem is the unknown-value refusal for the
// SECOND axis (Verify row 6): a `+typo` obligation token is fatal exactly as an
// unknown execution class is, while a well-formed compound cell is clean.
func TestRowClass_UnknownObligationIsProblem(t *testing.T) {
	dir := t.TempDir()
	briefV1WithVerify(t, dir, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check +mutaton | `true` | exit 0 |", // typo: mutaton
	}, "\n"))
	s := &Stream{Name: "t", Dir: dir, Root: dir}
	probs := verifyRowClassProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], `obligation "mutaton"`) {
		t.Fatalf("unknown obligation: got %v, want one PROBLEM naming mutaton", probs)
	}

	// A well-formed obligation compound cell is clean.
	dir2 := t.TempDir()
	briefV1WithVerify(t, dir2, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check +mutation +flow | `true` | exit 0 |",
	}, "\n"))
	s2 := &Stream{Name: "t", Dir: dir2, Root: dir2}
	if probs := verifyRowClassProblems([]*Stream{s2}); len(probs) != 0 {
		t.Errorf("well-formed obligation compound cell: got %v, want none", probs)
	}
}

func TestRowClass_ScriptPathDetection(t *testing.T) {
	yes := "docs/streams/verdict-lane/verify.d/brief-02/row-1.sh"
	if scriptPath(yes) != yes {
		t.Errorf("scriptPath(%q) = %q, want the path", yes, scriptPath(yes))
	}
	for _, no := range []string{
		"cat docs/streams/x/verify.d/brief-02/row-1.sh", // inline command mentioning a path
		"go test ./...",
		"docs/streams/x/verify.d/brief-02/row-1.py", // not a .sh
		"scripts/other.sh",                          // not under verify.d
	} {
		if scriptPath(no) != "" {
			t.Errorf("scriptPath(%q) = %q, want empty (not a scripted row)", no, scriptPath(no))
		}
	}
}

func TestRowClass_LintUnknownClassIsProblem(t *testing.T) {
	dir := t.TempDir()
	briefV1WithVerify(t, dir, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | florp | `true` | exit 0 |",
	}, "\n"))
	s := &Stream{Name: "t", Dir: dir, Root: dir}
	probs := verifyRowClassProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], `class "florp"`) {
		t.Fatalf("unknown class: got %v, want one PROBLEM naming florp", probs)
	}
}

func TestRowClass_LintMissingScriptScopedToNonTodo(t *testing.T) {
	writeBrief := func(dir string) {
		briefV1WithVerify(t, dir, "01", strings.Join([]string{
			"| # | Class | Command | Expect |",
			"|---|-------|---------|--------|",
			"| 1 | check:ci | docs/streams/t/verify.d/brief-01/row-1.sh | exit 0 |",
		}, "\n"))
	}

	// todo brief → the planned script is exempt (a plan, not a claim).
	todoDir := t.TempDir()
	streamDir := filepath.Join(todoDir, "docs", "streams", "t")
	if err := os.MkdirAll(streamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBrief(streamDir)
	todo := &Stream{Name: "t", Dir: streamDir, Root: todoDir, Briefs: []Brief{{Num: "01", Status: "todo"}}}
	if probs := verifyRowClassProblems([]*Stream{todo}); len(probs) != 0 {
		t.Errorf("todo brief with missing script: got %v, want none (exempt)", probs)
	}

	// implemented brief → the missing script is a PROBLEM.
	impl := &Stream{Name: "t", Dir: streamDir, Root: todoDir, Briefs: []Brief{{Num: "01", Status: "implemented"}}}
	probs := verifyRowClassProblems([]*Stream{impl})
	if len(probs) != 1 || !strings.Contains(probs[0], "does not exist") {
		t.Fatalf("implemented brief with missing script: got %v, want one missing-script PROBLEM", probs)
	}
}

func TestRowClass_LintNonExecutableScriptIsProblem(t *testing.T) {
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "t")
	scriptDir := filepath.Join(streamDir, "verify.d", "brief-01")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	briefV1WithVerify(t, streamDir, "01", strings.Join([]string{
		"| # | Class | Command | Expect |",
		"|---|-------|---------|--------|",
		"| 1 | check:ci | docs/streams/t/verify.d/brief-01/row-1.sh | exit 0 |",
	}, "\n"))
	script := filepath.Join(scriptDir, "row-1.sh")

	// Present but NOT executable → PROBLEM.
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Stream{Name: "t", Dir: streamDir, Root: root, Briefs: []Brief{{Num: "01", Status: "verified"}}}
	probs := verifyRowClassProblems([]*Stream{s})
	if len(probs) != 1 || !strings.Contains(probs[0], "not executable") {
		t.Fatalf("non-executable script: got %v, want one not-executable PROBLEM", probs)
	}

	// chmod +x → clean.
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	if probs := verifyRowClassProblems([]*Stream{s}); len(probs) != 0 {
		t.Errorf("executable script: got %v, want none", probs)
	}
}

func TestRowClass_ScriptDiffNotice(t *testing.T) {
	// No verify.d path in the diff → inert.
	if n := verifyScriptDiffNotices([]string{"statusgen/main.go", "docs/streams/t/README.md"}); len(n) != 0 {
		t.Errorf("no verify.d change: got %v, want no notice", n)
	}
	// A verify.d path in the diff → one conspicuous NOTICE naming the script.
	changed := []string{
		"statusgen/rowclass.go",
		"docs/streams/verdict-lane/verify.d/brief-02/row-3.sh",
	}
	n := verifyScriptDiffNotices(changed)
	if len(n) != 1 {
		t.Fatalf("verify.d change: got %d notices, want 1", len(n))
	}
	if !strings.Contains(n[0], "row-3.sh") || !strings.Contains(n[0], "VERIFY-SCRIPT REVIEW") {
		t.Errorf("notice does not name the script conspicuously: %q", n[0])
	}
}

func TestRowClass_VerifyrunSkipsExplicitCheckInCI(t *testing.T) {
	rows := []verifyRow{
		{ID: "1", Command: "true", Expect: "exit 0", Class: classCheck, Classed: true},  // explicit → skip in CI
		{ID: "2", Command: "true", Expect: "exit 0", Class: classCheck, Classed: false}, // legacy → still runs
	}
	ws := runWitnesses(t.TempDir(), rows, "test", "0000", "2026-08-17", 30*time.Second, true /* ci */)
	if ws[0].State != stateSkipped {
		t.Errorf("explicit check row in CI: state %q, want %q", ws[0].State, stateSkipped)
	}
	if ws[0].Note != verifyEnvBoundNote {
		t.Errorf("skip note = %q, want %q", ws[0].Note, verifyEnvBoundNote)
	}
	if ws[1].State != statePass {
		t.Errorf("legacy check row in CI: state %q, want pass (the inherited corpus is still re-executed)", ws[1].State)
	}
	// A skip is not written into the witness table.
	if strings.Contains(witnessTable(ws), "skip") {
		t.Error("witnessTable rendered a skip row; a skip is the absence of a run and must not appear")
	}
}

func TestRowClass_VerifyrunHermeticCheckCI(t *testing.T) {
	rows := []verifyRow{{ID: "1", Command: "true", Expect: "exit 0", Class: classCheckCI, Classed: true}}
	ws := runWitnesses(t.TempDir(), rows, "test", "0000", "2026-08-17", 30*time.Second, false)
	// On Linux the network-off sandbox runs `true` → pass. Off Linux (or where
	// unprivileged user namespaces are unavailable) the row is could-not-run with
	// the sandbox-unavailable reason — never a silent pass.
	if runtime.GOOS == "linux" {
		if ws[0].State != statePass && ws[0].State != stateCouldNotRun {
			t.Errorf("check:ci on linux: state %q, want pass or could-not-run", ws[0].State)
		}
	} else {
		if ws[0].State != stateCouldNotRun || !strings.Contains(ws[0].Note, "hermetic") {
			t.Errorf("check:ci off-linux: state=%q note=%q, want could-not-run naming the missing hermetic sandbox", ws[0].State, ws[0].Note)
		}
	}
}

func TestRowClass_NetworkOffWrapperHonestPerHost(t *testing.T) {
	_, ok, why := networkOffWrapper()
	if runtime.GOOS != "linux" {
		if ok || why == "" {
			t.Errorf("off-linux: ok=%v why=%q, want unavailable with a reason", ok, why)
		}
	}
	// On linux the result depends on kernel config; we only assert it does not
	// panic and returns a reason string when unavailable.
	if !ok && why == "" {
		t.Error("unavailable wrapper must carry a human-legible reason")
	}
}
