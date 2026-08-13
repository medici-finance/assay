// Register-id collision detection — the mechanical form of #72's second unique failure
// shape (tracker#566 reused `id: F-39`).
//
// A worktree correctly cut from origin/main can still collide with a SIBLING branch that
// was ALSO correctly cut from origin/main, if both independently add a register entry
// (docs/streams/findings/, docs/streams/intake/) whose `id:`
// frontmatter happens to match. Neither branch contains the other's commits — there is no
// foreign commit and no merge masquerade for foreigncommit.go's checks to find — so today the collision is invisible on either branch alone and is only
// discovered once the second one merges and statusgen's existing duplicate-id lint reds
// main's CI (statusgen/registers.go: duplicateIDs).
//
// This file closes that gap at push time: for every register entry newly added on the
// pushed ref, it checks whether the same id is already claimed by a register entry present
// on some OTHER remote branch that is not itself already merged into origin/main (an
// in-flight sibling). It is local-only — it only consults already-fetched remote-tracking
// branches (`git branch -r`), the same data source foreigncommit.go's checks would use, so
// it adds no network/gh call — and fails open on any ambiguity, matching this tool's stated
// Fail-OPEN contract (main.go's package doc).
//
// NOTE on naming: as of this writing #746 (foreigncommit.go) is not yet merged to main, so
// this file deliberately does not share its gitOut/shaRe/branchIsAncestorOfMain helpers —
// they are reproduced here under a regID* prefix instead. Once #746 lands, a small follow-up
// should collapse the duplication into one shared helper file; until then this keeps the two
// PRs mechanically non-conflicting (no redeclared symbols) regardless of merge order.
package main

import (
	"errors"
	"os/exec"
	"regexp"
	"strings"
)

// regIDShaRe bounds localSHA before it ever reaches a constructed git argv — a real
// pre-push hook always supplies a full 40-char hex object id, so anything else is
// refused-into-skip rather than shelled out.
var regIDShaRe = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)

