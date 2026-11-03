package serve_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// decode unmarshals a recorded JSON response body into out.
func decode(t *testing.T, rec *httptest.ResponseRecorder, out any) error {
	t.Helper()
	return json.Unmarshal(rec.Body.Bytes(), out)
}

// v1Fixture builds a /v1 read handler over an index seeded with one open
// escalation in a named repo, and returns the handler plus that escalation's id.
func v1Fixture(t *testing.T, authz serve.SurfaceAuthorizer) (http.Handler, string) {
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
	return serve.NewV1(db, cfg, authz, nil, nil, nil, nil, nil), "IT1"
}

// TestV1ReadsCarryStalenessAndETag — the v1 contract task 2: every /v1 read carries
// the contract staleness envelope (index_age_seconds present, null on a cold
// index) in the body AND an ETag in the header. Design §10: staleness is
// displayed, not hidden.
func TestV1ReadsCarryStalenessAndETag(t *testing.T) {
	h, id := v1Fixture(t, nil)
	for _, path := range []string{
		"/v1/inbox", "/v1/cells", "/v1/escalations/" + id,
		"/v1/roadmap", "/v1/portfolio", "/v1/metrics",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s answered %d, want 200", path, rec.Code)
			continue
		}
		if rec.Header().Get("ETag") == "" {
			t.Errorf("GET %s carries no ETag header", path)
		}
		// index_age_seconds key must be present (null on a cold index — never
		// absent, which would let a consumer infer freshness from silence).
		if !strings.Contains(rec.Body.String(), `"index_age_seconds"`) {
			t.Errorf("GET %s does not disclose index_age_seconds: %s", path, rec.Body.String())
		}
	}
}

// TestV1ConditionalGetReturns304 — an ETag is only useful if it validates: a
// repeated read of the same index generation with If-None-Match gets 304, not
// the body again.
func TestV1ConditionalGetReturns304(t *testing.T) {
	h, _ := v1Fixture(t, nil)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("could-not-check: first read returned no ETag")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match with the current ETag answered %d, want 304", rec2.Code)
	}
	if body := strings.TrimSpace(rec2.Body.String()); body != "" {
		t.Errorf("a 304 must carry no body; got %q", body)
	}

	// A non-matching validator must still return the body.
	req3 := httptest.NewRequest(http.MethodGet, "/v1/inbox", nil)
	req3.Header.Set("If-None-Match", `"stale-tag"`)
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("a stale If-None-Match answered %d, want 200 with the body", rec3.Code)
	}
}

// TestV1InboxDecodesAsContractType — the served /v1/inbox body must decode into
// the contract type the client (inc-1) reads, so the seam a diff can break is
// actually exercised end to end.
func TestV1InboxDecodesAsContractType(t *testing.T) {
	h, _ := v1Fixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/inbox?cell=assay", nil))

	var resp contract.InboxResponse
	if err := decode(t, rec, &resp); err != nil {
		t.Fatalf("body does not decode into contract.InboxResponse: %v", err)
	}
	if resp.Cell != "assay" {
		t.Errorf("cell = %q, want assay", resp.Cell)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("want exactly 1 item, got total=%d items=%d", resp.Total, len(resp.Items))
	}
	if resp.Items[0].Title != "a v1 decision" {
		t.Errorf("item title = %q", resp.Items[0].Title)
	}
}

// TestV1EscalationItemView — GET /v1/escalations/{id} returns the item; an OPEN
// escalation has a nil Outcome (an answered one is closed and out of window —
// deriving a closed item's outcome from the answer-record marker is the adapter
// increment, not this one).
func TestV1EscalationItemView(t *testing.T) {
	h, id := v1Fixture(t, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/escalations/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET item answered %d, want 200", rec.Code)
	}
	var resp contract.EscalationResponse
	if err := decode(t, rec, &resp); err != nil {
		t.Fatalf("body does not decode into contract.EscalationResponse: %v", err)
	}
	if string(resp.Item.ID) != id {
		t.Errorf("item id = %q, want %q", resp.Item.ID, id)
	}
	if resp.Outcome != nil {
		t.Errorf("an open escalation must have a nil Outcome; got %+v", resp.Outcome)
	}
}

