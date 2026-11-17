package main

// bodycap_test.go — deskpost's REST reader reads every body through the shared forge byte
// cap, and no longer discards a read error: an over-cap or cut-off body is could-not-check
// (exit 6), never decoded as a (truncated) answer.

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
	prev := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = prev })
	return &ghClient{owner: "example-org", repo: "example-repo", token: "example-credential", http: srv.Client()}
}

// regression: #6730
func TestDoJSONOverCapBody(t *testing.T) {
	t.Cleanup(deskkit.SetForgeBodyCapForTest(64))
	c := bodyCapClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "[]"+strings.Repeat(" ", 200))
	})
	var out []any
	err := c.doJSONRetry(http.MethodGet, "/repos/example-org/example-repo/pulls/1/reviews", nil, &out, false)
	if err == nil {
		t.Fatalf("an over-cap answer decoded as success: %v", out)
	}
	if !deskkit.IsForgeBodyOverCap(err) || deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want over-cap could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// A 200 whose body is cut off after a complete-looking prefix was decoded as that prefix
// while the read error was discarded.
//
// regression: #6730
func TestDoJSONCutOffBody(t *testing.T) {
	c := bodyCapClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "[]")
	})
	var out []any
	err := c.doJSONRetry(http.MethodGet, "/repos/example-org/example-repo/pulls/1/reviews", nil, &out, false)
	if err == nil {
		t.Fatalf("a body cut off after `[]` decoded as success: %v", out)
	}
	if deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}
