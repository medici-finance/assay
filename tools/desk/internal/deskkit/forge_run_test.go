package deskkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// forge_run_test.go — the negative paths of forge-neutral/14's run and gate-approval ops,
// asserted by name (the brief's Verify rows 5 and 6) on top of the golden corpus that pins
// each op's wire shape. Every case runs against a RECORDED server; nothing here reaches a
// live forge.

// TestRunWorkflowAmbiguousCorrelationRefuses — Verify row 5. Two runs of the same workflow
// were created inside the correlation window (two dispatches raced), so the dispatch's own
// run cannot be told apart. The op must REFUSE as could-not-check naming the ambiguity and
// both candidate runs — never pick the newest.
func TestRunWorkflowAmbiguousCorrelationRefuses(t *testing.T) {
	s := newGoldenServer(t)
	s.workflowRuns = map[string]any{"total_count": 2, "workflow_runs": []map[string]any{
		ghRunFixture(601, "2026-09-23T12:00:09Z", "main", "release-runner-app[bot]"),
		ghRunFixture(600, "2026-09-23T12:00:03Z", "main", "release-runner-app[bot]"),
	}}
	f := s.forge()
	f.now = fixedRunClock
	f.sleep = func(time.Duration) {
		t.Fatal("an ambiguous correlation must refuse on the first read, not wait and re-read")
	}

	ref, err := f.RunWorkflow(forgeTestRepo, RunWorkflowInput{Workflow: "release.yml", Ref: "main",
		Actor: "release-runner-app[bot]"})
	if err == nil {
		t.Fatalf("RunWorkflow resolved %+v from two matching runs — a newest-first guess, the exact failure the refusal exists for", ref)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit %d, want %d (could-not-check): %v", ExitCodeOf(err), ExitUnverifiable, err)
	}
	msg := err.Error()
	for _, want := range []string{"ambiguous run correlation", "600", "601", "WAS accepted"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not name %q: %s", want, msg)
		}
	}
	if ref.ID != "" {
		t.Errorf("a refused correlation still handed back run %q", ref.ID)
	}
	if n := len(s.requests); n != 2 {
		t.Errorf("%d requests, want exactly 2 (the dispatch and one list read): %+v", n, s.requests)
	}
	t.Logf("refused as could-not-check: %s", msg)
}

// TestRunWorkflowCorrelationWaitsForTheRun pins the bounded wait: the first list read comes
// back before the forge has listed the run, the second finds exactly one. The op must wait
// (through the injected sleep — no real elapsed time) and resolve it, with the SAME floor on
// both reads.
func TestRunWorkflowCorrelationWaitsForTheRun(t *testing.T) {
	calls := 0
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusNoContent)
		default:
			calls++
			queries = append(queries, r.URL.RawQuery)
			if calls == 1 {
				_, _ = w.Write([]byte(`{"total_count":0,"workflow_runs":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"total_count":1,"workflow_runs":[{"id":700,"event":"workflow_dispatch",` +
				`"head_branch":"main","created_at":"2026-09-23T12:00:04Z","html_url":"https://example/runs/700",` +
				`"actor":{"login":"release-runner-app[bot]"}}]}`))
		}
	}))
	t.Cleanup(srv.Close)
	waits := 0
	f := &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client(), now: fixedRunClock,
		sleep: func(time.Duration) { waits++ }}
	ref, err := f.RunWorkflow(forgeTestRepo, RunWorkflowInput{Workflow: "release.yml", Ref: "main"})
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if ref.ID != "700" || waits != 1 || calls != 2 {
		t.Fatalf("ref=%+v waits=%d list-reads=%d, want run 700 after exactly one wait and two reads", ref, waits, calls)
	}
	if queries[0] != queries[1] {
		t.Fatalf("the correlation key moved between reads (%q vs %q) — the floor must be taken once, before the dispatch", queries[0], queries[1])
	}
}

// TestApproveGateUnmatchedEnvironmentRefuses — Verify row 6, both backends. The run is waiting
// on a gate, but not on the one named. The op must refuse as could-not-check naming the run and
// the requested gate, and must NOT approve the environment that happened to be pending.
func TestApproveGateUnmatchedEnvironmentRefuses(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		s := newGoldenServer(t)
		s.pendingDeployments = []map[string]any{
			{"environment": map[string]any{"id": 11, "name": "staging"}, "current_user_can_approve": true},
		}
		err := s.forge().ApproveGate(forgeTestRepo, RunRef{ID: "501"}, ApproveGateInput{Gate: "production"})
		assertUnmatchedGateRefusal(t, err, "501", "production")
		for _, r := range s.requests {
			if r.Method != http.MethodGet {
				t.Fatalf("an unmatched gate still produced a %s %s — a different pending environment was approved", r.Method, r.Path)
			}
		}
		if len(s.requests) != 1 {
			t.Fatalf("%d requests, want exactly the one pending-deployments read", len(s.requests))
		}
	})
	t.Run("gitlab", func(t *testing.T) {
		s := newGLServer(t)
		s.deployments = []map[string]any{
			{"id": 300, "status": "blocked", "environment": map[string]any{"name": "production"},
				"deployable": map[string]any{"pipeline": map[string]any{"id": 8000}}},
		}
		err := s.forge().ApproveGate(glRepo, RunRef{ID: "9001"},
			ApproveGateInput{Gate: "production", Shape: GateShapeEnvironment})
		assertUnmatchedGateRefusal(t, err, "9001", "production")
		for _, r := range s.requests {
			if r.Method != http.MethodGet {
				t.Fatalf("an unmatched gate still produced a %s %s — another pipeline's deployment was approved", r.Method, r.Path)
			}
		}
	})
}

