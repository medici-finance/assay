package deskkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghissues "github.com/medici-finance/assay/forgeread/adapters/github"
)

// PositiveIssueState is a fresh, explicit issue observation, never listing absence.
type PositiveIssueState = ghissues.IssueState

const PositiveIssueStateBatchSize = ghissues.IssueStateBatchSize

type issueStateGetter interface {
	GetIssue(ForgeRepo, int) (*Issue, error)
}

type positiveIssueStateReader interface {
	positiveIssueStates(ForgeRepo, []int) ([]PositiveIssueState, error)
}

// ReadPositiveIssueStates retains the original wrapped reader. Only an explicit
// read capability opts into batching; every other backend/wrapper uses its own
// original GetIssue method. A failed batch never retries through another path.
func ReadPositiveIssueStates(f issueStateGetter, repo ForgeRepo, numbers []int) ([]PositiveIssueState, error) {
	seen := map[int]bool{}
	for _, n := range numbers {
		if n <= 0 || int64(n) > 1<<31-1 || seen[n] {
			return nil, Unverifiable("positive issue-state request has an invalid or repeated issue identity", nil)
		}
		seen[n] = true
	}
	if batch, ok := f.(positiveIssueStateReader); ok && len(numbers) > 1 && positiveIssueStateBatchAllowed(f) {
		return batch.positiveIssueStates(repo, numbers)
	}
	out := make([]PositiveIssueState, 0, len(numbers))
	for _, n := range numbers {
		issue, err := f.GetIssue(repo, n)
		if err != nil {
			return nil, Unverifiable(fmt.Sprintf("could-not-check: positive issue state for %s#%d could not be read", repo.Slug(), n), err)
		}
		if issue == nil || issue.Number != n || (issue.State != "open" && issue.State != "closed") {
			return nil, Unverifiable(fmt.Sprintf("positive issue-state observation for issue %d is missing or has an unknown identity/state", n), nil)
		}
		out = append(out, PositiveIssueState{Number: n, State: issue.State, Title: issue.Title})
	}
	return out, nil
}

// An unknown decorator may override GetIssue while embedding a backend whose
// optional method is promoted by Go. Its inherited batch method must not bypass
// that decorator's control. Only these explicit owners opt into forwarding.
func positiveIssueStateBatchAllowed(f issueStateGetter) bool {
	switch f.(type) {
	case *GitHubForge, *outboundForge, *readOnlyForge:
		return true
	default:
		return false
	}
}

// The wrappers delegate this fixed read explicitly. No backend unwrapping or
// write API is exposed; unknown inner wrappers keep their original serial path.
func (o *outboundForge) positiveIssueStates(repo ForgeRepo, numbers []int) ([]PositiveIssueState, error) {
	return ReadPositiveIssueStates(o.Forge, repo, numbers)
}

func (r *readOnlyForge) positiveIssueStates(repo ForgeRepo, numbers []int) ([]PositiveIssueState, error) {
	return ReadPositiveIssueStates(r.inner, repo, numbers)
}

type githubIssueStateAccess struct{ forge *GitHubForge }

func (a githubIssueStateAccess) IssueStateBatch(_ context.Context, slug string, numbers []int) ([]byte, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ValidRepoSlug(slug) || !ok || owner == "" || name == "" || strings.Contains(name, "/") || (a.forge.onlyRepo != "" && !strings.EqualFold(a.forge.onlyRepo, slug)) {
		return nil, Refused("positive issue-state read is outside the admitted repository")
	}
	query, err := ghissues.IssueStateQuery(numbers)
	if err != nil {
		return nil, Refused(err.Error())
	}
	variables, err := ghissues.IssueStateVariables(numbers)
	if err != nil {
		return nil, Refused(err.Error())
	}
	variables["owner"], variables["name"] = owner, name
	var raw json.RawMessage
	if err := a.forge.doJSON("POST", "/graphql", map[string]any{
		"query": query, "variables": variables,
	}, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (g *GitHubForge) positiveIssueStates(repo ForgeRepo, numbers []int) ([]PositiveIssueState, error) {
	out, err := ghissues.PositiveIssueStates(context.Background(), githubIssueStateAccess{g}, repo.Slug(), numbers)
	if err != nil {
		return nil, Unverifiable("cannot read positive issue states for "+repo.Slug(), err)
	}
	return out, nil
}
