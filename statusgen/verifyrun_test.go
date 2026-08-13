package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Expect parsing
// ---------------------------------------------------------------------------

func TestParseExpectReadsTheCorpusConventions(t *testing.T) {
	cases := []struct {
		name     string
		expect   string
		wantExit int
		wantLine string
		wantMin  int
		minSet   bool
	}{
		{"bare exit 0", "exit 0", 0, "", 0, false},
		{"explicit non-zero exit", "exit 1 (the gate refuses)", 1, "", 0, false},
		{"output is", "output is `rc=0`", 0, "rc=0", 0, false},
		{"exit plus minimum", "exit 0; output ≥ `1`", 0, "", 1, true},
		{"ascii minimum", "exit 0; >= 4 (doc index present)", 0, "", 4, true},
		{"at least", "at least 2 rows", 0, "", 2, true},
		{"prose only", "the reviewer weighs the residue", 0, "", 0, false},
		// The digit inside the expected OUTPUT must never be read as an
		// expected exit code — this is the row-4 shape from ground-truth/01's
		// own Verify table, where `rc=2` is what the command prints.
		{"digit in expected output", "output is `rc=2` (could-not-run is its own exit, not pass)", 0, "rc=2", 0, false},
		// "its own exit, not pass" has no digits after `exit`; a looser regex
		// would have swallowed a number from elsewhere in the sentence.
		{"exit word without a code", "exit code is its own, not pass", 0, "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseExpect(tc.expect)
			if got.exitCode != tc.wantExit {
				t.Errorf("exitCode = %d, want %d", got.exitCode, tc.wantExit)
			}
			if got.wantLine != tc.wantLine {
				t.Errorf("wantLine = %q, want %q", got.wantLine, tc.wantLine)
			}
			if got.minSet != tc.minSet || (tc.minSet && got.minCount != tc.wantMin) {
				t.Errorf("min = (%d, %v), want (%d, %v)", got.minCount, got.minSet, tc.wantMin, tc.minSet)
			}
		})
	}
}

