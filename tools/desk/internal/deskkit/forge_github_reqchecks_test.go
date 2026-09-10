package deskkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
)

// forge_github_reqchecks_test.go — RequiredStatusChecks admin-free fallback.
//
// The legacy `branches/{b}/protection/required_status_checks` endpoint answers 403 for a
// token without the `administration` scope (every reviewer/worker App token). A 403 is NOT
// "nothing required" and must NOT be read as green off an absent rollup; but neither is it a
// dead end, because two OTHER endpoints the same token CAN read answer the same question:
//   - GET /repos/{o}/{r}/branches/{b}        → `.protected` (is anything protecting it at all)
//   - GET /repos/{o}/{r}/rules/branches/{b}  → the active rules, incl. required_status_checks
// These tests pin the four branches of that fallback.

// reqChecksMux routes the three endpoints RequiredStatusChecks may touch. A handler set to nil
// means "endpoint not expected"; if it is hit anyway the test fails. Each handler returns the
// (status, body) the scenario needs.
type reqChecksMux struct {
	t                   *testing.T
	legacyStatus        int    // /protection/required_status_checks
	legacyBody          string //   (only used on 2xx)
	branchStatus        int    // /branches/{b}
	branchBody          string
	rulesStatus         int // /rules/branches/{b}
	rulesBody           string
	sawBranch, sawRules bool
	branchNil, rulesNil bool // handler intentionally not wired (must not be hit)
}

func newReqChecksServer(t *testing.T, m *reqChecksMux) *httptest.Server {
	t.Helper()
	m.t = t
	legacyPath := fmt.Sprintf("/repos/%s/%s/branches/main/protection/required_status_checks",
		forgeTestRepo.Owner, forgeTestRepo.Name)
	branchPath := fmt.Sprintf("/repos/%s/%s/branches/main", forgeTestRepo.Owner, forgeTestRepo.Name)
	rulesPath := fmt.Sprintf("/repos/%s/%s/rules/branches/main", forgeTestRepo.Owner, forgeTestRepo.Name)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case legacyPath:
			if m.legacyStatus == 0 || m.legacyStatus == http.StatusOK {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(m.legacyBody))
				return
			}
			w.WriteHeader(m.legacyStatus)
		case branchPath:
			m.sawBranch = true
			if m.branchNil {
				m.t.Errorf("branches/{b} endpoint was hit but the scenario did not expect it")
			}
			if m.branchStatus == 0 || m.branchStatus == http.StatusOK {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(m.branchBody))
				return
			}
			w.WriteHeader(m.branchStatus)
		case rulesPath:
			m.sawRules = true
			if m.rulesNil {
				m.t.Errorf("rules/branches/{b} endpoint was hit but the scenario did not expect it")
			}
			if m.rulesStatus == 0 || m.rulesStatus == http.StatusOK {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(m.rulesBody))
				return
			}
			w.WriteHeader(m.rulesStatus)
		default:
			m.t.Errorf("unexpected request path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRequiredStatusChecksAdminFreeFallback(t *testing.T) {
	const injected = "test-injected-token-0000"

	cases := []struct {
		name        string
		mux         *reqChecksMux
		wantSet     []string
		wantErr     bool // could-not-check
		wantSawBr   bool
		wantSawRule bool
	}{
		{
			// (a) legacy 404 ⇒ nothing required (empty), no fallback consulted.
			name: "legacy_404_empty",
			mux: &reqChecksMux{
				legacyStatus: http.StatusNotFound,
				branchNil:    true,
				rulesNil:     true,
			},
			wantSet: nil,
		},
		{
			// (b) legacy 403 + branch not protected ⇒ empty; rules not consulted.
			name: "legacy_403_unprotected_empty",
			mux: &reqChecksMux{
				legacyStatus: http.StatusForbidden,
				branchStatus: http.StatusOK,
				branchBody:   `{"name":"main","protected":false}`,
				rulesNil:     true,
			},
			wantSet:   nil,
			wantSawBr: true,
		},
		{
			// (c) legacy 403 + protected + rules carry contexts ⇒ that union.
			name: "legacy_403_protected_rules_contexts",
			mux: &reqChecksMux{
				legacyStatus: http.StatusForbidden,
				branchStatus: http.StatusOK,
				branchBody:   `{"name":"main","protected":true}`,
				rulesStatus:  http.StatusOK,
				rulesBody: `[
					{"type":"pull_request","parameters":{}},
					{"type":"required_status_checks","parameters":{"required_status_checks":[
						{"context":"ci/build","integration_id":1},
						{"context":"leak-sweep"}
					]}},
					{"type":"required_status_checks","parameters":{"required_status_checks":[
						{"context":"ci/build"},
						{"context":"go-test"}
					]}}
				]`,
			},
			wantSet:     []string{"ci/build", "go-test", "leak-sweep"},
			wantSawBr:   true,
			wantSawRule: true,
		},
		{
			// (d) legacy 403 + both fallbacks fail ⇒ could-not-check.
			name: "legacy_403_both_fallbacks_fail",
			mux: &reqChecksMux{
				legacyStatus: http.StatusForbidden,
				branchStatus: http.StatusForbidden,
				rulesStatus:  http.StatusForbidden,
			},
			wantErr:     true,
			wantSawBr:   true,
			wantSawRule: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newReqChecksServer(t, tc.mux)
			f := &GitHubForge{Token: injected, BaseURL: srv.URL, Client: srv.Client()}
			got, err := f.RequiredStatusChecks(forgeTestRepo, "main")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected a could-not-check error, got set=%v nil err", got)
				}
				if !IsUnverifiable(err) {
					t.Fatalf("could-not-check should be Unverifiable, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				gs := append([]string(nil), got...)
				ws := append([]string(nil), tc.wantSet...)
				sort.Strings(gs)
				sort.Strings(ws)
				if len(gs) != 0 || len(ws) != 0 {
					if !reflect.DeepEqual(gs, ws) {
						t.Fatalf("required set mismatch\n got: %v\nwant: %v", gs, ws)
					}
				}
			}
			if tc.mux.sawBranch != tc.wantSawBr {
				t.Errorf("branches/{b} consulted=%v, want %v", tc.mux.sawBranch, tc.wantSawBr)
			}
			if tc.mux.sawRules != tc.wantSawRule {
				t.Errorf("rules/branches/{b} consulted=%v, want %v", tc.mux.sawRules, tc.wantSawRule)
			}
		})
	}
}
