package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// boardAddSh is the one home of the ignored-board-path check the scaffolded
// regen job runs before it stages the board (#2915). Both CI templates embed it
// verbatim; boardIgnoreHits below runs the same two git commands, so `statusgen
// init`, `statusgen --lint` and the workflow step reach one verdict.
//
//go:embed boardadd.sh
var boardAddSh string

// boardAddPaths are the paths the scaffolded regen job stages with board_add.
// The GitHub half stages both; the GitLab half stages STATUS.md only.
var boardAddPaths = []string{"STATUS.md", "docs/streams"}

// boardIgnoreHit is one board path git would refuse to stage, with the ignore
// rule that matches it (<source>:<line>:<pattern>; empty when git names none).
type boardIgnoreHit struct {
	path, rule string
}

// message is the line boardadd.sh's board_ignore_message prints for the same
// path and rule — the shell oracle test pins the two byte-identical.
func (h boardIgnoreHit) message() string {
	if h.rule == "" {
		return fmt.Sprintf("%s cannot be staged: git add refuses it, but git check-ignore names no rule; run \"git add --dry-run -- %s\" to see why", h.path, h.path)
	}
	if strings.Contains(h.path, "/") {
		return fmt.Sprintf("%s is ignored by %s; add \"!%s\" to .gitignore, or narrow that rule if it ignores a parent directory (git cannot re-include a path under an ignored directory)", h.path, h.rule, h.path)
	}
	return fmt.Sprintf("%s is ignored by %s; add \"!%s\" to .gitignore", h.path, h.rule, h.path)
}

// errNotWorkTree reports that root is not inside a git work tree, so no ignore
// rule applies yet and the check could not run.
var errNotWorkTree = errors.New("not a git work tree")

// boardIgnoreHits asks git, for each path the regen job stages, whether the
// path-scoped `git add` would refuse it. The verdict is `git add --dry-run
// --ignore-missing` (it writes nothing, and it works before the path exists);
// `git check-ignore -v --no-index` then names the rule. check-ignore's exit
// status is NOT the verdict: with -v a matching `!` rule exits 0 too, and it
// misses a tracked directory under an ignored parent, which git add still
// refuses. A non-nil error means could-not-check, never clean.
func boardIgnoreHits(root string) ([]boardIgnoreHit, error) {
	var hits []boardIgnoreHit
	for _, p := range boardAddPaths {
		err := exec.Command("git", "-C", root, "add", "--dry-run", "--ignore-missing", "--", p).Run()
		if err == nil {
			continue
		}
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, err
		}
		switch ee.ExitCode() {
		case 1:
			hits = append(hits, boardIgnoreHit{path: p, rule: boardIgnoreRule(root, p)})
		case 128:
			return nil, errNotWorkTree
		default:
			return nil, fmt.Errorf("git add --dry-run -- %s exited %d", p, ee.ExitCode())
		}
	}
	return hits, nil
}

// boardIgnoreRule mirrors boardadd.sh's board_ignore_rule: the first
// `git check-ignore -v --no-index` line, cut at the tab that precedes the path.
func boardIgnoreRule(root, p string) string {
	out, _ := exec.Command("git", "-C", root, "check-ignore", "-v", "--no-index", "--", p).Output()
	line, _, _ := strings.Cut(string(out), "\n")
	rule, _, _ := strings.Cut(line, "\t")
	return rule
}

// boardIgnoreLint is the --lint half (#2915): a board path the scaffolded regen
// job would fail to stage is a PROBLEM, so an adopter finds it on a pull request
// rather than as a red first run on main. A tree with no .git is skipped — the
// run's up-front "git metadata unavailable" NOTICE already says git-dependent
// checks are degraded — and any other git failure is a could-not-check NOTICE.
func boardIgnoreLint(root string) (problems, notices []string) {
	if hasNoGitDir(root) {
		return nil, nil
	}
	hits, err := boardIgnoreHits(root)
	if err != nil {
		return nil, []string{fmt.Sprintf("could-not-check: whether .gitignore stops the regen job staging %s (%v) — not a pass", strings.Join(boardAddPaths, " and "), err)}
	}
	for _, h := range hits {
		problems = append(problems, h.message()+" — the scaffolded regen job stages it on every push to main, and git refuses an ignored path (statusgen never force-adds over your ignore rules)")
	}
	return problems, nil
}
