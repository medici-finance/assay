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

// freshWorktreeSafetyWindow is the minimum admin-entry age a pristine at-tip worktree
// must reach before a sweep may remove it. `deskwt add` never locks the tree it creates,
// so a just-created pristine at-tip tree (the verifier/reviewer shape) is
// content-identical to reclaim residue — only age tells them apart. The window is the
// roster's session-freshness window: one definition of "a session could still be setting
// up" governs both.
const freshWorktreeSafetyWindow = beaconFreshWindow

// pruneResult is the outcome of one sweep.
type pruneResult struct {
	bookkept   int // admin entries dropped by `git worktree prune` (dirs already gone)
	removed    int // merged+clean worktrees removed
	lockedHeld int // the subset of skips held by the lock gate
	skips      []skipEntry
	reclaimed  []reclaimEntry // locks retired this sweep (only with --reclaim-stale-locks)
	warns      []string       // things the sweep could not do, said out loud

	// staleRegs are the registrations the STALE-REGISTRATION pass cleared (stalereg.go):
	// worktree directory present but its .git gitdir file gone. Clearing deletes the
	// admin dir (.git/worktrees/<id>/), so it happens only behind the pass's three gates:
	// the scope gates every arm enforces, an admin-HEAD proof that its target remains retained
	// (a positively resolving branch, or on the remote mainline), and — for a locked husk — a lock the
	// reclaim pass retired THIS sweep. The directory is always left on disk, untouched.
	staleRegs []staleRegEntry

	// reaped counts the worktrees the DEAD-SESSION arm removed (deadsession.go). It is
	// kept apart from `removed` because the two arms cleared different populations under
	// different gates, and one number that merged them could not tell an operator whether
	// the leftover backlog is draining at all.
	reaped int
	// branches are the stale local branches deleted alongside those worktrees.
	branches []string
	// plan is the reap arm's judgement for every worktree it considered, REAP and KEEP
	// alike — what `--dry-run` prints in full.
	plan []reapVerdict
	// branchGC is the LOCAL-BRANCH GC half of the sweep (branchgc.go, Step C): the local
	// refs the worktree lifecycle leaves behind, collected ONLY when provably on the
	// remote mainline. branchGC.ran is false when --no-branches skipped the pass.
	branchGC branchGCResult
	// cache is the CELL-CACHE pressure pass (cachepressure.go, Step D): a pressure check
	// per marked cell Go-cache root under the cells root, plus the reap of a root whose
	// owning directory this sweep removed. cache.ran is false when --no-cache (or
	// --branches-only) skipped the pass.
	cache cacheResult
	// dryRun records which mode produced this result, so the renderer prints the FULL plan
	// for an inspection run and only the acted-on rows for a live one.
	dryRun bool
}

