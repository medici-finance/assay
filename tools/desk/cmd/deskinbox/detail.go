package main

// detail.go — the "detail fetch" build_item() makes for walk/html (assay-inbox.sh:543-567):
// one issue's body plus its comments, the input format.go's buildRendered() consumes.
//
// Body reads through the resolved Forge's GetIssue (a frozen, forge-neutral op — GitHub
// AND GitLab). Comments do not: no typed Forge op returns comment BODIES (ContentEvent, the
// trust-gate read, carries only author+time — deliberately, per its own doc comment), and
// growing the frozen interface for one consumer here would mean implementing and
// golden-pinning a GitLab discussion-notes mapping this brief does not need. Instead this
// keeps its own small, package-local, GitHub-only REST reader — the same shape
// cmd/deskpost's ghClient already uses for reads the interface does not cover (contents,
// commit-author, the trust GraphQL query). The oracle itself only ever worked against
// GitHub (`gh issue view`), so this is not a narrowing: a non-GitHub repo gets the
// detail-unavailable path (Unread — could-not-check, never empty), the SAME state the
// oracle renders when its own detail fetch fails.
//
// testdata/spec.md records this as the scope this brief's port covers; a GitLab
// discussion-notes comments reader is a natural, separately-sized follow-up.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fetchDetail is the package-var seam build_item's callers (walk.go) use — swapped in
// tests for a canned detail, so format.go's parity tests never need a network or a minted
// credential.
var fetchDetail = func(repo deskkit.ForgeRepo, number int) issueDetail {
	f, fr, ferr := forgeFor(repo.Slug())
	if ferr != nil {
		return issueDetail{Unavailable: true}
	}
	iss, ierr := f.GetIssue(fr, number)
	if ierr != nil {
		return issueDetail{Unavailable: true}
	}
	cs, cerr := fetchComments(repo, number)
	if cerr != nil {
		// The oracle treats a FAILED detail fetch as fully unread (body AND comments), not
		// a body-only partial read — build_item() has one `detail` object, and a `gh issue
		// view` failure loses both fields at once. A comments-only failure here is kept to
		// that same all-or-nothing shape rather than inventing a body-only rendering the
		// oracle never produced.
		return issueDetail{Unavailable: true}
	}
	return issueDetail{Body: iss.Body, Comments: cs}
}

// fetchComments resolves this session's role/token exactly as forge.go's custody minter
// does, then walks the comments read GitHub-only (see file header for why).
func fetchComments(repo deskkit.ForgeRepo, number int) ([]comment, error) {
	role, _, rerr := sessionRoleFn("deskinbox")
	if rerr != nil {
		return nil, rerr
	}
	if res, kerr := deskkit.ForgeKindFor(repo); kerr == nil && res.Kind != deskkit.ForgeGitHub {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: deskinbox has no %s comment reader yet — %s resolves to the %s forge",
			res.Kind, repo.Slug(), res.Kind), nil)
	}
	tok, _, merr := mintTokenFn(role, repo.Slug())
	if merr != nil {
		return nil, merr
	}
	return listIssueComments(tok, repo, number)
}

// listIssueComments walks GET /issues/{n}/comments to exhaustion.
func listIssueComments(token string, repo deskkit.ForgeRepo, number int) ([]comment, error) {
	var out []comment
	for page := 1; ; page++ {
		var chunk []struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
			Body string `json:"body"`
		}
		url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100&page=%d",
			deskkit.GitHubBaseURLOrDefault(forgeAPIBase), repo.Owner, repo.Name, number, page)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, deskkit.Unverifiable("cannot build comments request", err)
		}
		req.Header.Set("Authorization", "token "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, derr := http.DefaultClient.Do(req)
		if derr != nil {
			return nil, deskkit.Unverifiable("GET issues/comments failed", derr)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, deskkit.Unverifiable(fmt.Sprintf("GET issues/comments returned HTTP %d", resp.StatusCode), nil)
		}
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil, deskkit.Unverifiable("cannot parse issues/comments response", err)
		}
		for _, c := range chunk {
			out = append(out, comment{Author: c.User.Login, Body: c.Body})
		}
		if len(chunk) < 100 {
			break
		}
	}
	return out, nil
}
