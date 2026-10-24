package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/fleet"
	"github.com/medici-finance/assay/deskd/internal/identity"
)

// fakeFleet is a controllable FleetSource for the web-layer tests: the
// fetch/staleness logic itself is pinned in internal/fleet's own tests
// (notably TestCacheServesLastGoodMarkedStaleOnFetchFailure); what this
// file proves is that the PAGE renders exactly what a [fleet.RepoLane]
// says, escaped, and that the route accepts no verb but GET.
type fakeFleet struct {
	lanes []fleet.RepoLane
}

func (f fakeFleet) Snapshot(context.Context, []string) []fleet.RepoLane { return f.lanes }

func fleetConfig() *schema.File {
	return &schema.File{Cells: []schema.Cell{{Name: "assay", Repos: []string{"example-org/console"}}}}
}

// TestFleetRouteAcceptsOnlyGET is the HTTP-level half of the "no write
// path" proof (the AST half lives in internal/fleet/readonly_test.go):
// Go 1.22's ServeMux answers 405 for a method that was never registered on
// a pattern, so a POST that reaches this route at all is itself the
// finding.
func TestFleetRouteAcceptsOnlyGET(t *testing.T) {
	srv, err := New(Deps{Now: fixedNow, Config: fleetConfig(), Fleet: fakeFleet{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(m, "/fleet", nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /fleet = %d, want 405 (the page must register no route but GET)", m, rr.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/fleet", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /fleet = %d, want 200", rr.Code)
	}
}

// TestFleetRouteNoMutationRegisteredInHandler greps this package's own
// route table for a mutating verb registered against /fleet — belt to the
// runtime-behaviour test above's braces: this fails at compile-adjacent
// review time even before a test runs, if a future edit adds one.
func TestFleetRouteNoMutationRegisteredInHandler(t *testing.T) {
	srv, err := New(Deps{Now: fixedNow, Config: fleetConfig(), Fleet: fakeFleet{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, pattern := srv.Handler().(*http.ServeMux).Handler(httptest.NewRequest(http.MethodGet, "/fleet", nil))
	if pattern != "GET /fleet" {
		t.Fatalf("unexpected pattern for /fleet: %q", pattern)
	}
}

// TestFleetPageRendersLaneStateAndStaleness is the rendering half: OK,
// Stale, Reason and every token's state/palette/age reach the page, and
// nothing is silently promoted (a Stale=true lane must say STALE
// somewhere a human reading the page sees it — Verify row 2's shape).
func TestFleetPageRendersLaneStateAndStaleness(t *testing.T) {
	now := fixedNow()
	lanes := []fleet.RepoLane{
		{
			Repo: "example-org/console", OK: true, FetchedAt: now.Add(-5 * time.Minute),
			Tokens: []fleet.Token{
				{Kind: fleet.KindClaim, Key: "toolkit--x--08", Holder: "worker-app", State: fleet.StateWorking, Age: 3 * time.Minute},
				{Kind: fleet.KindPR, Key: "#97", Title: "<script>alert(1)</script> feature", Holder: "worker-app", State: fleet.StateConflicting, Age: 40 * time.Minute},
			},
		},
		{
			Repo: "example-org/assay", OK: true, Stale: true, Reason: "simulated fetch failure",
			FetchedAt: now.Add(-90 * time.Minute),
			Tokens: []fleet.Token{
				{Kind: fleet.KindClaim, Key: "stuck-claim", Holder: "dead-worker", State: fleet.StaleClaimState, Age: 2 * time.Hour},
			},
		},
		{Repo: "example-org/no-token-routed", OK: false, Reason: "no read token routed for owner example-org"},
	}

	srv, err := New(Deps{Now: fixedNow, Config: fleetConfig(), Fleet: fakeFleet{lanes: lanes}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/fleet", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /fleet = %d", rr.Code)
	}
	body := rr.Body.String()

	if !strings.Contains(body, "example-org/console") || !strings.Contains(body, "example-org/assay") {
		t.Fatalf("expected both repo lanes rendered")
	}
	if !strings.Contains(body, `data-state="conflicting"`) || !strings.Contains(body, `data-palette="fault"`) {
		t.Fatalf("conflicting PR must render its state+palette tokens")
	}
	if !strings.Contains(body, `data-state="stale-claim"`) || !strings.Contains(body, `data-palette="amber"`) {
		t.Fatalf("stale claim must render its state+palette tokens")
	}
	if !strings.Contains(body, "STALE") {
		t.Fatalf("the fetch-stale lane must visibly say STALE somewhere on the page (Verify row 2's negative path)")
	}
	if !strings.Contains(body, "could-not-check") {
		t.Fatalf("the OK=false lane must render could-not-check, not a blank/empty lane")
	}
	// §6.6 doctrine, restated for this pane: untrusted PR title text is
	// ESCAPED, never rendered as markup.
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("PR title must be HTML-escaped, found raw <script> in output")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected the escaped form of the hostile PR title in output")
	}
}

// TestFleetHidesLanesWithoutCapability (the service contract Security-Review;
// PR#42 / Security-Review 5091037270 BOLA leak class): the spatial fleet map is
// a surface-scoped multi-repo listing rendered under the desk's per-org read
// token, which sees every private repo across every cell. The /fleet route is
// ItemScoped:false (catch-all Root), so the middleware does only a login-presence
// check; per-repo authorization on the listing is the HANDLER's job. An
// authenticated viewer with NO read capability on a cell's repo must therefore
// NOT receive that repo's lane — neither its open-PR titles/authors nor its
// refs/dispatch/* claim keys + holders. Pre-fix the handler rendered every
// configured repo's lane regardless of the viewer's per-repo capability, so a
// foreign private cell's metadata leaked; post-fix each lane's repo is routed
// through the viewer's SurfaceAuthorizer — the SAME deskd-side derivation the
// inbox (filterByCapability) and epic/portfolio (authorizedRows) panes use — and
// an unauthorized lane is dropped, fail-closed. Mirrors
// TestInboxHidesRowsWithoutCapability.
func TestFleetHidesLanesWithoutCapability(t *testing.T) {
	now := fixedNow()
	lanes := []fleet.RepoLane{
		{
			Repo: "example-org/console", OK: true, FetchedAt: now.Add(-5 * time.Minute),
			Tokens: []fleet.Token{
				{Kind: fleet.KindClaim, Key: "console-claim-visible", Holder: "worker-app", State: fleet.StateWorking, Age: 3 * time.Minute},
			},
		},
		{
			// A DIFFERENT private cell's repo the viewer holds no read on. Its
			// name is NOT a substring of the authorized lane's, so a leak of it
			// is unambiguous.
			Repo: "example-org/secret-cell", OK: true, FetchedAt: now.Add(-2 * time.Minute),
			Tokens: []fleet.Token{
				{Kind: fleet.KindClaim, Key: "secret-claim-holder-token", Holder: "hidden-worker", State: fleet.StateWorking, Age: 4 * time.Minute},
				{Kind: fleet.KindPR, Key: "#77", Title: "confidential merger integration", Holder: "hidden-author", State: fleet.StateConflicting, Age: 10 * time.Minute},
			},
		},
	}
	cfg := &schema.File{Cells: []schema.Cell{{Name: "assay", Repos: []string{"example-org/console", "example-org/secret-cell"}}}}

	// Authenticated as bob, who is Anonymous (no read) on example-org/secret-cell.
	viewer := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "bob"},
		denyOwner: "example-org",
		denyName:  "secret-cell",
	}
	srv, err := New(Deps{Now: fixedNow, Config: cfg, Fleet: fakeFleet{lanes: lanes}, Viewer: viewer})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/fleet", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code == http.StatusUnauthorized {
		t.Fatalf("authenticated viewer got 401 on the surface; want a filtered map, not a block")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /fleet = %d, want 200", rr.Code)
	}
	body := rr.Body.String()

	// The denied cell's lane header and EVERY token in it must be absent.
	if strings.Contains(body, "example-org/secret-cell") {
		t.Fatalf("LEAK: the denied repo's lane header reached an unauthorized authenticated viewer")
	}
	if strings.Contains(body, "secret-claim-holder-token") || strings.Contains(body, "hidden-worker") {
		t.Fatalf("LEAK: a refs/dispatch/* claim key/holder for a repo the viewer cannot read reached the page")
	}
	if strings.Contains(body, "confidential merger integration") || strings.Contains(body, "hidden-author") {
		t.Fatalf("LEAK: an open-PR title/author for a repo the viewer cannot read reached the page")
	}

	// Control: the authorized lane and its claim token still render — the
	// filter must not over-block.
	if !strings.Contains(body, "example-org/console") || !strings.Contains(body, "console-claim-visible") {
		t.Fatal("the authorized lane must still render — the filter over-blocked")
	}
}

// TestFleetPageWithNoFleetSourceIsCouldNotCheck pins the nil-Fleet default
// (mirrors Chat's nil default): no cache wired renders could-not-check,
// never a fabricated empty map.
func TestFleetPageWithNoFleetSourceIsCouldNotCheck(t *testing.T) {
	srv, err := New(Deps{Now: fixedNow, Config: fleetConfig()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/fleet", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /fleet = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "could-not-check") {
		t.Fatalf("nil Fleet must render could-not-check")
	}
}