func assertUnmatchedGateRefusal(t *testing.T, err error, run, gate string) {
	t.Helper()
	if err == nil {
		t.Fatal("ApproveGate approved a gate the run is not waiting on")
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit %d, want %d (could-not-check): %v", ExitCodeOf(err), ExitUnverifiable, err)
	}
	for _, want := range []string{run, `"` + gate + `"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not name %s: %v", want, err)
		}
	}
	t.Logf("refused as could-not-check: %v", err)
}

// TestGitLabRunWorkflowSendsNoAccessTokenHeader pins the trigger call's credential shape: the
// pipeline trigger token travels as the request's `token` field and is NOT also presented as a
// PRIVATE-TOKEN access token (which the instance would reject as an invalid access token, and
// which is not what a trigger token is). Header shape is not in the golden capture, so it is
// pinned here.
func TestGitLabRunWorkflowSendsNoAccessTokenHeader(t *testing.T) {
	var gotHeader, gotAuth string
	var seen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		gotHeader = r.Header.Get("PRIVATE-TOKEN")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":9001,"status":"created","web_url":"https://gitlab.example/p/9001"}`))
	}))
	t.Cleanup(srv.Close)
	f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
	if _, err := f.RunWorkflow(glRepo, RunWorkflowInput{Ref: "main"}); err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if seen != 1 {
		t.Fatalf("%d requests, want 1", seen)
	}
	if gotHeader != "" || gotAuth != "" {
		t.Fatalf("the trigger call presented an access-token header (PRIVATE-TOKEN=%q Authorization=%q) — a trigger token "+
			"authenticates as the request's token field only", gotHeader, gotAuth)
	}
}

// TestGitLabRunWorkflowRefusesUnmintedToken — the backend layer's refusal on GitLab: with no
// minted token the trigger client is never built and zero requests go out.
func TestGitLabRunWorkflowRefusesUnmintedToken(t *testing.T) {
	s := newGLServer(t)
	f := s.forge()
	f.Token = ""
	if _, err := f.RunWorkflow(glRepo, RunWorkflowInput{Ref: "main"}); err == nil {
		t.Fatal("RunWorkflow reached the forge with no minted token")
	}
	if len(s.requests) != 0 {
		t.Fatalf("%d requests went out with no minted token: %+v", len(s.requests), s.requests)
	}
}

// TestRunStatusUnknownStateIsCouldNotCheck — a status the mapping does not know is never
// rounded to a known one, on either backend.
func TestRunStatusUnknownStateIsCouldNotCheck(t *testing.T) {
	gh := newGoldenServer(t)
	gh.run = map[string]any{"id": 501, "status": "teleporting"}
	if _, err := gh.forge().RunStatus(forgeTestRepo, RunRef{ID: "501"}); ExitCodeOf(err) != ExitUnverifiable {
		t.Errorf("github: unknown status read as %v, want could-not-check", err)
	}
	gl := newGLServer(t)
	gl.pipeline = map[string]any{"id": 9001, "status": "teleporting"}
	if _, err := gl.forge().RunStatus(glRepo, RunRef{ID: "9001"}); ExitCodeOf(err) != ExitUnverifiable {
		t.Errorf("gitlab: unknown status read as %v, want could-not-check", err)
	}
}

// TestRunIDValidatedBeforeAnyRequest — a run id that is not a bare integer is refused with zero
// requests on both backends (it is interpolated into a path).
func TestRunIDValidatedBeforeAnyRequest(t *testing.T) {
	for _, id := range []string{"", "12/../../x", "0", "-4", "12a"} {
		gh := newGoldenServer(t)
		if _, err := gh.forge().RunStatus(forgeTestRepo, RunRef{ID: id}); err == nil || len(gh.requests) != 0 {
			t.Errorf("github: run id %q: err=%v requests=%d, want a refusal with zero requests", id, err, len(gh.requests))
		}
		gl := newGLServer(t)
		if err := gl.forge().ApproveGate(glRepo, RunRef{ID: id}, ApproveGateInput{Gate: "g", Shape: GateShapeManualJob}); err == nil || len(gl.requests) != 0 {
			t.Errorf("gitlab: run id %q: err=%v requests=%d, want a refusal with zero requests", id, err, len(gl.requests))
		}
	}
}
