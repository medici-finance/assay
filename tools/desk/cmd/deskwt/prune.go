package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// skipEntry records a worktree that prune LEFT untouched, with why. The reason vocabulary
// mirrors the safety gate: locked / dirty / unpushed / unmerged / current / not-under-prefix / shared.
type skipEntry struct{ path, reason string }

// pruneResult is the outcome of one sweep.
type pruneResult struct {
	bookkept int // admin entries dropped by `git worktree prune` (dirs already gone)
	removed  int // merged+clean worktrees removed
	skips    []skipEntry
}

// cmdPrune implements `deskwt prune [--repo <path>] [--interval <dur>]`: bounded
// worktree-count reduction so stale worktrees can never accumulate into the E2BIG sandbox
// failure or the #742 writeguard false-positives (both driven by worktree sprawl).
//
// One sweep runs two steps:
//
//	Step A (always, safe bookkeeping): `git worktree prune --verbose` drops admin entries
//	  whose working directories are already gone. This changes no on-disk working tree.
//
//	Step B (count reduction, safe gate): walk the registered worktrees under the sanctioned
//	  prefixes and REMOVE (via the exact same safe-remove primitive as `remove`) ONLY the
//	  ones proven safe — NOT a LOCKED worktree (git will refuse to deregister it, so deleting
//	  its directory first would strand the registration — issue #264; the lock is checked
//	  FIRST, ahead of every content heuristic), NOT the shared checkout (identity), NOT the current worktree (cwd),
//	  tracked-clean, AND fully merged into origin/main (HEAD an ancestor of origin/main → every
//	  commit reachable from HEAD is already on the remote mainline, so nothing is lost). The
//	  merge check is the ACTIVE-WORKER guard: an unmerged branch (an open PR still in flight)
//	  is NOT an ancestor of origin/main, so it is LEFT untouched. Anything failing any check is
//	  LEFT and reported as skipped with a reason. There is NO --force anywhere.
//
// Without --interval it runs ONE sweep and exits (the boot-step behavior). With
// `--interval 30m` it becomes a self-contained ticking loop — sweep, sleep the interval,
// sweep again, forever — so a k8s desk pod can run `deskwt prune --interval 30m` as its own
// prune loop with no external scheduler; the prune travels with the pod. Each tick logs one
// summary line (timestamp + counts) to STDOUT (captured by pod logs), re-checks the
// kill switch / STOP flags between ticks (clean exit 3 on STOP/DISABLED), and exits 0 on
// SIGTERM/SIGINT for a clean pod shutdown.
//
// It is a local-only verb (filesystem/worktree state only, no outward call): it takes the
// audit line and the kill switch but NOT the outward-write rate limit, mirroring
// add/remove's classification (deskkit/ratelimit.go "Verb classes").
func cmdPrune(args []string) (err error) {
	ac := &auditCtx{verb: "prune"}
	defer func() { ac.finalize(err) }()

	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))
	// --repo lets a pod / manual sweep point prune at a sibling repo without a session cwd.
	// It only sets the git working DIR (never an argv value), so no injection surface; it
	// must be an existing directory inside a git worktree (verified via newPathGuard).
	repo := fs.String("repo", "", "run against this repo root instead of the current directory")
	// --interval turns the one-shot sweep into a self-contained ticking loop (Go
	// time.ParseDuration, e.g. 30m). Empty/zero → one-shot.
	intervalStr := fs.String("interval", "", "if set (e.g. 30m), loop: sweep every interval instead of once")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("refused: prune takes no flags but --repo/--interval (there is no --force): " + perr.Error())
	}
	if len(positionals) != 0 {
		return deskkit.Refused("refused: prune takes no positional arguments")
	}

	var interval time.Duration
	if *intervalStr != "" {
		d, derr := time.ParseDuration(*intervalStr)
		if derr != nil {
			return deskkit.Refused("refused: --interval is not a valid duration (e.g. 30m): " + *intervalStr)
		}
		if d <= 0 {
			return deskkit.Refused("refused: --interval must be positive: " + *intervalStr)
		}
		interval = d
	}

	dir := *repo
	if dir == "" {
		wd, gerr := getwd()
		if gerr != nil {
			return deskkit.Unverifiable("cannot resolve working directory", gerr)
		}
		dir = wd
	} else {
		info, serr := os.Stat(dir)
		if serr != nil {
			return deskkit.Unverifiable("--repo path does not exist or is unreadable: "+dir, serr)
		}
		if !info.IsDir() {
			return deskkit.Refused("refused: --repo path is not a directory: " + dir)
		}
	}
	ac.repo = repoOrEmpty(dir)

	guard, pgerr := newPathGuard(dir)
	if pgerr != nil {
		return pgerr
	}
	cwd := resolvePath(mustAbsOrRaw(dir))

	// One-shot mode: single sweep, detailed per-skip summary to stderr.
	if interval == 0 {
		res, serr := pruneSweep(guard, dir, cwd)
		if serr != nil {
			return serr
		}
		fmt.Fprintf(os.Stderr, "deskwt prune: pruned %d bookkeeping entr%s, removed %d merged+clean worktree%s, skipped %d\n",
			res.bookkept, plural(res.bookkept, "y", "ies"), res.removed, plural(res.removed, "", "s"), len(res.skips))
		for _, s := range res.skips {
			fmt.Fprintf(os.Stderr, "  skipped %s — %s\n", s.path, s.reason)
		}
		ac.detail = fmt.Sprintf("pruned %d bookkeeping, removed %d, skipped %d", res.bookkept, res.removed, len(res.skips))
		if res.bookkept == 0 && res.removed == 0 {
			ac.successResult = deskkit.ResultNoop
		}
		return nil
	}

	// Interval mode: self-contained ticking loop. SIGTERM/SIGINT → clean exit 0.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	stop := make(chan struct{})
	go func() { <-sigCh; close(stop) }()
	return runPruneLoop(guard, dir, cwd, interval, stop, ac)
}

