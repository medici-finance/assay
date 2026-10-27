package serve_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// localAuthz is a stub authorizer that answers both the read predicate
// ([serve.SurfaceAuthorizer], allow-all) and the per-item local-state question
// ([serve.LocalStateAuthorizer]) with fixed verdicts.
type localAuthz struct{ authenticated, allowed bool }

func (a localAuthz) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(string, string) bool { return true }
}

func (a localAuthz) LocalStateAllowed(*http.Request, string, string) (bool, bool) {
	return a.authenticated, a.allowed
}

// repoAuthz is a per-item authorizer that grants Commenter on exactly ONE repo
// and records every (owner, name) it is asked about — so a test can pin WHICH
// repo the handler authorized the write against, not merely the verdict.
type repoAuthz struct {
	grant string    // the one "owner/name" this viewer may mark read on
	asked *[]string // every "owner/name" LocalStateAllowed was asked about
}

func (a repoAuthz) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(string, string) bool { return true }
}

func (a repoAuthz) LocalStateAllowed(_ *http.Request, owner, name string) (bool, bool) {
	*a.asked = append(*a.asked, owner+"/"+name)
	return true, owner+"/"+name == a.grant
}

// readOnlyAuthz implements ONLY the read predicate — an authorizer that cannot
// answer the per-item write question.
type readOnlyAuthz struct{}

func (readOnlyAuthz) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(string, string) bool { return true }
}

// readFixture seeds one open escalation and returns the /v1 handler plus the db
// so a test can read the local attention table back.
func readFixture(t *testing.T, authz serve.SurfaceAuthorizer) (http.Handler, *index.DB) {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := adapter.Repo{Owner: "example-org", Name: "console"}
	escs := []domain.Escalation{{
		ID: "IT1", Cell: "assay", Kind: domain.KindNeedsDecision, Provenance: domain.AgentRaised,
		Urgency: domain.UrgencyRoutine, Title: "a v1 decision", Situation: "a real decision",
		Rev: "r1", FiledAt: time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{
		"IT1": {Number: 11, URL: "https://github.com/example-org/console/issues/11"},
	}
	if err := db.UpsertEscalations(repo, escs, raw); err != nil {
		t.Fatalf("UpsertEscalations: %v", err)
	}
	cfg := &schema.File{Cells: []schema.Cell{{Name: "assay", Repos: []string{"example-org/console"}, SilenceThreshold: "48h"}}}
	return serve.NewV1(db, cfg, authz, nil, nil, nil, nil, nil), db
}

func postRead(h http.Handler, id string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, contract.Prefix+"/escalations/"+id+"/read", nil))
	return rec
}

func readMarker(t *testing.T, db *index.DB, id string) *time.Time {
	t.Helper()
	read, _, err := db.Attention(domain.EscalationID(id))
	if err != nil {
		t.Fatalf("Attention: %v", err)
	}
	return read
}

// TestV1ReadRouteIsMounted: the contract declares POST /v1/escalations/{id}/read
// (KindLocalState, ruling D7) and contract.Client.Read calls it; before
// increment 4 the no-general-write catch-all 404'd it. It must record the local
// read marker and answer 204, as the contract documents.
func TestV1ReadRouteIsMounted(t *testing.T) {
	h, db := readFixture(t, localAuthz{authenticated: true, allowed: true})
	rec := postRead(h, "IT1")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("POST /v1/escalations/IT1/read answered %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if readMarker(t, db, "IT1") == nil {
		t.Fatal("the read marker was not recorded in the local table")
	}
}

// TestV1ReadThroughContractClient is the round trip the gap was about: the ONE
// client a service caller uses (contract.Client) marks an item read over the
// real handler.
func TestV1ReadThroughContractClient(t *testing.T) {
	h, db := readFixture(t, localAuthz{authenticated: true, allowed: true})
	srv := httptest.NewServer(h)
	defer srv.Close()
	if err := contract.NewClient(srv.URL).Read(context.Background(), "IT1"); err != nil {
		t.Fatalf("contract.Client.Read: %v", err)
	}
	if readMarker(t, db, "IT1") == nil {
		t.Fatal("contract.Client.Read succeeded but no read marker was recorded")
	}
}

