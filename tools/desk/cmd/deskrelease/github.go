package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// apiBaseURL is the GitHub API base. It is overridable ONLY from in-package tests (a
// fake httptest server); there is deliberately NO env var or flag override — a
// production override could redirect the tag create at an attacker-controlled host.
// Same test-hook shape deskpost uses. The host literal is sourced from deskkit.GitHubAPIBase
// (the forge module) so it is never constructed in a cmd package (the forge-abstraction seam).
var apiBaseURL = deskkit.GitHubAPIBase

// execCommand is the single seam through which the ONLY external program this tool ever
// runs — `desktoken` — is invoked. Tests bind it to a recorder so the "no git, no gh,
// ever" property is asserted against the real constructed argv rather than assumed.
var execCommand = exec.Command

// deskTokenPath is the desktoken binary. It is an absolute path so a PATH entry cannot
// substitute a different program for the identity mint.
const deskTokenPath = "/opt/desk-tools/bin/desktoken"

// sha40 matches a full git object id. A ref response that does not carry one is
// unverifiable, never "probably fine".
var sha40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// mintDeskToken runs `desktoken desk --repo <slug>` and reads the installation token
// from the cache path it prints. Acting as the desk App (the desk App) is what
// puts a NAMED identity in the release's audit trail instead of whatever ambient
// credential the calling session happens to hold.
//
// The token value never reaches stdout, the audit line, or an error message.
func mintDeskToken(repo string) (string, error) {
	cmd := execCommand(deskTokenPath, "desk", "--repo", repo)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"desktoken desk --repo %s failed (%s) — the desk App token cannot be minted",
			repo, strings.TrimSpace(errb.String())), err)
	}
	// desktoken prints the cache PATH (never the secret); take the last non-empty line.
	var tokenPath string
	for _, line := range strings.Split(out.String(), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			tokenPath = s
		}
	}
	if tokenPath == "" {
		return "", deskkit.Unverifiable("desktoken printed no token path", nil)
	}
	b, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", deskkit.Unverifiable("cannot read desk App token from "+tokenPath, err)
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", deskkit.Unverifiable("desk App token at "+tokenPath+" is empty", nil)
	}
	return tok, nil
}

// ghClient is the desk-App-authenticated client for the one compiled-in repo. It exposes
// exactly two ref operations — read a ref, and CREATE a tag ref. There is no update, no
// delete, and no PATCH: the verbs that could move or remove a ref are absent from the
// type, so no future edit to planCut can reach them by accident.
type ghClient struct {
	owner, repo string
	token       string
	http        *http.Client
}

func newGHClient(slug string) (*ghClient, error) {
	owner, name, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || name == "" {
		return nil, deskkit.Unverifiable("compiled-in repo slug is malformed: "+slug, nil)
	}
	tok, err := mintDeskToken(slug)
	if err != nil {
		return nil, err
	}
	return &ghClient{owner: owner, repo: name, token: tok, http: http.DefaultClient}, nil
}

// do performs one REST call. A 2xx body is decoded into out; a 404 is reported via the
// found flag; every OTHER non-2xx is Unverifiable (exit 6) — an API error means the
// precondition could not be positively verified, and this tool never guesses past one.
func (c *ghClient) do(method, path string, in, out any) (found bool, err error) {
	var body io.Reader
	if in != nil {
		b, merr := json.Marshal(in)
		if merr != nil {
			return false, deskkit.Unverifiable("cannot marshal request body", merr)
		}
		body = bytes.NewReader(b)
	}
	req, rerr := http.NewRequest(method, apiBaseURL+path, body)
	if rerr != nil {
		return false, deskkit.Unverifiable("cannot build request", rerr)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, derr := c.http.Do(req)
	if derr != nil {
		return false, deskkit.Unverifiable(fmt.Sprintf("%s %s failed", method, path), derr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, deskkit.Unverifiable(fmt.Sprintf(
			"GitHub API %s %s returned HTTP %d", method, path, resp.StatusCode), nil)
	}
	if out != nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, out); uerr != nil {
			return false, deskkit.Unverifiable(fmt.Sprintf("cannot parse %s %s response", method, path), uerr)
		}
	}
	return true, nil
}

// refResponse is the git-data ref shape (only the fields consumed).
type refResponse struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

// getRef reads a single ref (`tags/<tag>` or `heads/main`) and returns the commit it
// points at. found=false means the ref does not exist (HTTP 404) — the ONLY non-error
// negative. A ref that exists but does not point at a commit, or whose sha is not a
// 40-hex object id, is Unverifiable.
//
// GET /repos/{owner}/{repo}/git/ref/{ref} — an exact-match read. The plural
// `/git/matching-refs/` form is deliberately not used: it PREFIX-matches, so
// `tags/desk-tools/v0.1.1` would also report `…v0.1.10`, and an existence check that
// prefix-matches is an existence check that can be fooled.
func (c *ghClient) getRef(ref string) (sha string, found bool, err error) {
	var r refResponse
	path := fmt.Sprintf("/repos/%s/%s/git/ref/%s", c.owner, c.repo, refPathEscape(ref))
	found, err = c.do(http.MethodGet, path, nil, &r)
	if err != nil || !found {
		return "", found, err
	}
	if r.Object.Type != "commit" {
		return "", true, deskkit.Unverifiable(fmt.Sprintf(
			"ref %s points at a %q object, not a commit", ref, r.Object.Type), nil)
	}
	if !sha40.MatchString(r.Object.SHA) {
		return "", true, deskkit.Unverifiable(fmt.Sprintf(
			"ref %s reports a malformed object id %q", ref, r.Object.SHA), nil)
	}
	return r.Object.SHA, true, nil
}

// createTagRef creates refs/tags/<tag> at sha and returns the commit GitHub reports the
// new ref pointing at.
//
// POST /repos/{owner}/{repo}/git/refs is CREATE-ONLY: GitHub answers 422 for a ref that
// already exists and there is no force parameter. Moving a ref would need PATCH
// /git/refs/{ref}, which this file does not implement. So "never move a published tag"
// is a property of the transport, not only of the check in planCut.
//
// A 422 is mapped to a REFUSAL rather than exit 6 — it is a definite answer from the
// remote ("that ref is already there"), not an unverifiable one, and it is the racing
// twin of the exists-check.
func (c *ghClient) createTagRef(tag, sha string) (string, error) {
	if !sha40.MatchString(sha) {
		return "", deskkit.Unverifiable("refusing to create a tag at malformed sha "+sha, nil)
	}
	var r refResponse
	path := fmt.Sprintf("/repos/%s/%s/git/refs", c.owner, c.repo)
	in := map[string]any{"ref": "refs/tags/" + tag, "sha": sha}
	_, err := c.do(http.MethodPost, path, in, &r)
	if err != nil {
		if strings.Contains(err.Error(), "returned HTTP 422") {
			return "", deskkit.Refused(fmt.Sprintf(
				"refused: GitHub rejected creating refs/tags/%s (HTTP 422) — the ref already "+
					"exists and is never moved; fix forward with the next patch version", tag))
		}
		return "", err
	}
	if !sha40.MatchString(r.Object.SHA) {
		return "", deskkit.Unverifiable(fmt.Sprintf(
			"create refs/tags/%s returned a malformed object id %q", tag, r.Object.SHA), nil)
	}
	return r.Object.SHA, nil
}

// refPathEscape percent-encodes each path SEGMENT of a ref, preserving the `/` separators
// the API needs (`tags/desk-tools/v0.1.3`). validateTag has already excluded everything
// interesting; this is the belt to that braces, so no argument can extend the URL path.
func refPathEscape(ref string) string {
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
