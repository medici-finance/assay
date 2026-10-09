package deskkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestChangeDiffReturnsAFailedBodyReadAsAnError: a diff response that ends before the length
// it declared is a failed read, not a short diff. The caller gets the error and no text.
func TestChangeDiffReturnsAFailedBodyReadAsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("diff --git a/a.go b/a.go\n"))
	}))
	t.Cleanup(srv.Close)
	g := &GitHubForge{BaseURL: srv.URL, Token: "test-token"}
	diff, err := g.ChangeDiff(ForgeRepo{Owner: "example-org", Name: "tracker"}, 7)
	if err == nil {
		t.Fatalf("ChangeDiff returned %q and no error for a response that ended early", diff)
	}
	if diff != "" {
		t.Errorf("ChangeDiff returned part of the diff beside its error: %q", diff)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Errorf("exit code = %d, want %d", ExitCodeOf(err), ExitUnverifiable)
	}
}
