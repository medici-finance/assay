package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
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
	// singletonTTL is the prune singleton's RECENCY debounce: a sweep that completed less
	// than this ago holds a new one. 0 disables the debounce ONLY — the singleton's lock
	// is always taken and cannot be disabled (prunesingleton.go).
	singletonTTL time.Duration
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
	// --singleton-ttl is the RECENCY half of the prune singleton (prunesingleton.go): a
	// sweep that completed less than this ago holds a new one, so N desk windows booting
	// together run ONE sweep rather than N. 0 disables the debounce; the singleton's LOCK
	// is always taken and there is no flag that disables it.
	singletonTTLStr := fs.String("singleton-ttl", defaultSingletonTTL.String(),
		"skip the sweep when one completed less than this ago (e.g. 10m); 0 disables the recency check (the singleton lock is always taken)")
	noSingleton := fs.Bool("no-singleton", false,
		"disable the singleton's recency check (equivalent to --singleton-ttl 0); the lock is still taken, so a live sweep still holds this one")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		return deskkit.Refused("refused: prune takes no flags but --repo, --interval, " +
			"--reclaim-stale-locks, --lock-ttl, --dry-run, --singleton-ttl and --no-singleton " +
			"(there is no --force): " + perr.Error())
	}
	if len(positionals) != 0 {
		return deskkit.Refused("refused: prune takes no positional arguments")
	}

	opts := pruneOpts{reclaimStaleLocks: *reclaim, dryRun: *dryRun, singletonTTL: defaultSingletonTTL}
	if s := strings.TrimSpace(*singletonTTLStr); s != "" {
		d, derr := time.ParseDuration(s)
		if derr != nil {
			return deskkit.Refused("refused: --singleton-ttl is not a valid duration (e.g. 10m): " + *singletonTTLStr)
		}
		if d < 0 {
			return deskkit.Refused("refused: --singleton-ttl must not be negative: " + *singletonTTLStr)
		}
		opts.singletonTTL = d
	}
	if *noSingleton {
		opts.singletonTTL = 0
	}
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
		res, swept, why, serr := sweepWithSingleton(guard, dir, cwd, opts)
		if serr != nil {
			return serr
		}
		if !swept {
			// Another sweep is live, or one finished inside the TTL. This is a clean
			// no-op, not a failure: the work was done (or is being done) by the sweep
			// that holds the singleton.
			fmt.Fprintf(os.Stderr, "deskwt prune: %s — skipping this sweep\n", why)
			ac.detail = "singleton: " + why
			ac.successResult = deskkit.ResultNoop
			return nil
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

		// The singleton is taken and RELEASED per tick, never held for the supervisor's
		// lifetime: a long-lived `--interval` loop that kept the lock would lock out every
		// boot-time and manual sweep on the machine, turning an efficiency mechanism into
		// an outage.
		res, swept, why, err := sweepWithSingleton(guard, dir, cwd, opts)
		if err != nil {
			ac.detail = fmt.Sprintf("interval=%s: %d tick(s), %d removed, %d locks reclaimed, aborted on sweep error",
				interval, ticks, totalRemoved, totalReclaimed)
			return err
		}
		ticks++
		if !swept {
			fmt.Fprintf(os.Stdout, "%s deskwt prune tick %d: %s — skipped\n", nowStamp(), ticks, why)
		} else {
			totalRemoved += res.removed
			totalReclaimed += len(res.reclaimed)
			fmt.Fprintf(os.Stdout, "%s deskwt prune tick %d: bookkept=%d removed=%d held=%d locked_held=%d locks_reclaimed=%d\n",
				nowStamp(), ticks, res.bookkept, res.removed, len(res.skips), res.lockedHeld, len(res.reclaimed))
			renderSweepDetail(os.Stdout, res)
		}

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

// sweepWithSingleton runs ONE sweep under the prune singleton (prunesingleton.go). It
// returns the result, whether a sweep actually ran, and — when it did not — why.
//
// A --dry-run takes no singleton at all and always sweeps: a dry run writes nothing
// anywhere, including the stamp, so it can neither hold another sweep nor be held by the
// TTL. It is a read-only report, and a report that silently declined to look would be the
// opposite of one.
//
// A singleton that cannot be SET UP (no state directory, an unopenable stamp) does not
// stop the sweep: the singleton exists to deduplicate work, and an efficiency mechanism
// that fails closed on its own setup would wedge the boot step. The condition is reported
// as a sweep warning so it is never silent.
func sweepWithSingleton(guard *pathGuard, dir, cwd string, opts pruneOpts) (pruneResult, bool, string, error) {
	if opts.dryRun {
		res, err := pruneSweep(guard, dir, cwd, opts)
		return res, true, "", err
	}
	now := time.Now()
	s, outcome, why := acquirePruneSingleton(cwd, opts.singletonTTL, now)
	switch outcome {
	case singletonHeld, singletonRecent:
		return pruneResult{}, false, why, nil
	}
	res, err := pruneSweep(guard, dir, cwd, opts)
	if outcome == singletonUnavailable {
		res.warns = append(res.warns, why+" — sweeping anyway (the singleton deduplicates work; it never gates it)")
	}
	s.release(cwd, time.Now())
	return res, true, "", err
}

// sweepCtx is the per-sweep state that used to be rebuilt per candidate. Three things live
// here, and each replaces N copies of itself:
//
//   - objects: ONE go-git object cache for the whole sweep. gitcore.Open builds a fresh
//     one per call and prune called it 3-4 times per worktree, so one 5,779-commit history
//     was inflated out of the pack ~700 times in a single sweep.
//   - repos: one *gitcore.Repo per worktree, so each worktree is opened at most once.
//   - mainAncestors: the commits reachable from refs/remotes/origin/main, walked ONCE.
//     The per-candidate merge test is then a HEAD resolve plus a map lookup, replacing
//     go-git's unmemoized IsAncestor — which walks that history to exhaustion for every
//     candidate that is NOT merged, and 19 in 20 are not.
//
// This is the same fix internal/gitcore/contains.go documents at length for a different
// caller: one shared walk instead of N.
type sweepCtx struct {
	objects gitcore.ObjectCache
	repos   map[string]*gitcore.Repo

	// mainAncestors is nil when the set could not be built; ancestorErr then says why.
	mainAncestors map[string]struct{}
	ancestorErr   error
}

// newSweepCtx builds the per-sweep state, walking refs/remotes/origin/main exactly once
// from the SHARED checkout.
//
// Every linked worktree reads its refs and objects from the main checkout's common .git
// (which is what gitcore.Open's commondir routing exists to honour), so a set built once
// here is the same set each worktree would have built for itself.
//
// The base is spelled FULLY QUALIFIED for the reason mergedToOriginMain's comment gives at
// length (issue #885): refs/heads/ wins gitrevisions disambiguation, so a stray local
// branch literally named `origin/main` would silently become the baseline.
//
// A failure to build the set is RECORDED, never swallowed and never softened into an empty
// set. isMergedToOriginMain returns it to every candidate, so a sweep that could not
// establish the mainline holds everything and removes nothing.
func newSweepCtx(dir string) *sweepCtx {
	sc := &sweepCtx{
		objects: gitcore.NewObjectCache(),
		repos:   map[string]*gitcore.Repo{},
	}
	hashes, lerr := logOriginMain(sc, dir)
	if lerr != nil {
		sc.ancestorErr = lerr
		return sc
	}
	set := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		set[h] = struct{}{}
	}
	sc.mainAncestors = set
	return sc
}

