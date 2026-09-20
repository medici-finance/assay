package main

// deadsession.go — prune's DEAD-SESSION reap arm.
//
// The defect this arm exists for: a desk session that DIES (crashed, or killed by a usage
// limit) leaves its worktree behind in one of two shapes that the ordinary sweep can never
// clear, however long it runs.
//
//  1. The worktree is LOCKED with a `… session=<id>` reason. The lock gate is consulted
//     first and holds every locked worktree unconditionally, so the lock outlives the
//     session that took it — forever.
//  2. The worktree is unlocked but its branch is NOT an ancestor of the remote mainline
//     (the state an open PR is in). The merge gate reads that as active work and holds it —
//     which is right while a session still owns the tree, and wrong once nothing does.
//
// Both shapes accumulate. Every later resume or dispatch of such a branch then fails at
// worktree-create, because git permits exactly one worktree per branch, so the leftovers
// wedge the queue they were left behind by.
//
// The rule this arm adds is deliberately NOT "unlock it and let the ordinary gates decide":
// the ordinary merge gate would still hold shape 2, which is the majority of the population.
// It is a separate judgement with its own, different safety gate:
//
//	OWNERSHIP — who, if anyone, is still entitled to this tree?
//	  locked + the locking session is LIVE  → held, whatever its content says. The lock is
//	                                          the cooperative half of the liveness guard and
//	                                          a live session's tree is never a candidate.
//	  locked + the locking session is GONE  → the tree is ownerless; continue to SAFETY.
//	  unlocked                              → "unowned": no session ever claimed it (the
//	                                          lock is what a live session takes), continue.
//
//	SAFETY — is removing it provably lossless? Both halves must hold:
//	  clean    — `git status --porcelain` empty, UNTRACKED FILES INCLUDED. This is stricter
//	             than the ordinary sweep's tracked-only gate, on purpose: an untracked file
//	             is reachable from no commit, so it exists nowhere but this directory.
//	  pushed   — HEAD is reachable from its own upstream, or (no upstream) from
//	             refs/remotes/origin/main. Every commit is then already on the remote and the
//	             directory is the only thing being deleted.
//
// Anything that fails SAFETY is LISTED with the reason that failed it and left alone —
// never silently skipped, and never removed. Anything that cannot be CHECKED is likewise
// left, with the could-not-check reason, because an unreadable gate is not a passed one.
//
// Ownership and safety are independent layers over independent signals: ownership asks the
// roster about a session, safety asks the worktree about its own content and refs. Neither
// can mask a failure of the other — a live session's lock holds a tree the safety gate would
// have cleared, and a dirty tree is held however dead its session is.

