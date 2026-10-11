package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

type claimTarget struct {
	item      string
	pr, issue int
}

var claimStreamRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var claimNumberRE = regexp.MustCompile(`^[0-9]+$`)

// Invert only the dispatcher's documented key forms, bound to this repository's
// configured alias or basename. Unknown/historical prefixes need configuration;
// no filesystem path or forge target is guessed from an arbitrary claim key.
// Non-brief item IDs retain their full key so review lanes keep distinct probe state.
func decodeClaimTarget(key, repo string) (claimTarget, error) {
	owner, basename, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || basename == "" || strings.Contains(basename, "/") {
		return claimTarget{}, fmt.Errorf("invalid claim repository %q", repo)
	}
	tail := ""
	for _, label := range []string{deskkit.RepoShortLabel(repo), basename} {
		if rest, ok := strings.CutPrefix(key, label+"--"); ok {
			tail = rest
			break
		}
	}
	parts := strings.Split(tail, "--")
	if strings.HasPrefix(parts[0], "pr-") {
		n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "pr-"))
		if err == nil && n > 0 && deskkit.ValidateReviewClaimKey(key, repo, n) == nil {
			return claimTarget{item: key, pr: n}, nil
		}
	}
	if len(parts) == 1 && strings.HasPrefix(parts[0], "issue-") {
		n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "issue-"))
		if err == nil && n > 0 && claimNumberRE.MatchString(strings.TrimPrefix(parts[0], "issue-")) {
			return claimTarget{item: key, issue: n}, nil
		}
	} else if len(parts) == 2 && claimStreamRE.MatchString(parts[0]) && claimNumberRE.MatchString(parts[1]) {
		return claimTarget{item: parts[0] + "/" + parts[1]}, nil
	}
	return claimTarget{}, fmt.Errorf("claim %q has no recognized work-item mapping for %s", key, repo)
}

// Ordinary issues have no stream board or associated PR in the claim payload.
// Their state/hold labels pass through the same eligibility policy as PR claims;
// callers relabel the terminal reason to issue-closed.
func issueEligibilityState(issue *deskkit.Issue, number int) (loopengine.PRState, error) {
	if issue == nil || issue.Number != number || issue.IsPullRequest {
		return loopengine.PRState{}, fmt.Errorf("issue response does not match issue %d", number)
	}
	kind := deskkit.PROpen
	switch issue.State {
	case "open":
	case "closed":
		kind = deskkit.PRClosed
	default:
		return loopengine.PRState{}, fmt.Errorf("issue %d has unknown state %q", number, issue.State)
	}
	return loopengine.PRState{Kind: kind, Labels: issue.Labels}, nil
}

func reconcileForge(repoStr string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repoStr, "/")
	repo := deskkit.ForgeRepo{Owner: owner, Name: name}
	if !ok || owner == "" || name == "" {
		return nil, repo, fmt.Errorf("repo %q is not owner/name", repoStr)
	}
	role, _, err := deskkit.SessionTokenRole("desksupervise")
	if err != nil {
		return nil, repo, fmt.Errorf("cannot resolve a token role: %w", err)
	}
	forge, err := deskkit.ForgeFor(repo, role)
	if err != nil {
		return nil, repo, fmt.Errorf("cannot resolve a forge for %s: %w", repoStr, err)
	}
	return forge, repo, nil
}

func liveIssueReader() func(string, int) (*deskkit.Issue, error) {
	return func(repoStr string, n int) (*deskkit.Issue, error) {
		forge, repo, err := reconcileForge(repoStr)
		if err != nil {
			return nil, err
		}
		issue, err := forge.GetIssue(repo, n)
		if err != nil {
			return nil, fmt.Errorf("GetIssue %s#%d: %w", repoStr, n, err)
		}
		return issue, nil
	}
}
