package main

// huskremove.go — `deskwt remove` for a registered worktree that has NO git link left: an
// empty-directory "husk" (the directory exists and holds nothing) or a registration whose
// directory is gone entirely.
//
// THE DEFECT. remove opens the worktree to run its tracked-clean and pushed gates. A worktree
// whose `.git` file is gone cannot be opened, so remove exited 6 ("stat <path>/.git: no such
// file or directory") and the registration stayed in the shared admin registry forever. A
// registration keeps its branch "checked out", so no other worktree can take that branch and
// every tool that needs a PR's head branch (deskreply, deskdispatch --pr) refuses. Hand
// pruning is outside the desk verbs, so only a human could clear it. `git worktree remove`
// cannot help either — it needs the very `.git` file that is missing — and a lock makes
// `git worktree prune` keep the entry however dead it is.
//
// THE RULE. A husk is cleared by remove only when NOTHING is lost and NOBODY is using it:
//
//	CONTENT  — the directory holds nothing (or is gone). A husk with any entry is still
//	           refused: its files exist nowhere else.
//	HANDLES  — no process holds the directory open (cwd included). An instrument that
//	           cannot look (no lsof) is could-not-check and refuses, never a pass.
//	HEAD     — the admin entry's HEAD is the last git record the registration has and
//	           clearing deletes it: a branch must positively resolve to a commit (the branch
//	           survives in the shared refs), a detached commit must be on a remote-tracking
//	           ref. This is the SAME gate prune's stale-registration sweep applies
//	           (adminHeadHold); the two cannot drift apart.
//	OWNER    — a lock is the cooperative "a live session is using this" promise. It is
//	           retired only when the CALLER is the session the lock names, or the roster
//	           proves that session gone (judgeLock — the one staleness judgement the
//	           reclaim pass and the reap arm also use). A lock naming no session, or
//	           git's in-flight "initializing" lock, is never retired here.
//
// Only then: unlock, `git worktree prune`, verify the registration is gone (the same batched
// verifier every other removal uses), remove the now-empty directory. If the clear fails
// after the unlock, the original lock is restored with its original reason.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// huskLsofTimeout bounds the open-handle probe: a probe that hangs is could-not-check.
const huskLsofTimeout = 10 * time.Second

// huskHolders reports the processes holding rt open (as `pid` strings). It is a seam so a
// test can plant a holder, or an unavailable instrument, without a real process. A probe
// that cannot run is an ERROR (could-not-check), never an empty list.
var huskHolders = lsofHolders

// lsofHolders asks lsof which processes have the directory open. `lsof -t -- <dir>` lists
// every process with the directory open, a process whose cwd is the directory included.
// Exit 1 with no output is lsof's "nothing has it open"; any other failure — lsof absent,
// timeout, an exit with output on stderr — proves nothing.
func lsofHolders(rt string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), huskLsofTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-t", "--", rt)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("lsof timed out after %s", huskLsofTimeout)
	}
	pids := strings.Fields(string(out))
	if err == nil {
		return pids, nil
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 && len(pids) == 0 && strings.TrimSpace(string(ee.Stderr)) == "" {
		return nil, nil
	}
	return nil, err
}

