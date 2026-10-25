package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// Capture the production factory before TestMain installs its offline default.
// This keeps the factory closure and its resolver path under the existing guard.
var performanceIssueSource = issueSourceFn

// Exercise the production callback; only external startup activation is a fixture.
// The executable test below independently pins main's actual wrapper arguments.
func measuredVerify(t *testing.T, args []string) (int, []perfrec.Record) {
	t.Helper()
	ctx := perfrec.ResolveContext(perfrec.ContextInput{})
	code := deskkit.PerfRunMeasured("verifyloop", verifyPerfVerbs, args, performanceCallback(ctx, func() bool { return true }), verifyPerfPhases...)
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil {
		t.Fatal(err)
	}
	if history.Skipped != 0 {
		t.Fatalf("invalid performance output: %d skipped", history.Skipped)
	}
	return code, history.Records
}

func performanceFixture() verifyPerformance {
	ctx := perfrec.ResolveContext(perfrec.ContextInput{})
	probe := perfrec.SystemProbe()
	return verifyPerformance{session: perfrec.BeginSelected(probe, probe.Mark(), verifyPerfPhases, ctx), context: ctx}
}

func phaseCount(r perfrec.Record, name string) int64 {
	for _, p := range r.Phases {
		if p.Name == name {
			return p.N
		}
	}
	return 0
}
func TestVerifyPerformanceOperationsAndParity(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no")})
	old := preflightRoot
	preflightRoot = func(string) error { return nil }
	t.Cleanup(func() { preflightRoot = old })
	for _, tc := range []struct {
		name string
		args []string
		verb string
		exit int
	}{
		{"plan", []string{"plan", "--root", root, "--json", "--no-forge"}, "plan", 0},
		{"verdict-refusal", []string{"verdict", "--unknown-option"}, "verdict", 5},
		{"dry-run", []string{"--dry-run", "--unknown-option"}, "verdict", 5},
		{"single-dash-dry-run", []string{"-dry-run", "--unknown-option"}, "verdict", 5},
		{"unknown", []string{"private-subject-canary"}, "other", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outputs []string
			for _, on := range []string{"0", "1"} {
				plantAllowedRoster(t, "example-org/tracker")
				t.Setenv("ASSAY_TELEMETRY", on)
				t.Setenv("ASSAY_PERF_PROFILE", "")
				t.Setenv("DESK_ROOTS", "")
				t.Setenv("DESK_LOOP", "verify-desk")
				var code int
				var records []perfrec.Record
				out := captureStdout(t, func() {
					code, records = measuredVerify(t, tc.args)
				})
				outputs = append(outputs, out)
				if code != tc.exit {
					t.Fatalf("recording=%s exit=%d want=%d", on, code, tc.exit)
				}
				if on == "0" {
					if len(records) != 0 {
						t.Fatal("disabled recording wrote a record")
					}
					if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "assay", "perf")); !os.IsNotExist(err) {
						t.Fatalf("disabled recording created state: %v", err)
					}
					continue
				}
				if len(records) != 1 || records[0].Verb != tc.verb || records[0].Schema != perfrec.SchemaV2 {
					t.Fatalf("wrong operation/schema: %+v", records)
				}
				if phaseCount(records[0], "guard") != 1 {
					t.Fatal("missing guard phase")
				}
				if tc.verb == "plan" {
					var doc planDoc
					if err := json.Unmarshal([]byte(out), &doc); err != nil {
						t.Fatalf("JSON stdout polluted: %v", err)
					}
					for _, phase := range []string{"preflight", "queue-scan", "classify", "render"} {
						if phaseCount(records[0], phase) != 1 {
							t.Fatalf("missing %s phase", phase)
						}
					}
				}
				raw, _ := json.Marshal(records)
				if strings.Contains(string(raw), "private-subject-canary") || strings.Contains(string(raw), root) {
					t.Fatal("argv or path leaked")
				}
			}
			if outputs[0] != outputs[1] {
				t.Fatal("recording changed stdout")
			}
		})
	}
}

type verifyRoundTripper func(*http.Request) (*http.Response, error)