// logOriginMain is THE walk: the one traversal of refs/remotes/origin/main a sweep makes,
// however many candidates it has.
//
// It is a package-level var so a test can COUNT it. That count is the machine-independent
// assertion behind the whole change — "origin/main was walked exactly once for 600
// candidates" holds on any machine, where a wall-clock assertion only holds on the one that
// ran it.
var logOriginMain = func(sc *sweepCtx, dir string) ([]string, error) {
	repo, err := sc.open(dir)
	if err != nil {
		return nil, err
	}
	return repo.Log("refs/remotes/origin/main")
}

// open returns the sweep's handle on the worktree rooted at rt, opening it at most once
// and routing every object read through the sweep's single shared cache.
func (sc *sweepCtx) open(rt string) (*gitcore.Repo, error) {
	if r, ok := sc.repos[rt]; ok {
		return r, nil
	}
	r, err := gitcore.OpenWith(rt, sc.objects)
	if err != nil {
		return nil, err
	}
	sc.repos[rt] = r
	return r, nil
}

// isMergedToOriginMain reports whether the worktree's HEAD is an ancestor of the remote
// mainline, answering from the per-sweep ancestor set rather than by walking.
//
// It is EXACT, not an approximation: `IsAncestor(HEAD, origin/main)` is true precisely when
// HEAD is one of the commits reachable from origin/main, and Log returns that set including
// origin/main itself.
//
// It fails CLOSED in both directions. A sweep-level failure to build the set makes every
// candidate unverifiable — never "not merged" (which is safe but silently degrades the skip
// reasons) and above all never "merged". A per-worktree failure to resolve HEAD is likewise
// Unverifiable, exactly as the shelled form was.
func (sc *sweepCtx) isMergedToOriginMain(rt string) (bool, error) {
	if sc.mainAncestors == nil {
		return false, deskkit.Unverifiable(
			"cannot determine merge status vs refs/remotes/origin/main for this sweep (the mainline history could not be read)",
			sc.ancestorErr)
	}
	repo, err := sc.open(rt)
	if err != nil {
		return false, deskkit.Unverifiable("cannot determine merge status vs refs/remotes/origin/main (does it resolve?)", err)
	}
	head, err := repo.Resolve("HEAD")
	if err != nil {
		return false, deskkit.Unverifiable("cannot determine merge status vs refs/remotes/origin/main (does it resolve?)", err)
	}
	_, ok := sc.mainAncestors[head.String()]
	return ok, nil
}

