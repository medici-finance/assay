package main

// workerdircollision_test.go — FAIL-FIRST coverage for the WORKER arm of worktreeCreateHint
// against deskwt's target-DIRECTORY refusal (#1309 item 6, review finding 2).
//
// THE DEFECT. deskwt has three refusals that can abort a worker dispatch's worktree-create
// step, and two of them are about a BRANCH while the third is not:
//
//	refused: branch <br> already exists and is CHECKED OUT in the worktree <path>   (branch, active)
//	refused: branch <br> already exists, is checked out in NO worktree, and ...     (branch, delivered)
//	refused: target already exists (never clobbered): <target>                      (DIRECTORY)
//
// The third names no branch at all — it is a stale local directory on this session key — but it
// contains the substring "already exists", and the worker arm tested for exactly that substring
// AFTER the CHECKED OUT case. So a directory collision fell through into the DELIVERED case and
// told the operator the brief was already delivered and to go look for a merged or open PR. The
// operator then hunts for a PR that does not exist while the actual obstruction is a directory
// that `deskwt remove` clears in one command.
//
// The VERIFIER arm never had this bug: it is selected by KIT before any message test, and its
// sentence already points at the stale target dir. That arm is the pattern the worker arm now
// mirrors, and TestWorkerHintSplitsCheckedOutFromDelivered only ever fed the directory message
// to worktreeCreateHint("verifier", ...) — never to the worker kit. That untested gap is
// precisely where this shipped wrong, and it is what this file closes.

import (
	"strings"
	"testing"
)

// The exact strings deskwt emits, kept here verbatim so a change to either message breaks this
// test rather than silently re-merging the two classes.
const (
	deskwtDirCollisionSaid = "refused: target already exists (never clobbered): " +
		"/private/tmp/tracker-item-1-stale"
	deskwtBranchDeliveredSaid = "refused: branch feat/item-1 already exists, is checked out in NO worktree, " +
		"and carries 2 commit(s) not in origin/main"
	deskwtBranchUnverifiableSaid = "branch feat/item-1 already exists and no worktree holds it, but its commits " +
		"ahead of origin/main could not be read"
)

// BOTH messages through the WORKER arm: each must render its own hint, and the directory
// collision must never render the DELIVERED one.
func TestWorkerHintSeparatesDirCollisionFromDeliveredBranch(t *testing.T) {
	dir := worktreeCreateHint("worker", "feat/item-1", deskwtDirCollisionSaid)
	delivered := worktreeCreateHint("worker", "feat/item-1", deskwtBranchDeliveredSaid)

	// The directory collision points at the directory and at `deskwt remove`, and NEVER at a PR.
	if strings.Contains(dir, "merged or open PR") || strings.Contains(dir, "DELIVERED") {
		t.Errorf("the worker dir-collision hint sends the operator PR-hunting over a stale directory:\n%s", dir)
	}
	if !strings.Contains(dir, "deskwt remove") {
		t.Errorf("the worker dir-collision hint does not name the reclaim command:\n%s", dir)
	}
	if !strings.Contains(dir, "TARGET DIRECTORY") {
		t.Errorf("the worker dir-collision hint does not name the directory as the cause:\n%s", dir)
	}
	// It must also not blame the branch: no branch is implicated by that deskwt message.
	if strings.Contains(dir, "The brief's branch") {
		t.Errorf("the worker dir-collision hint blames a branch deskwt never mentioned:\n%s", dir)
	}

	// The branch-exists-without-a-worktree message keeps the DELIVERED hint unchanged.
	if !strings.Contains(delivered, "DELIVERED") || !strings.Contains(delivered, "merged or open PR") {
		t.Errorf("the delivered-branch hint regressed:\n%s", delivered)
	}

	// And the two are genuinely distinct — the whole point of the split.
	if dir == delivered {
		t.Errorf("the directory collision and the delivered branch render the SAME hint:\n%s", dir)
	}
}

// The other branch-exists wording deskwt can emit (the Unverifiable arm, "no worktree holds it")
// must reach the DELIVERED hint too: the fix keys on branch-shaped substrings instead of bare
// "already exists", so both branch spellings have to be covered or the fix narrows the hint.
func TestWorkerHintStillDeliversOnTheUnverifiableBranchWording(t *testing.T) {
	got := worktreeCreateHint("worker", "feat/item-1", deskwtBranchUnverifiableSaid)
	if !strings.Contains(got, "DELIVERED") || !strings.Contains(got, "merged or open PR") {
		t.Errorf("the 'no worktree holds it' wording no longer reaches the delivered hint:\n%s", got)
	}
}

// All three worker-lane messages render three DIFFERENT sentences. A regression that re-merged
// any two classes would be caught here even if each individual assertion above still passed.
func TestWorkerHintRendersThreeDistinctClasses(t *testing.T) {
	checkedOut := worktreeCreateHint("worker", "feat/item-1",
		"refused: branch feat/item-1 already exists and is CHECKED OUT in the worktree "+
			"/private/tmp/tracker-item-1 — that worktree owns it")
	delivered := worktreeCreateHint("worker", "feat/item-1", deskwtBranchDeliveredSaid)
	dir := worktreeCreateHint("worker", "feat/item-1", deskwtDirCollisionSaid)

	seen := map[string]string{}
	for name, hint := range map[string]string{
		"checked-out": checkedOut,
		"delivered":   delivered,
		"dir":         dir,
	} {
		if prev, dup := seen[hint]; dup {
			t.Errorf("worker lane renders the same sentence for %s and %s:\n%s", prev, name, hint)
		}
		seen[hint] = name
	}
}

// The verifier arm is unchanged by the worker-arm fix — it is selected by KIT, before any
// message test, so the same directory message still takes the verifier's own sentence.
func TestVerifierArmUnaffectedByTheWorkerDirCollisionFix(t *testing.T) {
	got := worktreeCreateHint("verifier", "", deskwtDirCollisionSaid)
	if !strings.Contains(got, "not a branch collision") || strings.Contains(got, "merged or open PR") {
		t.Errorf("the verifier arm changed:\n%s", got)
	}
}
