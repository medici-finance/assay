package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Differential register lint (#191).
//
// `--lint` is the PR-side gate, but the register lint walks the whole product
// scope regardless of the diff: any register defect already sitting on main
// hard-fails the check on EVERY open PR that touches docs/streams/**, no matter
// what that PR changed, and the stale red never clears when main is fixed
// because nothing re-evaluates a pull_request check when the base moves.
//
// `--diff-base <ref>` turns `--lint` differential: it evaluates the register at
// the base (the merge-base of HEAD and <ref>) AND at the working tree, then
// emits PROBLEM: only for problems the diff INTRODUCES. A problem that is
// already present at the base is demoted to NOTICE: — it is already red on
// main's own status-regen, which is where it has an owner — so the gate keeps
// its teeth for the case it exists for (a PR that adds an unbacked `verified`
// row) while shedding the cross-PR coupling. It always prints a summary line
// distinguishing base-side from diff-side problems, so a reviewer or desk can
// route in one read rather than reconstructing the merge state by hand.
//
// Implementation mirrors the lint-audit self-shell (lintaudit.go): the current
// binary is re-invoked as `--lint` against a materialised base worktree and
// against the head tree, and the two PROBLEM sets are compared. The inner runs
// never carry --diff-base, so there is no recursion.
//
// Fail-safe direction: whenever the base cannot be resolved, materialised, or
// linted (a shallow CI checkout lacking the base commit, an unresolvable ref, a
// git-archive export with no history), NOTHING is demoted — every head problem
// stays a PROBLEM (identical to a plain `--lint`) and a could-not-check NOTICE
// says the differential was unavailable. A three-state instrument that did not
// look never rounds up to green (docs/three-state-instrument-rule.md).
//
// Inert on main itself: when the base resolves to HEAD (nothing to diff against,
// e.g. a run on main or a branch not ahead of it), the run falls back to a
// full-teeth `--lint` with a NOTICE — demoting against an identical tree would
// silence every real problem, which is exactly backwards for main, whose job is
// to catch them.

type diffLintConfig struct {
	root    string
	baseRef string
	budget  []string
	changed string // path to the --changed file, passed through verbatim
	scope   string

	// Injectable seams (production wiring in diffLintProductionConfig; fakes in
	// the tests). resolveBase returns the base and head SHAs; when they are
	// equal the caller treats the run as inert.
	resolveBase func(root, baseRef string) (baseSHA, headSHA string, err error)
	relFromTop  func(root string) (string, error)
	worktreeFn  func(root, sha string) (dir string, cleanup func(), err error)
	lintRunner  func(treeRoot string, budget []string, changed, scope string) (problems, notices []string, err error)
}

// diffLintResult is the parsed outcome of one root's differential run.
type diffLintResult struct {
	introduced    []string // diff-introduced problems (relative form) — these fire
	demoted       []string // pre-existing (base-side) problems — demoted to NOTICE
	notices       []string // head-side NOTICE lines, passed through
	couldNotCheck string   // non-empty => differential unavailable, full teeth applied
}

// runDiffLintRoots is the --diff-base entry point. It applies the differential
// per root (each root is its own git repo, exactly as the multi-root lint
// treats them) and returns a process exit code: 1 if any root has a
// diff-introduced problem OR could not be linted at all, 0 otherwise.
func runDiffLintRoots(roots []string, baseRef string, budget []string, changed, scope string) int {
	if len(roots) == 0 {
		roots = []string{"."}
	}
	exit := 0
	total := 0
	for _, root := range roots {
		if len(roots) > 1 {
			fmt.Fprintf(os.Stderr, "statusgen: === root %s ===\n", root)
		}
		cfg := diffLintProductionConfig(root, baseRef, budget, changed, scope)
		res, err := runDiffLintOne(cfg)
		if err != nil {
			// A head lint that would not even execute is a genuine failure, not a
			// demotable problem: report it and fail the root.
			fmt.Fprintln(os.Stderr, "statusgen: --diff-base:", err)
			exit = 1
			continue
		}
		n := emitDiffLintResult(res)
		total += n
		if n > 0 {
			exit = 1
		}
	}
	if total == 0 {
		fmt.Println("LINT: PASS")
	} else {
		fmt.Printf("LINT: FAIL %d problem(s)\n", total)
	}
	return exit
}

// runDiffLintOne performs the differential for a single root.
func runDiffLintOne(cfg diffLintConfig) (diffLintResult, error) {
	// Head lint first — its problems and notices are the run's subject. A head
	// lint that cannot execute at all is a hard error (returned to the caller);
	// a head lint that merely reports problems is the normal case.
	headProblems, headNotices, err := cfg.lintRunner(cfg.root, cfg.budget, cfg.changed, cfg.scope)
	if err != nil {
		return diffLintResult{}, fmt.Errorf("head lint: %w", err)
	}
	res := diffLintResult{notices: headNotices}

	fail := func(reason string) (diffLintResult, error) {
		// Full teeth: nothing demoted, every head problem fires, plus a
		// could-not-check NOTICE naming why the differential was unavailable.
		res.introduced = headProblems
		res.couldNotCheck = reason
		return res, nil
	}

	baseSHA, headSHA, err := cfg.resolveBase(cfg.root, cfg.baseRef)
	if err != nil || strings.TrimSpace(baseSHA) == "" {
		return fail(fmt.Sprintf("base ref %q could not be resolved to a merge-base with HEAD (%v) — every problem below is reported at full strength, none demoted", cfg.baseRef, err))
	}
	if baseSHA == headSHA {
		return fail(fmt.Sprintf("base (%s) resolves to HEAD — nothing to diff against, so this is a full `--lint` (demoting against an identical tree would silence every real problem)", short(baseSHA)))
	}

	rel, err := cfg.relFromTop(cfg.root)
	if err != nil {
		return fail(fmt.Sprintf("could not locate the root inside its worktree (%v) — full-strength lint, none demoted", err))
	}

	dir, cleanup, err := cfg.worktreeFn(cfg.root, baseSHA)
	if err != nil {
		return fail(fmt.Sprintf("base tree %s could not be materialised (%v) — a shallow checkout that lacks the base commit produces this; fetch enough history to reach the merge-base. Full-strength lint, none demoted", short(baseSHA), err))
	}
	defer cleanup()

	baseRoot := filepath.Join(dir, rel)
	baseProblems, _, err := cfg.lintRunner(baseRoot, cfg.budget, cfg.changed, cfg.scope)
	if err != nil {
		return fail(fmt.Sprintf("base lint against %s failed to execute (%v) — full-strength lint, none demoted", short(baseSHA), err))
	}

	// Compare on the tree-relative form so absolute paths (which differ between
	// the head root and the temp base worktree) never make an identical problem
	// look diff-introduced. A base problem present at head is pre-existing =>
	// demoted; a head problem absent from base is diff-introduced => fires.
	baseSet := map[string]bool{}
	for _, p := range baseProblems {
		baseSet[normalizeDiffProblem(p, baseRoot)] = true
	}
	seen := map[string]bool{}
	for _, p := range headProblems {
		key := normalizeDiffProblem(p, cfg.root)
		if seen[key] {
			continue
		}
		seen[key] = true
		if baseSet[key] {
			res.demoted = append(res.demoted, p)
		} else {
			res.introduced = append(res.introduced, p)
		}
	}
	return res, nil
}

// emitDiffLintResult prints one root's differential outcome and returns the
// number of diff-introduced problems (the exit-code contributors). NOTICEs
// (head-side, demoted, and the could-not-check line) never affect the count.
func emitDiffLintResult(res diffLintResult) int {
	var notices []string
	notices = append(notices, res.notices...)
	if res.couldNotCheck != "" {
		notices = append(notices, "diff-lint could-not-check: "+res.couldNotCheck)
	}
	for _, d := range res.demoted {
		notices = append(notices, "[pre-existing on base] "+stripProblemPrefix(d))
	}
	sort.Strings(notices)
	for _, n := range notices {
		fmt.Fprintln(os.Stderr, "NOTICE:", n)
	}
	for _, p := range res.introduced {
		fmt.Fprintln(os.Stderr, "PROBLEM:", stripProblemPrefix(p))
	}
	// The summary line — always printed, so a reviewer or desk routes base-side
	// vs diff-side in one read (#191, option 2, included even when option 1 fires).
	if res.couldNotCheck != "" {
		fmt.Fprintf(os.Stderr, "diff-lint: base-vs-diff comparison UNAVAILABLE — %d problem(s) reported at full strength, 0 demoted\n", len(res.introduced))
	} else {
		fmt.Fprintf(os.Stderr, "diff-lint: %d base-side problem(s) demoted to NOTICE, %d diff-introduced problem(s)\n", len(res.demoted), len(res.introduced))
	}
	return len(res.introduced)
}

// normalizeDiffProblem strips the PROBLEM:/NOTICE: prefix and the tree's own
// absolute root from a line so the same logical problem keys identically
// whether it came from the head root or the temp base worktree.
func normalizeDiffProblem(line, treeRoot string) string {
	s := stripProblemPrefix(line)
	if abs, err := filepath.Abs(treeRoot); err == nil && abs != "" {
		s = strings.ReplaceAll(s, abs, "")
	}
	s = strings.ReplaceAll(s, treeRoot, "")
	// A stripped absolute root leaves a leading separator ("/repo/docs/..." ->
	// "/docs/..."); trim it so the key is a clean tree-relative path. Both trees
	// are normalised the same way, so this only affects legibility, not matching.
	return strings.TrimPrefix(strings.TrimSpace(s), "/")
}

func stripProblemPrefix(line string) string {
	s := strings.TrimSpace(line)
	for _, p := range []string{"PROBLEM:", "NOTICE:"} {
		if strings.HasPrefix(s, p) {
			return strings.TrimSpace(strings.TrimPrefix(s, p))
		}
	}
	return s
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// ---- production wiring ----

func diffLintProductionConfig(root, baseRef string, budget []string, changed, scope string) diffLintConfig {
	return diffLintConfig{
		root:        root,
		baseRef:     baseRef,
		budget:      budget,
		changed:     changed,
		scope:       scope,
		resolveBase: productionResolveBase,
		relFromTop:  productionRelFromTop,
		worktreeFn:  productionWorktree, // shared with lintaudit.go
		lintRunner:  productionDiffLintRunner,
	}
}

// productionResolveBase returns the merge-base of HEAD and baseRef, and HEAD.
// baseRef defaults to the fully-qualified remote-main ref when empty.
func productionResolveBase(root, baseRef string) (string, string, error) {
	if strings.TrimSpace(baseRef) == "" {
		baseRef = remoteMainRef
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", "", fmt.Errorf("rev-parse HEAD: %w", err)
	}
	mb, err := exec.Command("git", "-C", root, "merge-base", "HEAD", baseRef).Output()
	if err != nil {
		return "", strings.TrimSpace(string(head)), fmt.Errorf("merge-base HEAD %s: %w", baseRef, err)
	}
	return strings.TrimSpace(string(mb)), strings.TrimSpace(string(head)), nil
}

// productionRelFromTop returns root's path relative to its git worktree top, so
// the same subpath can be addressed inside the materialised base worktree.
func productionRelFromTop(root string) (string, error) {
	top, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse --show-toplevel: %w", err)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(strings.TrimSpace(string(top)), absRoot)
	if err != nil {
		return "", err
	}
	return rel, nil
}

// productionDiffLintRunner self-shells the current binary as `--lint` against
// one tree and returns its PROBLEM/NOTICE lines. It never passes --diff-base,
// so the inner run is an ordinary lint (no recursion). The lint exit code is
// deliberately ignored: a base or head tree legitimately reports problems, and
// the differential is a set comparison, not an exit-code relay. A genuine
// inability to execute the binary is surfaced as an error.
func productionDiffLintRunner(treeRoot string, budget []string, changed, scope string) ([]string, []string, error) {
	exe, err := os.Executable()
	if err != nil {
		exe = "statusgen"
	}
	args := []string{"--root", treeRoot, "--lint"}
	for _, b := range budget {
		args = append(args, "--budget", b)
	}
	if changed != "" {
		args = append(args, "--changed", changed)
	}
	if scope != "" {
		args = append(args, "--scope", scope)
	}
	cmd := exec.Command(exe, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// exec.ExitError means the binary ran and exited non-zero (the normal
		// "problems found" case) — that is NOT a runner error. Any other error
		// (binary not found, permission) is a genuine execution failure.
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, nil, fmt.Errorf("exec %s --lint: %w", exe, err)
		}
	}
	var problems, notices []string
	for _, l := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(l, "PROBLEM:"):
			problems = append(problems, l)
		case strings.HasPrefix(l, "NOTICE:"):
			notices = append(notices, l)
		}
	}
	return problems, notices, nil
}
