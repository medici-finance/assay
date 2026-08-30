package main

// ghfetch.go — the ONLY network code in statusgen (derived-board/03).
//
// It reads GitHub's REST API only (never the GraphQL API — Verify row 8 pins this)
// to turn a repo's pull requests and their reviews into the witness records the
// lifecycle fold consumes. It is the "fetch half" prlink.go (brief 02) named as
// belonging here.
//
// Three-state contract (docs/three-state-instrument-rule.md), enforced at every
// exit: a fetch either SUCCEEDS (lookedAt=true, records returned) or reports
// lookedAt=false WITH the HTTP status / transport error as the reason. It NEVER
// returns an empty slice that reads like "nothing found" for a call that actually
// failed — an auth failure is an `unknown`, never a clean board.
//
// Testability: all HTTP goes through the httpDoer seam, so go test injects a
// recorded-response double and never touches the network (the offline envelope).
// Read-only endpoints only: GET /pulls and GET /pulls/{n}/reviews.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// httpDoer is the seam every request goes through. *http.Client satisfies it; a
// test injects a recorded-response double.
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ghClient is a minimal read-only GitHub REST client.
type ghClient struct {
	doer  httpDoer
	base  string // API base, e.g. https://api.github.com
	token string
}

// githubAPIBase is the default REST base.
const githubAPIBase = "https://api.github.com"

// newGHClient builds a client with a bounded-timeout http.Client. token may be
// empty (an unauthenticated call still reaches the API; a private repo then 404s,
// which surfaces as a could-not-check, not a clean board).
func newGHClient(token string) *ghClient {
	return &ghClient{
		doer:  &http.Client{Timeout: 30 * time.Second},
		base:  githubAPIBase,
		token: token,
	}
}

// resolveGitHubToken reads the API token from --token-file if given, else from
// GITHUB_TOKEN. A missing token is not an error here (the caller decides whether a
// repo needs auth); an unreadable --token-file IS an error.
func resolveGitHubToken(tokenFile string) (string, error) {
	if tokenFile != "" {
		b, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("reading --token-file %s: %w", tokenFile, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return strings.TrimSpace(os.Getenv("GITHUB_TOKEN")), nil
}

// ghPull is the subset of a REST pull object this tool reads.
type ghPull struct {
	Number         int    `json:"number"`
	State          string `json:"state"` // "open" | "closed"
	Draft          bool   `json:"draft"`
	Body           string `json:"body"`
	MergedAt       string `json:"merged_at"`        // "" (null) when not merged
	MergeCommitSHA string `json:"merge_commit_sha"` // the merge SHA on the base branch
	Head           struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// ghRestReview is the subset of a REST review object this tool reads. It is a
// SEPARATE type from corroborate.go's ghReview (which models the `gh pr view
// --json reviews` shape and cannot carry commit_id); this one reads the REST
// endpoint's own JSON, where commit_id IS returned.
type ghRestReview struct {
	State    string `json:"state"` // "APPROVED" | "CHANGES_REQUESTED" | "DISMISSED" | ...
	CommitID string `json:"commit_id"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ListPRs fetches every open-or-closed PR for repo, pages through them, and
// returns one PRRecord per PR that carries EXACTLY ONE `Brief:` trailer (the only
// PR→brief edge — no title or branch-name guessing). A merged PR (state=closed +
// merged_at set) becomes prMerged; an open PR becomes prOpen; a closed-unmerged PR
// becomes prClosed. lookedAt is false with an HTTP/transport reason on any failure.
func (c *ghClient) ListPRs(repo string) (prs []PRRecord, lookedAt bool, reason string) {
	const perPage = 100
	const maxPages = 20 // 2000 PRs; a hard bound so a bad Link loop cannot spin forever
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/repos/%s/pulls?state=all&per_page=%d&page=%d", c.base, repo, perPage, page)
		body, status, err := c.get(url)
		if err != nil {
			return nil, false, err.Error()
		}
		if status != http.StatusOK {
			return nil, false, httpReason(status, body)
		}
		var pulls []ghPull
		if err := json.Unmarshal(body, &pulls); err != nil {
			return nil, false, fmt.Sprintf("HTTP 200 but response did not parse as a pulls list: %v", err)
		}
		for _, p := range pulls {
			ref, ok := singleBriefTrailer(p.Body)
			if !ok {
				continue // unlinked or multi-linked: not a derivable PR→brief edge
			}
			rec := PRRecord{
				BriefRef: ref,
				Number:   p.Number,
				HeadSHA:  p.Head.SHA,
			}
			switch {
			case p.MergedAt != "":
				rec.State = prMerged
				rec.MergeSHA = p.MergeCommitSHA
			case p.State == "open":
				rec.State = prOpen
				rec.Draft = p.Draft
			default:
				rec.State = prClosed
			}
			prs = append(prs, rec)
		}
		if len(pulls) < perPage {
			break // last page
		}
	}
	return prs, true, ""
}

// ReviewsAtHead reports whether pr carries an APPROVED review at headSHA — the
// gate:model `done` witness (the same "approval must report at the merged head"
// property autoflip.go enforces). lookedAt is false with a reason on any failure.
func (c *ghClient) ReviewsAtHead(repo string, pr int, headSHA string) (approved, atHead, lookedAt bool, reason string) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews?per_page=100", c.base, repo, pr)
	body, status, err := c.get(url)
	if err != nil {
		return false, false, false, err.Error()
	}
	if status != http.StatusOK {
		return false, false, false, httpReason(status, body)
	}
	var reviews []ghRestReview
	if err := json.Unmarshal(body, &reviews); err != nil {
		return false, false, false, fmt.Sprintf("HTTP 200 but response did not parse as a reviews list: %v", err)
	}
	for _, r := range reviews {
		if r.State == "APPROVED" {
			approved = true
			if headSHA != "" && r.CommitID == headSHA {
				atHead = true
			}
		}
	}
	return approved, atHead, true, ""
}

// get performs one authenticated GET, returning body + status. A transport error
// is returned as an error whose message begins with "HTTP" so the three-state
// reason is uniform (the offline arm and an auth failure read the same way to the
// caller and to the Verify assertions).
func (c *ghClient) get(url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request build failed: %v", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP transport error: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d but body read failed: %v", resp.StatusCode, err)
	}
	return body, resp.StatusCode, nil
}

// httpReason renders a non-200 status into a stable "HTTP <code>: <message>"
// reason. It lifts GitHub's JSON `message` field when present so the operator sees
// "Bad credentials" rather than a bare code.
func httpReason(status int, body []byte) string {
	msg := ""
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) == nil {
		msg = e.Message
	}
	if msg == "" {
		msg = http.StatusText(status)
	}
	return "HTTP " + strconv.Itoa(status) + ": " + msg
}

// singleBriefTrailer returns the brief ref of a body carrying EXACTLY ONE `Brief:`
// trailer (outside fenced code), reusing prlink.go's classifier for the count so
// the fetch half and the tree half agree on what a link is. ok is false for an
// unlinked or multi-linked body.
func singleBriefTrailer(body string) (ref string, ok bool) {
	if ClassifyPRLink([]PRLinkRecord{{Number: 0, Body: body}})[0] != PRLinkLinked {
		return "", false
	}
	inFence := false
	for _, raw := range strings.Split(body, "\n") {
		if rePRLinkFence.MatchString(raw) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if rePRLinkBrief.MatchString(raw) {
			_, after, _ := strings.Cut(raw, "Brief:")
			return strings.TrimSpace(after), true
		}
	}
	return "", false
}
