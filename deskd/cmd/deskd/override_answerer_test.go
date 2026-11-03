package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// overridePost drives the REAL cell-override POST (serve.Overrides.handlePost)
// with the REAL tier gate cmd/deskd wires (overrideAnswerer), for a viewer whose
// authenticated session login is sessLogin.
func overridePost(t *testing.T, sessLogin string) *httptest.ResponseRecorder {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "ov.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// No per-cell answer_as: the effective answerer is the startup principal.
	cfg := &schema.File{Cells: []schema.Cell{{Name: "alpha"}}}
	store := identity.NewSessionStore(nil)
	sess, err := store.Begin(sessLogin, "fixture-session-credential")
	if err != nil {
		t.Fatalf("begin session: %v", err)
	}
	ov := &serve.Overrides{
		Store:    local.NewOverrideStore(db),
		Answerer: overrideAnswerer(store, cfg),
		Cell:     func(name string) (*schema.Cell, bool) { return &cfg.Cells[0], name == "alpha" },
	}
	h := serve.NewV1(db, cfg, nil, nil, ov, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/cells/alpha/overrides", strings.NewReader(`{"hold_merges":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: identity.SessionCookie, Value: sess.ID})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// regression: F-human-login-case — the override gate compared the session login
// with the effective answer_as case-sensitively, so the startup principal signed
// in under a different letter case was granted answerer by the capability ladder
// (EqualFold) but refused 403 here.
func TestOverrideAnswererIsCaseInsensitive(t *testing.T) {
	want := humanid.Login()
	if want == "" {
		t.Skip("no startup principal configured for this test process")
	}
	mixed := strings.ToUpper(want[:1]) + want[1:]
	if mixed == want {
		mixed = strings.ToUpper(want)
	}
	if mixed == want {
		t.Fatalf("fixture principal %q has no letter whose case can change", want)
	}
	if rec := overridePost(t, mixed); rec.Code != http.StatusOK {
		t.Fatalf("POST as %q (startup principal %q) → HTTP %d, want 200; body: %s", mixed, want, rec.Code, rec.Body.String())
	}
	// Planted second instance of the class: the ladder's own answerer decision
	// for the same viewer must agree with the override gate.
	if !humanid.Matches(mixed) {
		t.Fatalf("humanid.Matches(%q) = false; the principal comparison must be case-insensitive", mixed)
	}
}

// The negative twin: a different login stays a 403, whatever its case.
func TestOverrideAnswererRefusesOtherLogin(t *testing.T) {
	if humanid.Login() == "" {
		t.Skip("no startup principal configured for this test process")
	}
	if rec := overridePost(t, "Someone-Else"); rec.Code != http.StatusForbidden {
		t.Fatalf("POST as an unrelated login → HTTP %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}
