package main

// deskrun_test.go — the verb's suite (forge-neutral brief 14 Verify rows 7, 8, 9 and the
// row-12 mutation target).
//
// Two instruments. A RECORDING fake forge drives the positive dispatch/approve cases and every
// refusal, so "zero forge calls" is asserted against what the fake saw. The custody cases
// instead drive the REAL resolver (deskkit.ResolveForge) against a counting httptest server,
// so the refusal they assert is the resolver's / backend's own, not a double's. Nothing here
// reaches a live forge, and no case runs a real mint.

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeForge records every call. The embedded nil deskkit.Forge PANICS on any method deskrun
// was not expected to reach.
type fakeForge struct {
	deskkit.Forge
	runs      []deskkit.RunWorkflowInput
	approvals []approval
	statuses  []deskkit.RunRef
	retries   []deskkit.RunRef
	logs      []deskkit.RunRef
	fail      error // when set, the read ops (RunStatus, RunLog) answer it
}

type approval struct {
	run deskkit.RunRef
	in  deskkit.ApproveGateInput
}

func (f *fakeForge) RunWorkflow(fr deskkit.ForgeRepo, in deskkit.RunWorkflowInput) (deskkit.RunRef, error) {
	f.runs = append(f.runs, in)
	return deskkit.RunRef{ID: "4242", URL: "https://example/runs/4242"}, nil
}

func (f *fakeForge) ApproveGate(fr deskkit.ForgeRepo, run deskkit.RunRef, in deskkit.ApproveGateInput) error {
	f.approvals = append(f.approvals, approval{run, in})
	return nil
}

func (f *fakeForge) RunStatus(fr deskkit.ForgeRepo, run deskkit.RunRef) (*deskkit.RunState, error) {
	f.statuses = append(f.statuses, run)
	if f.fail != nil {
		return nil, f.fail
	}
	return &deskkit.RunState{Status: deskkit.RunStatusWaiting}, nil
}

func (f *fakeForge) RetryRun(fr deskkit.ForgeRepo, run deskkit.RunRef) error {
	f.retries = append(f.retries, run)
	return nil
}

func (f *fakeForge) RunLog(fr deskkit.ForgeRepo, run deskkit.RunRef) ([]deskkit.RunLogPart, error) {
	f.logs = append(f.logs, run)
	if f.fail != nil {
		return nil, f.fail
	}
	return []deskkit.RunLogPart{
		{Name: "build", Text: "compiling\x1b[31m ok\x1b[0m\r\nlinking\n"},
		{Name: "test", Text: "FAIL: TestThing", Truncated: true},
	}, nil
}

func (f *fakeForge) calls() int {
	return len(f.runs) + len(f.approvals) + len(f.statuses) + len(f.retries) + len(f.logs)
}

// world is one test's isolated environment and its counters.
type world struct {
	fake       *fakeForge
	forgeCalls int      // how many times the resolver seam was asked for a backend
	mints      int      // how many times the GitHub mint seam ran
	logRoles   []string // the roles `log` asked the resolver for, in order
}

// plantWorld isolates HOME (roster fixture, audit log), sets a loop identity, and points the
// resolver seam at a recording fake that reports the given forge kind. No mint runs.
func plantWorld(t *testing.T, kind deskkit.ForgeKind) *world {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "deskrun-test")
	t.Setenv("DESK_LOOP", "worker-desk")

	w := &world{fake: &fakeForge{}}
	oldForge, oldMint, oldNow, oldBase, oldLog := forgeForFn, mintTokenFn, nowFunc, forgeAPIBase, logForgeFn
	forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
		w.forgeCalls++
		return w.fake, deskkit.ForgeResolution{Repo: fr, Kind: kind, Source: "test"}, nil
	}
	logForgeFn = func(fr deskkit.ForgeRepo, role string) (deskkit.Forge, deskkit.ForgeResolution, error) {
		w.forgeCalls++
		w.logRoles = append(w.logRoles, role)
		return w.fake, deskkit.ForgeResolution{Repo: fr, Kind: kind, Source: "test"}, nil
	}
	mintTokenFn = func(role, repo string) (string, string, error) {
		w.mints++
		return "", "", errors.New("the fake world mints nothing")
	}
	nowFunc = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		forgeForFn, mintTokenFn, nowFunc, forgeAPIBase, logForgeFn = oldForge, oldMint, oldNow, oldBase, oldLog
	})
	return w
}

