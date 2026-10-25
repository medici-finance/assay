package deskkit

// Run-log SHAPE tests against stub servers (review findings runlog-tail-lost-past-read-bound
// and github-runlog-part-per-zip-entry). They drive the public RunLog of each backend over the
// wire, so they pin what a caller actually receives — not a helper fed directly.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// rlRepo is the coordinate every stub here addresses.
var rlRepo = ForgeRepo{Owner: "example-org", Name: "tracker"}

// ghRunLogStub serves one run-log archive behind GitHub's redirect.
func ghRunLogStub(t *testing.T, archive []byte) *GitHubForge {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/runs/501/logs"):
			http.Redirect(w, r, "/_run_archive/501.zip", http.StatusFound)
		case r.URL.Path == "/_run_archive/501.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(archive)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}
}

// glRunLogStub serves a pipeline's job list (paged by per_page/page, continuation in the
// X-Next-Page header, as GitLab does) and each job's trace.
type glRunLogStub struct {
	jobs   []map[string]any
	traces map[int]string
	pages  int // job-list pages served
}

var glStubJobs = regexp.MustCompile(`^/api/v4/projects/[^/]+/pipelines/[0-9]+/jobs$`)
var glStubTrace = regexp.MustCompile(`^/api/v4/projects/[^/]+/jobs/([0-9]+)/trace$`)

