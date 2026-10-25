package index_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// fixtureAPI is a stand-in GitHub that serves recorded payload SHAPES over
// the real HTTP surface.
//
// It exists because the interesting failures are in the wire handling, not
// in the mapping: the label parameter's conjunction semantics, the 304 path,
// the Link walk, the aliased-response decode. A test double at the Fetcher
// interface skips all four. This one is exercised through
// adapter.NewGitHub — same client, same read-only transport, same decode —
// so those four are actually covered.
type fixtureAPI struct {
	listCalls  atomic.Int32
	graphCalls atomic.Int32
	// serveNotModified makes every list page answer 304, to exercise the
	// conditional-request path.
	serveNotModified atomic.Bool
	// issues is the corpus, keyed by number.
	issues map[int]fixtureIssue
}

type fixtureIssue struct {
	Number int
	Title  string
	Body   string
	Labels []string
	Author string
	Filed  time.Time
}

// typedV2Body is the spec §1.4 grammar. Note the U+2014 em dashes: this is
// the one fixture that proves the typed path end to end, and an ASCII hyphen
// here would make it silently prove the malformed path instead.
const typedV2Body = "Situation: the index is complete for escalations and not for the roadmap.\n\n" +
	"### Options\n" +
	"<!-- assay:options v2 -->\n" +
	"#### `ship-partial` — Ship the partial index\n" +
	"Consequence: inbox live today; roadmap pane lags a week.\n" +
	"Pros: the queue becomes visible now.\n\n" +
	"#### `wait-for-fix` — Hold until the roadmap derivation lands\n" +
	"Consequence: nothing ships this week; no rework risk.\n"

func newFixtureAPI() *fixtureAPI {
	base := time.Now().UTC().Add(-72 * time.Hour)
	return &fixtureAPI{issues: map[int]fixtureIssue{
		// Carries BOTH needs-decision and question, so a conjunctive
		// `labels=a,b` query would find it and an OR-union must not
		// double-count it.
		418: {418, "URGENT: reports overclaim", typedV2Body,
			[]string{"needs-decision", "question"}, "a-review-app", base},
		367: {367, "governance gates: three candidate designs", "Situation: a fork, in prose.",
			[]string{"needs-decision"}, "a-verify-app", base.Add(time.Hour)},
		504: {504, "help with a capability edge", "Situation: the loop cannot proceed.",
			[]string{"help wanted"}, "nobody-configured-app", base.Add(2 * time.Hour)},
	}}
}

func (f *fixtureAPI) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		f.listCalls.Add(1)
		w.Header().Set("ETag", `W/"fixture"`)
		if f.serveNotModified.Load() {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		label := r.URL.Query().Get("labels")
		var out []map[string]any
		for _, iss := range f.issues {
			for _, l := range iss.Labels {
				if l == label {
					out = append(out, map[string]any{"number": iss.Number})
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		f.graphCalls.Add(1)
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		repoObj := map[string]any{}
		// The document names each issue as `aN: issue(number: X)`. Parse the
		// aliases back out so the response mirrors the request exactly the
		// way GitHub's would.
		for _, line := range strings.Split(body.Query, "\n") {
			var alias, num int
			if _, err := fmt.Sscanf(strings.TrimSpace(line), "a%d: issue(number: %d)", &alias, &num); err != nil {
				continue
			}
			iss, ok := f.issues[num]
			if !ok {
				repoObj[fmt.Sprintf("a%d", alias)] = nil
				continue
			}
			labels := []map[string]string{}
			for _, l := range iss.Labels {
				labels = append(labels, map[string]string{"name": l})
			}
			repoObj[fmt.Sprintf("a%d", alias)] = map[string]any{
				"number": iss.Number, "title": iss.Title, "body": iss.Body,
				"createdAt": iss.Filed.Format(time.RFC3339), "updatedAt": iss.Filed.Format(time.RFC3339),
				"closedAt": nil, "state": "OPEN",
				"url":    fmt.Sprintf("https://example.invalid/issues/%d", iss.Number),
				"author": map[string]string{"login": iss.Author, "__typename": "Bot"},
				"labels": map[string]any{"nodes": labels},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"repository": repoObj}})
	})

	return mux
}

// TestEndToEndThroughTheRealClient walks a whole cycle — conditional list,
// aliased derivation, mapping, storage, serving — through the shipped HTTP
// client rather than a Fetcher stub.
//
// It also produces the JSON shape the read surface's Verify rows query with jq, so
// the contract those rows assert is pinned by a test rather than by a
// one-off manual run.
func TestEndToEndThroughTheRealClient(t *testing.T) {
	api := newFixtureAPI()
	srv := httptest.NewServer(api.handler())
	defer srv.Close()

	db, _ := openTemp(t)
	cfg := testConfig()
	p := &index.Poller{
		DB:      db,
		Fetcher: adapter.NewGitHub(srv.URL, "read-scoped-placeholder", nil),
		Config:  cfg,
		Dir:     index.NewDirectory(cfg),
	}

	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle was not clean: %s (%+v)", res.Outcome, res.Health)
	}

	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("indexed %d rows, want 3 — a conjunctive label query returns 0 or 1 here, which is exactly the "+
			"confident-empty answer this index must never give", len(rows))
	}

	// --- the union must not duplicate a multi-labelled issue -------------
	seen := map[domain.EscalationID]bool{}
	for _, r := range rows {
		if seen[r.ID] {
			t.Fatalf("issue indexed twice: %s — the per-label union does not deduplicate", r.Title)
		}
		seen[r.ID] = true
	}

	// --- the read surface Verify 4: options carry id/label/consequence -----------
	var typed *index.Row
	for i := range rows {
		if rows[i].Options != nil {
			typed = &rows[i]
			break
		}
	}
	if typed == nil {
		t.Fatal("no typed-v2 row survived the round trip")
	}
	if len(typed.Options) != 2 {
		t.Fatalf("typed row has %d options, want 2", len(typed.Options))
	}
	for _, o := range typed.Options {
		if o.ID == "" || o.Label == "" || o.Consequence == "" {
			t.Errorf("option %+v is missing a field the peek panel binds to", o)
		}
	}
	if typed.Rev == "" || typed.Rev != domain.OptionsRev(typed.Options) {
		t.Error("the stored OptionsRev does not re-derive from the stored options")
	}

	// --- the read surface Verify 5: provenance is agent-raised (ruling D4) -------
	for _, r := range rows {
		if r.Provenance != domain.AgentRaised {
			t.Errorf("row %q derived %q; nothing in this build produces the other value (D4)", r.Title, r.Provenance)
		}
	}

	// --- kind precedence on the dual-labelled issue ----------------------
	for _, r := range rows {
		if strings.HasPrefix(r.Title, "URGENT") {
			if r.Kind != domain.KindNeedsDecision {
				t.Errorf("a needs-decision+question issue derived kind %q — the human-actionable label must win", r.Kind)
			}
			if r.Urgency != domain.UrgencyUrgent {
				t.Errorf("an URGENT-titled issue derived urgency %s", r.Urgency)
			}
		}
	}

	// --- the unjoinable row is served, unresolved, and unkeyed -----------
	var unresolved int
	for _, r := range rows {
		if !r.Attributable() {
			unresolved++
			if r.Fields["automation"].Confidence != domain.CouldNotCheck {
				t.Errorf("an unattributable row rendered %s", r.Fields["automation"].Confidence)
			}
			if r.Key().String() != "" {
				t.Error("an unattributable row was given a collapse key")
			}
		}
	}
	if unresolved != 1 {
		t.Fatalf("%d unresolved rows, want 1 (the fixture files one from an unconfigured identity)", unresolved)
	}

	// --- the 304 path: a second cycle must cost no derivation ------------
	graphBefore := api.graphCalls.Load()
	api.serveNotModified.Store(true)
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("the 304 cycle was not clean: %s", res.Outcome)
	}
	if got := api.graphCalls.Load(); got != graphBefore {
		t.Errorf("a fully-304 cycle still issued %d GraphQL call(s) — conditional requests are the reason a 60s "+
			"interval fits inside the rate budget", got-graphBefore)
	}
	after, _ := db.Inbox("assay", false)
	if len(after) != len(rows) {
		t.Fatalf("a 304 cycle changed the row count: %d → %d", len(rows), len(after))
	}

	// --- and the served envelope carries what a consumer needs ----------
	h := serve.New(db, cfg, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inbox?cell=assay", nil))

	var env struct {
		IndexAgeSeconds *int64 `json:"index_age_seconds"`
		Data            struct {
			Items []struct {
				Provenance string `json:"provenance"`
				Automation *struct {
					Cell string `json:"cell"`
					Role string `json:"role"`
				} `json:"automation"`
				Options []struct {
					ID          string `json:"id"`
					Label       string `json:"label"`
					Consequence string `json:"consequence"`
				} `json:"options"`
				Links map[string]string `json:"links"`
			} `json:"items"`
			Total struct {
				Value *int `json:"value"`
			} `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.IndexAgeSeconds == nil {
		t.Error("a polled index still reported an unknown age")
	}
	if env.Data.Total.Value == nil || *env.Data.Total.Value != 3 {
		t.Errorf("served total = %v, want 3", env.Data.Total.Value)
	}
	var nullAutomation, withOptions int
	for _, it := range env.Data.Items {
		if it.Provenance != "agent-raised" {
			t.Errorf("served provenance = %q", it.Provenance)
		}
		if it.Automation == nil {
			nullAutomation++
		}
		if len(it.Options) > 0 {
			withOptions++
			for _, o := range it.Options {
				if o.ID == "" || o.Label == "" || o.Consequence == "" {
					t.Errorf("served option %+v is missing a field", o)
				}
			}
		}
		if it.Links["platform"] == "" {
			t.Error("a served row has no platform deep-link; it is presentation data and must be attached at this edge")
		}
	}
	if nullAutomation != 1 {
		t.Errorf("%d rows served automation:null, want 1", nullAutomation)
	}
	if withOptions != 1 {
		t.Errorf("%d rows served typed options, want 1", withOptions)
	}
}

// TestClientRefusesToWriteEvenThroughTheRealServer closes the loop on the
// structural read-only claim at the integration level: the client wired by
// the poller cannot be made to mutate the fixture API.
func TestClientRefusesToWriteEvenThroughTheRealServer(t *testing.T) {
	var mutated atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			mutated.Store(true)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := &http.Client{Transport: adapter.NewReadOnlyTransport(nil)}
	for _, m := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		req, _ := http.NewRequest(m, srv.URL+"/repos/o/r/issues/1", strings.NewReader(`{"state":"closed"}`))
		if _, err := c.Do(req); err == nil {
			t.Errorf("%s was performed against the server", m)
		}
	}
	if mutated.Load() {
		t.Fatal("POSITIVE CONTROL SURVIVED: a write reached the server")
	}
	_ = filepath.Separator // keep the import set honest across edits
}
