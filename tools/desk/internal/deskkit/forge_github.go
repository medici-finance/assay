package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	ghapi "github.com/cli/go-gh/v2/pkg/api"
)

// forge_github.go — the GitHub implementation of Forge, seated on the official go-gh
// library (github.com/cli/go-gh/v2, its pkg/api REST client + pkg/auth resolution) rather
// than a hand-rolled net/http stack or a shelled `gh` binary. It is an EXTRACTION of the
// behavior the desk tools already run against GitHub, pinned by the golden corpus in
// forge_github_golden_test.go so re-seating the transport changed nothing observable at the
// wire: request method/path/query, pagination, and error mapping are captured per operation
// and stay byte-identical.
//
// Why go-gh, and what it buys:
//   - The GitHub backend now talks the forge through the same class of library-backed client
//     the GitLab backend uses, so both are API-behind-one-interface, and the `gh` CLI need no
//     longer be installed/version-matched on a runner for these operations.
//   - Auth binds to the EXPLICITLY minted desk token. The client is constructed with both a
//     Host and an AuthToken set AND an explicit Transport, which makes go-gh's
//     optionsNeedResolution false — so it never consults gh's ambient keyring/config for a
//     token or host. An empty token is REFUSED (restClient below), never silently resolved
//     to an ambient gh-CLI identity. This preserves the refuse-if-unminted posture the desk
//     tools depend on (mirrors #562/#563) at the transport floor.
//
// It is handed an already-minted token (App installation token or PAT) — minting is the
// identity layer (spec §2/§5) and deliberately not part of this seam.

// GitHubForge implements Forge against the GitHub REST/GraphQL API with a bearer token.
// Same shape as HTTPRepoInfoFetcher (repovis.go): BaseURL defaults to GitHubAPIBase, Client
// defaults to go-gh's own transport, so a test points BaseURL at an httptest server (and may
// supply a Client whose Transport reaches it).
type GitHubForge struct {
	Token   string
	BaseURL string
	Client  *http.Client

	// rc caches the go-gh REST client built from Token/BaseURL. Lazily constructed by
	// restClient so a bare struct literal (the golden test's construction shape) still works.
	rc *ghapi.RESTClient
}

var _ Forge = (*GitHubForge)(nil)

func (g *GitHubForge) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return GitHubAPIBase
}

// restClient returns the go-gh REST client for this forge, building it on first use.
//
// The token is bound EXPLICITLY: an empty Token is refused here rather than allowed to fall
// through to go-gh's ambient resolution (which would read gh's keyring/config). Setting Host,
// AuthToken and Transport all non-empty makes go-gh's optionsNeedResolution false, so no
// ambient lookup ever runs. The host is derived from BaseURL so go-gh's token roundtripper —
// which attaches the Authorization header only when the request host matches Host — sends the
// token to the real API host and to a test server, but never to an unrelated host.
func (g *GitHubForge) restClient() (*ghapi.RESTClient, error) {
	if g.Token == "" {
		return nil, Unverifiable("refusing to reach the GitHub forge without an explicitly minted token — "+
			"the go-gh backend never falls back to an ambient gh-CLI keyring/config identity", nil)
	}
	if g.rc != nil {
		return g.rc, nil
	}
	host := "github.com"
	if u, perr := url.Parse(g.baseURL()); perr == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	// Transport must be non-nil so go-gh does not treat the options as needing ambient
	// resolution. In production that is the default transport; a test may pass a Client whose
	// Transport reaches its httptest server.
	transport := http.DefaultTransport
	if g.Client != nil && g.Client.Transport != nil {
		transport = g.Client.Transport
	}
	rc, err := ghapi.NewRESTClient(ghapi.ClientOptions{
		Host:      host,
		AuthToken: g.Token,
		Transport: transport,
	})
	if err != nil {
		return nil, Unverifiable("cannot build go-gh REST client", err)
	}
	g.rc = rc
	return rc, nil
}

// ForgeAPIError is a non-2xx REST/GraphQL response. A caller maps it to Unverifiable (an
// API error mid-check means the precondition could not be positively verified). A 404 is
// distinguished via IsForgeNotFound — the only status that licenses a kind re-resolution.
type ForgeAPIError struct {
	Status int
	Method string
	Path   string
}

func (e *ForgeAPIError) Error() string {
	return fmt.Sprintf("forge API %s %s returned HTTP %d", e.Method, e.Path, e.Status)
}

