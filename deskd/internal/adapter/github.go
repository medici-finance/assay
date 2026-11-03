package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Repo is an owner/name pair as it appears in cells.yaml.
type Repo struct {
	Owner string
	Name  string
}

// ParseRepo splits "owner/name".
func ParseRepo(slug string) (Repo, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" {
		return Repo{}, fmt.Errorf("repo %q is not owner/name", slug)
	}
	return Repo{Owner: owner, Name: name}, nil
}

func (r Repo) String() string { return r.Owner + "/" + r.Name }

// escalationLabels is the label set that makes an issue an escalation
// (the escalation wire contract, design §1.1). `help wanted` carries a space on
// the wire and a hyphen in the domain; this is the only place that drift is
// spelled out.
var escalationLabels = []string{"needs-decision", "question", "help wanted", "ask"}

// pageBudget caps the conditional-REST walk. Its ONLY effect when exhausted
// is to mark the probe Capped, which makes every count derived from it
// refuse to publish (domain.NewCount). It never silently truncates a set:
// a repo whose walk was cut short reports could-not-check for its whole
// slice, because a partial issue list and a complete one are the same shape.
const pageBudget = 40

// perPage is the REST page size. 100 is GitHub's maximum.
const perPage = 100

// RepoHealth is the per-repo three-state (design §10: one unreachable repo
// degrades that repo's slice, not the whole inbox).
type RepoHealth struct {
	Repo       string            `json:"repo"`
	Assessment domain.Assessment `json:"assessment"`
	// NotModified records that the conditional request answered 304 — the
	// repo is healthy AND cost nothing. Distinct from "no escalations".
	NotModified bool `json:"not_modified"`
	// CheckedAt is when this verdict was formed.
	CheckedAt time.Time `json:"checked_at"`
}

// Client is a read-only GitHub reader.
//
// It has no mutating method, and — the load-bearing half — it cannot acquire
// one by accident, because its transport refuses any request that could
// write ([ReadOnlyTransport]). See that type for why the enforcement lives
// there rather than in a review checklist.
type Client struct {
	http    *http.Client
	apiBase string // e.g. https://api.github.com
	token   string

	mu    sync.Mutex
	etags map[string]string // url -> last ETag, for conditional requests
}

// NewGitHub builds the ONLY client this package offers.
//
// The read-only transport is applied unconditionally — there is no option to
// pass a bare transport, and no exported field to swap one in afterwards.
// A caller that wants a write path must edit this constructor, which is a
// reviewable act.
//
// token is a READ-SCOPED credential supplied by the caller (deskd reads it
// from the environment). Nothing in this package logs it, embeds it, or
// writes it to the index.
func NewGitHub(apiBase, token string, base http.RoundTripper) *Client {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &Client{
		http:    &http.Client{Transport: NewReadOnlyTransport(base), Timeout: 30 * time.Second},
		apiBase: strings.TrimSuffix(apiBase, "/"),
		token:   token,
		etags:   map[string]string{},
	}
}

// candidateProbe declares the change-detection/candidate-set probe.
func candidateProbe(repo Repo, capped bool) domain.Probe {
	return domain.Probe{
		Source: fmt.Sprintf("GET %s/repos/%s/issues?state=open&labels=… (conditional, If-None-Match), Link rel=next followed", "{api}", repo),
		Method: domain.MethodRESTList,
		Window: "open issues carrying any of " + strings.Join(escalationLabels, ", ") + "; all ages",
		Limit:  pageBudget * perPage,
		Capped: capped,
	}
}

// derivationProbe declares the by-number derivation probe. Aliased lookups
// have NO paging cap, which is why the field content comes from here and not
// from the list response.
func derivationProbe(repo Repo, n int) domain.Probe {
	return domain.Probe{
		Source: fmt.Sprintf("POST {api}/graphql: repository(owner:%q,name:%q){ i0: issue(number:…) … } — %d aliased by-number lookups", repo.Owner, repo.Name, n),
		Method: domain.MethodGraphQLAliased,
		Window: fmt.Sprintf("the %d candidate issues named by this cycle's list probe", n),
		Limit:  0, // aliased lookups do not page and cannot truncate
	}
}