// TestV1ReadRouteIsGatedPerItem: the marker is a WRITE, so it is never looser
// than the un-versioned item-scoped route — unauthenticated is 401, an
// authenticated viewer without commenter is 403, an unknown id is 404 — and in
// none of those cases is anything recorded.
func TestV1ReadRouteIsGatedPerItem(t *testing.T) {
	cases := []struct {
		name  string
		authz localAuthz
		id    string
		want  int
	}{
		{"unauthenticated", localAuthz{authenticated: false}, "IT1", http.StatusUnauthorized},
		// Authentication is decided BEFORE existence: an unauthenticated caller
		// probing an id that does not exist gets the same 401 as for one that
		// does, so 401-vs-404 is never an enumeration oracle.
		{"unauthenticated, unknown item", localAuthz{authenticated: false}, "nope", http.StatusUnauthorized},
		{"no commenter on the repo", localAuthz{authenticated: true, allowed: false}, "IT1", http.StatusForbidden},
		{"unknown item", localAuthz{authenticated: true, allowed: true}, "nope", http.StatusNotFound},
	}
	for _, c := range cases {
		h, db := readFixture(t, c.authz)
		rec := postRead(h, c.id)
		if rec.Code != c.want {
			t.Errorf("%s: answered %d, want %d: %s", c.name, rec.Code, c.want, rec.Body.String())
		}
		if readMarker(t, db, "IT1") != nil {
			t.Errorf("%s: a refused request still recorded the read marker", c.name)
		}
	}
}

// TestV1ReadBindsIndexedRepo pins the per-item BINDING, not just the
// verdict: the repo the write is authorized against is the one the trusted
// index row's platform link names, never one the request supplies. The viewer
// here holds Commenter on a DIFFERENT repo and names it every way a request
// could (query and header); the item belongs to example-org/console,
// so the answer is 403, nothing is recorded, and the only repo the authorizer
// was ever asked about is the indexed one.
func TestV1ReadBindsIndexedRepo(t *testing.T) {
	var asked []string
	h, db := readFixture(t, repoAuthz{grant: "attacker/writable", asked: &asked})
	req := httptest.NewRequest(http.MethodPost,
		contract.Prefix+"/escalations/IT1/read?repo=attacker/writable&owner=attacker&name=writable&o=attacker&n=writable", nil)
	req.Header.Set("X-Desk-Repo", "attacker/writable")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a request steering the repo answered %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if readMarker(t, db, "IT1") != nil {
		t.Error("a request steering the repo recorded the read marker")
	}
	if len(asked) != 1 || asked[0] != "example-org/console" {
		t.Errorf("the write was authorized against %q, want exactly [example-org/console] (the index row's platform link)", asked)
	}
}

// TestV1ReadAuthnBeforeIndexErr: a failing index read must not be
// answered ahead of authentication — an unauthenticated caller gets the 401
// every other unauthenticated request gets, never a 500 carrying index state.
func TestV1ReadAuthnBeforeIndexErr(t *testing.T) {
	h, db := readFixture(t, localAuthz{authenticated: false})
	if err := db.Close(); err != nil {
		t.Fatalf("could-not-check: closing the index to force a read failure: %v", err)
	}
	if rec := postRead(h, "IT1"); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated read with a failing index answered %d, want 401: %s", rec.Code, rec.Body.String())
	}
	h, db = readFixture(t, localAuthz{authenticated: true, allowed: true})
	if err := db.Close(); err != nil {
		t.Fatalf("could-not-check: closing the index to force a read failure: %v", err)
	}
	if rec := postRead(h, "IT1"); rec.Code != http.StatusInternalServerError {
		t.Errorf("authenticated read with a failing index answered %d, want 500 (BLIND, not empty): %s", rec.Code, rec.Body.String())
	}
}

// TestReadMarkerAbsentWithoutItemGate: with no authorizer that can answer
// the per-item write question, the route is not mounted — the catch-all 404,
// never a route that exists and writes ungated.
func TestReadMarkerAbsentWithoutItemGate(t *testing.T) {
	for name, authz := range map[string]serve.SurfaceAuthorizer{
		"nil authorizer":       nil,
		"read-only authorizer": readOnlyAuthz{},
	} {
		h, db := readFixture(t, authz)
		if rec := postRead(h, "IT1"); rec.Code != http.StatusNotFound {
			t.Errorf("%s: POST read answered %d, want 404 (unmounted)", name, rec.Code)
		}
		if readMarker(t, db, "IT1") != nil {
			t.Errorf("%s: the unmounted route still recorded a read marker", name)
		}
	}
}