// pruneSweep performs ONE prune sweep against the repo rooted at dir (Step A bookkeeping,
// the opt-in Step A2 lock reclaim, then Step B safe removals) and returns the counts. It
// prints nothing — the caller renders the one-shot or per-tick summary. cwd is the resolved
// current directory (never removed).
func pruneSweep(guard *pathGuard, dir, cwd string, opts pruneOpts) (pruneResult, error) {
	var res pruneResult

	// Step A — bookkeeping prune (always safe: only drops entries for dirs already gone).
	// --verbose prints one line per dropped entry so we can report the count.
	//
	// Under --dry-run this is git's OWN --dry-run: it prints the same per-entry report on
	// stderr and drops nothing. Before this, Step A ran unconditionally and the dry-run
	// check sat a hundred lines further down inside the candidate loop, so `prune
	// --dry-run` mutated .git/worktrees/ before it reported anything.
	dropped, aErr := bookkeepingPrune(dir, opts.dryRun)
	if aErr != nil {
		return res, deskkit.Unverifiable("git worktree prune (bookkeeping) failed", aErr)
	}
	res.bookkept = dropped

	// Step A2 — OPT-IN lock reclaim. It only UNLOCKS; nothing below this point is relaxed,
	// so a reclaimed worktree still has to pass every removal gate on its own merits. A
	// second bookkeeping prune follows any reclaim because Step A cannot drop the dangling
	// admin entry of a LOCKED worktree — those entries are exactly the ones a permanent lock
	// makes immortal, and they only become droppable once the lock is gone.
	//
	// Under --dry-run this pass REPORTS what it would unlock and unlocks nothing: a dry run
	// that silently retired locks would be a worse surprise than one that silently pruned
	// bookkeeping, and both are now off the dry-run path.
	if opts.reclaimStaleLocks {
		reclaimed, warns, rerr := reclaimStaleLocksMode(guard, dir, cwd, opts.lockTTL, opts.dryRun)
		res.reclaimed = reclaimed
		res.warns = warns
		if rerr != nil {
			return res, rerr
		}
		if len(reclaimed) > 0 && !opts.dryRun {
			second, sErr := bookkeepingPrune(dir, false)
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

	// Per-sweep state: ONE object cache, one repo handle per worktree, and ONE walk of
	// refs/remotes/origin/main. See sweepCtx.
	sc := newSweepCtx(dir)
	if sc.mainAncestors == nil {
		res.warns = append(res.warns,
			"could-not-check: refs/remotes/origin/main history unreadable, so NO worktree can be proven merged — every candidate is held and nothing is removed: "+
				errText(sc.ancestorErr))
	}

	// pendingRemoved collects the trees deleted in this loop. Their admin entries are
	// dropped ONCE after the loop (see the batched deregistration below), not per removal.
	var pendingRemoved []string

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

		// GATE ORDER. The two ancestry gates run FIRST and the tracked-clean gate last.
		//
		// Every gate below is the one it always was and holds exactly what it always held;
		// only the order changed. The two ancestry gates are now a hash comparison and a
		// map lookup, while the tracked-clean gate is a full go-git Status() over the
		// worktree (~4,100 files on the repo in issue #1037) — and ~95% of candidates are
		// held by an ancestry gate, so running Status() first spent ~25% of the sweep
		// computing an answer that was then discarded.
		//
		// One observable consequence, stated because it is real: a worktree that would fail
		// MORE THAN ONE gate is now reported with the reason of whichever gate comes first
		// in this order. A worktree that is both dirty and unmerged used to be reported
		// "dirty" and is now reported "unmerged". Both are holds; nothing that was removed
		// is removed differently, and nothing that was held stops being held.

		// Fresh-worktree guard (prune-only): a worktree whose HEAD is exactly at
		// origin/main tip (zero landed commits) may hold untracked new work. The
		// automatic sweep must not delete another session's uncommitted new source
		// files — dirtyTracked ignores untracked files deliberately (build artifacts
		// don't block), but in prune this creates a hole: a fresh worktree with
		// untracked new .go files is "tracked-clean AND merged" and would be removed.
		// Skip it; a genuine merge-commit/rebase landing has HEAD below the tip and
		// is still removed. remove (human-named single path) is not gated.
		atTip, tipErr := headAtOriginMainTip(sc, rt)
		if tipErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "unverifiable origin/main position: " + tipErr.Error()})
			continue
		}
		if atTip {
			res.skips = append(res.skips, skipEntry{rt, "fresh worktree at origin/main (no landed commits — may hold untracked new work)"})
			continue
		}

		// Merge gate — the active-worker protection. HEAD must be an ancestor of origin/main.
		merged, mErr := sc.isMergedToOriginMain(rt)
		if mErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "unverifiable merge status: " + mErr.Error()})
			continue
		}
		if !merged {
			res.skips = append(res.skips, skipEntry{rt, unmergedReason(sc, rt)})
			continue
		}

		// Tracked-clean gate — identical to remove's (untracked build artifacts ignored).
		dirtyOut, derr := dirtyTrackedIn(sc, rt)
		if derr != nil {
			// Fail closed for THIS worktree: cannot verify → leave it, note it, keep going.
			res.skips = append(res.skips, skipEntry{rt, "unverifiable tracked status: " + derr.Error()})
			continue
		}
		if dirtyOut != "" {
			res.skips = append(res.skips, skipEntry{rt, "dirty (uncommitted tracked changes)"})
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

		// Delete the tree only. The admin entry is dropped, and the deregistration
		// POSITIVELY verified, once for the whole sweep below — `git worktree prune` and
		// `git worktree list --porcelain` are both repo-wide, and running them per removal
		// is quadratic in the worktree count (~1 s per listing at ~675 worktrees). The
		// verification is batched, never dropped.
		//
		// A worktree closes its own repo handle's reference here: the sweep may hold an
		// open *gitcore.Repo on a tree it is about to delete, and nothing reads it again.
		delete(sc.repos, rt)
		if rmErr := removeWorktreeTree(rt); rmErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "removal failed: " + rmErr.Error()})
			continue
		}
		pendingRemoved = append(pendingRemoved, rt)
	}

	// ONE deregistration pass for every tree deleted above. A path still registered after
	// it is DEMOTED — it does not count as removed, and it is named in a warning — so the
	// batching changes how many git invocations run, never the per-path verdict.
	if len(pendingRemoved) > 0 {
		stranded, sErr := strandedAfterDeregister(guard, dir, pendingRemoved)
		if sErr != nil {
			// The deregistration itself could not be run or read. Every deleted tree is
			// therefore UNVERIFIED: report them as such rather than counting them removed.
			for _, rt := range pendingRemoved {
				res.skips = append(res.skips, skipEntry{rt, "removed from disk but deregistration unverifiable: " + sErr.Error()})
			}
			return res, nil
		}
		strandedSet := make(map[string]bool, len(stranded))
		for _, rt := range stranded {
			strandedSet[rt] = true
			res.warns = append(res.warns,
				"worktree removed from disk but "+rt+" is still registered after prune (not counted as removed)")
			res.skips = append(res.skips, skipEntry{rt, "removed from disk but still registered after prune"})
		}
		for _, rt := range pendingRemoved {
			if !strandedSet[rt] {
				res.removed++
			}
		}
	}
	return res, nil
}