// RawIssue is the platform payload, still platform-shaped. It exists ONLY
// inside this package; nothing here crosses into internal/domain.
type RawIssue struct {
	Number    int
	Title     string
	Body      string
	Labels    []string
	Author    string
	AuthorBot bool
	CreatedAt time.Time
	UpdatedAt time.Time
	Closed    bool
	ClosedAt  *time.Time
	URL       string // the deep link, attached at the serving edge as links.platform
	// Comments is the tail of the issue's comment thread (the service contract
	// increment 3, task 6): the last [commentWindow] comments, oldest first,
	// as GraphQL returns them. It is what [DeriveAnswer] scans for the
	// the answer-record grammar answer-record marker ON POLL. Empty on every issue this
	// build's candidate list never names as closed (the marker is only
	// meaningful once an issue has settled), which is the common case for the
	// open-issue set.
	Comments []RawComment
}

// RawComment is one issue comment, platform-shaped. Only the fields
// [DeriveAnswer] needs are carried — this is not a general comment reader.
type RawComment struct {
	Body      string
	Login     string
	AuthorBot bool
	CreatedAt time.Time
}

// FetchResult is one repo's slice of a poll cycle.
type FetchResult struct {
	Repo   Repo
	Health RepoHealth
	Issues []RawIssue
	// Candidates is how many issue keys the list probe named. Published as
	// a domain.Count so it can never be read as a settled total when the
	// walk was capped.
	Candidates domain.Count
	// Derived is how many of those the aliased lookup actually returned.
	// A gap between Candidates and Derived is an UNJOINABLE set and is
	// reported, not smoothed over.
	Derived domain.Count
}

// FetchEscalations runs one repo's poll cycle.
//
// The two-probe shape is deliberate:
//
//  1. A CONDITIONAL REST list names the candidate keys. It is the cheap
//     change detector — a 304 costs no rate limit at all, which is what
//     makes a 60s interval comfortable (design §5.1: ~5 calls × 10 repos ×
//     60 cycles ≈ 3,000 req/hr against a 5,000/hr floor, and 304s are free).
//  2. An ALIASED GraphQL document derives the content by number. Aliased
//     lookups have no paging cap, so nothing published from step 2 can be a
//     truncated list silently reported as a total.
//
// Every failure path in here ends at could-not-check with a reason. NONE of
// them ends at an empty slice with a clean verdict — an empty result from an
// unreachable source is BLIND, not idle, and the shared-token secondary rate
// limit (a 403 under concurrency) produces exactly that empty-looking shape.
func (c *Client) FetchEscalations(ctx context.Context, repo Repo) FetchResult {
	now := time.Now().UTC()
	res := FetchResult{Repo: repo}

	numbers, notModified, capped, err := c.listCandidates(ctx, repo)
	if err != nil {
		p := candidateProbe(repo, capped)
		reason := "list probe did not complete: " + err.Error() +
			" — this repo's slice is UNKNOWN, not empty (an empty result from an unreachable source is blind, not idle)"
		res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(p, reason)}
		res.Candidates = domain.NoCount(p, reason)
		res.Derived = domain.NoCount(p, reason)
		return res
	}

	if notModified {
		p := candidateProbe(repo, false)
		res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, NotModified: true, Assessment: domain.Clean(p)}
		res.Candidates = domain.NewCount(len(numbers), p)
		res.Derived = domain.NewCount(0, derivationProbe(repo, 0))
		return res
	}

	candProbe := candidateProbe(repo, capped)
	res.Candidates = domain.NewCount(len(numbers), candProbe)

	if capped {
		reason := fmt.Sprintf("the conditional list walk hit its %d-page budget without reaching the last page; "+
			"the candidate set is a PREFIX of the real one, so this repo's slice is could-not-check rather than partial",
			pageBudget)
		res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(candProbe, reason)}
		res.Derived = domain.NoCount(candProbe, reason)
		return res
	}

	issues, err := c.deriveByNumber(ctx, repo, numbers)
	derProbe := derivationProbe(repo, len(numbers))
	if err != nil {
		reason := "aliased by-number derivation failed: " + err.Error()
		res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(derProbe, reason)}
		res.Derived = domain.NoCount(derProbe, reason)
		return res
	}

	res.Issues = issues
	res.Derived = domain.NewCount(len(issues), derProbe)

	if len(issues) != len(numbers) {
		// The list named keys the derivation did not return. This is the
		// unjoinable case at the fetch layer, and it degrades the REPO's
		// verdict — the rows that did come back are still served, because
		// dropping the whole slice over a partial join would hide more than
		// it protects.
		reason := fmt.Sprintf("the list probe named %d issues but the by-number derivation returned %d; "+
			"%d key(s) are UNRESOLVED — served rows are honest, the repo total is not",
			len(numbers), len(issues), len(numbers)-len(issues))
		res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(derProbe, reason)}
		return res
	}

	res.Health = RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Clean(derProbe)}
	return res
}

