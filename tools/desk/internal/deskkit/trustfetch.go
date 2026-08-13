package deskkit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GraphQL trust-payload plumbing for the gate. The surfaces (issueboard/deskboard via
// `gh api graphql`, deskpost via its App-authenticated POST /graphql) fetch ONE query
// per untrusted-author item and hand the raw response here — deskkit owns the queries
// and the parsing so the three consumers cannot drift.
//
// Why GraphQL: the gate needs lastEditedAt (CONTENT-edit tracking) on bodies and
// comments — REST only exposes updated_at, which moves on unrelated events (labels)
// and would re-quarantine blessed items spuriously. Why numeric ids: databaseId is
// the permanent identity a recycled login cannot fake (see TrustedAuthorID).
//
// Bounded on purpose: first:100 on every connection and NO cursor-walking. A response
// with any hasNextPage=true reports complete=false and the caller QUARANTINES (fail
// closed — an external thread too busy to read in one page is never silently
// admitted; a blessing of such a thread waits for a human look).

// IssueTrustQuery fetches an issue's body-edit time and its comment content events.
const IssueTrustQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){issue(number:$number){lastEditedAt comments(first:100){pageInfo{hasNextPage} nodes{createdAt lastEditedAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}}}}}`

// PRTrustQuery fetches a PR's body-edit time plus all three comment surfaces:
// conversation comments, reviews, and review comments (via reviewThreads).
const PRTrustQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){lastEditedAt comments(first:100){pageInfo{hasNextPage} nodes{createdAt lastEditedAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}} reviews(first:100){pageInfo{hasNextPage} nodes{submittedAt lastEditedAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}} reviewThreads(first:100){pageInfo{hasNextPage} nodes{comments(first:100){pageInfo{hasNextPage} nodes{createdAt lastEditedAt author{login __typename ...on User{databaseId} ...on Bot{databaseId}}}}}}}}}`

// --- response shapes (only what the gate consumes) ---

type gqlActor struct {
	Login      string `json:"login"`
	Typename   string `json:"__typename"`
	DatabaseID int64  `json:"databaseId"`
}

// renderedLogin reconstructs the REST rendering TrustedAuthor expects: GraphQL Bot
// actors carry the BARE slug as login, which the trust set deliberately rejects
// (username-squatting defense) — so a Bot is re-suffixed to "<slug>[bot]". A plain
// User squatting a bot slug stays bare and therefore untrusted.
func (a *gqlActor) renderedLogin() string {
	if a == nil {
		return "" // deleted account (ghost) — untrusted, fail closed
	}
	if a.Typename == "Bot" {
		return a.Login + "[bot]"
	}
	return a.Login
}

type gqlPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
}

type gqlComment struct {
	CreatedAt    string    `json:"createdAt"`
	SubmittedAt  string    `json:"submittedAt"` // reviews only
	LastEditedAt string    `json:"lastEditedAt"`
	Author       *gqlActor `json:"author"`
}

type gqlConn struct {
	PageInfo gqlPageInfo  `json:"pageInfo"`
	Nodes    []gqlComment `json:"nodes"`
}

type gqlThreadConn struct {
	PageInfo gqlPageInfo `json:"pageInfo"`
	Nodes    []struct {
		Comments gqlConn `json:"comments"`
	} `json:"nodes"`
}

type gqlItem struct {
	LastEditedAt  string        `json:"lastEditedAt"`
	Comments      gqlConn       `json:"comments"`
	Reviews       gqlConn       `json:"reviews"`
	ReviewThreads gqlThreadConn `json:"reviewThreads"`
}

type gqlEnvelope struct {
	Data struct {
		Repository struct {
			Issue       *gqlItem `json:"issue"`
			PullRequest *gqlItem `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// parseTrustTime parses a GraphQL timestamp; "" (JSON null) is the zero time.
func parseTrustTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}

// ParseIssueTrustPayload parses an IssueTrustQuery response. complete=false means a
// connection overflowed first:100 — the caller must treat the item as NOT blessed.
// A malformed/errored payload is an error: the caller fails closed as unverifiable,
// never guessing either way.
func ParseIssueTrustPayload(raw []byte) (bodyEdited time.Time, events []ContentEvent, complete bool, err error) {
	item, err := parseEnvelope(raw, false)
	if err != nil {
		return time.Time{}, nil, false, err
	}
	return collectEvents(item, false)
}

// ParsePRTrustPayload parses a PRTrustQuery response (same contract).
func ParsePRTrustPayload(raw []byte) (bodyEdited time.Time, events []ContentEvent, complete bool, err error) {
	item, err := parseEnvelope(raw, true)
	if err != nil {
		return time.Time{}, nil, false, err
	}
	return collectEvents(item, true)
}

func parseEnvelope(raw []byte, pr bool) (*gqlItem, error) {
	var env gqlEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("cannot parse trust query response: %w", err)
	}
	if len(env.Errors) > 0 {
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			msgs = append(msgs, e.Message)
		}
		return nil, fmt.Errorf("trust query returned GraphQL errors: %s", strings.Join(msgs, "; "))
	}
	item := env.Data.Repository.Issue
	if pr {
		item = env.Data.Repository.PullRequest
	}
	if item == nil {
		return nil, fmt.Errorf("trust query returned no item (wrong number, or no access)")
	}
	return item, nil
}

func collectEvents(item *gqlItem, pr bool) (bodyEdited time.Time, events []ContentEvent, complete bool, err error) {
	bodyEdited, err = parseTrustTime(item.LastEditedAt)
	if err != nil {
		return time.Time{}, nil, false, fmt.Errorf("bad lastEditedAt: %w", err)
	}
	complete = !item.Comments.PageInfo.HasNextPage
	add := func(conn gqlConn) error {
		for i := range conn.Nodes {
			n := &conn.Nodes[i]
			created := n.CreatedAt
			if created == "" {
				created = n.SubmittedAt // reviews use submittedAt
			}
			if created == "" {
				continue // an unsubmitted (PENDING) review has no visible content yet
			}
			ct, cerr := parseTrustTime(created)
			if cerr != nil {
				return fmt.Errorf("bad event createdAt: %w", cerr)
			}
			et, eerr := parseTrustTime(n.LastEditedAt)
			if eerr != nil {
				return fmt.Errorf("bad event lastEditedAt: %w", eerr)
			}
			var id int64
			if n.Author != nil {
				id = n.Author.DatabaseID
			}
			events = append(events, ContentEvent{
				Author: n.Author.renderedLogin(), AuthorID: id, CreatedAt: ct, EditedAt: et,
			})
		}
		return nil
	}
	if err = add(item.Comments); err != nil {
		return time.Time{}, nil, false, err
	}
	if pr {
		if item.Reviews.PageInfo.HasNextPage || item.ReviewThreads.PageInfo.HasNextPage {
			complete = false
		}
		if err = add(item.Reviews); err != nil {
			return time.Time{}, nil, false, err
		}
		for _, th := range item.ReviewThreads.Nodes {
			if th.Comments.PageInfo.HasNextPage {
				complete = false
			}
			if err = add(th.Comments); err != nil {
				return time.Time{}, nil, false, err
			}
		}
	}
	return bodyEdited, events, complete, nil
}
