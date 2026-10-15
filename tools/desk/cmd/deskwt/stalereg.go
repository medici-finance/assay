package main

// stalereg.go — prune's STALE-REGISTRATION sweep (Step A1): registrations whose worktree
// directory EXISTS but whose .git gitdir file is GONE.
//
// The shape is half-dead residue: the directory survived while the git link inside it did
// not (a partial delete, a cleaner that took the file but not the tree). git can do
// nothing with it:
//
//   - `git worktree remove` refuses — "validation failed: '<dir>/.git' does not exist" —
//     even with --force, because there is no tree state left to validate;
//   - `git worktree prune` keeps it while it is LOCKED (a lock vetoes pruning however
//     dead the tree is), so the locked variant survives every sweep forever;
//   - every content gate in the candidate loop fails CLOSED on it (a worktree with no
//     .git link cannot be opened), so Step B would report it "unverifiable" and hold it
//     every sweep, forever.
//
// Meanwhile the registration still counts as a branch holder and as a row of
// `git worktree list` — residue that wedges exactly the things this sweep exists to keep
// moving.
//
// THE PASS runs in two phases, and clearing one of these registrations is gated harder
// than the bookkeeping Step A it follows, because clearing deletes the admin dir
// (.git/worktrees/<id>/ — HEAD, index, reflog), which is the last git record the husk
// has:
//
//   - SCOPE (phase 1, judgeStaleRegistrations): the same predicate the lock-reclaim pass
//     enforces — never the shared checkout, never the current worktree, never anything
//     outside the sanctioned prefixes. A husk outside the sweep's scope is not judged
//     and not touched.
//   - CONTENT (phase 1): the admin dir's HEAD is the one git record that survives the
//     loss of the worktree's .git file, and clearing the registration deletes it. A
//     branch-pointing HEAD must name a valid branch that resolves to a commit. Missing,
//     invalid or unreadable branch targets hold the registration and its lock. A DETACHED
//     HEAD must be proven reachable from refs/remotes/origin/main via the sweep's
//     ancestor set — otherwise clearing would strand possibly-unpushed commits, and the
//     husk is HELD with that reason. Anything unreadable or unclassifiable is held as
//     could-not-check. A content-held husk is also excluded from the lock-reclaim pass:
//     an unlock there would feed it to the post-reclaim bookkeeping prune, which drops
//     what this gate just held.
//   - LOCK (phase 2, clearStaleRegistrations): the default sweep NEVER retires a lock.
//     A locked husk is cleared only when --reclaim-stale-locks has already retired its
//     lock this sweep — judgeLock's proven-staleness evidence, the one place unlocks may
//     happen — and git's own in-flight add lock (reason "initializing") is never judged
//     stale anywhere (judgeLock itself exempts it).
//
// What a clear removes is the REGISTRATION — never the tree: a directory with no .git
// link is git-inert, so the admin entry can never again map to a functioning worktree.
// The directory itself is LEFT ON DISK, UNTOUCHED, for the operator or the OS's own temp
// reaper. (Note what this pass does NOT govern: git's own Step A prune drops an UNLOCKED
// husk on sight, as it always has — that is git's bookkeeping semantics, unchanged here.)
//
// Fail-closed, as everywhere in this tool: any stat that errors for a reason other than
// "not there" leaves the entry alone; a prune failure or a still-registered verification
// is a named warning and never counts as cleared; and an entry this pass attempted after
// the reclaim pass retired its lock is re-locked, if possible, with its original reason. If restoration fails, the sweep reports could-not-check and stops before
// any later pruning; the surviving registration is left for operator recovery.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// staleRegEntry is one stale registration (worktree dir present, .git file gone) that the
// sweep judged. wasLocked records whether it was locked when the sweep read lock state,
// and lockReason carries that lock's reason, so the held report can name git's in-flight
// add lock and a failed clear can restore the exact reason it found.
type staleRegEntry struct {
	path       string
	wasLocked  bool
	lockReason string
}

// staleRegVerdict is phase 1's judgement for one in-scope stale registration:
// holdReason == "" means the CONTENT gate proves the HEAD target is retained by a
// resolving branch or the remote mainline; anything else is the reported hold reason.
type staleRegVerdict struct {
	entry staleRegEntry
	// holdReason is the CONTENT hold only. The lock question is phase 2's, because it
	// can only be answered after the reclaim pass has run.
	holdReason string
}

