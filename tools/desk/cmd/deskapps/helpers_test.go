package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// setupTest points HOME at a fresh temp dir and clears ASSAY_CONFIG_HOME, exactly like
// desktoken's setupTest — every write this package makes (apps.env, apps.state.json, PEMs,
// the audit log) resolves under it, and nothing here ever touches a real machine's config.
func setupTest(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	return home
}

// fakeConversionServer serves a stand-in for GitHub's
// POST /app-manifests/{code}/conversions. code "expired" returns 404; any other code
// returns cr (with id, slug filled if unset).
func fakeConversionServer(t *testing.T, cr conversionResult) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app-manifests/{code}/conversions", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if code == "expired" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(cr)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// withFakeGitHubAPI points githubAPIBase at srv for the duration of the test.
func withFakeGitHubAPI(t *testing.T, srv *httptest.Server) {
	t.Helper()
	prev := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = prev })
}

// plantPendingRow returns a StateFile with one pending row for spec, plus the nonce issued
// to it.
func plantPendingRow(t *testing.T, spec AppSpec, tier string) (*StateFile, string) {
	t.Helper()
	nonce, err := newStateNonce()
	if err != nil {
		t.Fatal(err)
	}
	sf := &StateFile{Schema: stateSchema, Apps: []AppRow{{
		App:        spec.Name,
		Tier:       tier,
		Roles:      spec.Roles,
		ReadOnly:   spec.ReadOnly,
		State:      StatePending,
		StateNonce: nonce,
	}}}
	return sf, nonce
}
