package deskkit

import (
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type gitlabEvidenceSnapshot struct {
	project *gitlab.Project
}

// Reuse the visibility response's default branch for publication; do not repeat
// this project read at the write or the protected-branch fallback. The issue's
// edit provenance and resource-label events retain their existing native reads.
func (g *GitLabForge) evidenceProject(repo ForgeRepo) (*gitlabEvidenceSnapshot, error) {
	cl, err := g.client()
	if err != nil {
		return nil, err
	}
	p, _, err := cl.Projects.GetProject(repo.Slug(), nil)
	if err != nil {
		return nil, g.mapErr(http.MethodGet, fmt.Sprintf("/projects/%s", g.projectPath(repo)), err)
	}
	if p == nil || p.Visibility == "" || p.DefaultBranch == "" {
		return nil, Unverifiable("evidence project response has no visibility or default branch", nil)
	}
	return &gitlabEvidenceSnapshot{project: p}, nil
}
