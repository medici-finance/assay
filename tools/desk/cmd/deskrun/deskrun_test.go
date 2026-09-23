package main

// deskrun_test.go — the verb's suite (forge-neutral/14 Verify rows 7, 8, 9 and the
// row-12 mutation target).
//
// Two instruments. A RECORDING fake forge drives the positive dispatch/approve cases and every
// refusal, so "zero forge calls" is asserted against what the fake saw. The custody cases
// instead drive the REAL resolver (deskkit.ResolveForge) against a counting httptest server,
// so the refusal they assert is the resolver's / backend's own, not a double's. Nothing here
// reaches a live forge, and no case runs a real mint.

import (
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
	return &deskkit.RunState{Status: deskkit.RunStatusWaiting}, nil
}

func (f *fakeForge) calls() int { return len(f.runs) + len(f.approvals) + len(f.statuses) }

// world is one test's isolated environment and its counters.
type world struct {
	fake       *fakeForge
	forgeCalls int // how many times the resolver seam was asked for a backend
	mints      int // how many times the GitHub mint seam ran
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
	oldForge, oldMint, oldNow, oldBase := forgeForFn, mintTokenFn, nowFunc, forgeAPIBase
	forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
		w.forgeCalls++
		return w.fake, deskkit.ForgeResolution{Repo: fr, Kind: kind, Source: "test"}, nil
	}
	mintTokenFn = func(role, repo string) (string, string, error) {
		w.mints++
		return "", "", errors.New("the fake world mints nothing")
	}
	nowFunc = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { forgeForFn, mintTokenFn, nowFunc, forgeAPIBase = oldForge, oldMint, oldNow, oldBase })
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