func TestVerdictThreeState(t *testing.T) {
	cases := []struct {
		name   string
		expect string
		res    runResult
		want   string
	}{
		{"exit 0 with nothing else decidable", "exit 0", runResult{exit: 0}, statePass},
		{"wrong exit", "exit 0", runResult{exit: 1}, stateFail},
		{"expected output present", "output is `rc=0`", runResult{output: []byte("noise\nrc=0\n")}, statePass},
		{"expected output absent", "output is `rc=0`", runResult{output: []byte("rc=1\n")}, stateFail},
		{"count meets the floor", "≥ `3`", runResult{output: []byte("3\n")}, statePass},
		{"count below the floor", "≥ `3`", runResult{output: []byte("2\n")}, stateFail},
		// grep's multi-file form: the count is the trailing integer of the
		// last line, not the whole line.
		{"count in grep multi-file form", "≥ `1`", runResult{output: []byte("a.go:2\nb_test.go:5\n")}, statePass},
		// A declared numeric floor with no number to read against it is NOT a
		// pass on the exit status. This is the invariant the whole brief exists
		// for, in its subtlest form: the command ran, but no verdict came out.
		{"declared floor, unreadable output", "≥ `1`", runResult{output: []byte("no numbers here\n")}, stateCouldNotRun},
		{"could-not-run beats a matching exit", "exit 127", runResult{exit: 127, couldNotRun: true, reason: "not found"}, stateCouldNotRun},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := parseExpect(tc.expect).verdict(tc.res)
			if got != tc.want {
				t.Errorf("verdict = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCouldNotRunIsNeverPass is the invariant stated as its own test, because it
// is the one property every other part of the design is arranged to protect.
func TestCouldNotRunIsNeverPass(t *testing.T) {
	for _, expect := range []string{"exit 0", "exit 127", "output is `x`", "≥ `1`", "anything at all"} {
		res := runResult{exit: 127, couldNotRun: true, reason: "command not found"}
		if got, _ := parseExpect(expect).verdict(res); got == statePass {
			t.Fatalf("Expect %q turned a could-not-run into a pass", expect)
		}
	}
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

func TestRunVerifyCommandClassifiesCouldNotRun(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name        string
		command     string
		wantCouldNo bool
	}{
		{"ordinary success", "true", false},
		{"ordinary failure", "false", false},
		{"command not found", "definitely-not-a-real-binary-9f3a", true},
		{"empty command", "   ", true},
		{"unsubstituted placeholder", "cat <path-to-file>", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runVerifyCommand(root, tc.command, 30*time.Second)
			if got.couldNotRun != tc.wantCouldNo {
				t.Errorf("couldNotRun = %v (reason %q), want %v", got.couldNotRun, got.reason, tc.wantCouldNo)
			}
			if got.couldNotRun && got.reason == "" {
				t.Error("a could-not-run result must say why")
			}
		})
	}
}

func TestRunVerifyCommandTimesOutAsCouldNotRun(t *testing.T) {
	got := runVerifyCommand(t.TempDir(), "sleep 5", 100*time.Millisecond)
	if !got.couldNotRun {
		t.Fatalf("a timed-out row must be could-not-run, got %+v", got)
	}
	if !strings.Contains(got.reason, "timed out") {
		t.Errorf("reason = %q, want it to name the timeout", got.reason)
	}
}

func TestRunVerifyCommandRunsAtTheRootAndUnescapesPipes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "marker.txt"), []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := runVerifyCommand(root, `cat marker.txt \| grep -c .`, 30*time.Second)
	if got.couldNotRun || got.exit != 0 {
		t.Fatalf("unexpected result %+v", got)
	}
	if strings.TrimSpace(string(got.output)) != "2" {
		t.Errorf("output = %q, want %q — the `\\|` must reach the shell as a pipe", got.output, "2")
	}
}

// TestRunVerifyCommandFreshShellPerRow pins the isolation claim: a row cannot
// leave shell state behind for the next one.
func TestRunVerifyCommandFreshShellPerRow(t *testing.T) {
	root := t.TempDir()
	if r := runVerifyCommand(root, "GT01_LEAK=leaked; export GT01_LEAK", 30*time.Second); r.exit != 0 {
		t.Fatalf("setup row failed: %+v", r)
	}
	got := runVerifyCommand(root, `printf '[%s]\n' "$GT01_LEAK"`, 30*time.Second)
	if strings.TrimSpace(string(got.output)) != "[]" {
		t.Errorf("output = %q — state leaked from the previous row's shell", got.output)
	}
}

// ---------------------------------------------------------------------------
// The witness row
// ---------------------------------------------------------------------------

func TestWitnessRowShapeAndRoundTrip(t *testing.T) {
	w := witness{
		ID: "2", Command: `go test ./... \| tee out`, State: statePass, Exit: 0,
		OutHash: "0123456789ab", Date: "2026-08-13", Runner: "human:alex", Tree: "b988d1753038",
	}
	row := w.row()
	want := "| 2 | `go test ./... \\| tee out` | pass exit=0 | sha256:0123456789ab | 2026-08-13 | human:alex @ b988d1753038 |"
	if row != want {
		t.Fatalf("row =\n%s\nwant\n%s", row, want)
	}
	if !isWitnessRow(row) {
		t.Error("a row verifyrun wrote must be recognised as a witness row")
	}
	if got := witnessStateOf(row); got != statePass {
		t.Errorf("state = %q, want pass", got)
	}
	// The escaped pipe must survive the table: splitting the row has to yield
	// the command back intact, not two shredded cells.
	if got := witnessCommandOf(row); got != w.Command {
		t.Errorf("command round-trip = %q, want %q", got, w.Command)
	}
}

// TestWitnessStateIsReadFromTheResultCellOnly is a REGRESSION test for a defect
// this brief's own Verify table caught. Row 5 of ground-truth/01 is
// `grep -c 'could-not-run' statusgen/verifyrun*.go` — a passing row whose
// COMMAND contains the literal text `could-not-run`. Reading the verdict by
// scanning the whole row found that token first and reported the green row as
// could-not-run, which flipped `--check` from 0 to 2. Verbatim content must
// never be able to impersonate a verdict.
func TestWitnessStateIsReadFromTheResultCellOnly(t *testing.T) {
	w := witness{ID: "5", Command: `grep -c 'could-not-run' statusgen/verifyrun*.go`,
		State: statePass, Exit: 0, OutHash: "70bfa9204c8b", Date: "2026-08-13",
		Runner: "app[bot]", Tree: "25156e9b2f3b"}
	if got := witnessStateOf(w.row()); got != statePass {
		t.Errorf("state = %q, want pass — a command mentioning could-not-run is not a could-not-run verdict", got)
	}
	// The mirror: `pass` appearing in a command must not green a failed row.
	w2 := witness{ID: "6", Command: `grep -c 'pass' verifyrun.go`, State: stateFail, Exit: 1,
		OutHash: "70bfa9204c8b", Date: "2026-08-13", Runner: "app[bot]", Tree: "25156e9b2f3b"}
	if got := witnessStateOf(w2.row()); got != stateFail {
		t.Errorf("state = %q, want fail", got)
	}
	// And a prose Evidence row that happens to say "exit=0" and quote a hash in
	// the wrong columns is not a witness.
	if isWitnessRow("| 1 | ran it, exit=0, sha256:aaaaaaaaaaaa | 2026-08-01 | human:alex |") {
		t.Error("a prose row was accepted as a witness")
	}
}

// TestCouldNotRunWitnessStaysUnrun pins the interlock with unrun.go: writing a
// could-not-run witness must not clear that row's UNRUN state on the board.
func TestCouldNotRunWitnessStaysUnrun(t *testing.T) {
	w := witness{ID: "1", Command: "flaky", State: stateCouldNotRun, Exit: -1,
		OutHash: "0123456789ab", Date: "2026-08-13", Runner: "human:alex", Tree: "abc123abc123"}
	row := w.row()
	if !strings.Contains(row, "exit=-") {
		t.Errorf("row = %q — a row that never ran must not show a numeric exit code", row)
	}
	rows := parseEvidenceRows(witnessHeader + "\n" + row)
	got := rows["1"]
	if len(got) != 1 {
		t.Fatalf("parseEvidenceRows found %d rows for id 1, want 1", len(got))
	}
	// The interlock: unrun.go must read a could-not-run witness as NOT proving
	// the row was run. If this ever passes, writing a could-not-run witness
	// would launder an unrunnable row into board coverage.
	if evidenceRowComplete(got[0]) {
		t.Error("a could-not-run witness cleared the UNRUN derivation — it must not")
	}
	// A passing witness, by contrast, must clear it.
	pass := witness{ID: "1", Command: "true", State: statePass, Exit: 0,
		OutHash: "0123456789ab", Date: "2026-08-13", Runner: "human:alex", Tree: "abc123abc123"}
	passRows := parseEvidenceRows(witnessHeader + "\n" + pass.row())
	if !evidenceRowComplete(passRows["1"][0]) {
		t.Error("a passing witness must count as a completed Evidence row")
	}
}

// ---------------------------------------------------------------------------
// --check
// ---------------------------------------------------------------------------

func TestCheckWitnessesThreeStatePerRow(t *testing.T) {
	verify := "| # | Command | Expect |\n" +
		"|---|---------|--------|\n" +
		"| 1 | `true` | exit 0 |\n" +
		"| 2 | `test -d statusgen` | exit 0 |\n" +
		"| 3 | `printf 'ok\\n'` | output is `ok` |\n" +
		"| 4 | `false` | exit 0 |\n"
	evidence := witnessHeader + "\n" +
		"| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-08-13 | app[bot] @ 000000000000 |\n" +
		"| 2 | `test -d docs` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-08-13 | app[bot] @ 000000000000 |\n" +
		"| 4 | `false` | fail exit=1 | sha256:aaaaaaaaaaaa | 2026-08-13 | app[bot] @ 000000000000 |\n"

	got := checkWitnesses(verify, evidence)
	want := map[string]string{"1": statePass, "2": stateCouldNotRun, "3": stateCouldNotRun, "4": stateFail}
	if len(got) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(got), len(want), got)
	}
	for _, f := range got {
		if want[f.ID] != f.State {
			t.Errorf("row %s: state = %q, want %q (%s)", f.ID, f.State, want[f.ID], f.Detail)
		}
	}
	if code := checkExitCode(got); code != verifyrunExitCouldNot {
		t.Errorf("exit = %d, want %d — could-not-check must dominate a plain failure", code, verifyrunExitCouldNot)
	}
}

