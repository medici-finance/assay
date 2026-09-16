package deskkit

// forge_github_listissues_partial_test.go — the #1032 seam half: a degraded open-issue read
// must never come back as a SHORTER listing with a nil error, because cmd/issueboard reads an
// issue's absence from that listing as evidence about it. Three shapes the incident's slow
// third sweep could have taken, each of which the unfixed backend answered with a truncated
// listing and no error.

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func issueJSON(n int) string {
	return `{"number":` + strconv.Itoa(n) + `,"title":"i` + strconv.Itoa(n) + `","user":{"login":"u","id":5},"labels":[],"created_at":"2026-01-01T00:00:00Z"}`
}

// TestListOpenIssues_EmptyBodyIsNotEmptyListing: a 200 with a ZERO-BYTE body (a proxy or a
// timed-out upstream handing back nothing) is not an empty repository — `[]` is. It must be
// an error, never a nil-error empty listing.
func TestListOpenIssues_EmptyBodyIsNotEmptyListing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // no body at all
	}))
	defer srv.Close()
	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "r"})
	if err == nil {
		t.Fatalf("#1032: a zero-byte 200 body came back as an EMPTY listing (%d issues) with no error — an absence that reads like an answer", len(got))
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("the error should say the body was empty; got: %v", err)
	}
}

// TestListOpenIssues_TruncatedBodyIsAnError: the server declares a longer body than it
// delivers (the connection dropped mid-transfer). What arrived happens to parse (`[]`), so a
// backend that discards the read error hands back an empty listing with a nil error.
func TestListOpenIssues_TruncatedBodyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "r"})
	if err == nil {
		t.Fatalf("#1032: a body cut off mid-transfer came back as a listing of %d issues with no error", len(got))
	}
}

// TestListOpenIssues_ShortPageWithNextLinkContinues: the forge's own `Link: rel="next"` is the
// authoritative more-pages signal (the GitLab arm's NextPage twin). A page shorter than the
// cap that still carries rel="next" is NOT the end of the walk.
func TestListOpenIssues_ShortPageWithNextLinkContinues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Link", `<`+"http://"+r.Host+r.URL.Path+`?state=open&per_page=100&page=2>; rel="next"`)
			_, _ = w.Write([]byte("[" + issueJSON(1) + "]"))
		case "2":
			_, _ = w.Write([]byte("[" + issueJSON(2) + "]"))
		default:
			http.Error(w, "unexpected page", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "r"})
	if err != nil {
		t.Fatalf("ListOpenIssues: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("#1032: a short page carrying rel=\"next\" ended the walk early — got %d issues, want 2", len(got))
	}
}
