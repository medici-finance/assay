package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- a nested claim key is still a claim (SEC-5) ------------------------------------

// nestedKey is a key the claim tool accepts (it carries `--`, and `/` is a legal ref
// separator), written at refs/dispatch/<key> like any other.
const nestedKey = "item--stream/07"

// A held claim under a nested key keeps its bare-locked home in the reap arm. Pre-fix the
// listing dropped nested refs, so the key read as released and the home was reaped.
// regression: #6863
func TestReapNestedKeyHeld(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "nestreap")
	runKeyed(t, target, nestedKey)
	holdClaim(t, work, nestedKey)
	bareLock(t, work, target)

	// The plan names the reason: the claim, not some other hold.
	rc, plan := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, plan)
	}
	if !strings.Contains(plan, "dispatch claim "+nestedKey+" is held") {
		t.Fatalf("the plan did not hold the tree on the nested claim; stderr:\n%s", plan)
	}

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("a held run's branch %s was deleted; stderr:\n%s", branch, errout)
	}
}

// `remove` refuses a bare-locked home whose nested-key claim is held.
// regression: #6863
func TestRemoveNestedKeyHeld(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "nestrm")
	runKeyed(t, target, nestedKey)
	holdClaim(t, work, nestedKey)
	bareLock(t, work, target)

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitRefused {
		t.Fatalf("remove rc = %d, want %d; stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "dispatch claim "+nestedKey+" is held") {
		t.Fatalf("the refusal did not name the nested claim; stderr:\n%s", errout)
	}
}

// --- a broken sibling admin entry does not hold every tree (F8) ----------------------

// brokenLockedSibling adds a second worktree, locks it (so git's bookkeeping prune keeps its
// entry) and empties its admin gitdir pointer, which makes the sweep's admin inventory
// incomplete.
func brokenLockedSibling(t *testing.T, work string) string {
	t.Helper()
	sib := addWorktree(t, "brokensib")
	agedOut(t, work, sib)
	bareLock(t, work, sib)
	writeFile(t, filepath.Join(work, ".git", "worktrees", filepath.Base(sib), "gitdir"), "")
	return sib
}

// The ordinary arm removes a merged, undispatched tree although a sibling's admin entry
// cannot be inventoried. Pre-fix the claim gate held it: "could-not-check: run key".
// regression: #6863
func TestPruneBrokenSiblingNoRunKey(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "sibprune")
	agedOut(t, work, target)
	brokenLockedSibling(t, work)
	// Merged but behind the tip, so the at-tip age gate (which needs the whole inventory, and
	// predates this change) is not what decides the tree.
	advanceMain(t, work, "sibprune")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// The reap arm reaps an unlocked, undispatched, pushed tree under the same broken sibling.
// regression: #6863
func TestReapBrokenSiblingNoRunKey(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "sibreap")
	brokenLockedSibling(t, work)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	gone(t, target, errout)
}

// Under the same broken sibling a dispatched home whose claim is HELD is still held: the run
// record comes from its own confirmed entry, and the claim decides.
// regression: #6863
func TestReapBrokenSiblingHeldClaim(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "sibheld")
	runKeyed(t, target, "item-s")
	holdClaim(t, work, "item-s")
	brokenLockedSibling(t, work)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !strings.Contains(errout, "dispatch claim item-s is held") {
		t.Fatalf("the hold did not name the claim; stderr:\n%s", errout)
	}
}

// Unit: with the inventory incomplete, claimFor reads the candidate's own entry, and only an
// index-wide error takes that path (a different common store still holds).
// regression: #6863
func TestSweepClaimOwnEntryFallback(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "ownentry")
	guard, err := newPathGuard(work)
	if err != nil {
		t.Fatal(err)
	}
	broken := func() *worktreeAdminIndex {
		return &worktreeAdminIndex{paths: map[string]string{}, ambiguous: map[string]bool{}, err: errors.New("one entry unreadable")}
	}
	rt := resolvePath(target)

	sc := &sweepCtx{commonDir: guard.commonDir, admins: broken()}
	if c := sc.claimFor(guard, work, rt); c.holds() || c.key != "" {
		t.Fatalf("a confirmed own entry with no run key must not hold; got %+v", c)
	}

	runKeyed(t, target, "item-o")
	sc = &sweepCtx{commonDir: guard.commonDir, admins: broken()}
	if c := sc.claimFor(guard, work, rt); c.key != "item-o" || c.state != claimFree {
		t.Fatalf("the own entry's run record must be read; got %+v", c)
	}

	amb := broken()
	amb.ambiguous[rt] = true
	sc = &sweepCtx{commonDir: guard.commonDir, admins: amb}
	if c := sc.claimFor(guard, work, rt); !c.holds() || c.state != claimUnknown {
		t.Fatalf("a tree the readable index names twice must hold; got %+v", c)
	}

	sc = &sweepCtx{commonDir: guard.commonDir + "-other", admins: broken()}
	if c := sc.claimFor(guard, work, rt); !c.holds() || c.state != claimUnknown {
		t.Fatalf("an index of another common store must hold; got %+v", c)
	}
}
