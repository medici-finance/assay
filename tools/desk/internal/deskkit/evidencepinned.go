package deskkit

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// GitHub's Contents API compares only the destination blob. A verified outcome
// also depends on other files, so use the existing Contents permission with an
// atomic whole-branch precondition. File CAS, shrink and idempotency checks have
// already run in the shared writer. No retry or mutable-head fallback is safe.
// Schema: https://docs.github.com/en/graphql/reference/commits
func (g *GitHubForge) writePinnedEvidence(repo ForgeRepo, in WriteFileInput, res *WriteFileResult) (*WriteFileResult, error) {
	if !fullCommitID(in.ExpectedHead) || in.StartBranch != "" {
		return nil, Refused("pinned evidence requires an existing branch and full parent commit id")
	}
	headline, body, _ := strings.Cut(in.Message, "\n")
	const query = `mutation PinnedEvidence($input:CreateCommitOnBranchInput!){createCommitOnBranch(input:$input){commit{oid parents(first:2){nodes{oid}}}}}`
	input := map[string]any{
		"branch":          map[string]any{"repositoryNameWithOwner": repo.Slug(), "branchName": strings.TrimPrefix(in.Branch, "refs/heads/")},
		"expectedHeadOid": in.ExpectedHead,
		"message":         map[string]any{"headline": headline, "body": body},
		"fileChanges":     map[string]any{"additions": []any{map[string]any{"path": in.File, "contents": base64.StdEncoding.EncodeToString(in.Content)}}},
	}
	var out struct {
		Data struct {
			CreateCommitOnBranch *struct {
				Commit *struct {
					OID     string
					Parents struct{ Nodes []struct{ OID string } }
				}
			}
		}
		Errors []struct{ Type string }
	}
	if err := g.doJSON(http.MethodPost, "/graphql", map[string]any{"query": query, "variables": map[string]any{"input": input}}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, Unverifiable("conditional verified outcome write not confirmed; reconcile target branch before retrying", nil)
	}
	w := out.Data.CreateCommitOnBranch
	if w == nil || w.Commit == nil || !fullCommitID(w.Commit.OID) || len(w.Commit.Parents.Nodes) != 1 || w.Commit.Parents.Nodes[0].OID != in.ExpectedHead {
		return nil, Unverifiable("conditional verified outcome receipt does not prove its inspected parent; reconcile before retrying", nil)
	}
	res.Changed = true
	res.CommitSHA = CommitSHA(w.Commit.OID)
	// The caller independently resolves this exact receipt for App attribution.
	return res, nil
}

func fullCommitID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}