// runPruneLoop is the ticking body factored out for testability: it sweeps immediately,
// then every `interval` until `stop` is signalled (clean nil → exit 0) or the kill
// switch / STOP flag fires between ticks (Guard's Disabled error → exit 3). It never
// sleeps a partial tick past a stop — the select blocks on both channels. Each tick emits
// one stdout summary line; a sweep error aborts the loop (fail closed).
func runPruneLoop(guard *pathGuard, dir, cwd string, interval time.Duration, stop <-chan struct{}, ac *auditCtx) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ticks := 0
	totalRemoved := 0
	for {
		// Re-check the kill switch / STOP flags at every iteration boundary:
		// a STOP/DISABLED armed mid-loop halts on the next tick with a clean exit-3 audit.
		if gErr := deskkit.Guard(); gErr != nil {
			fmt.Fprintf(os.Stdout, "%s deskwt prune: halting loop — %s\n", nowStamp(), gErr.Error())
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, halted (%s)", interval, ticks, totalRemoved, gErr.Error())
			return gErr
		}

		res, err := pruneSweep(guard, dir, cwd)
		if err != nil {
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, aborted on sweep error", interval, ticks, totalRemoved)
			return err
		}
		ticks++
		totalRemoved += res.removed
		fmt.Fprintf(os.Stdout, "%s deskwt prune tick %d: bookkept=%d removed=%d skipped=%d\n",
			nowStamp(), ticks, res.bookkept, res.removed, len(res.skips))

		select {
		case <-stop:
			fmt.Fprintf(os.Stdout, "%s deskwt prune: shutdown signal — exiting after %d tick(s)\n", nowStamp(), ticks)
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, stopped by signal", interval, ticks, totalRemoved)
			return nil
		case <-ticker.C:
			continue
		}
	}
}

