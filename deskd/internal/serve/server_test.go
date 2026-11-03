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
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// denyRepoAuthz authenticates the request but DENIES a named repo on the surface
// authorizer — the shape of an authenticated viewer with no read capability on a
// cell's private repo.
type denyRepoAuthz struct{ denyOwner, denyName string }

func (a denyRepoAuthz) SurfaceAuthorizer(*http.Request) func(owner, name string) bool {
	return func(owner, name string) bool {
		return !(owner == a.denyOwner && name == a.denyName)
	}
}

// TestInboxJSONHidesRowsWithoutCapability (the capability model security fix, Security-Review
// 5091037270): the JSON read surface's inbox listing must filter per-repo. An
// authenticated viewer with no read capability on a cell's private repo must NOT
// receive that repo's rows. Pre-fix the listing served every row (gated on login
// presence alone in the middleware, no per-repo backstop on the data path).
func TestInboxJSONHidesRowsWithoutCapability(t *testing.T) {
	db, err := index.Open(filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := adapter.Repo{Owner: "example-org", Name: "console"} // PRIVATE
	escs := []domain.Escalation{{
		ID: "IT1", Cell: "assay", Kind: domain.KindNeedsDecision, Provenance: domain.AgentRaised,
		Urgency: domain.UrgencyRoutine, Title: "a withheld private decision",
		Situation: "a real decision", Rev: "r1", FiledAt: time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{
		"IT1": {Number: 11, URL: "https://github.com/example-org/console/issues/11"},
	}
	if err := db.UpsertEscalations(repo, escs, raw); err != nil {
		t.Fatalf("UpsertEscalations: %v", err)
	}
	cfg := &schema.File{Cells: []schema.Cell{{Name: "assay", Repos: []string{"example-org/console"}, SilenceThreshold: "48h"}}}

	// Denied viewer: the row's repo is filtered out.
	h := serve.New(db, cfg, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	body := rec.Body.String()
	if strings.Contains(body, "a withheld private decision") || strings.Contains(body, "console/issues/11") {
		t.Fatal("LEAK: the JSON inbox served a private row to a viewer with no read capability on its repo")
	}

	// Authorized viewer: the row is served.
	h2 := serve.New(db, cfg, denyRepoAuthz{denyOwner: "nobody", denyName: "nowhere"})
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/inbox", nil))
	if !strings.Contains(rec2.Body.String(), "a withheld private decision") {
		t.Fatal("an authorized viewer must still receive the row — the filter over-blocked")
	}
}

func newServer(t *testing.T) http.Handler {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := &schema.File{Cells: []schema.Cell{{
		Name: "assay", Repos: []string{"owner/alpha"}, StreamsRoot: "docs/streams",
		Loops:            []schema.Loop{{Role: "review", Schedule: "*/10 * * * *", Models: []string{"m"}, App: "a-review-app"}},
		SilenceThreshold: "48h",
	}}}
	return serve.New(db, cfg, nil)
}

// TestSurfaceIsReadOnly is the HTTP half of the structural no-write proof.
//
// It walks the ONE route table and asserts that every registered path
// refuses every write verb. This is a positive control per route × per verb:
// each case must come back 405. A future endpoint that accepted a POST would
// fail here without anyone having to remember to extend the test, because
// the table it iterates is the same table the server registers from.
func TestSurfaceIsReadOnly(t *testing.T) {
	h := newServer(t)
	writeVerbs := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	if len(serve.Routes) == 0 {
		t.Fatal("could-not-check: the route table is empty, so this test proved nothing")
	}
	for _, route := range serve.Routes {
		for _, verb := range writeVerbs {
			t.Run(verb+" "+route.Path, func(t *testing.T) {
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, httptest.NewRequest(verb, route.Path, nil))
				if rec.Code != http.StatusMethodNotAllowed {
					t.Fatalf("POSITIVE CONTROL SURVIVED: %s %s answered %d, not 405. "+
						"This build must have no write path of any kind — the human gate is the service contract.",
						verb, route.Path, rec.Code)
				}
				if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
					t.Errorf("Allow header = %q, want \"GET, HEAD\"", got)
				}
			})
		}
	}
}

// TestReadsAreServed is the counterweight: a guard that also blocks GET
// would be removed, and a removed guard blocks nothing.
func TestReadsAreServed(t *testing.T) {
	h := newServer(t)
	for _, route := range serve.Routes {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route.Path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s answered %d", route.Path, rec.Code)
		}
	}
}

// TestEveryResponseCarriesItsStaleness — design §10: every view carries
// "as of <t>"; staleness is displayed, not hidden. On a COLD index the age
// must serialise as null with a could-not-check beside it, never as 0.
func TestEveryResponseCarriesItsStaleness(t *testing.T) {
	h := newServer(t)
	for _, route := range serve.Routes {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, route.Path, nil))

		var env map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: response is not JSON: %v", route.Path, err)
		}
		age, present := env["index_age_seconds"]
		if !present {
			t.Errorf("%s: no index_age_seconds key at all", route.Path)
			continue
		}
		if age != nil {
			t.Errorf("%s: a cold index reported an age of %v; it has never polled, so its age is unknown", route.Path, age)
		}
		conf, _ := env["index_age_confidence"].(map[string]any)
		if conf["confidence"] != "could-not-check" {
			t.Errorf("%s: cold index age confidence = %v, want could-not-check", route.Path, conf["confidence"])
		}
		if _, ok := env["sources_not_read"]; !ok {
			t.Errorf("%s: the response does not disclose what this index does NOT read", route.Path)
		}
	}
}