func runArgs(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out bytes.Buffer
	var err error
	switch args[0] {
	case "approve":
		err = cmdApprove(args[1:], &out)
	case "status":
		err = cmdStatus(args[1:], &out)
	case "retry":
		err = cmdRetry(args[1:], &out)
	case "log":
		err = cmdLog(args[1:], &out)
	default:
		err = cmdDispatch(args, &out)
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return deskkit.ExitCodeOf(err), out.String(), msg
}

// TestDeskrunDispatchesThroughBackend — Verify row 7 (dispatch half). A release-runner-bound
// repo dispatches through the resolved backend: exactly ONE RunWorkflow call, carrying the
// workflow, ref, inputs and the release-runner App's actor login, and the run it returned is
// printed.
func TestDeskrunDispatchesThroughBackend(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	code, out, msg := runArgs(t, "example-org/tracker", "release.yml", "--ref", "main", "-f", "version=v1.2.3", "-f", "dry_run=true")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d: %s", code, msg)
	}
	want := []deskkit.RunWorkflowInput{{
		Workflow: "release.yml", Ref: "main",
		Inputs: map[string]string{"version": "v1.2.3", "dry_run": "true"},
		Actor:  "example-release-runner-app[bot]",
	}}
	if !reflect.DeepEqual(w.fake.runs, want) {
		t.Fatalf("RunWorkflow calls = %+v, want exactly %+v", w.fake.runs, want)
	}
	if w.fake.calls() != 1 || w.forgeCalls != 1 {
		t.Fatalf("forge calls=%d resolver calls=%d, want exactly one of each", w.fake.calls(), w.forgeCalls)
	}
	if !strings.Contains(out, "run 4242") || strings.Contains(out, "v1.2.3") {
		t.Fatalf("output must name the run and never echo input VALUES: %q", out)
	}
	t.Logf("dispatched through the recording fake: %s", strings.TrimSpace(out))
}

// TestDeskrunApprovesThroughBackend — Verify row 7 (approve half). The gate is approved through
// the backend with the gate shape the ROSTER declares — manual-job for the GitLab repo, the
// GitHub default (empty → environment) for the GitHub one.
func TestDeskrunApprovesThroughBackend(t *testing.T) {
	t.Run("gitlab-declared-shape", func(t *testing.T) {
		w := plantWorld(t, deskkit.ForgeGitLab)
		code, out, msg := runArgs(t, "approve", "example-org/platform", "9001", "--gate", "deploy-production")
		if code != deskkit.ExitOK {
			t.Fatalf("exit %d: %s", code, msg)
		}
		want := []approval{{deskkit.RunRef{ID: "9001"},
			deskkit.ApproveGateInput{Gate: "deploy-production", Shape: deskkit.GateShapeManualJob}}}
		if !reflect.DeepEqual(w.fake.approvals, want) || w.fake.calls() != 1 {
			t.Fatalf("ApproveGate calls = %+v (total %d), want exactly %+v", w.fake.approvals, w.fake.calls(), want)
		}
		t.Logf("approved through the recording fake: %s", strings.TrimSpace(out))
	})
	t.Run("github", func(t *testing.T) {
		w := plantWorld(t, deskkit.ForgeGitHub)
		code, _, msg := runArgs(t, "approve", "example-org/tracker", "501", "--gate", "production")
		if code != deskkit.ExitOK {
			t.Fatalf("exit %d: %s", code, msg)
		}
		want := []approval{{deskkit.RunRef{ID: "501"}, deskkit.ApproveGateInput{Gate: "production"}}}
		if !reflect.DeepEqual(w.fake.approvals, want) || w.fake.calls() != 1 {
			t.Fatalf("ApproveGate calls = %+v (total %d), want exactly %+v", w.fake.approvals, w.fake.calls(), want)
		}
	})
}

// TestDeskrunRefusesOnHumanBoundCredential — Verify row 8. A repo whose run credential is bound
// to human:<name> is REFUSED (exit 5) naming the human and the repo, for every verb, BEFORE
// anything is minted or resolved: the fake records zero calls, the resolver seam is never
// asked for a backend, and the mint seam never runs — so no ambient gh/glab credential can
// have been read.
func TestDeskrunRefusesOnHumanBoundCredential(t *testing.T) {
	for _, args := range [][]string{
		{"example-org/console", "release.yml", "--ref", "main"},
		{"approve", "example-org/console", "501", "--gate", "production"},
		{"status", "example-org/console", "501"},
	} {
		w := plantWorld(t, deskkit.ForgeGitHub)
		code, _, msg := runArgs(t, args...)
		if code != deskkit.ExitRefused {
			t.Fatalf("%v: exit %d (%s), want %d — a human-bound repo must be refused", args, code, msg, deskkit.ExitRefused)
		}
		for _, want := range []string{"human:ada", "example-org/console", "human action"} {
			if !strings.Contains(msg, want) {
				t.Errorf("%v: refusal does not name %q: %s", args, want, msg)
			}
		}
		if w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 {
			t.Fatalf("%v: forge calls=%d resolver calls=%d mints=%d — a human-bound repo must reach none of them",
				args, w.fake.calls(), w.forgeCalls, w.mints)
		}
		t.Logf("%v refused (exit 5) with zero forge calls: %s", args, msg)
	}
}

