package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestChangeCommandUsesRequestedSubject(t *testing.T) {
	// Use the ordinary resolver and role admission against the existing fixture roster.
	newEtagForge(t, 0)
	const repo = "example-org/tracker"
	t.Setenv("DESK_LOOP", "worker-desk")
	mintTokenFn = func(role, slug string) (string, string, error) {
		if role != "worker" || slug != repo {
			t.Fatalf("unexpected credential request %s %s", role, slug)
		}
		return "fixture-token", "", nil
	}
	head := strings.Repeat("a", 40)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var result any
		switch r.URL.Path {
		case "/repos/" + repo + "/pulls/7":
			result = map[string]any{"number": 7, "state": "open", "head": map[string]string{"sha": head}}
		case "/repos/" + repo + "/pulls/7/reviews":
			result = []any{map[string]any{"id": 81, "body": "Retain this finding.", "commit_id": head}}
		case "/repos/" + repo + "/commits/" + head + "/status":
			result = map[string]any{"statuses": []any{}}
		case "/repos/" + repo + "/commits/" + head + "/check-runs":
			result = map[string]any{"check_runs": []any{}}
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()
	forgeAPIBase = srv.URL
	var out, errout bytes.Buffer
	code := run([]string{"change", repo, "7", head, "--json"}, &out, &errout)
	var result deskkit.ChangeInspection
	if code != 0 || json.Unmarshal(out.Bytes(), &result) != nil || result.Head != head || len(result.Reviews) != 1 || result.Reviews[0].Body != "Retain this finding." || result.CheckRead != "empty" {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	if calls != 4 {
		t.Fatalf("expected four requests for the admitted change, got %d", calls)
	}
	out.Reset()
	code = run([]string{"change", "outside/repo", "7", "--json"}, &out, &errout)
	if code != deskkit.ExitRefused || calls != 4 {
		t.Fatalf("unadmitted subject reached the forge: %d, %d", code, calls)
	}
}

// The inventory exercises each verb with an ordinary successful subject.
func inspectionSubjectFixture(t *testing.T) {
	t.Helper()
	forgeHooks.getPR = func(_ string, number int) (*deskkit.PullRequest, error) {
		return &deskkit.PullRequest{Number: number, State: "open", HeadSHA: strings.Repeat("a", 40)}, nil
	}
}
