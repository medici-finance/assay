package adapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

// fakeGraphQL serves one canned GraphQL response body and records every
// query string it was sent, so a test can assert both the RESULT and that
// the request that produced it went through the read-only endpoint.
func fakeGraphQL(t *testing.T, body string, status int) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /graphql", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		queries = append(queries, payload.Query)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	return srv, &queries
}

func TestFetchOpenPullRequestsClassifiesShape(t *testing.T) {
	const resp = `{"data":{"repository":{"pullRequests":{"totalCount":2,"nodes":[
	  {"number":12,"title":"conflicting one","isDraft":false,"createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-08T00:00:00Z","mergeable":"CONFLICTING","reviewDecision":"APPROVED","author":{"login":"worker-a"}},
	  {"number":13,"title":"needs review","isDraft":false,"createdAt":"2026-09-02T00:00:00Z","updatedAt":"2026-09-07T00:00:00Z","mergeable":"MERGEABLE","reviewDecision":"","author":{"login":"worker-b"}}
	]}}}}`
	srv, queries := fakeGraphQL(t, resp, http.StatusOK)
	defer srv.Close()

	c := adapter.NewGitHub(srv.URL, "read-token", nil)
	prs, health := c.FetchOpenPullRequests(context.Background(), adapter.Repo{Owner: "o", Name: "r"})
	if !health.OK {
		t.Fatalf("health = %+v, want OK", health)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs, want 2", len(prs))
	}
	if prs[0].Number != 12 || prs[0].Mergeable != "CONFLICTING" || prs[0].Author != "worker-a" {
		t.Fatalf("prs[0] = %+v", prs[0])
	}
	if len(*queries) != 1 {
		t.Fatalf("expected exactly one GraphQL request per repo (no per-PR detail calls), got %d", len(*queries))
	}
	if !strings.Contains((*queries)[0], "mergeable") || !strings.Contains((*queries)[0], "reviewDecision") {
		t.Fatalf("query must ask for mergeable+reviewDecision in the SAME list request: %s", (*queries)[0])
	}
}

func TestFetchOpenPullRequestsCappedReportsNotOK(t *testing.T) {
	// totalCount exceeds the page — the response is a PREFIX, so this must
	// never be read as the full set (C4: could-not-check, never a silent
	// truncation).
	const resp = `{"data":{"repository":{"pullRequests":{"totalCount":9999,"nodes":[
	  {"number":1,"title":"x","isDraft":false,"createdAt":"2026-09-01T00:00:00Z","updatedAt":"2026-09-01T00:00:00Z","mergeable":"MERGEABLE","reviewDecision":"APPROVED","author":{"login":"a"}}
	]}}}}`
	srv, _ := fakeGraphQL(t, resp, http.StatusOK)
	defer srv.Close()
	c := adapter.NewGitHub(srv.URL, "tok", nil)
	prs, health := c.FetchOpenPullRequests(context.Background(), adapter.Repo{Owner: "o", Name: "r"})
	if health.OK {
		t.Fatalf("a capped page must report OK=false, got OK=true")
	}
	if !health.Capped {
		t.Fatalf("expected Capped=true")
	}
	if health.Reason == "" {
		t.Fatalf("a not-OK health must carry a reason")
	}
	if len(prs) != 1 {
		t.Fatalf("the (partial, honestly-marked) page should still be returned, got %d rows", len(prs))
	}
}

func TestFetchOpenPullRequestsRepoNotVisibleIsCouldNotCheck(t *testing.T) {
	const resp = `{"data":{"repository":null}}`
	srv, _ := fakeGraphQL(t, resp, http.StatusOK)
	defer srv.Close()
	c := adapter.NewGitHub(srv.URL, "tok", nil)
	prs, health := c.FetchOpenPullRequests(context.Background(), adapter.Repo{Owner: "o", Name: "r"})
	if health.OK {
		t.Fatalf("a null repository must report OK=false, never an empty-looking clean pass")
	}
	if prs != nil {
		t.Fatalf("expected no PRs, got %+v", prs)
	}
}

