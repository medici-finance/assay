package main

// Tests for the two halves of the detached-at-main drain fix and its review-hardening:
//
//   - the CONTENT-AWARE fresh-worktree guard (prune.go): a pristine worktree whose HEAD
//     IS the origin/main tip drains only when it is also OLD enough (past the
//     fresh-worktree safety window) and carries no git-ignored payload;
//   - the STALE-REGISTRATION sweep (stalereg.go): a registration whose directory exists
//     but whose .git gitdir file is gone is cleared only when it passes the scope gates
//     every sweep arm enforces, is not locked behind a lock the default sweep may not
//     touch, and its admin HEAD can be proven to lose nothing (branch-pointing, or a
//     commit already on the remote mainline).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ageWorktreeAdmin backdates a worktree's admin entry past the fresh-worktree safety
// window, so a test's pristine at-tip tree is old enough to drain and ONLY the gate under
// test can hold it.
func ageWorktreeAdmin(t *testing.T, work, target string) {
	t.Helper()
	admin := filepath.Join(work, ".git", "worktrees", filepath.Base(target))
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(admin, old, old); err != nil {
		t.Fatalf("age admin entry %s: %v", admin, err)
	}
}

// --- the core defect: a pristine detached-at-main worktree is REMOVED ----------------
//
// Reclaim-by-detach leaves a worktree HEAD-detached exactly at the origin/main tip with
// no commits and no files of its own. The blanket fresh-worktree guard held that shape
// forever, so the residue grew without bound. A pristine at-tip tree old enough to be no
// session's just-created workspace is removed (and deregistered), exactly as
// merged-below-tip trees already were.
func TestPruneRemovesPristineDetachedAtMainTip(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target := addWorktree(t, "pristine-detached")
	// Detach AT the current tip: HEAD == refs/remotes/origin/main exactly, tree pristine.
	mustGit(t, target, "checkout", "--detach", "HEAD")
	ageWorktreeAdmin(t, work, target) // past the fresh-worktree safety window

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("pristine detached-at-main worktree %s survived the sweep (err=%v); stderr:\n%s", target, err, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("worktree still registered after prune:\n%s", list)
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("expected summary to report 1 removal; got:\n%s", errout)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during prune: %v", gitCalls(*calls))
	}
}

// A pristine BRANCHED worktree at the tip is the same lossless shape (a `deskwt add` the
// session never wrote into): it too drains once it is old enough, rather than waiting for
// the mainline to move past it.
func TestPruneRemovesPristineBranchedAtMainTip(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "pristine-branched")
	ageWorktreeAdmin(t, work, target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("pristine branched at-tip worktree %s survived the sweep (err=%v); stderr:\n%s", target, err, errout)
	}
	if !strings.Contains(errout, "removed 1 merged+clean") {
		t.Fatalf("expected summary to report 1 removal; got:\n%s", errout)
	}
}

// --- the fresh-worktree safety window (review finding F4) -----------------------------
//
// `deskwt add` never locks the new worktree, so a just-created pristine at-tip tree —
// the verifier/reviewer shape — is indistinguishable from reclaim residue by content
// alone. A tree whose admin entry is younger than the safety window (the roster's
// one-hour freshness window) is HELD as too fresh; only an aged one drains.
func TestPruneHoldsFreshPristineAtTipWithinSafetyWindow(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "just-made")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	// NOT aged: the admin entry is seconds old, inside the safety window.

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "fresh worktree at origin/main") || !strings.Contains(errout, "safety window") {
		t.Fatalf("expected the too-fresh hold reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a just-created worktree; stderr:\n%s", errout)
	}
}

// --- the safety properties are unchanged ---------------------------------------------

// A dirty detached-at-main worktree is HELD: aged past the safety window, so ONLY the
// untracked-inclusive content gate can be what holds it.
func TestPruneHoldsDirtyDetachedAtMainTip(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "dirty-detached")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	writeFile(t, filepath.Join(target, "README.md"), "seed\nmodified\n")
	ageWorktreeAdmin(t, work, target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "fresh worktree at origin/main") ||
		!strings.Contains(errout, "uncommitted or untracked") {
		t.Fatalf("expected the content-gate hold reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a dirty detached-at-main worktree; stderr:\n%s", errout)
	}
}

