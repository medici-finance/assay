package main

// bodycap_test.go — deskrelease's REST reader reads every body through the shared forge byte
// cap and no longer discards a read error: an over-cap or cut-off ref read is
// could-not-check (exit 6), never decoded as a (truncated) answer.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func bodyCapClient(t *testing.T, h http.HandlerFunc) *ghClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	prev := apiBaseURL
	apiBaseURL = srv.URL
	t.Cleanup(func() { apiBaseURL = prev })
	return &ghClient{owner: "example-org", repo: "example-repo", token: "example-credential", http: srv.Client()}
}

// regression: #6730
func TestDoOverCapBody(t *testing.T) {
	t.Cleanup(deskkit.SetForgeBodyCapForTest(64))
	c := bodyCapClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"ref":"refs/heads/main"}`+strings.Repeat(" ", 200))
	})
	var out refResponse
	found, err := c.do(http.MethodGet, "/repos/example-org/example-repo/git/ref/heads/main", nil, &out)
	if err == nil {
		t.Fatalf("an over-cap answer read as success (found=%v): %+v", found, out)
	}
	if !deskkit.IsForgeBodyOverCap(err) || deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want over-cap could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// regression: #6730
func TestDoCutOffBody(t *testing.T) {
	c := bodyCapClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, `{}`)
	})
	var out refResponse
	found, err := c.do(http.MethodGet, "/repos/example-org/example-repo/git/ref/heads/main", nil, &out)
	if err == nil {
		t.Fatalf("a body cut off after `{}` read as success (found=%v)", found)
	}
	if deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}
