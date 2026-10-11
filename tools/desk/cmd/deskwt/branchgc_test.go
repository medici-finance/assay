package main

// Tests for prune's local-branch GC (branchgc.go, Step C).
//
// The invariant under test is the one the pass states at its head: a branch is collected
// ONLY when its content is provably on refs/remotes/origin/main — ancestry-merged or
// patch-id-clean — and EVERY other shape (unique patches, main/master, checked out
// anywhere, a proof that cannot be completed, a delete that fails) is kept. The fixtures
// are real git repos (newRepo): the proofs are git's own, so a test that drifts from what
// git considers true fails loudly instead of asserting against a mock's opinion.
//
// One fixture fact recurs in the expectations: the shared checkout always carries a local
// `main` (checked out there), so the GC's protected-name rule accounts for exactly one
// held branch in every fixture below.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// landOnMain commits <content> to <file> directly on main and re-points
// refs/remotes/origin/main at the new tip. Unlike advanceMain (the shared fixture helper,
// which writes its own marker content) this takes the content verbatim, so a test can land
// the EXACT diff a branch carries — the squash-merge shape the cherry-clean proof collects.
func landOnMain(t *testing.T, work, file, content string) {
	t.Helper()
	writeFile(t, filepath.Join(work, file), content)
	mustGit(t, work, "add", file)
	mustGit(t, work, "commit", "-m", "land on the mainline")
	mustGit(t, work, "update-ref", "refs/remotes/origin/main", mustGit(t, work, "rev-parse", "HEAD"))
}

