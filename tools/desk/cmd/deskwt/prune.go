package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
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
	bookkept   int // admin entries dropped by `git worktree prune` (dirs already gone)
	removed    int // merged+clean worktrees removed
	lockedHeld int // the subset of skips held by the lock gate
	skips      []skipEntry
	reclaimed  []reclaimEntry // locks retired this sweep (only with --reclaim-stale-locks)
	warns      []string       // things the sweep could not do, said out loud
}

// pruneOpts carries the sweep's opt-in behaviour. The zero value is the historical sweep:
// bookkeeping prune + safe removals, every lock left exactly where it is.
type pruneOpts struct {
	// reclaimStaleLocks turns on the lock-lifecycle pass (lockreclaim.go): unlock locks
	// proven stale so the ORDINARY eligibility rules can then apply to those worktrees.
	reclaimStaleLocks bool
	// lockTTL is the age fallback for locks that name no session. 0 disables it.
	lockTTL time.Duration
	// dryRun reports what a sweep WOULD remove (and the before_remove hook plan) without
	// deleting anything. It is one-shot only (refused with --interval).
	dryRun bool
}

// cmdPrune implements `deskwt prune [--repo <path>] [--interval <dur>]
// [--reclaim-stale-locks [--lock-ttl <dur>]]`: bounded
// worktree-count reduction so stale worktrees can never accumulate into the E2BIG sandbox
// failure or the #742 writeguard false-positives (both driven by worktree sprawl).
//
// One sweep runs two steps:
//
//	Step A (always, safe bookkeeping): `git worktree prune --verbose` drops admin entries
//	  whose working directories are already gone. This changes no on-disk working tree.
//
//	Step A2 (OPT-IN, --reclaim-stale-locks): give worktree locks a LIFECYCLE. Nothing else
//	  ever unlocks a worktree, so a lock taken by a session that has since died is permanent
//	  and the locked population grows without bound — and a lock even blocks Step A from
//	  dropping the admin entry of a worktree whose directory is already gone. This pass
//	  UNLOCKS (never removes) the locks it can prove stale — the locking session is gone per
//	  the roster beacons, or the lock is older than --lock-ttl — and then re-runs Step A so
//	  newly-unlockable dangling entries are dropped too. Every unlock prints the worktree,
//	  the lock reason, and the evidence. See lockreclaim.go.
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
	// --reclaim-stale-locks is the lock LIFECYCLE opt-in. Default OFF: a sweep that was not
	// asked to reclaim behaves exactly as it always has. It only ever UNLOCKS — every removal
	// gate below still runs, unchanged, on the unlocked worktree.
	reclaim := fs.Bool("reclaim-stale-locks", false,
		"unlock worktree locks PROVEN stale (locking session gone, or older than --lock-ttl) so the normal prune rules can apply; default off")
	// --lock-ttl is the age fallback for locks that name no session. Default 0 = disabled,
	// because "old" is not by itself evidence that a session is gone.
	lockTTLStr := fs.String("lock-ttl", "0",
		"with --reclaim-stale-locks: also treat any lock older than this (e.g. 24h) as stale; 0 disables the age test")
	dryRun := fs.Bool("dry-run", false, "report what a sweep would remove and the before_remove hook plan; delete nothing (one-shot only)")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("refused: prune takes no flags but --repo, --interval, " +
			"--reclaim-stale-locks, --lock-ttl and --dry-run (there is no --force): " + perr.Error())
	}
	if len(positionals) != 0 {
		return deskkit.Refused("refused: prune takes no positional arguments")
	}

	opts := pruneOpts{reclaimStaleLocks: *reclaim, dryRun: *dryRun}
	if s := strings.TrimSpace(*lockTTLStr); s != "" && s != "0" {
		d, derr := time.ParseDuration(s)
		if derr != nil {
			return deskkit.Refused("refused: --lock-ttl is not a valid duration (e.g. 24h): " + *lockTTLStr)
		}
		if d < 0 {
			return deskkit.Refused("refused: --lock-ttl must not be negative: " + *lockTTLStr)
		}
		opts.lockTTL = d
	}
	// A TTL without the opt-in would be silently inert — the exact shape of failure this
	// tool refuses everywhere else. Say so instead of accepting a knob that does nothing.
	if opts.lockTTL > 0 && !opts.reclaimStaleLocks {
		return deskkit.Refused("refused: --lock-ttl has no effect without --reclaim-stale-locks")
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
	// A dry run of a forever-loop is meaningless — refuse the pairing rather than silently
	// looping a read-only sweep.
	if opts.dryRun && interval > 0 {
		return deskkit.Refused("refused: --dry-run is one-shot only; it cannot be combined with --interval")
	}
	if opts.dryRun {
		line, herr := deskkit.HookDryRunLine(deskkit.HookBeforeRemove)
		if herr != nil {
			return herr
		}
		fmt.Fprintln(os.Stderr, line)
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

	// One-shot mode: single sweep, detailed per-worktree summary to stderr.
	if interval == 0 {
		res, serr := pruneSweep(guard, dir, cwd, opts)
		if serr != nil {
			return serr
		}
		fmt.Fprintln(os.Stderr, pruneSummaryLine(res))
		renderSweepDetail(os.Stderr, res)
		for _, s := range res.skips {
			fmt.Fprintf(os.Stderr, "  skipped %s — %s\n", s.path, s.reason)
		}
		ac.detail = pruneAuditDetail(res)
		if res.bookkept == 0 && res.removed == 0 && len(res.reclaimed) == 0 {
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
	return runPruneLoop(guard, dir, cwd, interval, opts, stop, ac)
}

// pruneSummaryLine is the ONE line a sweep always emits. It reports the four counts a
// caller needs to tell a drained repo from a stuck one: what bookkeeping was dropped, what
// was removed, what was HELD (and how much of that hold is the lock gate specifically — the
// number that used to grow without bound), and how many locks were retired.
func pruneSummaryLine(res pruneResult) string {
	return fmt.Sprintf(
		"deskwt prune: pruned %d bookkeeping entr%s, removed %d merged+clean worktree%s, held %d (locked-held %d), locks-reclaimed %d",
		res.bookkept, plural(res.bookkept, "y", "ies"),
		res.removed, plural(res.removed, "", "s"),
		len(res.skips), res.lockedHeld, len(res.reclaimed))
}

// pruneAuditDetail is the same four counts in the audit line's detail field.
func pruneAuditDetail(res pruneResult) string {
	return fmt.Sprintf("pruned %d bookkeeping, removed %d, held %d (locked-held %d), locks-reclaimed %d",
		res.bookkept, res.removed, len(res.skips), res.lockedHeld, len(res.reclaimed))
}

// renderSweepDetail writes the lines that must never be reduced to a count: every lock this
// sweep retired (worktree, lock reason, and the evidence that judged it stale) and everything
// the sweep could not do. Both are rare and both change on-disk state or explain why it did
// not change, so both print in one-shot AND in interval mode.
func renderSweepDetail(w io.Writer, res pruneResult) {
	for _, r := range res.reclaimed {
		fmt.Fprintf(w, "  reclaimed lock on %s — reason %s — stale: %s\n", r.path, lockReasonText(r.reason), r.why)
	}
	for _, warn := range res.warns {
		fmt.Fprintf(w, "  warning: %s\n", warn)
	}
}

// runPruneLoop is the ticking body factored out for testability: it sweeps immediately,
// then every `interval` until `stop` is signalled (clean nil → exit 0) or the kill
// switch / STOP flag fires between ticks (Guard's Disabled error → exit 3). It never
// sleeps a partial tick past a stop — the select blocks on both channels. Each tick emits
// one stdout summary line; a sweep error aborts the loop (fail closed).
func runPruneLoop(guard *pathGuard, dir, cwd string, interval time.Duration, opts pruneOpts, stop <-chan struct{}, ac *auditCtx) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ticks := 0
	totalRemoved := 0
	totalReclaimed := 0
	for {
		// Re-check the kill switch / STOP flags at every iteration boundary:
		// a STOP/DISABLED armed mid-loop halts on the next tick with a clean exit-3 audit.
		if gErr := deskkit.Guard(); gErr != nil {
			fmt.Fprintf(os.Stdout, "%s deskwt prune: halting loop — %s\n", nowStamp(), gErr.Error())
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, %d locks reclaimed, halted (%s)",
				interval, ticks, totalRemoved, totalReclaimed, gErr.Error())
			return gErr
		}

		res, err := pruneSweep(guard, dir, cwd, opts)
		if err != nil {
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, %d locks reclaimed, aborted on sweep error",
				interval, ticks, totalRemoved, totalReclaimed)
			return err
		}
		ticks++
		totalRemoved += res.removed
		totalReclaimed += len(res.reclaimed)
		fmt.Fprintf(os.Stdout, "%s deskwt prune tick %d: bookkept=%d removed=%d held=%d locked_held=%d locks_reclaimed=%d\n",
			nowStamp(), ticks, res.bookkept, res.removed, len(res.skips), res.lockedHeld, len(res.reclaimed))
		renderSweepDetail(os.Stdout, res)

		select {
		case <-stop:
			fmt.Fprintf(os.Stdout, "%s deskwt prune: shutdown signal — exiting after %d tick(s)\n", nowStamp(), ticks)
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, %d locks reclaimed, stopped by signal",
				interval, ticks, totalRemoved, totalReclaimed)
			return nil
		case <-ticker.C:
			continue
		}
	}
}

