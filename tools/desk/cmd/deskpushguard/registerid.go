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
// This file closes that gap at push time: for every register entry newly ADDED or MODIFIED on
// the pushed ref, it checks whether the same id is already claimed by a register entry present
// on some OTHER remote branch that is not itself already merged into origin/main (an
// in-flight sibling). It is local-only — it only consults already-fetched remote-tracking
// branches (`git branch -r`), the same data source foreigncommit.go's checks would use, so
// it adds no network/gh call — and fails open on any ambiguity, matching this tool's stated
// Fail-OPEN contract (main.go's package doc).
//
// ADVISORY, aligned with the authoritative gate. statusgen's duplicateIDs lint
// (splitFrontmatter + yaml.v3) is the authoritative fail-closed CI gate; this pre-push layer
// is an additive advisory signal that always defers to it. So the local signal AGREES with the
// authoritative one instead of failing open more often than it must, the id is extracted here
// with the SAME frontmatter-scoped yaml.v3 semantics statusgen uses (see extractRegisterID) —
// not the looser unanchored line regex it previously used, which diverged from statusgen on
// `id :` (whitespace before the colon), duplicate `id:` keys, and an `id:` line appearing in
// the body OUTSIDE the frontmatter block.
//
// NOTE on shared helpers: #746 (foreigncommit.go) has landed, so this file now shares that
// file's shaRe / gitOut / branchIsAncestorOfMain helpers rather than reproducing them under a
// regID* prefix — collapsing the duplication flagged inline on #759.
package main

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// registerEntryDirs are the register entry directories whose per-file `id:` frontmatter
// must be collision-free (spec/registers-v1.md). Trailing slash matches this file's own
// git-pathspec usage below.
var registerEntryDirs = []string{"docs/streams/findings/", "docs/streams/intake/"}

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

// splitFrontmatter isolates a register entry file's leading YAML frontmatter block, kept
// behaviourally identical to statusgen's canonical splitFrontmatter (statusgen/parse.go) so
// this advisory layer and the authoritative CI gate agree on WHERE the frontmatter is.
// Line-based and tolerant: the opening and closing fences only need to trim to "---", and
// CRLF is normalized to LF up front so a Windows-authored (CRLF) file parses identically to
// an LF one instead of silently swallowing the whole file as frontmatter. Returns
// (frontmatter, body, error); an absent first fence or an unterminated block is an error, on
// which the caller fails open.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", errors.New("no frontmatter: first line must be ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", errors.New("unterminated frontmatter")
}

// registerIDFrontmatter is the single frontmatter field this guard reads — the same `id` key
// statusgen's register-entry structs unmarshal (statusgen/registerentries.go).
type registerIDFrontmatter struct {
	ID string `yaml:"id"`
}

// extractRegisterID returns the `id:` frontmatter value from a register entry file's content,
// read with the SAME semantics as statusgen's authoritative duplicateIDs gate: splitFrontmatter
// to scope the read to the leading frontmatter block, then yaml.v3 to read the `id` key.
//
// It returns "" (skip — fail open) when there is no frontmatter, the frontmatter does not
// parse as yaml (yaml.v3 rejects duplicate `id:` keys — a shape statusgen also reds on
// authoritatively), or no `id` key is present (not every file under a register dir is a
// conforming entry — e.g. a generated view or a README; those are silently skipped).
//
// This deliberately replaces the earlier unanchored `(?m)^id:\s*"?(...)"?\s*$` line regex,
// whose divergences from statusgen this advisory layer used to miss: `id :` (whitespace before
// the colon, which yaml.v3 accepts and the regex did not), duplicate `id:` keys (the regex
// silently took the first; yaml.v3 rejects the file), and an `id:` line matched in the body
// OUTSIDE the frontmatter block (the multiline regex scanned the whole file).
func extractRegisterID(content string) string {
	fm, _, err := splitFrontmatter(content)
	if err != nil {
		return ""
	}
	var e registerIDFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &e); err != nil {
		return ""
	}
	return e.ID
}

