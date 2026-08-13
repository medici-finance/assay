package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	defer rosterCleanup()
	os.Setenv("GH_TOKEN", "test-token")
	os.Unsetenv("GITHUB_TOKEN")
	os.Setenv("DESK_TOOLS_DISABLED", "")
	code := m.Run()
	os.Exit(code)
}

// withMockAPI starts a test HTTP server, sets githubAPIBase to its URL.
func withMockAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := githubAPIBase
	githubAPIBase = srv.URL + "/"
	t.Cleanup(func() { githubAPIBase = orig })
}

// NOTE: there is deliberately no "mock everything" helper here. An earlier one
// redirected EVERY execCommand call to /usr/bin/true, including the check tools
// inside runChecks — so no test in the package ever ran a check, and deleting the
// branch that turns a failing check into a failing run passed the whole suite. Every
// seam below intercepts git/gh ONLY and lets check tools run for real.

// mockGitOnly intercepts git and gh commands (remapping them to /usr/bin/true)
// but lets every other command (including check tools) run via the real
// exec.Command. This lets tests exercise the pass/fail decision in runChecks.
func mockGitOnly(t *testing.T) *[][]string {
	t.Helper()
	return mockGitSeed(t, "")
}

// mockGitSeed is mockGitOnly plus a stand-in for the fetched tree: the `git checkout`
// call is replaced by `sh -c <seed>`, which runGit runs with cmd.Dir set to the
// temp directory the real fetch would have populated. Check tools then run for real
// over that tree, so a test can drive the whole pass/fail decision — including the
// cases where a tool exits successfully having examined nothing.
func mockGitSeed(t *testing.T, seed string) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		if name == "git" && seed != "" && len(args) > 0 && args[0] == "checkout" {
			return old("sh", "-c", seed)
		}
		if name == "git" || name == "gh" {
			return old("/usr/bin/true")
		}
		return old(name, args...)
	}
	t.Cleanup(func() { execCommand = old })
	return calls
}

