package main

import (
	"os/exec"
	"strings"
)

// exactRefCommit returns the commit the fully-qualified ref names, or "" when
// that EXACT ref does not exist. A full refname handed to git as a revision is
// still expanded by git's short-name rules: when refs/remotes/origin/main is
// absent, git reads refs/refs/remotes/origin/main, refs/tags/refs/remotes/origin/main,
// refs/heads/refs/remotes/origin/main or refs/remotes/refs/remotes/origin/main in
// its place. Any of those on a commit of the PR makes that commit the base.
// `git show-ref --verify` matches the full name only, so the ref's existence is
// shown before its object id is used; everything downstream takes the object id,
// never the name.
func exactRefCommit(root, ref string) string {
	if !strings.HasPrefix(ref, "refs/") || exec.Command("git", "check-ref-format", ref).Run() != nil {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "show-ref", "--verify", "--hash", "--", ref).Output()
	if err != nil {
		return ""
	}
	oid := strings.TrimSpace(string(out))
	if !isObjectID(oid) {
		return ""
	}
	out, err = exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "--end-of-options", oid+"^{commit}").Output()
	if err != nil {
		return ""
	}
	commit := strings.TrimSpace(string(out))
	if !isObjectID(commit) {
		return ""
	}
	return commit
}

// mergeBaseExact is where a HARD-CODED base ref (remoteMainRef, or
// refs/remotes/origin/<the PR's base branch>) becomes a merge-base with HEAD:
// the ref is resolved exactly (exactRefCommit) and merge-base runs on its object
// id. "" when the ref does not exist or no merge-base resolves, so each caller
// takes its own unresolvable-base path. Callers: the corroborate lane, both
// offline register guards, register-ID grandfathering, stream-cap, the unrun
// gate and the Verify-row obligation derivation. NOT covered: the revisions an
// operator supplies (--consumers --base, the diff-lint base, mergecheck --base),
// whose DEFAULT is the same fixed ref but which git still resolves by name.
// TestMergeBaseChokePoint fails on any `git merge-base` call outside its
// allow-list, and TestFixedBaseNotHandedToHelpers on a fixed ref passed into one
// of that allow-list's by-name helpers.
//
// Inside a run's git read session (gitbatch.go) the answer is memoised per root
// and ref for that run only, so the fixed-base callers of one --lint resolve the
// ref once rather than once each; outside a session every call resolves afresh.
func mergeBaseExact(root, ref string) string {
	s := activeGitReads
	if s == nil {
		return resolveMergeBaseExact(root, ref)
	}
	key := root + "\x00" + ref
	s.mu.Lock()
	defer s.mu.Unlock()
	if mb, ok := s.exactBases[key]; ok {
		return mb
	}
	mb := resolveMergeBaseExact(root, ref)
	s.exactBases[key] = mb
	return mb
}

// resolveMergeBaseExact is mergeBaseExact's uncached body.
func resolveMergeBaseExact(root, ref string) string {
	oid := exactRefCommit(root, ref)
	if oid == "" {
		return ""
	}
	out, err := exec.Command("git", "-C", root, "merge-base", "--end-of-options", oid, "HEAD").Output()
	if err != nil {
		return ""
	}
	mb := strings.TrimSpace(string(out))
	if !isObjectID(mb) {
		return ""
	}
	return mb
}

// isObjectID reports whether s is one full hex object id (SHA-1 or SHA-256).
func isObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
