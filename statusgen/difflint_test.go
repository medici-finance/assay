package main

import (
	"reflect"
	"sort"
	"testing"
)

// fakeDiffLint builds a diffLintConfig whose seams are driven by in-memory maps
// keyed by tree root, so the differential logic is exercised without git.
func fakeDiffLint(baseSHA, headSHA string, byRoot map[string][]string, notices []string) diffLintConfig {
	const baseDir = "/tmp/base-worktree"
	return diffLintConfig{
		root:    "/repo",
		baseRef: "refs/remotes/origin/main",
		resolveBase: func(root, baseRef string) (string, string, error) {
			return baseSHA, headSHA, nil
		},
		relFromTop: func(root string) (string, error) { return ".", nil },
		worktreeFn: func(root, sha string) (string, func(), error) {
			return baseDir, func() {}, nil
		},
		lintRunner: func(treeRoot string, budget []string, changed, scope string) ([]string, []string, error) {
			if treeRoot == "/repo" {
				return byRoot["head"], notices, nil
			}
			return byRoot["base"], nil, nil
		},
	}
}

func sortedDiffKeys(res diffLintResult) ([]string, []string) {
	intro := append([]string(nil), res.introduced...)
	dem := append([]string(nil), res.demoted...)
	sort.Strings(intro)
	sort.Strings(dem)
	return intro, dem
}

func TestDiffLint_PreExistingDemoted(t *testing.T) {
	// A problem present on BOTH base and head is pre-existing => demoted, not fired.
	cfg := fakeDiffLint("basesha", "headsha", map[string][]string{
		"base": {"PROBLEM: brief-25: verifier row under ## Review"},
		"head": {"PROBLEM: brief-25: verifier row under ## Review"},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.introduced) != 0 {
		t.Fatalf("pre-existing problem should not fire; introduced=%v", res.introduced)
	}
	if len(res.demoted) != 1 {
		t.Fatalf("pre-existing problem should be demoted; demoted=%v", res.demoted)
	}
	if emitDiffLintResult(res) != 0 {
		t.Fatal("a demoted-only run must contribute 0 to the exit count")
	}
}

func TestDiffLint_DiffIntroducedFires(t *testing.T) {
	// A problem present ONLY at head is diff-introduced => fires.
	cfg := fakeDiffLint("basesha", "headsha", map[string][]string{
		"base": {},
		"head": {"PROBLEM: reconciler-spinout/brief-9: verified row with empty ## Evidence"},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.introduced) != 1 {
		t.Fatalf("diff-introduced problem must fire; introduced=%v", res.introduced)
	}
	if len(res.demoted) != 0 {
		t.Fatalf("nothing to demote; demoted=%v", res.demoted)
	}
	if emitDiffLintResult(res) != 1 {
		t.Fatal("a diff-introduced problem must contribute 1 to the exit count")
	}
}

func TestDiffLint_Mixed(t *testing.T) {
	// One pre-existing (demoted) + one diff-introduced (fires), in the same run.
	cfg := fakeDiffLint("basesha", "headsha", map[string][]string{
		"base": {"PROBLEM: brief-30: ledger paths missing prefix"},
		"head": {
			"PROBLEM: brief-30: ledger paths missing prefix",     // pre-existing
			"PROBLEM: brief-42: unbacked verified row (this PR)", // introduced
		},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	intro, dem := sortedDiffKeys(res)
	wantIntro := []string{"PROBLEM: brief-42: unbacked verified row (this PR)"}
	wantDem := []string{"PROBLEM: brief-30: ledger paths missing prefix"}
	if !reflect.DeepEqual(intro, wantIntro) {
		t.Fatalf("introduced=%v want %v", intro, wantIntro)
	}
	if !reflect.DeepEqual(dem, wantDem) {
		t.Fatalf("demoted=%v want %v", dem, wantDem)
	}
}

func TestDiffLint_PathPrefixNormalized(t *testing.T) {
	// The same logical problem carries the head root path at head and the base
	// worktree path at base; stripping the tree root must make them compare equal
	// (else a pre-existing problem would look diff-introduced).
	cfg := fakeDiffLint("basesha", "headsha", map[string][]string{
		"base": {"PROBLEM: /tmp/base-worktree/docs/streams/x/README.md: bad row"},
		"head": {"PROBLEM: /repo/docs/streams/x/README.md: bad row"},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.introduced) != 0 {
		t.Fatalf("path-only difference must not read as diff-introduced; introduced=%v", res.introduced)
	}
	if len(res.demoted) != 1 {
		t.Fatalf("path-normalized pre-existing problem must be demoted; demoted=%v", res.demoted)
	}
}

func TestDiffLint_BaseEqualsHeadInert(t *testing.T) {
	// base == HEAD: nothing to diff against => FULL TEETH (every head problem
	// fires, none demoted), never "demote everything".
	cfg := fakeDiffLint("samesha", "samesha", map[string][]string{
		"head": {"PROBLEM: brief-1: something", "PROBLEM: brief-2: else"},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.introduced) != 2 {
		t.Fatalf("base==HEAD must apply full teeth; introduced=%v", res.introduced)
	}
	if len(res.demoted) != 0 {
		t.Fatalf("base==HEAD must demote nothing; demoted=%v", res.demoted)
	}
	if res.couldNotCheck == "" {
		t.Fatal("base==HEAD must record a fall-back reason (three-state honesty)")
	}
	if emitDiffLintResult(res) != 2 {
		t.Fatal("full-teeth fall-back must count every head problem")
	}
}

func TestDiffLint_UnresolvableBaseFailsSafe(t *testing.T) {
	// Base ref unresolvable (shallow CI, git-archive): nothing demoted, every
	// head problem fires, and a could-not-check reason is recorded. A three-state
	// instrument that did not look never rounds up to green.
	cfg := fakeDiffLint("", "headsha", map[string][]string{
		"head": {"PROBLEM: brief-7: unbacked row"},
	}, nil)
	res, err := runDiffLintOne(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.introduced) != 1 || len(res.demoted) != 0 {
		t.Fatalf("unresolvable base must apply full teeth; introduced=%v demoted=%v", res.introduced, res.demoted)
	}
	if res.couldNotCheck == "" {
		t.Fatal("unresolvable base must record a could-not-check reason")
	}
	if emitDiffLintResult(res) != 1 {
		t.Fatal("full-teeth count must include the un-demoted head problem")
	}
}

func TestNormalizeDiffProblem(t *testing.T) {
	got := normalizeDiffProblem("PROBLEM: /repo/docs/streams/x/README.md: bad", "/repo")
	want := "docs/streams/x/README.md: bad"
	if got != want {
		t.Fatalf("normalize got %q want %q", got, want)
	}
	// NOTICE prefix and a no-path message also normalize.
	if g := normalizeDiffProblem("NOTICE: brief-9 message", "/repo"); g != "brief-9 message" {
		t.Fatalf("normalize no-path got %q", g)
	}
}
