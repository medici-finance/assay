// Package github is the GitHub open-issues read adapter. It owns the request shape (route, query,
// page size), the pagination walk and its ceiling, the pull-request filter, and the reduction of
// GitHub's issue JSON to forgeread.Issue. It reads only through the adapters.Access it is given.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/adapters"
)

const (
	// PerPage is the requested page size.
	PerPage = 100
	// MaxPages bounds the walk; a provider still paginating after it is a TruncatedError.
	MaxPages = 100
)

// Adapter reads a repository's open issues through Access.
type Adapter struct {
	access adapters.Access
	skip   func(title, authorLogin string) bool
}

// New returns an Adapter over access. skip, when non-nil, drops an issue the composition does not
// want listed (the desk's verifier-attestation issues); the adapter itself holds no such policy.
func New(access adapters.Access, skip func(title, authorLogin string) bool) *Adapter {
	return &Adapter{access: access, skip: skip}
}

type wireIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	HTMLURL     string    `json:"html_url"`
	PullRequest *struct{} `json:"pull_request"`
}

// OpenIssues walks the repository's open issues to exhaustion. The walk continues while EITHER the
// page came back full OR the provider's own signal says there is more, and stops only when both
// say there is not: inferring the end from a short page alone let a page shortened under load end
// the walk early with no error. Pull requests (served by the same route) are dropped.
func (a *Adapter) OpenIssues(ctx context.Context, repo string) ([]forgeread.Issue, error) {
	out := []forgeread.Issue{}
	for page := 1; page <= MaxPages; page++ {
		q := url.Values{}
		q.Set("state", "open")
		q.Set("per_page", strconv.Itoa(PerPage))
		q.Set("page", strconv.Itoa(page))
		pg, err := a.access.GetPage(ctx, repo, "issues", q)
		if err != nil {
			return nil, err
		}
		var chunk []wireIssue
		if err := json.Unmarshal(pg.Body, &chunk); err != nil {
			return nil, fmt.Errorf("could-not-check: cannot parse the open-issue page %d of %s: %w", page, repo, err)
		}
		for _, is := range chunk {
			if is.PullRequest != nil || (a.skip != nil && a.skip(is.Title, is.User.Login)) {
				continue
			}
			labels := make([]string, 0, len(is.Labels))
			for _, l := range is.Labels {
				labels = append(labels, l.Name)
			}
			out = append(out, forgeread.Issue{
				Number: is.Number, Title: is.Title, State: "open",
				AuthorLogin: is.User.Login, AuthorID: is.User.ID,
				Labels: labels, CreatedAt: is.CreatedAt, UpdatedAt: is.UpdatedAt, URL: is.HTMLURL,
			})
		}
		if len(chunk) < PerPage && !pg.More {
			return out, nil
		}
	}
	return nil, &adapters.TruncatedError{Repo: repo, Pages: MaxPages, PageSize: PerPage}
}
