package deskkit

// ownpr.go — the ONE own-PR checkout guard the worker-side write verbs share (#1901).
//
// A worker may write to a pull request only from a checkout of THAT pull request: deskreply
// replies on it, and `deskpr edit --pr N` corrects its body. (`deskpr edit --pr N` has one
// further, narrower admission for any other checkout — a same-repository PR whose current
// body already carries a link trailer, which the edit cannot change; see cmd/deskpr/edit.go
// requireLinkedSameRepoPR, #2085.) The guard used to be a bare
// branch-NAME compare (`the worktree's branch == the PR's head branch`), which refused a
// worktree sitting exactly on the PR's head commit under a differently-named branch. Git
// allows one worktree per branch, so a rework worker whose PR head branch is still checked
// out by an older worktree cannot check it out at all; it works on a neutral branch, pushes
// by explicit refspec, and was then refused every reply and body correction.
//
// The lineage that matters is the COMMIT, not the ref name. A checkout is the PR's own when
// EITHER:
//
//   - it is on a branch whose name is the PR's head branch (the original rule), OR
//   - its HEAD commit is EXACTLY the PR's head commit (headRefOid) — any branch name, or a
//     detached HEAD.
//
// EXACT equality, deliberately. A HEAD that is a DESCENDANT of the PR head (unpushed commits
// on top of it) is refused: a reply or a body correction from there describes code the PR
// does not carry yet. The remedy is to push first, after which HEAD == headRefOid and the
// guard admits the checkout. A HEAD that is an ancestor, or unrelated, is refused for the
// same reason.
//
// The PR must also be OPEN: a merged or closed PR is refused whatever the checkout, and that
// check runs FIRST so a checkout that happens to sit on a merged PR's head commit can never
// be read as licence to write to it.

import (
	"fmt"
	"strings"
)

// OwnPRLocal is the checkout side of the guard: the branch the worktree is on ("" when HEAD
// is detached) and the full hex SHA of its HEAD commit.
type OwnPRLocal struct {
	Branch string
	Head   string
}

// OwnPRRemote is the pull-request side of the guard, as the forge reports it.
type OwnPRRemote struct {
	Number  int
	State   string // OPEN | MERGED | CLOSED (any case)
	HeadRef string // the PR's head (source) branch name
	HeadOid string // the PR's head commit SHA; "" when the forge did not report one
}

// OwnPRMatch names which rule admitted a checkout.
type OwnPRMatch string

const (
	OwnPRByBranch OwnPRMatch = "branch"      // the worktree's branch IS the PR's head branch
	OwnPRByHead   OwnPRMatch = "head commit" // the worktree's HEAD IS the PR's head commit
)

// CheckOwnPR decides whether the checkout `local` is the PR `remote`'s own. It returns the
// rule that admitted it, or a Refused (exit 5) error whose message names both sides and the
// remedy. tool is the verb name the message is phrased for ("deskreply", "deskpr edit").
func CheckOwnPR(tool string, local OwnPRLocal, remote OwnPRRemote) (OwnPRMatch, error) {
	if !strings.EqualFold(strings.TrimSpace(remote.State), "OPEN") {
		return "", Refused(fmt.Sprintf(
			"refused: PR #%d is %s, not OPEN — %s only writes to open PRs", remote.Number, remote.State, tool))
	}
	if local.Branch != "" && local.Branch == remote.HeadRef {
		return OwnPRByBranch, nil
	}
	if sameCommit(local.Head, remote.HeadOid) {
		return OwnPRByHead, nil
	}
	where := fmt.Sprintf("on %q", local.Branch)
	if local.Branch == "" {
		where = "on a detached HEAD"
	}
	return "", Refused(fmt.Sprintf(
		"refused: this worktree is %s at %s but PR #%d's head branch is %q at %s — %s only writes to YOUR OWN PR: "+
			"the worktree's branch must be the PR's head branch, or its HEAD commit must BE the PR's head commit "+
			"(if you have commits on top of the PR head, push them to the PR's head branch first)",
		where, shortOid(local.Head), remote.Number, remote.HeadRef, shortOid(remote.HeadOid), tool))
}

// sameCommit reports whether two SHAs name the same commit. Both must be present and of the
// same length (no abbreviated-prefix matching: a short SHA is ambiguous by construction), and
// hex case is ignored.
func sameCommit(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" || len(a) != len(b) {
		return false
	}
	return strings.EqualFold(a, b)
}

func shortOid(sha string) string {
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return "(unknown commit)"
	}
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}
