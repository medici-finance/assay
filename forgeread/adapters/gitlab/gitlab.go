// Package gitlab is the GitLab open-issues read adapter. It owns the request shape (route, query,
// page size), the pagination walk and its ceiling, and the reduction of GitLab's issue JSON to
// forgeread.Issue. It reads only through the adapters.Access it is given.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/adapters"
)

const (
	// PerPage is the requested page size.
	PerPage = 100
	// MaxPages bounds the walk (2500 issues); a project still paginating after it is a
	// TruncatedError.
	MaxPages = 25
)

// Adapter reads a project's open issues through Access.
type Adapter struct {
	access adapters.Access
	skip   func(title, authorLogin string) bool
}

// New returns an Adapter over access. skip, when non-nil, drops an issue the composition does not
// want listed.
func New(access adapters.Access, skip func(title, authorLogin string) bool) *Adapter {
	return &Adapter{access: access, skip: skip}
}

type wireIssue struct {
	IID       int64      `json:"iid"`
	Title     string     `json:"title"`
	Labels    []string   `json:"labels"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	WebURL    string     `json:"web_url"`
	Author    *struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"author"`
}

func stamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// OpenIssues walks the project's open issues to exhaustion on the provider's own end-of-walk
// signal (X-Next-Page), never on a short page. GitLab numbers issues and merge requests in separate
// sequences served by separate routes, so nothing is filtered by kind. The author login stays the
// bare username, the rendering the desk's trust-events reader also produces.
func (a *Adapter) OpenIssues(ctx context.Context, repo string) ([]forgeread.Issue, error) {
	out := []forgeread.Issue{}
	for page := 1; page <= MaxPages; page++ {
		q := url.Values{}
		q.Set("state", "opened")
		q.Set("per_page", strconv.Itoa(PerPage))
		q.Set("page", strconv.Itoa(page))
		pg, err := a.access.GetPage(ctx, repo, "issues", q)
		if err != nil {
			return nil, err
		}
		var chunk []*wireIssue
		if err := json.Unmarshal(pg.Body, &chunk); err != nil {
			return nil, fmt.Errorf("could-not-check: cannot parse the open-issue page %d of %s: %w", page, repo, err)
		}
		for _, is := range chunk {
			if is == nil || (is.Author != nil && a.skip != nil && a.skip(is.Title, is.Author.Username)) {
				continue
			}
			fi := forgeread.Issue{
				Number: int(is.IID), Title: is.Title, State: "open",
				Labels: append([]string(nil), is.Labels...), CreatedAt: stamp(is.CreatedAt), UpdatedAt: stamp(is.UpdatedAt), URL: is.WebURL,
			}
			if is.Author != nil {
				fi.AuthorLogin, fi.AuthorID = is.Author.Username, is.Author.ID
			}
			out = append(out, fi)
		}
		if !pg.More {
			return out, nil
		}
	}
	return nil, &adapters.TruncatedError{Repo: repo, Pages: MaxPages, PageSize: PerPage}
}
