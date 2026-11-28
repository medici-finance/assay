package main

import (
	"strings"
	"testing"
	"time"
)

// goNoTests is what `go test -v -run '^TestGone$'` prints, exit 0, when the
// named test does not exist.
const goNoTests = "testing: warning: no tests to run\nPASS\nok  \texample.com/pkg\t0.004s [no tests to run]\n"

// regression: #7389
// TestContainsPassLineNeedsTest — an Expect of "output contains `--- PASS:
// <name>`" is decided on the output, never on the exit status alone. A missing
// test (go test exits 0 with "no tests to run"), a different test whose name
// extends the named one, and a passing subtest under a failing parent each
// fail; only the named test's own top-level PASS line passes.
func TestContainsPassLineNeedsTest(t *testing.T) {
	const expect = "exit 0; output contains `--- PASS: TestGone`"
	for _, c := range []struct {
		name string
		res  runResult
		want string
	}{
		{"the test does not exist", runResult{exit: 0, output: []byte(goNoTests)}, stateFail},
		{"a longer test name", runResult{exit: 0, output: []byte("=== RUN   TestGoneLonger\n--- PASS: TestGoneLonger (0.00s)\nPASS\n")}, stateFail},
		{"only a subtest passes", runResult{exit: 0, output: []byte("=== RUN   TestGone\n    --- PASS: TestGone/a (0.00s)\n")}, stateFail},
		{"the named test passes", runResult{exit: 0, output: []byte("=== RUN   TestGone\n--- PASS: TestGone (0.00s)\nPASS\nok  \texample.com/pkg\t0.004s\n")}, statePass},
		{"the named test passes, CRLF output", runResult{exit: 0, output: []byte("--- PASS: TestGone (0.00s)\r\nPASS\r\n")}, statePass},
		{"the PASS line but a non-zero exit", runResult{exit: 1, output: []byte("--- PASS: TestGone (0.00s)\nFAIL\n")}, stateFail},
	} {
		if got, note := parseExpect(expect).verdict(c.res); got != c.want {
			t.Errorf("%s: verdict = %q (%s), want %q", c.name, got, note, c.want)
		}
	}
	// A subtest literal names an indented line.
	sub := parseExpect("output contains `--- PASS: TestGone/a`")
	if got, note := sub.verdict(runResult{output: []byte("    --- PASS: TestGone/a (0.00s)\n")}); got != statePass {
		t.Errorf("a named subtest's own line must pass; got %q (%s)", got, note)
	}
}