// IsForgeNotFound reports whether err is a 404 from the forge REST layer. It unwraps, so a
// ForgeAPIError nested in a DeskError is still recognised.
func IsForgeNotFound(err error) bool {
	var ae *ForgeAPIError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// doJSON performs one REST call through the go-gh client, decoding a 2xx body into out (if
// non-nil and non-empty). A non-2xx is mapped to a *ForgeAPIError carrying the status, method
// and path — go-gh surfaces a non-2xx as its own *api.HTTPError, which is translated back to
// the forge's stable error shape so error CLASSIFICATION (401 vs 403 vs 404, and
// IsForgeNotFound) is unchanged from the pre-go-gh backend. A transport/marshal/parse failure
// is Unverifiable. The full URL is built here (baseURL()+path) and passed to go-gh's client,
// whose restURL passes an absolute URL through unchanged — so path, query and body are emitted
// exactly as constructed, which is what the golden corpus pins.
func (g *GitHubForge) doJSON(method, path string, in, out any) error {
	rc, err := g.restClient()
	if err != nil {
		return err
	}
	var bodyReader io.Reader
	if in != nil {
		b, merr := json.Marshal(in)
		if merr != nil {
			return Unverifiable("cannot marshal request body", merr)
		}
		bodyReader = bytes.NewReader(b)
	}
	resp, rerr := rc.Request(method, g.baseURL()+path, bodyReader)
	if rerr != nil {
		var he *ghapi.HTTPError
		if errors.As(rerr, &he) {
			return &ForgeAPIError{Status: he.StatusCode, Method: method, Path: path}
		}
		return Unverifiable(fmt.Sprintf("%s %s failed", method, path), rerr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, out); uerr != nil {
			return Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), uerr)
		}
	}
	return nil
}

// --- REST wire shapes (only the fields consumed) ---

type ghPullWire struct {
	Number       int    `json:"number"`
	State        string `json:"state"`
	Draft        bool   `json:"draft"`
	NodeID       string `json:"node_id"`
	ChangedFiles int    `json:"changed_files"`
	User         struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
	HTMLURL string `json:"html_url"`
}

type ghIssueWire struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	User   struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request"`
	HTMLURL string `json:"html_url"`
}

type ghReviewWire struct {
	ID   int64 `json:"id"`
	User struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"user"`
	State       string `json:"state"`
	CommitID    string `json:"commit_id"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

type ghFileWire struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
}

type ghCombinedStatusWire struct {
	State      string `json:"state"`
	TotalCount int    `json:"total_count"`
	Statuses   []struct {
		State   string `json:"state"`
		Context string `json:"context"`
	} `json:"statuses"`
}