// branchExists reports whether refs/heads/<name> resolves.
func branchExists(t *testing.T, work, name string) bool {
	t.Helper()
	_, err := runGit(work, "rev-parse", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// --- branch GC: an ancestry-merged branch is collected ---------------------------------

func TestPruneBranchGCCollectsMergedBranch(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	// A branch cut at the CURRENT main tip, then the mainline marches past it: the tip is
	// an ancestor of refs/remotes/origin/main, so every commit it holds is on the remote.
	// (A fresh, never-used handle is exactly this shape — tip == merge-base(tip, main) —
	// and is collected by the same proof: the two are topologically indistinguishable.)
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if branchExists(t, work, "stale-merged") {
		t.Fatalf("merged local branch survived the branch GC; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-gc 1 (merged 1, cherry-clean 0), branches-held 1") {
		t.Fatalf("expected the merged branch counted in the summary (held 1 = the protected main); got:\n%s", errout)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during the branch GC: %v", gitCalls(*calls))
	}
}

// --- branch GC: a cherry-clean (squash-merged) branch is collected ---------------------

func TestPruneBranchGCCollectsCherryCleanBranch(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	// A branch whose commit is NOT on the mainline as a commit, but whose PATCH is: the
	// squash-merge shape. The branch gets a commit adding cherry.txt; the mainline then
	// lands the identical diff as a different commit. patch-id is computed from the diff,
	// never the commit metadata, so the two ids are equal — while the branch tip is
	// genuinely not an ancestor, so ONLY the patch-id proof can collect it.
	mustGit(t, work, "checkout", "-b", "squashed")
	writeFile(t, filepath.Join(work, "cherry.txt"), "the same change\n")
	mustGit(t, work, "add", "cherry.txt")
	mustGit(t, work, "commit", "-m", "feature work, later squash-merged")
	mustGit(t, work, "checkout", "main")
	landOnMain(t, work, "cherry.txt", "the same change\n")

	// POSITIVE CONTROL: the branch really is in the cherry-clean shape — not an ancestor
	// (the merged proof must NOT be what collects it) and `git cherry` sees no '+'.
	if ahead, herr := runGit(work, "rev-list", "--count", "refs/remotes/origin/main..refs/heads/squashed"); herr != nil || ahead != "1" {
		t.Fatalf("positive control void: branch ahead count = %q (err %v), want 1", ahead, herr)
	}
	// merge-base --is-ancestor answers by EXIT STATUS (1 = not an ancestor), so it goes
	// through runGit: mustGit would fatal on the very answer this control wants.
	if _, aerr := runGit(work, "merge-base", "--is-ancestor", "refs/heads/squashed", "refs/remotes/origin/main"); aerr == nil {
		t.Fatalf("positive control void: branch IS an ancestor of the mainline — not the cherry-clean shape")
	}
	cherry := mustGit(t, work, "cherry", "refs/remotes/origin/main", "refs/heads/squashed")
	if strings.Contains(cherry, "+") {
		t.Fatalf("positive control void: git cherry sees unique commits:\n%s", cherry)
	}

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if branchExists(t, work, "squashed") {
		t.Fatalf("cherry-clean local branch survived the branch GC; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-gc 1 (merged 0, cherry-clean 1), branches-held 1") {
		t.Fatalf("expected the branch counted under the cherry-clean class (held 1 = the protected main); got:\n%s", errout)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag during the branch GC: %v", gitCalls(*calls))
	}
}

// --- branch GC: a branch with unique patches is KEPT, always ---------------------------

func TestPruneBranchGCKeepsUniquePatches(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	// A branch carrying a change the mainline has never seen: the unique-patch case the
	// whole invariant exists to protect.
	mustGit(t, work, "checkout", "-b", "active-work")
	writeFile(t, filepath.Join(work, "novel.txt"), "never landed anywhere\n")
	mustGit(t, work, "add", "novel.txt")
	mustGit(t, work, "commit", "-m", "genuinely new work")
	mustGit(t, work, "checkout", "main")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "active-work") {
		t.Fatalf("the branch GC deleted a branch with UNIQUE patches; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-held 2") {
		t.Fatalf("expected the unique branch and the protected main counted as held; got:\n%s", errout)
	}
	if strings.Contains(errout, "branches-gc 1") {
		t.Fatalf("the summary claims a collection that must not have happened:\n%s", errout)
	}
}

// --- branch GC: main and master are never candidates ------------------------------------

func TestPruneBranchGCNeverDeletesMainOrMaster(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// A stale local `master` at the ORIGINAL tip, then the mainline marches: merged by
	// ancestry, checked out nowhere — every content gate passes. Only the protected-name
	// rule can keep it. (main itself is checked out in the shared checkout, so it is
	// additionally held by the checked-out guard; master exercises the name rule alone.)
	original := mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "branch", "master", original)
	advanceMain(t, work, "mainline")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "master") {
		t.Fatalf("the branch GC deleted master — a protected name; stderr:\n%s", errout)
	}
	if !branchExists(t, work, "main") {
		t.Fatalf("the branch GC deleted main — a protected name; stderr:\n%s", errout)
	}
}

// --- branch GC: a branch checked out in any worktree is never a candidate ---------------

func TestPruneBranchGCKeepsCheckedOutBranch(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// A worktree on a branch, LOCKED so the worktree arms hold the tree and the branch
	// stays checked out when the branch GC runs. The branch is merged-below-tip — every
	// content gate passes — so only the checked-out guard can keep the ref.
	target := addWorktree(t, "held-branch")
	advanceMain(t, work, "mainline")
	mustGit(t, work, "worktree", "lock", target)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	assertExists(t, target)
	if !branchExists(t, work, "held-branch") {
		t.Fatalf("the branch GC deleted a branch CHECKED OUT in a worktree; stderr:\n%s", errout)
	}
}

// --- branch GC: the branch of a worktree Step B just removed is collected in-sweep ------

func TestPruneBranchGCCollectsBranchOfRemovedWorktree(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// The ordering contract: Step B removes the merged+clean worktree, and Step C in the
	// SAME sweep collects the local ref it leaves behind — the exact residue class the
	// pass exists for (git worktree remove never deletes the branch).
	target := addWorktree(t, "residue")
	advanceMain(t, work, "mainline")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("merged+clean worktree %s still exists after prune (err=%v)", target, err)
	}
	if branchExists(t, work, "residue") {
		t.Fatalf("the removed worktree's local branch survived the same sweep; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "branches-gc 1 (merged 1, cherry-clean 0)") {
		t.Fatalf("expected the leftover branch collected in-sweep; got:\n%s", errout)
	}
}

// --- branch GC: a merge commit ahead of the mainline holds the branch -------------------

func TestPruneBranchGCHoldsBranchWithMergeAhead(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	// A branch whose NON-merge commit is cherry-clean (its patch is on the mainline) but
	// which also carries a MERGE commit the mainline does not. Merge commits have no
	// patch-id, so the patch-id proof cannot see content a merge introduced (a conflict
	// resolution, an evil merge) — the branch is held however clean its other commits are.
	mustGit(t, work, "checkout", "-b", "mergey")
	writeFile(t, filepath.Join(work, "cherry.txt"), "the same change\n")
	mustGit(t, work, "add", "cherry.txt")
	mustGit(t, work, "commit", "-m", "feature work, later squash-merged")
	mustGit(t, work, "checkout", "main")
	landOnMain(t, work, "cherry.txt", "the same change\n")
	advanceMain(t, work, "mainline")
	mustGit(t, work, "checkout", "mergey")
	mustGit(t, work, "merge", "--no-ff", "-m", "merge mainline into mergey", "refs/remotes/origin/main")
	mustGit(t, work, "checkout", "main")

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "mergey") {
		t.Fatalf("the branch GC deleted a branch with a MERGE commit ahead of the mainline; stderr:\n%s", errout)
	}
}

// --- branch GC: --dry-run reports the plan and deletes nothing --------------------------

func TestPruneBranchGCDryRunDeletesNothing(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	rc, errout := runCapErr(t, []string{"prune", "--dry-run"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --dry-run rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "stale-merged") {
		t.Fatalf("a --dry-run deleted a branch; stderr:\n%s", errout)
	}
	if !strings.Contains(errout, "COLLECT branch stale-merged — merged [dry-run: not deleted]") {
		t.Fatalf("expected the dry-run plan to name the branch and its proof class; got:\n%s", errout)
	}
}

// --- branch GC: a failed delete is a skip and a warning, never a sweep failure ----------

func TestPruneBranchGCDeleteFailureIsSkipNotError(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	// Break exactly the compare-and-delete, leaving every PROOF intact: the branch is
	// provably merged, git simply refuses the ref update. The sweep must still exit 0,
	// must NOT count the branch as collected, and must say why out loud.
	inner := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 2 && args[0] == "update-ref" && args[1] == "-d" {
			return inner("false")
		}
		return inner(name, args...)
	}
	t.Cleanup(func() { execCommand = inner })

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0 — a failed branch delete is a skip, not a sweep failure; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "stale-merged") {
		t.Fatalf("the branch vanished despite its delete failing")
	}
	if !strings.Contains(errout, "could not be deleted") {
		t.Fatalf("expected a warning naming the failed delete; got:\n%s", errout)
	}
	if strings.Contains(errout, "branches-gc 1") {
		t.Fatalf("a branch whose delete FAILED was counted as collected:\n%s", errout)
	}
}