// pruneSweep performs ONE prune sweep against the repo rooted at dir (Step A bookkeeping +
// Step B safe removals) and returns the counts. It prints nothing — the caller renders the
// one-shot or per-tick summary. cwd is the resolved current directory (never removed).
func pruneSweep(guard *pathGuard, dir, cwd string) (pruneResult, error) {
	var res pruneResult

	// Step A — bookkeeping prune (always safe: only drops entries for dirs already gone).
	// --verbose prints one line per dropped entry so we can report the count.
	pruneOut, aErr := runGit(dir, "worktree", "prune", "--verbose")
	if aErr != nil {
		return res, deskkit.Unverifiable("git worktree prune (bookkeeping) failed", aErr)
	}
	res.bookkept = countNonEmptyLines(pruneOut)

	// Enumerate AFTER the bookkeeping prune so already-gone entries are not iterated.
	roots, lerr := guard.worktreeRoots(dir)
	if lerr != nil {
		return res, lerr
	}

	// Lock state, read ONCE up front. A locked worktree is one git will refuse to
	// deregister; deleting its directory first (before discovering the lock) destroys
	// state AND strands the registration — the exact half-destroyed outcome of issue #264.
	// So the lock is consulted FIRST, ahead of every content heuristic, and a locked
	// worktree is LEFT untouched with a lock reason. Fail CLOSED if lock state is
	// unreadable: an unknown lock state is treated as unsafe, never as "nothing is locked".
	locked, lkErr := guard.lockedWorktrees(dir)
	if lkErr != nil {
		return res, lkErr
	}

	for _, rt := range roots {
		// Lock gate FIRST: a locked worktree is never deleted, whatever its content state
		// says (a live agent's worktree was spared here only by luck of a content heuristic).
		if reason, isLocked := locked[rt]; isLocked {
			res.skips = append(res.skips, skipEntry{rt, lockedReason(reason)})
			continue
		}
		switch {
		case rt == guard.sharedCheckout:
			res.skips = append(res.skips, skipEntry{rt, "shared checkout (refused by identity — never removed)"})
			continue
		case rt == cwd:
			res.skips = append(res.skips, skipEntry{rt, "current worktree"})
			continue
		case !guard.allowed(rt):
			res.skips = append(res.skips, skipEntry{rt, "not under a sanctioned prefix"})
			continue
		}

		// Tracked-clean gate — identical to remove's (untracked build artifacts ignored).
		dirtyOut, derr := dirtyTracked(rt)
		if derr != nil {
			// Fail closed for THIS worktree: cannot verify → leave it, note it, keep going.
			res.skips = append(res.skips, skipEntry{rt, "unverifiable tracked status: " + derr.Error()})
			continue
		}
		if dirtyOut != "" {
			res.skips = append(res.skips, skipEntry{rt, "dirty (uncommitted tracked changes)"})
			continue
		}

		// Fresh-worktree guard (prune-only): a worktree whose HEAD is exactly at
		// origin/main tip (zero landed commits) may hold untracked new work. The
		// automatic sweep must not delete another session's uncommitted new source
		// files — dirtyTracked ignores untracked files deliberately (build artifacts
		// don't block), but in prune this creates a hole: a fresh worktree with
		// untracked new .go files is "tracked-clean AND merged" and would be removed.
		// Skip it; a genuine merge-commit/rebase landing has HEAD below the tip and
		// is still removed. remove (human-named single path) is not gated.
		atTip, tipErr := headAtOriginMainTip(rt)
		if tipErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "unverifiable origin/main position: " + tipErr.Error()})
			continue
		}
		if atTip {
			res.skips = append(res.skips, skipEntry{rt, "fresh worktree at origin/main (no landed commits — may hold untracked new work)"})
			continue
		}

		// Merge gate — the active-worker protection. HEAD must be an ancestor of origin/main.
		merged, mErr := mergedToOriginMain(rt)
		if mErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "unverifiable merge status: " + mErr.Error()})
			continue
		}
		if !merged {
			res.skips = append(res.skips, skipEntry{rt, unmergedReason(rt)})
			continue
		}

		// Proven safe: tracked-clean, NOT at origin/main tip, AND fully merged into origin/main. Remove via the exact
		// same primitive remove uses (recursive delete of the proven dir + prune + verify).
		if rmErr := removeWorktreeDir(guard, dir, rt); rmErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "removal failed: " + rmErr.Error()})
			continue
		}
		res.removed++
	}
	return res, nil
}

// lockedReason renders the skip reason for a locked worktree, surfacing git's lock message
// (e.g. `claude agent … pid 98225`) so the skip is visibly lock-based — the run makes a
// lock decision instead of silently masking it behind a content heuristic (issue #264).
func lockedReason(reason string) string {
	if reason == "" {
		return "locked"
	}
	return "locked (" + reason + ")"
}

