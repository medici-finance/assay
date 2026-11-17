package main

// bodycap_test.go — ghAPI reads its body through the shared forge byte cap: an over-cap
// answer is could-not-check (exit 6), never a truncated success.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6730
func TestGhAPIOverCapBody(t *testing.T) {
	t.Cleanup(deskkit.SetForgeBodyCapForTest(64))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"private":true}` + strings.Repeat(" ", 200)))
	}))
	t.Cleanup(srv.Close)
	prev := githubAPIBase
	githubAPIBase = srv.URL + "/"
	t.Cleanup(func() { githubAPIBase = prev })
	t.Setenv("GH_TOKEN", "example-credential")

	b, err := ghAPI("repos/example-org/example-repo")
	if err == nil {
		t.Fatalf("an over-cap answer read as success: %d bytes", len(b))
	}
	if !deskkit.IsForgeBodyOverCap(err) || deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want over-cap could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}
