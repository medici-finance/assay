package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// This file adds the two read probes the service contract spatial fleet map
// needs: open pull requests and refs/dispatch/* claims. Both ride the same
// read-only GraphQL document shape [Client.deriveByNumber] already uses, so
// they get the read-only guarantee ([ReadOnlyTransport]) and the per-org
// token routing ([ResolveTokens] / [RoutingFetcher]) for free — no new
// transport, no new token plumbing, no new daemon (the read surface §Delivery).
//
// Everything here is a single bounded GraphQL page per repo, never an
// unbounded walk: a repo that has MORE open PRs or claims than the page size
// reports Capped rather than a silently truncated list (same discipline as
// [Client.listByLabel]'s pageBudget, scaled down because a fleet map is a
// glance at the ACTIVE set, not an archive).

// pullRequestPageSize bounds one repo's open-PR page. A repo that is
// carrying more than this many open PRs at once is itself the signal worth
// seeing (Capped), not a count to keep paging for on a spatial-glance view.
const pullRequestPageSize = 50

// claimPageSize bounds one repo's refs/dispatch/* page, same reasoning.
const claimPageSize = 100

// RawPullRequest is one open pull request as GitHub's GraphQL API reports
// it, still platform-shaped (this package's boundary — see internal/purity).
type RawPullRequest struct {
	Number    int
	Title     string
	Author    string // login; "" when the author account was deleted
	Draft     bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// Mergeable is GitHub's own enum: "CONFLICTING" | "MERGEABLE" | "UNKNOWN".
	// UNKNOWN means GitHub has not finished computing it yet — never read as
	// either of the other two.
	Mergeable string
	// ReviewDecision is GitHub's own enum: "APPROVED" | "CHANGES_REQUESTED" |
	// "REVIEW_REQUIRED" | "" (no review requested/submitted yet).
	ReviewDecision string
}

// RawClaim is one refs/dispatch/* ref as GitHub's GraphQL API reports it.
type RawClaim struct {
	// RefName is the full ref, e.g. "refs/dispatch/toolkit--x--08".
	RefName string
	// Holder is the commit author name at the ref tip. Empty when the tip
	// is not a commit (an annotated tag, say) or carries no author.
	Holder string
	// ClaimedAt is the commit's committed-date at the ref tip. Zero when it
	// could not be read (same "empty when it could not be read" reasoning
	// keeps a zero-value claim from ever being misread as "just now").
	ClaimedAt time.Time
}

// FleetHealth is the fleet map's own three-state read verdict for one repo
// probe — deliberately NOT [RepoHealth]/[domain.Assessment], which are
// wired to the escalation index's derive pipeline this feature does not
// join. OK=false always carries Reason; there is no "empty means clean"
// path anywhere below.
type FleetHealth struct {
	OK        bool
	Reason    string
	Capped    bool
	CheckedAt time.Time
}