// removeHusk is remove's arm for a registered worktree whose `.git` link is gone. It returns
// handled=false (and no error) when the worktree HAS a link, so the ordinary gates run
// unchanged; handled=true means this arm owns the outcome, success or refusal.
func removeHusk(guard *pathGuard, dir, rt string) (handled bool, detail string, err error) {
	if _, serr := os.Lstat(filepath.Join(rt, ".git")); serr == nil || !os.IsNotExist(serr) {
		// A live link — or a stat failure that is not "not there", which the ordinary
		// path fails closed on — is not this arm's business.
		return false, "", nil
	}

	// CONTENT.
	dirMissing := false
	entries, rerr := os.ReadDir(rt)
	switch {
	case rerr == nil:
		if len(entries) != 0 {
			return true, "", deskkit.Unverifiable(fmt.Sprintf("refused: %s has no .git link and its directory holds %d "+
				"entr(ies) — remove only clears an EMPTY husk, because those files exist nowhere else; "+
				"inspect and clear the directory by hand first", rt, len(entries)), nil)
		}
	case os.IsNotExist(rerr):
		dirMissing = true
	default:
		return true, "", deskkit.Unverifiable("cannot read the husk directory "+rt, rerr)
	}

	// HANDLES.
	if !dirMissing {
		holders, herr := huskHolders(rt)
		if herr != nil {
			return true, "", deskkit.Unverifiable("cannot prove no process holds "+rt+" open (could-not-check): "+herr.Error(), herr)
		}
		if len(holders) != 0 {
			return true, "", deskkit.Refused("refused: " + rt + " is held open by process(es) " + strings.Join(holders, ", "))
		}
	}

	// HEAD.
	admin, aerr := guard.adminDirFor(rt)
	if aerr != nil {
		return true, "", aerr
	}
	if hold := adminHeadHold(dir, admin, huskDetachedProof(dir)); hold != "" {
		if strings.HasPrefix(hold, "could-not-check:") {
			return true, "", deskkit.Unverifiable("cannot clear the registration of "+rt+": "+hold, nil)
		}
		return true, "", deskkit.Refused("refused: " + hold)
	}

	// OWNER.
	locked, lerr := guard.lockedWorktrees(dir)
	if lerr != nil {
		return true, "", lerr
	}
	lockReason, isLocked := locked[rt]
	ownerNote := ""
	if isLocked {
		ok, why := huskLockOwner(guard, rt, lockReason)
		if !ok {
			return true, "", deskkit.Refused("refused: " + rt + " is locked (" + lockReasonText(lockReason) + ") and " + why)
		}
		ownerNote = "; lock retired: " + why
	}

	// before_remove — same LOGGED failure class as the ordinary path, and the same contract:
	// the hook runs with cwd = the worktree about to be removed. When the directory is gone
	// there is no such cwd and nothing left to clean, so the hook is SKIPPED — never run with
	// an empty worktree, where it would inherit the caller's checkout as its cwd.
	if !dirMissing {
		if _, herr := deskkit.RunHook(deskkit.HookBeforeRemove, deskkit.HookEnv{
			RunKey: filepath.Base(rt), Worktree: rt, Repo: repoOrEmpty(dir),
		}); herr != nil {
			fmt.Fprintln(os.Stderr, "deskwt: before_remove hook failed (deletion proceeds): "+herr.Error())
		}
	}

	if isLocked {
		if _, uerr := runGit(dir, "worktree", "unlock", rt); uerr != nil {
			return true, "", deskkit.Unverifiable("could not unlock the husk registration "+rt, uerr)
		}
	}
	if derr := deregisterWorktrees(guard, dir, []string{rt}); derr != nil {
		if isLocked {
			if _, relErr := runGit(dir, "worktree", "lock", "--reason", lockReason, rt); relErr != nil {
				return true, "", deskkit.Unverifiable("could not clear the husk registration and could not restore its lock: "+derr.Error(), relErr)
			}
			fmt.Fprintln(os.Stderr, "deskwt: clearing failed; the original lock on "+rt+" was restored")
		}
		return true, "", derr
	}
	if !dirMissing {
		// Non-recursive on purpose: it can only remove a directory that is STILL empty.
		if rmErr := os.Remove(rt); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintln(os.Stderr, "deskwt: registration cleared but the empty directory "+rt+" was not removed: "+rmErr.Error())
		}
	}
	shape := "empty husk"
	if dirMissing {
		shape = "directory already gone"
	}
	return true, fmt.Sprintf("cleared registration of %s (%s, no .git link)%s", rt, shape, ownerNote), nil
}

// huskDetachedProof is the detached-HEAD arm of the HEAD gate for remove: the commit must be
// reachable from a remote-tracking ref — the proof remove's own detached path already
// accepts — so clearing the registration cannot strand the only reference to it.
func huskDetachedProof(dir string) func(head string) string {
	return func(head string) string {
		repo, err := gitcore.Open(dir)
		if err != nil {
			return "could-not-check: cannot open the repository to prove the detached HEAD is on a remote: " + errText(err)
		}
		names, err := repo.RefsContaining(head, "refs/remotes/")
		if err != nil {
			return "could-not-check: cannot check whether the detached HEAD is on a remote: " + errText(err)
		}
		if len(names) == 0 {
			return "detached HEAD " + head[:12] + " is on no remote-tracking ref — clearing the registration would strand possibly-unpushed commits"
		}
		return ""
	}
}

// huskLockOwner decides whether the caller may retire a lock — a husk's, or (remove's ordinary
// path) a live tree's — and says why in words
// (the evidence line for the audit row and the refusal alike). It never retires git's
// in-flight "initializing" lock, nor a lock naming no session — nobody is identifiable as
// its owner.
func huskLockOwner(guard *pathGuard, rt, reason string) (bool, string) {
	if strings.TrimSpace(reason) == gitInitLockReason {
		return false, `it is git's in-flight "initializing" add lock — never retired`
	}
	sess := sessionFromLockReason(reason)
	if sess == "" {
		return false, "it names no session, so no owner can be matched or proven gone"
	}
	if caller := resolveSession(""); caller == sess {
		return true, "the caller is the locking session " + sess
	}
	mt, haveMtime := guard.lockFileMtimes()[rt]
	v := judgeLock(reason, mt, haveMtime, 0, rosterBeaconDir(), time.Now())
	if v.stale {
		return true, v.why
	}
	return false, "the owner is not the caller and not proven gone: " + v.why
}