func (f verifyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Compose the real scheduling source/backend/counted transport with a fake leaf.
// Removing the invocation observer loses the request/cost assertions below.
func TestVerifyPerformanceSchedulingRequests(t *testing.T) {
	oldSource := issueSourceFn
	issueSourceFn = performanceIssueSource
	t.Cleanup(func() { issueSourceFn = oldSource })
	root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no")})
	gitT(t, root, "init", "-q")
	gitT(t, root, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
	gitT(t, root, "add", ".")
	gitT(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "source")
	if got := deskkit.RepoSlugForDir(root); got != "example-org/tracker" {
		t.Fatalf("fixture repo %q", got)
	}
	stubPreflight(t, func(string) error { return nil })
	for _, profile := range []string{"", "tool", "workflow"} {
		t.Run("profile-"+profile, func(t *testing.T) {
			plantAllowedRoster(t, "example-org/tracker")
			t.Setenv("ASSAY_TELEMETRY", "1")
			t.Setenv("ASSAY_PERF_PROFILE", profile)
			t.Setenv("DESK_ROOTS", "")
			sends := 0
			metadata := false
			// Keep issueSourceFn, resolveForge and ForgeFor intact. Inject only the
			// existing credential and physical transport seams, with the same role/scope.
			roster := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
			raw, err := os.ReadFile(roster)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(roster, append(raw, []byte("ASSAY_REPO_FORGES=example-org/tracker=github\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			deskkit.ReloadConfig()
			mints := 0
			deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (string, string, error) {
				if role != verifyloopRole || repo.Slug() != "example-org/tracker" {
					t.Fatalf("wrong custody: %s %s", role, repo.Slug())
				}
				mints++
				return "fixture", "", nil
			})
			originalTransport := http.DefaultTransport
			http.DefaultTransport = verifyRoundTripper(func(r *http.Request) (*http.Response, error) {
				sends++
				body := `{"default_branch":"main"}`
				switch r.URL.Path {
				case "/graphql":
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatal(err)
					}
					metadata = strings.Contains(string(raw), "_assayPerf")
					body = `{"data":{"repository":{"pullRequests":{"pageInfo":{"hasNextPage":false},"nodes":[]}},"_assayPerf":{"cost":2}}}`
				case "/repos/example-org/tracker":
				default:
					t.Fatalf("unexpected provider path %s", r.URL.Path)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil); http.DefaultTransport = originalTransport })
			var records []perfrec.Record
			var code int
			planOutput := captureStdout(t, func() {
				code, records = measuredVerify(t, []string{"plan", "--root", root, "--json"})
			})
			if code != 0 || sends != 2 || mints != 1 || len(records) != 1 {
				t.Fatalf("exit=%d sends=%d mints=%d records=%d output=%s", code, sends, mints, len(records), planOutput)
			}
			r := records[0]
			if phaseCount(r, "pending-discovery") != 1 || phaseCount(r, "pending-evidence") != 1 {
				t.Fatal("scheduling phases missing")
			}
			if r.Counters[perfrec.CounterHTTP] != 2 {
				t.Fatal("actual attempts not counted")
			}
			if profile == "" {
				if r.V3 != nil || metadata {
					t.Fatal("legacy consent expanded")
				}
				return
			}
			if r.V3 == nil || !metadata || r.V3.RequestCoverage != perfrec.CoverageMissing {
				t.Fatal("observer absent or full inventory falsely claimed")
			}
			var attempts, points int64
			for _, q := range r.V3.Requests {
				attempts += q.Attempts
				if q.Points.Coverage == perfrec.CoverageMeasured {
					points += *q.Points.Value
				}
			}
			if attempts != 2 || points != 2 {
				t.Fatalf("attempts=%d points=%d", attempts, points)
			}
			sum, err := perfrec.ReduceV3(records, perfrec.ProfileTool, "tool", perfrec.V3Filter{})
			if err != nil || len(sum.Requests) != 2 {
				t.Fatalf("record/report flow: %+v %v", sum, err)
			}
		})
	}
}
func TestVerifyPerformancePendingAdmissionParity(t *testing.T) {
	v, f, _, _ := pendingSetup(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "tool")
	baseline, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	source := &forgeIssueSource{forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }}
	v.Issues = source
	p := performanceFixture()
	v.performance, source.performance = p, p
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, baseline) {
		t.Fatal("measurement changed pending admission")
	}
	record := p.session.Finish("verifyloop", "plan", "none", 0, "")
	for phase, n := range map[string]int64{"queue-scan": 1, "pending-discovery": 1, "pending-evidence": 2, "attestation": 1} {
		if phaseCount(record, phase) != n {
			t.Fatalf("%s count=%d want=%d", phase, phaseCount(record, phase), n)
		}
	}
	if !record.PhaseOverlap {
		t.Fatal("inclusive nested phases not marked")
	}
}
func TestVerifyPerformanceVerdictPhases(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "")
	pem, _ := writeTestKey(t)
	cfg := verdictRunConfig{root: demoRoot(t), pem: pem, dryRun: true, exec: fakeExec, now: fixedNow, out: io.Discard}
	p := performanceFixture()
	cfg.performance = p
	if err := runVerdict(cfg); err != nil {
		t.Fatal(err)
	}
	record := p.session.Finish("verifyloop", "verdict", "none", 0, "")
	for _, phase := range []string{"queue-scan", "rows", "sign"} {
		if phaseCount(record, phase) == 0 {
			t.Fatalf("missing %s", phase)
		}
	}
}

