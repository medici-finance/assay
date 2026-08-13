package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The tests in this file exist because every mechanism that decides PASS from FAIL
// must die when its branch is deleted. A guard that cannot be shown to fail when
// removed has not been tested.

// --- parse-time structural guards ---

func TestParseCheckList_RejectsCheckWithNoRequireFiles(t *testing.T) {
	_, err := parseCheckList([]byte(
		`{"version":1,"checks":[{"name":"c","tool":"grep","args":["x","."],"invertExit":true}]}`))
	if err == nil || !strings.Contains(err.Error(), "declares no requireFiles") {
		t.Fatalf("a check with no requireFiles must be rejected at parse time, got: %v", err)
	}
}

func TestParseCheckList_RejectsPlainCheckWithNoRequireOutputMatch(t *testing.T) {
	_, err := parseCheckList([]byte(
		`{"version":1,"checks":[{"name":"c","tool":"echo","args":["ok"],"requireFiles":["."]}]}`))
	if err == nil || !strings.Contains(err.Error(), "declares no requireOutputMatch") {
		t.Fatalf("a non-inverted check with no requireOutputMatch must be rejected, got: %v", err)
	}
}

func TestParseCheckList_RejectsInvertExitWithOutputMatch(t *testing.T) {
	_, err := parseCheckList([]byte(
		`{"version":1,"checks":[{"name":"c","tool":"grep","args":["x","."],"invertExit":true,` +
			`"requireFiles":["."],"requireOutputMatch":"anything"}]}`))
	if err == nil || !strings.Contains(err.Error(), "invertExit and requireOutputMatch") {
		t.Fatalf("invertExit + requireOutputMatch is unsatisfiable and must be rejected, got: %v", err)
	}
}

func TestParseCheckList_RejectsBadOutputMatchRegexp(t *testing.T) {
	_, err := parseCheckList([]byte(
		`{"version":1,"checks":[{"name":"c","tool":"echo","args":["ok"],"requireFiles":["."],` +
			`"requireOutputMatch":"[unclosed"}]}`))
	if err == nil || !strings.Contains(err.Error(), "invalid requireOutputMatch") {
		t.Fatalf("an uncompilable requireOutputMatch must be rejected at parse time, got: %v", err)
	}
}

func TestParseCheckList_RejectsNegativeMinFiles(t *testing.T) {
	_, err := parseCheckList([]byte(
		`{"version":1,"checks":[{"name":"c","tool":"grep","args":["x","."],"invertExit":true,` +
			`"requireFiles":["."],"minFiles":-1}]}`))
	if err == nil || !strings.Contains(err.Error(), "minFiles") {
		t.Fatalf("minFiles < 1 must be rejected, got: %v", err)
	}
}

// --- countTreeFiles ---

func TestCountTreeFiles_ExcludesGitAndCountsRegularFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "a")
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustWrite(t, filepath.Join(dir, "sub", "b.txt"), "b")
	mustMkdir(t, filepath.Join(dir, ".git"))
	mustWrite(t, filepath.Join(dir, ".git", "obj"), "noise")

	n, err := countTreeFiles(dir)
	if err != nil {
		t.Fatalf("countTreeFiles: %v", err)
	}
	if n != 2 {
		t.Fatalf("countTreeFiles = %d, want 2 (.git must not count toward the work floor)", n)
	}

	if _, err := countTreeFiles(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("countTreeFiles must error on an absent path")
	}
}

// --- runChecks: the vacuous-pass guards ---

func TestRunChecks_AbsentRequiredPathIsUnverifiable(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rn","X","scripts"],`+
		`"invertExit":true,"requireFiles":["scripts"]}]}`)
	dir := t.TempDir()

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("a check whose required path is absent must not pass")
	}
	if !strings.Contains(err.Error(), "not usable in fetched tree") {
		t.Fatalf("error should name the unusable path, got: %v", err)
	}
}

// TestRunChecks_PresentButEmptyPathIsUnverifiable is the case a plain os.Stat cannot
// catch: the directory survives a rename, the content does not. `grep` over an empty
// directory exits 1 ("no match") — indistinguishable from a clean tree unless the
// runner insists something was there to read.
func TestRunChecks_PresentButEmptyPathIsUnverifiable(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rn","X","scripts"],`+
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("a check over a path that exists but holds nothing must NOT pass")
	}
	if !strings.Contains(err.Error(), "examined nothing") {
		t.Fatalf("error should say the check examined nothing, got: %v", err)
	}
}

