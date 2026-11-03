package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
)

type updateActor struct {
	Username string `json:"username"`
	ID       int64  `json:"id"`
	Bot      *bool  `json:"bot"`
}

func (a updateActor) shared() adapters.UpdateActor {
	typ := ""
	if a.Bot != nil {
		if *a.Bot {
			typ = "Bot"
		} else {
			typ = "User"
		}
	}
	return adapters.UpdateActor{Login: a.Username, ID: a.ID, Type: typ}
}

type updatedWire struct {
	IID             int             `json:"iid"`
	Title           string          `json:"title"`
	Description     json.RawMessage `json:"description"`
	State           string          `json:"state"`
	Author          updateActor     `json:"author"`
	Labels          []string        `json:"labels"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	ClosedAt        string          `json:"closed_at"`
	MergedAt        string          `json:"merged_at"`
	SHA             string          `json:"sha"`
	SourceBranch    string          `json:"source_branch"`
	TargetBranch    string          `json:"target_branch"`
	SourceProjectID int64           `json:"source_project_id"`
	DiffRefs        *struct {
		BaseSHA string `json:"base_sha"`
		HeadSHA string `json:"head_sha"`
	} `json:"diff_refs"`
	Reviewers []updateActor `json:"reviewers"`
}

func (w updatedWire) issue(repo string, change bool) (adapters.IssueUpdate, error) {
	state := ""
	switch w.State {
	case "opened":
		state = "open"
	case "closed":
		state = "closed"
	case "merged":
		if change {
			state = "merged"
		}
	case "locked":
		if change {
			state = "locked"
		}
	}
	if w.IID <= 0 || state == "" {
		return adapters.IssueUpdate{}, fmt.Errorf("could-not-check: GitLab update has invalid identity or state")
	}
	out := adapters.IssueUpdate{Provider: "gitlab", Repository: repo, Number: w.IID, Title: w.Title, State: state, Author: w.Author.shared(), Labels: nil}
	if state == "merged" {
		out.ClosureReason = "merged"
	}
	if len(w.Description) > 0 {
		out.BodyKnown = true
		if string(w.Description) != "null" {
			if err := json.Unmarshal(w.Description, &out.Body); err != nil {
				return adapters.IssueUpdate{}, fmt.Errorf("could-not-check: invalid GitLab issue body")
			}
		}
	}
	if w.Labels != nil {
		out.Labels = append([]string{}, w.Labels...)
	}
	var err error
	if out.UpdatedAt, err = adapters.UpdateTimestamp(w.UpdatedAt, true); err != nil {
		return adapters.IssueUpdate{}, err
	}
	if out.CreatedAt, err = adapters.UpdateTimestamp(w.CreatedAt, false); err != nil {
		return adapters.IssueUpdate{}, err
	}
	if out.ClosedAt, err = adapters.UpdateTimestamp(w.ClosedAt, false); err != nil {
		return adapters.IssueUpdate{}, err
	}
	return out, nil
}

func updatedQuery(q adapters.UpdateQuery, page int) url.Values {
	v := url.Values{"state": {"all"}, "order_by": {"updated_at"}, "sort": {"desc"}, "per_page": {strconv.Itoa(PerPage)}, "page": {strconv.Itoa(page)}}
	if !q.Since.IsZero() {
		v.Set("updated_after", q.Since.UTC().Format(time.RFC3339Nano))
	}
	return v
}

func (a *Adapter) updatedPage(ctx context.Context, repo, rel string, q url.Values) ([]updatedWire, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	pg, err := a.access.GetPage(ctx, repo, rel, q)
	if err != nil {
		return nil, false, err
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	var rows []updatedWire
	if err := json.Unmarshal(pg.Body, &rows); err != nil || rows == nil {
		return nil, false, fmt.Errorf("could-not-check: invalid GitLab incremental %s page", rel)
	}
	return rows, pg.More, nil
}

// IssuesUpdatedSince reads the issues-only route in all lifecycle states. It
// retains uncertainty on any failed page and reports a page-ceiling explicitly.
func (a *Adapter) IssuesUpdatedSince(ctx context.Context, repo string, q adapters.UpdateQuery) (adapters.IssueUpdates, error) {
	out := adapters.IssueUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "gitlab", Repository: repo, Since: q.Since, ServerSideSince: !q.Since.IsZero()}, Records: []adapters.IssueUpdate{}}
	if err := adapters.ValidateUpdateRead(ctx, repo, q); err != nil {
		return adapters.IssueUpdates{}, err
	}
	for page := 1; page <= MaxPages; page++ {
		rows, more, err := a.updatedPage(ctx, repo, "issues", updatedQuery(q, page))
		if err != nil {
			return adapters.IssueUpdates{}, err
		}
		out.PageCount = page
		for _, w := range rows {
			r, err := w.issue(repo, false)
			if err != nil {
				return adapters.IssueUpdates{}, err
			}
			if r.UpdatedAt.After(out.ObservedUpTo) {
				out.ObservedUpTo = r.UpdatedAt
			}
			if !q.Since.IsZero() && r.UpdatedAt.Before(q.Since) {
				continue
			}
			out.Records = append(out.Records, r)
		}
		if !more {
			return out, nil
		}
	}
	out.Incomplete = true
	return out, nil
}

// ChangesUpdatedSince reads merge requests, preserving their own numbering and
// explicit merge state. Approval and pipeline coverage are not list properties.
func (a *Adapter) ChangesUpdatedSince(ctx context.Context, repo string, q adapters.UpdateQuery) (adapters.ChangeUpdates, error) {
	out := adapters.ChangeUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "gitlab", Repository: repo, Since: q.Since, ServerSideSince: !q.Since.IsZero()}, Records: []adapters.ChangeUpdate{}}
	if err := adapters.ValidateUpdateRead(ctx, repo, q); err != nil {
		return adapters.ChangeUpdates{}, err
	}
	for page := 1; page <= MaxPages; page++ {
		rows, more, err := a.updatedPage(ctx, repo, "merge_requests", updatedQuery(q, page))
		if err != nil {
			return adapters.ChangeUpdates{}, err
		}
		out.PageCount = page
		for _, w := range rows {
			base, err := w.issue(repo, true)
			if err != nil {
				return adapters.ChangeUpdates{}, err
			}
			if base.UpdatedAt.After(out.ObservedUpTo) {
				out.ObservedUpTo = base.UpdatedAt
			}
			if !q.Since.IsZero() && base.UpdatedAt.Before(q.Since) {
				continue
			}
			r := adapters.ChangeUpdate{IssueUpdate: base, Kind: "merge-request", HeadSHA: w.SHA, HeadBranch: w.SourceBranch, BaseBranch: w.TargetBranch, SourceProjectID: w.SourceProjectID, RequestedReviewers: nil}
			if w.DiffRefs != nil {
				r.BaseSHA = w.DiffRefs.BaseSHA
				if r.HeadSHA == "" {
					r.HeadSHA = w.DiffRefs.HeadSHA
				}
			}
			if r.HeadSHA == "" {
				return adapters.ChangeUpdates{}, fmt.Errorf("could-not-check: GitLab change has no exact head revision")
			}
			if r.MergedAt, err = adapters.UpdateTimestamp(w.MergedAt, false); err != nil {
				return adapters.ChangeUpdates{}, err
			}
			if w.Reviewers != nil {
				r.RequestedReviewers = []adapters.UpdateActor{}
			}
			for _, reviewer := range w.Reviewers {
				r.RequestedReviewers = append(r.RequestedReviewers, reviewer.shared())
			}
			out.Records = append(out.Records, r)
		}
		if !more {
			return out, nil
		}
	}
	out.Incomplete = true
	return out, nil
}
