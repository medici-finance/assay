package main

// worktree.go — validation of an operator-STATED home worktree for a --dry-run render.
//
// A dry run normally prints the agent's home as a not-yet-known placeholder, deliberately:
// deskwt owns where a worktree lands, and a predicted path in a prompt is a second source
// of truth for the one value the isolation floor rests on. But a dry run is also how a
// batch of prompts is previewed, and each preview then has that placeholder substituted by
// hand before it reaches an agent. When the operator STATES a home that already exists,
// that is not a prediction — so the verb may render it, AFTER proving it is a real worktree
// of the item's own repo under a sanctioned prefix and is not the shared checkout. The
// proof is what keeps "render a stated path" from re-opening "let the verb guess a path".
//
// The prefix rule here is the same two-line rule the worktree verb enforces on the paths it
// creates. The two commands are separate `main` packages, so the rule is DUPLICATED rather
// than imported; it is deliberately no LOOSER than the original. Do not relax it.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mainlineRef is the start point of a FRESH dispatch, spelled in full. The full spelling is
// load-bearing: git resolves `refs/heads/` ahead of `refs/remotes/`, so a bare `origin/main`
// silently prefers a stray LOCAL branch of that name where one exists, warns only on stderr,
// and exits 0.
const mainlineRef = "refs/remotes/origin/main"

// worktreeBase returns the ref the agent's worktree is cut from.
//
// A FRESH dispatch cuts from the mainline: there is nothing else to cut from.
//
// A RESUME — `--pr <N>`, an already-open change whose branch exists on the forge — cuts
// from THAT BRANCH's own remote ref. Cutting a resume from the mainline is the defect this
// exists to close: `deskwt` creates the branch with `-b <branch> <base>`, so with the
// mainline as base the new worktree's branch sat at MAIN's tip while the change's commits
// existed only on `refs/remotes/origin/<branch>`. Nothing said so — the branch name and the
// PR were right, only the commit was wrong — so a resuming agent that did not compare its
// HEAD against the change's reported head either lost the existing work or produced a diff
// that read as a full rewrite of it.
//
// The branch's remote tip is REFRESHED first: a remote-tracking ref this checkout last
// fetched hours ago is not the change's real tip either. The fetch is best-effort — it
// touches exactly one ref, and a failure (offline, no such branch) falls through to the
// checks below rather than failing a dispatch over a refresh.
//
// A READ-ONLY lane carries `--pr` as the change it is READING, not as a branch to resume:
// a reviewer checks the change's head out itself, as a detached HEAD, and a verifier reads
// merged main. Both keep the mainline as their start point.
//
// Every arm falls back to the mainline, so a `--pr` whose branch cannot be resolved here is
// the behaviour this verb has always had, never a base `deskwt` would refuse.
func worktreeBase(o dispatchOpts, branch string) string {
	if o.pr <= 0 || reviewKit(o.kit) || verifierKit(o.kit) {
		return mainlineRef
	}
	ref := "refs/remotes/origin/" + branch
	// Constructed argv, no shell: the branch name is already bounded by branchNameRe, and
	// the refspec is built from it rather than from any caller-supplied ref.
	_ = runCmd(o.root, "git", "fetch", "--quiet", "origin",
		"+refs/heads/"+branch+":"+ref)
	r := runCmd(o.root, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if r.err != nil || strings.TrimSpace(r.stdout) == "" {
		return mainlineRef
	}
	return ref
}

// worktreeTmpBase is the parent of the sanctioned `tracker-*` worktree prefix. It is a
// package var ONLY so tests can point it at a temp dir; production keeps the compiled-in
// `/private/tmp`, matching the worktree verb's own fixed allowlist. It is deliberately NOT
// wired to any env var or flag — an operator-relocatable prefix would defeat the allowlist.
var worktreeTmpBase = "/private/tmp"

// validateOperatorWorktree proves that an operator-stated path is a usable home worktree
// for the item whose repo is rooted at root, and returns the RESOLVED path to render. Each
// of the three checks fails closed (exit 5) naming the check that failed; the shared
// checkout is refused by IDENTITY first so it reads as the isolation-floor violation it is
// rather than as a bare prefix miss.
func validateOperatorWorktree(root, path string) (string, error) {
	// The item repo's own shared git-common-dir is the reference every check is made
	// against. A root that is not a git repo is could-not-check, never a named refusal:
	// "the item's checkout is not readable" is a different answer from "the stated path is
	// wrong", and only the second is a decision about the operator's input.
	itemCommon, err := gitOut(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"step %s: cannot resolve the item repo's git-common-dir under %s — the stated --worktree "+
				"cannot be checked against a repo whose own state is unreadable.", stepWorktreeCreate, root), err)
	}
	itemCommon = resolvePath(itemCommon)
	sharedCheckout := resolvePath(filepath.Dir(itemCommon))

	abs, aerr := filepath.Abs(path)
	if aerr != nil {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree %q cannot be made absolute (%v).", stepWorktreeCreate, path, aerr))
	}
	resolved := resolvePath(abs)

	// Check 3, FIRST: the shared checkout is the isolation floor — refused by identity, the
	// same order the worktree verb refuses it, so it never masquerades as a prefix miss.
	if resolved == sharedCheckout {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree %q resolves to the shared checkout (%s) — the isolation floor. An agent's "+
				"home is never the shared checkout; that is exactly the placement this verb refuses to render.",
			stepWorktreeCreate, path, sharedCheckout))
	}

	// Check 1: the resolved path sits under a sanctioned worktree prefix.
	if !worktreePrefixAllowed(resolved, sharedCheckout) {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree %q resolves to %s, outside the sanctioned worktree prefixes "+
				"(%s/tracker-* or <repo-root>/.claude/worktrees/).",
			stepWorktreeCreate, path, resolved, worktreeTmpBase))
	}

	// Check 2: the path IS a registered git worktree OF THE ITEM'S REPO. `show-toplevel`
	// equal to the resolved path rejects a plain directory or a subdir of one; the
	// git-common-dir equal to the item repo's rejects a worktree of a DIFFERENT repo — the
	// case a bare "directory exists" check would wave through.
	top, terr := gitOut(resolved, "rev-parse", "--show-toplevel")
	if terr != nil || resolvePath(top) != resolved {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree %q is not a registered git worktree (its `rev-parse --show-toplevel` is not "+
				"the path itself) — a typo or a plain directory is refused, never rendered.",
			stepWorktreeCreate, path))
	}
	common, cerr := gitOut(resolved, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if cerr != nil || resolvePath(common) != itemCommon {
		return "", deskkit.Refused(fmt.Sprintf(
			"step %s: --worktree %q is a worktree of a DIFFERENT repo (its git-common-dir is not the item "+
				"repo's) — the item's agent must be homed in the item's own repo.", stepWorktreeCreate, path))
	}
	return resolved, nil
}

