package deskkit

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
)

const mirrorMetadataQuery = `query MirrorPullRequestMetadata($owner: String!, $name: String!, $after: String) {
  repository(owner: $owner, name: $name) {
    nameWithOwner
    pullRequests(first: 100, after: $after, states: [OPEN, CLOSED, MERGED], orderBy: {field: CREATED_AT, direction: ASC}) {
      totalCount
      pageInfo { hasNextPage endCursor }
      nodes {
        number title state createdAt updatedAt headRefOid baseRefOid
        author {
          login __typename
          ... on User { databaseId }
          ... on Bot { databaseId }
          ... on Mannequin { databaseId }
          ... on Organization { databaseId }
        }
        labels(first: 100) { totalCount pageInfo { hasNextPage } nodes { name } }
        reviewRequests(first: 100) {
          totalCount pageInfo { hasNextPage }
          nodes { requestedReviewer {
            __typename
            ... on User { login databaseId }
            ... on Bot { login databaseId }
            ... on Mannequin { login databaseId }
          } }
        }
      }
    }
  }
}`

type mirrorMetadataRequest struct {
	Query     string                  `json:"query"`
	Variables mirrorMetadataVariables `json:"variables"`
}

type mirrorMetadataVariables struct {
	Owner string  `json:"owner"`
	Name  string  `json:"name"`
	After *string `json:"after"`
}

type metadataConnection[T any] struct {
	Total *int `json:"totalCount"`
	Nodes []*T `json:"nodes"`
	Page  struct {
		More *bool   `json:"hasNextPage"`
		Next *string `json:"endCursor"`
	} `json:"pageInfo"`
}

func (c *metadataConnection[T]) complete() bool {
	return c != nil && c.Total != nil && c.Nodes != nil && *c.Total == len(c.Nodes) &&
		c.Page.More != nil && !*c.Page.More
}

type metadataActor struct {
	Login string `json:"login"`
	ID    int64  `json:"databaseId"`
	Kind  string `json:"__typename"`
}

func (a metadataActor) update() adapters.UpdateActor {
	login := a.Login
	if a.Kind == "Bot" {
		// GraphQL uses the bare slug. Mirror identities follow the REST spelling.
		login += "[bot]"
	}
	return adapters.UpdateActor{Login: login, ID: a.ID, Type: a.Kind}
}

type metadataLabel struct{ Name string }
type metadataReview struct{ RequestedReviewer *metadataActor }

type metadataPull struct {
	Number     int
	Title      *string
	State      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	HeadRefOID string `json:"headRefOid"`
	BaseRefOID string `json:"baseRefOid"`
	Author     *metadataActor
	Labels     *metadataConnection[metadataLabel]
	Reviews    *metadataConnection[metadataReview] `json:"reviewRequests"`
}

type metadataEnvelope struct {
	Data struct {
		Repository *struct {
			NameWithOwner string
			PullRequests  metadataConnection[metadataPull]
		}
	}
	Errors []struct{ Type string }
}

func metadataOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (p *metadataPull) observation(repo string) (adapters.ChangeUpdate, bool) {
	if p == nil || p.Number <= 0 || p.Title == nil || p.CreatedAt.IsZero() || p.UpdatedAt.Before(p.CreatedAt) ||
		!metadataOID(p.HeadRefOID) || !metadataOID(p.BaseRefOID) || !p.Labels.complete() || !p.Reviews.complete() {
		return adapters.ChangeUpdate{}, false
	}
	row := adapters.ChangeUpdate{IssueUpdate: adapters.IssueUpdate{
		Provider: "github", Repository: repo, Number: p.Number, Title: *p.Title,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, Labels: []string{},
	}, Kind: "pull-request", HeadSHA: p.HeadRefOID, BaseSHA: p.BaseRefOID, RequestedReviewers: []adapters.UpdateActor{}}
	switch p.State {
	case "OPEN", "CLOSED":
		row.State = strings.ToLower(p.State)
	case "MERGED":
		row.State, row.ClosureReason = "merged", "merged"
	default:
		return adapters.ChangeUpdate{}, false
	}
	if p.Author != nil {
		if p.Author.Login == "" || p.Author.ID <= 0 {
			return adapters.ChangeUpdate{}, false
		}
		row.Author = p.Author.update()
	}
	for _, label := range p.Labels.Nodes {
		if label == nil || label.Name == "" {
			return adapters.ChangeUpdate{}, false
		}
		row.Labels = append(row.Labels, label.Name)
	}
	for _, entry := range p.Reviews.Nodes {
		if entry == nil || entry.RequestedReviewer == nil {
			return adapters.ChangeUpdate{}, false
		}
		reviewer := entry.RequestedReviewer
		switch reviewer.Kind {
		case "Team", "EnterpriseTeam":
			// The canonical observation contains individual review requests only.
		case "User", "Bot", "Mannequin":
			if reviewer.Login == "" || reviewer.ID <= 0 {
				return adapters.ChangeUpdate{}, false
			}
			row.RequestedReviewers = append(row.RequestedReviewers, reviewer.update())
		default:
			return adapters.ChangeUpdate{}, false
		}
	}
	return row, true
}

