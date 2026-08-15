// Foreign-commit / merge-masquerade detection — the mechanical form of the worker-desk
// skill's manual "Pre-PR self-check" (#22, #72).
//
// A worktree cut with `git worktree add -b <branch> <path>` off the CURRENT HEAD — instead
// of the mandated `git worktree add <path> origin/main --detach` — silently copies whatever
// branch that worktree's HEAD happened to be on, dragging a sibling PR's unreviewed commits
// into the new branch. #22's 2026-07-30 recurrence showed a worker briefed against exactly
// this in writing do it anyway: prose in a dispatch prompt is not a control. Something that
// resolves real repository state is.
//
// This file resolves two pieces of real state, both already documented as manual steps in
// .claude/skills/worker-desk/SKILL.md and now enforced automatically on every push:
//
//  1. Foreign commit: a commit ahead of origin/main on the pushed ref that is ALSO reachable
//     from some OTHER remote branch which is not itself already merged into origin/main —
//     i.e. it was not authored on this branch, it was dragged in from a sibling branch/PR
//     still in flight (#22).
//  2. Single-parent masquerade: a commit whose subject claims to be a merge (contains the
//     word "merge"/"merged") but has fewer than two parents — a fake rebase pretending to be
//     a merge (#72's `merge: rebase onto origin/main`, tracker#259, which left the branch 353
//     commits behind while claiming to be caught up).
//  3. Stray-base cut: the pushed ref's branch point IS the tip of a stray local branch
//     literally named `origin/main`. This is the second, quieter half of "cut from a non-main
//     base" and the one actually happening in this checkout — see strayBase below. A sibling
//     cut (1) drags foreign commits in; a stray-base cut drags nothing in, it just starts the
//     branch tens of commits behind, so (1) cannot see it.
//
// COVERAGE LIMITS, on the record rather than assumed away:
//   - Un-pushed / un-fetched siblings escape (1). Foreign attribution keys on
//     `git branch -r --contains`, so a commit dragged from a sibling branch that was never
//     pushed, or whose remote ref this repo has not fetched, is invisible. Consulting LOCAL
//     branches instead was considered and rejected: in a worktree-per-worker checkout the
//     pushed branch's own commits are reachable from several local refs, so it cry-wolfs.
//   - The remote-tracking base is read, not fetched (see resolveOriginMain's KNOWN CAP).
//
// FAIL-OPEN, BUT NEVER SILENT. This is a client-side pre-push hook with a documented
// Fail-OPEN contract (brief-10): it refuses only on a POSITIVE finding, never on an inability
// to check. That contract is kept — but "cannot check" is now REPORTED as could-not-check
// (baseFindings.indeterminate, printed loudly by main.go and audited) instead of returning an
// empty result that reads identically to "base is fine". Every skip path in this file appends
// a reason; none of them just `continue`s.
package main

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// shaRe bounds localSHA before it ever reaches a constructed git argv: a real pre-push
// hook always supplies a full 40-char hex object id, so anything else (including the
// placeholder values existing tests use) is refused-into-skip rather than shelled out —
// this also forecloses a leading-dash value being read as a git flag.
var shaRe = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// mergeWordRe matches a subject that OPENS with "merge"/"merged" as a whole word
// (case-insensitive) — the shape #72's actual failure took (`merge: rebase onto
// origin/main`, a verb claiming the commit's own action). Anchoring at the start (rather
// than matching the word anywhere in the subject) is deliberate: an interior mention —
// "resolve merge conflict", "describe the merge workflow", "simplify merge-base
// comparison" — describes ordinary work touching merges, not a commit claiming to BE one,
// and must not be refused. A genuine two-parent merge commit (e.g. GitHub's own "Merge
// pull request #N from ...", which also opens with the word) is still excluded downstream
// by the parent-count check, not by this regex.
var mergeWordRe = regexp.MustCompile(`(?i)^merged?\b`)

// foreignCommit names one laundered commit and the sibling branch it was found reachable
// from.
type foreignCommit struct {
	sha, subject, sourceBranch string
}

