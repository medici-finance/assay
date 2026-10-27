package main

import (
	"strings"
	"testing"
)

// regression: #1953
func TestRenameKeepsGuardPath(t *testing.T) {
	root := renameFixture(t, "// needle\n")
	writeFile(t, root, "tools/old/w.go.bak", "guard\n")
	rows := parseVerifyRows(verifyHead + "| 1 | check | `grep -q needle tools/old/w.go && ! test -e tools/old/w.go.bak` | exit 0 |\n")
	f := gatherRowFacts(root, rows[0], false, "")
	if f.RenameHop != "tools/new/w.go" {
		t.Fatalf("positive control: hop = %q", f.RenameHop)
	}
	if got := Classify(f); got.Verdict != RefusedBehaviourChanged {
		t.Fatalf("existing .bak guard must fail: got %s, probe %s", got.Verdict, f.RenameProbe)
	}
	// Both consumers must preserve the guard: force only the rewrite decision here.
	cmd, _, ok := computeNewRow(root, rows[0], f, Classification{Verdict: SafeRename})
	if want := "grep -q needle tools/new/w.go && ! test -e tools/old/w.go.bak"; !ok || cmd != want {
		t.Fatalf("open command = %q, changed=%v, want %q", cmd, ok, want)
	}
}

// regression: #1953
func TestRenameWholePaths(t *testing.T) {
	f := RowFacts{PathRef: "tools/old/w.go", RenameHop: "tools/new/w.go"}
	for _, tc := range []struct{ in, want string }{
		{"test -f tools/old/w.go", "test -f tools/new/w.go"},
		{"cat 'tools/old/w.go'", "cat 'tools/new/w.go'"},
		{"cat \"tools/old/w.go\"", "cat \"tools/new/w.go\""},
		{"output is `tools/old/w.go`", "output is `tools/new/w.go`"},
		{"check --file=tools/old/w.go", "check --file=tools/new/w.go"},
		{"FILE='tools/old/w.go'", "FILE='tools/new/w.go'"},
		{"tools/old/w.go;tools/old/w.go", "tools/new/w.go;tools/new/w.go"},
		{"tools/old/w.go.bak tools/old/w.go2 pretools/old/w.go /tools/old/w.go tools/old/w.go/x", "tools/old/w.go.bak tools/old/w.go2 pretools/old/w.go /tools/old/w.go tools/old/w.go/x"},
		{"tools/old/w.go+copy tools/old/w.go:part tools/old/w.go@x tools/old/w.goé", "tools/old/w.go+copy tools/old/w.go:part tools/old/w.go@x tools/old/w.goé"},
		{"\"prefix tools/old/w.go suffix\"", "\"prefix tools/old/w.go suffix\""},
		{"\"tools/old/w.go\".bak pre'tools/old/w.go'", "\"tools/old/w.go\".bak pre'tools/old/w.go'"},
	} {
		if got := renamed(tc.in, f); got != tc.want {
			t.Errorf("renamed(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	f.PathRef, f.RenameHop = "tools/old/a file.go", "tools/new/a file.go"
	if got := renamed("cat 'tools/old/a file.go' 'tools/old/a file.go.bak'", f); got != "cat 'tools/new/a file.go' 'tools/old/a file.go.bak'" {
		t.Errorf("quoted spaces = %q", got)
	}
	if got := renamed("unchanged", RowFacts{}); got != "unchanged" {
		t.Errorf("empty path changed text: %q", got)
	}
}

// regression: #1953
func TestNumericExitWinsProse(t *testing.T) {
	for _, n := range []struct {
		expect string
		rc     int
	}{
		{"exit 1 (the compound really exits non-zero)", 1},
		{"exit 0; fail-first should be non-zero", 0},
		{"Exit code = 2; a nonzero result", 2},
	} {
		e := parseExpect(n.expect)
		if got := e.judge(true, n.rc, ""); got != judgePass {
			t.Errorf("%q matching rc %d = %s, want pass", n.expect, n.rc, got)
		}
		if got := e.judge(true, n.rc+1, ""); got != judgeFail {
			t.Errorf("%q wrong rc = %s, want fail", n.expect, got)
		}
	}
	root := renameFixture(t, "// needle\n")
	row := parseVerifyRows(verifyHead + "| 1 | check | `grep -q needle tools/old/w.go` | exit 0; fail-first is non-zero |\n")[0]
	if got := Classify(gatherRowFacts(root, row, false, "")); got.Verdict != SafeRename {
		t.Fatalf("explicit numeric rename: %s", got.Verdict)
	}
	row.Command = "grep -q missing tools/new/w.go"
	if got := Classify(gatherRowFacts(root, row, false, "")); got.Verdict != RefusedBehaviourChanged {
		t.Fatalf("explicit numeric probe: %s", got.Verdict)
	}
}

// regression: #1953
func TestExpectStaysFailClosed(t *testing.T) {
	huge := strings.Repeat("9", 40)
	for _, expect := range []string{"exit non-zero", "exits nonzero; output is `ok`", "prose only", "`exit=0`", "exit " + huge, "exit " + huge + "; output is `ok`", "exit 0; ≥ " + huge} {
		for _, out := range []string{"ok\n", "9223372036854775807\n"} {
			if got := parseExpect(expect).judge(true, 0, out); got == judgePass {
				t.Errorf("%q = pass without a checkable constraint", expect)
			}
		}
	}
	if got := parseExpect("exit 0").judge(false, 0, ""); got != judgeCouldNotRun {
		t.Errorf("could-not-run = %s", got)
	}
}

// regression: #1953
func TestCommentRenameNeverRuns(t *testing.T) {
	root := renameFixture(t, "// needle\n")
	orig := runCommand
	calls := 0
	runCommand = func(_, _ string) (bool, int, string) { calls++; return true, 0, "" }
	t.Cleanup(func() { runCommand = orig })
	for _, path := range []string{"tools/old/w.go", "tools/new/w.go"} {
		for _, cell := range []string{"`# test -f " + path + "`", "`# test -f " + path + "`: confirm it", "`cmd: # test -f " + path + "`"} {
			row := parseVerifyRows(verifyHead + "| 1 | check | " + cell + " | exit 0 |\n")[0]
			f := gatherRowFacts(root, row, false, "")
			if path == "tools/old/w.go" && f.RenameHop != "tools/new/w.go" {
				t.Fatalf("positive control: hop = %q", f.RenameHop)
			}
			if got := Classify(f); calls != 0 || f.CommandProbed || f.RenameProbe != "" || got.Verdict != RefusedUnclassified {
				t.Errorf("comment %q: calls=%d facts=%+v verdict=%s", cell, calls, f, got.Verdict)
			}
		}
	}
	// A caller constructing a row directly must receive the same comment-only protection.
	f := gatherRowFacts(root, verifyRow{Num: 1, Command: "# test -f tools/old/w.go", ExpectCell: "exit 0"}, false, "")
	if calls != 0 || Classify(f).Verdict != RefusedUnclassified {
		t.Errorf("direct comment: calls=%d facts=%+v", calls, f)
	}
	row := parseVerifyRows(verifyHead + "| 1 | check | `test -f tools/old/w.go` | exit 0 |\n")[0]
	if got := Classify(gatherRowFacts(root, row, false, "")); calls != 1 || got.Verdict != SafeRename {
		t.Fatalf("positive control: calls=%d verdict=%s", calls, got.Verdict)
	}
}