// TestRunChecks_ZeroExitWithNoWorkIsUnverifiable is the same defect on the plain-check
// path: a tool that reports "0 resource found" exits 0, and a bare exit code cannot
// tell that apart from "validated everything".
func TestRunChecks_ZeroExitWithNoWorkIsUnverifiable(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"sh",`+
		`"args":["-c","echo 'Summary: 0 resource found in 0 file - Valid: 0'"],`+
		`"requireFiles":["base"],"minFiles":1,"requireOutputMatch":"Valid: [1-9][0-9]*"}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "base"))
	mustWrite(t, filepath.Join(dir, "base", "x.yaml"), "kind: X\n")

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("exit 0 with output proving nothing was examined must NOT pass")
	}
	if !strings.Contains(err.Error(), "requireOutputMatch") {
		t.Fatalf("error should name requireOutputMatch, got: %v", err)
	}
}

// TestRunChecks_NonZeroExitFailsEvenWhenOutputMatches keeps the two plain-check
// rejections independent. Without it, a check that exits non-zero is caught only
// incidentally — by its output failing requireOutputMatch — so deleting the exit-code
// branch leaves the suite green while a FAILING check reports only "could not be run".
// The tool here exits 1 but prints output that satisfies requireOutputMatch, so the
// exit code is the ONLY thing that can reject it.
func TestRunChecks_NonZeroExitFailsEvenWhenOutputMatches(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"sh",`+
		`"args":["-c","echo 'Summary: 3 resources found in 2 files - Valid: 3, Invalid: 1'; exit 1"],`+
		`"requireFiles":["base"],"minFiles":1,"requireOutputMatch":"Valid: [1-9][0-9]*"}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "base"))
	mustWrite(t, filepath.Join(dir, "base", "x.yaml"), "kind: X\n")

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("a plain check that exits non-zero must FAIL even when its output looks like work")
	}
	if !strings.Contains(err.Error(), `check "ck" failed`) {
		t.Fatalf("error must report a check FAILURE (not merely 'could not be run'), got: %v", err)
	}
}

func TestRunChecks_ZeroExitWithWorkPasses(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"sh",`+
		`"args":["-c","echo 'Summary: 21 resources found in 18 files - Valid: 11'"],`+
		`"requireFiles":["base"],"minFiles":1,"requireOutputMatch":"Valid: [1-9][0-9]*"}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "base"))
	mustWrite(t, filepath.Join(dir, "base", "x.yaml"), "kind: X\n")

	cov, err := runChecks(cl, dir)
	if err != nil {
		t.Fatalf("a check that reports real work must pass: %v", err)
	}
	if cov.Checks != 1 || cov.Files != 1 {
		t.Fatalf("coverage = %+v, want 1 check / 1 file", cov)
	}
}

// --- runChecks: invertExit exit-code discrimination ---

// TestRunChecks_InvertExit_ToolErrorIsNotAPass is the fail-open the exit-code
// classification exists to close: `grep` exits 1 for "no match" but 2 for an ERROR
// (bad regex, unreadable file). Accepting "any non-zero" would let a guard that never
// ran report as a clean tree.
func TestRunChecks_InvertExit_ToolErrorIsNotAPass(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"sh","args":["-c","exit 2"],`+
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))
	mustWrite(t, filepath.Join(dir, "scripts", "a.sh"), "echo hi\n")

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("an inverted check whose tool ERRORED (exit 2) must not pass")
	}
	if !strings.Contains(err.Error(), "guard malfunctioned") {
		t.Fatalf("error should say the guard malfunctioned, got: %v", err)
	}
}

// TestRunChecks_InvertExit_RealGrepBadRegexIsNotAPass is the same case driven through
// a real grep rather than a stand-in exit code, so it stays true if grep's convention
// is what changes.
func TestRunChecks_InvertExit_RealGrepBadRegexIsNotAPass(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rnE","${[A-Za-z_","scripts"],`+
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))
	mustWrite(t, filepath.Join(dir, "scripts", "a.sh"), "envsubst < \"${SRC:-/dev/null}\"\n")

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("a real grep with an unbalanced bracket expression must not report the tree clean")
	}
	if !strings.Contains(err.Error(), "could not be run") {
		t.Fatalf("error should classify this as unrunnable, got: %v", err)
	}
}

func TestRunChecks_InvertExit_PatternFoundFails(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rnE","FORBIDDEN","scripts"],`+
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))
	mustWrite(t, filepath.Join(dir, "scripts", "a.sh"), "FORBIDDEN\n")

	_, err := runChecks(cl, dir)
	if err == nil {
		t.Fatal("an inverted check whose forbidden pattern is present must fail")
	}
	if !strings.Contains(err.Error(), "forbidden pattern FOUND") {
		t.Fatalf("error should say the pattern was found, got: %v", err)
	}
}

func TestRunChecks_InvertExit_PatternAbsentPasses(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rnE","FORBIDDEN","scripts"],`+
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))
	mustWrite(t, filepath.Join(dir, "scripts", "a.sh"), "all clear\n")

	cov, err := runChecks(cl, dir)
	if err != nil {
		t.Fatalf("an inverted check whose pattern is absent must pass: %v", err)
	}
	if cov.Files != 1 {
		t.Fatalf("coverage files = %d, want 1", cov.Files)
	}
}

