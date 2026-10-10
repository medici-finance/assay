package main

// readpath_test.go — review findings log-reads-charge-write-budget and verify-row-2-backend-leg.
//
// The read verbs (status, log) must not spend the write verbs' budget, and the GitLab legs of
// `log` and `retry` must reach the REAL GitLab backend through the real resolver — a recording
// fake reports a forge kind the verb never looks at, so it cannot fail on a backend defect.

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glPlaceholder is a low-entropy placeholder per role — NOT a real credential, and carrying no
// vendor token prefix (the leak gate's pattern leg matches on the prefix shape alone).
func glPlaceholder(role string) string { return "example-placeholder-" + role + "-0000" }

// plantGitlabCustody writes each role's GitLab PAT custody file (0600, where the resolver
// looks) and points GITLAB_API_BASE at the stub.
func plantGitlabCustody(t *testing.T, base string, roles ...string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
	for _, role := range roles {
		if err := os.WriteFile(filepath.Join(dir, "gitlab-"+role+".token"), []byte(glPlaceholder(role)+"\n"), 0o600); err != nil {
			t.Fatalf("planting the %s custody file: %v", role, err)
		}
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	t.Setenv("GITLAB_API_BASE", base)
}

// glRunStub is a fake GitLab instance speaking the pipeline-jobs list, the job trace and the
// job retry routes. It records every request with the PRIVATE-TOKEN it carried.
type glRunStub struct {
	srv      *httptest.Server
	jobs     []map[string]any
	traces   map[string]string
	requests []string // "METHOD path token"
}

var (
	glJobsRoute  = regexp.MustCompile(`^/api/v4/projects/[^/]+/pipelines/[0-9]+/jobs$`)
	glTraceRoute = regexp.MustCompile(`^/api/v4/projects/[^/]+/jobs/([0-9]+)/trace$`)
	glRetryRoute = regexp.MustCompile(`^/api/v4/projects/[^/]+/jobs/([0-9]+)/retry$`)
)

func newGLRunStub(t *testing.T) *glRunStub {
	t.Helper()
	s := &glRunStub{traces: map[string]string{}}
	s.srv = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		path := r.URL.EscapedPath()
		s.requests = append(s.requests, r.Method+" "+path+" "+r.Header.Get("PRIVATE-TOKEN"))
		switch {
		case r.Method == http.MethodGet && glJobsRoute.MatchString(path):
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(s.jobs)
		case r.Method == http.MethodGet && glTraceRoute.MatchString(path):
			tr, ok := s.traces[glTraceRoute.FindStringSubmatch(path)[1]]
			if !ok {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			rw.Header().Set("Content-Type", "text/plain")
			_, _ = rw.Write([]byte(tr))
		case r.Method == http.MethodPost && glRetryRoute.MatchString(path):
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(rw).Encode(map[string]any{"id": 99, "status": "pending"})
		default:
			rw.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// bigTail is a log over the per-part cap whose LAST line is the failure, with terminal-active
// bytes planted in it.
func bigTail() string {
	return strings.Repeat("x", deskkit.RunLogPartCap+64) + "\x1b]0;evil\x07\rFAIL: TestThing\n"
}

// TestDeskrunLogSucceedsWorkerAndReviewer — Verify row 2 (+flow). `log` reads the run's log under
// the CALLING role's own credential for BOTH the worker and the reviewer, on BOTH forges, end to
// end: verb → the real resolver → the real backend → a stub forge. The request reaches the forge
// bearing that role's own token (never the release-runner's), one section per job comes back
// with terminal-active bytes stripped, an over-cap job keeps its tail, and a repo whose run
// credential is bound to a human reads fine — there is deliberately no binding gate on a read.
func TestDeskrunLogSucceedsWorkerAndReviewer(t *testing.T) {
	for _, kind := range []deskkit.ForgeKind{deskkit.ForgeGitHub, deskkit.ForgeGitLab} {
		for _, tc := range []struct{ loop, role string }{{"worker-desk", "worker"}, {"pr-review-desk", "reviewer"}} {
			t.Run(string(kind)+"-"+tc.role, func(t *testing.T) {
				w := plantWorld(t, kind)
				setLoop(t, tc.loop)
				logForgeFn = func(fr deskkit.ForgeRepo, role string) (deskkit.Forge, deskkit.ForgeResolution, error) {
					w.logRoles = append(w.logRoles, role)
					return deskkit.ResolveForge(fr, role)
				}
				var repo, wantAuth string
				var wire func() []string
				switch kind {
				case deskkit.ForgeGitHub:
					// console is bound to a HUMAN for the write verbs; a read must not care.
					repo = "example-org/console"
					arc := zipBytes(t, [][2]string{
						{"build.txt", "compiling\x1b[31m ok\x1b[0m\r\nlinking\n"},
						{"test.txt", bigTail()},
						{"build/1_Compile.txt", "compiling ok\n"},
						{"test/1_Run tests.txt", "FAIL: TestThing\n"},
					})
					var got []string
					srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
						got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
						switch {
						case strings.HasSuffix(r.URL.Path, "/actions/runs/501/logs"):
							http.Redirect(rw, r, "/archive/501.zip", http.StatusFound)
						case r.URL.Path == "/archive/501.zip":
							_, _ = rw.Write(arc)
						default:
							rw.WriteHeader(http.StatusNotFound)
						}
					}))
					t.Cleanup(srv.Close)
					forgeAPIBase = srv.URL
					mintTokenFn = func(role, r string) (string, string, error) {
						w.mints++
						return "stub-" + role + "-token", "", nil
					}
					wantAuth = "GET /repos/example-org/console/actions/runs/501/logs token stub-" + tc.role + "-token"
					wire = func() []string { return got }
				case deskkit.ForgeGitLab:
					// ledger is a GitLab repo bound to a HUMAN for the write verbs.
					repo = "example-org/ledger"
					s := newGLRunStub(t)
					s.jobs = []map[string]any{{"id": 70, "name": "build", "status": "success"}, {"id": 71, "name": "test", "status": "failed"}}
					s.traces = map[string]string{"70": "compiling\x1b[31m ok\x1b[0m\r\nlinking\n", "71": bigTail()}
					plantGitlabCustody(t, s.srv.URL, tc.role, deskkit.ReleaseRunnerRole)
					wantAuth = "GET /api/v4/projects/example-org%2Fledger/pipelines/501/jobs " + glPlaceholder(tc.role)
					wire = func() []string { return s.requests }
				}
				code, out, msg := runArgs(t, "log", repo, "501")
				if code != deskkit.ExitOK {
					t.Fatalf("exit %d: %s", code, msg)
				}
				if len(w.logRoles) != 1 || w.logRoles[0] != tc.role {
					t.Fatalf("resolver roles=%v, want exactly one resolution as %q", w.logRoles, tc.role)
				}
				reqs := wire()
				if len(reqs) == 0 || reqs[0] != wantAuth {
					t.Fatalf("first forge request %q, want %q (all: %q)", reqs, wantAuth, reqs)
				}
				for _, r := range reqs {
					if strings.Contains(r, deskkit.ReleaseRunnerRole) {
						t.Fatalf("a log request carried the release-runner credential: %q", r)
					}
				}
				for _, want := range []string{"===== build", "compiling ok", "linking", "===== test", "(truncated: the last", "FAIL: TestThing"} {
					if !strings.Contains(out, want) {
						t.Errorf("output lacks %q", want)
					}
				}
				if n := strings.Count(out, "====="); n != 4 {
					t.Errorf("want exactly two job sections (4 rule marks), got %d — a job printed more than once?", n)
				}
				if strings.ContainsAny(out, "\x1b\r\x07") {
					t.Errorf("terminal-active bytes survived into the output")
				}
			})
		}
	}
}

// TestDeskrunRetrySucceedsOnAppRoster — Verify row 4. With the repo bound to the dedicated
// release-runner credential, `retry` reaches the REAL backend through the real resolver under the
// release-runner credential (never the log seam) on both forges: GitHub one POST to
// rerun-failed-jobs; GitLab one POST per FAILED job and none for a job that passed. --dry-run
// reaches nothing.
func TestDeskrunRetrySucceedsOnAppRoster(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		w := plantWorld(t, deskkit.ForgeGitHub)
		var got []string
		srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			got = append(got, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
			rw.WriteHeader(http.StatusCreated)
		}))
		t.Cleanup(srv.Close)
		forgeAPIBase = srv.URL
		forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
			return deskkit.ResolveForge(fr, deskkit.ReleaseRunnerRole)
		}
		mintTokenFn = func(role, repo string) (string, string, error) {
			w.mints++
			return "stub-" + role + "-token", "", nil
		}
		code, out, msg := runArgs(t, "retry", "example-org/tracker", "9001")
		want := []string{"POST /repos/example-org/tracker/actions/runs/9001/rerun-failed-jobs token stub-release-runner-token"}
		if code != deskkit.ExitOK || strings.Join(got, "|") != strings.Join(want, "|") || len(w.logRoles) != 0 {
			t.Fatalf("exit %d (%s) wire=%q log-seam=%v, want exactly %q", code, msg, got, w.logRoles, want)
		}
		if !strings.Contains(out, "run 9001") {
			t.Errorf("output does not name the run: %q", out)
		}
	})
	t.Run("gitlab", func(t *testing.T) {
		w := plantWorld(t, deskkit.ForgeGitLab)
		s := newGLRunStub(t)
		s.jobs = []map[string]any{
			{"id": 70, "name": "build", "status": "failed"},
			{"id": 71, "name": "lint", "status": "success"},
			{"id": 72, "name": "test", "status": "failed"},
		}
		plantGitlabCustody(t, s.srv.URL, deskkit.ReleaseRunnerRole, "worker")
		forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
			return deskkit.ResolveForge(fr, deskkit.ReleaseRunnerRole)
		}
		code, out, msg := runArgs(t, "retry", "example-org/platform", "9001")
		tok := glPlaceholder(deskkit.ReleaseRunnerRole)
		want := []string{
			"GET /api/v4/projects/example-org%2Fplatform/pipelines/9001/jobs " + tok,
			"POST /api/v4/projects/example-org%2Fplatform/jobs/70/retry " + tok,
			"POST /api/v4/projects/example-org%2Fplatform/jobs/72/retry " + tok,
		}
		if code != deskkit.ExitOK || strings.Join(s.requests, "|") != strings.Join(want, "|") || len(w.logRoles) != 0 {
			t.Fatalf("exit %d (%s) wire=%q log-seam=%v, want exactly %q", code, msg, s.requests, w.logRoles, want)
		}
		if !strings.Contains(out, "run 9001") {
			t.Errorf("output does not name the run: %q", out)
		}
	})
	w := plantWorld(t, deskkit.ForgeGitHub)
	code, out, _ := runArgs(t, "retry", "example-org/tracker", "9001", "--dry-run")
	if code != deskkit.ExitOK || !strings.HasPrefix(out, "dry-run:") || w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 {
		t.Fatalf("--dry-run: exit %d out=%q fake=%d resolver=%d mints=%d", code, out, w.fake.calls(), w.forgeCalls, w.mints)
	}
}