import (
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// reapVerdict is the reap arm's judgement for ONE worktree, and it is the unit the plan is
// printed from. `reason` is filled in BOTH directions: a KEEP whose reason was not recorded
// is indistinguishable from a worktree the sweep never looked at, which is the failure the
// #1375 incident was diagnosed through (an operator could see that ~550 trees survived every
// sweep but not which gate had held each one).
type reapVerdict struct {
	path    string
	session string // the session the lock names; "" when the worktree is unowned
	reap    bool
	reason  string
	branch  string // local branch deleted with the tree; "" when none was
}

// owner renders the session field for the plan, naming the unowned case explicitly rather
// than printing an empty column where an id should be.
func (v reapVerdict) owner() string {
	if v.session == "" {
		return "unowned"
	}
	return v.session
}

// planLine is one row of `--dry-run`'s plan: path, session, verdict, reason — the four
// things an operator needs to sign off a sweep BEFORE the supervisor runs it unattended.
func (v reapVerdict) planLine() string {
	verdict := "KEEP"
	if v.reap {
		verdict = "REAP"
	}
	return fmt.Sprintf("  %s %s — session %s — %s", verdict, v.path, v.owner(), v.reason)
}

// judgeReap is the whole reap decision for one worktree: ownership, then safety. It is a
// pure judgement — it deletes nothing — so the plan `--dry-run` prints is produced by the
// very same code that a live sweep acts on, rather than by a second description of it that
// could drift.
//
// isLocked/lockReason come from the sweep's single `git worktree list --porcelain` read;
// mt/haveMtime from its single admin-file stat; rosterDir and now are passed in so the
// judgement takes no clock and reads no environment of its own.
func judgeReap(sc *sweepCtx, rt, lockReason string, isLocked bool, mt time.Time, haveMtime bool,
	ttl time.Duration, rosterDir string, now time.Time) reapVerdict {

	v := reapVerdict{path: rt}

	// --- OWNERSHIP ---------------------------------------------------------------
	if isLocked {
		v.session = sessionFromLockReason(lockReason)
		// judgeLock is the SAME staleness judgement the lock-reclaim pass makes, reached
		// here rather than re-derived: one definition of "that session is gone" governs
		// both retiring a lock and reaping the tree behind it, so the two can never
		// disagree about the same session in the same sweep.
		lv := judgeLock(lockReason, mt, haveMtime, ttl, rosterDir, now)
		if !lv.stale {
			v.reason = "held: " + lv.why
			return v
		}
		v.reason = "dead session: " + lv.why
	} else {
		// No lock means no session ever claimed this tree — a LIVE desk session takes the
		// lock at boot (role-init), which is the cooperative half of this guard. An
		// unlocked tree is therefore ownerless by construction, and the safety gate below
		// is the only thing standing between it and removal. That is the whole reason the
		// safety gate here counts untracked files.
		v.reason = "unowned: no live-session lock on this worktree"
	}

	// --- SAFETY ------------------------------------------------------------------
	// Clean first: it is the gate whose failure is most often a human's unsaved work, and
	// the one an operator most needs named.
	dirty, derr := dirtyAnyIn(sc, rt)
	if derr != nil {
		v.reason += "; could-not-check: unverifiable working-tree status: " + errText(derr)
		return v
	}
	if dirty != "" {
		v.reason += "; dirty (uncommitted or untracked files present)"
		return v
	}

	pushed, why, perr := pushedState(sc, rt)
	if perr != nil {
		v.reason += "; could-not-check: " + errText(perr)
		return v
	}
	if !pushed {
		v.reason += "; " + why
		return v
	}

	v.reap = true
	v.reason += "; safe to reap: clean working tree, " + why
	return v
}

// dirtyAnyIn is the reap arm's cleanliness gate — `git status --porcelain` with untracked
// files COUNTED — run against the sweep's own handle on rt so no worktree is opened twice.
//
// It is a package-level var for the same reason dirtyTrackedIn is: a test can count the
// calls, which is how "the ownership gate held this tree before its content was ever read"
// is asserted as behaviour rather than inferred from timing.
var dirtyAnyIn = func(sc *sweepCtx, rt string) (string, error) {
	repo, err := sc.open(rt)
	if err != nil {
		return "", deskkit.Unverifiable("cannot check the worktree's working-tree status", err)
	}
	out, serr := repo.DirtyPorcelain()
	if serr != nil {
		return "", deskkit.Unverifiable("cannot check the worktree's working-tree status", serr)
	}
	return out, nil
}

// pushedState reports whether every commit in this worktree's HEAD is already on the remote,
// and says in words which ref proved it (or which one failed to).
//
// Two acceptable proofs, in order:
//
//	HEAD is reachable from its own UPSTREAM — the normal case for a branch behind a draft
//	  PR. Nothing local is lost: the upstream ref holds HEAD and everything below it.
//	HEAD is reachable from refs/remotes/origin/main — the fallback for a branch that never
//	  had an upstream configured (a detached checkout, a branch cut and never pushed).
//
// The mainline is spelled FULLY QUALIFIED for the reason #885 gives at length: a stray local
// `refs/heads/origin/main` wins gitrevisions disambiguation, and a gate that compared against
// that decoy would clear a branch against a ref no remote has ever seen.
//
// Every failure to READ a ref is returned as an error, never as a false — an unreadable
// upstream must leave the worktree alone, not condemn it.
func pushedState(sc *sweepCtx, rt string) (bool, string, error) {
	repo, err := sc.open(rt)
	if err != nil {
		return false, "", deskkit.Unverifiable("cannot open the worktree to check whether HEAD is pushed", err)
	}
	upstream, uerr := repo.UpstreamRef()
	if uerr == nil {
		anc, aerr := repo.IsAncestor("HEAD", upstream)
		if aerr != nil {
			return false, "", deskkit.Unverifiable("cannot compare HEAD with its upstream "+upstream, aerr)
		}
		if anc {
			return true, "HEAD is reachable from its upstream " + upstream, nil
		}
	}

	onMain, merr := repo.IsAncestor("HEAD", "refs/remotes/origin/main")
	if merr != nil {
		return false, "", deskkit.Unverifiable("cannot compare HEAD with refs/remotes/origin/main", merr)
	}
	if onMain {
		return true, "HEAD is reachable from refs/remotes/origin/main", nil
	}
	if uerr != nil {
		return false, "no-upstream-and-not-on-main (this branch has no upstream and HEAD is not reachable from refs/remotes/origin/main)", nil
	}
	if upstream == "refs/remotes/origin/main" {
		// A worktree cut by `deskwt add` tracks the mainline itself, so naming the same ref
		// twice here would read as two separate proofs where there is only one.
		return false, "unpushed (HEAD is not reachable from its upstream refs/remotes/origin/main)", nil
	}
	return false, "unpushed (HEAD is not reachable from its upstream " + upstream +
		", nor from refs/remotes/origin/main)", nil
}

// branchSafeToDelete returns the local branch name that may be deleted along with this
// worktree, or "" when none may be.
//
// Deleting the branch is the other half of the fix, not a tidy-up: `deskwt add --branch <b>`
// fails outright while a local `refs/heads/<b>` exists, so a reaped worktree whose stale
// branch survives leaves the SAME resume wedged, one error message further on. With the ref
// gone, the next add cuts fresh from the remote tip.
//
// The bar is "equal to or behind its upstream": the branch carries nothing the remote does
// not already have. A branch with no upstream is NEVER deleted here — a local-only ref is
// the only record of its own commits, and this arm does not delete records.
func branchSafeToDelete(repo *gitcore.Repo) (string, error) {
	branch, berr := repo.AbbrevRefHEAD()
	if berr != nil || branch == "" || branch == "HEAD" {
		// Detached HEAD checks out no branch, so there is no stale ref to collide with.
		return "", nil
	}
	upstream, uerr := repo.UpstreamRef()
	if uerr != nil {
		return "", nil
	}
	anc, aerr := repo.IsAncestor(branch, upstream)
	if aerr != nil {
		return "", deskkit.Unverifiable("cannot compare branch "+branch+" with its upstream "+upstream, aerr)
	}
	if !anc {
		return "", nil
	}
	return branch, nil
}

// deleteLocalBranch drops a stale local branch with `git branch -d` — the NON-force form,
// chosen deliberately.
//
// `-d` refuses unless the branch is fully merged into its upstream, which is the very
// condition branchSafeToDelete just proved independently. That redundancy is the point: this
// tool's standing rule is that there is no --force anywhere, and the two checks fail on
// different signals in different components — ours is an ancestry walk in go-git over the
// refs this sweep read, git's is its own reachability test over the refs on disk at the
// moment of deletion. A branch that advanced between the two (a push landing mid-sweep)
// trips git's check even though ours passed, and the branch survives. `-D` would silently
// delete it.
func deleteLocalBranch(dir, branch string) error {
	if _, err := runGit(dir, "branch", "-d", branch); err != nil {
		return err
	}
	return nil
}