// regression: #7389
// TestContainsClausesAreRead — every readable output-containment clause is
// checked, and a clause that names output but whose literal cannot be read is
// could-not-run, never a pass on the exit status.
func TestContainsClausesAreRead(t *testing.T) {
	for _, c := range []struct {
		expect string
		out    string
		want   string
	}{
		{"output contains `ok`", "ok  \tpkg\t0.1s\n", statePass},
		{"output contains `ok`", "FAIL\n", stateFail},
		{`output contains "LINT: PASS"`, "LINT: PASS\n", statePass},
		{`output contains "LINT: PASS"`, "LINT: FAIL\n", stateFail},
		{"output contains PASS; the golden records a 403", "PASS\n", statePass},
		{"output contains PASS; the golden records a 403", "FAIL\n", stateFail},
		{"output contains fact-check, fail-first, security", "fact-check fail-first security\n", statePass},
		{"output contains fact-check, fail-first, security", "fact-check fail-first\n", stateFail},
		{"output contains `checked-clean` and `exit=0`", "checked-clean\nexit=1\n", stateFail},
		{"output contains both `A1` and `B1`", "A1\n", stateFail},
		{"output contains both `A1` and `B1`", "A1 B1\n", statePass},
		{"output contains the literal line `--- PASS: TestA` (not the exit)", "--- PASS: TestA (0.01s)\n", statePass},
		{"output contains the literal line `--- PASS: TestA` (not the exit)", "ok  \tpkg\t0.1s [no tests to run]\n", stateFail},
		{"output contains BOTH literal lines `--- PASS: TestA` and `--- PASS: TestB`", "--- PASS: TestA (0.00s)\n", stateFail},
		{"exit 0; output contains `ok`, exit 0 again", "ok\n", statePass},
		{"output line contains `inactive`", "state: inactive\n", statePass},
		{"stderr contains `refused`", "nope\n", stateFail},
		{"output does not contain `DISPROVED` (run on the branch)", "3 corroborated, 0 disproved\n", statePass},
		{"output does not contain `DISPROVED` (run on the branch)", "DISPROVED: x\n", stateFail},
		{"output does not contain DISPROVED", "DISPROVED\n", stateFail},
		{"output never contains `reviewer=example-act`", "reviewer=example-act\n", stateFail},
		{"output must not contain `secret`", "a secret\n", stateFail},
		// Unreadable: prose, a disjunction, an empty literal.
		{"output contains the named PASS line. Cases: x", "--- PASS: TestX (0.00s)\n", stateCouldNotRun},
		{"output must contain the named top-level or subtest `--- PASS:` line", "--- PASS: TestX (0.00s)\n", stateCouldNotRun},
		{"output contains `A1` or `B1`", "A1\n", stateCouldNotRun},
		{"output contains ``", "x\n", stateCouldNotRun},
		{"output contains it. A token list exits 6", "it\n", stateCouldNotRun},
		{"output contains `PE32` / `MS Windows`", "MS Windows\n", stateCouldNotRun},
		{"output contains summary: 3 corroborated, 0 disproved", "summary: 3 corroborated, 0 disproved\n", stateCouldNotRun},
		// Another process's stream is not the row's output: unreadable, never a
		// fail on a row that prints only its test's PASS line. Shapes from
		// desk-tools/29 rows 3 and 13.
		{"exit 0, `--- PASS:` printed. (e) `deskverdict verify --body <file>` exits 6 AND its stderr contains `verdict-payload block found` — the 6 is the refusal", "--- PASS: TestX (0.00s)\n", stateCouldNotRun},
		{"exit 0, `--- PASS:` printed. Subtest `no-roster-refuses`: `HOME` is empty; `verdict --unsigned-out <file>` exits 6, stderr contains `could-not-check: inactive`, and the file does not exist", "--- PASS: TestX (0.00s)\n", stateCouldNotRun},
		{"the binary's stdout contains `x`", "x\n", stateCouldNotRun},
		{"exit 0; whose output contains `x`", "x\n", stateCouldNotRun},
		// A negation before the subject is never read as a presence claim.
		{"exit 0; no output contains `secret`", "a secret\n", stateCouldNotRun},
		{"exit 0; neither stdout nor stderr contains `tok`", "tok\n", stateCouldNotRun},
		{"exit 0; nothing in stdout contains `x`", "x\n", stateCouldNotRun},
		// The row's own exit before the subject keeps the clause the row's.
		{"exit 0, output contains `ok`", "ok\n", statePass},
		{"exit 0 and output contains `ok`", "FAIL\n", stateFail},
		// A readable clause that fails is a fail even beside an unreadable one.
		{"output contains `ok`; output contains the named line", "FAIL\n", stateFail},
		// No containment clause: unchanged.
		{"exit 0", "anything\n", statePass},
	} {
		got, note := parseExpect(c.expect).verdict(runResult{exit: 0, output: []byte(c.out)})
		if got != c.want {
			t.Errorf("Expect %q on %q: verdict = %q (%s), want %q", c.expect, c.out, got, note, c.want)
		}
	}
	// The could-not-run reason is written into the witness row, so it is the
	// fixed constant and never carries table syntax.
	if _, note := parseExpect("output contains the named line").verdict(runResult{}); note != unreadableContainsNote || strings.Contains(note, "|") {
		t.Errorf("unreadable note = %q", note)
	}
}

// regression: #7389
// TestFailFirstBaseMissingTestRed — the fail-first base run reads the row
// through the same Expect reader as the head run. A row asserting a test's PASS
// line, run at a base where the test does not exist yet (go test exits 0 with
// "no tests to run"), is red at base, not green, so it is not reported as
// non-discriminating.
func TestFailFirstBaseMissingTestRed(t *testing.T) {
	root, _, base, _, _, _ := ffRepo(t)
	// flag.txt is "new" only at head: the stand-in for a test the change adds.
	cmd := "if grep -q new flag.txt; then echo '--- PASS: TestNew (0.00s)'; else echo 'testing: warning: no tests to run'; fi; echo PASS"
	rows := []verifyRow{{ID: "1", Command: cmd, Expect: "exit 0; output contains `--- PASS: TestNew`"}}
	risky := map[string]bool{"1": true}
	got, err := runFailFirstBase(resolveShellPlan(), sandboxUnshare, root, base, rows, risky, time.Minute, false)
	if err != nil {
		t.Fatalf("runFailFirstBase: %v", err)
	}
	if got["1"].State != baseRed {
		t.Errorf("a base without the named test must be red; got %+v", got["1"])
	}
	head := runWitnessesSandboxed(resolveShellPlan(), sandboxUnshare, root, rows, "", "", "", "", time.Minute, false)
	if len(head) != 1 || head[0].State != statePass {
		t.Fatalf("the head holds the test, so the row passes there; got %+v", head)
	}
}