// worktreePrefixAllowed is the worktree verb's own two-line prefix rule, duplicated (the
// verbs are separate main packages) and no looser: a direct `tracker-*` child of the
// resolved tmp base, or strictly under the resolved <repo-root>/.claude/worktrees/.
func worktreePrefixAllowed(resolved, sharedCheckout string) bool {
	if filepath.Dir(resolved) == resolvePath(worktreeTmpBase) &&
		strings.HasPrefix(filepath.Base(resolved), "tracker-") {
		return true
	}
	wtDir := resolvePath(filepath.Join(sharedCheckout, ".claude", "worktrees"))
	return strings.HasPrefix(resolved, wtDir+string(filepath.Separator))
}

// gitOut runs `git <args...>` in dir through the recording seam and returns trimmed stdout.
//
// A failure returns git's OWN message, not the bare `exit status 128` os/exec produces. That
// bare form is the hardest failure in the suite to bisect: there is nothing in it to search
// for and nothing that names which of the several git reads failed, so an operator's only
// move was to re-run each by hand. The caller decides what a failure MEANS (each of the
// checks above renders it as could-not-check, deliberately, because "the item's checkout is
// unreadable" is a different answer from "the stated path is wrong"); this only makes sure
// the words git said survive to be rendered.
func gitOut(dir string, args ...string) (string, error) {
	r := runCmd(dir, "git", args...)
	if r.err != nil {
		return "", r.run.Fail(deskkit.ExitUnverifiable, "`git %s` failed in %s",
			strings.Join(args, " "), dir)
	}
	return strings.TrimSpace(r.stdout), nil
}

// resolvePath returns p with symlinks resolved on its longest EXISTING ancestor and the
// non-existent tail preserved and cleaned — the same primitive the worktree verb uses so
// the prefix/identity checks compare RESOLVED paths and a symlink cannot masquerade as
// inside a sanctioned prefix. On a path whose ancestors are all unresolvable it returns
// the cleaned input (still subject to the checks, which then refuse it).
func resolvePath(p string) string {
	p = filepath.Clean(p)
	cur := p
	var tail []string
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			full := resolved
			for i := len(tail) - 1; i >= 0; i-- {
				full = filepath.Join(full, tail[i])
			}
			return filepath.Clean(full)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