// TestDeskrunUnboundIsCouldNotCheck — an UNBOUND repo is a configuration gap (exit 6, naming the
// key), distinct from the deliberate human-bound refusal above, and likewise reaches nothing.
func TestDeskrunUnboundIsCouldNotCheck(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	code, _, msg := runArgs(t, "example-org/agents", "release.yml", "--ref", "main")
	if code != deskkit.ExitUnverifiable || !strings.Contains(msg, deskkit.EnvRunCredentials) {
		t.Fatalf("unbound repo: exit %d (%s), want %d naming %s", code, msg, deskkit.ExitUnverifiable, deskkit.EnvRunCredentials)
	}
	if w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 {
		t.Fatalf("unbound repo reached the forge/resolver/mint: %d/%d/%d", w.fake.calls(), w.forgeCalls, w.mints)
	}
}

// TestDeskrunDryRunMintsNothing — --dry-run stops before the resolver: no mint, no forge call.
func TestDeskrunDryRunMintsNothing(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	code, out, msg := runArgs(t, "example-org/tracker", "release.yml", "--ref", "main", "--dry-run")
	if code != deskkit.ExitOK || !strings.HasPrefix(out, "dry-run:") {
		t.Fatalf("exit %d out=%q msg=%s", code, out, msg)
	}
	if w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 {
		t.Fatalf("--dry-run reached the forge/resolver/mint: %d/%d/%d", w.fake.calls(), w.forgeCalls, w.mints)
	}
}

// countingServer is a forge stand-in that counts every request it receives and answers 404.
func countingServer(t *testing.T) (*httptest.Server, *int64) {
	t.Helper()
	var n int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&n, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

// TestDeskrunRefusesWithoutMintedToken — Verify row 9. A release-runner binding whose custody
// token is ABSENT is refused with ZERO forge requests. This drives the REAL resolver
// (deskkit.ResolveForge → custody), not the fake: the GitHub mint fails or hands back an empty
// token, and the GitLab custody file is not provisioned. The counting server stands where the
// forge would be; it must see nothing.
func TestDeskrunRefusesWithoutMintedToken(t *testing.T) {
	realForgeFor := func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
		return deskkit.ResolveForge(fr, deskkit.ReleaseRunnerRole)
	}
	cases := []struct {
		name string
		mint func(role, repo string) (string, string, error)
		args []string
	}{
		{"github-mint-fails", func(string, string) (string, string, error) {
			return "", "", errors.New("no release-runner-app.pem on the App-credential search path")
		}, []string{"example-org/tracker", "release.yml", "--ref", "main"}},
		{"github-empty-token", func(string, string) (string, string, error) { return "", "", nil },
			[]string{"approve", "example-org/tracker", "501", "--gate", "production"}},
		{"gitlab-no-custody-file", nil, []string{"example-org/platform", "-", "--ref", "main"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			srv, hits := countingServer(t)
			forgeAPIBase = srv.URL
			t.Setenv("GITLAB_API_BASE", srv.URL)
			forgeForFn = realForgeFor
			if tc.mint != nil {
				mintTokenFn = func(role, repo string) (string, string, error) {
					w.mints++
					if role != deskkit.ReleaseRunnerRole {
						t.Fatalf("minted role %q — deskrun must only ever mint %s", role, deskkit.ReleaseRunnerRole)
					}
					return tc.mint(role, repo)
				}
			}
			code, _, msg := runArgs(t, tc.args...)
			if code != deskkit.ExitRefused {
				t.Fatalf("exit %d (%s), want %d — an unminted release-runner credential must refuse", code, msg, deskkit.ExitRefused)
			}
			if got := atomic.LoadInt64(hits); got != 0 {
				t.Fatalf("%d forge request(s) went out with no minted token", got)
			}
			t.Logf("refused (exit 5) with zero forge requests: %s", msg)
		})
	}
}