// FetchOpenPullRequests reads one repo's open PRs in a single bounded
// GraphQL page (no REST list + N detail calls: mergeable/reviewDecision are
// only on the single-PR REST payload, but the GraphQL schema carries both
// on the list, which is what keeps this ONE request per repo).
func (c *Client) FetchOpenPullRequests(ctx context.Context, repo Repo) ([]RawPullRequest, FleetHealth) {
	now := time.Now().UTC()
	query := fmt.Sprintf(`query {
  repository(owner: %q, name: %q) {
    pullRequests(states: OPEN, first: %d, orderBy: {field: UPDATED_AT, direction: DESC}) {
      totalCount
      nodes {
        number title isDraft createdAt updatedAt mergeable reviewDecision
        author { login }
      }
    }
  }
}`, repo.Owner, repo.Name, pullRequestPageSize)

	raw, err := c.graphQL(ctx, query)
	if err != nil {
		return nil, FleetHealth{OK: false, Reason: "open-PR probe did not complete: " + err.Error(), CheckedAt: now}
	}

	var env struct {
		Data struct {
			Repository *struct {
				PullRequests struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						Number         int       `json:"number"`
						Title          string    `json:"title"`
						IsDraft        bool      `json:"isDraft"`
						CreatedAt      time.Time `json:"createdAt"`
						UpdatedAt      time.Time `json:"updatedAt"`
						Mergeable      string    `json:"mergeable"`
						ReviewDecision string    `json:"reviewDecision"`
						Author         *struct {
							Login string `json:"login"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, FleetHealth{OK: false, Reason: "open-PR response was not the expected JSON: " + err.Error(), CheckedAt: now}
	}
	if len(env.Errors) > 0 {
		return nil, FleetHealth{OK: false, Reason: "open-PR GraphQL errors: " + env.Errors[0].Message, CheckedAt: now}
	}
	if env.Data.Repository == nil {
		return nil, FleetHealth{OK: false, Reason: "repository not visible to this token — indistinguishable from 'does not exist', so could-not-check, not empty", CheckedAt: now}
	}

	pr := env.Data.Repository.PullRequests
	out := make([]RawPullRequest, 0, len(pr.Nodes))
	for _, n := range pr.Nodes {
		author := ""
		if n.Author != nil {
			author = n.Author.Login
		}
		out = append(out, RawPullRequest{
			Number: n.Number, Title: n.Title, Author: author, Draft: n.IsDraft,
			CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
			Mergeable: n.Mergeable, ReviewDecision: n.ReviewDecision,
		})
	}
	if pr.TotalCount > pullRequestPageSize {
		return out, FleetHealth{OK: false, Capped: true,
			Reason:    fmt.Sprintf("repo carries %d open PRs, over the %d-page bound — the page is a PREFIX, not the full set", pr.TotalCount, pullRequestPageSize),
			CheckedAt: now}
	}
	return out, FleetHealth{OK: true, CheckedAt: now}
}

// FetchDispatchClaims reads one repo's refs/dispatch/* claims in a single
// bounded GraphQL page.
func (c *Client) FetchDispatchClaims(ctx context.Context, repo Repo) ([]RawClaim, FleetHealth) {
	now := time.Now().UTC()
	query := fmt.Sprintf(`query {
  repository(owner: %q, name: %q) {
    refs(refPrefix: "refs/dispatch/", first: %d) {
      totalCount
      nodes {
        name
        target {
          oid
          ... on Commit { committedDate author { name } }
        }
      }
    }
  }
}`, repo.Owner, repo.Name, claimPageSize)

	raw, err := c.graphQL(ctx, query)
	if err != nil {
		return nil, FleetHealth{OK: false, Reason: "claim-ref probe did not complete: " + err.Error(), CheckedAt: now}
	}

	var env struct {
		Data struct {
			Repository *struct {
				Refs struct {
					TotalCount int `json:"totalCount"`
					Nodes      []struct {
						Name   string `json:"name"`
						Target struct {
							OID           string     `json:"oid"`
							CommittedDate *time.Time `json:"committedDate"`
							Author        *struct {
								Name string `json:"name"`
							} `json:"author"`
						} `json:"target"`
					} `json:"nodes"`
				} `json:"refs"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, FleetHealth{OK: false, Reason: "claim-ref response was not the expected JSON: " + err.Error(), CheckedAt: now}
	}
	if len(env.Errors) > 0 {
		return nil, FleetHealth{OK: false, Reason: "claim-ref GraphQL errors: " + env.Errors[0].Message, CheckedAt: now}
	}
	if env.Data.Repository == nil {
		return nil, FleetHealth{OK: false, Reason: "repository not visible to this token — indistinguishable from 'does not exist', so could-not-check, not empty", CheckedAt: now}
	}

	refs := env.Data.Repository.Refs
	out := make([]RawClaim, 0, len(refs.Nodes))
	for _, n := range refs.Nodes {
		claim := RawClaim{RefName: "refs/dispatch/" + trimRefsHeads(n.Name)}
		if n.Target.CommittedDate != nil {
			claim.ClaimedAt = *n.Target.CommittedDate
		}
		if n.Target.Author != nil {
			claim.Holder = n.Target.Author.Name
		}
		out = append(out, claim)
	}
	if refs.TotalCount > claimPageSize {
		return out, FleetHealth{OK: false, Capped: true,
			Reason:    fmt.Sprintf("repo carries %d refs/dispatch/* claims, over the %d-page bound — the page is a PREFIX, not the full set", refs.TotalCount, claimPageSize),
			CheckedAt: now}
	}
	return out, FleetHealth{OK: true, CheckedAt: now}
}

// trimRefsHeads is a defensive no-op today: the `refs(refPrefix:)` query
// already returns the SHORT name (with the prefix stripped by GitHub's own
// API), so this exists only so a future GraphQL schema change that starts
// returning the full ref cannot silently double the prefix in RawClaim's
// RefName — it is idempotent either way.
func trimRefsHeads(short string) string {
	const p = "refs/dispatch/"
	if len(short) >= len(p) && short[:len(p)] == p {
		return short[len(p):]
	}
	return short
}