// --- branch GC: an unreadable mainline holds EVERYTHING (fail closed) -------------------

func TestPruneBranchGCHoldsEverythingWhenMainlineUnreadable(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	mustGit(t, work, "branch", "stale-merged")
	// Remove the remote-tracking ref entirely: the sweep cannot build the mainline
	// ancestor set, so NO branch can be proven merged — every branch must be held.
	mustGit(t, work, "update-ref", "-d", "refs/remotes/origin/main")

	rc, errout := runCapErr(t, []string{"prune", "--branches-only"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --branches-only rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "stale-merged") {
		t.Fatalf("the branch GC collected a branch it could not prove merged (mainline unreadable)")
	}
	if !strings.Contains(errout, "no branch can be proven merged") {
		t.Fatalf("expected the fail-closed warning; got:\n%s", errout)
	}
}

// --- the cadence split: --no-branches / --branches-only ---------------------------------

func TestPruneNoBranchesSkipsTheGC(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "wt-only")
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	rc, errout := runCapErr(t, []string{"prune", "--no-branches"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --no-branches rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	// The worktree arms ran (the merged+clean worktree is gone) ...
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("prune --no-branches left the merged+clean worktree (err=%v)", err)
	}
	// ... but the branch pass did not: both branches survive, and the summary does not
	// claim a GC that never ran (a zero count would read as "nothing to collect").
	if !branchExists(t, work, "stale-merged") || !branchExists(t, work, "wt-only") {
		t.Fatalf("prune --no-branches collected branches anyway; stderr:\n%s", errout)
	}
	if strings.Contains(errout, "branches-gc") {
		t.Fatalf("the summary claims a branch GC that --no-branches skipped:\n%s", errout)
	}
}

func TestPruneBranchesOnlySkipsTheWorktreeArms(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := addWorktree(t, "branches-only-wt")
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	rc, errout := runCapErr(t, []string{"prune", "--branches-only"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune --branches-only rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	// The branch pass ran: the merged standalone branch is collected ...
	if branchExists(t, work, "stale-merged") {
		t.Fatalf("prune --branches-only did not collect the merged branch; stderr:\n%s", errout)
	}
	// ... and the worktree arms did NOT: the merged+clean worktree (and therefore the
	// branch it has checked out) is exactly where it was.
	assertExists(t, target)
	if !branchExists(t, work, "branches-only-wt") {
		t.Fatalf("--branches-only deleted a checked-out branch (the worktree arms must not have run)")
	}
	if !strings.Contains(errout, "branches-gc 1 (merged 1, cherry-clean 0)") {
		t.Fatalf("expected the branches-only summary; got:\n%s", errout)
	}
}

func TestPruneBranchFlagsRefuseInertCombinations(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	for _, args := range [][]string{
		{"prune", "--no-branches", "--branches-only"},
		{"prune", "--branches-only", "--reclaim-stale-locks"},
		{"prune", "--branches-only", "--reap-dead-sessions"},
		{"prune", "--branches-only", "--lock-ttl", "24h"},
	} {
		if rc, _ := runCapErr(t, args); rc != deskkit.ExitRefused {
			t.Fatalf("prune %v rc = %d, want 5 (refused) — an inert or contradictory flag combination is never accepted silently", args, rc)
		}
	}
}

// --- branch GC: the delete is the compare-and-delete, never a force verb ----------------

func TestPruneBranchGCDeletesByCompareAndDelete(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	mustGit(t, work, "branch", "stale-merged")
	wantSHA := mustGit(t, work, "rev-parse", "refs/heads/stale-merged")
	advanceMain(t, work, "mainline")

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if branchExists(t, work, "stale-merged") {
		t.Fatalf("merged branch survived; stderr:\n%s", errout)
	}
	// The ref went by `git update-ref -d --no-deref <ref> <sha-at-proof>` — the
	// compare-and-delete — never `git branch -D` and never any force flag.
	sawCAD := false
	for _, c := range gitCalls(*calls) {
		args := c[1:]
		for i, a := range args {
			if a == "branch" && i+1 < len(args) && (args[i+1] == "-D" || args[i+1] == "--delete") {
				t.Fatalf("the branch GC used a force delete: %v", c)
			}
		}
		if len(args) == 5 && args[0] == "update-ref" && args[1] == "-d" && args[2] == "--no-deref" &&
			args[3] == "refs/heads/stale-merged" && args[4] == wantSHA {
			sawCAD = true
		}
	}
	if !sawCAD {
		t.Fatalf("no compare-and-delete of refs/heads/stale-merged at its proven sha %s in: %v", wantSHA, gitCalls(*calls))
	}
}

// --- branch GC: a holder that APPEARS mid-sweep is caught by the pre-delete re-read ----
//
// The checked-out guard is the holder set and nothing else — `git update-ref -d` carries
// no checked-out refusal for linked worktrees (that refusal belongs to `git branch -d`,
// which cannot take the cherry-clean class). The candidate snapshot is taken before the
// classification loop; the whole classification of every earlier candidate sits between
// it and a given branch's delete. This test watches the SECOND read catch a worktree that
// "appeared" inside that window: the branch is provably merged, and only the re-read can
// keep it.
func TestPruneBranchGCRechecksHolderAtDelete(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	mustGit(t, work, "branch", "stale-merged")
	advanceMain(t, work, "mainline")

	// The listBranchHolders seam: the FIRST read (the candidate filter) sees the branch
	// free; every LATER read (the pre-delete re-verification) reports it checked out — a
	// worktree created mid-sweep.
	real := listBranchHolders
	reads := 0
	listBranchHolders = func(dir string) (map[string]string, error) {
		reads++
		h, err := real(dir)
		if reads >= 2 {
			if h == nil {
				h = map[string]string{}
			}
			h["refs/heads/stale-merged"] = "worktree-appeared-mid-sweep"
		}
		return h, err
	}
	t.Cleanup(func() { listBranchHolders = real })

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "stale-merged") {
		t.Fatalf("the pre-delete holder re-read failed to keep a branch a worktree appeared on; stderr:\n%s", errout)
	}
	if reads < 2 {
		t.Fatalf("the holder set was read %d time(s) — the pre-delete re-verification did not run", reads)
	}
	for _, c := range gitCalls(*calls) {
		args := c[1:]
		if len(args) >= 2 && args[0] == "update-ref" && args[1] == "-d" {
			t.Fatalf("a delete ran for a branch the re-read reported held: %v", c)
		}
	}
}

// --- branch GC: the shared checkout's own non-protected branch is held ------------------
//
// Every other fixture sits the shared checkout on `main`, which the protected-NAME rule
// already holds — so without this test the suite cannot see whether the checked-out guard
// covers the MAIN checkout's own branch. (Reviewer-driven coverage close.)
func TestPruneBranchGCKeepsBranchCheckedOutInSharedCheckout(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	// Advance the mainline FIRST (advanceMain commits onto the checked-out branch), then
	// sit the shared checkout on a merged-below-tip branch with a non-protected name.
	advanceMain(t, work, "mainline")
	original := mustGit(t, work, "rev-parse", "refs/remotes/origin/main~1")
	mustGit(t, work, "checkout", "-b", "trunk", original)

	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if !branchExists(t, work, "trunk") {
		t.Fatalf("the branch GC deleted the SHARED CHECKOUT's own branch; stderr:\n%s", errout)
	}
	// trunk (checked out) and main (protected name) are the held two; nothing collected.
	if !strings.Contains(errout, "branches-gc 0") || !strings.Contains(errout, "branches-held 2") {
		t.Fatalf("expected 0 collected and 2 held (trunk checked-out, main protected); got:\n%s", errout)
	}
}

// --- branch GC: a planted refs/heads symref can never delete its TARGET -----------------
//
// `git update-ref -d` dereferences symbolic refs by default: with refs/heads/decoy ->
// refs/heads/main planted, deleting "decoy" WITHOUT --no-deref deletes main itself (the
// enumerator reads the symref at its target's objectname, so the proof runs against the
// target's position). The delete carries --no-deref so the symref is deleted as itself —
// or refused — and the target is never touched.
func TestPruneBranchGCPlantedSymrefCannotDeleteItsTarget(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)
	mainSHA := mustGit(t, work, "rev-parse", "refs/heads/main")
	mustGit(t, work, "symbolic-ref", "refs/heads/decoy", "refs/heads/main")

	resetCalls(calls)
	rc, errout := runCapErr(t, []string{"prune"})
	if rc != deskkit.ExitOK {
		t.Fatalf("prune rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if got := mustGit(t, work, "rev-parse", "refs/heads/main"); got != mainSHA {
		t.Fatalf("deleting the planted symref touched its TARGET (main moved from %s to %s)", mainSHA, got)
	}
	if anyGitForce(*calls) {
		t.Fatalf("a git argv carried a force flag: %v", gitCalls(*calls))
	}
}

// --- the patch-id pipe: a dead consumer is an error, never a hang -----------------------
//
// patchIDsForRange joins git log to git patch-id with a real OS pipe whose ends the parent
// drops immediately after both children start, so a dead consumer leaves the pipe
// readerless and the producer dies by SIGPIPE instead of blocking a boot-path sweep
// forever. This test kills the consumer outright and requires an error within a bounded
// time — the io.Pipe shape this replaced hangs here even on a one-commit fixture.
func TestPatchIDConsumerFailureIsAnErrorNotAHang(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	inner := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 1 && args[0] == "patch-id" {
			return inner("false")
		}
		return inner(name, args...)
	}
	t.Cleanup(func() { execCommand = inner })

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		_, err := patchIDsForRange(work, "refs/remotes/origin/main")
		done <- result{err}
	}()
	select {
	case r := <-done:
		if r.err == nil {
			t.Fatal("patchIDsForRange with a dead consumer returned no error")
		}
	case <-time.After(60 * time.Second):
		t.Fatal("patchIDsForRange hung on a dead consumer — the pipe ends are not being dropped")
	}
}
