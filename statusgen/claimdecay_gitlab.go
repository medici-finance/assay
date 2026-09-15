package main

// claimdecay_gitlab.go — the GitLab half of the dead-claim decay's state read.
//
// The decay needs ONE fact per claiming branch: is the change that branch belongs
// to still in flight, or has it already merged/closed? On GitHub that fact comes
// from `gh pr list` (claimdecay.go). On GitLab there is no `gh`, and installing
// one would not help — a GitLab project has no pull requests to list. This file
// is the equivalent read against GitLab's REST v4 merge-request endpoint.
//
// Why a statusgen-LOCAL reader rather than the desk's forge seam: statusgen is its
// own Go module (statusgen/go.mod, module .../assay/statusgen) and the desk tools
// are another (tools/desk/go.mod). There is no go.work joining them, and statusgen
// ships as a single pinned binary that runs inside an adopter's CI with no
// desk-tools installed — so importing the desk's forge package would both breach
// the module boundary and drag the desk's dependency tree into every adopter's
// regen job. The reader below therefore mirrors the GitHub lister's interface
// (root → set of dead head-branch names) over net/http and the standard library
// only, through the same httpDoer seam ghfetch.go already uses, so it is fully
// exercised offline in tests.
//
// Three-state contract (docs/three-state-instrument-rule.md): every exit is either
// a read that SUCCEEDED (a set, possibly empty) or an error naming why it could
// not look. It never returns an empty set for a call that failed — an empty set
// means "read the merge requests, none of them are dead", which decays nothing,
// and an unauthenticated 401 must never be able to say that.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// gitlabMRPerPage / gitlabMRMaxPages bound the merge-request listing. The product
// (1000) deliberately matches the GitHub lister's `--limit 1000`, so the two
// forges decay the same depth of history. Reaching the cap is REPORTED (rule 2 of
// the three-state invariant: a cap that silently drops data is a lie), not
// silently treated as the whole list.
const (
	gitlabMRPerPage  = 100
	gitlabMRMaxPages = 10
)

// gitlabAPI is everything one merge-request listing needs: where the API lives,
// which project to ask about, and the credential to ask with.
type gitlabAPI struct {
	base    string // e.g. https://gitlab.example.com/api/v4
	project string // numeric id, or the URL-escaped full path
	token   string
	header  string // "PRIVATE-TOKEN" for a PAT/project token, "JOB-TOKEN" for CI_JOB_TOKEN
	source  string // which env var supplied the token, for the could-not-check reason
}

