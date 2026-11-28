package main

import "testing"

// regression: #7389
// TestContainsMirrorsStatusgen — the mirror judges output-containment clauses
// with statusgen's vectors (statusgen/expectcontains_test.go). A re-baseline
// probe of a row asserting `--- PASS: <name>` is never a pass when the test is
// missing, renamed, or only a subtest passed; an unreadable clause is
// undecided, where statusgen says could-not-run. A clause never turns an
// undecided probe into a pass: re-baselining rests only on readings it had.
func TestContainsMirrorsStatusgen(t *testing.T) {
	const noTests = "testing: warning: no tests to run\nPASS\nok  \texample.com/pkg\t0.004s [no tests to run]\n"
	for _, tc := range []struct {
		expect string
		rc     int
		out    string
		want   judgement
	}{
		{"exit 0; output contains `--- PASS: TestGone`", 0, noTests, judgeFail},
		{"exit 0; output contains `--- PASS: TestGone`", 0, "--- PASS: TestGoneLonger (0.00s)\n", judgeFail},
		{"exit 0; output contains `--- PASS: TestGone`", 0, "    --- PASS: TestGone/a (0.00s)\n", judgeFail},
		{"exit 0; output contains `--- PASS: TestGone`", 0, "--- PASS: TestGone (0.00s)\nPASS\n", judgePass},
		{"exit 0; output contains `--- PASS: TestGone/a`", 0, "    --- PASS: TestGone/a (0.00s)\n", judgePass},
		// No exit named: a containment clause adds a way to fail, never a pass.
		{"output contains `--- PASS: TestGone/a`", 0, "    --- PASS: TestGone/a (0.00s)\n", judgeUndecided},
		{"output contains `checked-clean` and `exit=0`", 0, "checked-clean\nexit=0\n", judgeUndecided},
		{"output contains `checked-clean` and `exit=0`", 0, "checked-clean\nexit=1\n", judgeFail},
		{`output contains "LINT: PASS"`, 0, "LINT: FAIL\n", judgeFail},
		{"output contains PASS; the golden records a 403", 0, "FAIL\n", judgeFail},
		{"output does not contain `DISPROVED` (run on the branch)", 0, "DISPROVED: x\n", judgeFail},
		{"exit 0; output does not contain `DISPROVED`", 0, "0 disproved\n", judgePass},
		{"exit 0; output contains the named PASS line. Cases: x", 0, "--- PASS: TestX (0.00s)\n", judgeUndecided},
		{"output contains `A1` or `B1`", 0, "A1\n", judgeUndecided},
		{"exit 0; output contains the literal line `--- PASS: TestA` (not the exit)", 0, "ok  \tpkg [no tests to run]\n", judgeFail},
		{"exit 0; output contains BOTH literal lines `--- PASS: TestA` and `--- PASS: TestB`", 0, "--- PASS: TestA (0.00s)\n--- PASS: TestB (0.00s)\n", judgePass},
		{"output contains `ok`; output contains the named line", 0, "FAIL\n", judgeFail},
		// Another process's stream, or a leading negation: undecided, never a fail or a pass.
		{"exit 0, `--- PASS:` printed. (e) `deskverdict verify --body <file>` exits 6 AND its stderr contains `verdict-payload block found` — the 6 is the refusal", 0, "--- PASS: TestX (0.00s)\n", judgeUndecided},
		{"exit 0, `--- PASS:` printed. Subtest `no-roster-refuses`: `HOME` is empty; `verdict --unsigned-out <file>` exits 6, stderr contains `could-not-check: inactive`, and the file does not exist", 0, "--- PASS: TestX (0.00s)\n", judgeUndecided},
		{"exit 0; the binary's stdout contains `x`", 0, "x\n", judgeUndecided},
		{"exit 0; no output contains `secret`", 0, "a secret\n", judgeUndecided},
		{"exit 0; neither stdout nor stderr contains `tok`", 0, "tok\n", judgeUndecided},
		{"exit 0, output contains `ok`", 0, "ok\n", judgePass},
	} {
		if got := parseExpect(tc.expect).judge(true, tc.rc, tc.out); got != tc.want {
			t.Errorf("judge(%q, %q) = %s, want %s", tc.expect, tc.out, got, tc.want)
		}
	}
}