// TestRoadmapSkeletonIsCouldNotCheckNotEmpty — the roadmap route exists in
// this build; the derivation does not. Serving an empty stream array would
// assert there is no work. It serves an explicit could-not-check naming
// `statusgen --json` as the single declared source it will delegate to.
func TestRoadmapSkeletonIsCouldNotCheckNotEmpty(t *testing.T) {
	h := newServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/roadmap", nil))

	var env struct {
		Data struct {
			Streams    []any `json:"streams"`
			Assessment struct {
				Confidence string `json:"confidence"`
				Reason     string `json:"reason"`
				Probe      struct {
					Source string `json:"source"`
					Method string `json:"method"`
				} `json:"probe"`
			} `json:"assessment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Streams != nil {
		t.Error("the skeleton served a stream array; an empty array reads as \"no work\"")
	}
	if env.Data.Assessment.Confidence != "could-not-check" {
		t.Errorf("roadmap assessment = %q, want could-not-check", env.Data.Assessment.Confidence)
	}
	if env.Data.Assessment.Probe.Method != "delegated-to-generator" {
		t.Errorf("the skeleton does not declare a delegated source: %+v", env.Data.Assessment.Probe)
	}
	if env.Data.Assessment.Probe.Source == "" {
		t.Error("the delegated source is unnamed, so nobody can check it later")
	}
}

// TestInboxCountsCarryTheirProbe — no silent counts. Every number on the
// inbox surface arrives with the probe that produced it.
func TestInboxCountsCarryTheirProbe(t *testing.T) {
	h := newServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inbox?cell=assay", nil))

	var env struct {
		Data struct {
			Total struct {
				Value *int `json:"value"`
				Probe struct {
					Source string `json:"source"`
					Method string `json:"method"`
					Window string `json:"window"`
					Limit  int    `json:"limit"`
					Capped bool   `json:"capped"`
				} `json:"probe"`
			} `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	p := env.Data.Total.Probe
	if p.Source == "" || p.Method == "" || p.Window == "" {
		t.Fatalf("the inbox total was published without a full probe: %+v", p)
	}
	if _, ok := json.Marshal(env); ok != nil {
		t.Fatal(ok)
	}
}
