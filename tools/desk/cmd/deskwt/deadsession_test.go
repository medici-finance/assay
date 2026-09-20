package main

// deadsession_test.go — the DEAD-SESSION reap arm.
//
// The defect these cover: a desk session that dies leaves its worktree registered forever,
// in one of two shapes the ordinary sweep can never clear — LOCKED by the dead session, or
// unlocked but sitting on an unmerged (open-PR) branch the merge gate reads as active work.
// Every later resume of such a branch then fails at worktree-create, because git permits one
// worktree per branch, so the leftovers wedge the very queue that produced them.
//
// The invariant that matters most is the one the fixtures spend the most lines on: the arm
// removes a tree ONLY when nothing live owns it AND removing it loses nothing. A live
// session's lock, a dirty tree (untracked files counted), an unpushed commit, and anything
// unverifiable are each proof enough to hold, and each is held with its reason NAMED.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- fixtures ------------------------------------------------------------------

// headBranch reads the branch a worktree has checked out, straight from git — the tests
// never assume how `deskwt add` names it.
func headBranch(t *testing.T, target string) string {
	t.Helper()
	return mustGit(t, target, "rev-parse", "--abbrev-ref", "HEAD")
}

// localBranchExists asks git, not the tool's own report, whether a local ref survives. The
// stale local branch is half the defect: a reaped worktree whose branch ref is left behind
// leaves the next `deskwt add --branch` wedged one error message further on.
func localBranchExists(t *testing.T, work, branch string) bool {
	t.Helper()
	out := mustGit(t, work, "branch", "--list", branch)
	return strings.TrimSpace(out) != ""
}

// openPRWorktree builds the exact shape the incident is made of: a worktree carrying a
// commit that IS pushed to its own remote branch (so nothing local is at risk) but is NOT an
// ancestor of the remote mainline (so the merge gate reads it as active work and holds it
// forever). This is what a draft PR's worktree looks like after its session dies.
func openPRWorktree(t *testing.T, work, name string) (target, branch string) {
	t.Helper()
	target = addWorktree(t, name)
	branch = headBranch(t, target)
	writeFile(t, filepath.Join(target, "work.txt"), "landed work\n")
	mustGit(t, target, "add", "work.txt")
	mustGit(t, target, "commit", "-m", "work on the branch")
	// Push to the fixture's LOCAL bare origin: offline, and it gives the branch an upstream
	// of its own (refs/remotes/origin/<branch>) rather than the mainline it was cut from.
	mustGit(t, target, "push", "--quiet", "-u", "origin", branch)
	// March the mainline past it, so HEAD is neither at the tip nor an ancestor of it.
	advanceOriginMain(t, work)
	return target, branch
}

// lockAs locks a worktree with the reason shape role-init writes, naming a session.
func lockAs(t *testing.T, work, target, session string) {
	t.Helper()
	mustGit(t, work, "worktree", "lock", "--reason",
		"worker-desk live session (deskwt role-init session="+session+")", target)
}

// --- 1. locked by a DEAD session, open PR, clean and pushed → REAPED -------------
//
// The headline case, and the one the pre-fix sweep could not clear by ANY combination of the
// flags it had: --reclaim-stale-locks retires the lock, and the ordinary rules then hold the
// worktree anyway because its branch is unmerged. Only a rule that asks a different question
// — is anything LIVE still entitled to this tree, and is deleting it lossless — reaches it.
func TestReapLockedByDeadSession(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target, branch := openPRWorktree(t, work, "deadopenpr")
	lockAs(t, work, target, "dead-sess")
	plantBeacon(t, "dead-sess", time.Now().Add(-6*time.Hour)) // stopped reporting hours ago
	resetCalls(calls)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --reap-dead-sessions rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("a dead session's clean, pushed worktree %s survived the reap (err=%v); stderr:\n%s", target, err, errout)
	}
	if list := mustGit(t, work, "worktree", "list", "--porcelain"); strings.Contains(list, target) {
		t.Fatalf("worktree still registered after the reap:\n%s", list)
	}
	// The other half of the wedge: the stale local branch must go too, or the next
	// `deskwt add --branch` collides on the ref instead of on the worktree.
	if localBranchExists(t, work, branch) {
		t.Fatalf("stale local branch %s survived the reap; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 1") {
		t.Fatalf("expected the summary to count the reap; got:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-deleted 1") {
		t.Fatalf("expected the summary to count the branch deletion; got:\n%s", errout)
	}
	// A reap that could not be explained afterwards is indistinguishable from a bug: the
	// session it judged dead must be named.
	if !strings.Contains(errout, "session dead-sess") {
		t.Fatalf("the reap did not name the session it judged dead; got:\n%s", errout)
	}
	if !strings.Contains(errout, "REAP "+resolvePath(target)) {
		t.Fatalf("the plan row does not name the reaped worktree; got:\n%s", errout)
	}
	// The shared checkout is never a candidate, whatever else a sweep does.
	assertExists(t, work)
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during a reaping prune: %v", gitCalls(*calls))
	}
	// The branch delete is the NON-force form; git's own merged-into-upstream refusal is
	// the second, independent layer over our ancestry proof, and `-D` would remove it.
	for _, c := range gitCalls(*calls) {
		for i, a := range c {
			if a == "branch" && i+1 < len(c) && c[i+1] == "-D" {
				t.Fatalf("the reap deleted a branch with the FORCE form: %v", c)
			}
		}
	}
}

