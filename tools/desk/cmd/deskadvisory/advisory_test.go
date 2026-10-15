package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// TestMain installs the roster fixture and hands the exit code through finishFixtureRoster
// so the fixture HOME is removed and proven gone before os.Exit — an explicit call, never a
// defer, which os.Exit would skip (#1195).
func TestMain(m *testing.M) {
	rosterCleanup, rerr := installFixtureRoster()
	if rerr != nil {
		panic("cannot install the test-fixture roster: " + rerr.Error())
	}
	os.Setenv("GH_TOKEN", "test-token")
	os.Unsetenv("GITHUB_TOKEN")
	os.Setenv("DESK_TOOLS_DISABLED", "")
	code := gitquiet.Run(m)
	os.Exit(finishFixtureRoster(rosterCleanup, code))
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

// mockGitSeed is mockGitOnly plus a stand-in for the fetched tree: the in-process tree
// fetch (fetchTreeFn) is replaced by `sh -c <seed>` run with the working directory set to
// the temp directory the real fetch would have populated. Check tools then run for real
// over that tree, so a test can drive the whole pass/fail decision — including the
// cases where a tool exits successfully having examined nothing. The recorded calls hold
// every execCommand call plus one ["gitcore","fetch-tree",<url>] entry per tree fetch.
func mockGitSeed(t *testing.T, seed string) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		*calls = append(*calls, append([]string{name}, args...))
		if name == "git" || name == "gh" {
			return old("/usr/bin/true")
		}
		return old(name, args...)
	}
	oldFetch := fetchTreeFn
	fetchTreeFn = func(opts gitcore.TreeOpts, dest string) (gitcore.TreeResult, error) {
		*calls = append(*calls, []string{"gitcore", "fetch-tree", opts.URL})
		if seed == "" {
			return gitcore.TreeResult{}, nil
		}
		cmd := old("sh", "-c", seed)
		cmd.Dir = dest
		if out, err := cmd.CombinedOutput(); err != nil {
			return gitcore.TreeResult{}, fmt.Errorf("seed: %w (%s)", err, out)
		}
		return gitcore.TreeResult{}, nil
	}
	t.Cleanup(func() { execCommand = old; fetchTreeFn = oldFetch })
	return calls
}

// branchSHA, when set, is the head SHA the mock branches endpoint serves (a real fixture
// commit for the tests that run the real in-process fetch).
var branchSHA string

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
			if branchSHA != "" {
				resp.Commit.SHA = branchSHA
			}
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

// --- In-process tree fetch ---

// realFetchTree wires fetchTreeFn to the REAL gitcore.FetchTree, pointed at a local fixture
// repository standing in for the fork, and records the options the production code built
// (the URL and credential it would have sent to the forge) before the redirect.
func realFetchTree(t *testing.T, fork string) *gitcore.TreeOpts {
	t.Helper()
	got := &gitcore.TreeOpts{}
	old := fetchTreeFn
	fetchTreeFn = func(opts gitcore.TreeOpts, dest string) (gitcore.TreeResult, error) {
		*got = opts
		opts.URL = fork
		opts.Auth = nil
		return gitcore.FetchTree(opts, dest)
	}
	t.Cleanup(func() { fetchTreeFn = old })
	return got
}

// A fixture fork, fetched by the real in-process path, yields the same advisory verdict as
// before: the check tool runs over the materialised tree.
func TestCheckAdvisory_FixtureFork_RealInProcessFetch(t *testing.T) {
	checkJSON := `{"version":1,"checks":[{"name":"ck","tool":"grep","args":["-rnE","FORBIDDEN","scripts"],` +
		`"invertExit":true,"requireFiles":["scripts"],"minFiles":1}]}`
	withMockAPI(t, advisoryAPI("draft", checkJSON))
	fork := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(fork.Dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	sha := fork.CommitFile(t, "scripts/a.sh", "clean\n", "fork head")
	branchSHA = sha
	t.Cleanup(func() { branchSHA = "" })
	mockGitOnly(t) // intercepts git/gh only; the tree-fetch seam is rebound next
	got := realFetchTree(t, fork.Dir)

	if err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333"); err != nil {
		t.Fatalf("clean fork tree must pass: %v", err)
	}
	if got.Commit != sha {
		t.Fatalf("fetched commit = %q, want %q", got.Commit, sha)
	}

	// The same fork, now carrying the forbidden pattern, fails the check.
	sha2 := fork.CommitFile(t, "scripts/b.sh", "FORBIDDEN\n", "bad head")
	branchSHA = sha2
	if err := checkAdvisory("example-org/example-k8s", "GHSA-1111-2222-3333"); err == nil {
		t.Fatal("a fork tree carrying the forbidden pattern must fail the advisory check")
	}
}

// The credential goes to the fetch as an in-memory value on the options: it is in no URL,
// no log line, and no file is written for it (no askpass script, no credential store).
func TestFetchAdvisoryTree_TokenInMemoryOnly(t *testing.T) {
	var gotURL string
	var gotAuth transport.AuthMethod
	var gotDest string
	old := fetchTreeFn
	t.Cleanup(func() { fetchTreeFn = old })
	fetchTreeFn = func(opts gitcore.TreeOpts, dest string) (gitcore.TreeResult, error) {
		gotURL, gotAuth, gotDest = opts.URL, opts.Auth, dest
		entries, _ := os.ReadDir(dest)
		if len(entries) != 0 {
			t.Errorf("destination not empty before the fetch: %v", entries)
		}
		return gitcore.TreeResult{}, nil
	}
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "deskadvisory-askpass-*"))

	dir, err := fetchAdvisoryTree("example-org/example-k8s-advisory-fork", strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if want := "https://github.com/example-org/example-k8s-advisory-fork.git"; gotURL != want {
		t.Fatalf("fetch URL = %q, want %q", gotURL, want)
	}
	if strings.Contains(gotURL, "test-token") || strings.Contains(gotURL, "@") {
		t.Fatalf("the credential is in the fetch URL: %q", gotURL)
	}
	ba, ok := gotAuth.(*githttp.BasicAuth)
	if !ok || ba.Password != "test-token" {
		t.Fatalf("the credential did not travel as the in-memory auth value: %#v", gotAuth)
	}
	if gotDest != dir {
		t.Fatalf("fetched into %q, returned %q", gotDest, dir)
	}
	after, _ := filepath.Glob(filepath.Join(os.TempDir(), "deskadvisory-askpass-*"))
	if len(after) != len(before) {
		t.Fatalf("an askpass temp dir was created: %v -> %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("a repository exists in the fetched tree dir: %v", err)
	}
}

// A failed fetch leaves no temp directory behind and is reported.
func TestFetchAdvisoryTree_FailureCleansUp(t *testing.T) {
	var gotDest string
	old := fetchTreeFn
	t.Cleanup(func() { fetchTreeFn = old })
	fetchTreeFn = func(opts gitcore.TreeOpts, dest string) (gitcore.TreeResult, error) {
		gotDest = dest
		return gitcore.TreeResult{}, fmt.Errorf("boom")
	}
	dir, err := fetchAdvisoryTree("example-org/example-k8s-advisory-fork", strings.Repeat("a", 40))
	if err == nil || dir != "" {
		t.Fatalf("want an error and no dir, got %q, %v", dir, err)
	}
	if _, serr := os.Stat(gotDest); !os.IsNotExist(serr) {
		t.Fatalf("temp dir %s survived a failed fetch: %v", gotDest, serr)
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
