package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDoer is the recorded-response test double for the httpDoer seam: it answers
// each request URL from a canned table, so go test exercises ghfetch with no
// network (the offline envelope). A url with no canned entry returns a transport
// error, which is itself a valid three-state case.
type fakeDoer struct {
	responses map[string]fakeResp // keyed by a substring the request URL must contain
	err       error               // when set, every Do returns this transport error
}

type fakeResp struct {
	status int
	body   string
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	for frag, resp := range f.responses {
		if strings.Contains(req.URL.String(), frag) {
			return &http.Response{
				StatusCode: resp.status,
				Body:       io.NopCloser(bytes.NewBufferString(resp.body)),
			}, nil
		}
	}
	return nil, fmt.Errorf("no canned response for %s", req.URL.String())
}

func newFakeClient(doer *fakeDoer) *ghClient {
	return &ghClient{doer: doer, base: githubAPIBase, token: "x"}
}

// TestGHFetchListPRs proves the happy path: a paged pulls list is turned into
// PRRecords, ONLY for PRs carrying exactly one Brief: trailer, with merged vs open
// classified from merged_at.
func TestGHFetchListPRs(t *testing.T) {
	page1 := `[
      {"number":67,"state":"closed","draft":false,"merged_at":"2026-08-20T00:00:00Z","merge_commit_sha":"0123456789abcdef","head":{"sha":"aaaa111"},"body":"Some work.\n\nBrief: desk-containers/02\n"},
      {"number":80,"state":"open","draft":true,"merged_at":"","merge_commit_sha":"","head":{"sha":"bbbb222"},"body":"WIP\n\nBrief: derived-board/03\n"},
      {"number":81,"state":"open","draft":false,"merged_at":"","head":{"sha":"cccc333"},"body":"no trailer here"},
      {"number":82,"state":"open","draft":false,"head":{"sha":"dddd444"},"body":"Brief: a/01\nBrief: a/02\n"}
    ]`
	c := newFakeClient(&fakeDoer{responses: map[string]fakeResp{
		"/pulls?state=all": {status: 200, body: page1},
	}})
	prs, lookedAt, reason := c.ListPRs("medici-finance/assay")
	if !lookedAt {
		t.Fatalf("want lookedAt=true; got false (reason %q)", reason)
	}
	// Only #67 (merged, one trailer) and #80 (open draft, one trailer) qualify.
	if len(prs) != 2 {
		t.Fatalf("want 2 linked PRs, got %d: %+v", len(prs), prs)
	}
	byNum := map[int]PRRecord{}
	for _, p := range prs {
		byNum[p.Number] = p
	}
	if byNum[67].State != prMerged || byNum[67].BriefRef != "desk-containers/02" || byNum[67].MergeSHA != "0123456789abcdef" {
		t.Errorf("#67 wrong: %+v", byNum[67])
	}
	if byNum[80].State != prOpen || !byNum[80].Draft || byNum[80].BriefRef != "derived-board/03" {
		t.Errorf("#80 wrong: %+v", byNum[80])
	}
}

// TestGHFetchAuthFailure proves a non-200 is a three-state could-not-check with the
// HTTP status as the reason — never an empty "nothing found".
func TestGHFetchAuthFailure(t *testing.T) {
	c := newFakeClient(&fakeDoer{responses: map[string]fakeResp{
		"/pulls?state=all": {status: 401, body: `{"message":"Bad credentials"}`},
	}})
	prs, lookedAt, reason := c.ListPRs("medici-finance/assay")
	if lookedAt {
		t.Fatalf("an auth failure must be lookedAt=false")
	}
	if prs != nil {
		t.Errorf("a failed fetch must return no records, not a partial slice; got %+v", prs)
	}
	if !strings.HasPrefix(reason, "HTTP 401") || !strings.Contains(reason, "Bad credentials") {
		t.Errorf("reason should be the HTTP status + message; got %q", reason)
	}
}

// TestGHFetchTransportError proves a transport error (no network) is a
// could-not-check with a reason, not a silent empty result.
func TestGHFetchTransportError(t *testing.T) {
	c := newFakeClient(&fakeDoer{err: fmt.Errorf("dial tcp: no route to host")})
	_, lookedAt, reason := c.ListPRs("medici-finance/assay")
	if lookedAt || reason == "" {
		t.Fatalf("transport error must be lookedAt=false with a reason; got lookedAt=%v reason=%q", lookedAt, reason)
	}
	if !strings.HasPrefix(reason, "HTTP") {
		t.Errorf("reason should be uniform (HTTP-prefixed); got %q", reason)
	}
}

// TestGHFetchReviewsAtHead proves the gate:model done witness: an APPROVED review
// whose commit_id matches the head is atHead; a stale one is not.
func TestGHFetchReviewsAtHead(t *testing.T) {
	reviews := `[
      {"state":"APPROVED","commit_id":"headsha123","user":{"login":"assay-reviewer-app"}},
      {"state":"CHANGES_REQUESTED","commit_id":"oldsha000","user":{"login":"assay-reviewer-app"}}
    ]`
	c := newFakeClient(&fakeDoer{responses: map[string]fakeResp{
		"/pulls/67/reviews": {status: 200, body: reviews},
	}})
	approved, atHead, lookedAt, reason := c.ReviewsAtHead("medici-finance/assay", 67, "headsha123")
	if !lookedAt {
		t.Fatalf("want lookedAt=true; reason %q", reason)
	}
	if !approved || !atHead {
		t.Errorf("want approved && atHead; got approved=%v atHead=%v", approved, atHead)
	}
	// A stale head → approved but not atHead (the dismissed/stale demotion input).
	_, atHead2, _, _ := c.ReviewsAtHead("medici-finance/assay", 67, "differenthead")
	if atHead2 {
		t.Errorf("an approval against a different head must NOT report atHead")
	}
}

// TestGHFetchResolveToken proves the token resolution order: --token-file over
// GITHUB_TOKEN, and an unreadable --token-file is an error.
func TestGHFetchResolveToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	if tok, err := resolveGitHubToken(""); err != nil || tok != "env-token" {
		t.Errorf("env token: want env-token,nil; got %q,%v", tok, err)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "tok")
	if err := os.WriteFile(f, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if tok, err := resolveGitHubToken(f); err != nil || tok != "file-token" {
		t.Errorf("file token: want file-token,nil; got %q,%v", tok, err)
	}
	if _, err := resolveGitHubToken(filepath.Join(dir, "missing")); err == nil {
		t.Errorf("an unreadable --token-file must be an error")
	}
}