// --- 1b. the same worktree is UNTOUCHED without the opt-in ----------------------
//
// The arm is a new destructive capability, so a sweep that was not asked for it must behave
// exactly as it always has — including under the strongest flags the tool had before.
func TestDeadSessionOpenPRHeldWithoutOptIn(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "noopt")
	lockAs(t, work, target, "dead-sess-2")
	plantBeacon(t, "dead-sess-2", time.Now().Add(-6*time.Hour))

	rc, errout := runCapErr(t, []string{"prune", "--reclaim-stale-locks", "--lock-ttl", "1ns"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("branch %s was deleted without the opt-in; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 0") {
		t.Fatalf("a sweep without --reap-dead-sessions reaped something; got:\n%s", errout)
	}
}

// --- 2. locked by a LIVE session → KEPT ----------------------------------------
//
// The failure that would make the verb unusable is deleting the worktree somebody is working
// in. A beacon inside the freshness window is positive evidence the session is alive, and it
// must outrank everything: --lock-ttl 1ns here would judge ANY lock that fell through to the
// age test stale, so the hold is proof the live-session evidence won — and the tree is
// otherwise perfectly reapable (clean and pushed), so ONLY the liveness evidence holds it.
func TestReapHoldsLiveSession(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "livesess")
	lockAs(t, work, target, "alive-1")
	plantBeacon(t, "alive-1", time.Now())

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--lock-ttl", "1ns"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("a LIVE session's branch %s was deleted; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 0") {
		t.Fatalf("a LIVE session's worktree was reaped; got:\n%s", errout)
	}
	if !strings.Contains(errout, "locked-held 1") {
		t.Fatalf("expected the live lock to be reported as locked-held; got:\n%s", errout)
	}
}

// --- 3. unlocked and DIRTY → KEPT, with the reason ------------------------------

func TestReapHoldsUnlockedDirtyTracked(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, branch := openPRWorktree(t, work, "dirtytracked")
	writeFile(t, filepath.Join(target, "work.txt"), "edited, never committed\n")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("branch %s of a dirty worktree was deleted; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 0") {
		t.Fatalf("a dirty worktree was reaped; got:\n%s", errout)
	}
	if !strings.Contains(errout, "KEEP "+resolvePath(target)) {
		t.Fatalf("the plan does not report the dirty worktree as KEEP; got:\n%s", errout)
	}
	if !strings.Contains(errout, "dirty (uncommitted or untracked files present)") {
		t.Fatalf("the hold did not NAME the dirty reason; got:\n%s", errout)
	}
	// An unlocked worktree belongs to nobody: the plan must say so rather than leave the
	// column blank, so an operator can tell "no session" from "session unread".
	if !strings.Contains(errout, "session unowned") {
		t.Fatalf("the plan does not attribute the unlocked worktree as unowned; got:\n%s", errout)
	}
}

// The strictness that is NEW here, and the reason this arm does not reuse the ordinary
// sweep's tracked-only gate: an untracked file is reachable from no commit, so this
// directory is the only copy of it. The tracked-only gate calls such a tree clean.
func TestReapHoldsUnlockedUntrackedOnly(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target, _ := openPRWorktree(t, work, "untrackedonly")
	writeFile(t, filepath.Join(target, "new-source.go"), "package wip\n")

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if _, err := os.Stat(filepath.Join(target, "new-source.go")); err != nil {
		t.Fatalf("the only copy of an untracked source file was destroyed: %v", err)
	}
	if !strings.Contains(errout, "dead-session-reaped 0") {
		t.Fatalf("a worktree holding untracked new work was reaped; got:\n%s", errout)
	}
}

// --- 4. unlocked and UNPUSHED → KEPT, with the reason ---------------------------

