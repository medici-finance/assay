package deskkit

// forge_github_ratelimit_test.go — the two additions cmd/deskmonitor consumes from the GitHub
// backend: the rate-limit classification of a non-2xx answer (IsForgeRateLimited), and the
// issue's updated_at carried on the open-issue summary.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsForgeRateLimited_ClassifiesTheForgeAnswer — a 429, and a 403 carrying the rate-limit
// signature (Retry-After, an exhausted X-RateLimit-Remaining, or a message naming the limit), are
// rate limits; a plain 403 (a missing scope) and a 404 are NOT — a poller that stopped its cycle on
// a permissions fault would stop on something retrying never clears.
func TestIsForgeRateLimited_ClassifiesTheForgeAnswer(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		headers map[string]string
		message string
		want    bool
	}{
		{"429", http.StatusTooManyRequests, nil, "Too Many Requests", true},
		{"403 secondary limit message", http.StatusForbidden, nil, "You have exceeded a secondary rate limit.", true},
		{"403 Retry-After", http.StatusForbidden, map[string]string{"Retry-After": "60"}, "Forbidden", true},
		{"403 primary limit exhausted", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0"}, "API rate limit exceeded", true},
		{"403 missing scope", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "4999"}, "Resource not accessible by integration", false},
		{"404", http.StatusNotFound, nil, "Not Found", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"message":"` + tc.message + `"}`))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			_, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "r"})
			if err == nil {
				t.Fatal("a non-2xx answer came back with no error")
			}
			if got := IsForgeRateLimited(err); got != tc.want {
				t.Fatalf("IsForgeRateLimited = %v, want %v (err: %v)", got, tc.want, err)
			}
			if tc.status == http.StatusForbidden && !IsForgeForbidden(err) {
				t.Fatalf("a rate-limited 403 must still classify as a 403: %v", err)
			}
		})
	}
}

// TestListOpenIssues_CarriesUpdatedAt — the summary carries the forge's updated_at verbatim: it is
// the half of deskmonitor's keyset that moves on a new comment.
func TestListOpenIssues_CarriesUpdatedAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"number":4,"title":"t","user":{"login":"u","id":5},"labels":[],` +
			`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-09-02T11:00:00Z"}]`))
	}))
	defer srv.Close()
	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "r"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UpdatedAt != "2026-09-02T11:00:00Z" {
		t.Fatalf("UpdatedAt not carried: %+v", got)
	}
}
