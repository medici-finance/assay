package main

// The fail-first base provenance question (verify-integrity/03), kept in its
// own file so the #2026 ancestry guard's allow-list entry (see
// TestNoAncestryWitnessJudge) covers exactly this one query and nothing else a
// later change adds to failfirst.go.

import (
	"errors"
	"os/exec"
	"regexp"
)

// hexSHARe is an abbreviated-or-full commit id as a witness records it.
var hexSHARe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// commitIsAncestor reports whether base names a commit in this clone that is
// the object `of`, or an ancestor of it. `of` is the merge-base of HEAD and the
// default branch, already resolved to an object id by mergeBaseExact, so the
// answer is "base is on the default branch's history at or before the point
// this branch left it": HEAD's own commit, a commit of the branch, and a
// sibling tree all answer no. why says what failed when it is not. The operand
// is validated as hex before git sees it, then resolved to a full object id, so
// no Evidence text reaches git as an option or a revision expression. A
// package var so tests can stand in for git.
var commitIsAncestor = func(root, base, of string) (bool, string) {
	if !hexSHARe.MatchString(base) {
		return false, "it is not a commit id"
	}
	if !isObjectID(of) {
		return false, "the merge-base of HEAD and origin/main could not be resolved"
	}
	full, err := gitOut(root, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil || !isObjectID(full) {
		return false, "it names no commit in this clone"
	}
	if full == of {
		return true, ""
	}
	cmd := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", full, of)
	err = cmd.Run()
	if err == nil {
		return true, ""
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, "it is not the merge-base of HEAD and origin/main, nor an ancestor of it (HEAD's own commit, a commit of this branch, or a sibling tree)"
	}
	return false, "the ancestry could not be checked: " + err.Error()
}

// failFirstMergeBase is the merge-base of HEAD and the default branch as an
// object id, "" when it cannot be resolved. A package var so tests can stand in
// for git.
var failFirstMergeBase = func(root string) string {
	return mergeBaseExact(root, remoteMainRef)
}