func (s *glRunLogStub) forge(t *testing.T) *GitLabForge {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodGet && glStubJobs.MatchString(path):
			s.pages++
			per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if per <= 0 {
				per = 20
			}
			if page <= 0 {
				page = 1
			}
			lo, hi := (page-1)*per, page*per
			if lo > len(s.jobs) {
				lo = len(s.jobs)
			}
			if hi >= len(s.jobs) {
				hi = len(s.jobs)
			} else {
				w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(s.jobs[lo:hi])
		case r.Method == http.MethodGet && glStubTrace.MatchString(path):
			id, _ := strconv.Atoi(glStubTrace.FindStringSubmatch(path)[1])
			tr, ok := s.traces[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(tr))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
}

// glJobs builds n failed jobs with ids 70.. and names job-0...
func glJobs(n int) ([]map[string]any, map[int]string) {
	jobs := make([]map[string]any, 0, n)
	traces := map[int]string{}
	for i := 0; i < n; i++ {
		jobs = append(jobs, map[string]any{"id": 70 + i, "name": "job-" + strconv.Itoa(i), "status": "failed"})
		traces[70+i] = "log of job-" + strconv.Itoa(i) + "\n"
	}
	return jobs, traces
}

// overBound is a log longer than four times the per-part cap — the size at which the first
// shape of RunLog read only a prefix and kept a slice from its middle.
func overBound(tail string) string {
	return strings.Repeat("a", 5*RunLogPartCap) + tail
}

// TestRunLogGithubKeepsTrueTail: a job log larger than any read buffer still yields its LAST
// bytes, so the failing step's message (at the end) is what the caller reads.
func TestRunLogGithubKeepsTrueTail(t *testing.T) {
	f := ghRunLogStub(t, zipOf(t, [][2]string{{"0_build.txt", overBound("THE-FAILURE\n")}}))
	parts, err := f.RunLog(rlRepo, RunRef{ID: "501"})
	if err != nil {
		t.Fatalf("RunLog: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("want 1 part, got %d", len(parts))
	}
	p := parts[0]
	if !p.Truncated || len(p.Text) > RunLogPartCap {
		t.Fatalf("an over-cap log must be truncated to at most %d bytes, got %d trunc=%v", RunLogPartCap, len(p.Text), p.Truncated)
	}
	if !strings.HasSuffix(p.Text, "THE-FAILURE\n") {
		t.Fatalf("the kept text is not the log's tail: ...%q", p.Text[max(0, len(p.Text)-24):])
	}
}

// TestRunLogGitlabKeepsTrueTail is the same obligation on GitLab's per-job trace.
func TestRunLogGitlabKeepsTrueTail(t *testing.T) {
	s := &glRunLogStub{jobs: []map[string]any{{"id": 70, "name": "build", "status": "failed"}},
		traces: map[int]string{70: overBound("THE-FAILURE\n")}}
	parts, err := s.forge(t).RunLog(rlRepo, RunRef{ID: "5"})
	if err != nil {
		t.Fatalf("RunLog: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("want 1 part, got %d", len(parts))
	}
	p := parts[0]
	if !p.Truncated || len(p.Text) > RunLogPartCap {
		t.Fatalf("an over-cap trace must be truncated to at most %d bytes, got %d trunc=%v", RunLogPartCap, len(p.Text), p.Truncated)
	}
	if !strings.HasSuffix(p.Text, "THE-FAILURE\n") {
		t.Fatalf("the kept text is not the trace's tail: ...%q", p.Text[max(0, len(p.Text)-24):])
	}
}

// realArchive is the layout GitHub's run-log archive really has: a top-level whole-job file
// per job, plus a directory per job holding one file per step (the same text, split).
func realArchive(t *testing.T) []byte {
	return zipOf(t, [][2]string{
		{"0_build.txt", "setup\ncompiled\n"},
		{"1_test.txt", "setup\nFAIL: TestThing\n"},
		{"build/1_Set up job.txt", "setup\n"},
		{"build/2_Compile.txt", "compiled\n"},
		{"test/1_Set up job.txt", "setup\n"},
		{"test/2_Run tests.txt", "FAIL: TestThing\n"},
	})
}

// TestRunLogGithubPartPerJob: the real archive yields ONE part per job (the whole-job file),
// never a part per zip entry — a step file would print each job's log twice and spend the
// part bound per step.
func TestRunLogGithubPartPerJob(t *testing.T) {
	parts, err := ghRunLogStub(t, realArchive(t)).RunLog(rlRepo, RunRef{ID: "501"})
	if err != nil {
		t.Fatalf("RunLog: %v", err)
	}
	var names []string
	for _, p := range parts {
		names = append(names, p.Name)
	}
	if len(parts) != 2 || parts[0].Name != "0_build.txt" || parts[1].Name != "1_test.txt" {
		t.Fatalf("want one part per job [0_build.txt 1_test.txt], got %q", names)
	}
	if parts[1].Text != "setup\nFAIL: TestThing\n" {
		t.Fatalf("the job part must be the whole-job log, got %q", parts[1].Text)
	}
}

// TestRunLogGithubStepsOnlyGroups: an archive carrying only per-step files (no whole-job
// file) is grouped into one part per job directory, its steps in archive order.
func TestRunLogGithubStepsOnlyGroups(t *testing.T) {
	arc := zipOf(t, [][2]string{
		{"build/1_Set up job.txt", "setup\n"},
		{"build/2_Compile.txt", "compiled\n"},
		{"test/1_Set up job.txt", "setup\n"},
		{"test/2_Run tests.txt", "FAIL: TestThing\n"},
	})
	parts, err := ghRunLogStub(t, arc).RunLog(rlRepo, RunRef{ID: "501"})
	if err != nil {
		t.Fatalf("RunLog: %v", err)
	}
	if len(parts) != 2 || parts[0].Name != "build" || parts[1].Name != "test" {
		t.Fatalf("want one part per job directory [build test], got %+v", parts)
	}
	if parts[0].Text != "setup\ncompiled\n" || parts[1].Text != "setup\nFAIL: TestThing\n" {
		t.Fatalf("steps must be joined in archive order, got %q / %q", parts[0].Text, parts[1].Text)
	}
}

// TestRunLogGithubEmptyArchive: an archive with no log file is a could-not-check, as GitLab's
// no-jobs case is — never an empty success a caller could read as "nothing went wrong".
func TestRunLogGithubEmptyArchive(t *testing.T) {
	parts, err := ghRunLogStub(t, zipOf(t, nil)).RunLog(rlRepo, RunRef{ID: "501"})
	if err == nil {
		t.Fatalf("an empty archive must be could-not-check, got %d parts", len(parts))
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("want could-not-check (exit %d), got exit %d: %v", ExitUnverifiable, ExitCodeOf(err), err)
	}
}