// An at-tip tree carrying git-IGNORED payload (no tracked or untracked changes) is HELD:
// the ignored file exists nowhere but this directory, so removing the tree is not
// lossless. Aged past the safety window, so ONLY the ignored-payload gate can hold it.
func TestPruneHoldsAtTipTreeWithIgnoredPayload(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// The ignore rule must be committed on the mainline BEFORE the worktree is cut,
	// else the .gitignore itself would be untracked content in the worktree.
	writeFile(t, filepath.Join(work, ".gitignore"), "scratch/\n")
	mustGit(t, work, "add", ".gitignore")
	mustGit(t, work, "commit", "-m", "ignore scratch")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))

	target := addWorktree(t, "ignored-payload")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	writeFile(t, filepath.Join(target, "scratch", "notes.txt"), "session scratch\n")
	ageWorktreeAdmin(t, work, target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "fresh worktree at origin/main") || !strings.Contains(errout, "ignored payload") {
		t.Fatalf("expected the ignored-payload hold reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed an at-tip worktree holding git-ignored payload; stderr:\n%s", errout)
	}
	if _, err := os.Stat(filepath.Join(target, "scratch", "notes.txt")); err != nil {
		t.Fatalf("ignored payload did not survive the sweep: %v", err)
	}
}

// A LOCKED pristine detached-at-main worktree is HELD unconditionally: the lock gate runs
// ahead of every content gate, so even the drainable shape stays put while a session
// claims it.
func TestPruneHoldsLockedDetachedAtMainTip(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "locked-detached")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	lockReason := "fixture lock (a live session would hold this)"
	mustGit(t, work, "worktree", "lock", "--reason", lockReason, target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "locked ("+lockReason+")") {
		t.Fatalf("expected a visible lock skip reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a LOCKED worktree; stderr:\n%s", errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, target) {
		t.Fatalf("locked worktree no longer registered after prune (half-destroyed):\n%s", list)
	}
}

// --- the stale-registration sweep -----------------------------------------------------

// A registration whose directory exists but whose .git file is gone is CLEARED — and only
// the registration: the directory stays on disk, untouched. On a git new enough to call
// the entry "prunable: gitdir file points to non-existent location" Step A's own
// bookkeeping prune drops the UNLOCKED shape; the sweep's contract is the outcome —
// registration gone, directory intact, and never reported as a content-gate hold —
// whichever bookkeeping step performs it.
func TestPruneClearsStaleRegistrationMissingGitFile(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-gitfile")
	// The half-dead shape: the tree survives, the git link does not.
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	// The REGISTRATION is gone...
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("stale registration survived the sweep:\n%s", list)
	}
	// ...the DIRECTORY is not.
	assertExists(t, target)
	// ...the sweep SAID it cleared it (Step A's bookkeeping count or Step A1's segment)...
	if !strings.Contains(errout, "stale-registrations-cleared 1") && !strings.Contains(errout, "pruned 1 bookkeeping") {
		t.Fatalf("the clearing appears in no summary count; got:\n%s", errout)
	}
	// ...and it was never mistaken for a content-gate hold.
	if strings.Contains(errout, "skipped "+resolvePath(target)) {
		t.Fatalf("stale registration was reported as a candidate-loop skip; got:\n%s", errout)
	}
}

// The DEFAULT sweep never retires a lock — not even the lock on a stale registration.
// A locked husk stays registered AND locked, and is reported held with the reason named.
// (Review finding S1: unlocking is --reclaim-stale-locks' opt-in, gated on judgeLock's
// proven-staleness evidence — never a default-sweep act.)
func TestPruneHoldsLockedStaleRegistrationByDefault(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-locked")
	mustGit(t, work, "worktree", "lock", "--reason", "session died mid-cleanup", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, target) {
		t.Fatalf("default sweep cleared a LOCKED stale registration:\n%s", list)
	}
	if !strings.Contains(list, "locked") {
		t.Fatalf("default sweep retired the lock on a stale registration:\n%s", list)
	}
	assertExists(t, target)
	// The hold must come from the LOCK RULE with its reason named — not from git's own
	// prune keeping the locked entry (which would leave the registration intact but
	// report nothing). A sweep that dropped the rule would print the phase-2 warning
	// instead; both assertions together make that regression visible.
	if !strings.Contains(errout, "the default sweep never retires a lock") {
		t.Fatalf("the lock-rule hold reason is missing; got:\n%s", errout)
	}
	if strings.Contains(errout, "still registered after prune") {
		t.Fatalf("the locked husk was handed to `git worktree prune` at all; got:\n%s", errout)
	}
}