type ghCheckRunsWire struct {
	TotalCount int `json:"total_count"`
	CheckRuns  []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"check_runs"`
}

// Pagination constants — extracted from deskpost's github.go, byte-for-byte. GitHub's
// default page size is 30, so an omitted per_page silently truncates a longer rollup; every
// walk here sends per_page=100 and reconciles against the total the head asserts.
const (
	forgeReviewPerPage = 100
	forgeFilePerPage   = 100
	forgeMaxFilePages  = 40 // 4000 entries; GitHub's own /files cap is 3000
	forgeCIPerPage     = 100
	forgeMaxCIPages    = 25 // 2500 items; exceeding it leaves len < TotalCount → fail closed
)

// --- Reads ---

func (g *GitHubForge) GetPullRequest(repo ForgeRepo, number int) (*PullRequest, error) {
	var w ghPullWire
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return &PullRequest{
		Number:       w.Number,
		State:        w.State,
		Draft:        w.Draft,
		NodeID:       w.NodeID,
		ChangedFiles: w.ChangedFiles,
		Author:       Account{Login: w.User.Login, ID: w.User.ID},
		HeadSHA:      w.Head.SHA,
	}, nil
}

func (g *GitHubForge) GetIssue(repo ForgeRepo, number int) (*Issue, error) {
	var w ghIssueWire
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &w); err != nil {
		return nil, err
	}
	return &Issue{
		Number:        w.Number,
		State:         w.State,
		Author:        Account{Login: w.User.Login, ID: w.User.ID},
		IsPullRequest: w.PullRequest != nil,
	}, nil
}

func (g *GitHubForge) ReviewsAtHead(repo ForgeRepo, number int) ([]Review, error) {
	var all []Review
	for page := 1; ; page++ {
		var chunk []ghReviewWire
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeReviewPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, r := range chunk {
			all = append(all, Review{
				ID:          r.ID,
				Author:      Account{Login: r.User.Login, ID: r.User.ID},
				State:       r.State,
				CommitID:    r.CommitID,
				Body:        r.Body,
				SubmittedAt: r.SubmittedAt,
			})
		}
		if len(chunk) < forgeReviewPerPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubForge) ListChangedFiles(repo ForgeRepo, number int) ([]ChangedFile, error) {
	var all []ChangedFile
	for page := 1; page <= forgeMaxFilePages; page++ {
		var chunk []ghFileWire
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=%d&page=%d",
			repo.Owner, repo.Name, number, forgeFilePerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &chunk); err != nil {
			return nil, err
		}
		for _, f := range chunk {
			all = append(all, ChangedFile{
				Filename:         f.Filename,
				PreviousFilename: f.PreviousFilename,
				Status:           f.Status,
			})
		}
		if len(chunk) < forgeFilePerPage {
			break
		}
	}
	return all, nil
}

func (g *GitHubForge) ChecksAtHead(repo ForgeRepo, sha string) (*ChecksAtHead, error) {
	out := &ChecksAtHead{}
	// Legacy combined-status rollup.
	for page := 1; page <= forgeMaxCIPages; page++ {
		var cs ghCombinedStatusWire
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/status?per_page=%d&page=%d",
			repo.Owner, repo.Name, sha, forgeCIPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &cs); err != nil {
			return nil, err
		}
		if page == 1 {
			out.CombinedState = cs.State
			out.StatusTotalCount = cs.TotalCount
		}
		for _, s := range cs.Statuses {
			out.Statuses = append(out.Statuses, StatusContext{State: s.State, Context: s.Context})
		}
		if len(cs.Statuses) < forgeCIPerPage {
			break
		}
	}
	// Check-runs rollup.
	for page := 1; page <= forgeMaxCIPages; page++ {
		var cr ghCheckRunsWire
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=%d&page=%d",
			repo.Owner, repo.Name, sha, forgeCIPerPage, page)
		if err := g.doJSON(http.MethodGet, path, nil, &cr); err != nil {
			return nil, err
		}
		if page == 1 {
			out.CheckRunsTotalCount = cr.TotalCount
		}
		for _, c := range cr.CheckRuns {
			out.CheckRuns = append(out.CheckRuns, CheckRun{Name: c.Name, Status: c.Status, Conclusion: c.Conclusion})
		}
		if len(cr.CheckRuns) < forgeCIPerPage {
			break
		}
	}
	return out, nil
}

// IssueReactions is SINGLE PAGE by decision — the same reasoning HTTPRepoInfoFetcher's
// IssueReactions carries in full (repovis.go): fails closed past 100 reactions on one
// issue, never a false pass. Routed through the go-gh client like every other read; the
// squirrel-girl preview accept header the pre-go-gh raw request set is no longer required
// (the reactions API has been GA for years and returns awards under the default accept).
func (g *GitHubForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	var reactions []Reaction
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions?per_page=100", repo.Owner, repo.Name, number)
	if err := g.doJSON(http.MethodGet, path, nil, &reactions); err != nil {
		return nil, err
	}
	return reactions, nil
}

func (g *GitHubForge) RepoVisibility(repo ForgeRepo) (string, error) {
	var info struct {
		Visibility string `json:"visibility"`
	}
	path := fmt.Sprintf("/repos/%s/%s", repo.Owner, repo.Name)
	if err := g.doJSON(http.MethodGet, path, nil, &info); err != nil {
		return "", err
	}
	if info.Visibility == "" {
		return "", Unverifiable(fmt.Sprintf("repo %s has no .visibility field in API response", repo.Slug()), nil)
	}
	return info.Visibility, nil
}

// --- Writes ---

// CreateDraftChange opens a draft pull request (the REST equivalent of `gh pr create
// --draft`, which deskpr runs today — see the inventory delta). draft:true is the frozen
// property: this seam opens changes as drafts, never ready.
func (g *GitHubForge) CreateDraftChange(repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", repo.Owner, repo.Name)
	body := map[string]any{
		"title": in.Title,
		"body":  in.Body,
		"head":  in.Head,
		"base":  in.Base,
		"draft": true,
	}
	var w ghPullWire
	if err := g.doJSON(http.MethodPost, path, body, &w); err != nil {
		return nil, err
	}
	return &PullRef{Number: w.Number, NodeID: w.NodeID, URL: w.HTMLURL}, nil
}

func (g *GitHubForge) PostComment(repo ForgeRepo, number int, body string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", repo.Owner, repo.Name, number)
	return g.doJSON(http.MethodPost, path, map[string]any{"body": body}, nil)
}

func (g *GitHubForge) PostReview(repo ForgeRepo, number int, in ReviewInput) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", repo.Owner, repo.Name, number)
	body := map[string]any{"commit_id": in.HeadSHA, "event": in.Event, "body": in.Body}
	return g.doJSON(http.MethodPost, path, body, nil)
}