// dirtyTrackedIn runs the tracked-clean gate against the sweep's own handle on rt, so the
// sweep never opens a worktree twice. It is the SAME check `remove` runs — see
// dirtyTrackedFrom, which both reach.
//
// It is a package-level var so a test can count how many times the gate actually ran. That
// count is the only way to assert the gate REORDER did what it claims: a test that measured
// wall time would pass on a fast machine whatever the order, whereas "Status() was never
// computed for the worktree the merge gate held" is the behaviour itself.
var dirtyTrackedIn = func(sc *sweepCtx, rt string) (string, error) {
	repo, err := sc.open(rt)
	if err != nil {
		return "", deskkit.Unverifiable("cannot check the worktree's tracked status", err)
	}
	return dirtyTrackedFrom(repo)
}

// errText renders an error for a report line, tolerating nil.
func errText(err error) string {
	if err == nil {
		return "(no cause recorded)"
	}
	return err.Error()
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
// It is called ONLY for worktrees the merge gate has already held, so it can never
// influence a removal — which is what made its former cost indefensible. It used to call
// gitcore.AheadCount, which walks the ENTIRE base history into a map and then walks head,
// purely to render the number in "unpushed (N commit(s) ahead ...)". No gate, no test and
// no caller ever parsed that number back out, and the two walks were ~36% of the whole
// sweep (issue #1037).
//
// What replaces it is a HASH COMPARISON: resolve the upstream ref and HEAD (two ref reads,
// no history walk) and ask whether they are the same commit. The COUNT is gone; the
// unpushed-vs-unmerged DISTINCTION is not, because two tests pin it — one asserting the
// word "unpushed" appears for a worktree ahead of its upstream, one asserting as a NEGATIVE
// control that a clean worktree pushed to its own upstream is NOT labelled unpushed.
//
// The wording changed with the measurement. "N commit(s) ahead of upstream" asserted a
// DIRECTION this comparison does not establish; "HEAD is not at its upstream" is what two
// hashes differing actually proves.
func unmergedReason(sc *sweepCtx, rt string) string {
	repo, rerr := sc.open(rt)
	if rerr != nil {
		return "unmerged (detached HEAD not an ancestor of origin/main)"
	}
	branch, berr := repo.AbbrevRefHEAD()
	if berr != nil || branch == "HEAD" || branch == "" {
		return "unmerged (detached HEAD not an ancestor of origin/main)"
	}
	if upstream, uerr := repo.UpstreamRef(); uerr == nil {
		upHash, uherr := repo.Resolve(upstream)
		headHash, hherr := repo.Resolve("HEAD")
		if uherr == nil && hherr == nil && upHash != headHash {
			return "unpushed (HEAD is not at its upstream, and not on origin/main)"
		}
	}
	return "unmerged (branch " + branch + " not an ancestor of origin/main — active work)"
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
//
// DO NOT hoist this resolve onto the sweep's ancestor walk. It looks like free saving — the
// tip cannot change inside a sweep, so N ref reads become one — and it was tried and
// reverted. This guard and the merge gate are deliberately INDEPENDENT layers over the same
// worktree: the merge gate asks an ancestry question answered from the walked set, and this
// one asks a ref-identity question answered from the worktree's own refs. Feeding both from
// one walk makes a single failure disable both, and it is measurable: with the tip taken
// from the walk, a mutation that removes the merge gate's fail-closed guard SURVIVES the
// suite, because this guard is already holding everything for the same reason. Two layers
// that fail on the same signal are one layer. The reads are two ref lookups per candidate,
// which is not where the time goes — the walks were.
func headAtOriginMainTip(sc *sweepCtx, rt string) (bool, error) {
	repo, err := sc.open(rt)
	if err != nil {
		return false, deskkit.Unverifiable("cannot resolve HEAD", err)
	}
	headHash, err := repo.Resolve("HEAD")
	if err != nil {
		return false, deskkit.Unverifiable("cannot resolve HEAD", err)
	}
	head := headHash.String()
	originMainHash, err := repo.Resolve("refs/remotes/origin/main")
	if err != nil {
		return false, deskkit.Unverifiable("cannot resolve refs/remotes/origin/main", err)
	}
	return head == originMainHash.String(), nil
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
func bookkeepingPrune(dir string, dryRun bool) (int, error) {
	argv := []string{"worktree", "prune", "--verbose"}
	if dryRun {
		// git's OWN dry run: it reports every entry it would drop, on the same stream,
		// and drops none. This is what makes `deskwt prune --dry-run` read-only without
		// reimplementing the entry-danglingness test.
		argv = append(argv, "--dry-run")
	}
	stdout, stderr, err := runGitStreams(dir, argv...)
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