// With --reclaim-stale-locks and a lock PROVEN stale (the named session has no roster
// beacon), the husk drains: the reclaim pass (the one place unlocks may happen) retires
// the lock and the registration is cleared — the directory still left on disk.
func TestPruneClearsStaleRegistrationAfterProvenStaleReclaim(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-reclaimable")
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session=dead-3)", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	// The roster is READABLE but holds no beacon for dead-3 — beaconAbsent, so the lock
	// is proven stale. (An unreadable roster would prove nothing and hold it.)
	plantBeacon(t, "someone-else", time.Now())

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("stale registration behind a PROVEN-stale lock survived a reclaiming sweep:\n%s", list)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("expected the proven-stale lock to be reclaimed; got:\n%s", errout)
	}
}

// git's own in-flight `worktree add` holds the new worktree's lock with the reason
// "initializing". That lock is NEVER retired — not by the age fallback, not by any sweep
// arm: retiring it mid-add would race git itself.
func TestPruneNeverRetiresInitializingLock(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-initializing")
	mustGit(t, work, "worktree", "lock", "--reason", "initializing", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	// Age the lock past any TTL so ONLY the initializing exemption can hold it.
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(lockAdminFile(work, target), old, old); err != nil {
		t.Fatalf("age lock file: %v", err)
	}

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks", "--lock-ttl", "1ns"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, target) {
		t.Fatalf("a sweep cleared a registration locked with git's in-flight add lock:\n%s", list)
	}
	if !strings.Contains(list, "locked") {
		t.Fatalf("a sweep retired git's in-flight \"initializing\" lock:\n%s", list)
	}
	if strings.Contains(errout, "locks-reclaimed 1") {
		t.Fatalf("the \"initializing\" lock was reclaimed; got:\n%s", errout)
	}
}