// MarkReadyForReview flips a draft change to ready via the GraphQL mutation (the only
// GitHub API for the transition; `gh pr ready` uses the same). It is issued over the SAME
// authenticated go-gh client as the REST operations, POSTing the mutation to an absolute
// /graphql URL. go-gh's dedicated GraphQLClient targets a host-derived https endpoint it
// gives no absolute-URL override for, so it cannot be pointed at the golden corpus's httptest
// server without changing the observed request; routing the one mutation through the REST
// client keeps the wire byte-identical to what the corpus pins while still binding auth to
// the minted token.
func (g *GitHubForge) MarkReadyForReview(nodeID string) error {
	q := `mutation($id:ID!){markPullRequestReadyForReview(input:{pullRequestId:$id}){pullRequest{isDraft}}}`
	in := map[string]any{"query": q, "variables": map[string]any{"id": nodeID}}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return Unverifiable("markPullRequestReadyForReview GraphQL error: "+out.Errors[0].Message, nil)
	}
	return nil
}

// FileIssue files a new issue (the REST equivalent of `gh issue create`, which deskfile
// runs today — see the inventory delta).
func (g *GitHubForge) FileIssue(repo ForgeRepo, in IssueInput) (*IssueRef, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues", repo.Owner, repo.Name)
	var w ghIssueWire
	if err := g.doJSON(http.MethodPost, path, map[string]any{"title": in.Title, "body": in.Body}, &w); err != nil {
		return nil, err
	}
	return &IssueRef{Number: w.Number, URL: w.HTMLURL}, nil
}

// CloseIssue closes an issue (the REST equivalent of `gh issue close`, which deskclose/
// deskfile run today — see the inventory delta). A stateReason of "" omits the field.
func (g *GitHubForge) CloseIssue(repo ForgeRepo, number int, stateReason string) error {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number)
	body := map[string]any{"state": "closed"}
	if stateReason != "" {
		body["state_reason"] = stateReason
	}
	return g.doJSON(http.MethodPatch, path, body, nil)
}

// DeleteRef deletes one git ref (the typed replacement for the `gh api -X DELETE
// repos/<o>/<r>/git/refs/<ref>` passthrough fanoutloop used to release a dispatch claim).
// The ref is validated by ValidateRefPath first, so the only thing this op can address is a
// ref inside the named repo — the arbitrary-endpoint reach of the call it replaces is gone,
// not renamed.
//
// A missing ref surfaces as a *ForgeAPIError the caller can test with IsForgeNotFound; the
// seam does not decide that "already gone" is success, because for a claim release it is and
// for a tag retraction it is not.
func (g *GitHubForge) DeleteRef(repo ForgeRepo, ref string) error {
	clean, err := ValidateRefPath(ref)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/refs/%s", repo.Owner, repo.Name, clean)
	return g.doJSON(http.MethodDelete, path, nil, nil)
}

// --- Identity / transport ---

// PushTransportHint returns GitHub's push-transport shape: an App installation token
// authenticates an https push as the username "x-access-token", supplied via an inline
// credential.helper reading the token file — never a token-in-URL (classifier-blocked, and
// it leaks the secret into argv/reflog). Pure function, no network call.
func (g *GitHubForge) PushTransportHint(repo ForgeRepo) PushTransport {
	return PushTransport{
		RemoteHost:    "github.com",
		TokenUsername: "x-access-token",
		CredentialHelperHint: "supply the token via an inline credential.helper that reads the 0600 token " +
			"file; never embed it in the remote URL",
	}
}