// TestDeskrunDispatchEndToEnd is the +flow case: verb → deskkit.ResolveForge → the REAL GitHub
// backend → a recorded forge. The mint seam hands over a stub token (no real credential); the
// forge answers the dispatch with 204 and lists the run it "created". The dispatch body and the
// correlation read are the backend's own.
func TestDeskrunDispatchEndToEnd(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") == "" {
			t.Errorf("%s %s carried no Authorization — the minted token did not reach the backend", r.Method, r.URL.Path)
		}
		switch r.Method {
		case http.MethodPost:
			rw.WriteHeader(http.StatusNoContent)
		default:
			fmt.Fprintf(rw, `{"total_count":1,"workflow_runs":[{"id":777,"event":"workflow_dispatch","head_branch":"main",`+
				`"created_at":%q,"html_url":"https://example/runs/777","actor":{"login":"example-release-runner-app[bot]"}}]}`,
				time.Now().UTC().Add(time.Second).Format(time.RFC3339))
		}
	}))
	t.Cleanup(srv.Close)
	forgeAPIBase = srv.URL
	forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
		return deskkit.ResolveForge(fr, deskkit.ReleaseRunnerRole)
	}
	mintTokenFn = func(role, repo string) (string, string, error) {
		w.mints++
		return "stub-release-runner-token", "", nil
	}
	code, out, msg := runArgs(t, "example-org/tracker", "release.yml", "--ref", "main")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d: %s", code, msg)
	}
	want := []string{
		"POST /repos/example-org/tracker/actions/workflows/release.yml/dispatches",
		"GET /repos/example-org/tracker/actions/workflows/release.yml/runs",
	}
	if !reflect.DeepEqual(got, want) || w.mints != 1 || !strings.Contains(out, "run 777") {
		t.Fatalf("wire=%v mints=%d out=%q, want %v, one mint, run 777", got, w.mints, out, want)
	}
}

// setLoop presents a desk loop identity ($DESK_LOOP); the log verb derives its role from it.
func setLoop(t *testing.T, loop string) {
	t.Helper()
	t.Setenv("DESK_LOOP", loop)
}

// TestDeskrunLogRefusesOtherRoles — the read grant is a closed set (worker, reviewer): a session
// acting as another desk role is refused (exit 5) before any resolver call.
func TestDeskrunLogRefusesOtherRoles(t *testing.T) {
	for _, loop := range []string{"verify-desk", "the-desk", "intake-desk"} {
		w := plantWorld(t, deskkit.ForgeGitHub)
		setLoop(t, loop)
		code, _, msg := runArgs(t, "log", "example-org/tracker", "501")
		if code != deskkit.ExitRefused {
			t.Fatalf("%s: exit %d (%s), want %d", loop, code, msg, deskkit.ExitRefused)
		}
		if w.forgeCalls != 0 || w.fake.calls() != 0 {
			t.Fatalf("%s: a refused role reached the resolver (%d) / forge (%d)", loop, w.forgeCalls, w.fake.calls())
		}
	}
	w := plantWorld(t, deskkit.ForgeGitHub)
	t.Setenv("DESK_LOOP", "")
	if code, _, _ := runArgs(t, "log", "example-org/tracker", "501"); code != deskkit.ExitRefused || w.forgeCalls != 0 {
		t.Fatalf("no loop identity: exit %d, resolver calls %d, want a refusal with none", code, w.forgeCalls)
	}
	if code, _, _ := runArgs(t, "log", "example-org/tracker", "../runs"); code != deskkit.ExitUnverifiable || w.forgeCalls != 0 {
		t.Fatalf("a path-shaped run id: exit %d, resolver calls %d, want could-not-check with none", code, w.forgeCalls)
	}
}

