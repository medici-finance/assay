package main

// wireshape_test.go — the two WIRE SHAPES of one comment author, and a reader that feeds the
// REST shape through deskinbox's REAL detail path.
//
// The oracle reads comments with `gh issue view --json comments`, whose `author.login` for a
// GitHub App is the BARE slug (`<slug>`). deskinbox reads them over REST
// (GET /issues/{n}/comments, detail.go), whose `user.login` for the SAME App is
// `<slug>[bot]`. A parity fixture that hands both sides one string cannot see that the two
// readers disagree before the format builder ever runs — which is exactly how the desk-note
// author label and the desk-note SELECTION drifted from the oracle unnoticed. Every fixture
// comment here therefore carries BOTH shapes: the oracle is fed `GH`, and deskinbox is fed
// `REST` over an httptest server and reads it back through the real fetchDetail →
// fetchComments → listIssueComments chain.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// wireComment is one comment as each reader receives it.
type wireComment struct {
	GH   string // author.login as `gh issue view --json comments` reports it (the oracle's input)
	REST string // user.login as GET /repos/{o}/{r}/issues/{n}/comments reports it (deskinbox's input)
	Body string
}

// appComment is a GitHub App's comment: REST suffixes the account with `[bot]`, gh does not.
func appComment(slug, body string) wireComment {
	return wireComment{GH: slug, REST: slug + "[bot]", Body: body}
}

// userComment is a user account's comment: both wire shapes spell the login identically.
func userComment(login, body string) wireComment {
	return wireComment{GH: login, REST: login, Body: body}
}

// oracleDetail is the `gh issue view --json body,comments` document the oracle's jq program
// consumes — the GH login shape.
func oracleDetail(body string, cs []wireComment) jqDetail {
	d := jqDetail{Body: body}
	for _, c := range cs {
		d.Comments = append(d.Comments, jqComment{Author: jqCommentAuthor{Login: c.GH}, Body: c.Body})
	}
	return d
}

// bodyForge serves GetIssue's body — the one typed Forge op fetchDetail reads.
type bodyForge struct {
	deskkit.Forge
	body string
}

func (f *bodyForge) GetIssue(_ deskkit.ForgeRepo, number int) (*deskkit.Issue, error) {
	return &deskkit.Issue{Number: number, State: "open", Body: f.body}, nil
}

// readDetailOverREST serves (body, cs) in the REST shape and reads it back through the REAL
// fetchDetail — the reader walk.go and html.go both call. Every seam the production path
// crosses (forge resolution, session role, token mint, API host) is pointed at the fake, so
// the comment logins reach buildRendered exactly as deskinbox would receive them from
// GitHub.
func readDetailOverREST(t *testing.T, repo deskkit.ForgeRepo, number int, body string, cs []wireComment) issueDetail {
	t.Helper()
	const token = "example-token"
	wantPath := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", repo.Owner, repo.Name, number)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath || r.Header.Get("Authorization") != "token "+token {
			http.Error(w, "unexpected request "+r.URL.Path, http.StatusNotFound)
			return
		}
		type restUser struct {
			Login string `json:"login"`
		}
		type restComment struct {
			User restUser `json:"user"`
			Body string   `json:"body"`
		}
		out := []restComment{}
		for _, c := range cs {
			out = append(out, restComment{User: restUser{Login: c.REST}, Body: c.Body})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)

	prevForge, prevRole, prevMint, prevBase := forgeFor, sessionRoleFn, mintTokenFn, forgeAPIBase
	t.Cleanup(func() {
		forgeFor, sessionRoleFn, mintTokenFn, forgeAPIBase = prevForge, prevRole, prevMint, prevBase
	})
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return &bodyForge{body: body}, repo, nil
	}
	sessionRoleFn = func(string) (string, string, error) { return "worker", "worker-desk", nil }
	mintTokenFn = func(string, string) (string, string, error) { return token, "", nil }
	forgeAPIBase = srv.URL

	d := fetchDetail(repo, number)
	if d.Unavailable {
		t.Fatalf("readDetailOverREST: the real detail reader reported Unavailable for %s#%d — the fake backend was not reached", repo.Slug(), number)
	}
	return d
}
