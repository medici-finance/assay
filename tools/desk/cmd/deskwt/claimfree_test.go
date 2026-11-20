package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- fixtures ------------------------------------------------------------------

// runKeyed records a run key in a fixture home exactly where deskdispatch records one: the
// worktree-scoped assay.runKey (addWorktree has already enabled the worktree-config
// extension), with the claim namespace deskdispatch records beside it for an ordinary dispatch:
// this checkout's origin, in the forge-ref store.
func runKeyed(t *testing.T, target, key string) {
	t.Helper()
	repo, err := currentRepo(target)
	if err != nil {
		t.Fatal(err)
	}
	runKeyedIn(t, target, key, repo, deskkit.ClaimStoreForgeRef)
}

// runKeyedIn records a run key with an explicit claim namespace; "" leaves that key unrecorded.
func runKeyedIn(t *testing.T, target, key, claimRepo, claimStore string) {
	t.Helper()
	mustGit(t, target, "config", "--worktree", "assay.runKey", key)
	if claimRepo != "" {
		mustGit(t, target, "config", "--worktree", "assay.claimRepo", claimRepo)
	}
	if claimStore != "" {
		mustGit(t, target, "config", "--worktree", "assay.claimStore", claimStore)
	}
}

// bareLock is the lock an agent writes on its own home at boot: no reason, no session.
func bareLock(t *testing.T, work, target string) {
	t.Helper()
	mustGit(t, work, "worktree", "lock", target)
}

// holdClaim plants key's dispatch claim on the fixture's local bare origin, the namespace the
// dispatcher acquires claims in. Offline, and visible to `git ls-remote origin`.
func holdClaim(t *testing.T, work, key string) {
	t.Helper()
	src := mustGit(t, work, "rev-parse", "origin/main")
	dst := deskkit.DispatchClaimActiveRefsPrefix + key
	mustGit(t, work, "push", "--quiet", "origin", src+":"+dst)
}

// agedOut backdates a home's admin entry past the just-created safety window, so the
// ordinary arm judges it on its content rather than its age.
func agedOut(t *testing.T, work, target string) {
	t.Helper()
	old := time.Now().Add(-2 * freshWorktreeSafetyWindow)
	admin := filepath.Join(work, ".git", "worktrees", filepath.Base(target))
	if err := os.Chtimes(admin, old, old); err != nil {
		t.Fatal(err)
	}
}

func gone(t *testing.T, target, errout string) {
	t.Helper()
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("%s survived (err=%v); stderr:\n%s", target, err, errout)
	}
}

// --- the reap arm ----------------------------------------------------------------

// A bare-locked home whose claim is released, clean and pushed, is reaped. Pre-fix the bare
// lock could never be proven stale, so this tree was held forever.
// regression: #6847
func TestReapReleasedBareLock(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "relbare")
	runKeyed(t, target, "item-a")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
	if localBranchExists(t, work, branch) {
		t.Fatalf("branch %s survived the reap; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dispatch claim item-a is released") {
		t.Fatalf("the reap did not name the released claim; stderr:\n%s", errout)
	}
}

// The released claim retires the owner, never the safety gates: an untracked file holds it.
// regression: #6847
func TestReapReleasedKeepsDirty(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "reldirty")
	runKeyed(t, target, "item-b")
	bareLock(t, work, target)
	writeFile(t, target+"/scratch.txt", "unsaved\n")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
}

// An UNLOCKED home whose claim is held is a live run: held, never "unowned". Pre-fix the reap
// arm counted every unlocked clean tree as unowned and removed it under a running agent.
// regression: #6847
func TestReapHeldClaimKeepsTree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "heldopen")
	runKeyed(t, target, "item-c")
	holdClaim(t, work, "item-c")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("a held run's branch %s was deleted; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dispatch claim item-c is held") {
		t.Fatalf("the hold did not name the claim; stderr:\n%s", errout)
	}
}

// An unreadable claim listing is could-not-check, which holds: never a released claim.
// regression: #6847
func TestReapClaimUnreadableHolds(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	old := listDispatchClaims
	listDispatchClaims = func(string) (map[string]bool, error) {
		return nil, errors.New("forge unreachable")
	}
	t.Cleanup(func() { listDispatchClaims = old })
	target, _ := openPRWorktree(t, work, "noread")
	runKeyed(t, target, "item-d")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "could-not-check: dispatch claim item-d") {
		t.Fatalf("the hold did not say could-not-check; stderr:\n%s", errout)
	}
}

// --- the ordinary arm ------------------------------------------------------------

// A merged, unlocked home whose claim is held is held by the ordinary sweep too, and says
// why. Pre-fix the ordinary arm never consulted the claim.
// regression: #6847
func TestPruneHeldClaimKeepsTree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "heldmerged")
	agedOut(t, work, target)
	runKeyed(t, target, "item-e")
	holdClaim(t, work, "item-e")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "dispatch claim item-e is held") {
		t.Fatalf("the hold did not name the claim; stderr:\n%s", errout)
	}
}

// --- remove ----------------------------------------------------------------------

// `remove` of a bare-locked home whose claim is released succeeds. Pre-fix the lock was
// refused as another session's, so every finished home needed a hand unlock.
// regression: #6847
func TestRemoveReleasedBareLock(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "rmrel")
	runKeyed(t, target, "item-f")
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// `remove` of a home whose claim is held is refused, locked or not. Pre-fix an unlocked
// live home was removed.
// regression: #6847
func TestRemoveHeldClaimRefused(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "rmheld")
	runKeyed(t, target, "item-g")
	holdClaim(t, work, "item-g")

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "dispatch claim item-g is held") {
		t.Fatalf("the refusal did not name the claim; stderr:\n%s", errout)
	}
}

// A home with no run key never reads the claim listing: undispatched trees are unchanged.
// regression: #6847
func TestNoRunKeyNoClaimRead(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	old := listDispatchClaims
	reads := 0
	listDispatchClaims = func(string) (map[string]bool, error) {
		reads++
		return nil, errors.New("must not be read")
	}
	t.Cleanup(func() { listDispatchClaims = old })
	target := addWorktree(t, "nokey")
	agedOut(t, work, target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
	if reads != 0 {
		t.Fatalf("claim listing read %d times for a tree with no run key", reads)
	}
}
