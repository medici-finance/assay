package main

// stalereg.go — prune's STALE-REGISTRATION sweep: registrations whose worktree directory
// EXISTS but whose .git gitdir file is GONE.
//
// The shape is half-dead residue: the directory survived while the git link inside it did
// not (a partial delete, a cleaner that took the file but not the tree). git can do
// nothing with it:
//
//   - `git worktree remove` refuses — "validation failed: '<dir>/.git' does not exist" —
//     even with --force, because there is no tree state left to validate;
//   - `git worktree prune` drops the unlocked variant (the entry reads "prunable: gitdir
//     file points to non-existent location") but keeps the LOCKED one — a lock vetoes
//     pruning however dead the tree behind it is — so a locked stale registration
//     survives every sweep forever;
//   - every content gate in the candidate loop fails CLOSED on it (a worktree with no
//     .git link cannot be opened), so Step B would report it "unverifiable" and hold it
//     every sweep, forever.
//
// Meanwhile the registration still counts as a branch holder and as a row of
// `git worktree list` — residue that wedges exactly the things this sweep exists to keep
// moving.
//
// What this pass clears is the REGISTRATION, never the tree. A directory with no .git
// link is git-inert — no git command runs in it, its HEAD is unreadable through it — so
// the admin entry can never again map to a functioning worktree. Clearing the entry frees
// the branch it held and takes it out of every later listing; the directory itself is
// LEFT ON DISK, UNTOUCHED, for the operator or the OS's own temp reaper.
//
// The lock question, stated plainly: this pass clears a LOCKED stale registration too,
// unlocking it first. That is not the lock gate weakening — the lock gate protects a
// removable WORKING TREE, and there is no working tree here for git to remove or for a
// session to work in; the lock's remaining content is a veto on bookkeeping, which is
// the exact mechanism that made this residue immortal. The candidate loop's lock gate is
// unchanged and still holds every locked worktree whose tree is intact.
//
// Fail-closed, as everywhere in this tool: any stat that errors for a reason other than
// "not there" leaves the entry alone (the candidate loop will hold it as unverifiable);
// an unlock or the deregistration prune failing leaves the entry registered and named in
// a warning; and an entry still registered after the prune is NOT counted as cleared.

import (
	"fmt"
	"os"
	"path/filepath"
)

// staleRegEntry is one registration this pass cleared (or, under --dry-run, would clear).
// wasLocked records whether the clearing had to retire a lock first, so the report line
// can say so — an unlock that hides inside a count is the kind of surprise this tool's
// audit output exists to prevent.
type staleRegEntry struct {
	path      string
	wasLocked bool
}

// clearStaleRegistrations finds the stale-registration shape among the sweep's already
// enumerated roots and clears it: unlock first where locked, then ONE repo-wide
// `git worktree prune`, then a POSITIVE verification listing — an entry still registered
// afterwards is named in a warning and not counted. Nothing here aborts a sweep: every
// failure is a named warning and the entry left registered (fail closed), so the
// signature carries no error.
//
// roots is the post-Step-A registration list and locked the sweep's single lock-state
// read, so the pass costs no extra git invocation until it has something to clear. Under
// --dry-run it reports and touches nothing: no unlock, no prune.
func clearStaleRegistrations(guard *pathGuard, dir string, roots []string, locked map[string]string, dryRun bool) (cleared []staleRegEntry, warns []string) {
	var stale []staleRegEntry
	for _, rt := range roots {
		// The shared checkout is identity-refused here as everywhere: its .git is a
		// directory, and its registration is the main checkout, not bookkeeping.
		if rt == guard.sharedCheckout {
			continue
		}
		if _, serr := os.Stat(rt); serr != nil {
			// Directory gone: Step A's class (dangling admin entry), already handled
			// there for the unlocked shape and by --reclaim-stale-locks for the locked
			// one. Any OTHER stat failure fails closed into the candidate loop, which
			// will hold the entry as unverifiable.
			continue
		}
		_, gerr := os.Stat(filepath.Join(rt, ".git"))
		switch {
		case gerr == nil:
			// A live link (file or directory): a functioning worktree, not this pass's
			// shape.
			continue
		case os.IsNotExist(gerr):
			_, isLocked := locked[rt]
			stale = append(stale, staleRegEntry{path: rt, wasLocked: isLocked})
		default:
			// Unreadable in some other way: fail closed, leave it to the candidate
			// loop's unverifiable hold.
			continue
		}
	}
	if len(stale) == 0 || dryRun {
		return stale, nil
	}

	// Unlock the locked ones first — `git worktree prune` will not drop a locked entry,
	// however dead the tree behind it. An unlock failure excludes the entry from the
	// prune set and says so; the entry stays registered (fail closed).
	prunable := make([]staleRegEntry, 0, len(stale))
	for _, e := range stale {
		if e.wasLocked {
			if _, uerr := runGit(dir, "worktree", "unlock", e.path); uerr != nil {
				warns = append(warns, "stale registration "+e.path+" (its .git file is gone) could not be unlocked for clearing: "+errText(uerr))
				continue
			}
		}
		prunable = append(prunable, e)
	}
	if len(prunable) == 0 {
		return nil, warns
	}
	if _, perr := runGit(dir, "worktree", "prune"); perr != nil {
		warns = append(warns, "git worktree prune failed while clearing stale registrations: "+errText(perr))
		return nil, warns
	}

	// POSITIVE verification, batched: one listing for the whole pass, and only the paths
	// proven GONE from the registration are counted as cleared.
	set, lerr := worktreePathsFn(guard, dir)
	if lerr != nil {
		for _, e := range prunable {
			warns = append(warns, "stale registration "+e.path+": deregistration unverifiable after prune: "+errText(lerr))
		}
		return nil, warns
	}
	for _, e := range prunable {
		if set[e.path] {
			warns = append(warns, "stale registration "+e.path+" is still registered after prune (not counted as cleared)")
			continue
		}
		cleared = append(cleared, e)
	}
	return cleared, warns
}

// staleRegSet renders the cleared list as a set, so the candidate loop can skip entries
// this pass already accounted for (they are gone from the registration in a live sweep,
// and spoken for in a dry run) instead of reporting each a second time as an
// unverifiable hold.
func staleRegSet(entries []staleRegEntry) map[string]bool {
	if len(entries) == 0 {
		return nil
	}
	set := make(map[string]bool, len(entries))
	for _, e := range entries {
		set[e.path] = true
	}
	return set
}

// staleRegLine is the per-entry report row, in both modes: a dry run says what it WOULD
// clear, a live sweep what it DID.
func staleRegLine(e staleRegEntry, dryRun bool) string {
	verb := "cleared stale registration "
	if dryRun {
		verb = "would clear stale registration "
	}
	line := verb + e.path + " — worktree directory exists but its .git file is gone (directory left on disk, untouched)"
	if e.wasLocked {
		if dryRun {
			line += "; locked — the lock would be retired with the registration"
		} else {
			line += "; its lock was retired with the registration"
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