// Build and run main itself so changing its roster, phase argument or callback
// cannot be hidden by a test-owned wrapper. Runtime PATH has no external commands,
// the home is disposable, and plan has no Git remote or credential to contact.
func TestVerifyPerformanceMainEntrypoint(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "verifyloop")
	if os.PathSeparator == '\\' {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		verb   string
		exit   int
		phases []string
	}{
		{"help", []string{"--help"}, "other", 0, []string{"startup"}},
		{"plan-envelope", []string{"plan", "--root", t.TempDir(), "--no-forge"}, "plan", 6, []string{"startup", "guard", "preflight"}},
		{"verdict-parse", []string{"verdict", "--unknown-option"}, "verdict", 5, []string{"startup", "guard"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plantAllowedRoster(t, "example-org/tracker")
			var baseline []byte
			for _, enabled := range []string{"0", "1"} {
				cmd := exec.Command(bin, tc.args...)
				cmd.Dir = t.TempDir()
				for _, pair := range os.Environ() {
					key, _, _ := strings.Cut(pair, "=")
					switch key {
					case "HOME", "USERPROFILE", "PATH", "XDG_CONFIG_HOME", "VERIFIER_PEM", "KUBECONFIG":
						continue
					}
					if strings.HasPrefix(key, "ASSAY_") || strings.HasPrefix(key, "DESK_") || strings.HasPrefix(key, "GH_") || strings.HasPrefix(key, "GITHUB_") {
						continue
					}
					cmd.Env = append(cmd.Env, pair)
				}
				cmd.Env = append(cmd.Env, "HOME="+os.Getenv("HOME"), "USERPROFILE="+os.Getenv("HOME"), "PATH="+filepath.Dir(bin), "KUBECONFIG=/dev/null", "DESK_LOOP=verify-desk", "ASSAY_TELEMETRY="+enabled, "ASSAY_PERF_PROFILE=tool", "ASSAY_CONFIG_ECHO=full")
				out, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.exit {
					t.Fatalf("exit: %v\n%s", cmd.ProcessState, out)
				}
				history, err := deskkit.LoadPerfHistory(1)
				if err != nil || history.Skipped != 0 {
					t.Fatalf("history: %v skipped=%d", err, history.Skipped)
				}
				if enabled == "0" {
					baseline = out
					if len(history.Records) != 0 {
						t.Fatal("main wrote telemetry without consent")
					}
					continue
				}
				if string(out) != string(baseline) {
					t.Fatalf("main recording changed output\noff:\n%s\non:\n%s", baseline, out)
				}
				if len(history.Records) != 1 {
					t.Fatalf("main records: %d", len(history.Records))
				}
				r := history.Records[0]
				if r.Schema != perfrec.SchemaV3 || r.Verb != tc.verb || r.V3 == nil || r.V3.Profile != perfrec.ProfileTool {
					t.Fatalf("main attribution: %+v", r)
				}
				for _, phase := range tc.phases {
					if phaseCount(r, phase) != 1 {
						t.Fatalf("main %s phase missing: %+v", phase, r.Phases)
					}
				}
			}
		})
	}
}

func TestVerifyPerformanceBaseAndMultiRootPhases(t *testing.T) {
	plantFixtureRoster(t)
	a, b := twoRoots(t)
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "tool")
	t.Setenv(deskkit.RootsEnv, "example-org/tracker="+a+",example-org/agents="+b)
	stubPreflight(t, func(string) error { return nil })
	serveBase(t, map[string][]baseRow{
		a: {{Brief: "stream-a/01", QueueVersion: 1, QueueClass: "judgment-dispatch", QueueValid: true}},
		b: {{Brief: "stream-b/01", QueueVersion: 1, QueueClass: "judgment-dispatch", QueueValid: true}},
	})
	var records []perfrec.Record
	var code int
	captureStdout(t, func() { code, records = measuredVerify(t, []string{"plan", "--json", "--no-forge", "--base-queue"}) })
	if code != 0 || len(records) != 1 {
		t.Fatalf("exit %d records %d", code, len(records))
	}
	for phase, n := range map[string]int64{"startup": 1, "guard": 1, "preflight": 3, "base-queue": 2, "queue-scan": 1, "classify": 1, "render": 1} {
		if phaseCount(records[0], phase) != n {
			t.Fatalf("%s phase count=%d want=%d", phase, phaseCount(records[0], phase), n)
		}
	}
}

