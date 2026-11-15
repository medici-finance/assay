package main

// kindcollision_test.go — #6267: a blocker reference that STATES its kind (an /issues/ URL) is
// read typed. On GitLab issue #N and merge request !N coexist; the bare resolver refuses such
// a number, so a held non-pass whose blocker was an /issues/ URL could never be released. A
// bare #N still names no kind and stays on the resolver, which keeps refusing.
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
func TestBlockerReadKindCollision(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	project := "/api/v4/projects/example-org%2Ftracker/"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch strings.TrimPrefix(r.URL.EscapedPath(), project) {
		case "issues/9":
			_, _ = rw.Write([]byte(`{"id":1009,"iid":9,"title":"blocker","state":"closed"}`))
		case "merge_requests/9":
			_, _ = rw.Write([]byte(`{"id":2009,"iid":9,"title":"change","state":"opened"}`))
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	gl := &deskkit.GitLabForge{Token: "fixture-token", BaseURL: srv.URL}
	src := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return gl, nil }}

	ref, err := deskkit.ParseBlockerRef("https://gitlab.example/example-org/tracker/issues/9", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if st, why := src.IssueState(ref); st != deskkit.BlockerClosed {
		t.Fatalf("#6267: an /issues/ blocker names its kind; the closed issue must release the hold, "+
			"not refuse as both-kinds (or read the open merge request): %v %q", st, why)
	}
	// Control: a bare #9 names no kind, and the resolver's refusal is unchanged.
	bare, _ := deskkit.ParseBlockerRef("example-org/tracker#9", "", "")
	if st, why := src.IssueState(bare); st != deskkit.BlockerCouldNotCheck || !strings.Contains(why, "BOTH") {
		t.Fatalf("a bare #9 on a both-kinds number must stay could-not-check naming the ambiguity; got %v %q", st, why)
	}
	// One pass's observations are keyed by what the reference names, in either order: the
	// refused bare read never poisons the typed one, nor the typed read answer the bare one.
	src = &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return gl, nil }}
	if st, _ := src.IssueState(bare); st != deskkit.BlockerCouldNotCheck {
		t.Fatalf("bare-first read: %v, want could-not-check", st)
	}
	if st, why := src.IssueState(ref); st != deskkit.BlockerClosed {
		t.Fatalf("the typed read after a refused bare read of the same number: %v %q, want closed", st, why)
	}
}