func TestCheckExitCodesAreThreeDistinctValues(t *testing.T) {
	cases := []struct {
		name     string
		findings []checkFinding
		want     int
	}{
		{"all pass", []checkFinding{{"1", statePass, ""}, {"2", statePass, ""}}, verifyrunExitPass},
		{"a failure", []checkFinding{{"1", statePass, ""}, {"2", stateFail, ""}}, verifyrunExitFail},
		{"a gap", []checkFinding{{"1", statePass, ""}, {"2", stateCouldNotRun, ""}}, verifyrunExitCouldNot},
		{"both", []checkFinding{{"1", stateFail, ""}, {"2", stateCouldNotRun, ""}}, verifyrunExitCouldNot},
	}
	seen := map[int]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkExitCode(tc.findings); got != tc.want {
				t.Errorf("exit = %d, want %d", got, tc.want)
			}
		})
		seen[tc.want] = true
	}
	if len(seen) != 3 {
		t.Fatalf("the table exercises %d distinct exit codes, want 3 — 1 and 2 must not collapse", len(seen))
	}
}

// TestCheckStaleWitnessIsNotPass pins the anti-laundering rule: editing a Verify
// row after running it must not leave a green witness behind.
func TestCheckStaleWitnessIsNotPass(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | `go test ./... -run Everything` | exit 0 |\n"
	evidence := witnessHeader + "\n| 1 | `true` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-08-13 | app[bot] @ 0000 |\n"
	got := checkWitnesses(verify, evidence)
	if len(got) != 1 || got[0].State != stateCouldNotRun {
		t.Fatalf("stale witness = %+v, want a single could-not-run finding", got)
	}
}