// judgeStaleRegistrations is phase 1 — PURE (no mutation, no clock): find the stale
// registrations among the sweep's enumerated roots, apply the scope gates every sweep
// arm enforces, and judge each survivor's CONTENT against the one record that survives
// it — the admin dir's HEAD.
func judgeStaleRegistrations(guard *pathGuard, sc *sweepCtx, cwd string, roots []string, locked map[string]string) []staleRegVerdict {
	var out []staleRegVerdict
	for _, rt := range roots {
		// The scope gates, identical to the lock-reclaim pass's: the shared checkout by
		// identity, the current worktree, and anything outside the sanctioned prefixes
		// are none of this pass's business.
		if rt == guard.sharedCheckout || rt == cwd || !guard.allowed(rt) {
			continue
		}
		if _, serr := os.Stat(rt); serr != nil {
			// Directory gone: Step A's class (dangling admin entry). Any OTHER stat
			// failure fails closed into the candidate loop, which holds the entry as
			// unverifiable.
			continue
		}
		_, gerr := os.Stat(filepath.Join(rt, ".git"))
		switch {
		case gerr == nil:
			continue // a live link (file or directory): a functioning worktree
		case os.IsNotExist(gerr):
			// the stale shape — judge it below
		default:
			continue // unreadable in some other way: fail closed, the loop holds it
		}

		reason, isLocked := locked[rt]
		v := staleRegVerdict{entry: staleRegEntry{path: rt, wasLocked: isLocked, lockReason: reason}}

		// The CONTENT gate: the admin dir's HEAD survives the lost .git file and is
		// deleted by the clearing, so it decides whether clearing can strand commits.
		admin, aerr := guard.adminDirFor(rt)
		if aerr != nil {
			v.holdReason = "could-not-check: " + errText(aerr)
		} else {
			v.holdReason = staleRegContentHold(sc, guard.sharedCheckout, admin)
		}
		out = append(out, v)
	}
	return out
}

// staleRegContentHold reads the husk's admin HEAD and returns "" when clearing the
// HEAD target is retained by a resolving branch or the remote mainline, else a hold.
//
// A branch-pointing HEAD ("ref: refs/heads/...") must positively resolve to a commit:
// syntax alone does not prove the branch exists. The branch lives in the shared git
// dir, not the admin dir, and the local-branch GC only
// ever deletes a branch proven on the mainline. A DETACHED HEAD is the admin dir's only
// ref to its commit, so clearing is allowed only when that commit is in the sweep's
// mainline ancestor set — already on the remote, so nothing is lost. Anything else
// (unreadable, unparseable, an ancestor set the sweep could not build) is a
// could-not-check hold.
func staleRegContentHold(sc *sweepCtx, dir, admin string) string {
	return adminHeadHold(dir, admin, func(head string) string {
		if sc.mainAncestors == nil {
			return "could-not-check: detached HEAD, and the sweep could not build the refs/remotes/origin/main ancestor set"
		}
		if _, ok := sc.mainAncestors[head]; !ok {
			return "detached HEAD " + head[:12] + " is not reachable from refs/remotes/origin/main — clearing the registration would strand possibly-unpushed commits"
		}
		return ""
	})
}

// adminHeadHold is the admin-HEAD content gate shared by the prune sweep (above) and
// `deskwt remove`'s husk arm (huskremove.go): the branch half is identical for both, and
// only the proof a DETACHED commit is retained differs, so the caller supplies it.
// detachedProof receives the full commit id and returns "" when clearing loses nothing,
// else the hold reason.
func adminHeadHold(dir, admin string, detachedProof func(head string) string) string {
	raw, rerr := os.ReadFile(filepath.Join(admin, "HEAD"))
	if rerr != nil {
		return "could-not-check: cannot read the admin entry's HEAD: " + errText(rerr)
	}
	head := strings.TrimSpace(string(raw))
	if strings.HasPrefix(head, "ref: refs/heads/") {
		ref := strings.TrimPrefix(head, "ref: ")
		if _, err := runGit(dir, "check-ref-format", ref); err != nil {
			return "could-not-check: invalid admin HEAD branch: " + errText(err)
		}
		if _, err := runGit(dir, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}"); err != nil {
			return "could-not-check: admin HEAD branch does not resolve to a commit: " + errText(err)
		}
		return "" // the valid branch target positively resolves to an existing commit
	}
	if !isFullSHA(head) {
		return "could-not-check: the admin entry's HEAD is neither a branch ref nor a commit id"
	}
	return detachedProof(head)
}