func (s *mirrorIngestSource) readMetadata(ctx context.Context, attempt *mirrorAttemptState) (MirrorReadResult, error) {
	// The constructor requires an exact repository-bound, read-only GitHub backend.
	repo := s.source.repo
	cp := WithForgeReadContext(s.metadataBackend, ctx).(*GitHubForge)
	cp.Client = mirrorResponseClient(cp.Client)
	// Point instrumentation can change the GraphQL document. The native guard
	// admits the exact document, and still records attempts and decoded bytes.
	cp.Observer.Sink = nil
	out := MirrorReadResult{Kind: MirrorChanges, Instance: s.repository.Host,
		UpdateCoverage: adapters.UpdateCoverage{Provider: "github", Repository: repo.Slug(), Since: attempt.since}}
	req := mirrorMetadataRequest{Query: mirrorMetadataQuery, Variables: mirrorMetadataVariables{Owner: repo.Owner, Name: repo.Name}}
	seen := make(map[int]bool)
	cursors := make(map[string]bool)
	var lastCreated time.Time
	total := -1
	for int64(out.PageCount) < attempt.grant.MaxHTTPAttempts {
		body, err := json.Marshal(req)
		if err != nil {
			return MirrorReadResult{}, err
		}
		attempt.mu.Lock()
		attempt.metadataBody = body
		attempt.mu.Unlock()
		var reply metadataEnvelope
		if err := cp.doJSON(http.MethodPost, "/graphql", req, &reply); err != nil {
			return MirrorReadResult{}, err
		}
		out.PageCount++
		if len(reply.Errors) != 0 || reply.Data.Repository == nil ||
			!strings.EqualFold(reply.Data.Repository.NameWithOwner, repo.Slug()) {
			return MirrorReadResult{}, Unverifiable("metadata inventory response refused", nil)
		}
		page := reply.Data.Repository.PullRequests
		if page.Total == nil || *page.Total < 0 || page.Nodes == nil || len(page.Nodes) > 100 ||
			page.Page.More == nil || (total >= 0 && total != *page.Total) {
			return MirrorReadResult{}, Unverifiable("metadata inventory population changed or was incomplete", nil)
		}
		total = *page.Total
		for _, pull := range page.Nodes {
			row, valid := pull.observation(repo.Slug())
			if !valid || seen[row.Number] || row.CreatedAt.Before(lastCreated) {
				return MirrorReadResult{}, Unverifiable("metadata inventory contains an invalid, repeated or out-of-order pull request", nil)
			}
			lastCreated = row.CreatedAt
			seen[row.Number] = true
			if !row.UpdatedAt.Before(attempt.since) {
				out.Changes = append(out.Changes, row)
			}
		}
		if !*page.Page.More {
			if total != len(seen) {
				return MirrorReadResult{}, Unverifiable("metadata inventory did not account for its reported population", nil)
			}
			return out, nil
		}
		next := page.Page.Next
		if len(page.Nodes) == 0 || len(seen) >= total || next == nil || *next == "" || len(*next) > 4096 || cursors[*next] {
			return MirrorReadResult{}, Unverifiable("metadata inventory pagination refused", nil)
		}
		cursors[*next] = true
		req.Variables.After = next
	}
	out.Changes = nil
	out.MarkIncomplete(adapters.UpdatePageLimit)
	return out, nil
}
