package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Dead-claim decay: statusgen builds its "stream/NN" claim set from OPEN origin
// branch heads (resolveClaims → listRemoteBranches → `git ls-remote --heads`).
// A branch is treated as an in-flight claim that subtracts from its stream's
// dispatch cap (perStreamCap). But `git ls-remote` is a pure ref view: it still
// reports the head of a branch whose PR already MERGED or CLOSED but that was
// never deleted. That corpse keeps consuming the cap forever, and a stream whose
// whole budget is spent on corpses shows ZERO board rows while its real backlog
// is silently held (HeldByStreamCap) — the class of silent-suppression bug this
// generator exists to prevent.
//
// The fix decays those dead claims at the data-collection boundary: before a
// branch name becomes a claim, drop it if its PR is merged or closed. A branch
// with an OPEN PR — or no PR yet (a worker that has pushed but not opened one) —
// is a live in-flight claim and is KEPT, so genuine parallelism is still
// serialized.
//
// Change state is not something a git-ref view can answer, so the decay reads it
// from the FORGE — and which forge that is decides the reader:
//
//   - GitHub (and an unclassifiable remote, where GitHub is the historical
//     fallback): one GraphQL read over the branches (ghBranchPRQuery), made in
//     process over net/http with GH_TOKEN / GITHUB_TOKEN (claimdecay_github.go).
//     It used to shell out to `gh api graphql`, and the board regen's runner has
//     no `gh`, so decay never ran there.
//   - GitLab: the project's merge requests over REST v4 (claimdecay_gitlab.go).
//     `gh` does not exist there and a GitLab project has no pull requests to
//     list, so routing the read by forge is the only way the pass can run at all
//     — before this, a GitLab-hosted adopter's claims never decayed and the board
//     silently held their briefs forever.
//
// Both readers are injected as package vars so the whole decay is exercised
// offline in tests.
//
// When neither reader can look — no token, an API error — the pass is
// LOUD and says so in its own words: `could-not-check: claims not decayed`, with
// the reason, carried out of the decay as a string so `--lint` and the emitted
// board can wear it too. A stderr-only NOTICE while the artifact still reads
// clean is a TWO-state instrument (docs/three-state-instrument-rule.md), and that
// is precisely what hid the GitLab gap.

// ghBranchPRQuery is the ONE read the GitHub reader makes: every branch of the
// repository with its current tip and its newest pull requests, paged 100
// branches at a time. It replaced `gh pr list --state all --limit 1000`, which
// read every pull request the repository ever had to learn the state of the few
// hundred branches that still exist — a cost that grew with history — and which
// silently stopped at 1,000 pull requests, so a corpse branch whose PR was older
// than the newest 1,000 was never decayed and held its brief off the board
// forever. Scoped to branches, the read costs one query per 100 live branches and
// has no cap.
//
// For each branch it asks for the newest ghPRWindow pull requests whose head IS
// that branch, newest first, with the fields sameRepoPR needs (isCrossRepository, headRepository,
// headRepositoryOwner) and the PR's head commit (headRefOid), which branchIsDead
// compares to the branch's current tip. $owner and $name come from the
// repository at root (githubDecayRepo).
//
// The per-branch window is ghPRWindow and is NOT unbounded: a branch whose window
// comes back full with no open pull request in it may have an open one behind the
// window, so branchIsDead refuses to call it dead (see ghPRWindow). The query does
// not page the inner connection: githubBranchPRPages drives its one outer cursor
// from the pageInfo in the response, and this read keeps that the only pageInfo.
const ghBranchPRQuery = `query($owner:String!,$name:String!,$endCursor:String){
  repository(owner:$owner,name:$name){
    refs(refPrefix:"refs/heads/",first:100,after:$endCursor){
      pageInfo{hasNextPage endCursor}
      nodes{name target{oid}
        associatedPullRequests(first:10,orderBy:{field:CREATED_AT,direction:DESC}){
          nodes{number state headRefName headRefOid isCrossRepository
            headRepository{name} headRepositoryOwner{login}}}}}}}`

// ghPRWindow is the number of pull requests per branch ghBranchPRQuery asks for;
// TestGHPRWindowMatchesQuery keeps the two equal. A branch with this many pull
// requests whose head it is, none of them open, is TRUNCATED: a pull request
// older than the window — an open one among them — is invisible to the read. The
// raw count is compared, not the counted one, so a window filled by pull requests
// the reader skips (a fork's, an unattributable one) is truncated too. Such a
// branch is a live claim and is reported as could-not-check, never decayed on
// the strength of a listing that may be missing its open pull request.
const ghPRWindow = 10

