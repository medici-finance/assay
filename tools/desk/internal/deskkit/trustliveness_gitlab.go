package deskkit

// trustliveness_gitlab.go — the GitLab account fetcher for `deskroster liveness`
// (assay#1667). GitLab account-liveness was, until this file, an untracked gap: the
// command printed a could-not-check notice pointing at a closed issue (#933, superseded by
// #943) and checked zero identities on a GitLab-backed repo. See trustliveness.go for the
// forge-agnostic classifier (classifyLiveness/CheckRosterLiveness/RenderLivenessNotices)
// this fetcher feeds; only the GitLab-specific lookup lives here.
//
// GitLab exposes no "one account by exact username" endpoint the way GitHub's
// GET /users/{login} does. The equivalent is GET /api/v4/users?username=<name> — an
// EXACT-MATCH list (GitLab's own account-lookup convention: zero or one entry, never a
// fuzzy match) — so "not found" here is an EMPTY 200 response, never a 404, which is the
// one structural difference from HTTPAccountFetcher's GitHub contract that this file's
// GetAccount has to encode.
//
// PLACEMENT, mirroring trustliveness.go's own recorded deviation: this is a plain
// standalone struct satisfying AccountFetcher, not a method on *GitLabForge — the
// closed-surface test (TestForgeNoPassthrough) asserts *GitLabForge's exported method set
// equals the Forge interface's exactly, so a GetAccount method there would trip it exactly
// as it would on *GitHubForge. HTTPRepoInfoFetcher (repovis.go) and HTTPAccountFetcher
// (trustliveness.go) are the same shape for the same reason; this is the GitLab sibling.
//
// This deliberately talks raw net/http, like HTTPAccountFetcher, rather than the official
// GitLab client library forge_gitlab.go's Forge implementation is seated on
// (gitlab.com/gitlab-org/api/client-go). The library's ListUsers call would reach the exact
// same endpoint, but this fetcher is not a Forge operation — it is a small, standalone,
// out-of-band read the same way HTTPAccountFetcher is on the GitHub side, and keeping it on
// the same raw-HTTP shape keeps the two fetchers reviewable side by side.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPGitLabAccountFetcher implements AccountFetcher against a GitLab instance's REST v4
// users endpoint. Same construction shape as HTTPAccountFetcher: BaseURL defaults to
// GitLabAPIBase (the INSTANCE root — this fetcher appends api/v4/ itself, the same
// convention forge_gitlab.go's GitLabForge.baseURL uses), Client defaults to
// http.DefaultClient, so a test points BaseURL at an httptest server — including a
// self-hosted base URL, which matters here: GitLab account-liveness has to work against any
// instance root, not just gitlab.com.
type HTTPGitLabAccountFetcher struct {
	Token   string
	BaseURL string // defaults to GitLabAPIBase (the INSTANCE root) when empty
	Client  *http.Client
}

func (f *HTTPGitLabAccountFetcher) baseURL() string {
	if strings.TrimSpace(f.BaseURL) != "" {
		return strings.TrimRight(f.BaseURL, "/")
	}
	return GitLabAPIBase
}

func (f *HTTPGitLabAccountFetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// gitlabUserWire is the subset of GitLab's users-list payload this fetcher reads.
type gitlabUserWire struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	State    string `json:"state"`
}

// GetAccount calls GET /api/v4/users?username=<login> — GitLab's exact-match username
// lookup — and returns the CURRENT id/username/state GitLab reports. An EMPTY result list
// (200 OK, `[]`) is GitLab's "no such account" signal on this endpoint (it never 404s on a
// no-match query) and returns (nil, ErrAccountNotFound); any transport failure, non-2xx
// status, or malformed body is returned as its own distinct error — the caller
// (classifyLiveness) is the one that classifies "deleted" apart from "could not tell", not
// this method, exactly mirroring HTTPAccountFetcher.GetAccount's GitHub contract.
func (f *HTTPGitLabAccountFetcher) GetAccount(login string) (*Account, error) {
	reqURL := fmt.Sprintf("%s/api/v4/users?username=%s", f.baseURL(), url.QueryEscape(login))
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", f.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", reqURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var wire []gitlabUserWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("cannot parse GitLab users response for %q: %w", login, err)
	}
	if len(wire) == 0 {
		return nil, ErrAccountNotFound
	}

	// The endpoint is documented exact-match, but this picks the case-insensitively
	// matching entry defensively rather than trusting position 0 blindly — falling back to
	// the first entry only when nothing matches the queried login at all, which still
	// leaves a non-empty response resolvable rather than silently dropped.
	entry := wire[0]
	for _, w := range wire {
		if strings.EqualFold(w.Username, login) {
			entry = w
			break
		}
	}
	if entry.Username == "" {
		return nil, fmt.Errorf("GitLab users response for %q carries no .username field", login)
	}
	return &Account{Login: entry.Username, ID: entry.ID, State: entry.State}, nil
}

var _ AccountFetcher = (*HTTPGitLabAccountFetcher)(nil)