// TestCheckIgnoresProseEvidence: the Evidence sections already on main are prose
// tables. They must read as "no witness", never as a witness.
func TestCheckIgnoresProseEvidence(t *testing.T) {
	verify := "| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n"
	evidence := "| # | Result | Date | Runner |\n|---|---|---|---|\n| 1 | ran it, all green | 2026-08-01 | human:alex |\n"
	got := checkWitnesses(verify, evidence)
	if len(got) != 1 || got[0].State != stateCouldNotRun {
		t.Fatalf("prose Evidence = %+v, want could-not-run (no witness)", got)
	}
}

// ---------------------------------------------------------------------------
// The shipped fixtures — the positive control and its red run
// ---------------------------------------------------------------------------

// repoRootFromTest returns the repository root as seen from the statusgen
// package directory, which is where `go test` runs.
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestFixtureBriefPassRunsGreen(t *testing.T) {
	root := repoRootFromTest(t)
	verify, _, err := briefSections(filepath.Join(root, "statusgen/testdata/verifyrun/brief-pass.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := briefVerifyRows(verify)
	if len(rows) != 4 {
		t.Fatalf("fixture has %d Verify rows, want 4", len(rows))
	}
	ws := runWitnesses(root, rows, "test-runner", "0000", "2026-08-13", 60*time.Second)
	for _, w := range ws {
		if w.State != statePass {
			t.Errorf("row %s: %s (%s) — the positive-control fixture must run green at the repo root", w.ID, w.State, w.Note)
		}
	}
}

// TestFixtureBriefMissingWitnessIsRed is the "a check ships with proof it can
// fail" requirement, discharged: the shipped positive control must make --check
// exit 2, and it must exercise all three arms of that verdict.
func TestFixtureBriefMissingWitnessIsRed(t *testing.T) {
	root := repoRootFromTest(t)
	verify, evidence, err := briefSections(filepath.Join(root, "statusgen/testdata/verifyrun/brief-missing-witness.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := checkWitnesses(verify, evidence)
	if code := checkExitCode(got); code != verifyrunExitCouldNot {
		t.Fatalf("the positive control exited %d, want %d — if this ever goes green the check has stopped checking", code, verifyrunExitCouldNot)
	}
	states := map[string]string{}
	for _, f := range got {
		states[f.ID] = f.State
	}
	want := map[string]string{"1": statePass, "2": stateCouldNotRun, "3": stateCouldNotRun, "4": stateFail}
	for id, wantState := range want {
		if states[id] != wantState {
			t.Errorf("row %s: %q, want %q", id, states[id], wantState)
		}
	}
}

// ---------------------------------------------------------------------------
// Writing the witness back
// ---------------------------------------------------------------------------

func TestAppendWitnessTableAppendsRatherThanOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brief-01-x.md")
	original := "# Brief\n\n## Verify\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n" +
		"## Evidence\n| # | Command | Result | Output | Date | Runner |\n" +
		"|---|---|---|---|---|---|\n" +
		"| 1 | `true` | fail exit=1 | sha256:aaaaaaaaaaaa | 2026-08-01 | human:alex @ 0000 |\n\n" +
		"## Review\nGate: model.\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	w := witness{ID: "1", Command: "true", State: statePass, Exit: 0, OutHash: "bbbbbbbbbbbb",
		Date: "2026-08-13", Runner: "human:alex", Tree: "1111"}
	if err := appendWitnessTable(path, witnessTable([]witness{w})); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	// The earlier, RED run must still be there. A green re-run that erased it
	// would be editing the recorded basis of a past judgement.
	if !strings.Contains(got, "fail exit=1") {
		t.Error("the pre-existing witness was overwritten — Evidence is a log, not a slot")
	}
	if !strings.Contains(got, "sha256:bbbbbbbbbbbb") {
		t.Error("the new witness was not appended")
	}
	// It must land INSIDE the Evidence section, not after the next heading.
	evIdx := strings.Index(got, "## Evidence")
	revIdx := strings.Index(got, "## Review")
	newIdx := strings.Index(got, "sha256:bbbbbbbbbbbb")
	if !(evIdx < newIdx && newIdx < revIdx) {
		t.Errorf("witness landed outside the Evidence section (evidence=%d new=%d review=%d)", evIdx, newIdx, revIdx)
	}
	// And both runs must be readable by the shared Evidence parser.
	_, ev, err := briefSections(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(parseEvidenceRows(ev)["1"]); n != 2 {
		t.Errorf("parseEvidenceRows sees %d rows for id 1, want 2 (both runs)", n)
	}
}

func TestAppendWitnessTableRefusesWithoutAnEvidenceSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "brief-01-x.md")
	if err := os.WriteFile(path, []byte("# Brief\n\n## Verify\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendWitnessTable(path, "| x |"); err == nil {
		t.Fatal("appending to a brief with no Evidence section must be an error, not a silent no-op")
	}
}

// ---------------------------------------------------------------------------
// Runner attribution — the security core
// ---------------------------------------------------------------------------

func TestRunnerFlagIsRefusedWithItsOwnMessage(t *testing.T) {
	for _, arg := range []string{"--runner=someone", "-runner", "--as", "--identity=x", "--attribution", "--who=me"} {
		out, errOut := captureVerifyrun(t, []string{arg, "--brief", "x.md"})
		if out.code != verifyrunExitUsageError {
			t.Errorf("%s: exit = %d, want %d", arg, out.code, verifyrunExitUsageError)
		}
		if !strings.Contains(errOut, "derived from the executing identity") {
			t.Errorf("%s: message = %q, want it to explain WHY the flag does not exist", arg, errOut)
		}
	}
}

func TestExecutingRunnerPrefersTheActionsIdentity(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_ACTOR", "assay-reviewer-app[bot]")
	got, ok := executingRunner(t.TempDir())
	if !ok || got != "assay-reviewer-app[bot]" {
		t.Fatalf("runner = (%q, %v), want the App slug verbatim", got, ok)
	}
}

func TestExecutingRunnerFallsBackToGitIdentity(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	root := t.TempDir()
	gitInit(t, root, "Alex Rivera", "alex@example.com")
	got, ok := executingRunner(root)
	if !ok || got != "human:alex" {
		t.Fatalf("runner = (%q, %v), want human:alex", got, ok)
	}
	// The token must be the shape the rest of the toolkit parses.
	if m := humanStampRe.FindStringSubmatch(" " + got); m == nil || m[1] != "alex" {
		t.Errorf("%q is not a parseable human stamp", got)
	}
}

func TestExecutingRunnerRecordsABotVerbatim(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	root := t.TempDir()
	gitInit(t, root, "assay-issue-loop-app[bot]", "1234+assay-issue-loop-app[bot]@users.noreply.github.com")
	got, ok := executingRunner(root)
	if !ok || got != "assay-issue-loop-app[bot]" {
		t.Fatalf("runner = (%q, %v), want the bot slug verbatim — a bot must not be rendered as human:", got, ok)
	}
	if strings.HasPrefix(got, "human:") {
		t.Error("a bot was recorded as a human")
	}
}

// TestExecutingRunnerFailsClosed: no derivable identity must yield NO runner, so
// the caller refuses rather than writing an unattributed witness.
func TestExecutingRunnerFailsClosed(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	t.Setenv("HOME", t.TempDir()) // no ~/.gitconfig to fall back on
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "absent"))
	root := t.TempDir()
	runGit(t, root, "init")
	if got, ok := executingRunner(root); ok {
		t.Fatalf("runner = %q, want a refusal — an unattributed witness is not a witness", got)
	}
}

