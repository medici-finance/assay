package main

// stalledbody_test.go — a 2xx whose body stalls or is empty is a could-not-check, never a
// complete answer (issue #6718, security review F-1): the release preconditions are read
// through this client, and a truncated page must not read as a verified empty result.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

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
	old := apiBaseURL
	apiBaseURL = base
	t.Cleanup(func() { apiBaseURL = old })
	return &ghClient{owner: "example-org", repo: "example-repo", token: "test-placeholder",
		http: &http.Client{Timeout: 300 * time.Millisecond}}
}

// regression: #6718
func TestDoStalledBodyRefuses(t *testing.T) {
	cases := []struct {
		name   string
		status int
		prefix string
		stall  bool
	}{
		{"truncated after a valid prefix", http.StatusOK, `[]`, true},
		{"stalled before any body", http.StatusOK, "", true},
		{"empty 200 body", http.StatusOK, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := stalledBodyClient(t, bodyServer(t, tc.status, tc.prefix, tc.stall))
			var out []map[string]any
			found, err := c.do(http.MethodGet, "/repos/example-org/example-repo/releases", nil, &out)
			if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
				t.Fatalf("do = found %v, err %v (exit %d), want an Unverifiable could-not-check", found, err, got)
			}
		})
	}
}

// regression: #6718
func TestDoNoContentStillOK(t *testing.T) {
	c := stalledBodyClient(t, bodyServer(t, http.StatusNoContent, "", false))
	var out map[string]any
	if found, err := c.do(http.MethodDelete, "/repos/example-org/example-repo/x", nil, &out); err != nil || !found {
		t.Fatalf("204 with out set: found %v err %v, want found and nil", found, err)
	}
	c = stalledBodyClient(t, bodyServer(t, http.StatusOK, "", false))
	if found, err := c.do(http.MethodPost, "/repos/example-org/example-repo/x", nil, nil); err != nil || !found {
		t.Fatalf("empty 200 with out nil: found %v err %v, want found and nil", found, err)
	}
}
