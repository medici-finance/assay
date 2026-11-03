package serve_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
	"github.com/medici-finance/assay/deskd/internal/mcp"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

func ovIndex(t *testing.T) *index.DB {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "ov.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func ovCfg() *schema.File {
	return &schema.File{Cells: []schema.Cell{{Name: "assay"}}}
}

// recordFilerStub records whether FileRecordPR was called. With identity ""
// (unconfigured) the handler must NEVER call it — a call is the "silent fallback
// to another identity" the negative path forbids.
type recordFilerStub struct {
	t        *testing.T
	identity string
	called   bool
}

func (s *recordFilerStub) RecordIdentity() string { return s.identity }
func (s *recordFilerStub) FileRecordPR(context.Context, mcp.RecordPRRequest) (mcp.RecordPRResult, error) {
	s.called = true
	if s.identity == "" {
		s.t.Fatalf("FileRecordPR was called with an UNCONFIGURED write-issues App — the filer must never fall back to another identity")
	}
	return mcp.RecordPRResult{Status: "record PR filed (draft)"}, nil
}

func overridesHandler(t *testing.T, db *index.DB, filer mcp.RecordFiler, answerer func(*http.Request, string) (string, bool)) http.Handler {
	cfg := ovCfg()
	ov := &serve.Overrides{
		Store:      local.NewOverrideStore(db),
		Filer:      filer,
		Answerer:   answerer,
		Cell:       func(name string) (*schema.Cell, bool) { return &cfg.Cells[0], name == "assay" },
		ConfigRepo: "example-org/console",
		ConfigPath: "config/cells.yaml",
	}
	return serve.NewV1(db, cfg, nil, nil, ov, nil, nil, nil)
}

func operator(*http.Request, string) (string, bool) { return "operator", true }

func postOverride(h http.Handler, cell, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/cells/"+cell+"/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestOverridesFilerUnconfiguredIsCouldNotCheck (row 7): with no write-issues
// App configured, a valid POST still writes the table, files NO PR, reports
// record_pr: could-not-check, and uses NO fallback identity.
func TestOverridesFilerUnconfiguredIsCouldNotCheck(t *testing.T) {
	db := ovIndex(t)
	stub := &recordFilerStub{t: t, identity: ""}
	h := overridesHandler(t, db, stub, operator)

	rec := postOverride(h, "assay", `{"dispatch_width":1,"hold_merges":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST → HTTP %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp contract.OverridesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.DispatchWidth == nil || *resp.DispatchWidth != 1 || resp.HoldMerges == nil || !*resp.HoldMerges {
		t.Fatalf("the write did not echo: %+v", resp)
	}
	if !strings.Contains(resp.RecordPR, "could-not-check") {
		t.Fatalf("record_pr must be could-not-check when the write-issues App is unconfigured, got %q", resp.RecordPR)
	}
	if stub.called {
		t.Fatalf("the filer was called despite being unconfigured — no fallback identity is allowed")
	}

	// The table WAS written locally (the knob exists even when the PR cannot be filed).
	got, _, err := local.NewOverrideStore(db).Get("assay")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DispatchWidth == nil || *got.DispatchWidth != 1 {
		t.Fatalf("the local table must be written even when the record PR is could-not-check, got %+v", got)
	}
}

// TestOverridesPostRequiresAnswerer (the tier leg of row 4 / the pre-mortem's
// "Commenter can set overrides" row): no identity → 401; an authenticated
// non-answerer → 403; the answerer → 200.
func TestOverridesPostRequiresAnswerer(t *testing.T) {
	db := ovIndex(t)
	stub := &recordFilerStub{t: t, identity: ""}

	// No identity → 401.
	noOne := func(*http.Request, string) (string, bool) { return "", false }
	if rec := postOverride(overridesHandler(t, db, stub, noOne), "assay", `{"dispatch_width":1}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no identity → HTTP %d, want 401; body: %s", rec.Code, rec.Body.String())
	}

	// Authenticated commenter (not the answerer) → 403.
	commenter := func(*http.Request, string) (string, bool) { return "bob", false }
	if rec := postOverride(overridesHandler(t, db, stub, commenter), "assay", `{"dispatch_width":1}`); rec.Code != http.StatusForbidden {
		t.Fatalf("commenter → HTTP %d, want 403; body: %s", rec.Code, rec.Body.String())
	}

	// The answerer → 200.
	if rec := postOverride(overridesHandler(t, db, stub, operator), "assay", `{"dispatch_width":1}`); rec.Code != http.StatusOK {
		t.Fatalf("answerer → HTTP %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

// TestOverridesPostRejectsOutOfRange (the validator leg of row 4): an
// out-of-range value is refused by the SAME validator the schema uses, and the
// error names the offending field.
func TestOverridesPostRejectsOutOfRange(t *testing.T) {
	db := ovIndex(t)
	stub := &recordFilerStub{t: t, identity: ""}
	h := overridesHandler(t, db, stub, operator)

	rec := postOverride(h, "assay", `{"dispatch_width":99}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range dispatch_width → HTTP %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "dispatch_width") {
		t.Fatalf("the refusal must name the offending field, got: %s", rec.Body.String())
	}
}

// TestOverridesGetReturnsTable proves the read route returns what the POST set.
func TestOverridesGetReturnsTable(t *testing.T) {
	db := ovIndex(t)
	stub := &recordFilerStub{t: t, identity: ""}
	h := overridesHandler(t, db, stub, operator)

	if rec := postOverride(h, "assay", `{"dispatch_width":3}`); rec.Code != http.StatusOK {
		t.Fatalf("seed POST → %d: %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/cells/assay/overrides", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET → HTTP %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var resp contract.OverridesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.DispatchWidth == nil || *resp.DispatchWidth != 3 {
		t.Fatalf("GET did not return the set table: %+v", resp)
	}
}
