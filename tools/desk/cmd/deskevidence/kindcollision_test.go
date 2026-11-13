package main

// kindcollision_test.go — #6267: a blocker_ref that STATES its kind (an /issues/ URL) is read
// typed. On GitLab issue #N and merge request !N coexist; the bare resolver refuses such a
// number, so an /issues/ blocker_ref was could-not-check on every colliding number. A bare #N
// still names no kind and stays on the resolver, which keeps refusing.
//
// The backend is the real GitLab forge against a recorded server.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: class #6267
func TestBlockerRefKindCollision(t *testing.T) {
	project := "/api/v4/projects/example-org%2Ftracker/"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.EscapedPath(), project) {
		case "issues/9":
			_, _ = rw.Write([]byte(`{"id":1009,"iid":9,"title":"blocker","state":"opened"}`))
		case "merge_requests/9":
			_, _ = rw.Write([]byte(`{"id":2009,"iid":9,"title":"change","state":"opened"}`))
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	gl := &deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}

	if err := validateReceiptBlockerRef(gl, "example-org", "tracker", "https://gitlab.example/example-org/tracker/issues/9"); err != nil {
		t.Fatalf("#6267: an /issues/ blocker_ref names its kind and must read the issue, not refuse as both-kinds: %v", err)
	}
	// Control: a bare #9 names no kind, and the resolver's refusal is unchanged.
	err := validateReceiptBlockerRef(gl, "example-org", "tracker", "#9")
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "BOTH") {
		t.Fatalf("a bare #9 on a both-kinds number must stay could-not-check naming the ambiguity; got %v", err)
	}
}
