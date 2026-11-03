package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

func intp(n int) *int { return &n }

// fakeForge is a minimal GitHub REST stand-in for the record-PR path: it answers
// the default-branch, ref, contents, ref-create, file-put and pulls-create calls
// the filer makes, and COUNTS how many pull requests it opened — the signal the
// idempotency guard is about.
type fakeForge struct {
	mu       sync.Mutex
	pulls    int
	cellsYML string
}

func (f *fakeForge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/contents/config/cells.yaml"):
		writeJSONTest(w, map[string]any{"content": base64.StdEncoding.EncodeToString([]byte(f.cellsYML)), "encoding": "base64", "sha": "filesha"})
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/git/ref/"):
		writeJSONTest(w, map[string]any{"object": map[string]any{"sha": "basesha"}})
	case r.Method == http.MethodGet && strings.Count(r.URL.Path, "/") == 3: // /repos/{o}/{n}
		writeJSONTest(w, map[string]any{"default_branch": "main"})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/git/refs"):
		writeJSONTest(w, map[string]any{"ref": "created"})
	case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/"):
		writeJSONTest(w, map[string]any{"commit": map[string]any{"sha": "commitsha"}})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pulls"):
		f.mu.Lock()
		f.pulls++
		n := f.pulls
		f.mu.Unlock()
		writeJSONTest(w, map[string]any{"number": n, "html_url": "https://example.invalid/pull/" + itoa(n)})
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusInternalServerError)
	}
}

func (f *fakeForge) prCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.pulls }

func writeJSONTest(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// TestOverridesRecordPRIsIdempotentPerHash proves the PR-storm guard (row 6):
// two filings of the SAME table open ONE PR on the forge; a CHANGED table opens
// a second. Without the (cell, table hash) idempotency the record PR would be
// filed on every tick — the "PR storm" the pre-mortem maps here.
func TestOverridesRecordPRIsIdempotentPerHash(t *testing.T) {
	forge := &fakeForge{cellsYML: "cells:\n  - name: assay\n    repos: [example-org/console]\n"}
	srv := httptest.NewServer(forge)
	defer srv.Close()

	g := NewGitHubFiler(srv.URL, "desk-app-token", nil)
	g.SetRecordApp("write-issues-token", "assay-cell-issues-app")

	ov := domain.Overrides{DispatchWidth: intp(1), HoldMerges: boolp(true)}
	req := RecordPRRequest{Cell: "assay", Repo: "example-org/console", ConfigPath: "config/cells.yaml", Overrides: ov}

	// First filing opens a PR.
	res1, err := g.FileRecordPR(context.Background(), req)
	if err != nil {
		t.Fatalf("first FileRecordPR: %v", err)
	}
	if res1.Ref == nil {
		t.Fatalf("first filing must return a PR ref, got %+v", res1)
	}
	if res1.Idempotent {
		t.Fatalf("first filing must not be idempotent")
	}

	// Second filing of the SAME table opens NO new PR.
	res2, err := g.FileRecordPR(context.Background(), req)
	if err != nil {
		t.Fatalf("second FileRecordPR: %v", err)
	}
	if !res2.Idempotent {
		t.Fatalf("refiling the same table must be idempotent, got %+v", res2)
	}
	if got := forge.prCount(); got != 1 {
		t.Fatalf("same table filed twice must open ONE PR, forge saw %d", got)
	}

	// A CHANGED table opens a second PR.
	req2 := req
	req2.Overrides = domain.Overrides{DispatchWidth: intp(2)}
	if _, err := g.FileRecordPR(context.Background(), req2); err != nil {
		t.Fatalf("changed FileRecordPR: %v", err)
	}
	if got := forge.prCount(); got != 2 {
		t.Fatalf("a changed table must open a SECOND PR, forge saw %d", got)
	}
}

// TestRecordPRUnconfiguredFilesNothing proves the honest-degrade path at the
// filer level: with no write-issues App the filer returns could-not-check and
// touches no forge — never a fallback to the desk App token.
func TestRecordPRUnconfiguredFilesNothing(t *testing.T) {
	forge := &fakeForge{cellsYML: "cells:\n  - name: assay\n    repos: [a/b]\n"}
	srv := httptest.NewServer(forge)
	defer srv.Close()

	g := NewGitHubFiler(srv.URL, "desk-app-token", nil) // desk App set, write-issues App NOT
	if g.RecordIdentity() != "" {
		t.Fatalf("record identity must be empty until SetRecordApp is called")
	}
	res, err := g.FileRecordPR(context.Background(), RecordPRRequest{
		Cell: "assay", Repo: "a/b", ConfigPath: "config/cells.yaml",
		Overrides: domain.Overrides{DispatchWidth: intp(1)},
	})
	if err != nil {
		t.Fatalf("unconfigured FileRecordPR: %v", err)
	}
	if res.Ref != nil {
		t.Fatalf("unconfigured filer must file nothing, got ref %+v", res.Ref)
	}
	if !strings.Contains(res.Status, "could-not-check") {
		t.Fatalf("unconfigured filer status must be could-not-check, got %q", res.Status)
	}
	if forge.prCount() != 0 {
		t.Fatalf("unconfigured filer must not touch the forge, saw %d pulls", forge.prCount())
	}
}

func boolp(b bool) *bool { return &b }