// ghBranchPRJSON runs the branch read for the repo rooted at root and returns
// the raw pages as one JSON array. The production read is in process, over
// net/http (githubBranchPRPages, claimdecay_github.go): the board regen runs on
// a runner with no `gh`, and the `gh api graphql` shell-out this replaced left
// decay unable to run there at all. A package-level var so tests substitute
// recorded output and the parse is exercised with no network. A failed read is
// returned as an error, never as an empty listing that would read as "no dead
// branches".
var ghBranchPRJSON = githubBranchPRPages

// ghBranchPage is one page of the branch read.
type ghBranchPage struct {
	Data struct {
		Repository *struct {
			Refs struct {
				Nodes []ghBranch `json:"nodes"`
			} `json:"refs"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ghBranch is one branch: its name, its tip, and its newest pull requests.
type ghBranch struct {
	Name   string `json:"name"`
	Target *struct {
		Oid string `json:"oid"`
	} `json:"target"`
	AssociatedPullRequests struct {
		Nodes []ghPR `json:"nodes"`
	} `json:"associatedPullRequests"`
}

// ghPR is the subset of a pull request the GitHub reader needs.
//
// IsCrossRepository, HeadRepository and HeadRepositoryOwner are not decoration:
// they are what tells a pull request opened from THIS repository from one opened
// from a fork. Only the former's headRefName names a branch of this repository,
// so only the former may ever contribute a corpse. They are pointers so that a
// field the API did not answer (a head repository that no longer exists) is
// distinguishable from one it answered false or empty — sameRepoPR is the
// predicate; its doc carries the reasoning and the fail direction.
type ghPR struct {
	Number            int    `json:"number"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	State             string `json:"state"`
	IsCrossRepository *bool  `json:"isCrossRepository"`
	HeadRepository    *struct {
		Name string `json:"name"`
	} `json:"headRepository"`
	HeadRepositoryOwner *struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
}

// headRepo returns the pull request's head repository as "owner/name", or "" when
// either half is unreadable.
func (pr ghPR) headRepo() string {
	if pr.HeadRepository == nil || pr.HeadRepositoryOwner == nil {
		return ""
	}
	owner := strings.TrimSpace(pr.HeadRepositoryOwner.Login)
	name := strings.TrimSpace(pr.HeadRepository.Name)
	if owner == "" || name == "" {
		return ""
	}
	return owner + "/" + name
}

// trackedGitHubRepo returns the "owner/name" of the repository behind root's
// `origin` remote, or "" when it cannot be read. It is the GitHub twin of the
// GitLab arm's numeric projectID: where it IS known, sameRepoPR checks each pull
// request's head against it, and where it is not the reader falls back to
// isCrossRepository alone.
func trackedGitHubRepo(root string) string {
	raw, err := remoteOriginURL(root)
	if err != nil {
		return ""
	}
	return remoteProjectPath(raw)
}

// sameRepoPR reports whether pr was opened FROM the tracked repository — the only
// case in which its headRefName names a branch of that repository — and whether
// the reader could attribute it at all.
//
// A pull-request listing can return pull requests TARGETING the repository, forks
// included, and a fork's headRefName is a name chosen inside the fork — unscoped
// to this repository entirely. Anyone who can fork and open a pull request (the
// ordinary contribution bar; no elevated access) could otherwise open and close a
// throwaway PR named after a live claim branch and have the decay drop that live
// claim, letting a second worker be dispatched onto a brief already in flight.
// That is a direct breach of the pass's load-bearing invariant: decay may only
// ever shrink the claim set to what it VERIFIED is dead. The GitLab arm closed
// this with sameProjectMR; this is the same guard on the same fail direction.
//
// Two signals, either of which is enough to REFUSE and both of which must agree
// to ADMIT: isCrossRepository (the forge's own verdict) and the head repository's
// owner/name against the tracked repository (belt and braces, where the tracked
// name is known). A pull request carrying neither signal — or only a head name
// with no tracked name to compare it to — is one this reader cannot attribute,
// and an unattributable PR is indistinguishable from a fork's. It is NOT read as
// same-repo: attributed is false so the caller counts and reports the skip.
// Under-decay is the safe direction; over-decay is the one that loses work.
func sameRepoPR(pr ghPR, tracked string) (same, attributed bool) {
	head := pr.headRepo()
	cross := pr.IsCrossRepository
	if head == "" && cross == nil {
		return false, false
	}
	if cross != nil && *cross {
		return false, true
	}
	if head != "" && tracked != "" && !strings.EqualFold(head, tracked) {
		// The forge says same-repo (or did not say) but the head names another
		// repository: a row we do not understand, so no conclusion is drawn.
		return false, true
	}
	if cross != nil {
		return true, true
	}
	if tracked == "" {
		return false, false
	}
	return true, true
}

// listMergedClosedBranches returns the set of branch names that are corpses: NO
// pull request opened from this repository on the branch is OPEN, the branch's
// NEWEST such pull request is MERGED or CLOSED, and the branch's tip is still that
// pull request's head commit. Everything else is a live claim and is absent from
// the set:
//
//   - no pull request yet (a worker that pushed but has not opened one);
//   - ANY counted pull request that is OPEN, however old — an older open pull
//     request with a newer closed one from the same branch is still in flight, so
//     the rule is "any is open", not "the newest is open";
//   - a full window of ghPRWindow pull requests with none open: an open one may
//     sit behind the window, so the branch is not concluded dead (reported once);
//   - a branch pushed again after its pull request closed (its tip moved past the
//     PR's head) — a reused branch name is new work, not the old corpse. The
//     all-PRs reader keyed on the NAME alone and marked it dead if ANY pull
//     request ever used it, the over-decay direction that lets two workers take
//     one brief.
//
// A branch whose tip or head commit cannot be read is not concluded dead. A read
// failure, or a GraphQL error on any page, is returned as an error;
// decayDeadClaims degrades to "decay nothing" so the board is never WORSE than
// the pre-decay superset.
//
// Only a pull request opened from THIS repository may make a branch a corpse
// (see sameRepoPR). Pull requests skipped for want of attribution are counted
// and reported, once; so are branches whose window was full.
//
// A package-level var so tests substitute a fake lister without a network call,
// exactly as listRemoteBranches / defaultIssueMetricLister are stubbed.
var listMergedClosedBranches = func(root string) (map[string]bool, error) {
	out, err := ghBranchPRJSON(root)
	if err != nil {
		return nil, err
	}
	var pages []ghBranchPage
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, fmt.Errorf("parsing the branch pull-request listing: %w", err)
	}
	tracked := trackedGitHubRepo(root)
	dead := map[string]bool{}
	unattributable := 0
	truncated := 0
	for i, pg := range pages {
		if len(pg.Errors) > 0 {
			return nil, fmt.Errorf("the branch pull-request listing returned an error on page %d: %s", i+1, pg.Errors[0].Message)
		}
		if pg.Data.Repository == nil {
			return nil, fmt.Errorf("the branch pull-request listing returned no repository on page %d", i+1)
		}
		for _, b := range pg.Data.Repository.Refs.Nodes {
			isDead, skipped, full := branchIsDead(b, tracked)
			unattributable += skipped
			if full {
				truncated++
			}
			if isDead {
				dead[b.Name] = true
			}
		}
	}
	// Skipping a PR we could not attribute is the safe direction, but a floor
	// presented as a total is still a lie (three-state rule 2) — so if any row was
	// skipped for want of a head repository, say how many, once.
	if unattributable > 0 {
		fmt.Fprintf(os.Stderr, "could-not-check: dead-claim decay skipped %d pull request(s) whose head repository could not be read, "+
			"so their branches were NOT decayed and may still be consuming their stream's dispatch cap; an unattributable pull request "+
			"cannot be told from a fork's, and a fork's branch name does not name a branch of this repository\n", unattributable)
	}
	if truncated > 0 {
		fmt.Fprintf(os.Stderr, "could-not-check: dead-claim decay saw a full window of %d pull requests, none open, on %d branch(es), "+
			"so those branches were NOT decayed and may still be consuming their stream's dispatch cap; an open pull request "+
			"older than the window would be invisible to this read\n", ghPRWindow, truncated)
	}
	return dead, nil
}

// branchIsDead decides one branch. Only pull requests headed by this branch and
// attributable to this repository count (sameRepoPR); skipped counts the
// unattributable ones passed over. Two rules, both required for a corpse:
//
//   - no counted pull request in the window is OPEN — an open PR anywhere on the
//     branch is a live claim, whatever order it was opened in (an older open PR
//     with a newer closed one from the same branch is still in flight);
//   - the NEWEST counted pull request is merged or closed AND the branch tip is
//     still that PR's head commit — a push after the close is new work.
//
// No counted pull request at all is a live claim (pushed, not yet opened). So is
// a branch whose window is FULL (ghPRWindow rows) with none open: truncated
// reports it, and the caller says so once.
func branchIsDead(b ghBranch, tracked string) (dead bool, skipped int, truncated bool) {
	name := strings.TrimSpace(b.Name)
	if name == "" || b.Target == nil || strings.TrimSpace(b.Target.Oid) == "" {
		return false, 0, false
	}
	var newest *ghPR
	for i := range b.AssociatedPullRequests.Nodes {
		pr := b.AssociatedPullRequests.Nodes[i]
		if strings.TrimSpace(pr.HeadRefName) != name {
			continue
		}
		same, attributed := sameRepoPR(pr, tracked)
		if !same {
			if !attributed {
				skipped++
			}
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(pr.State)) {
		case "MERGED", "CLOSED":
			if newest == nil {
				newest = &pr
			}
		default:
			return false, skipped, false // an open PR on the branch: a live claim
		}
	}
	if newest == nil {
		return false, skipped, false // no pull request of this repository: a live claim
	}
	if len(b.AssociatedPullRequests.Nodes) >= ghPRWindow {
		return false, skipped, true // an open PR may sit behind the window: not concluded dead
	}
	head := strings.TrimSpace(newest.HeadRefOid)
	return head != "" && strings.EqualFold(head, strings.TrimSpace(b.Target.Oid)), skipped, false
}

// decayReader picks the change-state reader for the forge behind root's `origin`
// remote, and names it for the could-not-check message.
//
// forgeUnknown (a self-hosted host naming neither forge, an SSH host alias, or no
// remote at all) keeps the historical GitHub attempt: "could not tell" is not
// "confirmed not GitHub". A GitHub Enterprise host that names neither word is
// reached by setting GITHUB_GRAPHQL_URL. A failure reports as an ordinary
// could-not-check with the reason, which is the honest answer for a forge we
// could not identify.
func decayReader(root string) (read func(string) (map[string]bool, error), what string) {
	if detectForge(root) == forgeGitLab {
		return listMergedClosedBranchesGitLab, "merge-request state over the GitLab REST v4 API"
	}
	return listMergedClosedBranches, "branch pull-request state through the GitHub GraphQL API"
}

// decayDeadClaims removes, from an open-branch list, the branches whose PR or
// merge request has already merged or closed — the corpses that
// `git ls-remote --heads` still reports and that would otherwise keep consuming
// their stream's dispatch cap.
//
// It returns the surviving branches AND the could-not-check reason: empty when
// the decay actually ran, and otherwise the reason the change-state read failed.
// The caller carries that reason onto ClaimSource so `--lint` and the emitted
// board wear it (claims.go). A stderr NOTICE alone left the artifact reading
// clean — a two-state instrument — and that is how a forge on which the pass
// could NEVER run went six days unnoticed.
//
// Fail direction (load-bearing, unchanged): the decay can only SHRINK the claim
// set, so a failed read falls back to the full open-branch set — exactly the
// pre-decay behaviour. That is the safe direction: the worst case is the old
// over-holding (a corpse still counts), never a NEW under-holding that drops a
// live open-change claim and lets two sessions pick the same brief.
func decayDeadClaims(root string, branches []string) ([]string, string) {
	if len(branches) == 0 {
		return branches, ""
	}
	read, what := decayReader(root)
	dead, err := read(root)
	if err != nil {
		reason := fmt.Sprintf("%s could not be read: %v", what, err)
		fmt.Fprintf(os.Stderr, "could-not-check: claims not decayed — %s. "+
			"Open branches of already merged/closed changes are still counted as claims, so they may be consuming their "+
			"stream's dispatch cap and silently holding its briefs back; regenerate once the forge read is available to release them.\n",
			reason)
		return branches, reason
	}
	if len(dead) == 0 {
		return branches, ""
	}
	live := make([]string, 0, len(branches))
	for _, b := range branches {
		if dead[b] {
			continue
		}
		live = append(live, b)
	}
	return live, ""
}
