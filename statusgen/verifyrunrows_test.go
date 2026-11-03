package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// verifyrun --rows (desktools-v2/74): a run executes and records only the named
// Verify rows. Every test here runs on fixtures: a git home with a committed
// brief whose rows touch marker files outside it, the container bracket's fake
// deskdispatch and stubbed interface list, the acceptance transition's fake
// forge, and a fake docker for --in-container. No container engine, network or
// forge is used.

// rowsFixture is a home whose brief has Verify rows 1-4, each touching its own
// marker file outside the home.
type rowsFixture struct {
	root, brief, outside string
}

func newRowsFixture(t *testing.T, verify string) rowsFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the rows are sh commands touching marker files")
	}
	t.Setenv("DESK_LOOP", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	f := rowsFixture{root: t.TempDir(), outside: t.TempDir()}
	f.brief = filepath.Join(f.root, "docs", "brief.md")
	gitInit(t, f.root, "fixture", "fixture@example.invalid")
	if err := os.MkdirAll(filepath.Dir(f.brief), 0o755); err != nil {
		t.Fatal(err)
	}
	if verify == "" {
		verify = f.table("| # | Command | Expect |\n|---|---|---|\n", "1", "2", "3", "4")
	}
	putFile(t, f.brief, "# Fixture\n\n## Verify\n\n"+verify+"\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, f.root, "add", ".")
	runGit(t, f.root, "commit", "-q", "-m", "fixture")
	resetReadMemo()
	return f
}

// table renders Verify rows under header, one per ID, row i touching marker i.
func (f rowsFixture) table(header string, ids ...string) string {
	var b strings.Builder
	b.WriteString(header)
	for _, id := range ids {
		fmt.Fprintf(&b, "| %s | `touch %s` | exit 0 |\n", id, f.marker(id))
	}
	return b.String()
}

func (f rowsFixture) marker(id string) string { return filepath.Join(f.outside, "m"+id) }

// touched lists the IDs (1-4) whose markers exist, then removes them.
func (f rowsFixture) touched(t *testing.T) []string {
	t.Helper()
	var got []string
	for _, id := range []string{"1", "2", "3", "4"} {
		if _, err := os.Stat(f.marker(id)); err == nil {
			got = append(got, id)
			if err := os.Remove(f.marker(id)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return got
}

func (f rowsFixture) args(extra ...string) []string {
	return append([]string{"--root", f.root, "--brief", f.brief}, extra...)
}

// pinClock fixes the witness Date for the test.
func pinClock(t *testing.T) {
	t.Helper()
	saved := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = saved })
}

// evidenceOf reads a brief's Evidence section fresh from disk.
func evidenceOf(t *testing.T, path string) string {
	t.Helper()
	resetReadMemo()
	_, ev, err := briefSections(path)
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// TestVerifyrunRowsUnrestrictedGolden: without --rows the run writes the table
// it wrote before --rows existed: a golden built from the witness format, byte
// for byte. It uses no --rows step, so it runs unchanged on the tree before the
// flag existed. TestVerifyrunRowsSelectsOnlyNamed runs the same check.
func TestVerifyrunRowsUnrestrictedGolden(t *testing.T) { checkUnrestrictedGolden(t) }

// checkUnrestrictedGolden runs the unrestricted golden on its own fresh fixture.
func checkUnrestrictedGolden(t *testing.T) {
	t.Helper()
	pinClock(t)
	f := newRowsFixture(t, "")
	before := getFile(t, f.brief)
	tree := treeSHA(f.root)
	if res, out := captureVerifyrunOut(t, f.args()); res.code != verifyrunExitPass || strings.Contains(out, "not selected") {
		t.Fatalf("unrestricted run: exit %d output %q", res.code, out)
	}
	if got := f.touched(t); strings.Join(got, ",") != "1,2,3,4" {
		t.Fatalf("an unrestricted run ran rows %v, want all four", got)
	}
	golden := witnessHeader
	for _, id := range []string{"1", "2", "3", "4"} {
		golden += fmt.Sprintf("\n| %s | `touch %s` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | human:fixture @ %s (git-config) |", id, f.marker(id), tree)
	}
	if want := strings.Replace(before, "## Evidence\n", "## Evidence\n\n"+golden+"\n", 1); getFile(t, f.brief) != want {
		t.Fatalf("an unrestricted run is not byte-identical to the pre-change output:\n--- got\n%s\n--- want\n%s", getFile(t, f.brief), want)
	}
}

// regression: #4247
// TestVerifyrunRowsSelectsOnlyNamed: `--rows 2,4` runs and records rows 2 and 4
// only, under the standard header so they key by their # IDs; --check then
// credits only those rows; stdout names the others not selected; a run without
// --rows writes the table it wrote before the flag existed (the golden, on its
// own fixture, first: it is also TestVerifyrunRowsUnrestrictedGolden);
// --in-container carries the selector to the inner run.
func TestVerifyrunRowsSelectsOnlyNamed(t *testing.T) {
	checkUnrestrictedGolden(t)
	pinClock(t)
	f := newRowsFixture(t, "")
	res, out := captureVerifyrunOut(t, f.args("--rows", "2,4"))
	if res.code != verifyrunExitPass {
		t.Fatalf("--rows 2,4: exit %d output %q", res.code, out)
	}
	if got := f.touched(t); strings.Join(got, ",") != "2,4" {
		t.Fatalf("--rows 2,4 ran rows %v, want exactly 2 and 4", got)
	}
	for _, want := range []string{"row 1: not selected\n", "row 3: not selected\n", "row 2: pass", "row 4: pass"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout does not name %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "row 1: not selected") > strings.Index(out, "row 2: pass") || strings.Index(out, "row 3: not selected") > strings.Index(out, "row 4: pass") {
		t.Fatalf("stdout is not in table order:\n%s", out)
	}
	// The appended table: the standard header, then exactly rows 2 and 4.
	ev := evidenceOf(t, f.brief)
	table := strings.TrimSpace(ev)
	lines := strings.Split(table, "\n")
	if !strings.HasPrefix(table, witnessHeader+"\n") || len(lines) != 4 {
		t.Fatalf("the appended table is not the standard header over two rows:\n%s", ev)
	}
	for i, id := range []string{"2", "4"} {
		if cells := splitRowEscaped(lines[2+i]); len(cells) < 6 || strings.TrimSpace(cells[0]) != id {
			t.Fatalf("data row %d is not row %s: %q", i+1, id, lines[2+i])
		}
	}
	rows := parseEvidenceRows(ev)
	if len(rows) != 2 || len(rows["2"]) != 1 || len(rows["4"]) != 1 || len(rows["1"]) != 0 {
		t.Fatalf("parseEvidenceRows keys %v, want exactly 2 and 4 (an ordinal key would be 1 and 2)", rowKeys(rows))
	}
	for _, id := range []string{"2", "4"} {
		if witnessCommandOf(rows[id][0].Text) != "touch "+f.marker(id) {
			t.Fatalf("row %s carries another row's witness: %q", id, rows[id][0].Text)
		}
	}
	// --check audits the whole table: 2 and 4 pass, 1 and 3 have no witness.
	res, out = captureVerifyrunOut(t, []string{"--check", f.brief})
	if res.code != verifyrunExitCouldNot {
		t.Fatalf("--check after --rows 2,4: exit %d, want could-not-run; output %q", res.code, out)
	}
	for _, want := range []string{"row 1: " + stateCouldNotRun, "row 2: " + statePass, "row 3: " + stateCouldNotRun, "row 4: " + statePass} {
		if !strings.Contains(out, want) {
			t.Fatalf("--check does not report %q:\n%s", want, out)
		}
	}

	// --in-container: the builder and the docker argv carry --rows 2,4.
	if inner := buildInnerCommand("docs/brief.md", false, false, false, "", "2,4"); !argvHasPair(inner, "--rows", "2,4") {
		t.Fatalf("the inner command drops --rows: %q", inner)
	}
	if inner := buildInnerCommand("docs/brief.md", false, false, false, "", ""); strings.Join(inner, " ") != "statusgen verifyrun --brief docs/brief.md" {
		t.Fatalf("an inner command without --rows changed: %q", inner)
	}
	root := vicHostIdentRoot(t, "\tname = Quillon Vex\n\temail = qvex@example.invalid\n")
	argvFile, envFile := filepath.Join(t.TempDir(), "argv"), filepath.Join(t.TempDir(), "env")
	vicFakeDocker(t, argvFile, envFile)
	if res, out := captureVerifyrunOut(t, []string{"--in-container", "--root", root, "--brief", filepath.Join(root, "b.md"), "--rows", "2,4"}); res.code != verifyrunExitPass {
		t.Fatalf("--in-container --rows 2,4: exit %d output %q", res.code, out)
	}
	argv := strings.Split(strings.TrimRight(getFile(t, argvFile), "\n"), "\n")
	if !argvHasPair(argv, "--rows", "2,4") || !argvHasPair(argv, "--brief", "b.md") {
		t.Fatalf("the docker argv does not carry --rows 2,4 to the inner run: %q", argv)
	}
}

func rowKeys(m map[string][]evidenceRow) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// regression: #4247
// TestVerifyrunRowsRefusals: every malformed or unmatched selector, and --rows
// with a mode that runs no rows, is a usage error that runs no row and leaves
// the brief byte-identical, with and without --dry-run, and names the ID or
// flag.
func TestVerifyrunRowsRefusals(t *testing.T) {
	f := newRowsFixture(t, "")
	// shared: two rows carry the ID 2 (they touch markers 2 and 3).
	shared := newRowsFixture(t, "")
	rewriteVerify(t, shared, "| # | Command | Expect |\n|---|---|---|\n"+
		"| 1 | `touch "+shared.marker("1")+"` | exit 0 |\n"+
		"| 2 | `touch "+shared.marker("2")+"` | exit 0 |\n"+
		"| 2 | `touch "+shared.marker("3")+"` | exit 0 |\n")
	// unnumbered: the same four rows with no # column.
	unnumbered := newRowsFixture(t, "")
	plain := "| Command | Expect |\n|---|---|\n"
	for _, id := range []string{"1", "2", "3", "4"} {
		plain += "| `touch " + unnumbered.marker(id) + "` | exit 0 |\n"
	}
	rewriteVerify(t, unnumbered, plain)

	cases := []struct {
		name  string
		fx    rowsFixture
		value string
		names string
	}{
		{"unknown ID", f, "9", "carries no row 9"},
		{"unknown among known", f, "2,9", "carries no row 9"},
		{"empty list", f, "", `--rows ""`},
		{"comma only", f, ",", "element 1 is empty"},
		{"trailing comma", f, "2,", "element 2 is empty"},
		{"duplicate", f, "2,2", "row 2 is named twice"},
		{"duplicate after spaces", f, "2, 2", "row 2 is named twice"},
		{"shared ID", shared, "2", "2 Verify rows share the ID 2"},
		{"no # column", unnumbered, "1", "no `#` column"},
	}
	for _, c := range cases {
		for _, dry := range []bool{false, true} {
			before := getFile(t, c.fx.brief)
			extra := []string{"--rows", c.value}
			if dry {
				extra = append(extra, "--dry-run")
			}
			res, stderr := captureVerifyrun(t, c.fx.args(extra...))
			if res.code != verifyrunExitUsageError || !strings.Contains(stderr, c.names) || !strings.Contains(stderr, "No row ran") {
				t.Fatalf("%s (dry-run %v): exit %d stderr %q, want a usage refusal naming %q", c.name, dry, res.code, stderr, c.names)
			}
			if got := c.fx.touched(t); len(got) != 0 {
				t.Fatalf("%s (dry-run %v): a refused selector ran rows %v", c.name, dry, got)
			}
			if getFile(t, c.fx.brief) != before {
				t.Fatalf("%s (dry-run %v): a refused selector changed the brief", c.name, dry)
			}
		}
	}

	// --rows with a mode that runs no rows: refused, the flag named.
	witness := tempWitness(t, "")
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	modes := map[string][]string{
		"--check":          {"--check", "--rows", "2", f.brief},
		"--open-bracket":   {"--open-bracket", "--root", f.root, "--brief", f.brief, "--out", receipt},
		"--close-bracket":  {"--close-bracket", "--root", f.root, "--brief", f.brief, "--admission-receipt", receipt, "--witness", witness},
		"--bracket-digest": {"--bracket-digest", "--witness", witness},
	}
	for flag, args := range modes {
		for _, dry := range []bool{false, true} {
			before := getFile(t, f.brief)
			a := append([]string{}, args...)
			if flag != "--check" {
				a = append(a, "--rows", "2")
			}
			if dry {
				a = append([]string{"--dry-run"}, a...)
			}
			res, stderr := captureVerifyrun(t, a)
			if res.code != verifyrunExitUsageError || !strings.Contains(stderr, "refusing --rows with "+flag) {
				t.Fatalf("--rows with %s (dry-run %v): exit %d stderr %q", flag, dry, res.code, stderr)
			}
			if got := f.touched(t); len(got) != 0 || getFile(t, f.brief) != before || getFile(t, witness) != "" {
				t.Fatalf("--rows with %s (dry-run %v): something ran or was written", flag, dry)
			}
			if _, err := os.Stat(receipt); err == nil {
				t.Fatalf("--rows with %s wrote a receipt", flag)
			}
		}
	}

	if res, out := captureVerifyrunOut(t, []string{"--help"}); res.code != verifyrunExitPass || !strings.Contains(out, "--rows <ids>") {
		t.Fatalf("verifyrun --help does not name --rows: exit %d", res.code)
	}

	// The control: a valid selector on the same fixture runs.
	if res, stderr := captureVerifyrun(t, f.args("--rows", " 3 ", "--dry-run")); res.code != verifyrunExitPass {
		t.Fatalf("control --rows ' 3 ' --dry-run: exit %d stderr %q", res.code, stderr)
	}
	if got := f.touched(t); strings.Join(got, ",") != "3" {
		t.Fatalf("control: ran rows %v, want 3", got)
	}
}

// rewriteVerify replaces the fixture brief's Verify table and commits it.
func rewriteVerify(t *testing.T, f rowsFixture, verify string) {
	t.Helper()
	putFile(t, f.brief, "# Fixture\n\n## Verify\n\n"+verify+"\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, f.root, "commit", "-q", "-am", "verify table")
	resetReadMemo()
}

// mixedFixture is the container bracket home with a brief whose check:ci rows
// C (1-3) print, and whose host-only rows H (4-5) pass only while a flag file
// outside the home exists: present for host runs, absent in the container.
type mixedFixture struct {
	containerFixture
	flag string
}

func newMixedFixture(t *testing.T) mixedFixture {
	t.Helper()
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	t.Setenv("GITHUB_ACTOR", "")
	mx := mixedFixture{containerFixture: fx, flag: filepath.Join(filepath.Dir(fx.receipt), "host-flag")}
	verify := "| # | Class | Command | Expect |\n|---|---|---|---|\n" +
		"| 1 | check:ci | `printf c1` | exit 0 |\n" +
		"| 2 | check:ci | `printf c2` | exit 0 |\n" +
		"| 3 | check:ci | `printf c3` | exit 0 |\n" +
		"| 4 | check | `test -e " + mx.flag + "` | exit 0 |\n" +
		"| 5 | check | `test -e " + mx.flag + " && printf h5` | exit 0 |\n"
	putFile(t, fx.brief, "# Fixture\n\n## Verify\n\n"+verify+"\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-q", "-m", "mixed")
	putFile(t, fx.check, validCheck)
	resetReadMemo()
	return mx
}

// container runs the container bracket over the brief as it stands (rows ""
// for every row): receipt, container-netns run, close. It returns the closed
// witness block.
func (mx mixedFixture) container(t *testing.T, rows string) string {
	t.Helper()
	if err := os.Remove(mx.flag); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Remove(mx.witness); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	mx.writeBracketReceipt(t, validCheck)
	args := mx.runArgs()
	if rows != "" {
		args = append(args, "--rows", rows)
	}
	res, out := captureVerifyrunOut(t, args)
	if res.code != verifyrunExitPass && res.code != verifyrunExitFail {
		t.Fatalf("container run --rows %q: exit %d output %q", rows, res.code, out)
	}
	if res, stderr := captureVerifyrun(t, []string{"--close-bracket", "--root", mx.home, "--brief", mx.brief, "--admission-receipt", mx.receipt, "--witness", mx.witness}); res.code != verifyrunExitPass {
		t.Fatalf("close after --rows %q: exit %d stderr %q", rows, res.code, stderr)
	}
	return getFile(t, mx.witness)
}

// host runs `--rows` on the host with the flag file present, so H passes.
func (mx mixedFixture) host(t *testing.T, rows string) {
	t.Helper()
	putFile(t, mx.flag, "host\n")
	res, out := captureVerifyrunOut(t, []string{"--root", mx.home, "--brief", mx.brief, "--rows", rows})
	if res.code != verifyrunExitPass {
		t.Fatalf("host run --rows %s: exit %d output %q", rows, res.code, out)
	}
	resetReadMemo()
}

// land appends a block at the end of Evidence, as the landing step places a
// closed container witness.
func (mx mixedFixture) land(t *testing.T, block string) {
	t.Helper()
	body := getFile(t, mx.brief)
	at := strings.Index(body, "\n## Review")
	if at < 0 {
		t.Fatalf("fixture: no Review section in %q", body)
	}
	putFile(t, mx.brief, strings.TrimRight(body[:at], "\n")+"\n\n"+strings.TrimRight(block, "\n")+"\n"+body[at:])
	resetReadMemo()
}

// check runs --check and returns its exit code and the state per row.
func (mx mixedFixture) check(t *testing.T) (int, map[string]string, string) {
	t.Helper()
	resetReadMemo()
	res, out := captureVerifyrunOut(t, []string{"--check", mx.brief})
	states := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^row (\S+): (\S+) — `).FindAllStringSubmatch(out, -1) {
		states[m[1]] = m[2]
	}
	return res.code, states, out
}

func wantStates(t *testing.T, name string, got map[string]string, want map[string]string, out string) {
	t.Helper()
	for id, s := range want {
		if got[id] != s {
			t.Fatalf("%s: row %s is %q, want %q\n%s", name, id, got[id], s, out)
		}
	}
}

// hostTableRows lifts the data rows of the host witness table in a brief.
func hostTableRows(t *testing.T, path string) []string {
	t.Helper()
	var rows []string
	for _, line := range strings.Split(evidenceOf(t, path), "\n") {
		if strings.HasPrefix(line, "| 4 |") || strings.HasPrefix(line, "| 5 |") {
			rows = append(rows, line)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("fixture: the host table does not hold rows 4 and 5: %q", rows)
	}
	return rows
}

// regression: #4247
// TestVerifyrunRowsMixedAssembly: Evidence assembled from a container bracket
// over the check:ci rows and a host run of the host-only rows, both with
// --rows, passes --check in either landing order; either table alone leaves
// the other rows could-not-run; last-wins is unchanged; a hand-pasted,
// headerless, fenced host table is credited to no H row.
func TestVerifyrunRowsMixedAssembly(t *testing.T) {
	all := map[string]string{"1": statePass, "2": statePass, "3": statePass, "4": statePass, "5": statePass}

	// (a) container first, then host.
	mx := newMixedFixture(t)
	mx.land(t, mx.container(t, "1,2,3"))
	mx.host(t, "4,5")
	code, states, out := mx.check(t)
	if code != verifyrunExitPass {
		t.Fatalf("(a) container then host: exit %d\n%s", code, out)
	}
	wantStates(t, "(a) container then host", states, all, out)
	if lines := verbLines(t, evidenceOf(t, mx.brief), verifyrunExitPass); len(lines) != 1 || !strings.Contains(lines[0], " verdict=match ") {
		t.Fatalf("(a) the landed bracket does not match: %q", lines)
	}

	// (a) host first, then container.
	mx = newMixedFixture(t)
	mx.host(t, "4,5")
	mx.land(t, mx.container(t, "1,2,3"))
	code, states, out = mx.check(t)
	if code != verifyrunExitPass {
		t.Fatalf("(a) host then container: exit %d\n%s", code, out)
	}
	wantStates(t, "(a) host then container", states, all, out)

	// (b) the container table alone: H has no witness.
	mx = newMixedFixture(t)
	mx.land(t, mx.container(t, "1,2,3"))
	code, states, out = mx.check(t)
	if code == verifyrunExitPass {
		t.Fatalf("(b) container table alone passed\n%s", out)
	}
	wantStates(t, "(b)", states, map[string]string{"1": statePass, "2": statePass, "3": statePass, "4": stateCouldNotRun, "5": stateCouldNotRun}, out)

	// (c) the host table alone: C has no witness.
	mx = newMixedFixture(t)
	mx.host(t, "4,5")
	code, states, out = mx.check(t)
	if code == verifyrunExitPass {
		t.Fatalf("(c) host table alone passed\n%s", out)
	}
	wantStates(t, "(c)", states, map[string]string{"1": stateCouldNotRun, "2": stateCouldNotRun, "3": stateCouldNotRun, "4": statePass, "5": statePass}, out)

	// (d) a host H table, then an unrestricted container table recording H as
	// fail: the later witness wins.
	mx = newMixedFixture(t)
	mx.host(t, "4,5")
	mx.land(t, mx.container(t, ""))
	code, states, out = mx.check(t)
	if code == verifyrunExitPass {
		t.Fatalf("(d) a later container fail was outweighed by an earlier host pass\n%s", out)
	}
	wantStates(t, "(d)", states, map[string]string{"1": statePass, "2": statePass, "3": statePass, "4": stateFail, "5": stateFail}, out)

	// (e) the host H rows hand-pasted, headerless, in a text fence after an
	// unrestricted container table recording H as fail: the fenced rows key by
	// ordinal (1, 2), so rows 1 and 2 are could-not-run and H keeps its fail.
	mx = newMixedFixture(t)
	pristine := getFile(t, mx.brief)
	mx.host(t, "4,5")
	pasted := hostTableRows(t, mx.brief)
	putFile(t, mx.brief, pristine)
	resetReadMemo()
	mx.land(t, mx.container(t, ""))
	mx.land(t, "```text\n"+strings.Join(pasted, "\n")+"\n```")
	code, states, out = mx.check(t)
	if code == verifyrunExitPass {
		t.Fatalf("(e) a fenced, headerless host table was credited\n%s", out)
	}
	wantStates(t, "(e)", states, map[string]string{"1": stateCouldNotRun, "2": stateCouldNotRun, "3": statePass, "4": stateFail, "5": stateFail}, out)
}

// regression: #4247
// TestVerifyrunRowsContainerBracket: a container-netns run with --rows
// witnesses only the named rows under one Tree-Digest, Container-Image and
// attestation line; the close step brackets it and --bracket-digest matches,
// with a host --rows table above or below the block; every container
// admission refusal still fires before any row with --rows given.
func TestVerifyrunRowsContainerBracket(t *testing.T) {
	fx := newContainerFixture(t)
	t.Setenv("DESK_LOOP", "")
	t.Setenv("GITHUB_ACTOR", "")
	marker := func(id string) string { return filepath.Join(filepath.Dir(fx.receipt), "r"+id) }
	verify := "| # | Command | Expect |\n|---|---|---|\n"
	for _, id := range []string{"1", "2", "3", "4"} {
		verify += "| " + id + " | `touch " + marker(id) + "` | exit 0 |\n"
	}
	putFile(t, fx.brief, "# Fixture\n\n## Verify\n\n"+verify+"\n## Evidence\n\n## Review\n\nnone\n")
	runGit(t, fx.home, "add", ".")
	runGit(t, fx.home, "commit", "-q", "-m", "four rows")
	putFile(t, fx.check, validCheck)
	resetReadMemo()
	ran := func() []string {
		var got []string
		for _, id := range []string{"1", "2", "3", "4"} {
			if _, err := os.Stat(marker(id)); err == nil {
				got = append(got, id)
				if err := os.Remove(marker(id)); err != nil {
					t.Fatal(err)
				}
			}
		}
		return got
	}

	// Every refusal, with --rows, before any row runs.
	fx.writeBracketReceipt(t, validCheck)
	source := filepath.Join(fx.home, "source.txt")
	inside := filepath.Join(fx.home, "witness.md")
	refusals := []struct {
		name, why string
		args      []string
		setup     func()
		undo      func()
	}{
		{name: "missing receipt", why: "without --admission-receipt",
			args: fx.containerArgs("--container-image-digest", fixtureImage, "--witness", fx.witness)},
		{name: "missing image digest", why: "without --container-image-digest",
			args: fx.containerArgs("--admission-receipt", fx.receipt, "--witness", fx.witness)},
		{name: "witness inside the home", why: "inside the home",
			args: fx.containerArgs("--admission-receipt", fx.receipt, "--container-image-digest", fixtureImage, "--witness", inside)},
		{name: "existing witness", why: "the file exists",
			args:  fx.runArgs(),
			setup: func() { putFile(t, fx.witness, "earlier\n") },
			undo: func() {
				if err := os.Remove(fx.witness); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "tree differs from the receipt", why: "not the sha256:",
			args:  fx.runArgs(),
			setup: func() { putFile(t, source, "TAMPERED source\n") },
			undo:  func() { putFile(t, source, "attested source\n") }},
	}
	for _, c := range refusals {
		if c.setup != nil {
			c.setup()
		}
		before := getFile(t, fx.brief)
		witnessBefore := getFile(t, fx.witness)
		res, stderr := captureVerifyrun(t, append(append([]string{}, c.args...), "--rows", "2,4"))
		if res.code == verifyrunExitPass || !strings.Contains(stderr, c.why) {
			t.Fatalf("%s with --rows: exit %d stderr %q, want a refusal naming %q", c.name, res.code, stderr, c.why)
		}
		if got := ran(); len(got) != 0 {
			t.Fatalf("%s with --rows: rows %v ran", c.name, got)
		}
		if getFile(t, fx.brief) != before || getFile(t, fx.witness) != witnessBefore {
			t.Fatalf("%s with --rows: a refused run wrote the brief or the witness", c.name)
		}
		if _, err := os.Stat(inside); err == nil {
			t.Fatalf("%s with --rows: a witness was written inside the home", c.name)
		}
		if c.undo != nil {
			c.undo()
		}
	}
	if calls := getFile(t, fx.log); calls != "" {
		t.Fatalf("a container run started deskdispatch: %q", calls)
	}

	// The run: only rows 2 and 4, then one line of each.
	fx.writeBracketReceipt(t, validCheck)
	res, out := captureVerifyrunOut(t, append(fx.runArgs(), "--rows", "2,4"))
	if res.code != verifyrunExitPass {
		t.Fatalf("container --rows 2,4: exit %d output %q", res.code, out)
	}
	if got := ran(); strings.Join(got, ",") != "2,4" {
		t.Fatalf("container --rows 2,4 ran rows %v", got)
	}
	witness := getFile(t, fx.witness)
	rows := parseEvidenceRows(witness)
	if len(rows) != 2 || len(rows["2"]) != 1 || len(rows["4"]) != 1 {
		t.Fatalf("the container witness keys %v, want exactly 2 and 4:\n%s", rowKeys(rows), witness)
	}
	for prefix, want := range map[string]int{treeDigestPrefix: 1, containerImagePrefix: 1, attestationPrefix: 1, witnessHeader: 1} {
		if n := strings.Count(witness, prefix); n != want {
			t.Fatalf("the container witness holds %d of %q, want %d:\n%s", n, prefix, want, witness)
		}
	}
	if !strings.HasPrefix(witness, witnessHeader+"\n") {
		t.Fatalf("the container witness does not start with the standard header:\n%s", witness)
	}

	// The close appends one bracket line, and the digest matches.
	if res, stderr := captureVerifyrun(t, []string{"--close-bracket", "--root", fx.home, "--brief", fx.brief, "--admission-receipt", fx.receipt, "--witness", fx.witness}); res.code != verifyrunExitPass {
		t.Fatalf("close: exit %d stderr %q", res.code, stderr)
	}
	closed := getFile(t, fx.witness)
	if !strings.HasPrefix(closed, witness) || strings.Count(closed, admissionBracketPrefix) != 1 {
		t.Fatalf("the close did not append one bracket line:\n%s", closed)
	}
	const oneEach = "span-lines=tree-digest:1,container-image:1,attestation:1"
	if lines := verbLines(t, closed, verifyrunExitPass); len(lines) != 1 || !strings.Contains(lines[0], " verdict=match "+oneEach) {
		t.Fatalf("--bracket-digest of the closed --rows witness: %q", lines)
	}

	// A host --rows table with its attestation line, above the container block
	// and again below its bracket: the container bracket still matches.
	resetReadMemo()
	if res, out := captureVerifyrunOut(t, []string{"--root", fx.home, "--brief", fx.brief, "--rows", "1,3"}); res.code != verifyrunExitPass {
		t.Fatalf("host --rows 1,3: exit %d output %q", res.code, out)
	}
	if got := ran(); strings.Join(got, ",") != "1,3" {
		t.Fatalf("host --rows 1,3 ran rows %v", got)
	}
	host := strings.TrimSpace(evidenceOf(t, fx.brief))
	if !strings.HasPrefix(host, witnessHeader+"\n") || strings.Count(host, "\n") != 3 {
		t.Fatalf("the host --rows table is not the header over rows 1 and 3:\n%s", host)
	}
	att, _, _, err := verifierAdmissionReceipt([]byte(validCheck), fx.brief)
	if err != nil {
		t.Fatal(err)
	}
	hostBlock := host + "\n" + att + "\n"
	for name, text := range map[string]string{
		"host table above": "## Evidence\n\n" + hostBlock + "\n" + closed,
		"host table below": "## Evidence\n\n" + closed + "\n" + hostBlock,
	} {
		lines := verbLines(t, text, verifyrunExitPass)
		if len(lines) != 1 || !strings.Contains(lines[0], " verdict=match "+oneEach) {
			t.Fatalf("%s: --bracket-digest %q", name, lines)
		}
	}
}

// regression: #4247
// TestVerifyrunRowsAcceptanceTransition: under --acceptance-record the
// selector picks from the protected run set: one original row and one
// author-added row are written under the descriptor once, the author-added row
// in its own state; an ID outside the run set is refused with nothing
// written; --dry-run prints the selected table and writes nothing.
func TestVerifyrunRowsAcceptanceTransition(t *testing.T) {
	f := transitionFixtureAt(t)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	args := func(extra ...string) []string {
		return append([]string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge"}, extra...)
	}
	pristine := getFile(t, f.path)

	// An ID outside the protected run set: refused, nothing written.
	for _, dry := range []bool{false, true} {
		extra := []string{"--rows", "2,9"}
		if dry {
			extra = append(extra, "--dry-run")
		}
		res, out := captureVerifyrunOut(t, args(extra...))
		if res.code != verifyrunExitUsageError || !strings.Contains(out, "the protected run set") || !strings.Contains(out, "carries no row 9") {
			t.Fatalf("--rows 2,9 (dry-run %v): exit %d output %q", dry, res.code, out)
		}
		if getFile(t, f.path) != pristine {
			t.Fatalf("--rows 2,9 (dry-run %v) wrote the brief", dry)
		}
	}

	// --dry-run: the selected table on stdout, the brief untouched.
	resetReadMemo()
	res, out := captureVerifyrunOut(t, args("--rows", "2,3", "--dry-run"))
	if res.code != verifyrunExitPass {
		t.Fatalf("--rows 2,3 --dry-run: exit %d output %q", res.code, out)
	}
	if getFile(t, f.path) != pristine {
		t.Fatal("--rows 2,3 --dry-run wrote the brief")
	}
	if !strings.Contains(out, "row 1: not selected") || strings.Count(out, f.descriptor) != 1 || !strings.Contains(out, "| 2 | `sh check.sh --no-pull` |") || !strings.Contains(out, "| 3 | `printf supplemental` |") || strings.Contains(out, "| 1 | `true` |") {
		t.Fatalf("--rows 2,3 --dry-run does not print the descriptor once over rows 2 and 3 only:\n%s", out)
	}

	// The run: descriptor once, rows 2 and 3 only, row 3 author-added.
	resetReadMemo()
	res, out = captureVerifyrunOut(t, args("--rows", "2,3"))
	if res.code != verifyrunExitPass {
		t.Fatalf("--rows 2,3: exit %d output %q", res.code, out)
	}
	ev := evidenceOf(t, f.path)
	if strings.Count(ev, f.descriptor) != 1 {
		t.Fatalf("the descriptor is not written exactly once:\n%s", ev)
	}
	rows := parseEvidenceRows(ev[strings.Index(ev, f.descriptor):])
	if len(rows) != 2 || len(rows["2"]) != 1 || len(rows["3"]) != 1 {
		t.Fatalf("the transition table keys %v, want exactly 2 and 3:\n%s", rowKeys(rows), ev)
	}
	if witnessCommandOf(rows["2"][0].Text) != "sh check.sh --no-pull" || witnessTreeOf(rows["2"][0].Text) != f.amendment {
		t.Fatalf("row 2: %q", rows["2"][0].Text)
	}
	if !strings.Contains(rows["3"][0].Text, "author-added; execution exit=0; no acceptance credit") {
		t.Fatalf("row 3 is not in the author-added state: %q", rows["3"][0].Text)
	}
	if !strings.Contains(out, "row 1: not selected") {
		t.Fatalf("stdout does not name row 1 not selected:\n%s", out)
	}
}

// regression: #4247
// TestVerifyrunRowsTransitionSplitRunsDoNotClose: under an acceptance
// transition the protected run set is witnessed by ONE run, because its
// Verification-Table descriptor must appear exactly once in the run closure
// reads. Splitting it into two --rows runs is not a supported shape, and it
// fails closed: the second run is refused while the first run's Evidence is
// uncommitted, and once that is committed, the two descriptors' witnesses do
// not close.
func TestVerifyrunRowsTransitionSplitRunsDoNotClose(t *testing.T) {
	f := transitionFixtureAt(t)
	vfGit(t, f.root, vfMailVerifier, "config", "user.name", vfRunner)
	vfGit(t, f.root, vfMailVerifier, "config", "user.email", vfMailVerifier)
	descriptor := filepath.Join(t.TempDir(), "transition.txt")
	if err := os.WriteFile(descriptor, []byte(f.descriptor), 0o600); err != nil {
		t.Fatal(err)
	}
	args := func(extra ...string) []string {
		return append([]string{"--brief", f.path, "--root", f.root, "--acceptance-record", descriptor, "--acceptance-forge"}, extra...)
	}
	resetReadMemo()
	if res, out := captureVerifyrunOut(t, args("--rows", "2,3")); res.code != verifyrunExitPass {
		t.Fatalf("first --rows 2,3 run: exit %d output %q", res.code, out)
	}
	resetReadMemo()
	if res, out := captureVerifyrunOut(t, args("--rows", "1")); res.code == verifyrunExitPass {
		t.Fatalf("a second run over the uncommitted first run passed:\n%s", out)
	}
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	if res, out := captureVerifyrunOut(t, args("--rows", "1")); res.code != verifyrunExitPass {
		t.Fatalf("second --rows 1 run: exit %d output %q", res.code, out)
	}
	if n := strings.Count(evidenceOf(t, f.path), f.descriptor); n != 2 {
		t.Fatalf("the split runs wrote %d descriptors, want 2", n)
	}
	vfAppend(t, f.root, "\n\n"+vfPass)
	vfCommit(t, f.root, vfMailVerifier)
	resetReadMemo()
	code, out, diag := captureVerifyclosure(t, []string{"--root", f.root, "--brief", "vf/01", "--acceptance-forge"})
	if code == 0 || !strings.Contains(out+diag, "NOT accepted — protected table:") {
		t.Fatalf("split protected runs closed: %d %s %s", code, out, diag)
	}
}

// regression: #4247
// TestVerifyrunRowsFailFirstScope: under --rows the fail-first risk tagging
// reads only the selected rows, so with no risk-bearing row among them the
// notice names the selection, not the table (row 1 carries the risk here).
func TestVerifyrunRowsFailFirstScope(t *testing.T) {
	f := newRowsFixture(t, "| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 (live) |\n| 2 | `true` | exit 0 |\n")
	base := mustGitOut(t, f.root, "rev-parse", "HEAD")
	// Selecting the risk-bearing row 1 prints no notice (its exit is the
	// fail-first verdict: green at base is non-discriminating).
	for _, c := range []struct {
		rows, want, not string
		code            int
	}{
		{"2", "--fail-first: no risk-bearing row among the selected rows;", "in this table", verifyrunExitPass},
		{"1", "row 1: non-discriminating", "no risk-bearing row", verifyrunExitFail},
	} {
		resetReadMemo()
		res, out := captureVerifyrunOut(t, f.args("--rows", c.rows, "--fail-first", "--base", base, "--dry-run"))
		if res.code != c.code || !strings.Contains(out, c.want) || strings.Contains(out, c.not) {
			t.Fatalf("--rows %s --fail-first: exit %d output %q", c.rows, res.code, out)
		}
	}
}

// dropHashColumn removes the `#` column from the Verify table of a brief body
// (or of a bare Verify table), leaving the rows to their ordinal IDs.
func dropHashColumn(body string) string {
	verify, rest := body, ""
	if i := strings.Index(body, "## Evidence"); i >= 0 {
		verify, rest = body[:i], body[i:]
	}
	verify = regexp.MustCompile(`(?m)^\| (#|\d+) \|`).ReplaceAllString(verify, "|")
	return regexp.MustCompile(`(?m)^\|---\|`).ReplaceAllString(verify, "|") + rest
}

// regression: #4247
// TestVerifyRowsHashColumnNeutral: the protected-closure and acceptance-
// transition audits compare row slices with reflect.DeepEqual, so a table that
// adds or drops only a `#` column whose values equal the ordinals must parse
// to equal rows; --rows reads the `#`-cell bit from numberedVerifyRows'
// parallel slice, never from verifyRow.
func TestVerifyRowsHashColumnNeutral(t *testing.T) {
	classed := strings.Replace(strings.Replace(strings.Replace(vfVerify, "| Expect |", "| Expect | Class |", 1), "|--------|", "|--------|---|", 1), "| exit 1 |", "| exit 1 | check |", 1)
	classed = strings.Replace(classed, "| exit 0 |", "| exit 0 | check |", 1)
	for _, c := range []struct{ name, table string }{{"plain", vfVerify}, {"class", classed}} {
		t.Run(c.name, func(t *testing.T) {
			bare := dropHashColumn(c.table)
			if bare == c.table || strings.Contains(bare, "| # |") {
				t.Fatalf("the fixture lost no `#` column:\n%s", bare)
			}
			withHash, bareRows := briefVerifyRows(c.table), briefVerifyRows(bare)
			if len(withHash) != 2 || !reflect.DeepEqual(withHash, bareRows) {
				t.Fatalf("a `#`-only difference compares unequal:\n%+v\n%+v", withHash, bareRows)
			}
			_, n1 := numberedVerifyRows(c.table)
			_, n2 := numberedVerifyRows(bare)
			if !reflect.DeepEqual(n1, []bool{true, true}) || !reflect.DeepEqual(n2, []bool{false, false}) {
				t.Fatalf("numbered bits %v / %v", n1, n2)
			}
		})
	}
	// End to end: a delivery or an amendment that drops only the `#` column
	// still passes the acceptance transition, its execution and closure.
	for _, stage := range []string{"delivery", "amendment"} {
		t.Run(stage, func(t *testing.T) {
			f := contextFixture(t, nil, func(s, body string) string {
				if s == stage {
					return dropHashColumn(body)
				}
				return body
			})
			if strings.Contains(extractSectionByPrefix(f.amended, "Verify"), "| # |") {
				t.Fatalf("the amended table kept its `#` column:\n%s", f.amended)
			}
			runTransitionExecution(t, f)
		})
	}
}
