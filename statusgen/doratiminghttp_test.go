package main

// doratiminghttp_test.go — the DORA-timing recorder's REST source, exercised
// against an httptest server standing in for GitHub.
//
// These tests exist because the recorder used to shell out to `gh api`, and on
// a runner image without `gh` on PATH every read failed with
// `exec: "gh": executable file not found in $PATH`. That failure is fail-open by
// design, so nothing went red — the substrate simply never accrued a record from
// the day the recorder shipped. The guard these tests pin is therefore not "the
// happy path parses": it is that the production source needs NO binary on PATH,
// and that a failed read is still distinguishable from an empty one.
//
// Offline envelope: every request goes to a local httptest server. Nothing here
// contacts github.com.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// doraFakeAPI is an httptest stand-in for the three REST endpoints the recorder
// reads. Handlers are keyed by URL path; an unregistered path is a 404 so a
// test that silently reads the wrong endpoint fails loudly rather than passing
// on an empty list.
type doraFakeAPI struct {
	srv    *httptest.Server
	mu     map[string]func(w http.ResponseWriter, r *http.Request)
	seen   []string // request paths+queries, in order
	auth   []string // the Authorization header of each request
	tester *testing.T
}

func newDoraFakeAPI(t *testing.T) *doraFakeAPI {
	t.Helper()
	f := &doraFakeAPI{mu: map[string]func(http.ResponseWriter, *http.Request){}, tester: t}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.seen = append(f.seen, r.URL.RequestURI())
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		if h, ok := f.mu[r.URL.Path]; ok {
			h(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *doraFakeAPI) handle(path string, h func(w http.ResponseWriter, r *http.Request)) {
	f.mu[path] = h
}

// json200 registers a handler returning body verbatim with HTTP 200.
func (f *doraFakeAPI) json200(path, body string) {
	f.handle(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	})
}

// status registers a handler returning a GitHub-shaped error at code.
func (f *doraFakeAPI) status(path string, code int, message string) {
	f.handle(path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"message":%q}`, message)
	})
}

// source builds the production source type pointed at the fake, with token.
// It uses the SAME struct and methods main.go constructs — only the base URL
// differs — so the tests exercise production code, not a parallel path.
func (f *doraFakeAPI) source(token string) httpDoraTimingSource {
	return httpDoraTimingSource{c: &ghClient{
		doer:  &http.Client{Timeout: 10 * time.Second},
		base:  f.srv.URL,
		token: token,
	}}
}

const (
	fakeRunsPath    = "/repos/owner/repo/actions/runs"
	fakePullsPath   = "/repos/owner/repo/pulls"
	fakeCommitsPath = "/repos/owner/repo/pulls/1616/commits"
)

// --- happy path -------------------------------------------------------------

// The whole point of the change: a production record pass that appends real
// records with NO `gh` anywhere on PATH. PATH is emptied for the duration so the
// test cannot pass by accidentally finding a CLI.
func TestDoraHTTPRecordsWithNoGhOnPath(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.json200(fakeRunsPath, `{"workflow_runs":[
		{"id":100,"name":"ci","path":".github/workflows/ci.yml","head_sha":"redsha","status":"completed","conclusion":"failure","updated_at":"2026-08-25T20:00:00Z"},
		{"id":101,"name":"ci","path":".github/workflows/ci.yml","head_sha":"greensha","status":"completed","conclusion":"success","updated_at":"2026-08-25T21:30:00Z"}
	]}`)
	f.json200(fakePullsPath, `[
		{"number":1616,"created_at":"2026-08-25T17:30:00Z","merged_at":"2026-08-25T18:37:51Z","merge_commit_sha":"9c34"},
		{"number":1615,"created_at":"2026-08-25T10:00:00Z","merged_at":null,"merge_commit_sha":""}
	]`)
	f.json200(fakeCommitsPath, `[
		{"commit":{"author":{"date":"2026-08-25T17:20:00Z"}}},
		{"commit":{"author":{"date":"2026-08-25T17:10:00Z"}}}
	]`)

	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("PATH", "") // no `gh`, no `git` — the runner condition that broke the old source
	os.Unsetenv("STATUSGEN_DORA_WORKFLOW")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, f.source("tok"), now) })

	if n != 2 {
		t.Fatalf("appended %d record(s), want 2 (1 restore episode + 1 lead time); stderr=%q", n, stderr)
	}
	if strings.Contains(stderr, "DEGRADED") {
		t.Errorf("a successful pass must not emit a degraded signal: %q", stderr)
	}

	recs, err := loadDoraTimingRecords(filepath.Join(dir, filepath.FromSlash(doraTimingRelPath)))
	if err != nil {
		t.Fatal(err)
	}
	var gotEpisode, gotLead bool
	for _, r := range recs {
		switch r.Type {
		case "restore_episode":
			gotEpisode = true
			if r.FailedRunID != 100 {
				t.Errorf("failed_run_id=%d, want 100", r.FailedRunID)
			}
			if r.RestoreSeconds != 5400 { // 20:00 -> 21:30
				t.Errorf("restore_seconds=%d, want 5400", r.RestoreSeconds)
			}
		case "pr_lead_time":
			gotLead = true
			if r.PR != 1616 {
				t.Errorf("pr=%d, want 1616", r.PR)
			}
			// first_commit anchor 17:10 -> merged 18:37:51 = 5271s. The
			// opened_at fallback would give 4071s, so this also pins that the
			// commits endpoint was really read.
			if r.LeadSeconds != 5271 {
				t.Errorf("lead_seconds=%d, want 5271 (first_commit anchor)", r.LeadSeconds)
			}
		}
	}
	if !gotEpisode || !gotLead {
		t.Errorf("missing records: episode=%v lead=%v", gotEpisode, gotLead)
	}

	// Field selection + auth are part of the contract the shell-out used to
	// carry in its URL string; assert they survived the port.
	joined := strings.Join(f.seen, "\n")
	for _, want := range []string{"branch=main", "status=completed", "per_page=100", "state=closed", "base=main"} {
		if !strings.Contains(joined, want) {
			t.Errorf("no request carried %q; requests were:\n%s", want, joined)
		}
	}
	for i, a := range f.auth {
		if a != "Bearer tok" {
			t.Errorf("request %d (%s) sent Authorization %q, want %q", i, f.seen[i], a, "Bearer tok")
		}
	}
}

// --- 401 --------------------------------------------------------------------

// A 401 must be a could-not-check that NAMES the status — never an empty read
// that looks like a quiet day, and never a fabricated interval. The old
// degraded line blamed a missing binary; this pins that it now blames the HTTP
// status it actually got.
func TestDoraHTTP401NamesStatus(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.status(fakeRunsPath, http.StatusUnauthorized, "Bad credentials")
	f.status(fakePullsPath, http.StatusUnauthorized, "Bad credentials")

	src := f.source("")
	runs, err := src.MainWorkflowRuns("owner/repo")
	if err == nil {
		t.Fatalf("a 401 must be an error, got runs=%v err=nil", runs)
	}
	if runs != nil {
		t.Errorf("a failed read must return no runs, got %v", runs)
	}
	if !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "Bad credentials") {
		t.Errorf("error must name the HTTP status and message, got %q", err)
	}

	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("PATH", "")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, src, now) })

	if n != 0 {
		t.Fatalf("a 401 must fail OPEN and append nothing, appended %d", n)
	}
	if _, serr := os.Stat(filepath.Join(dir, filepath.FromSlash(doraTimingRelPath))); serr == nil {
		t.Error("a 401 must never fabricate a substrate file")
	}
	if !strings.Contains(stderr, "DEGRADED") {
		t.Errorf("a 401 must emit the loud degraded signal, got %q", stderr)
	}
	if !strings.Contains(stderr, "HTTP 401") {
		t.Errorf("the degraded line must name the HTTP status, got %q", stderr)
	}
	// The signal must not point the operator at the thing that can no longer
	// fail here.
	if strings.Contains(stderr, "gh availability") {
		t.Errorf("the degraded line still blames gh availability: %q", stderr)
	}
}

// --- 5xx --------------------------------------------------------------------

// A server-side 503 is GitHub's problem, not the operator's token: it must
// still be a named could-not-check, and the recorder must still fail open.
func TestDoraHTTP503NamesStatus(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.status(fakeRunsPath, http.StatusServiceUnavailable, "Service unavailable")
	f.json200(fakePullsPath, `[]`)

	src := f.source("tok")
	if _, err := src.MainWorkflowRuns("owner/repo"); err == nil {
		t.Fatal("a 503 must be an error")
	} else if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error must name HTTP 503, got %q", err)
	}

	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("PATH", "")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, src, now) })

	if n != 0 {
		t.Fatalf("a 503 must append nothing, appended %d", n)
	}
	if !strings.Contains(stderr, "HTTP 503") {
		t.Errorf("degraded line must name HTTP 503, got %q", stderr)
	}
	// Only the restore read failed; the lead-time read succeeded-but-empty. The
	// signal must say so rather than smearing both.
	if !strings.Contains(stderr, "the restore-episode read") {
		t.Errorf("degraded line must name WHICH read failed, got %q", stderr)
	}
}

// --- empty page -------------------------------------------------------------

// An empty page is a genuinely healthy no-op: reads succeeded, nothing new. It
// must stay quiet on stderr, or the degraded signal loses its meaning.
func TestDoraHTTPEmptyPageIsQuiet(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.json200(fakeRunsPath, `{"workflow_runs":[]}`)
	f.json200(fakePullsPath, `[]`)

	src := f.source("tok")
	runs, err := src.MainWorkflowRuns("owner/repo")
	if err != nil || len(runs) != 0 {
		t.Fatalf("empty page: runs=%v err=%v, want 0 runs and no error", runs, err)
	}
	prs, perr := src.MergedPRs("owner/repo", mustTime(t, "2026-07-01T00:00:00Z"))
	if perr != nil || len(prs) != 0 {
		t.Fatalf("empty page: prs=%v err=%v, want 0 PRs and no error", prs, perr)
	}
	// An empty first page must stop paging, not walk the cap.
	if got := len(f.seen); got != 2 {
		t.Errorf("made %d requests (%v), want 2 — an empty first page must stop pagination", got, f.seen)
	}

	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("PATH", "")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, src, now) })
	if n != 0 {
		t.Fatalf("empty reads must append nothing, appended %d", n)
	}
	if strings.Contains(stderr, "DEGRADED") {
		t.Errorf("a healthy empty pass must stay quiet on stderr: %q", stderr)
	}
	if _, serr := os.Stat(filepath.Join(dir, filepath.FromSlash(doraTimingRelPath))); serr == nil {
		t.Error("a healthy empty pass must not create the substrate file")
	}
}

// --- pagination -------------------------------------------------------------

// A full page must be followed, a short page must stop, and the page cap must
// bound the walk — the same three properties the shell-out's loop had.
func TestDoraHTTPPagination(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.handle(fakeRunsPath, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		n := 100
		if page != "1" {
			n = 3 // a short second page ends the walk
		}
		runs := make([]workflowRun, 0, n)
		for i := 0; i < n; i++ {
			runs = append(runs, workflowRun{
				ID: int64(1000 + i), Name: "ci", HeadSHA: fmt.Sprintf("%s-%d", page, i),
				Status: "completed", Conclusion: "success", UpdatedAt: "2026-08-25T20:00:00Z",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			WorkflowRuns []workflowRun `json:"workflow_runs"`
		}{runs})
	})

	runs, err := f.source("tok").MainWorkflowRuns("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 103 {
		t.Errorf("got %d runs, want 103 (a full page followed, a short page stopping the walk)", len(runs))
	}
	if len(f.seen) != 2 {
		t.Errorf("made %d requests (%v), want 2", len(f.seen), f.seen)
	}
	if !strings.Contains(f.seen[1], "page=2") {
		t.Errorf("second request did not ask for page 2: %q", f.seen[1])
	}
}

// --- token resolution -------------------------------------------------------

func TestDoraTokenPrefersGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", " gh-one ")
	t.Setenv("GITHUB_TOKEN", "gha-two")
	if got := doraToken(); got != "gh-one" {
		t.Errorf("doraToken()=%q, want the trimmed GH_TOKEN %q", got, "gh-one")
	}
	t.Setenv("GH_TOKEN", "")
	if got := doraToken(); got != "gha-two" {
		t.Errorf("with GH_TOKEN empty, doraToken()=%q, want the GITHUB_TOKEN fallback", got)
	}
	t.Setenv("GITHUB_TOKEN", "")
	if got := doraToken(); got != "" {
		t.Errorf("with neither set, doraToken()=%q, want empty (unauthenticated is not an error here)", got)
	}
}

// --- FirstCommitAt three-state ----------------------------------------------

// An unavailable commit list is ok=false — the caller then anchors on opened_at
// and RECORDS that it did. It must never surface as a zero commit time, which
// would fabricate a lead time reaching back to the zero instant.
func TestDoraHTTPFirstCommitNotOK(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.status(fakeCommitsPath, http.StatusForbidden, "API rate limit exceeded")

	got, ok := f.source("tok").FirstCommitAt("owner/repo", 1616)
	if ok {
		t.Fatalf("a failed commits read must report ok=false, got %v", got)
	}
	if !got.IsZero() {
		t.Errorf("a failed commits read must return the zero time, got %v", got)
	}
}

// With the commits endpoint unavailable, the recorder still records a lead time
// — anchored on opened_at, and saying so — rather than recording nothing.
func TestDoraHTTPOpenedAnchorFallback(t *testing.T) {
	f := newDoraFakeAPI(t)
	f.json200(fakeRunsPath, `{"workflow_runs":[]}`)
	f.json200(fakePullsPath, `[{"number":1616,"created_at":"2026-08-25T17:30:00Z","merged_at":"2026-08-25T18:37:51Z","merge_commit_sha":"9c34"}]`)
	f.status(fakeCommitsPath, http.StatusForbidden, "API rate limit exceeded")

	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("PATH", "")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	n := recordDoraTiming(dir, f.source("tok"), now)
	if n != 1 {
		t.Fatalf("appended %d, want 1 (the opened_at-anchored lead time)", n)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(doraTimingRelPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"anchor":"opened"`) {
		t.Errorf("record must state the opened anchor it fell back to: %s", data)
	}
	// 17:30:00 -> 18:37:51 = 4071s
	if !strings.Contains(string(data), `"lead_seconds":4071`) {
		t.Errorf("lead_seconds wrong for the opened anchor: %s", data)
	}
}