// ---------------------------------------------------------------------------
// --lint NOTICE
// ---------------------------------------------------------------------------

func TestWitnessNoticesFireOnClosedBriefsWithoutWitnesses(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := `---
brief: wt/01
title: A closed brief with no witness
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [284]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture"]
---

# Brief 01

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`true`" + ` | exit 0 |
| 2 | ` + "`false`" + ` | exit 1 |

## Evidence
| # | Result | Date | Runner |
|---|--------|------|--------|
| 1 | ran it, green | 2026-08-01 | human:alex |

## Review
Gate: model.
`
	if err := os.WriteFile(filepath.Join(dir, "brief-01-closed.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Stream{Name: "wt", Dir: dir, Root: root, Status: "active",
		Briefs: []Brief{{Num: "01", Status: "done"}}}

	notices := witnessNotices([]*Stream{s})
	if len(notices) != 1 {
		t.Fatalf("got %d notices, want 1: %v", len(notices), notices)
	}
	if !strings.Contains(notices[0], "01 (done, 2 of 2 rows)") {
		t.Errorf("notice = %q, want it to name the brief and BOTH unwitnessed rows (prose Evidence is not a witness)", notices[0])
	}

	// A brief still open is not the NOTICE's business — the witness is a
	// closure-time claim.
	s.Briefs[0].Status = "implemented"
	if n := witnessNotices([]*Stream{s}); len(n) != 0 {
		t.Errorf("an implemented brief produced %v, want no notices", n)
	}
}

func TestWitnessNoticesSilentWhenEveryRowIsWitnessed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := `---
brief: wt/01
title: A closed brief whose rows are witnessed
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [284]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture"]
---

# Brief 01

## Verify
| # | Command | Expect |
|---|---------|--------|
| 1 | ` + "`true`" + ` | exit 0 |

## Evidence
` + witnessHeader + `
| 1 | ` + "`true`" + ` | pass exit=0 | sha256:aaaaaaaaaaaa | 2026-08-13 | app[bot] @ 000000000000 |

## Review
Gate: model.
`
	if err := os.WriteFile(filepath.Join(dir, "brief-01-witnessed.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Stream{Name: "wt", Dir: dir, Root: root, Status: "active",
		Briefs: []Brief{{Num: "01", Status: "verified"}}}
	if n := witnessNotices([]*Stream{s}); len(n) != 0 {
		t.Errorf("a fully-witnessed brief produced %v, want no notices", n)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

type verifyrunResult struct{ code int }

// captureVerifyrun runs the sub-command with stdout/stderr redirected to temp
// files and returns the exit code plus what landed on stderr.
func captureVerifyrun(t *testing.T, args []string) (verifyrunResult, string) {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "out")
	errPath := filepath.Join(t.TempDir(), "err")
	out, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errF, err := os.Create(errPath)
	if err != nil {
		t.Fatal(err)
	}
	defer errF.Close()
	code := runVerifyrun(args, out, errF)
	errF.Sync()
	raw, err := os.ReadFile(errPath)
	if err != nil {
		t.Fatal(err)
	}
	return verifyrunResult{code}, string(raw)
}

// gitInit scaffolds a repo with a pinned identity. It reuses gitinfo_test.go's
// runGit rather than adding a second one.
func gitInit(t *testing.T, root, name, email string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", name)
	runGit(t, root, "config", "user.email", email)
}