// pruneOpts carries the sweep's opt-in behaviour. The zero value is the historical sweep:
// bookkeeping prune + safe removals, every lock left exactly where it is.
type pruneOpts struct {
	// reclaimStaleLocks turns on the lock-lifecycle pass (lockreclaim.go): unlock locks
	// proven stale so the ORDINARY eligibility rules can then apply to those worktrees.
	reclaimStaleLocks bool
	// reapDeadSessions turns on the DEAD-SESSION arm (deadsession.go): remove a worktree
	// no live session owns, gated on clean-AND-pushed rather than on the mainline merge
	// the ordinary arm requires. Default OFF, exactly like reclaimStaleLocks: a sweep that
	// was not asked to reap behaves precisely as it always has.
	reapDeadSessions bool
	// lockTTL is the age fallback for locks that name no session. 0 disables it.
	lockTTL time.Duration
	// dryRun reports what a sweep WOULD remove (and the before_remove hook plan) without
	// deleting anything. It is one-shot only (refused with --interval).
	dryRun bool
	// singletonTTL is the prune singleton's RECENCY debounce: a sweep that completed less
	// than this ago holds a new one. 0 disables the debounce ONLY — the singleton's lock
	// is always taken and cannot be disabled (prunesingleton.go).
	singletonTTL time.Duration
	// noBranches skips the local-branch GC (Step C, branchgc.go) for this sweep — the
	// cadence knob for a boot path that wants the worktree arms more often than the
	// branch pass. Default OFF: the GC runs at the same moments as prune.
	noBranches bool
	// branchesOnly runs ONLY the local-branch GC, skipping every worktree arm — the other
	// half of the cadence split. The opt-in worktree arms (--reclaim-stale-locks,
	// --reap-dead-sessions) are REFUSED with it, because they would be silently inert.
	branchesOnly bool
	// noCache skips the cell-cache pressure pass (Step D, cachepressure.go) for this
	// sweep. Default OFF: every sweep pressure-checks the marked cell Go-cache roots.
	noCache bool
	// cellsRoot is the cells root Step D sweeps, already resolved from --cells-root /
	// $CELLS_ROOT / the compiled default by cmdPrune. "" skips the pass.
	cellsRoot string
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
//	Step A1 (always, safe bookkeeping): the STALE-REGISTRATION sweep (stalereg.go) handles
//	  the admin entries Step A cannot: worktree directory still present but its .git
//	  gitdir file gone — a shape `git worktree remove` refuses ("validation failed") and
//	  a lock makes `git worktree prune` keep forever. It runs the same scope gates as
//	  every other arm (never the shared checkout, never the current worktree, never
//	  outside the sanctioned prefixes), and clears a registration only when its admin
//	  HEAD target is retained (a positively resolving branch, or a detached commit already
//	  on refs/remotes/origin/main) AND it is not locked — the default sweep never
//	  retires a lock; a locked husk clears only when --reclaim-stale-locks has proven
//	  the lock stale this sweep. The directory is always left on disk, untouched. Each
//	  cleared entry prints its own line, and the summary gains a
//	  stale-registrations-cleared segment when the pass acted.
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
//	Step A3 (OPT-IN, --reap-dead-sessions): the DEAD-SESSION arm. Step A2 retires a lock and
//	  then hands the worktree to Step B's rules, which require HEAD to be merged into the
//	  remote mainline — so a dead session's OPEN-PR worktree (an unmerged branch, by
//	  definition) is held by Step B however certainly its session is gone, and a leftover
//	  population accumulates that no sweep can ever clear. This arm judges those trees by a
//	  different, narrower question: does any LIVE session still own this worktree, and is
//	  removing it provably lossless (clean INCLUDING untracked files, and HEAD already
//	  reachable from its upstream or from the mainline)? It removes the ones that pass and
//	  deletes their stale local branch, so the next `deskwt add --branch` cuts fresh from
//	  origin instead of colliding. A LIVE session's lock still holds its worktree
//	  unconditionally. See deadsession.go.
//
//	Step B (count reduction, safe gate): walk the registered worktrees under the sanctioned
//	  prefixes and REMOVE (via the exact same safe-remove primitive as `remove`) ONLY the
//	  ones proven safe — NOT a LOCKED worktree (git will refuse to deregister it, so deleting
//	  its directory first would strand the registration — issue #264; the lock is checked
//	  FIRST, ahead of every content heuristic), NOT the shared checkout (identity), NOT the current worktree (cwd),
//	  tracked-clean, AND fully merged into origin/main (HEAD an ancestor of origin/main → every
//	  commit reachable from HEAD is already on the remote mainline, so nothing is lost). The
//	  merge check is the ACTIVE-WORKER guard: an unmerged branch (an open PR still in flight)
//	  is NOT an ancestor of origin/main, so it is LEFT untouched. A worktree whose HEAD IS
//	  the origin/main tip (zero landed commits — the fresh-worktree case, and the shape a
//	  reclaim-by-detach leaves behind) must additionally prove it holds NOTHING the
//	  mainline does not: the untracked-INCLUSIVE status gate runs for exactly these, so a
//	  pristine detached-at-main tree is removed while one holding uncommitted or untracked
//	  work is LEFT. Anything failing any check is
//	  LEFT and reported as skipped with a reason. There is NO --force anywhere.
//
//	Step C (default ON, --no-branches to skip / --branches-only to run alone): the
//	  LOCAL-BRANCH GC (branchgc.go). Worktrees have this sweep and remote branches have the
//	  forge's delete-on-merge, but the LOCAL branch a worktree leaves behind has neither —
//	  they accumulate without bound. Step C collects a local branch ONLY when its content is
//	  provably on refs/remotes/origin/main: its tip is an ancestor of the mainline (which
//	  covers the fresh-never-used handle exactly), or every unique commit's patch-id is
//	  already in the mainline's history (the squash-merge case). main/master, any branch
//	  checked out in any worktree, and any branch whose proof fails or cannot be completed
//	  are KEPT — unique-patch branches always. The delete is a compare-and-delete against
//	  the proven sha (`git update-ref -d <ref> <sha>`), never a force verb.
//
//	Step D (default ON, --no-cache to skip): the CELL-CACHE pressure pass
//	  (cachepressure.go). A cell's managed Go cache (tools/desk/internal/cellcache) is
//	  otherwise pressure-checked only at launch admission and dispatch, so a long-lived
//	  cell can sit over its byte budget — or the filesystem under its free-space floor —
//	  indefinitely. Every sweep runs cellcache.Check over each marked cache root under
//	  the cells root (--cells-root, then $CELLS_ROOT, then the compiled default), with
//	  the package's own semantics: reclaim only under pressure, never while a consumer
//	  holds a lease, never a warm purge. A root whose owning directory this sweep just
//	  removed is reaped as an orphan, gated on its lease set re-read under its own lock
//	  at act time. The pass is SILENT when every cache is within policy; the summary
//	  line gains a cache segment only when it reclaimed, reaped, or found pressure it
//	  could not clear.
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
		"with --reclaim-stale-locks or --reap-dead-sessions: also treat any lock older than this (e.g. 24h) as stale; 0 disables the age test")
	// --reap-dead-sessions is the DEAD-SESSION arm's opt-in. Default OFF for the same
	// reason --reclaim-stale-locks is: it is a new destructive capability, and the operator
	// arms it deliberately after reading a --dry-run plan.
	reapDead := fs.Bool("reap-dead-sessions", false,
		"remove a worktree no live session owns (dead-session lock, or no lock at all) when it is clean AND fully pushed, deleting its stale local branch too; default off")
	dryRun := fs.Bool("dry-run", false, "report what a sweep would remove and the before_remove hook plan; delete nothing (one-shot only)")
	// --singleton-ttl is the RECENCY half of the prune singleton (prunesingleton.go): a
	// sweep that completed less than this ago holds a new one, so N desk windows booting
	// together run ONE sweep rather than N. 0 disables the debounce; the singleton's LOCK
	// is always taken and there is no flag that disables it.
	singletonTTLStr := fs.String("singleton-ttl", defaultSingletonTTL.String(),
		"skip the sweep when one completed less than this ago (e.g. 10m); 0 disables the recency check (the singleton lock is always taken)")
	noSingleton := fs.Bool("no-singleton", false,
		"disable the singleton's recency check (equivalent to --singleton-ttl 0); the lock is still taken, so a live sweep still holds this one")
	// --no-branches / --branches-only are the cadence split for the local-branch GC
	// (Step C, branchgc.go): the GC runs at the same moments as prune by default, and a
	// boot path that wants the worktree arms more (or less) often than the branch pass
	// controls that here.
	noBranches := fs.Bool("no-branches", false,
		"skip the local-branch GC (run the worktree arms only)")
	branchesOnly := fs.Bool("branches-only", false,
		"run ONLY the local-branch GC (skip every worktree arm)")
	// --no-cache / --cells-root are the cache pressure pass's (Step D,
	// cachepressure.go) cadence and scope knobs: the pass runs at the same moments as
	// prune by default, against $CELLS_ROOT unless told otherwise.
	noCache := fs.Bool("no-cache", false,
		"skip the cell-cache pressure pass (run the worktree arms and branch GC only)")
	cellsRootFlag := fs.String("cells-root", "",
		"cells root the cache pressure pass sweeps for marked cell Go-cache roots (default: $CELLS_ROOT, then the per-user data-directory default)")
	positionals, perr := parseInterspersed(fs, args)
	if perr != nil {
		// TIER TWO: a help screen is not a refusal and writes no audit row (deskkit/helprequest.go).
		if deskkit.IsHelpRequest(perr) {
			return deskkit.ErrHelpRequested
		}
		return deskkit.Refused("refused: prune takes no flags but --repo, --interval, " +
			"--reclaim-stale-locks, --reap-dead-sessions, --lock-ttl, --dry-run, --singleton-ttl, " +
			"--no-singleton, --no-branches, --branches-only, --no-cache and --cells-root " +
			"(there is no --force): " + perr.Error())
	}
	if len(positionals) != 0 {
		return deskkit.Refused("refused: prune takes no positional arguments")
	}

	opts := pruneOpts{
		reclaimStaleLocks: *reclaim,
		reapDeadSessions:  *reapDead,
		dryRun:            *dryRun,
		singletonTTL:      defaultSingletonTTL,
		noBranches:        *noBranches,
		branchesOnly:      *branchesOnly,
		noCache:           *noCache,
		cellsRoot:         cellsRootForSweep(*cellsRootFlag),
	}
	// Reaping needs the named lock as ownership evidence. Reclaiming it first
	// loses that proof in a live sweep while dry-run retains it. Refuse before
	// constructing the guard, taking the singleton, or mutating bookkeeping.
	if opts.reclaimStaleLocks && opts.reapDeadSessions {
		return deskkit.Refused("refused: run --reap-dead-sessions without --reclaim-stale-locks to preserve named owner evidence")
	}
	// The split flags are contradictory together, and the opt-in WORKTREE arms under
	// --branches-only would be silently inert — the exact shape of failure this tool
	// refuses everywhere else (see --lock-ttl below). Say so instead of accepting a knob
	// that does nothing.
	if opts.noBranches && opts.branchesOnly {
		return deskkit.Refused("refused: --no-branches and --branches-only are contradictory")
	}
	if opts.branchesOnly && (opts.reclaimStaleLocks || opts.reapDeadSessions) {
		return deskkit.Refused("refused: --branches-only runs no worktree arm, so " +
			"--reclaim-stale-locks and --reap-dead-sessions have no effect with it")
	}
	// The cache pass runs no more under --branches-only than the worktree arms do, so its
	// knobs are silently inert there — and --cells-root scopes nothing a --no-cache sweep
	// skipped. Refuse both pairings rather than accept a knob that does nothing.
	if opts.branchesOnly && opts.noCache {
		return deskkit.Refused("refused: --branches-only runs no cache pressure pass, so --no-cache has no effect with it")
	}
	if opts.branchesOnly && *cellsRootFlag != "" {
		return deskkit.Refused("refused: --branches-only runs no cache pressure pass, so --cells-root has no effect with it")
	}
	if opts.noCache && *cellsRootFlag != "" {
		return deskkit.Refused("refused: --cells-root scopes the cache pressure pass, which --no-cache skips")
	}
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
	// A TTL without one of the passes that reads it would be silently inert — the exact
	// shape of failure this tool refuses everywhere else. Say so instead of accepting a
	// knob that does nothing. Both passes reach judgeLock, so both arm the age fallback.
	if opts.lockTTL > 0 && !opts.reclaimStaleLocks && !opts.reapDeadSessions {
		return deskkit.Refused("refused: --lock-ttl has no effect without --reclaim-stale-locks or --reap-dead-sessions")
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
		if res.bookkept == 0 && res.removed == 0 && res.reaped == 0 && len(res.reclaimed) == 0 &&
			len(res.staleRegs) == 0 && res.branchGC.collected() == 0 && res.cache.reclaimed == 0 && len(res.cache.reaped) == 0 {
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
// number that used to grow without bound), and how many locks were retired. When the
// local-branch GC ran, the line carries its counts too — collected per proof class, and
// held — because "no branches collected" and "the branch pass did not run" are different
// facts and only one of them should ever read as zero. The cache pressure pass's segment
// appears only when that pass acted or found pressure (cachepressure.go, the noise-floor
// contract): a sweep whose caches are all within policy says nothing about them.
func pruneSummaryLine(res pruneResult) string {
	if res.branchGC.only {
		// --branches-only: the worktree arms did not run, so their zeros are not counts of
		// anything and the line leads with the half that did.
		return fmt.Sprintf("deskwt prune: branches-gc %d (merged %d, cherry-clean %d), branches-held %d",
			res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
	}
	line := fmt.Sprintf(
		"deskwt prune: pruned %d bookkeeping entr%s, removed %d merged+clean worktree%s, held %d (locked-held %d), locks-reclaimed %d, dead-session-reaped %d, branches-deleted %d",
		res.bookkept, plural(res.bookkept, "y", "ies"),
		res.removed, plural(res.removed, "", "s"),
		len(res.skips), res.lockedHeld, len(res.reclaimed), res.reaped, len(res.branches))
	line += staleRegSegment(len(res.staleRegs))
	if res.branchGC.ran {
		line += fmt.Sprintf(", branches-gc %d (merged %d, cherry-clean %d), branches-held %d",
			res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
	}
	if res.cache.ran && res.cache.notable() {
		line += ", " + res.cache.summarySegment()
	}
	return line
}

// pruneAuditDetail is the same counts in the audit line's detail field.
func pruneAuditDetail(res pruneResult) string {
	if res.branchGC.only {
		return fmt.Sprintf("branches-gc %d (merged %d, cherry-clean %d), branches-held %d",
			res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
	}
	detail := fmt.Sprintf("pruned %d bookkeeping, removed %d, held %d (locked-held %d), locks-reclaimed %d, dead-session-reaped %d, branches-deleted %d",
		res.bookkept, res.removed, len(res.skips), res.lockedHeld, len(res.reclaimed), res.reaped, len(res.branches))
	detail += staleRegSegment(len(res.staleRegs))
	if res.branchGC.ran {
		detail += fmt.Sprintf(", branches-gc %d (merged %d, cherry-clean %d), branches-held %d",
			res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
	}
	if res.cache.ran && res.cache.notable() {
		detail += ", " + res.cache.summarySegment()
	}
	return detail
}

// renderSweepDetail writes the lines that must never be reduced to a count: every lock this
// sweep retired (worktree, lock reason, and the evidence that judged it stale) and everything
// the sweep could not do. Both are rare and both change on-disk state or explain why it did
// not change, so both print in one-shot AND in interval mode.
func renderSweepDetail(w io.Writer, res pruneResult) {
	// The stale-registration rows print in BOTH modes and always in full: each names a
	// registration dropped (or, in a dry run, the one that would be) while its directory
	// stays on disk — a distinction an operator must never have to guess at.
	for _, e := range res.staleRegs {
		fmt.Fprintln(w, "  "+staleRegLine(e, res.dryRun))
	}
	for _, r := range res.reclaimed {
		fmt.Fprintf(w, "  reclaimed lock on %s — reason %s — stale: %s\n", r.path, lockReasonText(r.reason), r.why)
	}
	// The dead-session plan. A DRY RUN prints it whole — every worktree the arm considered,
	// REAP and KEEP alike, with the session it attributed and the reason — because
	// inspecting the plan before arming the supervisor is what the dry run is FOR. A live
	// sweep prints the rows it acted on; the KEEPs are already reported, one per line, by
	// the caller's skip listing.
	for _, v := range res.plan {
		if res.dryRun || v.reap {
			fmt.Fprintln(w, v.planLine())
		}
		if v.reap && v.branch != "" {
			fmt.Fprintf(w, "    deleted stale local branch %s (equal to or behind its upstream)\n", v.branch)
		}
	}
	// The branch-GC plan. A checkout that needs this pass can carry THOUSANDS of candidate
	// branches, so a LIVE sweep prints only the summary-line counts (the noise-floor
	// contract); the per-branch plan — the COLLECT rows an operator is signing off, each
	// with the proof class — is a --dry-run artifact, and a dry run prints nothing it would
	// not collect.
	if res.branchGC.ran && res.dryRun {
		for _, v := range res.branchGC.plan {
			if v.collect {
				fmt.Fprintln(w, v.planLine())
			}
		}
	}
	// The cache pass's per-root rows: the orphaned roots a live sweep reaped (named, never
	// counted into silence), and the dry-run would-act plan.
	renderCacheDetail(w, res.cache)
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
			var tick string
			if res.branchGC.only {
				// --branches-only: the worktree arms did not run, so their zeros are not
				// counts of anything and the tick leads with the half that did.
				tick = fmt.Sprintf("%s deskwt prune tick %d: branches_gc=%d (merged=%d cherry=%d) branches_held=%d",
					nowStamp(), ticks,
					res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
			} else {
				tick = fmt.Sprintf("%s deskwt prune tick %d: bookkept=%d removed=%d held=%d locked_held=%d locks_reclaimed=%d",
					nowStamp(), ticks, res.bookkept, res.removed, len(res.skips), res.lockedHeld, len(res.reclaimed))
				if res.branchGC.ran {
					tick += fmt.Sprintf(" branches_gc=%d (merged=%d cherry=%d) branches_held=%d",
						res.branchGC.collected(), res.branchGC.merged, res.branchGC.cherry, res.branchGC.held())
				}
				if res.cache.ran && res.cache.notable() {
					tick += " " + res.cache.summarySegment()
				}
			}
			fmt.Fprintln(os.Stdout, tick)
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
	res.dryRun = opts.dryRun

	// --branches-only: every worktree arm is skipped and the sweep is the local-branch GC
	// alone. The flag parse has already refused the opt-in worktree arms with it, so what
	// is skipped here can never be an arm the caller asked for.
	if opts.branchesOnly {
		sc := newSweepCtx(dir)
		res.branchGC = gcLocalBranches(sc, dir, opts.dryRun)
		res.branchGC.only = true
		res.warns = append(res.warns, res.branchGC.warns...)
		return res, nil
	}

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

	// Enumerate AFTER the bookkeeping prune so already-gone entries are not iterated —
	// but BEFORE the reclaim pass, because Step A1's content judgements (below) must feed
	// it: a husk the content gate holds is excluded from reclaim, so its lock is still
	// on when the post-reclaim bookkeeping prune runs.
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
	// refs/remotes/origin/main. See sweepCtx. Built before the reclaim pass for the same
	// reason the enumeration is: Step A1's content gate answers "is this husk's detached
	// HEAD on the mainline" from this set.
	sc := newSweepCtx(dir)
	if sc.mainAncestors == nil {
		res.warns = append(res.warns,
			"could-not-check: refs/remotes/origin/main history unreadable, so NO worktree can be proven merged — every candidate is held and nothing is removed: "+
				errText(sc.ancestorErr))
	}

	// Step A1, phase 1 — stale-registration CONTENT judgements (stalereg.go; pure reads,
	// no mutation): the directory EXISTS but its .git gitdir file is GONE. Clearing the
	// registration deletes the admin dir (.git/worktrees/<id>/ — HEAD, index, reflog), so
	// a husk whose admin HEAD is DETACHED and not on the mainline is HELD: clearing it
	// would strand possibly-unpushed commits. The sweep's scope gates (shared checkout,
	// current worktree, sanctioned prefixes) apply exactly as in every other arm.
	staleVerdicts := judgeStaleRegistrations(guard, sc, cwd, roots, locked)

	// Step A2 — OPT-IN lock reclaim. It only UNLOCKS; nothing below this point is relaxed,
	// so a reclaimed worktree still has to pass every removal gate on its own merits. A
	// second bookkeeping prune follows any reclaim because Step A cannot drop the dangling
	// admin entry of a LOCKED worktree — those entries are exactly the ones a permanent lock
	// makes immortal, and they only become droppable once the lock is gone.
	//
	// The exclusion is Step A1's content-held LOCKED husks: unlocking one would feed it to
	// that second bookkeeping prune, which drops the registration (and, for a detached
	// HEAD, the last ref to its commits) the content gate just held.
	//
	// Under --dry-run this pass REPORTS what it would unlock and unlocks nothing: a dry run
	// that silently retired locks would be a worse surprise than one that silently pruned
	// bookkeeping, and both are now off the dry-run path.
	if opts.reclaimStaleLocks {
		reclaimed, warns, rerr := reclaimStaleLocksMode(guard, dir, cwd, opts.lockTTL, opts.dryRun, staleRegHeldLocked(staleVerdicts))
		res.reclaimed = reclaimed
		res.warns = append(res.warns, warns...)
		if rerr != nil {
			return res, rerr
		}
	}

	// Step A1, phase 2 — ACT on the judgements. The lock rule is absolute: this pass has
	// no unlock in it. A locked husk is cleared only when the reclaim pass retired its
	// lock THIS sweep (proven stale by judgeLock); in the default sweep — or behind a
	// lock that proves nothing, or git's own in-flight "initializing" add lock — it is
	// HELD with the reason named. The clear is one repo-wide prune plus a positive
	// verification listing; an entry that survives it after its lock was reclaimed is
	// re-locked with its original reason; failed restoration stops the sweep.
	// Cleared AND held entries are skipped by the
	// candidate loop below (their trees cannot be opened; every gate there would only
	// fail closed into a misleading reason).
	reclaimedLocks := make(map[string]string, len(res.reclaimed))
	for _, r := range res.reclaimed {
		reclaimedLocks[r.path] = r.reason
	}
	staleCleared, staleHeld, staleWarns, staleErr := clearStaleRegistrations(guard, dir, staleVerdicts, reclaimedLocks, opts.dryRun)
	res.staleRegs = staleCleared
	res.skips = append(res.skips, staleHeld...)
	res.warns = append(res.warns, staleWarns...)
	if staleErr != nil {
		return res, staleErr
	}
	for _, held := range staleHeld {
		if _, isLocked := locked[held.path]; isLocked {
			res.lockedHeld++
		}
	}
	staleSet := staleRegSet(staleVerdicts)

	// Clear husks behind their rollback boundary before any later bookkeeping or
	// inventory read can fail. A failed clear has restored its lock by this point.
	if len(res.reclaimed) > 0 && !opts.dryRun {
		second, sErr := bookkeepingPrune(dir, false)
		if sErr != nil {
			return res, deskkit.Unverifiable("git worktree prune (bookkeeping, after lock reclaim) failed", sErr)
		}
		res.bookkept += second
		// The reclaim changed the state both views were read from: locks it retired
		// are gone, and the prune above may have dropped newly-droppable entries.
		// Re-read so the candidate loop judges the registration as it STANDS, never
		// iterating a path the prune just dropped or holding a tree by a retired lock.
		roots, lerr = guard.worktreeRoots(dir)
		if lerr != nil {
			return res, lerr
		}
		locked, lkErr = guard.lockedWorktrees(dir)
		if lkErr != nil {
			return res, lkErr
		}
	}

	// The sweep's clock, read once: the fresh-worktree safety window's age test.
	sweepNow := time.Now()

	// pendingRemoved collects the trees deleted in this loop. Their admin entries are
	// dropped ONCE after the loop (see the batched deregistration below), not per removal.
	var pendingRemoved []string
	// removedPaths is the sweep's removals (and, under --dry-run, its would-removals) from
	// BOTH arms, for the cache pressure pass's orphan reap (Step D): a marked cache root
	// whose trust domain binds it to one of these paths lost its owner this sweep.
	var removedPaths []string
	// reapedPaths / pendingBranch are the dead-session arm's half of that batch: which of
	// the deleted trees this arm cleared, and which stale local branch goes with each.
	// The branch delete CANNOT run inside the loop — git refuses to delete a branch that is
	// still checked out in a registered worktree, and the deregistration is batched — so it
	// happens after the one deregistration pass, and only for paths that actually left the
	// registration.
	reapedPaths := map[string]bool{}
	pendingBranch := map[string]string{}

	// Dead-session arm state, resolved ONCE per sweep rather than per worktree: one stat of
	// the lock admin files, one roster-directory resolution, one clock read.
	var (
		reapMtimes map[string]time.Time
		reapRoster string
		reapNow    time.Time
	)
	if opts.reapDeadSessions {
		reapMtimes = guard.lockFileMtimes()
		reapRoster = rosterBeaconDir()
		reapNow = time.Now()
		if reapRoster == "" {
			res.warns = append(res.warns,
				"could-not-check: roster beacons are unreadable, so no locked worktree can be proven to belong to a DEAD session — "+
					"the reap arm holds every locked worktree this sweep"+ttlHint(opts.lockTTL))
		}
	}

	for _, rt := range roots {
		// Stale registrations — cleared AND held alike — are Step A1's, already reported
		// there; the candidate loop has nothing left to ask of them (their tree cannot be
		// opened, so every gate would only fail closed into a misleading reason).
		if staleSet[rt] {
			continue
		}
		reason, isLocked := locked[rt]

		// The identity gates bind the dead-session arm too, and must therefore be decided
		// BEFORE it runs: that arm may act on a LOCKED worktree, which is the one case the
		// ordinary order (lock gate first, identity second) never reached. The shared
		// checkout, the current worktree and anything outside a sanctioned prefix are
		// refused here exactly as they are below; only the point at which the question is
		// asked moved, and the reported reason for each is unchanged.
		protected := rt == guard.sharedCheckout || rt == cwd || !guard.allowed(rt)

		if opts.reapDeadSessions && protected {
			// Named in the plan rather than omitted from it: a plan an operator signs off
			// must account for every registered worktree, and "this path was never a
			// candidate" is an answer the plan has to be able to give.
			res.plan = append(res.plan, reapVerdict{path: rt, reason: "protected: " + protectedReason(guard, cwd, rt)})
		}

		if opts.reapDeadSessions && !protected {
			mt, haveMtime := reapMtimes[rt]
			v := judgeReap(sc, rt, reason, isLocked, mt, haveMtime, opts.lockTTL, reapRoster, reapNow)
			if v.reap {
				// --dry-run stops here exactly as the ordinary arm does: count the
				// would-be reap, record the plan row, delete nothing and unlock nothing.
				if opts.dryRun {
					res.reaped++
					res.plan = append(res.plan, v)
					removedPaths = append(removedPaths, rt)
					continue
				}
				branch, rerr := reapWorktree(sc, dir, rt, isLocked)
				if rerr != nil {
					v.reap = false
					v.reason += "; reap failed: " + errText(rerr)
					res.plan = append(res.plan, v)
					res.skips = append(res.skips, skipEntry{rt, "dead-session reap failed: " + errText(rerr)})
					continue
				}
				v.branch = branch
				res.plan = append(res.plan, v)
				reapedPaths[rt] = true
				if branch != "" {
					pendingBranch[rt] = branch
				}
				pendingRemoved = append(pendingRemoved, rt)
				continue
			}
			// A KEEP is recorded and the worktree falls through to the ordinary arm,
			// which may still clear it under the merge rule. The reap arm only ever ADDS
			// removals; it takes none away.
			res.plan = append(res.plan, v)
		}

		// Lock gate FIRST: a locked worktree is never deleted, whatever its content state
		// says (a live agent's worktree was spared here only by luck of a content heuristic).
		if isLocked {
			res.skips = append(res.skips, skipEntry{rt, lockedReason(reason)})
			res.lockedHeld++
			continue
		}
		if protected {
			res.skips = append(res.skips, skipEntry{rt, protectedReason(guard, cwd, rt)})
			continue
		}

		// GATE ORDER. The tip gate runs FIRST, then the ancestry gate, and the
		// tracked-clean gate last.
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

		// Fresh-worktree guard (prune-only), CONTENT-AWARE: a worktree whose HEAD is
		// exactly at the origin/main tip (zero landed commits) is removed only when THREE
		// at-tip gates all pass, each answering a question the ancestry and tracked-clean
		// gates cannot:
		//
		//   1. AGE — the fresh-worktree safety window. `deskwt add` never locks the tree
		//      it creates, so a JUST-CREATED pristine tree (the verifier/reviewer shape)
		//      is content-identical to reclaim residue; only the admin entry's age tells
		//      them apart. Younger than the window → HELD as too fresh. An unreadable age
		//      fails closed.
		//   2. CONTENT — the untracked-INCLUSIVE status (the dead-session arm's own
		//      dirtyAnyIn): tracked-clean deliberately ignores untracked files, so the
		//      only way an at-tip tree can hold committed-form work is as uncommitted or
		//      untracked content. Any present → HELD with this guard's reason.
		//   3. IGNORED PAYLOAD — git-ignored files exist in no commit and no status, so a
		//      tree carrying them is not byte-redundant with the mainline. Any present →
		//      HELD. (The dead-session arm's review path checks the same inventory.)
		//
		// The blanket hold this design replaced kept EVERY at-tip worktree, pristine ones
		// included — and a pristine at-tip tree is the exact shape a reclaim-by-detach
		// flow leaves behind: HEAD detached at the mainline tip, no commits, no files of
		// its own. Those accumulated without bound because the guard never checked the
		// one thing it existed to protect. An at-tip tree passing all three gates now
		// FALLS THROUGH: the merge gate passes it (the tip is a member of its own
		// ancestor set) and the tracked-clean gate passes it (a fully clean tree is
		// tracked-clean), so it is removed — losslessly, since everything in it is
		// either on origin/main or absent. remove (human-named single path) is not gated.
		atTip, tipErr := headAtOriginMainTip(sc, rt)
		if tipErr != nil {
			res.skips = append(res.skips, skipEntry{rt, "unverifiable origin/main position: " + tipErr.Error()})
			continue
		}
		if atTip {
			age, ageErr := guard.adminEntryAge(rt, sweepNow)
			if ageErr != nil {
				res.skips = append(res.skips, skipEntry{rt, "unverifiable worktree age: " + errText(ageErr)})
				continue
			}
			if age < freshWorktreeSafetyWindow {
				res.skips = append(res.skips, skipEntry{rt, "fresh worktree at origin/main (created " + roundAge(age) +
					" ago — inside the " + freshWorktreeSafetyWindow.String() + " safety window for a just-created worktree)"})
				continue
			}
			anyDirty, aErr := dirtyAnyIn(sc, rt)
			if aErr != nil {
				res.skips = append(res.skips, skipEntry{rt, "unverifiable working-tree status: " + errText(aErr)})
				continue
			}
			if anyDirty != "" {
				res.skips = append(res.skips, skipEntry{rt, "fresh worktree at origin/main (no landed commits — holds uncommitted or untracked work)"})
				continue
			}
			ignored, iErr := ignoredReviewFiles(rt)
			if iErr != nil {
				res.skips = append(res.skips, skipEntry{rt, "unverifiable ignored-payload inventory: " + errText(iErr)})
				continue
			}
			if ignored != "" {
				res.skips = append(res.skips, skipEntry{rt, "fresh worktree at origin/main (no landed commits — holds git-ignored payload the mainline does not)"})
				continue
			}
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
			removedPaths = append(removedPaths, rt)
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
			if strandedSet[rt] {
				continue
			}
			if reapedPaths[rt] {
				res.reaped++
			} else {
				res.removed++
			}
			removedPaths = append(removedPaths, rt)
		}

		// Stale local branches, deleted only now: the branch of a worktree that is still
		// registered is still checked out, and git would (rightly) refuse. A path that
		// stranded keeps its branch — the collision it would cause is the lesser harm
		// against deleting the ref of a tree whose registration we could not clear.
		for _, rt := range pendingRemoved {
			branch := pendingBranch[rt]
			if branch == "" || strandedSet[rt] {
				continue
			}
			if derr := deleteLocalBranch(dir, branch); derr != nil {
				res.warns = append(res.warns,
					"worktree "+rt+" was reaped but its local branch "+branch+" could not be deleted "+
						"(a later `deskwt add --branch "+branch+"` will still collide): "+errText(derr))
				continue
			}
			res.branches = append(res.branches, branch)
		}
	}

	// Step D — the CELL-CACHE pressure pass (cachepressure.go), default ON, skipped by
	// --no-cache. It runs AFTER the worktree arms on purpose: a cache root that lived
	// inside a tree this sweep just removed is already gone — silently absent from
	// discovery, the best possible outcome — and the orphan reap binds to THIS sweep's
	// verified removals (removedPaths), never to a removal some earlier run made.
	if !opts.noCache {
		cacheRes, cacheWarns := sweepCellCaches(opts.cellsRoot, opts.dryRun, removedPaths)
		res.cache = cacheRes
		res.warns = append(res.warns, cacheWarns...)
	}

	// Step C — the LOCAL-BRANCH GC (branchgc.go), default ON, skipped by --no-branches. It
	// runs AFTER the worktree arms on purpose: a worktree Step B just removed leaves its
	// (merged) local branch behind, and this pass collects exactly that residue in the same
	// sweep. Its candidate view is read FRESH inside the pass (the holder set is re-listed),
	// so the worktree deletions above are already reflected. The sweep's ONE mainline walk
	// (sc) is shared, so the ancestry proof costs a map lookup per branch.
	if !opts.noBranches {
		res.branchGC = gcLocalBranches(sc, dir, opts.dryRun)
		res.warns = append(res.warns, res.branchGC.warns...)
	}
	return res, nil
}

// reapWorktree performs the dead-session arm's destructive step for ONE worktree a caller
// has already proven ownerless, clean and pushed, and returns the stale local branch that
// should be deleted once the batch deregistration has run (or "" when there is none).
//
// The unlock comes first and is not optional: `git worktree prune` will not drop the admin
// entry of a LOCKED worktree, so deleting the directory of one without unlocking it strands
// the registration — the half-destroyed outcome of #264, reached from the other side.
//
// The branch is resolved BEFORE the directory is deleted, because it is read from the
// worktree's own HEAD and upstream config, which stop being readable the moment the tree
// goes. The sweep's handle on rt is dropped for the same reason the ordinary arm drops it:
// nothing reads the tree again, and holding a repo handle on a deleted path is a footgun.
func reapWorktree(sc *sweepCtx, dir, rt string, isLocked bool) (string, error) {
	if isLocked {
		if _, uerr := runGit(dir, "worktree", "unlock", rt); uerr != nil {
			return "", deskkit.Unverifiable("cannot unlock the dead session's worktree "+rt, uerr)
		}
	}

	// A branch that cannot be resolved is not a reason to abandon the reap — the worktree
	// itself is what wedges the queue. The unresolvable branch is reported as a warning by
	// the caller only if a DELETE fails; an unreadable one simply yields "" and is left.
	var branch string
	if repo, oerr := sc.open(rt); oerr == nil {
		if b, berr := branchSafeToDelete(repo); berr == nil {
			branch = b
		}
	}

	delete(sc.repos, rt)
	if rmErr := removeWorktreeTree(rt); rmErr != nil {
		return "", rmErr
	}
	return branch, nil
}

// protectedReason names WHY a registered worktree is not a removal candidate at all, for
// the three identity refusals no content gate can ever override. It is the single source of
// those three strings, so the skip listing and the dead-session plan can never describe the
// same refusal in two different ways. The order matches the order the checks are made in.
func protectedReason(guard *pathGuard, cwd, rt string) string {
	switch {
	case rt == guard.sharedCheckout:
		return "shared checkout (refused by identity — never removed)"
	case rt == cwd:
		return "current worktree"
	default:
		return "not under a sanctioned prefix"
	}
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
// with new source files would be wrongly removed. The caller therefore pairs an affirmative
// answer with three more gates — the fresh-worktree safety window (admin-entry age), the
// untracked-INCLUSIVE status, and the ignored-payload inventory: an at-tip tree drains only
// when it is old enough to be no session's just-created workspace and provably holds nothing
// the mainline does not. This guard is prune-only: remove is a
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
