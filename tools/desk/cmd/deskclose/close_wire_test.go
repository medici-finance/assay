package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Exercise deskclose's reasons through the real REST adapter. The fake Forge
// tests compare against the caller's constants, which cannot catch a constant
// containing the gh CLI spelling instead of the REST API enum.
func TestCloseItemGitHubStateReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   deskkit.TargetKind
		reason string
		want   string
	}{
		{"superseded", deskkit.TargetIssue, (closeReq{mode: modeSuperseded}).stateReason(), "not_planned"},
		{"duplicate", deskkit.TargetIssue, (closeReq{mode: modeDuplicate}).stateReason(), "not_planned"},
		{"triage", deskkit.TargetIssue, reasonNotPlanned, "not_planned"},
		{"review-request", deskkit.TargetIssue, (closeReq{mode: modeReviewRequest}).stateReason(), "completed"},
		{"change-omits-reason", deskkit.TargetChange, reasonNotPlanned, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bodies := make(chan map[string]any, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch || r.URL.Path != "/repos/example-org/tracker/issues/55" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusNotFound)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode request: %v", err)
					http.Error(w, "invalid JSON", http.StatusBadRequest)
					return
				}
				bodies <- body
				// Literal API values, independent of deskclose's constants.
				if reason, present := body["state_reason"]; present && reason != "completed" && reason != "not_planned" {
					http.Error(w, "invalid state_reason", http.StatusUnprocessableEntity)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			previous := forgeForFn
			t.Cleanup(func() { forgeForFn = previous })
			forgeForFn = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
				return &deskkit.GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()},
					deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, nil
			}
			// verifyClosed=false keeps this a pure PATCH-wire test: it asserts the state_reason
			// the close SENDS, independent of the read-back the lanes layer on top (which has its
			// own coverage in deskclose_closeverify_test.go).
			if err := closeItem(testRepo, subjectIssue, tc.kind, tc.reason, false); err != nil {
				t.Fatalf("close rejected: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("PATCH count = %d, want 1", len(bodies))
			}
			body := <-bodies
			if body["state"] != "closed" {
				t.Errorf("state = %v, want closed", body["state"])
			}
			if tc.want == "" {
				if reason, present := body["state_reason"]; present {
					t.Errorf("change close must omit state_reason, got %v", reason)
				}
			} else if body["state_reason"] != tc.want {
				t.Errorf("state_reason = %v, want %s", body["state_reason"], tc.want)
			}
		})
	}
}