// TestV1EscalationUnknownAndSubRoutes — an unknown id, the bare collection, and
// a GET on a (not-yet-implemented) write sub-route are each 404: only
// GET /v1/escalations/{id} is a read route on this build.
func TestV1EscalationUnknownAndSubRoutes(t *testing.T) {
	h, id := v1Fixture(t, nil)
	for _, path := range []string{
		"/v1/escalations/does-not-exist",
		"/v1/escalations/",
		"/v1/escalations/" + id + "/read",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s answered %d, want 404", path, rec.Code)
		}
	}
}

// TestV1InboxFiltersByCapability — the private-repo gate applies to the versioned
// listing exactly as it does to the un-versioned one: a viewer with no read
// capability on the row's repo does not receive it.
func TestV1InboxFiltersByCapability(t *testing.T) {
	// Denied viewer: the row is filtered out of the listing AND the item view.
	h, id := v1Fixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	if strings.Contains(rec.Body.String(), "a v1 decision") || strings.Contains(rec.Body.String(), "issues/11") {
		t.Fatal("LEAK: /v1/inbox served a private row to a viewer with no read capability on its repo")
	}
	recItem := httptest.NewRecorder()
	h.ServeHTTP(recItem, httptest.NewRequest(http.MethodGet, "/v1/escalations/"+id, nil))
	if recItem.Code != http.StatusNotFound {
		t.Fatalf("LEAK: /v1/escalations/{id} served a private item (answered %d, want 404)", recItem.Code)
	}

	// Authorized viewer: the row is served.
	h2, _ := v1Fixture(t, denyRepoAuthz{denyOwner: "nobody", denyName: "nowhere"})
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	if !strings.Contains(rec2.Body.String(), "a v1 decision") {
		t.Fatal("an authorized viewer must still receive the row — the filter over-blocked")
	}
}

// TestV1MetricsFiltersByCapability — the metrics aggregate is computed over ONLY
// the rows the viewer may read, exactly as handleV1Inbox filters its listing. A
// viewer denied the seeded row's repo sees a ZEROED aggregate (no
// open_escalations, an empty urgency histogram, no ages); an authorized viewer
// sees the real count and distribution. Without the per-repo filter in
// handleV1Metrics the denied viewer receives the escalation count + urgency
// histogram of a repo it cannot read — the authorization/exposure class
// Security-Review 5091037270 closed on the inbox listing.
func TestV1MetricsFiltersByCapability(t *testing.T) {
	metricValue := func(h http.Handler, metric string) any {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/metrics?metric="+metric, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("/v1/metrics?metric=%s answered %d, want 200: %s", metric, rec.Code, rec.Body.String())
		}
		var out struct {
			Value any `json:"value"`
		}
		if err := decode(t, rec, &out); err != nil {
			t.Fatalf("decode metrics %s: %v", metric, err)
		}
		return out.Value
	}

	// Denied viewer: the one seeded row is filtered out of every aggregate.
	hDeny, _ := v1Fixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	if v := metricValue(hDeny, "open_escalations"); v != float64(0) {
		t.Fatalf("LEAK: denied viewer saw open_escalations=%v, want 0", v)
	}
	if v := metricValue(hDeny, "unresolved_rows"); v != float64(0) {
		t.Fatalf("LEAK: denied viewer saw unresolved_rows=%v, want 0", v)
	}
	if v := metricValue(hDeny, "oldest_age_seconds"); v != float64(0) {
		t.Fatalf("LEAK: denied viewer saw oldest_age_seconds=%v, want 0", v)
	}
	if hist, ok := metricValue(hDeny, "by_urgency").(map[string]any); !ok || len(hist) != 0 {
		t.Fatalf("LEAK: denied viewer saw by_urgency=%v, want an empty histogram", metricValue(hDeny, "by_urgency"))
	}

	// Authorized viewer: the aggregate reflects the real row.
	hAllow, _ := v1Fixture(t, denyRepoAuthz{denyOwner: "nobody", denyName: "nowhere"})
	if v := metricValue(hAllow, "open_escalations"); v != float64(1) {
		t.Fatalf("authorized viewer saw open_escalations=%v, want 1 (the filter over-blocked)", v)
	}
	if v := metricValue(hAllow, "oldest_age_seconds"); v == float64(0) {
		t.Fatal("authorized viewer saw oldest_age_seconds=0, want the seeded row's age (the filter over-blocked)")
	}
	if hist, ok := metricValue(hAllow, "by_urgency").(map[string]any); !ok || len(hist) == 0 {
		t.Fatalf("authorized viewer saw by_urgency=%v, want the seeded urgency bucket (the filter over-blocked)", metricValue(hAllow, "by_urgency"))
	}
}

