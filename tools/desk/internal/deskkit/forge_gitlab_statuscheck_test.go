package deskkit

// forge_gitlab_statuscheck_test.go — the external-status-check verdict lane and its
// three-state tier fallback (forge-gitlab brief 06, Verify item 3).
//
// The verdict lane posts a pass/fail against the MR head SHA through GitLab's external
// status checks, which are an ULTIMATE-only surface. The load-bearing property the brief
// makes its single point of failure is that tier detection routes verdict posting: an
// Ultimate instance posts the check, while a Premium/Free instance answers 403 on the
// Ultimate endpoint and the lane surfaces could-not-check — never a silent downgrade, and
// never a clean return that would mark a merge gate satisfied on a tier that has no gate.

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestForgeGitlabStatusCheckFallback backs Verify item 3.
//
//   - Ultimate fixture: the external-status-checks LIST resolves the lane check by name and
//     the verdict POSTs against the MR head SHA — the method returns success and the wire
//     footprint (sha, status, resolved check id) is exactly the verdict.
//   - Premium fixture: the Ultimate LIST endpoint answers 403; the method returns
//     could-not-check (a *ForgeAPIError at 403, exit ExitUnverifiable) and NEVER reaches the
//     response endpoint — a downgrade to a note or a clean return would be the exact failure
//     the brief's SPOF note forbids.
func TestForgeGitlabStatusCheckFallback(t *testing.T) {
	const checkName = "assay-verdict"
	const checkID = 77

	t.Run("ultimate_posts_the_check", func(t *testing.T) {
		var got struct {
			hit    bool
			sha    string
			status string
			id     int64
		}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case r.Method == http.MethodGet && strings.HasSuffix(path, "/external_status_checks"):
				_ = json.NewEncoder(w).Encode([]map[string]any{
					{"id": checkID, "name": checkName, "project_id": 1, "external_url": "https://desk.example/verdict"},
				})
			case r.Method == http.MethodPost && strings.HasSuffix(path, "/status_check_responses"):
				var in struct {
					SHA    string `json:"sha"`
					Status string `json:"status"`
					ID     int64  `json:"external_status_check_id"`
				}
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &in)
				got.hit, got.sha, got.status, got.id = true, in.SHA, in.Status, in.ID
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "status": in.Status})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()
		f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}

		if err := f.postExternalStatusCheckVerdict(glRepo, 7, checkName, glHead, true); err != nil {
			t.Fatalf("Ultimate: posting the verdict must succeed, got %v", err)
		}
		if !got.hit {
			t.Fatal("Ultimate: no status_check_responses POST was made — the verdict never reached the check")
		}
		if got.status != gitlabStatusCheckPassed {
			t.Fatalf("Ultimate: posted status = %q, want %q", got.status, gitlabStatusCheckPassed)
		}
		if got.sha != glHead {
			t.Fatalf("Ultimate: posted sha = %q, want the MR head %q", got.sha, glHead)
		}
		if got.id != checkID {
			t.Fatalf("Ultimate: posted external_status_check_id = %d, want the resolved id %d", got.id, checkID)
		}
	})

	t.Run("premium_yields_could_not_check", func(t *testing.T) {
		var posted bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.EscapedPath()
			switch {
			case strings.HasSuffix(path, "/external_status_checks"):
				// External status checks are Ultimate-only; a Premium/Free instance 403s here.
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]any{"message": "403 Forbidden"})
			case strings.HasSuffix(path, "/status_check_responses"):
				// Reaching the response endpoint after the tier gate 403 IS the silent
				// downgrade the SPOF note forbids.
				posted = true
				w.WriteHeader(http.StatusOK)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()
		f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}

		err := f.postExternalStatusCheckVerdict(glRepo, 7, checkName, glHead, true)
		if err == nil {
			t.Fatal("Premium: a 403 on the Ultimate endpoint must surface could-not-check, not a clean post")
		}
		if posted {
			t.Fatal("Premium: the verdict must NOT post after the tier gate 403 — silent downgrade")
		}
		if !strings.Contains(err.Error(), "could-not-check") {
			t.Fatalf("Premium: tier refusal must say could-not-check, got %q", err.Error())
		}
		var fae *ForgeAPIError
		if !errors.As(err, &fae) {
			t.Fatalf("Premium: tier 403 should carry a *ForgeAPIError, got %T (%v)", err, err)
		}
		if fae.Status != http.StatusForbidden {
			t.Fatalf("Premium: ForgeAPIError.Status = %d, want 403", fae.Status)
		}
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Fatalf("Premium: tier 403 should map to ExitUnverifiable (%d), got %d", ExitUnverifiable, code)
		}
		t.Logf("Premium external-status-check 403 → could-not-check: %v", err)
	})
}