// isFullSHA reports whether s is a full lowercase-or-uppercase hex commit id.
func isFullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// clearStaleRegistrations is phase 2 — the act half. It takes phase 1's verdicts and the
// set of locks the reclaim pass retired THIS sweep (reclaimed: path → original lock
// reason; under --dry-run, the locks it WOULD retire), and clears every verdict that is
// content-clearable AND not locked behind a lock this sweep may not touch.
//
// The lock rule is absolute here: this pass contains no unlock call. A locked husk whose
// lock the reclaim pass did not (or would not) retire is HELD — the default sweep never
// retires a lock, and git's in-flight add lock ("initializing") is never retired by any
// arm (judgeLock exempts it, so the reclaim pass never reaches it either).
//
// The clear itself is one repo-wide `git worktree prune` plus a POSITIVE verification
// listing. If the clear fails for an entry the reclaim pass unlocked this sweep, the
// pass attempts to re-lock the entry with its original reason. A restoration error
// stops the caller before any later prune. Under --dry-run it reports and touches nothing.
func clearStaleRegistrations(guard *pathGuard, dir string, verdicts []staleRegVerdict, reclaimed map[string]string, dryRun bool) (cleared []staleRegEntry, held []skipEntry, warns []string, err error) {
	var toClear []staleRegEntry
	for _, v := range verdicts {
		e := v.entry
		if v.holdReason != "" {
			held = append(held, skipEntry{e.path, "stale registration (its .git file is gone): " + v.holdReason})
			continue
		}
		if e.wasLocked {
			if _, unlocked := reclaimed[e.path]; !unlocked {
				why := "locked — the default sweep never retires a lock; --reclaim-stale-locks clears this registration once the lock is PROVEN stale"
				if strings.TrimSpace(e.lockReason) == gitInitLockReason {
					why = "locked with git's in-flight `worktree add` lock (\"initializing\") — never retired by any sweep arm"
				}
				held = append(held, skipEntry{e.path, "stale registration (its .git file is gone): " + why})
				continue
			}
		}
		toClear = append(toClear, e)
	}
	if len(toClear) == 0 {
		return nil, held, nil, nil
	}
	if dryRun {
		return toClear, held, nil, nil
	}

	if _, perr := runGit(dir, "worktree", "prune"); perr != nil {
		warns = append(warns, "git worktree prune failed while clearing stale registrations: "+errText(perr))
		restored, err := relockReclaimedHusks(guard, dir, toClear, reclaimed)
		warns = append(warns, restored...)
		return nil, held, warns, err
	}

	// POSITIVE verification, batched: one listing for the whole pass, and only the paths
	// proven GONE from the registration are counted as cleared.
	set, lerr := worktreePathsFn(guard, dir)
	if lerr != nil {
		for _, e := range toClear {
			warns = append(warns, "stale registration "+e.path+": deregistration unverifiable after prune: "+errText(lerr))
		}
		restored, err := relockReclaimedHusks(guard, dir, toClear, reclaimed)
		warns = append(warns, restored...)
		return nil, held, warns, err
	}
	var failed []staleRegEntry
	for _, e := range toClear {
		if set[e.path] {
			warns = append(warns, "stale registration "+e.path+" is still registered after prune (not counted as cleared)")
			failed = append(failed, e)
			continue
		}
		cleared = append(cleared, e)
	}
	if len(failed) > 0 {
		restored, restoreErr := relockReclaimedHusks(guard, dir, failed, reclaimed)
		warns = append(warns, restored...)
		err = restoreErr
	}
	return cleared, held, warns, err
}

