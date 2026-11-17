package main

// bodycap_test.go — the comments reader reads each page through the shared forge byte cap: an
// over-cap page is could-not-check (exit 6), never a truncated, complete-looking "[]".

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #6730
func TestCommentsOverCapBody(t *testing.T) {
	t.Cleanup(deskkit.SetForgeBodyCapForTest(64))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]" + strings.Repeat(" ", 200)))
	}))
	t.Cleanup(srv.Close)
	prev := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = prev })

	cs, err := listIssueComments("example-credential", deskkit.ForgeRepo{Owner: "example-org", Name: "example-repo"}, 1)
	if err == nil {
		t.Fatalf("an over-cap page read as success: %d comments", len(cs))
	}
	if !deskkit.IsForgeBodyOverCap(err) || deskkit.ExitCodeOf(err) != 6 {
		t.Errorf("want over-cap could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}