// --- Step A1's scope gates (review finding S1/F2) --------------------------------------
//
// A stale registration OUTSIDE the sanctioned prefixes is none of the sweep's business:
// never unlocked, never deregistered, whatever its state.
func TestPruneStaleRegOutOfPrefixUntouched(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// Register a worktree in an arbitrary temp dir — outside every sanctioned prefix.
	outside := filepath.Join(t.TempDir(), "not-a-tracker")
	mustGit(t, work, "worktree", "add", "--detach", outside, "HEAD")
	mustGit(t, work, "worktree", "lock", "--reason", "out-of-prefix residue", outside)
	if err := os.Remove(filepath.Join(outside, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	for _, args := range [][]string{
		{"prune"},
		{"prune", "--reclaim-stale-locks", "--lock-ttl", "1ns"},
	} {
		rc, errout := runCapErr(t, args)
		if rc != deskkit.ExitOK {
			t.Fatalf("prune %v rc = %d, want 0; stderr:\n%s", args, rc, errout)
		}
		list := mustGit(t, work, "worktree", "list", "--porcelain")
		if !strings.Contains(list, outside) {
			t.Fatalf("sweep %v deregistered an out-of-prefix worktree:\n%s", args, list)
		}
		if !strings.Contains(list, "locked") {
			t.Fatalf("sweep %v retired an out-of-prefix lock:\n%s", args, list)
		}
		// The sweep must account for it as the CANDIDATE LOOP's lock hold — the
		// stale-registration pass must not have judged it at all (a dropped scope gate
		// would route it into phase 2, which reports it with a stale-registration reason).
		if !strings.Contains(errout, "locked (out-of-prefix residue)") {
			t.Fatalf("sweep %v: expected the candidate loop's lock hold for the out-of-prefix husk; got:\n%s", args, errout)
		}
		if strings.Contains(errout, "stale registration") {
			t.Fatalf("sweep %v: the stale-registration pass judged an out-of-prefix entry (scope gate dropped); got:\n%s", args, errout)
		}
	}
	assertExists(t, outside)
}

// --- Step A1's content gate (review finding S1/F3) -------------------------------------
//
// Clearing a registration deletes its admin dir (.git/worktrees/<id>/), and for a
// DETACHED HEAD that admin dir's HEAD file is the only ref to the commit. A detached husk
// whose commit is not on the remote mainline is HELD — clearing it would strand
// possibly-unpushed work — even across --reclaim-stale-locks: the reclaim pass must not
// unlock what the content gate holds (the post-reclaim bookkeeping prune would drop it).
func TestPruneHoldsStaleRegWithUnpushedDetachedHead(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-unpushed")
	// An unpushed commit, then DETACH onto it and delete the branch, so the commit is
	// reachable from no ref but the worktree's admin HEAD.
	writeFile(t, filepath.Join(target, "unpushed.txt"), "unpushed\n")
	mustGit(t, target, "add", "unpushed.txt")
	mustGit(t, target, "commit", "-m", "unpushed husk work")
	sha := mustGit(t, target, "rev-parse", "HEAD")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	mustGit(t, work, "branch", "-D", "stale-unpushed")
	// POSITIVE CONTROL: the commit really is reachable from no branch and not from the
	// mainline — only the husk's admin HEAD will name it.
	if branches := mustGit(t, work, "branch", "--contains", sha); branches != "" {
		t.Fatalf("positive control void: unpushed commit is on a branch: %s", branches)
	}
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session=dead-4)", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	plantBeacon(t, "someone-else", time.Now()) // roster readable, dead-4 absent → provable-stale lock

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, target) {
		t.Fatalf("a sweep cleared a stale registration whose detached HEAD is unpushed (commit %s stranded):\n%s", sha, list)
	}
	if !strings.Contains(list, "locked") {
		t.Fatalf("the reclaim pass unlocked a husk the content gate must hold:\n%s", list)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "not reachable from refs/remotes/origin/main") {
		t.Fatalf("expected the unpushed-detached-HEAD hold reason; got:\n%s", errout)
	}
	// The commit must still be recoverable — the admin HEAD is intact.
	if catErr := mustGit(t, work, "cat-file", "-t", sha); catErr != "commit" {
		t.Fatalf("the unpushed commit is gone from the object store: %v", catErr)
	}
}

// A husk whose admin HEAD points at a BRANCH loses nothing when its registration is
// cleared: the branch ref holds the commits. This one is cleared once its lock is proven
// stale.
func TestPruneClearsBranchPointingStaleRegistration(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-branched")
	// A commit on the worktree's branch — unpushed, but held by the branch ref, so
	// clearing the REGISTRATION strands nothing.
	writeFile(t, filepath.Join(target, "work.txt"), "branch work\n")
	mustGit(t, target, "add", "work.txt")
	mustGit(t, target, "commit", "-m", "branch-held work")
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session=dead-5)", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	plantBeacon(t, "someone-else", time.Now())

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("branch-pointing stale registration survived a reclaiming sweep:\n%s", list)
	}
	assertExists(t, target)
	// The branch — and with it the commit — survives the clearing.
	if branches := mustGit(t, work, "branch", "--list", "stale-branched"); branches == "" {
		t.Fatalf("the husk's branch was deleted with its registration")
	}
}

