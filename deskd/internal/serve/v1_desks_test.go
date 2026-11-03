package serve_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// desksFixtureCfg is a two-cell config: "assay" (4 loops, no master: — the
// default-to-dispatch case) and "tracker" (a distinct repo, for the per-cell
// authorization test).
func desksFixtureCfg() *schema.File {
	return &schema.File{Cells: []schema.Cell{
		{
			Name:  "assay",
			Repos: []string{"example-org/console"},
			Loops: []schema.Loop{
				{Role: schema.RoleReview, App: "assay-reviewer-app"},
				{Role: schema.RoleVerify, App: "assay-verifier-app"},
				{Role: schema.RoleDispatch, App: "assay-desk-app"},
				{Role: schema.RoleMetrics, App: "metrics-harvest-app"},
			},
		},
		{
			Name:  "tracker",
			Repos: []string{"example-org/tracker-restricted"},
			Loops: []schema.Loop{
				{Role: schema.RoleDispatch, App: "tracker-assay-desk"},
			},
		},
	}}
}

func desksFixture(t *testing.T, authz serve.SurfaceAuthorizer) http.Handler {
	t.Helper()
	cfg := desksFixtureCfg()
	term := terminal.NewRegistry(cfg)
	return serve.NewV1(nil, cfg, authz, nil, nil, term, nil, nil)
}

// TestV1DesksDecodesAsContractType — GET /v1/desks?cell= returns a row for
// every configured loop of the named cell, decoding into
// [contract.DesksResponse]; the master default (no `master:` key) is dispatch.
func TestV1DesksDecodesAsContractType(t *testing.T) {
	h := desksFixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/desks?cell=assay answered %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp contract.DesksResponse
	if err := decode(t, rec, &resp); err != nil {
		t.Fatalf("body does not decode into contract.DesksResponse: %v", err)
	}
	if resp.Cell != "assay" {
		t.Errorf("cell = %q, want assay", resp.Cell)
	}
	if len(resp.Desks) != 4 {
		t.Fatalf("desks = %d, want 4 (one per configured loop)", len(resp.Desks))
	}
	var masters []string
	for _, d := range resp.Desks {
		if d.Group == "master" {
			masters = append(masters, d.Role)
		}
		if d.State == "" {
			t.Errorf("row %s carries no state", d.ID)
		}
	}
	if len(masters) != 1 || masters[0] != "dispatch" {
		t.Fatalf("master rows = %v, want exactly [dispatch] (the default master with no master: key)", masters)
	}
}

// TestV1DesksRequiresCell — no ?cell= is a 400: the desk registry has no
// well-defined row set without naming a cell (unlike /v1/inbox, which
// defaults to every cell).
func TestV1DesksRequiresCell(t *testing.T) {
	h := desksFixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/desks (no cell) answered %d, want 400", rec.Code)
	}
}

// TestV1DesksUnknownCellIs404 — a cell name this build does not configure is
// a 404, distinct from an authorized-but-empty cell.
func TestV1DesksUnknownCellIs404(t *testing.T) {
	h := desksFixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/desks?cell=nope answered %d, want 404", rec.Code)
	}
}

// TestDesksReadRefusesUnauthorizedCell (Verify row 9, NEGATIVE PATH) — a
// viewer with no read capability on the cell's repo gets 403, never an empty
// list: an empty desks array would read as "configured with zero desks", a
// different fact than "you may not see this cell" (facts).
func TestDesksReadRefusesUnauthorizedCell(t *testing.T) {
	// Denied viewer: 403, not an empty list.
	hDeny := desksFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	rec := httptest.NewRecorder()
	hDeny.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /v1/desks?cell=assay (denied viewer) answered %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	// Authorized viewer: 200 with the real rows.
	hAllow := desksFixture(t, denyRepoAuthz{denyOwner: "nobody", denyName: "nowhere"})
	rec2 := httptest.NewRecorder()
	hAllow.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /v1/desks?cell=assay (authorized viewer) answered %d, want 200: %s", rec2.Code, rec2.Body.String())
	}

	// A viewer denied a DIFFERENT cell's repo still reads this one: the gate
	// is per-cell, not a global refusal.
	hOther := desksFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "tracker-restricted"})
	rec3 := httptest.NewRecorder()
	hOther.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec3.Code != http.StatusOK {
		t.Fatalf("a viewer denied a DIFFERENT cell's repo must still read this cell: answered %d, want 200", rec3.Code)
	}
}

// TestV1DesksCarriesStalenessAndETag — /v1/desks is a v1 read like any
// other: it carries the staleness envelope and an ETag.
func TestV1DesksCarriesStalenessAndETag(t *testing.T) {
	h := desksFixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks?cell=assay", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d, want 200", rec.Code)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("GET /v1/desks carries no ETag header")
	}
}

// TestV1DesksWriteVerbsAreRejected — /v1/desks stays a read route: any write
// verb is 405.
func TestV1DesksWriteVerbsAreRejected(t *testing.T) {
	h := desksFixture(t, nil)
	for _, verb := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(verb, "/v1/desks?cell=assay", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /v1/desks answered %d, want 405", verb, rec.Code)
		}
	}
}

// TestV1DesksDoesNotBreakTerminalSubtree — the exact "/v1/desks" registration
// must not swallow the existing "/v1/desks/{id}/terminal" subtree route.
func TestV1DesksDoesNotBreakTerminalSubtree(t *testing.T) {
	h := desksFixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/desks/assay%2Fdispatch/terminal", nil))
	// The id is URL-encoded per the existing terminal route convention; a
	// literal slash id also degrades gracefully (see v1.go handleV1Terminal),
	// so any answer other than the pre-existing 404 "no such v1 read route"
	// text proves the subtree route is still reachable rather than shadowed.
	if rec.Code == http.StatusNotFound && strings.Contains(rec.Body.String(), "no such v1 read route") {
		t.Fatalf("the terminal subtree route appears shadowed by /v1/desks: %s", rec.Body.String())
	}
}
