package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// stubPullsServer serves one page of REST pulls (or a fixed error status) and
// points reconcileNewClient at it for the duration of the test.
func stubPullsServer(t *testing.T, status int, pulls []map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(pulls)
	}))
	t.Cleanup(srv.Close)
	prev := reconcileNewClient
	reconcileNewClient = func(token string) *ghClient {
		return &ghClient{doer: srv.Client(), base: srv.URL, token: token}
	}
	t.Cleanup(func() { reconcileNewClient = prev })
}

func mergedPull(n int, ref, body string) map[string]any {
	return map[string]any{
		"number": n, "state": "closed", "body": body,
		"merged_at": "2026-10-01T00:00:00Z", "merge_commit_sha": strings.Repeat("a", 40),
		"head": map[string]any{"sha": strings.Repeat("b", 40), "ref": ref},
	}
}

func runReconcileCaptured(t *testing.T, args ...string) (code int, stdout string) {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	errf, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	code = runReconcile(args, out, errf)
	b, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errf.Name())
	t.Logf("stderr: %s", e)
	return code, string(b)
}

// TestApplyWithoutBackfillWritesTrailerWitnessOnly pins the hourly path's
// match rule (review B2): `reconcile --apply` WITHOUT --backfill writes a row
// only for a merged PR carrying the brief's `Brief:` trailer. A merged PR that
// merely names the brief in its branch or body — an authoring PR, a mention —
// is not a witness on this path and its row is untouched.
func TestApplyWithoutBackfillWritesTrailerWitnessOnly(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	stubPullsServer(t, http.StatusOK, []map[string]any{
		mergedPull(3, "feat/thing", "Implements it.\n\nBrief: dg/03\n"),
		mergedPull(1, "docs/dg/01-author", "Authors dg/01 (the brief file only)."),
	})

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r", "--json")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0; stdout:\n%s", code, stdout)
	}
	var res reconcileResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("decoding --json: %v\n%s", err, stdout)
	}
	if len(res.Applied) != 1 || res.Applied[0].ID != "dg/03" {
		t.Fatalf("want exactly the trailer-witnessed dg/03 applied, got %+v", res.Applied)
	}
	readme := readFixtureReadme(t, root)
	if !strings.Contains(readme, "| 01 | [Risk-gated, in-progress, record present](./brief-01-record-present.md) | 0 | S | in-progress |") {
		t.Fatalf("a branch/body mention is not a witness on the trailer-only path; dg/01 must stay in-progress:\n%s", readme)
	}
}

// TestApplyCouldNotCheckExitsNonzero pins review A1: when the PR read fails
// (HTTP 401 here), --apply writes nothing AND exits nonzero, so a scheduled
// writer that looked at nothing never reads as "board current".
func TestApplyCouldNotCheckExitsNonzero(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "bad-token")
	root := copyDesignGateFixture(t)
	before := readFixtureReadme(t, root)
	stubPullsServer(t, http.StatusUnauthorized, nil)

	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r")
	if code != reconcileCouldNotCheck {
		t.Fatalf("exit %d on an HTTP 401, want %d (could-not-check); stdout:\n%s", code, reconcileCouldNotCheck, stdout)
	}
	if readFixtureReadme(t, root) != before {
		t.Fatal("a could-not-check run must write nothing")
	}
}

// TestApplyTextReportNamesWitnessAndHeld pins the text report the job's commit
// and PR body are built from: every written row names its witness, and every
// held row names the PROBLEM it would have added.
func TestApplyTextReportNamesWitnessAndHeld(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	root := copyDesignGateFixture(t)
	stubPullsServer(t, http.StatusOK, []map[string]any{
		mergedPull(3, "feat/a", "Brief: dg/03\n"),
		mergedPull(5, "feat/b", "Brief: dg/05\n"),
	})
	code, stdout := runReconcileCaptured(t, "--apply", "--root", root, "--repo", "o/r")
	if code != reconcileOK {
		t.Fatalf("exit %d, want 0:\n%s", code, stdout)
	}
	for _, want := range []string{
		"reconcile --apply: wrote 1 row(s)",
		"dg/03",
		"PR #3",
		"reconcile --apply: held 1 row(s)",
		"would add: dg/brief-05",
		"no design: record",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text report missing %q:\n%s", want, stdout)
		}
	}
}
