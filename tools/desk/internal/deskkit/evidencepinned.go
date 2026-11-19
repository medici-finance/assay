package deskkit

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// GitHub's Contents API compares only the destination blob. A verified outcome
// also depends on other files, so use the existing Contents permission with an
// atomic whole-branch precondition. File CAS, shrink and idempotency checks have
// already run in the shared writer. No retry or mutable-head fallback is safe.
// Schema: https://docs.github.com/en/graphql/reference/commits
// parent is the commit the write is conditional on; extra are companion files
// committed with File (WriteFileInput.Companions), already checked absent there.
func (g *GitHubForge) writePinnedEvidence(repo ForgeRepo, in WriteFileInput, parent string, extra []CompanionFile, res *WriteFileResult) (*WriteFileResult, error) {
	if !fullCommitID(parent) || in.StartBranch != "" {
		return nil, Refused("pinned evidence requires an existing branch and full parent commit id")
	}
	additions := []any{map[string]any{"path": in.File, "contents": base64.StdEncoding.EncodeToString(in.Content)}}
	for _, c := range extra {
		additions = append(additions, map[string]any{"path": c.File, "contents": base64.StdEncoding.EncodeToString(c.Content)})
	}
	headline, body, _ := strings.Cut(in.Message, "\n")
	const query = `mutation PinnedEvidence($input:CreateCommitOnBranchInput!){createCommitOnBranch(input:$input){commit{oid parents(first:2){nodes{oid}}}}}`
	input := map[string]any{
		"branch":          map[string]any{"repositoryNameWithOwner": repo.Slug(), "branchName": strings.TrimPrefix(in.Branch, "refs/heads/")},
		"expectedHeadOid": parent,
		"message":         map[string]any{"headline": headline, "body": body},
		"fileChanges":     map[string]any{"additions": additions},
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
		return nil, Unverifiable("conditional evidence write not confirmed; reconcile target branch before retrying", nil)
	}
	w := out.Data.CreateCommitOnBranch
	if w == nil || w.Commit == nil || !fullCommitID(w.Commit.OID) || len(w.Commit.Parents.Nodes) != 1 || w.Commit.Parents.Nodes[0].OID != parent {
		return nil, Unverifiable("conditional evidence write receipt does not prove its inspected parent; reconcile before retrying", nil)
	}
	res.Changed = true
	res.CommitSHA = CommitSHA(w.Commit.OID)
	// The caller independently resolves this exact receipt for App attribution.
	return res, nil
}

// writePairedEvidence commits File with its companions (a witness row and its
// read-set file) in one commit. The Contents API writes one file, so this uses
// the pinned write's createCommitOnBranch, parented on the branch head it
// re-reads File at: File must still be the blob the checks before it judged
// (exists, priorSHA), so the pair keeps the single-file write's conditional
// precondition and adds the forge's own whole-branch one.
func (g *GitHubForge) writePairedEvidence(repo ForgeRepo, in WriteFileInput, exists bool, priorSHA string, res *WriteFileResult) (*WriteFileResult, error) {
	if err := companionPlan(in); err != nil {
		return nil, err
	}
	parent := in.ExpectedHead
	if parent == "" {
		head, err := g.branchHead(repo, in.Branch)
		if err != nil {
			return nil, err
		}
		parent = head
	}
	read := func(r ReadFileInput) (*FileContent, error) { return g.ReadFile(repo, r) }
	cur, err := read(ReadFileInput{File: in.File, Ref: parent})
	if err != nil && !IsForgeNotFound(err) {
		return nil, err
	}
	now := err == nil && cur != nil && cur.Exists
	if now != exists || (now && (cur.Kind != FileKindRegular || cur.SHA != priorSHA)) {
		return nil, Refused(fmt.Sprintf(
			"refusing a paired write to %s on %s: at the branch head %s it is not the version this write was judged against; re-fetch and retry",
			in.File, in.Branch, parent))
	}
	var add []CompanionFile
	for _, c := range in.Companions {
		want, err := companionWanted(read, c, in.Branch, parent)
		if err != nil {
			return nil, err
		}
		if want {
			add = append(add, c)
		}
	}
	return g.writePinnedEvidence(repo, in, parent, add, res)
}

// branchHead resolves a branch's head commit through the single-reference read.
func (g *GitHubForge) branchHead(repo ForgeRepo, branch string) (string, error) {
	clean, err := ValidateRefPath("heads/" + strings.TrimPrefix(branch, "refs/heads/"))
	if err != nil {
		return "", err
	}
	var ref struct {
		Object struct{ SHA, Type string }
	}
	if err := g.doJSON(http.MethodGet, fmt.Sprintf("/repos/%s/%s/git/ref/%s", repo.Owner, repo.Name, clean), nil, &ref); err != nil {
		return "", err
	}
	if ref.Object.Type != "commit" || !fullCommitID(ref.Object.SHA) {
		return "", Unverifiable("cannot resolve the head of "+branch+" a paired write is parented on", nil)
	}
	return ref.Object.SHA, nil
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