// A dry run REPORTS the stale registration it would clear and mutates nothing: no unlock,
// no deregistration, .git/worktrees byte-identical. The husk here is UNLOCKED (a locked
// one is not clearable at all — the default sweep never retires a lock, and a dry run
// reports would-reclaims without performing them).
func TestPruneDryRunReportsStaleRegistrationWithoutClearing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-dry")
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	adminDir := filepath.Join(work, ".git", "worktrees")
	before := hashTree(t, adminDir)

	rc, errout := runCapErr(t, []string{"prune", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if after := hashTree(t, adminDir); after != before {
		t.Fatalf("--dry-run mutated .git/worktrees/ (%s -> %s); stderr:\n%s", before[:12], after[:12], errout)
	}
	if !strings.Contains(errout, "would clear stale registration "+resolvePath(target)) {
		t.Fatalf("dry run did not report the stale registration it would clear; got:\n%s", errout)
	}
	if !strings.Contains(errout, "stale-registrations-cleared 1") {
		t.Fatalf("dry run did not count the would-clear in the summary; got:\n%s", errout)
	}
}

// A stale registration must not ALSO be reported as a candidate-loop hold: the sweep
// accounts for every registered path exactly once, whether the verdict was clear or hold.
func TestPruneStaleRegistrationNotDoubleReported(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	cleared := addWorktree(t, "stale-once")
	if err := os.Remove(filepath.Join(cleared, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	held := addWorktree(t, "stale-once-held")
	mustGit(t, work, "worktree", "lock", "--reason", "held husk", held)
	if err := os.Remove(filepath.Join(held, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	rc, errout := runCapErr(t, []string{"prune", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	// The clearable one is reported ONCE, by Step A1 — never as a candidate-loop skip
	// with a misleading "unverifiable" reason. (The separator is part of the match: a
	// longer sibling name must not read as this path.)
	if strings.Contains(errout, "skipped "+resolvePath(cleared)+" — ") {
		t.Fatalf("cleared stale registration was ALSO reported as a candidate-loop skip; got:\n%s", errout)
	}
	// The held one is reported ONCE, with its stale-registration reason — never twice.
	if strings.Count(errout, resolvePath(held)) != 1 {
		t.Fatalf("held stale registration was reported %d times, want 1; got:\n%s",
			strings.Count(errout, resolvePath(held)), errout)
	}
}

// --- N2: a failed clear restores the lock it found ------------------------------------
//
// The re-lock mechanism, unit-level: an entry the reclaim pass unlocked this sweep gets
// its lock back with the EXACT original reason when the clear fails.
func TestRelockReclaimedHusksRestoresOriginalReason(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "relock-unit")
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatalf("newPathGuard: %v", err)
	}
	reason := "worker-desk live session (deskwt role-init session=dead-9)"
	warns := relockReclaimedHusks(guard, work,
		[]staleRegEntry{{path: resolvePath(target)}},
		map[string]string{resolvePath(target): reason})
	if len(warns) != 1 || !strings.Contains(warns[0], "restored") {
		t.Fatalf("expected one restored-lock warning; got: %v", warns)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if !strings.Contains(list, "locked "+reason) {
		t.Fatalf("lock not restored with the original reason:\n%s", list)
	}
	// An entry the sweep found UNLOCKED (never reclaimed) has nothing to restore.
	warns = relockReclaimedHusks(guard, work,
		[]staleRegEntry{{path: resolvePath(target)}}, nil)
	if len(warns) != 0 {
		t.Fatalf("an un-reclaimed entry must not be re-locked; got: %v", warns)
	}
}

// The wiring: when the verification listing reports a cleared-intended husk as STILL
// registered, the sweep does not count it cleared AND attempts the re-lock of the lock
// the reclaim pass retired — and SAYS which way the re-lock went, never leaving the
// state silent.
func TestPruneRelocksHuskWhenClearFails(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-relock")
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session=dead-10)", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	plantBeacon(t, "someone-else", time.Now()) // lock is provably stale → reclaimed

	// Force the phase-2 verification listing to report the husk as still registered —
	// the clear-failed path (the seam TestPruneBatchDemotesStrandedPath established).
	rt := resolvePath(target)
	orig := worktreePathsFn
	worktreePathsFn = func(g *pathGuard, dir string) (map[string]bool, error) {
		set, err := orig(g, dir)
		if err != nil {
			return nil, err
		}
		set[rt] = true
		return set, nil
	}
	t.Cleanup(func() { worktreePathsFn = orig })

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !strings.Contains(errout, "still registered after prune (not counted as cleared)") {
		t.Fatalf("expected the not-cleared warning; got:\n%s", errout)
	}
	// The re-lock was attempted; the sweep must SAY which way it went. (On a modern git
	// the post-reclaim bookkeeping prune has already dropped the entry, so the re-lock
	// itself reports failure — the assertion is on the NAMED outcome, not its direction.)
	if !strings.Contains(errout, rt) ||
		!(strings.Contains(errout, "its lock was restored") || strings.Contains(errout, "lock could not be restored")) {
		t.Fatalf("expected a named re-lock outcome for the failed clear; got:\n%s", errout)
	}
}