// checkRegisterIDCollisions inspects register entry files newly ADDED or MODIFIED by localSHA
// relative to origin/main and reports any whose id is also claimed by a register entry file
// present on some other remote branch that is not itself already merged into origin/main.
//
// dir is the repository to run git in (empty = process cwd). ownBranch's own remote-tracking
// ref (if already pushed) is excluded from the "other branch" search so an UPDATE push never
// flags itself against its own prior state.
//
// Fails open (nil, nil) on any ambiguity: origin/main unresolvable, localSHA not a
// well-formed object id or not resolvable, or a git error partway through — per-branch and
// per-file errors just skip that branch/file rather than aborting the whole check.
func checkRegisterIDCollisions(dir, ownBranch, localSHA string) ([]registerIDCollision, error) {
	if !shaRe.MatchString(localSHA) {
		return nil, nil
	}
	// FULLY-QUALIFIED remote-tracking ref, not the bare short name `origin/main`
	// (#885): a stray local `refs/heads/origin/main` decoy would otherwise shadow
	// the real remote tip and drive the added-file diff off a stale base. The
	// sibling foreigncommit.go in this same binary already spells it in full.
	originMain, err := gitOut(dir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main")
	if err != nil || originMain == "" {
		return nil, nil // cannot resolve origin/main — nothing to compare against
	}
	if _, err := gitOut(dir, "cat-file", "-e", localSHA); err != nil {
		return nil, nil // localSHA doesn't resolve in this repo — fail open
	}

	// Files newly added OR modified by this push, restricted to register entry directories.
	// MODIFY is included alongside ADD (--diff-filter=AM): an id changed in place on an
	// existing entry is a MODIFY, not an ADD, and the ADD-only scan would let it past this
	// advisory layer even though statusgen's duplicateIDs gate still sees the changed id.
	diffArgs := []string{"diff", "--name-status", "--diff-filter=AM", originMain + ".." + localSHA, "--"}
	diffArgs = append(diffArgs, registerEntryDirs...)
	diffOut, derr := gitOut(dir, diffArgs...)
	if derr != nil || strings.TrimSpace(diffOut) == "" {
		return nil, nil // no new/changed register entries on this push — nothing to check
	}

	ownIDs := map[string]string{} // id -> own path
	for _, line := range strings.Split(diffOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || (fields[0] != "A" && fields[0] != "M") {
			continue
		}
		path := fields[len(fields)-1]
		if !inRegisterDir(path) {
			continue
		}
		content, cerr := gitOut(dir, "show", localSHA+":"+path)
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

	branchesOut, berr := gitOut(dir, "branch", "-r")
	if berr != nil {
		return nil, nil
	}
	ownRemote := "origin/" + ownBranch
	var collisions []registerIDCollision
	for _, raw := range strings.Split(branchesOut, "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "* "))
		// Skip git's symbolic default-branch pointer `origin/HEAD` by EXACT match — the
		// prior HasSuffix(b, "HEAD") was too broad and would also skip a real branch whose
		// name merely ends in "HEAD" (e.g. origin/detached-HEAD), silently leaving its ids
		// unchecked. The `->` guard still catches the `origin/HEAD -> origin/main` form that
		// some `git branch -r` outputs render on one line.
		if b == "" || b == ownRemote || b == "origin/main" || b == "origin/HEAD" || strings.Contains(b, "->") {
			continue
		}
		isAnc, determinate := branchIsAncestorOfMain(dir, b, originMain)
		if !determinate || isAnc {
			// Already merged into origin/main (or unresolvable) — its ids are already
			// covered by the origin/main-relative diff above via a fresh checkout, or we
			// simply can't say anything useful; either way, skip.
			continue
		}

		lsOut, lserr := gitOut(dir, "ls-tree", "-r", "--name-only", b, "--")
		if lserr != nil {
			continue
		}
		for _, path := range strings.Split(lsOut, "\n") {
			path = strings.TrimSpace(path)
			if path == "" || !inRegisterDir(path) {
				continue
			}
			content, cerr := gitOut(dir, "show", b+":"+path)
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