// relockReclaimedHusks restores the original lock on each entry whose lock the reclaim
// pass retired this sweep but whose clearing FAILED. Entries found unlocked have
// nothing to restore. Failed restoration is an error: the caller must stop subsequent
// pruning and leave the surviving registration for operator recovery.
func relockReclaimedHusks(guard *pathGuard, dir string, entries []staleRegEntry, reclaimed map[string]string) (warns []string, err error) {
	for _, e := range entries {
		reason, wasReclaimed := reclaimed[e.path]
		if !wasReclaimed {
			continue
		}
		if _, lerr := runGit(dir, "worktree", "lock", "--reason", reason, e.path); lerr != nil {
			failure := deskkit.Unverifiable("stale registration "+e.path+" lock could not be restored; stopping before further pruning", lerr)
			warns = append(warns, failure.Error())
			err = errors.Join(err, failure)
			continue
		}
		warns = append(warns, "stale registration "+e.path+" survived the clearing; its lock was restored with the original reason")
	}
	return warns, err
}

// staleRegHeldLocked is phase 1's output projected onto the reclaim pass's input: the
// LOCKED husks the content gate holds. The reclaim pass must not unlock them — an unlock
// there would feed them to the post-reclaim bookkeeping prune, which drops what the
// content gate just held.
func staleRegHeldLocked(verdicts []staleRegVerdict) map[string]bool {
	var out map[string]bool
	for _, v := range verdicts {
		if v.entry.wasLocked && v.holdReason != "" {
			if out == nil {
				out = map[string]bool{}
			}
			out[v.entry.path] = true
		}
	}
	return out
}

// staleRegSet renders the verdict list as a set, so the candidate loop can skip entries
// this pass already accounted for — cleared AND held alike — instead of reporting each a
// second time as an unverifiable hold. (Their trees cannot be opened, so every gate
// there would only fail closed into a misleading reason.)
func staleRegSet(verdicts []staleRegVerdict) map[string]bool {
	if len(verdicts) == 0 {
		return nil
	}
	set := make(map[string]bool, len(verdicts))
	for _, v := range verdicts {
		set[v.entry.path] = true
	}
	return set
}

// staleRegLine is the per-entry report row for a CLEAR, in both modes: a dry run says
// what it WOULD clear, a live sweep what it DID.
func staleRegLine(e staleRegEntry, dryRun bool) string {
	verb := "cleared stale registration "
	if dryRun {
		verb = "would clear stale registration "
	}
	line := verb + e.path + " — worktree directory exists but its .git file is gone (directory left on disk, untouched)"
	if e.wasLocked {
		if dryRun {
			line += "; its lock would be retired by the reclaim pass first"
		} else {
			line += "; its lock was retired by the reclaim pass first"
		}
	}
	return line
}

// staleRegSegment is the summary line's additive segment, present only when the pass
// acted — the noise-floor contract: a sweep with no stale registrations says nothing
// about them.
func staleRegSegment(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(", stale-registrations-cleared %d", n)
}

// adminDirFor returns the worktree's admin directory under the shared git dir, matched by
// the gitdir pointer rather than by name (git deduplicates names), so it works even when
// the worktree's own .git file is GONE — the whole point of the stale-registration pass.
// The comparison resolves the pointer the same way lockFileMtimes does, so keys compare
// equal to the resolved roots the caller iterates.
func (g *pathGuard) adminDirFor(rt string) (string, error) {
	if g.commonDir == "" {
		return "", deskkit.Unverifiable("the shared git-common-dir is not resolved", nil)
	}
	adminRoot := filepath.Join(g.commonDir, "worktrees")
	entries, err := os.ReadDir(adminRoot)
	if err != nil {
		return "", deskkit.Unverifiable("cannot read the worktree admin registry", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		admin := filepath.Join(adminRoot, e.Name())
		gd, rerr := os.ReadFile(filepath.Join(admin, "gitdir"))
		if rerr != nil {
			continue
		}
		dotGit := strings.TrimSpace(string(gd))
		if dotGit != "" && resolvePath(filepath.Dir(dotGit)) == rt {
			return admin, nil
		}
	}
	return "", deskkit.Unverifiable("no admin entry points at "+rt, nil)
}

// adminEntryAge returns how long ago the worktree's admin entry was last touched — the
// only "how fresh is this worktree" signal available without opening the tree (which the
// at-tip gate's callers sometimes cannot do, and a stale registration never can).
func (g *pathGuard) adminEntryAge(rt string, now time.Time) (time.Duration, error) {
	admin, err := g.adminDirFor(rt)
	if err != nil {
		return 0, err
	}
	fi, serr := os.Stat(admin)
	if serr != nil {
		return 0, deskkit.Unverifiable("cannot stat the worktree's admin entry", serr)
	}
	return now.Sub(fi.ModTime()), nil
}
