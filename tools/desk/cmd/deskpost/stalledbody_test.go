package main

// stalledbody_test.go — a 2xx whose body stalls or is empty is a could-not-check, never a
// complete answer (issue #6718, security review F-1). With the per-request deadline in place a
// body that stalls past it ends in a read error; reading what arrived as the whole answer would
// turn a truncated check-run or review page into an empty, "all clear" list.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bodyServer answers every request with status, writes prefix, flushes, then stalls (when
// stall is set) until the test ends.
func bodyServer(t *testing.T, status int, prefix string, stall bool) string {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !stall {
			w.Header().Set("Content-Length", "0")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(prefix))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		if stall {
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	return srv.URL
}

func stalledBodyClient(t *testing.T, base string) *ghClient {
	t.Helper()
	old := forgeAPIBase
	forgeAPIBase = base
	t.Cleanup(func() { forgeAPIBase = old })
	return &ghClient{owner: "example-org", repo: "example-repo", token: "test-placeholder",
		http: &http.Client{Timeout: 300 * time.Millisecond}}
}

// regression: #6718
func TestDoJSONStalledBodyRefuses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		prefix string
		stall  bool
	}{
		// A complete-looking JSON prefix, then the body stalls past the deadline: the
		// decode of what arrived would succeed and read as an empty list.
		{"truncated after a valid prefix", http.StatusOK, `{"total_count":0,"check_runs":[]}`, true},
		// Headers and 200, then no body at all before the deadline.
		{"stalled before any body", http.StatusOK, "", true},
		// A 200 that carries no body where a result was asked for.
		{"empty 200 body", http.StatusOK, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := stalledBodyClient(t, bodyServer(t, tc.status, tc.prefix, tc.stall))
			var out struct {
				TotalCount int `json:"total_count"`
			}
			err := c.doJSONRetry(http.MethodGet, "/repos/example-org/example-repo/commits/x/check-runs", nil, &out, false)
			if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
				t.Fatalf("doJSONRetry = %v (exit %d), want an Unverifiable could-not-check", err, got)
			}
		})
	}
}

// regression: #6718
func TestDoJSONNoContentStillOK(t *testing.T) {
	// 204 carries no body by definition; a caller that passed out still succeeds.
	c := stalledBodyClient(t, bodyServer(t, http.StatusNoContent, "", false))
	var out map[string]any
	if err := c.doJSONRetry(http.MethodPut, "/repos/example-org/example-repo/x", nil, &out, false); err != nil {
		t.Fatalf("204 with out set: %v, want nil", err)
	}
	// An empty 2xx where the caller asked for nothing back is fine too.
	c = stalledBodyClient(t, bodyServer(t, http.StatusOK, "", false))
	if err := c.doJSONRetry(http.MethodPost, "/repos/example-org/example-repo/x", nil, nil, false); err != nil {
		t.Fatalf("empty 200 with out nil: %v, want nil", err)
	}
}