func TestRunChecks_MissingToolIsUnverifiable(t *testing.T) {
	cl := mustParse(t, `{"version":1,"checks":[{"name":"ck","tool":"deskadvisory-no-such-tool",`+
		`"args":["x"],"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`)
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "scripts"))
	mustWrite(t, filepath.Join(dir, "scripts", "a.sh"), "x\n")

	if _, err := runChecks(cl, dir); err == nil {
		t.Fatal("a check whose tool is absent from PATH must refuse, not skip")
	}
}

// --- advisory-state gate ---

// The temporary private fork exists only while the advisory is pre-disclosure. A
// pre-disclosure advisory reports state "draft" WITH a private_fork; the same advisory
// after publication reports state "published" with private_fork null. A gate admitting
// only "published" would refuse every advisory this tool can ever check.

func TestResolveFork_AdmitsPreDisclosureStates(t *testing.T) {
	for _, state := range []string{"triage", "draft"} {
		t.Run(state, func(t *testing.T) {
			withMockAPI(t, advisoryAPI(state, ""))
			slug, err := resolveFork("example-org/example-k8s", "GHSA-1111-2222-3333")
			if err != nil {
				t.Fatalf("state %q must be checkable (it is when the fork exists): %v", state, err)
			}
			if slug != "example-org/example-k8s-advisory-fork" {
				t.Fatalf("slug = %q", slug)
			}
		})
	}
}

func TestResolveFork_RefusesPostDisclosureStates(t *testing.T) {
	for _, state := range []string{"published", "closed"} {
		t.Run(state, func(t *testing.T) {
			withMockAPI(t, advisoryAPI(state, ""))
			_, err := resolveFork("example-org/example-k8s", "GHSA-1111-2222-3333")
			if err == nil {
				t.Fatalf("state %q carries no temporary private fork and must be refused", state)
			}
			if !strings.Contains(err.Error(), state) {
				t.Fatalf("error should name the offending state, got: %v", err)
			}
			if !strings.Contains(err.Error(), "draft") {
				t.Fatalf("error should name the states that DO carry a fork, got: %v", err)
			}
		})
	}
}

func TestAdvisoryStatesWithFork_ExcludesPublished(t *testing.T) {
	if advisoryStatesWithFork["published"] {
		t.Fatal("published advisories have no private fork — admitting one " +
			"means the tool refuses every advisory it can actually check")
	}
	if !advisoryStatesWithFork["draft"] {
		t.Fatal("draft is a pre-disclosure state that carries a private fork")
	}
}

// --- coverage reporting ---

func TestCoverage_StringNamesEachCheck(t *testing.T) {
	cov := coverage{Checks: 2, Files: 7, PerCheck: []string{"a: 3 file(s)", "b: 4 file(s)"}}
	s := cov.String()
	for _, want := range []string{"2 check(s)", "7 file(s) examined", "a: 3 file(s)", "b: 4 file(s)"} {
		if !strings.Contains(s, want) {
			t.Fatalf("coverage string %q missing %q", s, want)
		}
	}
}

func TestExitCodeOf_NonExitErrorIsNegative(t *testing.T) {
	if got := exitCodeOf(os.ErrNotExist); got != -1 {
		t.Fatalf("exitCodeOf(non-exit error) = %d, want -1 (a tool that never started "+
			"must not be classified as a clean exit status)", got)
	}
}

// TestCheckAdvisory_VacuousTreeIsUnverifiableEndToEnd drives the whole pipeline over a
// tree where the checkdef's paths exist but hold nothing, and the defect the guard
// looks for lives one directory away from where it looks.
func TestCheckAdvisory_VacuousTreeIsUnverifiableEndToEnd(t *testing.T) {
	checkJSON := `{"version":1,"checks":[{"name":"ck","tool":"grep",` +
		`"args":["-rnE","\\$\\{[A-Za-z_][A-Za-z_0-9]*:[-=]","deploy/scripts"],` +
		`"invertExit":true,"requireFiles":["deploy/scripts"],"minFiles":1}]}`
	withMockAPI(t, advisoryAPI("draft", checkJSON))
	// deploy/scripts exists but is empty; the unfixed script sits at scripts/.
	mockGitSeed(t, `mkdir -p deploy/scripts scripts && printf 'envsubst < "${SRC:-/dev/null}"\n' > scripts/realm-import.sh`)

	err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333")
	if err == nil {
		t.Fatal("a tree whose checked paths hold nothing — while the defect sits beside them — must NOT pass")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("expected IsUnverifiable, got %T: %v", err, err)
	}
}

// --- helpers ---

func mustParse(t *testing.T, s string) *checkList {
	t.Helper()
	cl, err := parseCheckList([]byte(s))
	if err != nil {
		t.Fatalf("parseCheckList(%s): %v", s, err)
	}
	return cl
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