// pruneSweep performs ONE prune sweep against the repo rooted at dir (Step A bookkeeping,
// the opt-in Step A2 lock reclaim, then Step B safe removals) and returns the counts. It
// prints nothing — the caller renders the one-shot or per-tick summary. cwd is the resolved
// current directory (never removed).
func pruneSweep(guard *pathGuard, dir, cwd string, opts pruneOpts) (pruneResult, error) {
	var res pruneResult

	// Step A — bookkeeping prune (always safe: only drops entries for dirs already gone).
	// --verbose prints one line per dropped entry so we can report the count.
	dropped, aErr := bookkeepingPrune(dir)
	if aErr != nil {
		return res, deskkit.Unverifiable("git worktree prune (bookkeeping) failed", aErr)
	}
	res.bookkept = dropped

	// Step A2 — OPT-IN lock reclaim. It only UNLOCKS; nothing below this point is relaxed,
	// so a reclaimed worktree still has to pass every removal gate on its own merits. A
	// second bookkeeping prune follows any reclaim because Step A cannot drop the dangling
	// admin entry of a LOCKED worktree — those entries are exactly the ones a permanent lock
	// makes immortal, and they only become droppable once the lock is gone.
	if opts.reclaimStaleLocks {
		reclaimed, warns, rerr := reclaimStaleLocks(guard, dir, cwd, opts.lockTTL)
		res.reclaimed = reclaimed
		res.warns = warns
		if rerr != nil {
			return res, rerr
		}
		if len(reclaimed) > 0 {
			second, sErr := bookkeepingPrune(dir)
			if sErr != nil {
				return res, deskkit.Unverifiable("git worktree prune (bookkeeping, after lock reclaim) failed", sErr)
			}
			res.bookkept += second
		}
	}

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
			res.lockedHeld++
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

		// Proven safe: tracked-clean, NOT at origin/main tip, AND fully merged into origin/main.
		// --dry-run stops here: count it as a would-remove, delete nothing.
		if opts.dryRun {
			res.removed++
			continue
		}

		// before_remove — runs before each deletion. LOGGED failure class: a hook failure is
		// reported but the removal PROCEEDS (a cleanup hook must never wedge the sweep).
		if _, herr := deskkit.RunHook(deskkit.HookBeforeRemove, deskkit.HookEnv{
			RunKey: filepath.Base(rt), Worktree: rt, Repo: repoOrEmpty(dir),
		}); herr != nil {
			res.warns = append(res.warns, "before_remove hook failed for "+rt+" (removal proceeds): "+herr.Error())
		}

		// Remove via the exact same primitive remove uses (recursive delete of the proven dir
		// + prune + verify).
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

// bookkeepingPrune runs the always-safe Step A — `git worktree prune --verbose`, which drops
// admin entries whose working directories are already gone and touches no working tree — and
// returns how many entries it dropped.
//
// The count is read from git's stderr as well as its stdout, and that is the whole point of
// the helper: `git worktree prune --verbose` prints its per-entry report on STDERR, so a
// caller reading stdout alone counts zero however many entries it just dropped. The summary
// line is what tells an operator whether a sweep is draining the repo or spinning, so a
// structurally-always-zero count is worse than no count at all.
func bookkeepingPrune(dir string) (int, error) {
	stdout, stderr, err := runGitStreams(dir, "worktree", "prune", "--verbose")
	if err != nil {
		return 0, err
	}
	return countNonEmptyLines(stdout) + countNonEmptyLines(stderr), nil
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