func TestFetchDispatchClaimsReadsRefTipCommit(t *testing.T) {
	const resp = `{"data":{"repository":{"refs":{"totalCount":1,"nodes":[
	  {"name":"refs/dispatch/toolkit--x--08","target":{"oid":"abc123","committedDate":"2026-09-09T11:00:00Z","author":{"name":"assay-worker-app"}}}
	]}}}}`
	srv, queries := fakeGraphQL(t, resp, http.StatusOK)
	defer srv.Close()
	c := adapter.NewGitHub(srv.URL, "tok", nil)
	claims, health := c.FetchDispatchClaims(context.Background(), adapter.Repo{Owner: "o", Name: "r"})
	if !health.OK {
		t.Fatalf("health = %+v, want OK", health)
	}
	if len(claims) != 1 {
		t.Fatalf("got %d claims, want 1", len(claims))
	}
	if claims[0].Holder != "assay-worker-app" {
		t.Fatalf("claim holder = %q", claims[0].Holder)
	}
	if !strings.Contains((*queries)[0], `refPrefix: "refs/dispatch/"`) {
		t.Fatalf("query must scope to refs/dispatch/*, got: %s", (*queries)[0])
	}
}

func TestFetchDispatchClaimsMissingCommittedDateStaysZero(t *testing.T) {
	// No committedDate at all (the target is not a Commit, or the API
	// omitted the inline fragment's fields) must render as the ZERO time,
	// never "now" — a fabricated fresh claimed-at is exactly the false-
	// freshness the brief's single-point-of-failure section warns against.
	const resp = `{"data":{"repository":{"refs":{"totalCount":1,"nodes":[
	  {"name":"refs/dispatch/orphan","target":{"oid":"abc123"}}
	]}}}}`
	srv, _ := fakeGraphQL(t, resp, http.StatusOK)
	defer srv.Close()
	c := adapter.NewGitHub(srv.URL, "tok", nil)
	claims, health := c.FetchDispatchClaims(context.Background(), adapter.Repo{Owner: "o", Name: "r"})
	if !health.OK {
		t.Fatalf("health = %+v, want OK", health)
	}
	if len(claims) != 1 || !claims[0].ClaimedAt.IsZero() {
		t.Fatalf("expected one claim with a zero ClaimedAt, got %+v", claims)
	}
}

func TestFleetReadsUseOnlyReadOnlyTransport(t *testing.T) {
	// Both fleet reads are GraphQL POSTs; the read-only transport permits
	// those only when the document contains no mutation/subscription.
	// This proves the queries this file builds actually pass that guard
	// (rather than relying on the guard's own unit tests alone).
	srv, queries := fakeGraphQL(t, `{"data":{"repository":{"pullRequests":{"totalCount":0,"nodes":[]},"refs":{"totalCount":0,"nodes":[]}}}}`, http.StatusOK)
	defer srv.Close()
	c := adapter.NewGitHub(srv.URL, "tok", nil) // NewGitHub always wraps ReadOnlyTransport — see readonly_test.go
	if _, health := c.FetchOpenPullRequests(context.Background(), adapter.Repo{Owner: "o", Name: "r"}); !health.OK {
		t.Fatalf("FetchOpenPullRequests: %+v", health)
	}
	if _, health := c.FetchDispatchClaims(context.Background(), adapter.Repo{Owner: "o", Name: "r"}); !health.OK {
		t.Fatalf("FetchDispatchClaims: %+v", health)
	}
	for _, q := range *queries {
		lower := strings.ToLower(q)
		if strings.Contains(lower, "mutation") || strings.Contains(lower, "subscription") {
			t.Fatalf("fleet read query must never contain mutation/subscription: %s", q)
		}
	}
}
