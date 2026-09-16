package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
//     fallback): `gh pr list`, the same dependency the `--issues` metrics path
//     already shells out to (ghIssueMetricLister).
//   - GitLab: the project's merge requests over REST v4 (claimdecay_gitlab.go).
//     `gh` does not exist there and a GitLab project has no pull requests to
//     list, so routing the read by forge is the only way the pass can run at all
//     — before this, a GitLab-hosted adopter's claims never decayed and the board
//     silently held their briefs forever.
//
// Both readers are injected as package vars so the whole decay is exercised
// offline in tests.
//
// When neither reader can look — no `gh`, no token, an API error — the pass is
// LOUD and says so in its own words: `could-not-check: claims not decayed`, with
// the reason, carried out of the decay as a string so `--lint` and the emitted
// board can wear it too. A stderr-only NOTICE while the artifact still reads
// clean is a TWO-state instrument (docs/three-state-instrument-rule.md), and that
// is precisely what hid the GitLab gap.

// ghPRListJSONFields is the `--json` field set the GitHub reader asks `gh pr list`
// for. headRefName and state are what the decay keys on; isCrossRepository,
// headRepository and headRepositoryOwner are what tell a pull request opened from
// THIS repository from one opened from a fork — see sameRepoPR. A test pins the
// set so it cannot be trimmed back to the bare pair that let a fork's branch name
// decay a live claim.
const ghPRListJSONFields = "headRefName,state,isCrossRepository,headRepository,headRepositoryOwner"

// ghPRListJSON runs `gh pr list` for the repo rooted at root and returns its raw
// JSON. A package-level var so tests substitute recorded output and the parse is
// exercised with no `gh` and no network — the GitHub twin of the GitLab arm's
// gitlabHTTPDoer seam. A `gh` failure is returned as an error, never as an empty
// list an empty list would read as "no dead pull requests".
var ghPRListJSON = func(root string) ([]byte, error) {
	cmd := exec.Command("gh", "pr", "list",
		"--state", "all", "--limit", "1000",
		"--json", ghPRListJSONFields)
	// gh resolves the repo from its working directory; point it at the board's
	// root (the same repo listRemoteBranches read), mirroring `git -C root`.
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh pr list: %v %s", err, detail)
	}
	return out, nil
}

// ghPR is the subset of a `gh pr list` row the GitHub reader needs.
//
// IsCrossRepository, HeadRepository and HeadRepositoryOwner are not decoration:
// they are what tells a pull request opened from THIS repository from one opened
// from a fork. Only the former's headRefName names a branch of this repository,
// so only the former may ever contribute a corpse. They are pointers so that a
// field `gh` did not answer (an older `gh`, a head repository that no longer
// exists) is distinguishable from one it answered false or empty — sameRepoPR is
// the predicate; its doc carries the reasoning and the fail direction.
type ghPR struct {
	HeadRefName       string `json:"headRefName"`
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
// `gh pr list` returns every pull request TARGETING the repository, forks
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

// listMergedClosedBranches returns the set of head branch names whose PR is
// MERGED or CLOSED, for the repo rooted at `root`. It lists PRs in every state
// and keeps only the dead ones — an OPEN PR is deliberately absent from the set,
// because its branch is a live claim that must still be honoured. A `gh` failure
// is returned as an error; decayDeadClaims degrades to "decay nothing" so the
// board is never WORSE than the pre-decay superset (and never drops a live claim
// it could not verify, which would risk two sessions converging on one brief).
//
// Only a pull request opened from THIS repository may contribute a corpse (see
// sameRepoPR): a fork's headRefName may collide with a live claim here, and a PR
// whose head repository cannot be read at all is one this reader did not verify.
// Rows skipped for want of attribution are counted and reported, once.
//
// A package-level var so tests substitute a fake lister without a network call,
// exactly as listRemoteBranches / ghIssueMetricLister are stubbed.
var listMergedClosedBranches = func(root string) (map[string]bool, error) {
	out, err := ghPRListJSON(root)
	if err != nil {
		return nil, err
	}
	var raw []ghPR
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh pr list output: %w", err)
	}
	tracked := trackedGitHubRepo(root)
	dead := map[string]bool{}
	unattributable := 0
	for _, pr := range raw {
		name := strings.TrimSpace(pr.HeadRefName)
		if name == "" {
			continue
		}
		if same, attributed := sameRepoPR(pr, tracked); !same {
			if !attributed {
				unattributable++
			}
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(pr.State)) {
		case "MERGED", "CLOSED":
			dead[name] = true
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
	return dead, nil
}

// decayReader picks the change-state reader for the forge behind root's `origin`
// remote, and names it for the could-not-check message.
//
// forgeUnknown (a self-hosted host naming neither forge, or no remote at all)
// keeps the historical `gh` attempt: "could not tell" is not "confirmed not
// GitHub", and a GitHub Enterprise host that names neither word still answers
// `gh`. Its failure then reports as an ordinary could-not-check with the reason,
// which is the honest answer for a forge we could not identify.
func decayReader(root string) (read func(string) (map[string]bool, error), what string) {
	if detectForge(root) == forgeGitLab {
		return listMergedClosedBranchesGitLab, "merge-request state over the GitLab REST v4 API"
	}
	return listMergedClosedBranches, "PR state through `gh pr list`"
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
