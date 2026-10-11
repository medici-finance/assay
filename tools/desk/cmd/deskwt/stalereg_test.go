package main

// Tests for the two halves of the detached-at-main drain fix:
//
//   - the CONTENT-AWARE fresh-worktree guard (prune.go): a pristine worktree whose HEAD
//     IS the origin/main tip — the shape a reclaim-by-detach leaves behind — is removed,
//     while one holding uncommitted or untracked work is still held;
//   - the STALE-REGISTRATION sweep (stalereg.go): a registration whose directory exists
//     but whose .git gitdir file is gone is cleared — the locked variant included — with
//     the directory left on disk, untouched.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- the core defect: a pristine detached-at-main worktree is REMOVED ----------------
//
// Reclaim-by-detach leaves a worktree HEAD-detached exactly at the origin/main tip with
// no commits and no files of its own. The blanket fresh-worktree guard held that shape
// forever, so the residue grew without bound. Every byte of a pristine at-tip tree
// derives from origin/main, so removing it is provably lossless — the sweep must remove
// it (and drop the registration), exactly as it already does for merged-below-tip trees.
func TestPruneRemovesPristineDetachedAtMainTip(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target := addWorktree(t, "pristine-detached")
	// Detach AT the current tip: HEAD == refs/remotes/origin/main exactly, tree pristine.
	mustGit(t, target, "checkout", "--detach", "HEAD")

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
// session never wrote into): it too drains now rather than waiting for the mainline to
// move past it.
func TestPruneRemovesPristineBranchedAtMainTip(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "pristine-branched")

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

// --- the safety properties are unchanged ---------------------------------------------

// A dirty detached-at-main worktree is HELD: the untracked-inclusive gate sees the
// tracked modification and the fresh-worktree reason is recorded.
func TestPruneHoldsDirtyDetachedAtMainTip(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "dirty-detached")
	mustGit(t, target, "checkout", "--detach", "HEAD")
	writeFile(t, filepath.Join(target, "README.md"), "seed\nmodified\n")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "fresh worktree at origin/main") {
		t.Fatalf("expected the fresh-worktree hold reason; got:\n%s", errout)
	}
	if strings.Contains(errout, "removed 1") {
		t.Fatalf("prune removed a dirty detached-at-main worktree; stderr:\n%s", errout)
	}
}

// A LOCKED pristine detached-at-main worktree is HELD unconditionally: the lock gate runs
// ahead of every content gate, so even the newly-removable shape stays put while a session
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
// whichever bookkeeping step performs it. (The LOCKED shape, which git's prune keeps
// forever, is the next test.)
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

// The LOCKED variant is the immortal one: `git worktree prune` will not drop a locked
// entry however dead its tree is. The sweep retires the lock WITH the registration —
// the lock's object (a removable working tree) no longer exists — and still leaves the
// directory on disk.
func TestPruneClearsLockedStaleRegistration(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-locked")
	mustGit(t, work, "worktree", "lock", "--reason", "session died mid-cleanup", target)
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}
	// Fixture oracle: git itself confirms this shape survives its own prune while locked.
	mustGit(t, work, "worktree", "prune")
	if list := mustGit(t, work, "worktree", "list", "--porcelain"); !strings.Contains(list, target) {
		t.Fatalf("fixture drifted: git worktree prune dropped a LOCKED stale registration —\n%s", list)
	}

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	list := mustGit(t, work, "worktree", "list", "--porcelain")
	if strings.Contains(list, target) {
		t.Fatalf("locked stale registration survived the sweep:\n%s", list)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "stale-registrations-cleared 1") {
		t.Fatalf("expected the stale-registrations-cleared summary segment; got:\n%s", errout)
	}
	if !strings.Contains(errout, "its lock was retired with the registration") {
		t.Fatalf("expected the retired-lock note on the report line; got:\n%s", errout)
	}
}

// A dry run REPORTS the stale registration it would clear and mutates nothing: no unlock,
// no deregistration, .git/worktrees byte-identical.
func TestPruneDryRunReportsStaleRegistrationWithoutClearing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-dry")
	mustGit(t, work, "worktree", "lock", "--reason", "locked residue", target)
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
	// The lock, specifically, must survive a dry run: a report that retires a lock is not
	// a report.
	if list := mustGit(t, work, "worktree", "list", "--porcelain"); !strings.Contains(list, "locked") {
		t.Fatalf("--dry-run retired a lock:\n%s", list)
	}
}

// A stale registration must not ALSO be reported as a candidate-loop hold: the sweep
// accounts for every registered path exactly once.
func TestPruneStaleRegistrationNotDoubleReported(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "stale-once")
	if err := os.Remove(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("remove .git file: %v", err)
	}

	rc, errout := runCapErr(t, []string{"prune", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if strings.Contains(errout, "skipped "+resolvePath(target)) {
		t.Fatalf("stale registration was ALSO reported as a candidate-loop skip; got:\n%s", errout)
	}
}
