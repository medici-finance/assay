package main

import "testing"

// TestExpectMirrorsStatusgen — the Expect reader is a mirror of statusgen's (a separate
// module), so it is held to the same vectors as statusgen/verifyrun_test.go
// TestParseExpectReadsTheCorpusConventions and TestVerdictThreeState. Where statusgen passes
// on the exit status alone (nothing in the Expect is checkable), the mirror says undecided.
func TestExpectMirrorsStatusgen(t *testing.T) {
	for _, tc := range []struct {
		expect string
		exit   int
		line   string
		min    int
		minSet bool
	}{
		{"exit 0", 0, "", 0, false},
		{"exit 1 (the gate refuses)", 1, "", 0, false},
		{"output is `rc=0`", 0, "rc=0", 0, false},
		{"exit 0; output ≥ `1`", 0, "", 1, true},
		{"exit 0; >= 4 (doc index present)", 0, "", 4, true},
		{"at least 2 rows", 0, "", 2, true},
		{"the reviewer weighs the residue", 0, "", 0, false},
		{"output is `rc=2` (could-not-run is its own exit, not pass)", 0, "rc=2", 0, false},
		{"exit code is its own, not pass", 0, "", 0, false},
		{"output contains `checked-clean` and `exit=0`", 0, "", 0, false},
		{"`exit=1`; output names the leaked marker", 0, "", 0, false},
		{"exit 1 (the compound really exits non-zero)", 1, "", 0, false},
		{"`exit=0`; output ≥ `2`", 0, "", 2, true},
	} {
		got := parseExpect(tc.expect)
		if got.exitCode != tc.exit || got.wantLine != tc.line || got.minSet != tc.minSet || got.minCount != tc.min {
			t.Errorf("parseExpect(%q) = %+v", tc.expect, got)
		}
	}
	for _, tc := range []struct {
		expect string
		ran    bool
		rc     int
		out    string
		want   judgement
	}{
		{"exit 0", true, 0, "", judgePass},
		{"exit 0", true, 1, "", judgeFail},
		{"output is `rc=0`", true, 0, "noise\nrc=0\n", judgePass},
		{"output is `rc=0`", true, 0, "rc=1\n", judgeFail},
		{"≥ `3`", true, 0, "3\n", judgePass},
		{"≥ `3`", true, 0, "2\n", judgeFail},
		{"≥ `1`", true, 0, "a.go:2\nb_test.go:5\n", judgePass},
		{"≥ `1`", true, 0, "no numbers here\n", judgeCouldNotRun},
		{"exit 127", false, 127, "", judgeCouldNotRun},
		{"`exit=1`; output names the leaked marker", true, 0, "exit=1\n", judgeUndecided},
		{"the reviewer weighs the residue", true, 0, "", judgeUndecided},
		{"exit non-zero (the marker is absent)", true, 0, "", judgeUndecided},
		{"exits nonzero; output is `refused`", true, 0, "refused\n", judgeUndecided},
		{"output is `rc=0`", false, 127, "", judgeCouldNotRun},
	} {
		if got := parseExpect(tc.expect).judge(tc.ran, tc.rc, tc.out); got != tc.want {
			t.Errorf("judge(%q, rc %d) = %s, want %s", tc.expect, tc.rc, got, tc.want)
		}
	}
}

// TestRebaselineKeepsEscapes — the rewrite matches an un-escaped Command against the line's
// own `\|` form and writes the new command back escaped, so the table keeps its columns.
func TestRebaselineKeepsEscapes(t *testing.T) {
	content := verifyHead + "| 1 | check | `grep a tools/old/w.go \\| wc -l` | exit 0 |\n"
	row := parseVerifyRows(content)[0]
	f := RowFacts{PathRef: "tools/old/w.go", RenameHop: "tools/new/w.go"}
	_, _, newLine, ok := applyRebaseline(content, row, renamed(row.Command, f), row.Expect)
	if want := "| 1 | check | `grep a tools/new/w.go \\| wc -l` | exit 0 |"; !ok || newLine != want {
		t.Fatalf("rewrite = %q ok=%v, want %q", newLine, ok, want)
	}
}

// TestRenameUnjudgedFailsClosed — a rename hop whose re-run produced no verdict (or was never
// re-run) is not safe:rename.
func TestRenameUnjudgedFailsClosed(t *testing.T) {
	for _, j := range []judgement{"", judgeCouldNotRun} {
		f := RowFacts{PathRef: "a/x.go", RenameHop: "b/x.go", RenameProbe: j}
		if got := Classify(f); got.Verdict != RefusedUnclassified {
			t.Errorf("rename probe %q: got %s, want %s", j, got.Verdict, RefusedUnclassified)
		}
	}
}