// FetchOne re-derives ONE issue by number, bypassing the conditional list
// probe entirely (the service contract increment 3, task 2: the refresh nudge,
// POST /v1/escalations/{id}/refresh). It is the same aliased-by-number
// derivation [FetchEscalations] uses for its second probe — no paging cap,
// one document — so a refresh costs exactly one GraphQL call and reuses the
// identical decode path (comments included), never a parallel one-off shape
// that could drift from the regular poll's derivation.
//
// ok=false with a nil error means GitHub returned no such issue (deleted,
// transferred, or the number never existed) — a 404 at the caller, not a
// could-not-check: the aliased lookup has no paging cap to fall short of.
func (c *Client) FetchOne(ctx context.Context, repo Repo, number int) (RawIssue, bool, error) {
	issues, err := c.deriveByNumber(ctx, repo, []int{number})
	if err != nil {
		return RawIssue{}, false, err
	}
	if len(issues) == 0 {
		return RawIssue{}, false, nil
	}
	return issues[0], true, nil
}

// listCandidates walks the conditional REST list once PER LABEL and unions
// the results.
//
// One request per label, not one request naming them all, because the REST
// `labels=` parameter is a CONJUNCTION: `labels=needs-decision,question`
// selects issues carrying BOTH, not either. That distinction cost a full
// live cycle during this brief's build — every repo came back checked-clean
// with zero rows, which is precisely the confident-empty answer this index
// exists to make impossible. The clean verdict was honest about the
// transport and wrong about the question, so the fix is in the query, and
// this comment is the tripwire against re-collapsing the calls.
//
// notModified is reported only when EVERY label's first page answered 304 —
// one changed label means the union changed.
func (c *Client) listCandidates(ctx context.Context, repo Repo) (numbers []int, notModified bool, capped bool, err error) {
	seen := map[int]bool{}
	allNotModified := true

	for _, label := range escalationLabels {
		nums, nm, cap, err := c.listByLabel(ctx, repo, label)
		if err != nil {
			return nil, false, cap, fmt.Errorf("label %q: %w", label, err)
		}
		if !nm {
			allNotModified = false
		}
		capped = capped || cap
		for _, n := range nums {
			seen[n] = true
		}
	}
	if allNotModified {
		return nil, true, false, nil
	}
	for n := range seen {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	return numbers, false, capped, nil
}

// listByLabel walks one label's conditional list, following Link rel="next"
// to exhaustion or to the page budget.
func (c *Client) listByLabel(ctx context.Context, repo Repo, label string) (numbers []int, notModified bool, capped bool, err error) {
	q := url.Values{}
	q.Set("state", "open")
	q.Set("labels", label)
	q.Set("per_page", fmt.Sprint(perPage))
	next := fmt.Sprintf("%s/repos/%s/%s/issues?%s", c.apiBase, repo.Owner, repo.Name, q.Encode())

	for page := 0; next != ""; page++ {
		if page >= pageBudget {
			return numbers, false, true, nil
		}
		body, status, link, err := c.conditionalGet(ctx, next)
		if err != nil {
			return nil, false, false, err
		}
		if status == http.StatusNotModified {
			// Only meaningful on the first page: an unchanged first page
			// means the label-filtered set is unchanged.
			if page == 0 {
				return nil, true, false, nil
			}
			break
		}
		var payload []struct {
			Number int `json:"number"`
			// A pull request also appears on the issues endpoint; the
			// presence of this key is how they are told apart.
			PullRequest *struct{} `json:"pull_request"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, false, false, fmt.Errorf("list page %d is not the expected JSON: %w", page, err)
		}
		for _, it := range payload {
			if it.PullRequest == nil {
				numbers = append(numbers, it.Number)
			}
		}
		next = nextLink(link)
	}
	return numbers, false, false, nil
}

// conditionalGet issues a GET with If-None-Match and records the ETag.
func (c *Client) conditionalGet(ctx context.Context, rawURL string) (body []byte, status int, link string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	c.mu.Lock()
	if tag := c.etags[rawURL]; tag != "" {
		req.Header.Set("If-None-Match", tag)
	}
	c.mu.Unlock()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()

	if tag := resp.Header.Get("ETag"); tag != "" {
		c.mu.Lock()
		c.etags[rawURL] = tag
		c.mu.Unlock()
	}
	if resp.StatusCode == http.StatusNotModified {
		return nil, resp.StatusCode, "", nil
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, "", classifyHTTPError(resp.StatusCode, resp.Header, raw)
	}
	return raw, resp.StatusCode, resp.Header.Get("Link"), nil
}

// classifyHTTPError turns a non-200 into a reason a human can act on. The
// 403-with-no-remaining and 403-secondary cases are called out by name,
// because both look like "no data" to a caller that only checks len().
func classifyHTTPError(status int, h http.Header, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 200 {
		snippet = snippet[:200] + "…"
	}
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests:
		if h.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("HTTP %d: PRIMARY rate limit exhausted (resets %s)", status, h.Get("X-RateLimit-Reset"))
		}
		return fmt.Errorf("HTTP %d: SECONDARY rate limit or abuse detection — concurrent readers on one token trip this; "+
			"the correct reading is BLIND, never idle: %s", status, snippet)
	case http.StatusUnauthorized:
		return fmt.Errorf("HTTP 401: the read token was rejected (expired or wrong installation)")
	case http.StatusNotFound:
		return fmt.Errorf("HTTP 404: repo not visible to this token — indistinguishable from 'does not exist', so it is could-not-check, not empty")
	default:
		return fmt.Errorf("HTTP %d: %s", status, snippet)
	}
}

// nextLink extracts rel="next" from a Link header.
func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		seg := strings.Split(strings.TrimSpace(part), ";")
		if len(seg) < 2 {
			continue
		}
		for _, attr := range seg[1:] {
			if strings.Contains(strings.ReplaceAll(attr, " ", ""), `rel="next"`) {
				return strings.Trim(strings.TrimSpace(seg[0]), "<>")
			}
		}
	}
	return ""
}

// aliasChunk is how many by-number lookups ride in one GraphQL document.
// GitHub costs a document by the objects it names, so this trades request
// count against document size; 50 keeps both comfortable.
const aliasChunk = 50

// commentWindow bounds the comment tail fetched per issue (the service contract
// increment 3, task 6). The answer-record marker is written by the LAST
// commit of the answer (the answer flow's decisionBody, or a future re-home of it),
// so a small, fixed tail is enough to find it without an unbounded per-issue
// page walk — the same "bounded, not exhaustive" shape the label list already
// uses (pageBudget).
const commentWindow = 20

// deriveByNumber fetches issue content with ALIASED by-number lookups.
//
// This is the shape that has no paging cap: each issue is named explicitly,
// so there is no `first:` to truncate and no cursor to abandon. A count from
// here is a count. A count from a list call is a list call's limit.
func (c *Client) deriveByNumber(ctx context.Context, repo Repo, numbers []int) ([]RawIssue, error) {
	var out []RawIssue
	for start := 0; start < len(numbers); start += aliasChunk {
		end := min(start+aliasChunk, len(numbers))
		chunk := numbers[start:end]

		var b strings.Builder
		fmt.Fprintf(&b, "query { repository(owner: %q, name: %q) {\n", repo.Owner, repo.Name)
		for i, n := range chunk {
			fmt.Fprintf(&b, "  a%d: issue(number: %d) { number title body createdAt updatedAt closedAt state url "+
				"author { login __typename } labels(first: 50) { nodes { name } } "+
				"comments(last: %d) { nodes { body createdAt author { login __typename } } } }\n", i, n, commentWindow)
		}
		b.WriteString("} }")

		raw, err := c.graphQL(ctx, b.String())
		if err != nil {
			return nil, err
		}
		issues, err := decodeAliasedIssues(raw, len(chunk))
		if err != nil {
			return nil, err
		}
		out = append(out, issues...)
	}
	return out, nil
}

// graphQL POSTs a read document. The transport independently verifies it is
// a read — this function could not send a mutation even if it tried.
func (c *Client) graphQL(ctx context.Context, query string) ([]byte, error) {
	payload, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/graphql", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, classifyHTTPError(resp.StatusCode, resp.Header, raw)
	}
	return raw, nil
}

type gqlIssue struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	State     string     `json:"state"`
	URL       string     `json:"url"`
	Author    *struct {
		Login    string `json:"login"`
		TypeName string `json:"__typename"`
	} `json:"author"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	Comments struct {
		Nodes []struct {
			Body      string    `json:"body"`
			CreatedAt time.Time `json:"createdAt"`
			Author    *struct {
				Login    string `json:"login"`
				TypeName string `json:"__typename"`
			} `json:"author"`
		} `json:"nodes"`
	} `json:"comments"`
}

// decodeAliasedIssues reads the aliased response.
//
// A null alias — GitHub returning `"a3": null` for an issue that was deleted
// or transferred between the two probes — is SKIPPED here and shows up as a
// Candidates/Derived gap in [FetchResult], which degrades the repo verdict.
// It is not silently backfilled and not counted as present.
func decodeAliasedIssues(raw []byte, n int) ([]RawIssue, error) {
	var env struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("GraphQL response is not the expected JSON: %w", err)
	}
	if len(env.Errors) > 0 {
		// Partial GraphQL responses are the classic silent-truncation
		// shape: data alongside errors. Refuse the whole chunk.
		var msgs []string
		for _, e := range env.Errors {
			msgs = append(msgs, e.Type+": "+e.Message)
		}
		return nil, fmt.Errorf("GraphQL returned %d error(s), so the chunk is not trustworthy: %s",
			len(env.Errors), strings.Join(msgs, "; "))
	}
	if env.Data.Repository == nil {
		return nil, fmt.Errorf("GraphQL returned no repository object")
	}

	var out []RawIssue
	for i := range n {
		msg, ok := env.Data.Repository[fmt.Sprintf("a%d", i)]
		if !ok || string(msg) == "null" {
			continue
		}
		var gi gqlIssue
		if err := json.Unmarshal(msg, &gi); err != nil {
			return nil, fmt.Errorf("alias a%d did not decode: %w", i, err)
		}
		ri := RawIssue{
			Number:    gi.Number,
			Title:     gi.Title,
			Body:      gi.Body,
			CreatedAt: gi.CreatedAt,
			UpdatedAt: gi.UpdatedAt,
			ClosedAt:  gi.ClosedAt,
			Closed:    strings.EqualFold(gi.State, "CLOSED"),
			URL:       gi.URL,
		}
		if gi.Author != nil {
			ri.Author = gi.Author.Login
			ri.AuthorBot = gi.Author.TypeName == "Bot"
		}
		for _, l := range gi.Labels.Nodes {
			ri.Labels = append(ri.Labels, l.Name)
		}
		for _, c := range gi.Comments.Nodes {
			rc := RawComment{Body: c.Body, CreatedAt: c.CreatedAt}
			if c.Author != nil {
				rc.Login = c.Author.Login
				rc.AuthorBot = c.Author.TypeName == "Bot"
			}
			ri.Comments = append(ri.Comments, rc)
		}
		out = append(out, ri)
	}
	return out, nil
}
