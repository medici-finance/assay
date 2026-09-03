package main

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Local-branch collisions on `deskwt add`.
//
// `git worktree add --track -b <br> <path> <base>` fails outright when a local branch of
// that name already exists — and every worktree of a repo shares ONE refs store, so a
// branch left behind by an abandoned dispatch (git worktree remove does NOT delete the
// branch it was checked out on) blocks every later dispatch that derives the same branch
// name. Before this file, that arrived as a bare "git worktree add failed" with git's
// stderr trailing behind the tool's own config echo, which reads as a stuck claim rather
// than as a stray ref, and the operator had to shell out to raw git to learn the cause.
//
// The collision has exactly three shapes, and they are NOT one failure:
//
//   - the branch is checked out in another registered worktree — that worktree OWNS it,
//     so this is a refusal (5) that names the path;
//   - the branch is checked out nowhere but carries commits its comparison ref does not —
//     unfinished work, so this too is a refusal (5) that names the count;
//   - the branch is checked out nowhere and is 0 commits ahead of its comparison ref —
//     a leftover ref carrying nothing, so it is RECLAIMED (deleted, then recreated by the
//     ordinary `worktree add`) with an audit line saying so.
//
// The delete is `git update-ref -d <ref> <sha>`: a compare-and-delete against the sha the
// 0-ahead proof was taken on, so a branch that moved between the proof and the delete is
// not deleted at all. That is deliberately weaker than `git branch -D` — this tool has no
// force verb anywhere, and the tool's OWN proof (0 ahead, held by nobody) is the safety
// gate, exactly as it is on the remove path.

// branchHolders maps a full branch ref (refs/heads/<name>) to the RESOLVED path of the
// registered worktree that has it checked out, for every worktree of the repo rooted at
// dir. A worktree in detached HEAD contributes nothing. A git error propagates so callers
// fail CLOSED — an unreadable worktree list is never read as "nobody holds it", because
// that reading is the one that would delete a branch out from under a live worktree.
func branchHolders(dir string) (map[string]string, error) {
	out, err := runGit(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, deskkit.Unverifiable("cannot read `git worktree list` to find which worktree holds a branch", err)
	}
	holders := make(map[string]string)
	var cur string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = resolvePath(strings.TrimSpace(strings.TrimPrefix(line, "worktree ")))
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if cur != "" && ref != "" {
				holders[ref] = cur
			}
		}
	}
	return holders, nil
}

// resolveComparisonRef picks the ref a candidate stale branch is measured against: its
// configured upstream when that upstream RESOLVES to a commit, else the --base the caller
// asked to branch from. The fallback matters: a branch whose upstream config points at a
// ref that no longer exists would otherwise be unmeasurable, and an unmeasurable branch is
// a permanently stuck dispatch — the exact failure this file exists to end.
func resolveComparisonRef(dir, ref, base string) string {
	up, err := runGit(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", ref+"@{upstream}")
	if err != nil || up == "" {
		return base
	}
	if _, verr := runGit(dir, "rev-parse", "--verify", "--quiet", up+"^{commit}"); verr != nil {
		return base
	}
	return up
}

// reclaimStaleBranch resolves a local-branch collision BEFORE `git worktree add -b` can
// hit it. It returns the audit note for a reclaim (empty when there was no collision to
// resolve), or the named refusal / unverifiable for the two shapes that are not this
// tool's to clear. It NEVER creates or moves a branch: the ordinary `worktree add` still
// does that, on the same argv it always did.
func reclaimStaleBranch(dir, br, base string) (string, error) {
	ref := "refs/heads/" + br
	sha, err := runGit(dir, "rev-parse", "--verify", "--quiet", ref)
	if err != nil || sha == "" {
		// No such local branch: nothing collides, and `worktree add` proceeds untouched.
		return "", nil
	}

	holders, herr := branchHolders(dir)
	if herr != nil {
		return "", herr
	}
	if wt, held := holders[ref]; held {
		return "", deskkit.Refused("refused: branch " + br + " already exists and is CHECKED OUT in the worktree " +
			wt + " — that worktree owns it, so it is not a leftover. Finish and remove that worktree " +
			"(`deskwt remove " + wt + "`), or dispatch this item under a different --branch.")
	}

	cmp := resolveComparisonRef(dir, ref, base)
	ahead, aerr := runGit(dir, "rev-list", "--count", cmp+".."+ref)
	if aerr != nil {
		return "", deskkit.Unverifiable("branch "+br+" already exists and no worktree holds it, but its commits ahead of "+
			cmp+" could not be counted — refusing to delete a branch whose contents cannot be proven redundant. "+
			"Inspect it (`git log "+cmp+".."+br+"`) and delete it deliberately, or dispatch under a different --branch", aerr)
	}
	if ahead != "0" {
		return "", deskkit.Refused("refused: branch " + br + " already exists, is checked out in NO worktree, and carries " +
			ahead + " commit(s) not in " + cmp + " — that is unfinished work, not a leftover, so this tool will not " +
			"delete it. Push it, or delete it deliberately (`git branch -D " + br + "`), or dispatch under a different --branch.")
	}

	// Proven: no worktree holds it, and it carries nothing cmp does not. Compare-and-delete
	// against the sha the proof was taken on — a branch that moved in between is left alone.
	if _, derr := runGit(dir, "update-ref", "-d", ref, sha); derr != nil {
		return "", deskkit.Unverifiable("branch "+br+" is a stale leftover (0 commits ahead of "+cmp+
			", checked out in no worktree) but deleting its ref failed — it may have moved since the check", derr)
	}
	// Prove it is gone rather than assume the delete took.
	if still, serr := runGit(dir, "rev-parse", "--verify", "--quiet", ref); serr == nil && still != "" {
		return "", deskkit.Unverifiable("branch "+br+" still resolves after its ref was deleted", nil)
	}
	return "reclaimed stale local branch " + br + " (was " + shortSHA(sha) + ", 0 commits ahead of " + cmp +
		", checked out in no worktree)", nil
}

// shortSHA abbreviates a full object id for a human-readable audit line, leaving anything
// that is not a full-length id untouched.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