// advisoryAPI returns a handler serving the advisory/repo/branch endpoints for the
// fixture, with the given advisory state and a checkdef served over the contents API.
func advisoryAPI(state, checkJSON string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/security-advisories/"):
			resp := advisoryResponse{
				GHSAID:      "GHSA-1111-2222-3333",
				State:       state,
				PrivateFork: &forkInfo{FullName: "example-org/example-k8s-advisory-fork"},
			}
			b, _ := json.Marshal(resp)
			w.Write(b)
		case strings.Contains(r.URL.Path, "/contents/.deskadvisory.json"):
			if checkJSON == "" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"Not Found"}`))
				return
			}
			resp := struct {
				Content  string `json:"content"`
				Encoding string `json:"encoding"`
			}{
				Content:  base64.StdEncoding.EncodeToString([]byte(checkJSON)),
				Encoding: "base64",
			}
			b, _ := json.Marshal(resp)
			w.Write(b)
		case strings.Contains(r.URL.Path, "/branches/"):
			resp := branchResponse{}
			resp.Commit.SHA = "abc123def456"
			b, _ := json.Marshal(resp)
			w.Write(b)
		default:
			resp := repoResponse{Private: true, DefaultBranch: "main"}
			b, _ := json.Marshal(resp)
			w.Write(b)
		}
	}
}

// --- Row 7 mutation guard: repo admission is tested ---

// The fixture slug must be OUTSIDE the allowed set. Since the admission rule widened to an
// org-default, `medici-finance/not-in-allowlist` is ADMITTED (that is the widening), and
// this test would sail past the guard into a live API call. deskadvisory itself makes no
// remote write, so — like deskgit — it needs no public-repo gate; only its admission
// fixture had to move.
func TestCheckAdvisory_RefusesNonAllowlistedRepo(t *testing.T) {
	err := checkAdvisory("attacker/not-in-allowlist", "GHSA-1111-2222-3333")
	if err == nil {
		t.Fatal("expected error for non-allowlisted repo")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("expected IsRefused error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "not in the desk-tools repo set") {
		t.Fatalf("error should name the refusal and repo, got: %v", err)
	}
}

// --- Row 8 mutation guard: advisory-resolution error path is tested ---

func TestCheckAdvisory_RefusesOnUnresolvableAdvisory(t *testing.T) {
	withMockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	})

	err := checkAdvisory("example-org/example-k8s", "GHSA-0000-0000-0000")
	if err == nil {
		t.Fatal("expected error for unresolvable advisory")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("expected IsRefused error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "advisory could not be resolved or access was denied") {
		t.Fatalf("error should say advisory could not be resolved or access was denied, got: %v", err)
	}
}

// --- Happy-path test ---

// TestCheckAdvisory_SuccessfulFullPipeline drives the whole pipeline over a seeded
// tree with a check that really runs (`grep` for a pattern the tree does not
// contain). The advisory state is "draft" — the state a temporary private fork
// actually exists in.
func TestCheckAdvisory_SuccessfulFullPipeline(t *testing.T) {
	checkJSON := `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rnE","FORBIDDEN","scripts"],` +
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`
	withMockAPI(t, advisoryAPI("draft", checkJSON))
	mockGitSeed(t, "mkdir -p scripts && printf 'clean\\n' > scripts/a.sh")

	if err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Edge cases ---

func TestCheckAdvisory_RefusesNonPrivateFork(t *testing.T) {
	withMockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/security-advisories/"):
			resp := advisoryResponse{
				GHSAID: "GHSA-1111-2222-3333",
				State:  "draft",
				PrivateFork: &forkInfo{
					FullName: "example-org/example-k8s-advisory-fork",
				},
			}
			b, _ := json.Marshal(resp)
			w.Write(b)
		default:
			resp := repoResponse{Private: false, DefaultBranch: "main"}
			b, _ := json.Marshal(resp)
			w.Write(b)
		}
	})

	err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333")
	if err == nil {
		t.Fatal("expected error for non-private fork")
	}
	if !strings.Contains(err.Error(), "not private") {
		t.Fatalf("error should mention 'not private', got: %v", err)
	}
}

func TestCheckAdvisory_RefusesAdvisoryWithNoFork(t *testing.T) {
	withMockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		resp := advisoryResponse{GHSAID: "GHSA-1111-2222-3333", State: "draft"}
		b, _ := json.Marshal(resp)
		w.Write(b)
	})

	err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333")
	if err == nil {
		t.Fatal("expected error for advisory with no private fork")
	}
	if !deskkit.IsRefused(err) {
		t.Fatalf("expected IsRefused error, got %T: %v", err, err)
	}
}

// --- Env scrubbing ---

func TestScrubbedEnv_DropsGitAndDangerous(t *testing.T) {
	parent := []string{
		"PATH=/usr/bin", "HOME=/home/x", "SSH_AUTH_SOCK=/tmp/agent", "LC_ALL=C",
		"GIT_SSH_COMMAND=evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=remote.origin.uploadpack",
		"GIT_CONFIG_VALUE_0=pwn", "GIT_ASKPASS=evil", "GIT_DIR=/elsewhere",
		"TOTALLY_UNRELATED_VAR=x",
	}
	got := scrubbedEnv(parent)
	kept := map[string]bool{}
	for _, kv := range got {
		kept[strings.SplitN(kv, "=", 2)[0]] = true
	}
	for _, k := range []string{"PATH", "HOME", "SSH_AUTH_SOCK", "LC_ALL"} {
		if !kept[k] {
			t.Errorf("scrubbedEnv dropped allowlisted %q", k)
		}
	}
	for _, k := range []string{"GIT_SSH_COMMAND", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0", "GIT_ASKPASS", "GIT_DIR", "TOTALLY_UNRELATED_VAR"} {
		if kept[k] {
			t.Errorf("scrubbedEnv KEPT non-allowlisted %q", k)
		}
	}
	if !kept["GIT_TERMINAL_PROMPT"] {
		t.Error("scrubbedEnv must set GIT_TERMINAL_PROMPT=0")
	}
}

// --- Hardening pin presence ---

func TestFetchHardeningPinsPresent(t *testing.T) {
	if len(fetchHardening) != 3 {
		t.Fatalf("fetchHardening has %d entries, want 3", len(fetchHardening))
	}
	joined := strings.Join(fetchHardening, " ")
	for _, want := range []string{"--refmap=", "--upload-pack=git-upload-pack", "--no-recurse-submodules"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("fetchHardening %q missing %q", joined, want)
		}
	}
}

// --- Check list parsing ---

func TestParseCheckList(t *testing.T) {
	valid := []byte(`{"version":1,"checks":[{"name":"test","tool":"echo","args":["hello"],` +
		`"requireFiles":["x"],"requireOutputMatch":"hello"}]}`)
	cl, err := parseCheckList(valid)
	if err != nil {
		t.Fatalf("parseCheckList valid: %v", err)
	}
	if cl.Version != 1 {
		t.Fatalf("version = %d, want 1", cl.Version)
	}
	if len(cl.Checks) != 1 {
		t.Fatalf("checks len = %d, want 1", len(cl.Checks))
	}
	if cl.Checks[0].MinFiles != 1 {
		t.Fatalf("minFiles default = %d, want 1", cl.Checks[0].MinFiles)
	}

	_, err = parseCheckList([]byte(`{"version":2,"checks":[{"name":"t","tool":"e","args":[]}]}`))
	if err == nil || !strings.Contains(err.Error(), "unsupported check list version") {
		t.Fatalf("expected version error, got: %v", err)
	}

	_, err = parseCheckList([]byte(`{"version":1,"checks":[]}`))
	if err == nil || !strings.Contains(err.Error(), "no check") {
		t.Fatalf("expected no-checks error, got: %v", err)
	}

	_, err = parseCheckList([]byte(`{"version":1,"checks":[{"name":"t","tool":"","args":[]}]}`))
	if err == nil || !strings.Contains(err.Error(), "no tool") {
		t.Fatalf("expected no-tool error, got: %v", err)
	}
}

// --- is404 ---

func TestIs404(t *testing.T) {
	err := &ghAPIError{StatusCode: 404, Path: "x"}
	if !is404(err) {
		t.Fatal("is404 should be true for 404 status")
	}
	if is404(nil) {
		t.Fatal("is404 should be false for nil")
	}
	err2 := &ghAPIError{StatusCode: 403, Path: "x"}
	if is404(err2) {
		t.Fatal("is404 should be false for 403 status")
	}
}

// --- ghAPIError ---

func TestGhAPIError(t *testing.T) {
	e := &ghAPIError{StatusCode: 404, Body: "Not Found", Path: "repos/x"}
	if !strings.Contains(e.Error(), "returned 404") {
		t.Fatalf("Error() should contain 'returned 404', got: %s", e.Error())
	}
	if !strings.Contains(e.Error(), "Not Found") {
		t.Fatalf("Error() should contain body, got: %s", e.Error())
	}
}

// --- main/run tests ---

func TestRun_NoArgsRefused(t *testing.T) {
	if code := run(nil); code != deskkit.ExitRefused {
		t.Fatalf("no-args exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestRun_UnknownSubcommandRefused(t *testing.T) {
	if code := run([]string{"unknown"}); code != deskkit.ExitRefused {
		t.Fatalf("unknown subcommand exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestRun_CheckBadArgsRefused(t *testing.T) {
	if code := run([]string{"check"}); code != deskkit.ExitRefused {
		t.Fatalf("check no-args exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if code := run([]string{"check", "a", "b", "c"}); code != deskkit.ExitRefused {
		t.Fatalf("check too-many-args exit = %d, want %d", code, deskkit.ExitRefused)
	}
}

func TestRun_Help(t *testing.T) {
	if code := run([]string{"--help"}); code != deskkit.ExitOK {
		t.Fatalf("--help exit = %d, want %d", code, deskkit.ExitOK)
	}
}

func TestRun_Version(t *testing.T) {
	if code := run([]string{"--version"}); code != deskkit.ExitOK {
		t.Fatalf("--version exit = %d, want %d", code, deskkit.ExitOK)
	}
}

// --- Failing check (mutation guard: delete if-runErr-return → survives) ---

func TestCheckAdvisory_CheckFailureReported(t *testing.T) {
	// A check that will fail (sh -c 'exit 1').
	checkJSON := `{"version":1,"checks":[{"name":"fail-check","tool":"sh","args":["-c","exit 1"],` +
		`"requireFiles":["scripts"],"minFiles":1,"requireOutputMatch":"."}]}`
	withMockAPI(t, advisoryAPI("draft", checkJSON))

	// Intercept git/gh but let check tools (sh) run for real.
	mockGitSeed(t, "mkdir -p scripts && printf 'x\\n' > scripts/a.sh")

	err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333")
	if err == nil {
		t.Fatal("expected error for failing check")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("expected IsUnverifiable error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "checks failed") {
		t.Fatalf("error should contain 'checks failed', got: %v", err)
	}
	if !strings.Contains(err.Error(), "fail-check") {
		t.Fatalf("error should name the failing check, got: %v", err)
	}
}
