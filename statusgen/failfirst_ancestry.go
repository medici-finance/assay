package main

// The fail-first base provenance question (verify-integrity/03), kept in its
// own file so the #2026 ancestry guard's allow-list entry (see
// TestNoAncestryWitnessJudge) covers exactly this one question and nothing else
// a later change adds to failfirst.go: is the base a row was shown red on the
// merge-base, and does a recorded delivering commit lie on HEAD's history with
// the base as its parent (#2721).

import (
	"errors"
	"os/exec"
	"regexp"
)

// hexSHARe is an abbreviated-or-full commit id as a witness records it.
var hexSHARe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// witnessCommit resolves a commit id a witness records (or an operator passes)
// to a full object id in this clone. The operand is validated as hex before git
// sees it, so no Evidence text reaches git as an option or a revision
// expression. why says what failed when full is "".
func witnessCommit(root, sha string) (full, why string) {
	if !hexSHARe.MatchString(sha) && !isObjectID(sha) {
		return "", "it is not a commit id"
	}
	read := gitCoreRead{native: "abbreviated witness base retains native disambiguation"}
	if isObjectID(sha) {
		read = ordinaryGitCore(root)
	}
	if read.err != nil {
		return "", "it names no commit in this clone"
	}
	if read.native == "" {
		why, e := read.repo.CommitTreeReadReason(sha)
		if e != nil {
			return "", "it names no commit in this clone"
		}
		if why != "" {
			read.native = why
		}
	}
	var err error
	if read.native == "" {
		hash, e := read.repo.Resolve(sha)
		full, err = hash.String(), e
	} else {
		full, err = gitOut(root, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
	}
	if err != nil || !isObjectID(full) {
		return "", "it names no commit in this clone"
	}
	return full, ""
}

// gitIsAncestor asks git whether the commit full is the commit of or an
// ancestor of it. Both operands are full object ids already resolved by
// witnessCommit or mergeBaseExact; ok is false with a nil error when git
// answers no.
func gitIsAncestor(root, full, of string) (bool, error) {
	if !isObjectID(full) || !isObjectID(of) {
		return false, errors.New("an operand is not an object id")
	}
	if full == of {
		return true, nil
	}
	err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", full, of).Run()
	if err == nil {
		return true, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}

// commitIsAncestor reports whether base names a commit in this clone that is
// the object `of`, or an ancestor of it. `of` is the merge-base of HEAD and the
// default branch, already resolved to an object id by mergeBaseExact, so the
// answer is "base is on the default branch's history at or before the point
// this branch left it": HEAD's own commit, a commit of the branch, and a
// sibling tree all answer no. why says what failed when it is not. A package
// var so tests can stand in for git.
var commitIsAncestor = func(root, base, of string) (bool, string) {
	if !hexSHARe.MatchString(base) {
		return false, "it is not a commit id"
	}
	if !isObjectID(of) {
		return false, "the merge-base of HEAD and origin/main could not be resolved"
	}
	full, why := witnessCommit(root, base)
	if why != "" {
		return false, why
	}
	ok, err := gitIsAncestor(root, full, of)
	if err != nil {
		return false, "the ancestry could not be checked: " + err.Error()
	}
	if !ok {
		return false, "it is not the merge-base of HEAD and origin/main, nor an ancestor of it (HEAD's own commit, a commit of this branch, or a sibling tree)"
	}
	return true, ""
}

// commitIsMergeBase reports whether base names exactly the commit `of` (the
// merge-base object id). It is the bound for a witness that records no
// delivering commit: any other base, an older ancestor included, says nothing
// about which change it is the tree without (#2721). A package var so tests
// can stand in for git.
var commitIsMergeBase = func(root, base, of string) (bool, string) {
	if !hexSHARe.MatchString(base) {
		return false, "it is not a commit id"
	}
	if !isObjectID(of) {
		return false, "the merge-base of HEAD and origin/main could not be resolved"
	}
	full, why := witnessCommit(root, base)
	if why != "" {
		return false, why
	}
	if full != of {
		return false, "it is not the merge-base of HEAD and origin/main, and the witness records no delivering commit; an older base is accepted only as the parent of a recorded --delivering-commit"
	}
	return true, ""
}

// deliveringParent resolves a delivering commit and its first parent (#2721
// option 2). The delivering commit must be HEAD or an ancestor of HEAD, so a
// commit of another branch or a sibling tree cannot be named, and it must have
// a parent: the repository's root commit delivers no change a base could be
// without. It returns the full delivering id, its first parent and "", or why
// it is refused. A package var so tests can stand in for git.
var deliveringParent = func(root, delivering string) (full, parent, why string) {
	full, why = witnessCommit(root, delivering)
	if why != "" {
		return "", "", why
	}
	head, err := gitOut(root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil || !isObjectID(head) {
		return "", "", "HEAD could not be resolved"
	}
	ok, err := gitIsAncestor(root, full, head)
	if err != nil {
		return "", "", "the ancestry could not be checked: " + err.Error()
	}
	if !ok {
		return "", "", "it is not HEAD or an ancestor of HEAD (a commit of another branch, or a sibling tree)"
	}
	parent, err = gitOut(root, "rev-parse", "--verify", "--quiet", full+"^1^{commit}")
	if err != nil || !isObjectID(parent) {
		return "", "", "it has no parent (the repository's root commit), so no base is the tree without it"
	}
	return full, parent, ""
}

// failFirstMergeBase is the merge-base of HEAD and the default branch as an
// object id, "" when it cannot be resolved. A package var so tests can stand in
// for git.
var failFirstMergeBase = func(root string) string {
	return mergeBaseExact(root, remoteMainRef)
}
