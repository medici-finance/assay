package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Adopting a FINISHED home on a resume (`deskwt add --adopt-run <run key>`, #4272).
//
// A worker that opens its PR and releases its claim leaves its home behind: clean, pushed,
// on the PR branch at the PR head. The rework dispatch of that PR then hit one of two
// refusals: the same session re-derives the same target ("target already exists (never
// clobbered)"), or an earlier session's home still has the branch checked out ("that
// worktree owns it"). Both refusals are right for a tree nobody has proven finished, and
// both stay. What changes is the case that IS proven: such a home is handed back as the
// new home instead of refused. Nothing is deleted, moved or reset — adoption is the
// non-destructive answer, so a mistaken adoption cannot lose work, and the never-clobber
// rule is untouched.
//
// The proof, every clause required:
//
//   - the caller holds the item's claim and names its key (--adopt-run); the dispatcher
//     acquires that claim before it allocates, and claims are exclusive, so the run that
//     stamped this key is no longer holding it;
//   - the home's worktree-scoped assay.runKey (stamped at its own dispatch) equals that
//     key, so it is a home of THIS item's dispatch, never a human's or another lane's tree;
//   - it is a sanctioned, registered, unlocked worktree of this repo (a lock marks a live
//     owner and is never overridden here);
//   - it is on --branch at exactly the pinned --base, and --upstream resolves to that same
//     commit (checked by the caller): nothing in it is unpushed;
//   - its status is empty including untracked files, and no merge, cherry-pick, revert or
//     rebase is in progress.
//
// Any failed clause keeps the original refusal and appends which clause failed.

// adoptProof checks candidate against the clauses above. It returns the resolved home on
// success, a refusal reason when a clause fails, or an error when a read cannot be made.
func adoptProof(guard *pathGuard, dir, candidate, br, base, runKey string) (string, string, error) {
	rt, cerr := guard.check(candidate)
	if cerr != nil {
		return "", cerr.Error(), nil
	}
	set, lerr := guard.worktreePaths(dir)
	if lerr != nil {
		return "", "", lerr
	}
	if !set[rt] {
		return "", "it is not a registered worktree of this repo", nil
	}
	locked, kerr := guard.lockedWorktrees(dir)
	if kerr != nil {
		return "", "", kerr
	}
	if reason, ok := locked[rt]; ok {
		return "", "it is locked (" + reason + ") — a lock marks a live owner", nil
	}
	if top, err := runGit(rt, "rev-parse", "--show-toplevel"); err != nil || resolvePath(top) != rt {
		return "", "its git link is missing or broken", nil
	}
	if ext, _ := runGit(rt, "config", "--bool", "--get", "extensions.worktreeConfig"); ext != "true" {
		return "", "it carries no worktree-scoped run key", nil
	}
	if key, _ := runGit(rt, "config", "--worktree", "--get", "assay.runKey"); key != runKey {
		return "", fmt.Sprintf("its assay.runKey is %q, not this run's %q", key, runKey), nil
	}
	if ref, _ := runGit(rt, "symbolic-ref", "-q", "HEAD"); ref != "refs/heads/"+br {
		return "", "it is not on branch " + br, nil
	}
	if head, err := runGit(rt, "rev-parse", "--verify", "HEAD^{commit}"); err != nil || head != base {
		return "", "its HEAD is not the pinned head " + base, nil
	}
	status, serr := runGit(rt, "status", "--porcelain", "--untracked-files=all")
	if serr != nil {
		return "", "", deskkit.Unverifiable("cannot read the status of "+rt, serr)
	}
	if status != "" {
		return "", "it has uncommitted or untracked changes", nil
	}
	for _, op := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"} {
		if _, err := runGit(rt, "rev-parse", "-q", "--verify", op); err == nil {
			return "", "it has a " + op + " in progress", nil
		}
	}
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		p, err := runGit(rt, "rev-parse", "--git-path", d)
		if err != nil {
			return "", "", deskkit.Unverifiable("cannot locate the rebase state of "+rt, err)
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(rt, p)
		}
		if _, err := os.Stat(p); err == nil {
			return "", "it has a rebase in progress", nil
		}
	}
	return rt, "", nil
}

// adoptedHome is the identity half of an add, applied to an adopted home: the branch's
// upstream, the role identity and transport (or the cleared identity), and an optional
// session lock. A failure here returns the error and leaves the home in place: an adopted
// tree is never rolled back, because it was not created by this call.
type adoptedHome struct {
	guard                         *pathGuard
	dir, rt, br, upstream, runKey string
	roleKey, name, email, repo    string
	lockSession                   string
}

func (a adoptedHome) provision() (string, error) {
	if _, err := runGit(a.rt, "branch", "--set-upstream-to", a.upstream, a.br); err != nil {
		return "", deskkit.Unverifiable("cannot set the adopted home's upstream", err)
	}
	detail := "identity CLEARED (no --role; a commit here fails closed until an identity is set)"
	if a.roleKey != "" {
		if err := setCommitIdentity(a.rt, a.name, a.email); err != nil {
			return "", err
		}
		transport, err := wireRoleTransport(a.rt, a.roleKey, a.repo)
		if err != nil {
			return "", err
		}
		detail = "identity " + deskkit.RoleIdentityLabel(a.roleKey) + "; " + transport
	} else if err := clearCommitIdentity(a.rt); err != nil {
		return "", err
	}
	if a.lockSession != "" {
		if err := lockForSession(a.guard, a.dir, a.rt, a.lockSession); err != nil {
			return "", err
		}
		detail += "; locked for session " + a.lockSession
	}
	return detail, nil
}