func TestVerifyPerformanceBlockerPhase(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	p := performanceFixture()
	var calls []string
	source := &forgeIssueSource{performance: p, forgeFor: func(deskkit.ForgeRepo) (deskkit.Forge, error) { return stateForge{state: "open", calls: &calls}, nil }}
	ref, err := deskkit.ParseBlockerRef("example-org/tracker#99", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if state, why := source.IssueState(ref); state != deskkit.BlockerOpen {
		t.Fatalf("%v %s", state, why)
	}
	r := p.session.Finish("verifyloop", "plan", "none", 0, "")
	if phaseCount(r, "blocker") != 1 || len(calls) != 1 {
		t.Fatalf("blocker phase/calls %+v %v", r.Phases, calls)
	}
}

func TestVerifyPerformanceUnsignedRowsPhase(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	p := performanceFixture()
	cfg := verdictRunConfig{performance: p, root: demoRoot(t), unsignedOut: filepath.Join(t.TempDir(), "payload.json"), exec: fakeExec, now: fixedNow, out: io.Discard}
	if err := runVerdict(cfg); err != nil {
		t.Fatal(err)
	}
	r := p.session.Finish("verifyloop", "verdict", "none", 0, "")
	if phaseCount(r, "queue-scan") != 1 || phaseCount(r, "rows") == 0 || phaseCount(r, "sign") != 0 {
		t.Fatalf("unsigned phases %+v", r.Phases)
	}
}

// A hook may export GIT_DIR and related variables while running this suite.
// Fixture setup must write only its own repository, including config and index.
func TestVerifyPerformanceGitFixtureIsolation(t *testing.T) {
	victim := regression.HostileGitDir(t)
	before := regression.SnapshotTree(t, victim)
	fixture := t.TempDir()
	gitT(t, fixture, "init", "-q")
	gitT(t, fixture, "remote", "add", "origin", "https://github.com/example-org/tracker.git")
	if err := os.WriteFile(filepath.Join(fixture, "fixture.txt"), []byte("fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	gitT(t, fixture, "add", ".")
	gitT(t, fixture, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	if changes := before.Changes(regression.SnapshotTree(t, victim)); changes != "" {
		t.Fatalf("fixture modified caller repository:\n%s", changes)
	}
	if _, err := os.Stat(filepath.Join(fixture, ".git", "HEAD")); err != nil {
		t.Fatalf("fixture repository missing: %v", err)
	}
}

// One provider observation serves repeated logical blocker references in a pass.
// Production factory, custody and counted transport are intact; only the leaf is
// an offline fixture. A new pass must observe a changed provider state.
func TestVerifyPerformanceBlockerFanIn(t *testing.T) {
	plantAllowedRoster(t, "example-org/tracker")
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("ASSAY_PERF_PROFILE", "tool")
	roster := filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env")
	raw, err := os.ReadFile(roster)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(roster, append(raw, []byte("ASSAY_REPO_FORGES=example-org/tracker=github\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	deskkit.SetGitHubCustodyMinter(func(role string, r deskkit.ForgeRepo) (string, string, error) {
		if role != verifyloopRole || r.Slug() != "example-org/tracker" {
			t.Fatal("wrong role/repository")
		}
		return "fixture", "", nil
	})
	oldTransport := http.DefaultTransport
	sends := 0
	http.DefaultTransport = verifyRoundTripper(func(r *http.Request) (*http.Response, error) {
		sends++
		if r.URL.Path != "/repos/example-org/tracker/issues/7" {
			t.Fatalf("unexpected provider path %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"number":7,"state":"open"}`))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport; deskkit.SetGitHubCustodyMinter(nil) })
	ctx := perfrec.ResolveContext(perfrec.ContextInput{})
	code := deskkit.PerfRunMeasured("verifyloop", verifyPerfVerbs, []string{"plan"}, func(_ []string, session *perfrec.Session) (int, perfrec.Context) {
		session.SelectContext(ctx)
		source := performanceIssueSource().(*forgeIssueSource)
		source.performance = verifyPerformance{session: session, context: ctx}
		source.observer = deskkit.RequestObserverFor(ctx, session, perfrec.OriginDirect)
		for i := 0; i < 100; i++ {
			ref, _ := deskkit.ParseBlockerRef("example-org/tracker#7", "", "")
			if state, why := source.IssueState(ref); state != deskkit.BlockerOpen {
				t.Fatalf("%v %s", state, why)
			}
		}
		return 0, ctx
	}, verifyPerfPhases...)
	history, err := deskkit.LoadPerfHistory(1)
	if err != nil || len(history.Records) != 1 {
		t.Fatalf("perf record: %v", err)
	}
	record := history.Records[0]
	wantRequests := int64(1)
	if code != 0 || int64(sends) != wantRequests || record.Counters[perfrec.CounterHTTP] != wantRequests || phaseCount(record, "blocker") != 100 {
		t.Fatalf("fan-in: exit=%d sends=%d record=%+v", code, sends, record)
	}
	var observed int64
	for _, request := range record.V3.Requests {
		observed += request.Attempts
	}
	if observed != wantRequests {
		t.Fatalf("observer attempts=%d want=%d", observed, wantRequests)
	}
	encoded, _ := json.Marshal(record)
	t.Logf("blocker measurement: %s", encoded)
}