// TestDeskrunLogLeavesBudget — review finding log-reads-charge-write-budget. Reading logs (well
// past the per-PR write budget's count) must leave the write verbs' budget untouched: a retry
// after 25 successful reads on the same repo still runs.
func TestDeskrunLogLeavesBudget(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	nowFunc = time.Now // the meter reads wall time; the ledger must be stamped on the same clock
	for i := 0; i < 25; i++ {
		if code, _, msg := runArgs(t, "log", "example-org/tracker", "501"); code != deskkit.ExitOK {
			t.Fatalf("read %d: exit %d: %s", i+1, code, msg)
		}
	}
	code, _, msg := runArgs(t, "retry", "example-org/tracker", "9001")
	if code != deskkit.ExitOK || len(w.fake.retries) != 1 {
		t.Fatalf("retry after 25 log reads: exit %d (%s), retries %d — a read spent the write budget", code, msg, len(w.fake.retries))
	}
}

// readVerbsUnderTest is the class guard's verb list: every deskrun verb that only READS.
var readVerbsUnderTest = []string{"status", "log"}

// TestDeskrunReadsChargeNothing — the class guard for log-reads-charge-write-budget. Every read
// verb, on every path it has (success, a read failure at the forge, a custody failure, a
// refusal), records NOTHING in the ledger bucket the write verbs' budget and breaker are metered
// from (toolName) — its lines go to deskkit.DeskrunReadTool — so no read, failed or not, is ever
// counted as a charged write or as a writer's non-progress. readledger_test.go carries the
// breaker half of the class.
func TestDeskrunReadsChargeNothing(t *testing.T) {
	type path struct {
		name  string
		setup func(w *world)
	}
	paths := []path{
		{"success", func(w *world) {}},
		{"forge-read-fails", func(w *world) { w.fake.fail = errors.New("the forge answered 502") }},
		{"custody-fails", func(w *world) {
			fail := func() (deskkit.Forge, deskkit.ForgeResolution, error) {
				return nil, deskkit.ForgeResolution{}, errors.New("no token provisioned")
			}
			forgeForFn = func(deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) { return fail() }
			logForgeFn = func(deskkit.ForgeRepo, string) (deskkit.Forge, deskkit.ForgeResolution, error) { return fail() }
		}},
	}
	for _, verb := range readVerbsUnderTest {
		for _, p := range paths {
			t.Run(verb+"-"+p.name, func(t *testing.T) {
				w := plantWorld(t, deskkit.ForgeGitHub)
				p.setup(w)
				for i := 0; i < 3; i++ {
					runArgs(t, verb, "example-org/tracker", "501")
				}
				if bad := ledgerResults(t, toolName); len(bad) > 0 {
					t.Fatalf("%s/%s recorded line(s) %v in the write verbs' bucket — a read reached the write meters", verb, p.name, bad)
				}
			})
		}
	}
	// The refusal paths stay recorded — in the read bucket, never the write one: a role outside
	// the read grant, and (status) a human-bound repo.
	plantWorld(t, deskkit.ForgeGitHub)
	setLoop(t, "verify-desk")
	if code, _, _ := runArgs(t, "log", "example-org/tracker", "501"); code != deskkit.ExitRefused {
		t.Fatalf("a refused log read: exit %d, want %d", code, deskkit.ExitRefused)
	}
	setLoop(t, "worker-desk")
	if code, _, _ := runArgs(t, "status", "example-org/console", "501"); code != deskkit.ExitRefused {
		t.Fatalf("a refused status read: exit %d, want %d", code, deskkit.ExitRefused)
	}
	if got := ledgerResults(t, deskkit.DeskrunReadTool); strings.Join(got, ",") != "log:refused,status:refused" {
		t.Fatalf("read bucket after two refused reads = %v, want exactly the two refusals", got)
	}
	if got := ledgerResults(t, toolName); len(got) > 0 {
		t.Fatalf("write bucket after two refused reads = %v, want nothing", got)
	}
}

// ledgerResults returns "verb:result" for every line recorded under the tool key `tool` in this
// test's ledger, sorted.
func ledgerResults(t *testing.T, tool string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e struct{ Tool, Verb, Result string }
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("ledger line %q: %v", sc.Text(), err)
		}
		if e.Tool == tool {
			out = append(out, e.Verb+":"+e.Result)
		}
	}
	sort.Strings(out)
	return out
}

// zipBytes builds an in-memory archive, entries in the given order.
func zipBytes(t *testing.T, files [][2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := zw.Create(f[0])
		if err != nil {
			t.Fatalf("zip entry %q: %v", f[0], err)
		}
		_, _ = fw.Write([]byte(f[1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}