// mergeMasquerade names one commit whose subject claims to be a merge but has fewer than
// two parents.
type mergeMasquerade struct {
	sha, subject string
}

// strayBase names a pushed ref whose branch point off the true base is the tip of a stray
// LOCAL branch literally named `origin/main`.
type strayBase struct {
	strayTip string // refs/heads/origin/main
	trueBase string // refs/remotes/origin/main
	behind   int    // commits the pushed ref is missing from the true base (-1 = uncounted)
}

// baseFindings is one pushed ref's local base inspection.
//
// indeterminate is the field that makes this check honest: every path that cannot reach an
// answer records WHY here. An empty baseFindings therefore means "looked, found nothing";
// a populated indeterminate means "could not look" — two states the previous
// (nil, nil, nil) return conflated into one.
type baseFindings struct {
	foreign       []foreignCommit
	masquerades   []mergeMasquerade
	strayBases    []strayBase
	indeterminate []string
}

func (f *baseFindings) cannotCheck(format string, a ...any) {
	f.indeterminate = append(f.indeterminate, fmt.Sprintf(format, a...))
}

// gitOut runs git in dir (empty = current process cwd, matching the pre-push hook's own
// invocation contract — it always runs with cwd at the worktree root) via the execCommand
// test seam, returning trimmed stdout. Any error (non-zero exit, spawn failure) is returned
// unwrapped; every caller in this file treats it as "cannot determine — skip", matching this
// tool's stated Fail-OPEN contract (see main.go's package doc: brief-10).
func gitOut(dir string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveOriginMain resolves the base this check compares against, spelled FULLY QUALIFIED
// as `refs/remotes/origin/main`.
//
// The bare spelling `origin/main` is NOT safe here, and this tool's own subject is why. A
// checkout that has ever run `git branch origin/main` (or `git worktree add ... -b
// origin/main`) carries a real local branch literally named `refs/heads/origin/main`
// alongside the remote-tracking `refs/remotes/origin/main`. Bare `origin/main` is then
// AMBIGUOUS, and git's rev-parse precedence resolves it to `refs/heads/` FIRST — the local
// branch, which is exactly the ref that goes stale. Worse, `rev-parse --verify --quiet`
// SUPPRESSES the ambiguity warning: it exits 0 and prints the stale sha with no diagnostic
// at all. The guard would then compute `originMain..HEAD` against a base tens of commits
// behind the real main and find nothing — silently passing the wrong-base worktree it
// exists to catch. Measured in this repo, where such a stray branch exists.
//
// A remote-tracking ref cannot collide this way (`refs/remotes/origin/main` is one name for
// one ref), so the qualified form is unambiguous by construction. In a checkout WITHOUT the
// stray branch this resolves to the identical sha, so the qualification changes nothing for
// the normal case.
//
// KNOWN CAP (not covered here): this resolves the local remote-tracking ref and does NOT
// fetch. If that ref is itself stale — the repo has not fetched recently — the comparison
// base is still behind the true remote head, and a worktree cut from it is not detected.
// Fetching inside a pre-push hook is a separate contract decision (it would break offline
// pushes), so it is named as a limitation rather than silently assumed away.
func resolveOriginMain(dir string) (string, error) {
	return gitOut(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main")
}

// strayLocalOriginMain resolves the STRAY local branch `refs/heads/origin/main` — the branch
// that exists only because someone once ran `git branch origin/main` or `git worktree add
// ... -b origin/main`, and which is what makes the bare spelling ambiguous.
//
// Returns ("", false) when no such branch exists. Absence is a DETERMINATE answer here, not a
// could-not-check: this function is only ever called after resolveOriginMain has already
// succeeded, which proves git runs and the repo resolves, so the only remaining reason
// `rev-parse --verify --quiet refs/heads/origin/main` exits non-zero is that the ref is not
// there.
func strayLocalOriginMain(dir string) (string, bool) {
	sha, err := gitOut(dir, "rev-parse", "--verify", "--quiet", "refs/heads/origin/main")
	if err != nil || sha == "" {
		return "", false
	}
	return sha, true
}

// checkStrayBase detects the OTHER half of "cut from a non-main base": a ref whose branch
// point is the tip of the stray local `origin/main`.
//
// Why the foreign-commit check cannot see this. `git worktree add <path> origin/main
// --detach` — the exact recipe the worker-desk skill prescribes — resolves `origin/main`
// through git's rev-parse precedence, which puts refs/heads/ AHEAD of refs/remotes/. With the
// stray branch present that recipe silently checks out the STALE local branch and prints only
// a `warning: refname 'origin/main' is ambiguous.` Measured in this repo: the stray is ~34
// commits behind, and `git rev-parse --verify --quiet origin/main` returns the stale sha with
// no diagnostic at all. The resulting branch drags in no sibling commits — it is simply cut
// tens of commits behind — so the foreign-commit arm reports nothing and the push sails
// through. This arm is what sees it.
//
// Why it does not cry wolf. The finding is not "the branch is behind main" (every long-lived
// PR branch is, and refusing those would be useless noise). It is the far narrower "the
// branch point is EXACTLY the stray ref's tip, and that tip is not the true base" — a
// coincidence that essentially only occurs by having been cut from it. A branch that has
// since merged main has a branch point at the true base and is not flagged; a repo with no
// stray branch can never produce this finding at all.
func checkStrayBase(dir, localSHA, trueBase string, out *baseFindings) {
	strayTip, present := strayLocalOriginMain(dir)
	if !present || strayTip == trueBase {
		return // no stray branch, or it happens to point at the true base — harmless
	}
	mergeBase, err := gitOut(dir, "merge-base", localSHA, trueBase)
	if err != nil {
		out.cannotCheck("could not compute the branch point of %s off refs/remotes/origin/main "+
			"(git merge-base failed: %v) — stray-base check NOT performed", shortSHA(localSHA), err)
		return
	}
	if mergeBase != strayTip {
		return
	}
	behind := -1
	if countOut, cerr := gitOut(dir, "rev-list", "--count", localSHA+".."+trueBase); cerr == nil {
		if n, perr := strconv.Atoi(strings.TrimSpace(countOut)); perr == nil {
			behind = n
		}
	}
	out.strayBases = append(out.strayBases, strayBase{strayTip: strayTip, trueBase: trueBase, behind: behind})
}

// branchIsAncestorOfMain reports whether remote branch b is an ancestor of originMain via
// `git merge-base --is-ancestor`. determinate=false means the check itself could not be
// resolved (b doesn't resolve, or another git error) — the caller must then skip rather
// than guess, per this file's fail-open contract.
func branchIsAncestorOfMain(dir, b, originMain string) (isAncestor, determinate bool) {
	cmd := execCommand("git", "merge-base", "--is-ancestor", b, originMain)
	if dir != "" {
		cmd.Dir = dir
	}
	err := cmd.Run()
	if err == nil {
		return true, true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, true
	}
	return false, false
}

// checkForeignCommits inspects the commits unique to localSHA relative to origin/main (the
// range a git pre-push hook is given) and reports:
//
//   - foreign commits: also reachable from another remote branch not itself an ancestor of
//     origin/main (a sibling PR still in flight — #22's laundering failure mode).
//
//   - merge masquerades: subject claims "merge"/"merged" but has < 2 parents (#72's fake
//     single-parent "merge").
//
//   - stray-base cut: the ref's branch point is the tip of a stray local branch named
//     `origin/main` (see checkStrayBase).
//
// dir is the repository to run git in (empty = process cwd). ownBranch's own remote-tracking
// ref (if already pushed) is excluded from the "other branch" search so an UPDATE push never
// flags itself against its own prior state.
//
// Fails open — but never silently. Every "cannot determine" path (origin/main unresolvable,
// localSHA not a well-formed object id or not present in this repo, a git error mid-walk)
// records a could-not-check reason in the returned baseFindings instead of returning an empty
// result. The push is still allowed on indeterminacy alone (brief-10's documented Fail-OPEN
// contract for a client-side hook), but the caller prints and audits the reason, so
// "could not check the base" can never be mistaken for "the base is fine".
func checkForeignCommits(dir, ownBranch, localSHA string) (baseFindings, error) {
	var out baseFindings
	if !shaRe.MatchString(localSHA) {
		out.cannotCheck("local sha %q is not a well-formed object id — base checks NOT performed", localSHA)
		return out, nil
	}
	originMain, err := resolveOriginMain(dir)
	if err != nil || originMain == "" {
		reason := "refs/remotes/origin/main does not resolve (not fetched, or no origin remote)"
		if err != nil {
			reason = fmt.Sprintf("refs/remotes/origin/main does not resolve: %v", err)
		}
		// Deliberately NO fallback to the bare `origin/main` spelling: in a checkout carrying
		// the stray local branch, that fallback would silently substitute a stale base and
		// hand back a confident "nothing found". Report could-not-check instead.
		if strayTip, present := strayLocalOriginMain(dir); present {
			reason += fmt.Sprintf(" — and a stray local branch refs/heads/origin/main (%s) IS present, "+
				"so the bare spelling would resolve to it; refusing to guess", shortSHA(strayTip))
		}
		out.cannotCheck("%s — base checks NOT performed", reason)
		return out, nil
	}
	if _, err := gitOut(dir, "cat-file", "-e", localSHA); err != nil {
		out.cannotCheck("commit %s is not present in this repository (%v) — base checks NOT performed",
			shortSHA(localSHA), err)
		return out, nil
	}

	checkStrayBase(dir, localSHA, originMain, &out)

	logOut, err := gitOut(dir, "log", "--format=%H", originMain+".."+localSHA)
	if err != nil {
		out.cannotCheck("could not enumerate refs/remotes/origin/main..%s (%v) — foreign-commit "+
			"and masquerade checks NOT performed", shortSHA(localSHA), err)
		return out, nil
	}
	var shas []string
	for _, line := range strings.Split(logOut, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			shas = append(shas, line)
		}
	}
	if len(shas) == 0 {
		return out, nil // determinate: nothing ahead of the true base
	}

	ownRemote := "origin/" + ownBranch
	for _, sha := range shas {
		subject, serr := gitOut(dir, "log", "-1", "--format=%s", sha)
		if serr != nil {
			out.cannotCheck("could not read the subject of %s (%v) — that commit was NOT checked",
				shortSHA(sha), serr)
			continue
		}

		if mergeWordRe.MatchString(subject) {
			parentsOut, perr := gitOut(dir, "log", "-1", "--format=%P", sha)
			if perr != nil {
				out.cannotCheck("could not read the parents of %s (%v) — masquerade check NOT "+
					"performed for that commit", shortSHA(sha), perr)
			} else if parents := strings.Fields(parentsOut); len(parents) < 2 {
				out.masquerades = append(out.masquerades, mergeMasquerade{sha: sha, subject: subject})
			}
		}

		branchesOut, berr := gitOut(dir, "branch", "-r", "--contains", sha)
		if berr != nil {
			out.cannotCheck("could not list remote branches containing %s (%v) — foreign-commit "+
				"check NOT performed for that commit", shortSHA(sha), berr)
			continue
		}
		for _, raw := range strings.Split(branchesOut, "\n") {
			b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "* "))
			if b == "" || b == ownRemote || b == "origin/main" || strings.HasSuffix(b, "HEAD") {
				continue
			}
			isAnc, determinate := branchIsAncestorOfMain(dir, b, originMain)
			if !determinate {
				out.cannotCheck("could not determine whether %s is already merged into "+
					"refs/remotes/origin/main — %s was NOT cleared against it", b, shortSHA(sha))
				continue
			}
			if !isAnc {
				out.foreign = append(out.foreign, foreignCommit{sha: sha, subject: subject, sourceBranch: b})
				break
			}
		}
	}
	return out, nil
}

// shortSHA renders a commit id at the conventional 12-char display length.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
