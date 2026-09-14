package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lintdiff.go — the statusgen PROBLEM-diff guard behind deskevidence's landing gate
// (deskevidence: refuse a landing that adds a statusgen PROBLEM or lands outside
// docs/streams/). Same-day main-red episodes traced to deskevidence landing an Evidence
// row that reddened `statusgen --lint` — a bare sibling-path row, missing the `../<repo>/`
// prefix the lint requires — with no check catching it before the commit landed. The fix
// asks statusgen itself, twice: once against the landing worktree as it stands, once with
// the pending write staged, and refuses only what the second run adds.
//
// Two layers, two seams, matching this file's existing style (mintTokenFn, forgeForFn,
// publicRepoGateFn in deskevidence.go):
//
//   - lintDiffFn (production: lintDiffAt) is what cmdEvidence calls. It owns staging the
//     pending write into the worktree, running the before/after lint, diffing the PROBLEM
//     sets, and reverting the stage — the whole check as cmdEvidence sees it.
//   - statusgenLintFn (production: statusgenLintAt) is the one thing lintDiffAt calls
//     twice: a single `statusgen --root <dir> --lint` shell, returning the PROBLEM: lines
//     printed. Exposed as its own seam so lintDiffAt's staging/diff/revert mechanics can be
//     tested against a real temp directory without needing a real statusgen on PATH.
//
// Mirrors deskpreflight's own statusgen --lint shell (cmd/deskpreflight/main.go,
// runStatusgenLint): same neutral cwd (os.TempDir()), same "not on PATH is
// could-not-check, never a silent pass" rule (C-4 of the three-state instrument rule), same
// "a nonzero exit is a normal 'problems found' run, not a runner failure" reading — reused
// rather than reinvented, just applied twice and diffed instead of once and reported.

// lintDiffFn is the seam cmdEvidence calls for the whole statusgen PROBLEM-diff check.
// Tests stub it (see setupFake in deskevidence_test.go) so calling cmdEvidence never shells
// a real statusgen or touches a real tree merely by landing an Evidence row; a handful of
// dedicated tests exercise lintDiffAt directly, against a real t.TempDir() root, with
// statusgenLintFn stubbed one level down instead.
var lintDiffFn = lintDiffAt

// statusgenLintFn is the seam for ONE statusgen --lint shell; lintDiffAt calls it twice
// (before and after staging the pending write). Production: statusgenLintAt.
var statusgenLintFn = statusgenLintAt

// lintDiffAt runs statusgenLintFn(root) to get the PROBLEM set the landing worktree
// reports BEFORE this write (the pre-landing base), stages newContent at
// <root>/<targetRepoPath> (creating parent directories as needed), runs statusgenLintFn(root)
// again, and returns exactly the PROBLEM lines present after the stage but absent before it
// — what THIS landing would introduce. A PROBLEM already standing in the tree before the
// write is real, but it is not this landing's to block: that is the whole point of diffing
// against the pre-landing base rather than refusing on any red statusgen reports.
//
// Because the underlying statusgen PROBLEM: lines are returned VERBATIM (never
// re-summarised), a sibling-path PROBLEM introduced by this landing carries statusgen's own
// "prefix it ../<repo>/%s" hint unchanged — deskevidence does not need to know that hint's
// wording, only pass the line through.
//
// The stage is ALWAYS reverted before return — a deferred restore, on every path (a clean
// diff, an introduced-PROBLEM refusal, or a could-not-check from the second lint run) —
// because deskevidence's real write rides the GitHub Contents API, never this local
// mutation: the landing worktree must come back exactly as this function found it. Same
// discipline --root's own #1709 fix established for deskevidence: a tool that leaves a
// worktree silently different from what a human or another session expects has already
// caused one class of main-red.
func lintDiffAt(root, targetRepoPath string, newContent []byte) (introduced []string, err error) {
	before, err := statusgenLintFn(root)
	if err != nil {
		return nil, err
	}

	abs := filepath.Join(root, filepath.FromSlash(targetRepoPath))
	origContent, rerr := os.ReadFile(abs)
	origExists := rerr == nil
	if rerr != nil && !os.IsNotExist(rerr) {
		return nil, deskkit.Unverifiable("cannot read local "+abs+" to stage the statusgen PROBLEM-diff check", rerr)
	}

	if merr := os.MkdirAll(filepath.Dir(abs), 0o755); merr != nil {
		return nil, deskkit.Unverifiable("cannot create local directories under "+root+" to stage the statusgen PROBLEM-diff check", merr)
	}
	if werr := os.WriteFile(abs, newContent, 0o644); werr != nil {
		return nil, deskkit.Unverifiable("cannot stage the statusgen PROBLEM-diff check at "+abs, werr)
	}
	defer func() {
		if origExists {
			_ = os.WriteFile(abs, origContent, 0o644)
		} else {
			_ = os.Remove(abs)
		}
	}()

	after, aerr := statusgenLintFn(root)
	if aerr != nil {
		return nil, aerr
	}

	beforeSet := make(map[string]bool, len(before))
	for _, p := range before {
		beforeSet[p] = true
	}
	seen := map[string]bool{}
	for _, p := range after {
		if seen[p] || beforeSet[p] {
			continue
		}
		seen[p] = true
		introduced = append(introduced, p)
	}
	return introduced, nil
}

// statusgenLintAt shells `statusgen --root <root> --lint` from a neutral working directory
// (os.TempDir(), exactly as deskpreflight's runStatusgenLint does — cmd/deskpreflight/main.go
// — so statusgen scans exactly root and never the calling process's own cwd) and returns the
// PROBLEM: lines it printed. statusgen not being on PATH is Unverifiable (could-not-check,
// never a silent pass — C-4 of the three-state instrument rule, docs/three-state-instrument-rule.md);
// any other nonzero exit is the normal "problems found" outcome, not a runner failure —
// mirroring statusgen's own self-shell in statusgen/difflint.go (productionDiffLintRunner),
// which draws the same *exec.ExitError-is-not-an-error line.
func statusgenLintAt(root string) ([]string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return nil, deskkit.Unverifiable("statusgen is not on PATH — cannot check this landing for introduced PROBLEMs", err)
	}
	cmd := exec.Command("statusgen", "--root", root, "--lint")
	cmd.Dir = os.TempDir()
	out, cerr := cmd.CombinedOutput()
	if cerr != nil {
		if _, ok := cerr.(*exec.ExitError); !ok {
			return nil, deskkit.Unverifiable("could not run statusgen --lint at "+root, cerr)
		}
	}
	var problems []string
	for _, ln := range strings.Split(string(out), "\n") {
		if t := strings.TrimSpace(ln); strings.HasPrefix(t, "PROBLEM:") {
			problems = append(problems, t)
		}
	}
	return problems, nil
}
