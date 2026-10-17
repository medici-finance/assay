package main

import (
	"os"
	"strings"
)

// rollbackFailedAdd closes the "failed add leaves its target behind" defect class (#2746).
//
// `git worktree add` can fail AFTER it has created the target directory and registered the
// worktree: a post-checkout hook that exits non-zero is the reproducible case — git
// deliberately keeps the worktree then, and exits non-zero. cmdAdd refused to clobber an
// existing target, so the directory it was left holding made the very next `deskwt add` of
// the same name (a corrected re-dispatch) die with "target already exists" until an
// operator found and removed it by hand. Every later step of cmdAdd already rolls back what
// it made; this is the same rollback for the git step itself, shared by all three arms so
// no arm can be the one that forgets.
//
// OWNERSHIP. cmdAdd proved the target did not exist before git ran (the never-clobbered
// Lstat), so whatever sits there now was made by this invocation. The one way that proof can
// be stale is a concurrent `add` of the same name winning the race; git then says the path
// "already" exists or is registered, and that worktree is NOT ours. Any "already" in git's
// stderr therefore skips the rollback, and so does a target that is absent (git cleaned up
// after itself — nothing to do).
//
// The returned string is appended to the caller's error message; "" when nothing was done.
// A rollback that itself fails is reported, never swallowed: the operator then has the exact
// path to remove.
func rollbackFailedAdd(guard *pathGuard, dir, target, gitStderr string) string {
	if strings.Contains(gitStderr, "already") {
		return ""
	}
	rt := resolvePath(target)
	if _, err := os.Lstat(rt); err != nil {
		return ""
	}
	if err := removeWorktreeDir(guard, dir, rt); err != nil {
		return "; rollback of " + target + " failed (" + err.Error() + ") — remove it with `deskwt remove " + target + "` before retrying"
	}
	return "; worktree " + target + " ROLLED BACK (the failed add left it behind)"
}