// TestDeskrunLogEndToEnd — +flow through the REAL resolver and GitHub backend: the token the
// WORKER role minted is what reaches the forge (not the release-runner's), the logs endpoint
// redirects to an archive, and the job entries print.
func TestDeskrunLogEndToEnd(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	setLoop(t, "pr-review-desk")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"1_build.txt": "build ok\n", "2_test.txt": "boom\n"} {
		f, _ := zw.Create(name)
		f.Write([]byte(body))
	}
	zw.Close()
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/actions/runs/501/logs") {
			auth = append(auth, r.Header.Get("Authorization"))
			http.Redirect(rw, r, "/archive/501.zip", http.StatusFound)
			return
		}
		if r.URL.Path == "/archive/501.zip" {
			rw.Write(buf.Bytes())
			return
		}
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	forgeAPIBase = srv.URL
	logForgeFn = func(fr deskkit.ForgeRepo, role string) (deskkit.Forge, deskkit.ForgeResolution, error) {
		w.logRoles = append(w.logRoles, role)
		return deskkit.ResolveForge(fr, role)
	}
	mintTokenFn = func(role, repo string) (string, string, error) {
		w.mints++
		return "stub-" + role + "-token", "", nil
	}
	code, out, msg := runArgs(t, "log", "example-org/tracker", "501")
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d: %s", code, msg)
	}
	if len(auth) != 1 || auth[0] != "token stub-reviewer-token" || w.mints != 1 {
		t.Fatalf("logs request auth=%v mints=%d, want one request bearing the reviewer role's token", auth, w.mints)
	}
	for _, want := range []string{"===== 1_build.txt =====", "build ok", "===== 2_test.txt =====", "boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q: %q", want, out)
		}
	}
}

// TestDeskrunRetryRefusesOnHumanRoster — Verify row 3 (negative path). With the repo's run
// credential bound to a HUMAN, `retry` REFUSES (exit 5) before any request: the recording fake
// sees zero calls, the resolver is never asked for a backend, nothing is minted, and the
// refusal names the role and the resolved human — on both forges.
func TestDeskrunRetryRefusesOnHumanRoster(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind deskkit.ForgeKind
		repo string
	}{
		{"github", deskkit.ForgeGitHub, "example-org/console"},
		{"gitlab", deskkit.ForgeGitLab, "example-org/ledger"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := plantWorld(t, tc.kind)
			code, out, msg := runArgs(t, "retry", tc.repo, "501")
			if code != deskkit.ExitRefused {
				t.Fatalf("exit %d (%s), want %d — a human-bound repo must be refused", code, msg, deskkit.ExitRefused)
			}
			for _, want := range []string{deskkit.ReleaseRunnerRole, "human:ada", tc.repo} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal does not name %q: %s", want, msg)
				}
			}
			if w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 || out != "" {
				t.Fatalf("forge calls=%d resolver calls=%d mints=%d out=%q — a human-bound repo must reach none of them",
					w.fake.calls(), w.forgeCalls, w.mints, out)
			}
			t.Logf("refused (exit 5) with zero forge calls: %s", msg)
		})
	}
	// An unbound repo is could-not-check, and reaches nothing either.
	w := plantWorld(t, deskkit.ForgeGitHub)
	if code, _, _ := runArgs(t, "retry", "example-org/agents", "501"); code != deskkit.ExitUnverifiable || w.fake.calls() != 0 || w.forgeCalls != 0 {
		t.Fatalf("unbound repo: exit %d, fake calls %d, resolver calls %d", code, w.fake.calls(), w.forgeCalls)
	}
}

// TestDeskrunRetryEndToEnd — +flow: verb → real resolver → real GitHub backend. The release-runner
// token is minted and exactly one POST to the rerun-failed-jobs route is made.
func TestDeskrunRetryEndToEnd(t *testing.T) {
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
	code, _, msg := runArgs(t, "retry", "example-org/tracker", "777")
	want := []string{"POST /repos/example-org/tracker/actions/runs/777/rerun-failed-jobs token stub-release-runner-token"}
	if code != deskkit.ExitOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("exit %d (%s) wire=%v, want exactly %v", code, msg, got, want)
	}
}

// TestGithubCustodyMintIsClosed — the custody minter serves the release-runner (every run verb)
// and the worker and reviewer roles (log only); any other desk role is refused WITHOUT a mint.
func TestGithubCustodyMintIsClosed(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	mintTokenFn = func(role, repo string) (string, string, error) {
		w.mints++
		return "stub-" + role, "", nil
	}
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	for _, role := range []string{deskkit.ReleaseRunnerRole, "worker", "reviewer"} {
		if tok, _, err := githubCustodyMint(role, fr); err != nil || tok != "stub-"+role {
			t.Errorf("role %q: token %q err %v, want a minted token", role, tok, err)
		}
	}
	w.mints = 0
	for _, role := range []string{"verifier", "desk", "issue-loop", "board-writer", ""} {
		if tok, _, err := githubCustodyMint(role, fr); err == nil || tok != "" {
			t.Errorf("role %q: token %q err %v, want a refusal", role, tok, err)
		}
	}
	if w.mints != 0 {
		t.Errorf("a refused role minted %d token(s); custody must refuse before any mint", w.mints)
	}
}
