package deskkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// forge_github.go — the GitHub implementation of Forge. It is an EXTRACTION of the behavior
// the desk tools already run against GitHub (deskpost's reviewer client, deskrelease's ref
// client, deskclose's issue reads, the deskkit HTTPRepoInfoFetcher), pinned by the golden
// corpus in forge_github_golden_test.go so the extraction changed nothing observable at the
// wire: request method/path/query, pagination, and error mapping are captured per operation.
//
// It is handed an already-minted token (App installation token or PAT) — minting is the
// identity layer (spec §2/§5) and deliberately not part of this seam.

// GitHubForge implements Forge against the GitHub REST/GraphQL API with a bearer token.
// Same shape as HTTPRepoInfoFetcher (repovis.go): BaseURL defaults to GitHubAPIBase, Client
// to http.DefaultClient, so a test points it at an httptest server.
type GitHubForge struct {
	Token   string
	BaseURL string
	Client  *http.Client
}

var _ Forge = (*GitHubForge)(nil)

func (g *GitHubForge) baseURL() string {
	if g.BaseURL != "" {
		return g.BaseURL
	}
	return GitHubAPIBase
}

func (g *GitHubForge) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return http.DefaultClient
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

// doJSON performs one REST call, decoding a 2xx body into out (if non-nil). A non-2xx is a
// *ForgeAPIError; a transport/marshal/parse failure is Unverifiable. Extracted verbatim
// from deskpost's ghClient.doJSON (minus the 401 re-mint, which belongs to the token/identity
// layer the caller owns — a Forge holds an already-minted token).
func (g *GitHubForge) doJSON(method, path string, in, out any) error {
	var bodyReader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return Unverifiable("cannot marshal request body", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, g.baseURL()+path, bodyReader)
	if err != nil {
		return Unverifiable("cannot build request", err)
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := g.client().Do(req)
	if err != nil {
		return Unverifiable(fmt.Sprintf("%s %s failed", method, path), err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ForgeAPIError{Status: resp.StatusCode, Method: method, Path: path}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), err)
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
// issue, never a false pass. The reactions API requires the squirrel-girl preview accept
// header, so the standard doJSON header set cannot serve it — a raw request is used.
func (g *GitHubForge) IssueReactions(repo ForgeRepo, number int) ([]Reaction, error) {
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions?per_page=100", repo.Owner, repo.Name, number)
	req, err := http.NewRequest(http.MethodGet, g.baseURL()+path, nil)
	if err != nil {
		return nil, Unverifiable("cannot build reactions request", err)
	}
	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.squirrel-girl-preview+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, Unverifiable(fmt.Sprintf("GET %s failed", path), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ForgeAPIError{Status: resp.StatusCode, Method: http.MethodGet, Path: path}
	}
	body, _ := io.ReadAll(resp.Body)
	var reactions []Reaction
	if err := json.Unmarshal(body, &reactions); err != nil {
		return nil, Unverifiable("cannot parse reactions response", err)
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
// GitHub API for the transition; `gh pr ready` uses the same). Extracted from deskpost.
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