// TestV1WriteVerbsAreRejected — the READ half of the versioned surface stays
// structurally read-only after the service contract increment 2 added the two
// named-action writes. Three distinct answers, and the difference between them
// is the property:
//
//   - any write verb on a READ route is 405: the route exists and does not
//     write, exactly as increment 1 left it;
//   - PUT / PATCH / DELETE anywhere is 405: the contract has no such verb;
//   - POST to a declared named action on a fixture with no NamedActions wired
//     (this one — the read-only deployment shape) is 404, because the route is
//     not mounted at all. A route that exists but declines would be the "gap
//     between a flag and a grant" this surface refuses.
func TestV1WriteVerbsAreRejected(t *testing.T) {
	h, id := v1Fixture(t, nil)
	for _, verb := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(verb, "/v1/inbox", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /v1/inbox answered %d, want 405 (a read route never writes)", verb, rec.Code)
		}
	}
	for _, path := range []string{"/v1/escalations/" + id + "/answer", "/v1/escalations/" + id + "/ack"} {
		for _, verb := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(verb, path, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s answered %d, want 405 (the contract declares no such verb)", verb, path, rec.Code)
			}
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("POST %s answered %d, want 404 — with no named actions wired the write route is not mounted", path, rec.Code)
		}
	}
}

// TestV1DelegatedRoutesAreCouldNotCheck — roadmap and portfolio derive nothing
// in this build; they must serve an explicit could-not-check naming a delegated
// source, never an empty list a consumer would read as "no work" (§10).
func TestV1DelegatedRoutesAreCouldNotCheck(t *testing.T) {
	h, _ := v1Fixture(t, nil)
	for _, path := range []string{"/v1/roadmap", "/v1/portfolio"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		var resp struct {
			Assessment struct {
				Confidence string `json:"confidence"`
				Probe      struct {
					Source string `json:"source"`
					Method string `json:"method"`
				} `json:"probe"`
			} `json:"assessment"`
		}
		if err := decode(t, rec, &resp); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if resp.Assessment.Confidence != "could-not-check" {
			t.Errorf("%s confidence = %q, want could-not-check", path, resp.Assessment.Confidence)
		}
		if resp.Assessment.Probe.Method != "delegated-to-generator" || resp.Assessment.Probe.Source == "" {
			t.Errorf("%s does not name a delegated source: %+v", path, resp.Assessment.Probe)
		}
	}
}

// TestUnversionedSurfaceCarriesDeprecation — the un-versioned paths keep serving
// for one release (the service contract) but disclose the deprecation on every
// response so no reader is silently on a soon-to-be-removed surface.
func TestUnversionedSurfaceCarriesDeprecation(t *testing.T) {
	h := newServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if rec.Header().Get("Deprecation") != "true" {
		t.Errorf("the un-versioned /inbox carries no Deprecation header")
	}
	if link := rec.Header().Get("Link"); !strings.Contains(link, "successor-version") {
		t.Errorf("the un-versioned /inbox does not point to its successor version: %q", link)
	}
}