func TestReapHoldsUnlockedUnpushed(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "unpushed")
	branch := headBranch(t, target)
	writeFile(t, filepath.Join(target, "local-only.txt"), "committed, never pushed\n")
	mustGit(t, target, "add", "local-only.txt")
	mustGit(t, target, "commit", "-m", "local commit that reached no remote")
	advanceOriginMain(t, work)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("branch %s carrying an unpushed commit was deleted; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 0") {
		t.Fatalf("a worktree with an unpushed commit was reaped; got:\n%s", errout)
	}
	if !strings.Contains(errout, "unpushed (HEAD is not reachable from its upstream") {
		t.Fatalf("the hold did not NAME the unpushed reason; got:\n%s", errout)
	}
}

// --- 5. clean and BEHIND its upstream → REAPED, local branch deleted ------------
//
// The branch carries nothing the remote does not already hold — HEAD is strictly behind the
// upstream ref — so both the tree and the ref are pure leftovers.
func TestReapCleanBehindUpstreamDeletesBranch(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target, branch := openPRWorktree(t, work, "behind")
	// Push a SECOND commit, then step HEAD back onto the first: the branch is now strictly
	// behind refs/remotes/origin/<branch>.
	writeFile(t, filepath.Join(target, "more.txt"), "a later push\n")
	mustGit(t, target, "add", "more.txt")
	mustGit(t, target, "commit", "-m", "second commit, pushed")
	mustGit(t, target, "push", "--quiet", "origin", branch)
	mustGit(t, target, "reset", "--hard", "HEAD~1")
	lockAs(t, work, target, "dead-sess-3")
	plantBeacon(t, "dead-sess-3", time.Now().Add(-6*time.Hour))
	resetCalls(calls)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("a clean worktree behind its upstream survived the reap (err=%v); stderr:\n%s", err, errout)
	}
	if localBranchExists(t, work, branch) {
		t.Fatalf("stale local branch %s survived; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 1") || !strings.Contains(errout, "branches-deleted 1") {
		t.Fatalf("expected one reap and one branch deletion; got:\n%s", errout)
	}
	if !strings.Contains(errout, "deleted stale local branch "+branch) {
		t.Fatalf("the branch deletion was not reported by name; got:\n%s", errout)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag: %v", gitCalls(*calls))
	}
}

// --- 6. --dry-run prints the whole plan and changes NOTHING ---------------------
//
// A plan an operator signs off before arming an unattended supervisor is worth only as much
// as the guarantee that producing it changed nothing: not the tree, not the lock, not the ref.
func TestReapDryRunIsReadOnly(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	target, branch := openPRWorktree(t, work, "dryrun")
	lockAs(t, work, target, "dead-sess-4")
	plantBeacon(t, "dead-sess-4", time.Now().Add(-6*time.Hour))
	resetCalls(calls)

	rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	assertLocked(t, work, target, true)
	if !localBranchExists(t, work, branch) {
		t.Fatalf("a DRY RUN deleted branch %s; stderr:\n%s", branch, errout)
	}
	if !strings.Contains(errout, "dead-session-reaped 1") {
		t.Fatalf("the dry run did not report the reap it would make; got:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-deleted 0") {
		t.Fatalf("the dry run reported a branch deletion it did not make; got:\n%s", errout)
	}
	if !strings.Contains(errout, "REAP "+resolvePath(target)) || !strings.Contains(errout, "session dead-sess-4") {
		t.Fatalf("the plan row lacks the path or the session; got:\n%s", errout)
	}
	// The plan accounts for every registered worktree, including the ones no content gate
	// could ever reach.
	if !strings.Contains(errout, "KEEP "+resolvePath(work)) || !strings.Contains(errout, "protected:") {
		t.Fatalf("the plan does not account for the protected checkout; got:\n%s", errout)
	}
	for _, c := range gitCalls(*calls) {
		for i, a := range c {
			if a == "unlock" || (a == "branch" && i+1 < len(c) && strings.HasPrefix(c[i+1], "-")) {
				t.Fatalf("a DRY RUN ran a mutating git command: %v", c)
			}
		}
	}
}

// --- 7. an inert knob is refused, never silently accepted -----------------------

func TestLockTTLAcceptedWithReapOptIn(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	if rc, errout := runCapErr(t, []string{"prune", "--reap-dead-sessions", "--lock-ttl", "24h"}); rc != deskkit.ExitOK {
		t.Fatalf("--lock-ttl with --reap-dead-sessions rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	rc, errout := runCapErr(t, []string{"prune", "--lock-ttl", "24h"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("--lock-ttl alone rc = %d, want %d (refused as inert); stderr:\n%s", rc, deskkit.ExitRefused, errout)
	}
}