// unmergedReason produces a human sub-diagnosis for a NOT-merged worktree, distinguishing
// unpushed-relative-to-upstream from plain unmerged (both are LEFT — this only refines the
// skip report; the removal gate is solely "merged into origin/main").
func unmergedReason(rt string) string {
	branch, berr := runGit(rt, "rev-parse", "--abbrev-ref", "HEAD")
	if berr != nil || branch == "HEAD" || branch == "" {
		return "unmerged (detached HEAD not an ancestor of origin/main)"
	}
	if _, uerr := runGit(rt, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); uerr == nil {
		if ahead, aerr := runGit(rt, "rev-list", "--count", "@{u}..HEAD"); aerr == nil && ahead != "0" {
			return "unpushed (" + ahead + " commit(s) ahead of upstream, not on origin/main)"
		}
	}
	return "unmerged (branch " + branch + " not an ancestor of origin/main — active work)"
}

// mergedToOriginMain reports whether the worktree's HEAD is an ancestor of the remote
// mainline. It shells `git merge-base --is-ancestor HEAD refs/remotes/origin/main`: exit 0
// = ancestor (merged → safe), exit 1 = not an ancestor (unmerged → active work, LEFT), any
// other exit or a resolution failure = Unverifiable so the caller fails CLOSED and leaves
// the worktree.
//
// The base is spelled FULLY QUALIFIED (`refs/remotes/origin/main`), not the bare short name
// `origin/main`, and this is load-bearing (issue #885, the ambiguity variant of #22). A
// checkout that has ever run `git branch origin/main` / `git worktree add ... -b origin/main`
// carries a stray LOCAL branch literally named `refs/heads/origin/main`; the short name is
// then ambiguous and `refs/heads/` wins gitrevisions disambiguation order, so bare `origin/main`
// silently resolves to that stale decoy while git only warns on stderr at exit 0. Resolving
// against a stale decoy would compute this gate against the wrong baseline. A fully-qualified
// remote-tracking ref is unambiguous by construction and cannot be shadowed by the decoy; if
// it does not resolve at all, that surfaces as Unverifiable (could-not-check) — never a
// silent fall-through to the decoy.
func mergedToOriginMain(rt string) (bool, error) {
	cmd := execCommand("git", "merge-base", "--is-ancestor", "HEAD", "refs/remotes/origin/main")
	cmd.Dir = rt
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, deskkit.Unverifiable("cannot determine merge status vs refs/remotes/origin/main (does it resolve?)", err)
}

// headAtOriginMainTip reports whether the worktree's HEAD is exactly at the remote mainline
// tip (zero landed commits). Fresh worktrees at the tip may hold untracked new work
// — `dirtyTracked` intentionally ignores untracked files so build artifacts (node_modules,
// build/, dist/) don't block, but in an automatic sweep this means a fresh worktree
// with new source files would be wrongly removed. This guard is prune-only: remove is a
// human-named single-path deletion where the caller affirms the path is safe.
//
// Like mergedToOriginMain, the tip is spelled FULLY QUALIFIED (`refs/remotes/origin/main`)
// rather than the ambiguous short name `origin/main` (issue #885): a stray local branch
// `refs/heads/origin/main` would otherwise shadow the real remote-tracking ref and this
// guard would compare HEAD against a stale decoy tip. A resolution failure surfaces as
// Unverifiable (could-not-check → the worktree is LEFT), never a silent decoy comparison.
func headAtOriginMainTip(rt string) (bool, error) {
	head, err := runGit(rt, "rev-parse", "HEAD")
	if err != nil {
		return false, deskkit.Unverifiable("cannot resolve HEAD", err)
	}
	originMain, err := runGit(rt, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return false, deskkit.Unverifiable("cannot resolve refs/remotes/origin/main", err)
	}
	return head == originMain, nil
}

// mustAbsOrRaw returns filepath.Abs(p) or, if that fails, p unchanged — resolvePath still
// applies the prefix/identity check to whatever comes back, so a non-absolute fallback
// can never widen the allowlist.
func mustAbsOrRaw(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func countNonEmptyLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }
