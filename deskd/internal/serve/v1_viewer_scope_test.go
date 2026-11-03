package serve_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// scopeFixture is two cells, each with one repository and one open
// escalation. The viewer (denyRepoAuthz) can read example-org/console but
// not example-org/private, so every per-viewer route has something real to
// withhold.
type scopeFixture struct {
	db      *index.DB
	cfg     *schema.File
	authz   serve.SurfaceAuthorizer
	refresh *fixtureRefresher
	bus     *serve.EventBus
	h       http.Handler
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	seed := func(cell, repo, id string, n int) {
		owner, name, _ := strings.Cut(repo, "/")
		escs := []domain.Escalation{{
			ID: domain.EscalationID(id), Cell: domain.CellName(cell), Kind: domain.KindNeedsDecision,
			Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine, Title: "t " + id,
			Situation: "s", Rev: "r1", FiledAt: time.Now().Add(-time.Hour),
		}}
		raw := map[domain.EscalationID]adapter.RawIssue{
			domain.EscalationID(id): {Number: n, URL: "https://github.com/" + repo + "/issues/" + id},
		}
		if err := db.UpsertEscalations(adapter.Repo{Owner: owner, Name: name}, escs, raw); err != nil {
			t.Fatalf("UpsertEscalations: %v", err)
		}
	}
	seed("assay", "example-org/console", "1", 1)
	seed("private", "example-org/private", "2", 2)

	cfg := &schema.File{Cells: []schema.Cell{
		{Name: "assay", Repos: []string{"example-org/console"}, SilenceThreshold: "48h"},
		{Name: "private", Repos: []string{"example-org/private"}, SilenceThreshold: "48h"},
	}}
	f := &scopeFixture{db: db, cfg: cfg, authz: denyRepoAuthz{denyOwner: "example-org", denyName: "private"},
		refresh: &fixtureRefresher{}, bus: serve.NewEventBus()}
	ov := &serve.Overrides{
		Store:    local.NewOverrideStore(db),
		Answerer: operator,
		Cell: func(name string) (*schema.Cell, bool) {
			for i := range cfg.Cells {
				if cfg.Cells[i].Name == name {
					return &cfg.Cells[i], true
				}
			}
			return nil, false
		},
	}
	f.h = serve.NewV1(db, cfg, f.authz, nil, ov, nil, f.bus, f.refresh)
	return f
}

func (f *scopeFixture) do(method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

// regression: F-security-surface — GET /v1/cells (and the legacy /cells)
// listed every configured cell and its repositories to any signed-in viewer.
func TestCellsListHidesUnreadableCell(t *testing.T) {
	f := newScopeFixture(t)
	legacy := serve.New(f.db, f.cfg, f.authz)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"/v1/cells": f.do(http.MethodGet, "/v1/cells", "", ""),
		"/cells": func() *httptest.ResponseRecorder {
			r := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/cells", nil)
			req.Header.Set("Accept", "application/json")
			legacy.ServeHTTP(r, req)
			return r
		}(),
	} {
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s answered %d: %s", name, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, "example-org/private") || strings.Contains(body, `"private"`) {
			t.Errorf("GET %s disclosed a cell the viewer cannot read: %s", name, body)
		}
		if !strings.Contains(body, "example-org/console") {
			t.Errorf("GET %s dropped the viewer's own cell: %s", name, body)
		}
	}
}

// regression: F-security-surface — the override routes ran no per-cell
// check: any signed-in viewer read any cell's overrides.
func TestOverridesRefuseUnreadableCell(t *testing.T) {
	f := newScopeFixture(t)
	if rec := f.do(http.MethodGet, "/v1/cells/private/overrides", "", ""); rec.Code != http.StatusForbidden {
		t.Errorf("GET another cell's overrides answered %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(http.MethodPost, "/v1/cells/private/overrides", "application/json", `{"hold_merges":true}`); rec.Code != http.StatusForbidden {
		t.Errorf("POST another cell's overrides answered %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if ov, _, err := local.NewOverrideStore(f.db).Get("private"); err != nil || !ov.IsEmpty() {
		t.Errorf("a refused POST still wrote the table: %+v (err %v)", ov, err)
	}
	if rec := f.do(http.MethodGet, "/v1/cells/nope/overrides", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET an unconfigured cell's overrides answered %d, want 404", rec.Code)
	}
	if rec := f.do(http.MethodGet, "/v1/cells/assay/overrides", "", ""); rec.Code != http.StatusOK {
		t.Errorf("GET the viewer's own cell's overrides answered %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// regression: F-security-surface — the override POST decoded any body type,
// so a cross-site text/plain form could set overrides with no preflight.
func TestOverridesPostRequiresJSONContentType(t *testing.T) {
	f := newScopeFixture(t)
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", ""} {
		rec := f.do(http.MethodPost, "/v1/cells/assay/overrides", ct, `{"hold_merges":true}`)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("POST overrides as %q answered %d, want 415: %s", ct, rec.Code, rec.Body.String())
		}
	}
	if ov, _, err := local.NewOverrideStore(f.db).Get("assay"); err != nil || !ov.IsEmpty() {
		t.Fatalf("a non-JSON POST still wrote the table: %+v (err %v)", ov, err)
	}
	if rec := f.do(http.MethodPost, "/v1/cells/assay/overrides", "application/json; charset=utf-8", `{"hold_merges":true}`); rec.Code != http.StatusOK {
		t.Fatalf("a JSON POST answered %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

// regression: F-security-surface — the refresh nudge ran for any id, so a
// viewer could make the service re-read an item in a repository the viewer
// cannot see (and learn from the status whether it exists).
func TestRefreshRefusesUnreadableItem(t *testing.T) {
	f := newScopeFixture(t)
	rec := f.do(http.MethodPost, "/v1/escalations/2/refresh", "", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("refresh of an unreadable item answered %d, want 404", rec.Code)
	}
	if f.refresh.gotID != "" {
		t.Errorf("the Refresher ran for an unreadable item (%q)", f.refresh.gotID)
	}
	if rec := f.do(http.MethodPost, "/v1/escalations/1/refresh", "", ""); rec.Code != http.StatusOK || f.refresh.gotID != "1" {
		t.Fatalf("refresh of the viewer's own item answered %d (Refresher got %q)", rec.Code, f.refresh.gotID)
	}
}

// regression: F-security-surface — /v1/events fanned every event to every
// viewer: transitions of unreadable items, a badge counted over every
// repository, and the health of repositories the viewer cannot read.
func TestEventsAreFilteredPerViewer(t *testing.T) {
	f := newScopeFixture(t)
	f.bus.Publish(contract.EventResync, nil) // id 1: the resume point
	f.bus.Publish(contract.EventTransition, contract.TransitionData{ID: "2", To: "open"})
	f.bus.Publish(contract.EventTransition, contract.TransitionData{ID: "1", To: "open"})
	f.bus.Publish(contract.EventBadge, contract.BadgeData{Open: 2})
	f.bus.Publish(contract.EventIndexHealth, contract.IndexHealthData{CouldNotCheck: []string{"example-org/console", "example-org/private"}})
	f.bus.Publish(contract.EventType("unknown.kind"), map[string]string{"repo": "example-org/private"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil).WithContext(ctx)
	req.Header.Set("Last-Event-ID", "1")
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/events answered %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "example-org/private") {
		t.Errorf("the stream named an unreadable repository: %s", rec.Body.String())
	}

	stream := contract.NewEventStream(rec.Body, nil)
	var got []contract.Event
	for {
		e, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		got = append(got, e)
	}
	if len(got) != 3 {
		t.Fatalf("got %d events, want 3 (own transition, own badge, own health): %+v", len(got), got)
	}
	var tr contract.TransitionData
	if got[0].Type != contract.EventTransition || json.Unmarshal(got[0].Data, &tr) != nil || tr.ID != "1" {
		t.Errorf("first event = %s %s, want the viewer's own transition", got[0].Type, got[0].Data)
	}
	var b contract.BadgeData
	if got[1].Type != contract.EventBadge || json.Unmarshal(got[1].Data, &b) != nil || b.Open != 1 {
		t.Errorf("badge = %s, want a count over the viewer's readable rows (open 1)", got[1].Data)
	}
	var hd contract.IndexHealthData
	if got[2].Type != contract.EventIndexHealth || json.Unmarshal(got[2].Data, &hd) != nil ||
		len(hd.CouldNotCheck) != 1 || hd.CouldNotCheck[0] != "example-org/console" {
		t.Errorf("health = %s, want only the viewer's readable repository", got[2].Data)
	}
}