// remoteProjectPath extracts the `group/subgroup/project` path from a git remote
// URL, in both the scp-like SSH form (`git@host:group/proj.git`) and the URL forms
// (`https://host/group/proj.git`, `ssh://git@host/group/proj.git`). Returns "" when
// no path is discernible. It is the companion of forge.go's remoteHost, split the
// same way so both can be tested without a checkout.
func remoteProjectPath(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	var path string
	if !strings.Contains(s, "://") {
		// scp-like: [user@]host:path
		if at := strings.LastIndex(s, "@"); at >= 0 {
			s = s[at+1:]
		}
		colon := strings.Index(s, ":")
		if colon < 0 {
			return ""
		}
		path = s[colon+1:]
	} else {
		u, err := url.Parse(s)
		if err != nil {
			return ""
		}
		path = u.Path
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	return path
}

// resolveGitLabAPI builds the API coordinates for the project rooted at root.
//
// CI first, checkout second: inside a GitLab pipeline the predefined CI_API_V4_URL
// and CI_PROJECT_ID are authoritative (they are correct even on a self-hosted
// instance behind a relative URL root, where deriving from the remote would guess
// wrong). Outside CI they are unset and the `origin` remote is the only source, so
// the base is derived as https://<host>/api/v4 and the project as its URL-escaped
// full path.
//
// The token is REQUIRED and its absence is an error, never an unauthenticated
// attempt: GitLab answers an unauthenticated listing of a private project with
// 404, which JSON-decodes to nothing and would read exactly like "no dead merge
// requests" — the silent could-not-check this whole pass exists to prevent.
func resolveGitLabAPI(root string) (gitlabAPI, error) {
	api := gitlabAPI{}

	api.base = strings.TrimRight(strings.TrimSpace(os.Getenv("CI_API_V4_URL")), "/")
	api.project = strings.TrimSpace(os.Getenv("CI_PROJECT_ID"))
	if api.base == "" || api.project == "" {
		raw, err := remoteOriginURL(root)
		if err != nil {
			return api, fmt.Errorf("neither CI_API_V4_URL/CI_PROJECT_ID are set nor could the `origin` remote be read: %v", err)
		}
		if api.base == "" {
			host := remoteHost(raw)
			if host == "" {
				return api, fmt.Errorf("CI_API_V4_URL is unset and no host could be read from the `origin` remote %q", raw)
			}
			api.base = "https://" + host + "/api/v4"
		}
		if api.project == "" {
			path := remoteProjectPath(raw)
			if path == "" {
				return api, fmt.Errorf("CI_PROJECT_ID is unset and no project path could be read from the `origin` remote %q", raw)
			}
			// GitLab addresses a project by its URL-ENCODED full path, slashes included.
			api.project = url.PathEscape(path)
		}
	}

	// Token precedence: an explicitly-provisioned read credential first, the
	// pipeline's own job token last. CI_JOB_TOKEN is accepted because it is the
	// one credential every GitLab job has without the adopter provisioning
	// anything — but many instances do not expose the merge_requests endpoint to
	// it, so when it is refused the failure must (and does) surface as a named
	// could-not-check rather than as a clean pass.
	for _, cand := range []struct{ env, header string }{
		{"STATUSGEN_GITLAB_TOKEN", "PRIVATE-TOKEN"},
		{"GITLAB_TOKEN", "PRIVATE-TOKEN"},
		{"CI_JOB_TOKEN", "JOB-TOKEN"},
	} {
		if v := strings.TrimSpace(os.Getenv(cand.env)); v != "" {
			api.token, api.header, api.source = v, cand.header, cand.env
			break
		}
	}
	if api.token == "" {
		return api, fmt.Errorf("no GitLab API token in the environment — set STATUSGEN_GITLAB_TOKEN (or GITLAB_TOKEN) to a token with the read_api scope, or run inside a pipeline where CI_JOB_TOKEN is defined")
	}
	return api, nil
}

// gitlabHTTPDoer is the transport seam, a package var so tests substitute a
// recorded-response double and the decay is exercised with no network — the same
// injection shape ghfetch.go's httpDoer takes.
var gitlabHTTPDoer httpDoer = &http.Client{Timeout: 30 * time.Second}

// gitlabMR is the subset of a REST merge-request object this reader needs.
type gitlabMR struct {
	SourceBranch string `json:"source_branch"`
	State        string `json:"state"` // opened | closed | locked | merged
}

// listMergedClosedBranchesGitLab returns the set of source-branch names whose
// merge request is MERGED or CLOSED, for the project rooted at root. It mirrors
// listMergedClosedBranches exactly: every state is listed and only the dead ones
// are kept, so an OPEN (or locked) merge request is deliberately absent from the
// set and its branch stays a live claim.
//
// A package-level var for the same reason the GitHub lister is one — tests
// substitute a fake without a network call.
var listMergedClosedBranchesGitLab = func(root string) (map[string]bool, error) {
	api, err := resolveGitLabAPI(root)
	if err != nil {
		return nil, err
	}
	dead := map[string]bool{}
	for page := 1; page <= gitlabMRMaxPages; page++ {
		endpoint := fmt.Sprintf("%s/projects/%s/merge_requests?state=all&per_page=%d&page=%d&order_by=updated_at",
			api.base, api.project, gitlabMRPerPage, page)
		batch, err := api.getMRPage(endpoint)
		if err != nil {
			return nil, err
		}
		for _, mr := range batch {
			name := strings.TrimSpace(mr.SourceBranch)
			if name == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(mr.State)) {
			case "merged", "closed":
				dead[name] = true
			}
		}
		if len(batch) < gitlabMRPerPage {
			return dead, nil
		}
		if page == gitlabMRMaxPages {
			// Rule 2 of the three-state invariant: name the cap, the count seen, and
			// that it was reached. Under-decay is the SAFE direction (an undecayed
			// corpse keeps over-holding, it never drops a live claim), but a floor
			// presented as a total is still a lie, so say it out loud.
			fmt.Fprintf(os.Stderr, "could-not-check: dead-claim decay listed only the first %d merge requests "+
				"(page cap %d x %d reached, ordered by most-recently-updated); older merged/closed merge requests were not "+
				"examined and their branches may still be consuming their stream's dispatch cap\n",
				gitlabMRPerPage*gitlabMRMaxPages, gitlabMRMaxPages, gitlabMRPerPage)
		}
	}
	return dead, nil
}

// getMRPage performs one authenticated GET and decodes a merge-request page. Any
// non-200, transport error, or unparseable body is an error naming the reason —
// never an empty page, which the caller would read as "no more merge requests".
func (a gitlabAPI) getMRPage(endpoint string) ([]gitlabMR, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("GitLab API request build failed: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(a.header, a.token)
	resp, err := gitlabHTTPDoer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitLab API transport error: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GitLab API HTTP %d: reading the response body failed: %v", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(body))
		if len(detail) > 200 {
			detail = detail[:200]
		}
		return nil, fmt.Errorf("GitLab API HTTP %d listing merge requests (credential from %s): %s",
			resp.StatusCode, a.source, detail)
	}
	var batch []gitlabMR
	if err := json.Unmarshal(body, &batch); err != nil {
		return nil, fmt.Errorf("GitLab API HTTP 200 but the response did not parse as a merge-request list: %v", err)
	}
	return batch, nil
}
