package deskkit

// forge_github_ratelimit_test.go — the two additions cmd/deskmonitor consumes from the GitHub
// backend: the rate-limit classification of a non-2xx answer (IsForgeRateLimited), and the
// issue's updated_at carried on the open-issue summary.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsForgeRateLimited_ClassifiesTheForgeAnswer — the classification is the oracle scripts'
// is_ratelimit (`secondary rate limit|(http )?429|too many requests`, case-insensitive) over the
// text gh prints for the answer, `HTTP <status>: <message>` with each errors[] item on its own
// line, and over nothing else: no header, no status class on its own. So a 429, and any status
// whose text carries the signature, are rate limits; a plain 403 (a missing scope), a 404, a
// PRIMARY-quota 403, and a 403 whose only rate-limit sign is a Retry-After header are NOT — a
// poller that stopped its cycle on any of those would diverge from
// plugins/assay/scripts/{inbound,pr}-monitor.sh, which cannot see a header and treat the read as
// an ordinary failure (#1640 review F1).
func TestIsForgeRateLimited_ClassifiesTheForgeAnswer(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		headers map[string]string
		body    string // a JSON body, or raw text with contentType set
		ctype   string
		want    bool
	}{
		{"429", http.StatusTooManyRequests, nil, `{"message":"Too Many Requests"}`, "", true},
		{"429 with no message", http.StatusTooManyRequests, nil, `{}`, "", true},
		{"429 non-JSON body (go-gh uses the status line)", http.StatusTooManyRequests, nil, "slow down", "text/plain", true},
		{"403 secondary limit message", http.StatusForbidden, nil, `{"message":"You have exceeded a secondary rate limit."}`, "", true},
		{"403 secondary limit, any case", http.StatusForbidden, nil, `{"message":"SECONDARY RATE LIMIT"}`, "", true},
		{"403 too many requests, no header", http.StatusForbidden, nil, `{"message":"Too many requests"}`, "", true},
		{"403 signature in an errors[] item", http.StatusForbidden, nil, `{"message":"Forbidden","errors":["You have exceeded a secondary rate limit."]}`, "", true},
		{"502 naming too many requests", http.StatusBadGateway, nil, `{"message":"too many requests from this client"}`, "", true},
		{"403 primary quota whose id carries 429 (the scripts' bare 429 branch)", http.StatusForbidden, nil, `{"message":"API rate limit exceeded for installation ID 4290."}`, "", true},
		{"403 Retry-After alone (a header gh never prints)", http.StatusForbidden, map[string]string{"Retry-After": "60"}, `{"message":"Forbidden"}`, "", false},
		{"403 abuse-detection wording with Retry-After", http.StatusForbidden, map[string]string{"Retry-After": "60"}, `{"message":"You have triggered an abuse detection mechanism."}`, "", false},
		{"403 primary limit exhausted (X-RateLimit-Remaining only, no Retry-After)", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0"}, `{"message":"API rate limit exceeded for installation ID 1."}`, "", false},
		{"403 missing scope", http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "4999"}, `{"message":"Resource not accessible by integration"}`, "", false},
		{"500 plain", http.StatusInternalServerError, nil, `{"message":"Server Error"}`, "", false},
		{"404", http.StatusNotFound, nil, `{"message":"Not Found"}`, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				ct := tc.ctype
				if ct == "" {
					ct = "application/json"
				}
				w.Header().Set("Content-Type", ct)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			// The repo name carries 429: the request path is the verb's own text, never the
			// forge's answer, so it must not tip a plain failure into a rate limit.
			_, err := gh.ListOpenIssues(ForgeRepo{Owner: "o", Name: "app429"})
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