// regIDGitOut runs git in dir (empty = current process cwd) via the execCommand test seam
// shared with main.go, returning trimmed stdout. Any error is returned unwrapped; every
// caller in this file treats it as "cannot determine — skip".
func regIDGitOut(dir string, args ...string) (string, error) {
	cmd := execCommand("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// regIDBranchIsAncestorOfMain reports whether remote branch b is an ancestor of originMain
// via `git merge-base --is-ancestor`. determinate=false means the check itself could not be
// resolved — the caller must then skip rather than guess, per this file's fail-open contract.
func regIDBranchIsAncestorOfMain(dir, b, originMain string) (isAncestor, determinate bool) {
	cmd := execCommand("git", "merge-base", "--is-ancestor", b, originMain)
	if dir != "" {
		cmd.Dir = dir
	}
	err := cmd.Run()
	if err == nil {
		return true, true
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, true
	}
	return false, false
}

// registerEntryDirs are the register entry directories whose per-file `id:` frontmatter
// must be collision-free (spec/registers-v1.md). Trailing slash matches this file's own
// git-pathspec usage below.
var registerEntryDirs = []string{"docs/streams/findings/", "docs/streams/intake/"}

// idFrontmatterRe extracts a YAML frontmatter `id:` value — quoted or bare — from a
// register entry file's leading frontmatter block.
var idFrontmatterRe = regexp.MustCompile(`(?m)^id:\s*"?([A-Za-z0-9._-]+)"?\s*$`)

// registerIDCollision names one newly-pushed register entry whose id is already claimed by
// an entry on another in-flight branch.
type registerIDCollision struct {
	branch, id, ownPath, sourceBranch, sourcePath string
}

// inRegisterDir reports whether path falls under one of registerEntryDirs.
func inRegisterDir(path string) bool {
	for _, d := range registerEntryDirs {
		if strings.HasPrefix(path, d) {
			return true
		}
	}
	return false
}

// extractRegisterID returns the `id:` frontmatter value from blob content, or "" if none
// is present (not every file under a register dir need be a conforming entry — e.g. a
// generated view or a README; those are silently skipped).
func extractRegisterID(content string) string {
	m := idFrontmatterRe.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	return m[1]
}

// checkRegisterIDCollisions inspects register entry files newly ADDED by localSHA relative
// to origin/main and reports any whose id is also claimed by a register entry file present
// on some other remote branch that is not itself already merged into origin/main.
//
// dir is the repository to run git in (empty = process cwd). ownBranch's own remote-tracking
// ref (if already pushed) is excluded from the "other branch" search so an UPDATE push never
// flags itself against its own prior state.
//
// Fails open (nil, nil) on any ambiguity: origin/main unresolvable, localSHA not a
// well-formed object id or not resolvable, or a git error partway through — per-branch and
// per-file errors just skip that branch/file rather than aborting the whole check.
func checkRegisterIDCollisions(dir, ownBranch, localSHA string) ([]registerIDCollision, error) {
	if !regIDShaRe.MatchString(localSHA) {
		return nil, nil
	}
	originMain, err := regIDGitOut(dir, "rev-parse", "--verify", "--quiet", "origin/main")
	if err != nil || originMain == "" {
		return nil, nil // cannot resolve origin/main — nothing to compare against
	}
	if _, err := regIDGitOut(dir, "cat-file", "-e", localSHA); err != nil {
		return nil, nil // localSHA doesn't resolve in this repo — fail open
	}

	// Files newly added by this push, restricted to register entry directories.
	diffArgs := []string{"diff", "--name-status", "--diff-filter=A", originMain + ".." + localSHA, "--"}
	diffArgs = append(diffArgs, registerEntryDirs...)
	diffOut, derr := regIDGitOut(dir, diffArgs...)
	if derr != nil || strings.TrimSpace(diffOut) == "" {
		return nil, nil // no new register entries on this push — nothing to check
	}

	ownIDs := map[string]string{} // id -> own path
	for _, line := range strings.Split(diffOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "A" {
			continue
		}
		path := fields[len(fields)-1]
		if !inRegisterDir(path) {
			continue
		}
		content, cerr := regIDGitOut(dir, "show", localSHA+":"+path)
		if cerr != nil {
			continue // fail open on this file
		}
		if id := extractRegisterID(content); id != "" {
			ownIDs[id] = path
		}
	}
	if len(ownIDs) == 0 {
		return nil, nil
	}

	branchesOut, berr := regIDGitOut(dir, "branch", "-r")
	if berr != nil {
		return nil, nil
	}
	ownRemote := "origin/" + ownBranch
	var collisions []registerIDCollision
	for _, raw := range strings.Split(branchesOut, "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "* "))
		if b == "" || b == ownRemote || b == "origin/main" || strings.HasSuffix(b, "HEAD") || strings.Contains(b, "->") {
			continue
		}
		isAnc, determinate := regIDBranchIsAncestorOfMain(dir, b, originMain)
		if !determinate || isAnc {
			// Already merged into origin/main (or unresolvable) — its ids are already
			// covered by the origin/main-relative diff above via a fresh checkout, or we
			// simply can't say anything useful; either way, skip.
			continue
		}

		lsOut, lserr := regIDGitOut(dir, "ls-tree", "-r", "--name-only", b, "--")
		if lserr != nil {
			continue
		}
		for _, path := range strings.Split(lsOut, "\n") {
			path = strings.TrimSpace(path)
			if path == "" || !inRegisterDir(path) {
				continue
			}
			content, cerr := regIDGitOut(dir, "show", b+":"+path)
			if cerr != nil {
				continue
			}
			id := extractRegisterID(content)
			if id == "" {
				continue
			}
			if ownPath, clash := ownIDs[id]; clash {
				collisions = append(collisions, registerIDCollision{
					branch:       ownBranch,
					id:           id,
					ownPath:      ownPath,
					sourceBranch: b,
					sourcePath:   path,
				})
			}
		}
	}
	return collisions, nil
}
