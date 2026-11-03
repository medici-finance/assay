package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
)

type updateActor struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
	Type  string `json:"type"`
}

func (a updateActor) shared() adapters.UpdateActor {
	return adapters.UpdateActor{Login: a.Login, ID: a.ID, Type: a.Type}
}

type updatedWire struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        json.RawMessage `json:"body"`
	State       string          `json:"state"`
	StateReason string          `json:"state_reason"`
	User        updateActor     `json:"user"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	ClosedAt    string    `json:"closed_at"`
	MergedAt    string    `json:"merged_at"`
	PullRequest *struct{} `json:"pull_request"`
	Head        *struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
	Base *struct {
		SHA  string `json:"sha"`
		Ref  string `json:"ref"`
		Repo *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"base"`
	RequestedReviewers []updateActor `json:"requested_reviewers"`
}

func (w updatedWire) issue(repo string) (adapters.IssueUpdate, error) {
	out := adapters.IssueUpdate{Provider: "github", Repository: repo, Number: w.Number, Title: w.Title, State: w.State, ClosureReason: w.StateReason, Author: w.User.shared(), Labels: nil}
	if w.Number <= 0 || (w.State != "open" && w.State != "closed") {
		return adapters.IssueUpdate{}, fmt.Errorf("GitHub update has invalid issue identity or state")
	}
	if len(w.Body) > 0 {
		out.BodyKnown = true
		if string(w.Body) != "null" {
			if err := json.Unmarshal(w.Body, &out.Body); err != nil {
				return adapters.IssueUpdate{}, fmt.Errorf("could-not-check: invalid GitHub issue body")
			}
		}
	}
	if w.Labels != nil {
		out.Labels = []string{}
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
	for _, l := range w.Labels {
		out.Labels = append(out.Labels, l.Name)
	}
	return out, nil
}

func updatedQuery(q adapters.UpdateQuery, page int, issues bool) url.Values {
	v := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"desc"}, "per_page": {strconv.Itoa(PerPage)}, "page": {strconv.Itoa(page)}}
	// The pulls endpoint has no since parameter. It is walked in descending
	// update order until a page entirely predates the inclusive bound.
	if issues && !q.Since.IsZero() {
		// GitHub describes since as strictly after the supplied stamp. Request a
		// one-second overlap and enforce the caller's inclusive bound locally.
		// At the earliest representable stamp, omit the server bound instead.
		lower := q.Since.UTC().Add(-time.Second)
		if lower.Year() >= 1 {
			v.Set("since", lower.Format(time.RFC3339Nano))
		}
	}
	return v
}

func (a *Adapter) updatedPage(ctx context.Context, repo, rel string, q url.Values, coverage *adapters.UpdateCoverage) ([]updatedWire, bool, error) {
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
	if !coverage.ObservePage(len(pg.Body)) {
		return nil, false, nil
	}
	var rows []updatedWire
	if err := json.Unmarshal(pg.Body, &rows); err != nil || rows == nil {
		return nil, false, fmt.Errorf("could-not-check: invalid GitHub incremental %s page", rel)
	}
	return rows, pg.More || len(rows) >= PerPage, nil
}

// IssuesUpdatedSince lists all lifecycle states without treating pull requests
// from GitHub's shared issues route as issues. Complete means only that this
// bounded issue-list walk finished; thread/review/check coverage is not supplied.
func (a *Adapter) IssuesUpdatedSince(ctx context.Context, repo string, q adapters.UpdateQuery) (adapters.IssueUpdates, error) {
	out := adapters.IssueUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "github", Repository: repo, Since: q.Since, ServerSideSince: !q.Since.IsZero() && q.Since.Add(-time.Second).Year() >= 1}, Records: []adapters.IssueUpdate{}}
	if err := adapters.ValidateUpdateRead(ctx, repo, q); err != nil {
		return adapters.IssueUpdates{}, err
	}
	for page := 1; page <= MaxPages; page++ {
		rows, more, err := a.updatedPage(ctx, repo, "issues", updatedQuery(q, page, true), &out.UpdateCoverage)
		if err != nil {
			return adapters.IssueUpdates{}, err
		}
		if out.Incomplete {
			return out, nil
		}
		for _, w := range rows {
			if w.PullRequest != nil {
				continue
			}
			r, err := w.issue(repo)
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
	out.MarkIncomplete(adapters.UpdatePageLimit)
	return out, nil
}

// ChangesUpdatedSince reads the fixed pulls route in update order. It does not
// infer merge state from a closed issue: only an explicit merged_at marks merged.
func (a *Adapter) ChangesUpdatedSince(ctx context.Context, repo string, q adapters.UpdateQuery) (adapters.ChangeUpdates, error) {
	out := adapters.ChangeUpdates{UpdateCoverage: adapters.UpdateCoverage{Provider: "github", Repository: repo, Since: q.Since, ServerSideSince: false}, Records: []adapters.ChangeUpdate{}}
	if err := adapters.ValidateUpdateRead(ctx, repo, q); err != nil {
		return adapters.ChangeUpdates{}, err
	}
	for page := 1; page <= MaxPages; page++ {
		rows, more, err := a.updatedPage(ctx, repo, "pulls", updatedQuery(q, page, false), &out.UpdateCoverage)
		if err != nil {
			return adapters.ChangeUpdates{}, err
		}
		if out.Incomplete {
			return out, nil
		}
		allOlder := len(rows) > 0 && !q.Since.IsZero()
		for _, w := range rows {
			base, err := w.issue(repo)
			if err != nil {
				return adapters.ChangeUpdates{}, err
			}
			if base.UpdatedAt.After(out.ObservedUpTo) {
				out.ObservedUpTo = base.UpdatedAt
			}
			if q.Since.IsZero() || !base.UpdatedAt.Before(q.Since) {
				allOlder = false
			} else {
				continue
			}
			if w.Head == nil || w.Base == nil || w.Head.SHA == "" || w.Base.SHA == "" {
				return adapters.ChangeUpdates{}, fmt.Errorf("could-not-check: GitHub change has no exact head/base revision")
			}
			if w.Base.Repo != nil && w.Base.Repo.FullName != "" && !strings.EqualFold(w.Base.Repo.FullName, repo) {
				return adapters.ChangeUpdates{}, fmt.Errorf("could-not-check: GitHub change belongs to another repository")
			}
			r := adapters.ChangeUpdate{IssueUpdate: base, Kind: "pull-request", HeadSHA: w.Head.SHA, BaseSHA: w.Base.SHA, HeadBranch: w.Head.Ref, BaseBranch: w.Base.Ref, RequestedReviewers: nil}
			if w.Head.Repo != nil {
				r.SourceRepository = w.Head.Repo.FullName
			}
			if r.MergedAt, err = adapters.UpdateTimestamp(w.MergedAt, false); err != nil {
				return adapters.ChangeUpdates{}, err
			}
			if !r.MergedAt.IsZero() {
				r.State = "merged"
				r.ClosureReason = "merged"
			}
			if w.RequestedReviewers != nil {
				r.RequestedReviewers = []adapters.UpdateActor{}
			}
			for _, reviewer := range w.RequestedReviewers {
				r.RequestedReviewers = append(r.RequestedReviewers, reviewer.shared())
			}
			out.Records = append(out.Records, r)
		}
		if !more || allOlder {
			return out, nil
		}
	}
	out.MarkIncomplete(adapters.UpdatePageLimit)
	return out, nil
}
