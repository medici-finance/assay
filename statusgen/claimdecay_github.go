package main

// claimdecay_github.go — the GitHub half of the dead-claim decay's state read,
// in process.
//
// The decay used to shell out to `gh api graphql --paginate --slurp`. The board
// regen job builds statusgen from source on a runner that has no `gh`, so every
// regen on main printed `COULD-NOT-CHECK — dead-claim decay did not run ...
// exec: "gh": executable file not found in $PATH`, and branches whose pull
// request had already closed kept holding their stream's dispatch cap. The
// DORA-timing recorder hit the same wall earlier and moved to net/http for the
// same reason (doratiming.go).
//
// This file makes the SAME GraphQL read (ghBranchPRQuery, unchanged) over
// net/http and the standard library, and returns the same shape the `gh` call
// returned — a JSON array of pages — so listMergedClosedBranches and every rule
// that decides a corpse are untouched. Like the GitLab arm
// (claimdecay_gitlab.go) it is statusgen-local rather than the desk's forge seam:
// statusgen ships as one binary that runs in an adopter's CI with no desk-tools
// installed, so a read that needs a desk-tools verb on PATH would fail there for
// the same reason `gh` did.
//
// Credential: GH_TOKEN, then GITHUB_TOKEN (what a workflow step passes), then —
// only when neither is set and `gh` happens to be on PATH — `gh auth token`, so a
// desk machine that relied on gh's own login keeps decaying. `gh` is never
// REQUIRED. GitHub's GraphQL API refuses unauthenticated calls, so no credential
// at all is a named could-not-check before any request, never an anonymous try.
//
// Three-state contract (docs/three-state-instrument-rule.md): every exit is a
// listing that was read in full, or an error naming why it could not be. A
// non-200, a body that is not JSON, a cursor that does not advance, or a walk
// that is still paging at the safety bound is an error — never a short listing,
// which the caller would read as "these are all the branches".

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// githubDecayMaxPages bounds the branch walk (100 branches a page). It is a
// safety bound, not a size limit: a walk still paging when it reaches it is a
// could-not-check, never a complete listing. A variable only so a test can reach
// the bound without serving a thousand pages.
var githubDecayMaxPages = 1000

// githubDecayClient carries the request timeout. Its Transport is nil, so every
// request resolves through http.DefaultTransport — the one transport the --lint
// reach guard (lintoffline_test.go) watches and tests replace.
var githubDecayClient = &http.Client{Timeout: 30 * time.Second}

// githubDecayToken returns the credential for the branch read and where it came
// from (for the could-not-check text). Empty when there is none.
func githubDecayToken() (token, source string) {
	for _, env := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, env
		}
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "", ""
	}
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v, "`gh auth token`"
		}
	}
	return "", ""
}

// githubGraphQLEndpoint returns the GraphQL endpoint for the repository behind
// root. GITHUB_GRAPHQL_URL (set by Actions, and by an operator on a GitHub
// Enterprise host) wins. Otherwise a remote whose host carries the label "github"
// and is not github.com itself is a GitHub Enterprise Server host, answered at
// https://<host>/api/graphql; anything else — github.com, an SSH host alias, an
// unreadable remote — is github.com's endpoint.
func githubGraphQLEndpoint(root string) string {
	if v := strings.TrimSpace(os.Getenv("GITHUB_GRAPHQL_URL")); v != "" {
		return v
	}
	if raw, err := remoteOriginURL(root); err == nil {
		host := remoteHost(raw)
		if classifyForgeHost(host) == forgeGitHub && host != "github.com" && !strings.HasSuffix(host, ".github.com") {
			return "https://" + host + "/api/graphql"
		}
	}
	return "https://api.github.com/graphql"
}

// githubDecayRepo returns the owner and name to query: the `origin` remote's
// path first (what root really is, and what sameRepoPR compares each pull
// request's head against), GITHUB_REPOSITORY when the remote names no
// owner/name pair.
func githubDecayRepo(root string) (owner, name string, err error) {
	for _, cand := range []string{trackedGitHubRepo(root), strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))} {
		o, n, ok := strings.Cut(cand, "/")
		if ok && o != "" && n != "" && !strings.Contains(n, "/") {
			return o, n, nil
		}
	}
	return "", "", fmt.Errorf("no owner/name for the repository: the `origin` remote names none and GITHUB_REPOSITORY is unset")
}

// githubBranchPageInfo is the part of one page the walk itself reads.
type githubBranchPageInfo struct {
	Data struct {
		Repository *struct {
			Refs struct {
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"refs"`
		} `json:"repository"`
	} `json:"data"`
	Errors []json.RawMessage `json:"errors"`
}

// githubBranchPRPages runs ghBranchPRQuery for the repository rooted at root and
// returns every page as one JSON array — the shape `gh api graphql --paginate
// --slurp` returned. A page carrying GraphQL errors, or no repository, ends the
// walk and is returned as it stands: listMergedClosedBranches refuses it with
// the page number, so it is never read as an empty page.
func githubBranchPRPages(root string) ([]byte, error) {
	token, source := githubDecayToken()
	if token == "" {
		return nil, fmt.Errorf("no GitHub API token: set GH_TOKEN or GITHUB_TOKEN (in a workflow, the job's token with pull-requests read), " +
			"or log in with `gh`; the GitHub GraphQL API does not answer unauthenticated reads")
	}
	owner, name, err := githubDecayRepo(root)
	if err != nil {
		return nil, err
	}
	endpoint := githubGraphQLEndpoint(root)
	var pages []json.RawMessage
	var cursor any // JSON null on the first page
	for page := 1; page <= githubDecayMaxPages; page++ {
		payload, err := json.Marshal(map[string]any{
			"query":     ghBranchPRQuery,
			"variables": map[string]any{"owner": owner, "name": name, "endCursor": cursor},
		})
		if err != nil {
			return nil, fmt.Errorf("building the branch query: %v", err)
		}
		body, err := githubGraphQLPost(endpoint, token, source, payload)
		if err != nil {
			return nil, fmt.Errorf("page %d: %v", page, err)
		}
		var info githubBranchPageInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, fmt.Errorf("page %d: HTTP 200 but the response did not parse as JSON: %v", page, err)
		}
		pages = append(pages, json.RawMessage(body))
		done := len(info.Errors) > 0 || info.Data.Repository == nil
		if !done {
			pi := info.Data.Repository.Refs.PageInfo
			if pi.HasNextPage {
				if pi.EndCursor == "" || pi.EndCursor == cursor {
					return nil, fmt.Errorf("page %d says more branches follow but gives no new cursor, so the listing cannot be completed", page)
				}
				cursor = pi.EndCursor
				continue
			}
		}
		out, err := json.Marshal(pages)
		if err != nil {
			return nil, fmt.Errorf("assembling the branch listing: %v", err)
		}
		return out, nil
	}
	return nil, fmt.Errorf("the branch listing was still paging after %d pages (%d branches), so it is not provably complete",
		githubDecayMaxPages, githubDecayMaxPages*100)
}

// githubGraphQLPost sends one GraphQL request and returns a 200 body. Anything
// else is an error carrying the status and GitHub's message.
func githubGraphQLPost(endpoint, token, source string, payload []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("HTTP request build failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := githubDecayClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP transport error: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTTP %d but body read failed: %v", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s (credential from %s)", httpReason(resp.StatusCode, body), source)
	}
	return body, nil
}
