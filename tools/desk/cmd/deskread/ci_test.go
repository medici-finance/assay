package main

// ci_test.go — the CI workflow-token transport (forge-neutral brief 34): every refusal layer
// tested with every OTHER layer satisfied, so a test goes red only if its own layer is removed.
//
// A refused run must make zero forge requests: each row points the deskkit CI test seam at its own
// httptest server and asserts the server saw nothing. The token is a placeholder, not a credential.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	ciTestToken = "gh" + "s_placeholdertoken0000000000000000"
	ciRepo      = "example-org/alpha"
	ciOtherRepo = "example-org/beta"
	ciIssueBody = `[{"number": 11, "title": "first", "state": "open",
	  "user": {"login": "someone", "id": 42}, "labels": [{"name": "question"}],
	  "created_at": "2026-09-01T00:00:00Z",
	  "html_url": "https://example.test/example-org/alpha/issues/11"}]`
	ciTrustBody = `{"data":{"repository":{"issue":{"lastEditedAt":"2026-09-01T10:00:00Z",
	  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[
	  {"createdAt":"2026-09-02T00:00:00Z","lastEditedAt":null,
	   "author":{"login":"authority","__typename":"User","databaseId":100001}}]}}}}}`
)

// seenReq is one request a test server received.
type seenReq struct{ path, auth string }

// recSrv answers the GitHub issue list for ciRepo and the GraphQL trust query, and records every
// request with its Authorization header.
type recSrv struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []seenReq
}

func newRecSrv(t *testing.T) *recSrv {
	t.Helper()
	s := &recSrv{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, seenReq{r.URL.Path, r.Header.Get("Authorization")})
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/graphql":
			_, _ = io.WriteString(w, ciTrustBody)
		case strings.HasPrefix(r.URL.Path, "/repos/"+ciRepo+"/issues"):
			if r.URL.Query().Get("page") != "1" {
				_, _ = io.WriteString(w, `[]`)
				return
			}
			_, _ = io.WriteString(w, ciIssueBody)
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Server.Close)
	return s
}

func (s *recSrv) seen() []seenReq {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]seenReq(nil), s.reqs...)
}

// custodyCalls counts the custody seams. Under the CI transport both must stay at zero.
type custodyCalls struct{ role, mint int }

// ciSetup puts the process into a CI job for ciRepo with the opt-in token present, points the CI
// backend at srv, resolves ciRepo and ciOtherRepo to GitHub, and swaps the custody seams for
// counters. It returns the counters; everything is restored on cleanup.
func ciSetup(t *testing.T, srv *recSrv) *custodyCalls {
	t.Helper()
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_RUN_ID", "9001")
	t.Setenv("GITHUB_REPOSITORY", ciRepo)
	t.Setenv("GITHUB_EVENT_NAME", "push")
	t.Setenv(ciTokenEnv, ciTestToken)
	t.Setenv("GH_TOKEN", "gh"+"s_ambientdoesnotcount000000000000000")
	t.Setenv("GITHUB_TOKEN", "gh"+"s_ambientdoesnotcount000000000000000")
	t.Setenv(deskkit.EnvRepoForges, ciRepo+"=github,"+ciOtherRepo+"=github")
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	defer deskkit.ReloadConfig()
	t.Cleanup(deskkit.SetCITokenAPIBaseForTest(srv.URL))

	c := &custodyCalls{}
	prevRole, prevMint := sessionRoleFn, mintTokenFn
	sessionRoleFn = func(string) (string, string, error) { c.role++; return "worker-desk", "", nil }
	mintTokenFn = func(string, string) (string, string, error) { c.mint++; return testToken, "", nil }
	t.Cleanup(func() { sessionRoleFn, mintTokenFn = prevRole, prevMint })
	return c
}

// runCI runs the verb capturing stdout and stderr both.
func runCI(args ...string) (stdout, stderr string, code int) {
	var out, errb strings.Builder
	code = run(args, &out, &errb)
	return out.String(), errb.String(), code
}

func wantRefusal(t *testing.T, name, layer string, code int, stdout, stderr string, srv *recSrv) {
	t.Helper()
	if code != deskkit.ExitRefused {
		t.Errorf("%s: exit=%d, want %d; stderr=%s", name, code, deskkit.ExitRefused, stderr)
	}
	if !strings.Contains(stderr, "refused [ci-transport:"+layer+"]") {
		t.Errorf("%s: stderr lacks the [ci-transport:%s] tag: %s", name, layer, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("%s: a refused run wrote to stdout: %s", name, stdout)
	}
	if n := len(srv.seen()); n != 0 {
		t.Errorf("%s: a refused run made %d forge requests, want 0", name, n)
	}
	if strings.Contains(stderr, ciTestToken) || strings.Contains(stdout, ciTestToken) {
		t.Errorf("%s: the token appears in the output", name)
	}
}

// The happy path: flag + CI + token reads the job's own repo with the handed token.
func TestCITransportReadsOwnRepository(t *testing.T) {
	srv := newRecSrv(t)
	c := ciSetup(t, srv)
	stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--ci-workflow-token")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, stderr)
	}
	var env Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if len(env.Repos) != 1 || env.Repos[0].Repo != ciRepo || len(env.Repos[0].Issues) != 1 {
		t.Fatalf("repos=%+v", env.Repos)
	}
	reqs := srv.seen()
	if len(reqs) == 0 {
		t.Fatal("no request reached the server: the success path proves nothing")
	}
	for _, r := range reqs {
		if r.auth != "Bearer "+ciTestToken && r.auth != "token "+ciTestToken {
			t.Errorf("request %s carried Authorization %q, want the CI token", r.path, r.auth)
		}
	}
	if c.role != 0 || c.mint != 0 {
		t.Errorf("custody seams called under the CI transport: role=%d mint=%d, want 0/0", c.role, c.mint)
	}
}

// Layer: opt-in. A set token without the flag is ignored, loudly, and the custody path runs.
func TestCITransportRefusedWithoutFlag(t *testing.T) {
	srv := newRecSrv(t)
	c := ciSetup(t, srv)
	_, stderr, code := runCI("issues", "--repo", ciRepo)
	// The custody path runs: it resolves the role and mints (stubbed), and never uses the CI token.
	if c.role == 0 || c.mint == 0 {
		t.Errorf("the custody path did not run: role=%d mint=%d (exit %d; %s)", c.role, c.mint, code, stderr)
	}
	if !strings.Contains(stderr, ciTokenEnv+" is set but ignored") {
		t.Errorf("stderr lacks the one line naming the ignored variable: %s", stderr)
	}
	if strings.Contains(stderr, ciTestToken) {
		t.Error("the ignored line carries the token value")
	}
	for _, r := range srv.seen() {
		if strings.Contains(r.auth, ciTestToken) {
			t.Errorf("the CI token was sent without the opt-in flag (%s)", r.path)
		}
	}
	if strings.Contains(stderr, "ci-transport") && strings.Contains(stderr, "refused") {
		t.Errorf("the CI gate refused a run that never opted in: %s", stderr)
	}
}

// Layer: outside-ci. Each environment weakness refuses on its own, the others satisfied.
func TestCITransportRefusedOutsideCI(t *testing.T) {
	cases := []struct {
		name, key, val string
		layer          string
	}{
		{"actions unset", "GITHUB_ACTIONS", "", layerOutsideCI},
		{"actions false", "GITHUB_ACTIONS", "false", layerOutsideCI},
		{"run id empty", "GITHUB_RUN_ID", "", layerOutsideCI},
		{"run id not numeric", "GITHUB_RUN_ID", "abc", layerOutsideCI},
		{"repository empty", "GITHUB_REPOSITORY", "", layerOutsideCI},
		{"repository malformed", "GITHUB_REPOSITORY", "noslash", layerOutsideCI},
		{"fork-triggered event", "GITHUB_EVENT_NAME", "pull_request_target", layerEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newRecSrv(t)
			c := ciSetup(t, srv)
			t.Setenv(tc.key, tc.val)
			stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--ci-workflow-token")
			wantRefusal(t, tc.name, tc.layer, code, stdout, stderr, srv)
			if c.role != 0 || c.mint != 0 {
				t.Errorf("custody seams called on a refused CI run: %+v", c)
			}
		})
	}
}

// Layer: kind. A kind absent from ciTransportKinds is refused even though readKinds serves it.
func TestCITransportRefusesUnlistedKind(t *testing.T) {
	srv := newRecSrv(t)
	ciSetup(t, srv)
	for _, k := range []string{"issue-list", "changes", "default-branch"} {
		stdout, stderr, code := runCI(k, "--repo", ciRepo, "--ci-workflow-token")
		if k == "issue-list" || k == "changes" {
			// these need --state; supply it so the kind layer is the only refusal.
			stdout, stderr, code = runCI(k, "--repo", ciRepo, "--state", map[string]string{"issue-list": "open", "changes": "merged"}[k], "--ci-workflow-token")
		}
		wantRefusal(t, k, layerKind, code, stdout, stderr, srv)
	}
	// A kind added to readKinds but not to ciTransportKinds is refused (the closed-set property).
	readKinds["zz-test-only"] = "test-only kind absent from ciTransportKinds"
	defer delete(readKinds, "zz-test-only")
	stdout, stderr, code := runCI("zz-test-only", "--repo", ciRepo, "--ci-workflow-token")
	wantRefusal(t, "zz-test-only", layerKind, code, stdout, stderr, srv)
}

// Every ciTransportKinds entry is a real read kind.
func TestCITransportKindsAreReads(t *testing.T) {
	if len(ciTransportKinds) == 0 {
		t.Fatal("ciTransportKinds is empty")
	}
	for k := range ciTransportKinds {
		if _, ok := readKinds[k]; !ok {
			t.Errorf("ciTransportKinds names %q, which is not a readKinds kind", k)
		}
	}
}

// Layer: token-shape, at the verb and at the constructor.
func TestCITransportRefusesNonInstallationToken(t *testing.T) {
	for _, tok := range []string{"gh" + "p_personalaccesstoken0000", "gi" + "thub_pat_fine0000", "gh" + "o_oauth0000", "ghu_usertouser0000", "unprefixed0000"} {
		srv := newRecSrv(t)
		ciSetup(t, srv)
		t.Setenv(ciTokenEnv, tok)
		stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--ci-workflow-token")
		wantRefusal(t, tok[:4], layerTokenShape, code, stdout, stderr, srv)
		if strings.Contains(stderr, tok) {
			t.Errorf("%s: the refusal carries the token", tok[:4])
		}
	}
	// With the verb bypassed: the constructor re-checks.
	srv := newRecSrv(t)
	ciSetup(t, srv)
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "alpha"}
	if _, _, err := deskkit.ReadOnlyForgeForCIToken(fr, ciRepo, "gh"+"p_personalaccesstoken0000"); err == nil || !deskkit.IsRefused(err) {
		t.Errorf("constructor accepted a personal token: %v", err)
	}
	if n := len(srv.seen()); n != 0 {
		t.Errorf("constructor refusal made %d requests", n)
	}
}

// Layer: repository. Other repos never get the token; per-item kinds refuse outright.
func TestCITransportBindsJobRepository(t *testing.T) {
	srv := newRecSrv(t)
	ciSetup(t, srv)
	stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--repo", ciOtherRepo, "--ci-workflow-token")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, stderr)
	}
	var env Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Repos) != 1 || env.Repos[0].Repo != ciRepo {
		t.Errorf("repos=%+v, want only %s", env.Repos, ciRepo)
	}
	if len(env.Partial) != 1 || env.Partial[0].Repo != ciOtherRepo || !strings.Contains(env.Partial[0].Reason, "only the job's own repository") {
		t.Errorf("partial=%+v, want %s with the binding reason", env.Partial, ciOtherRepo)
	}
	for _, r := range srv.seen() {
		if strings.Contains(r.path, "beta") {
			t.Errorf("the server saw a request for the other repository: %s", r.path)
		}
	}

	srv2 := newRecSrv(t)
	ciSetup(t, srv2)
	stdout, stderr, code = runCI("trust", "--issue", ciOtherRepo+"#7", "--ci-workflow-token")
	wantRefusal(t, "trust other repo", layerRepository, code, stdout, stderr, srv2)

	// With the verb bypassed: the constructor re-checks the binding.
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "beta"}
	if _, _, err := deskkit.ReadOnlyForgeForCIToken(fr, ciRepo, ciTestToken); err == nil || !deskkit.IsRefused(err) {
		t.Errorf("constructor built a backend for another repository: %v", err)
	}
}

// Layer: no-token. The flag alone is not enough; ambient tokens never substitute.
func TestCITransportFlagWithoutTokenRefused(t *testing.T) {
	srv := newRecSrv(t)
	c := ciSetup(t, srv)
	t.Setenv(ciTokenEnv, "")
	stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--ci-workflow-token")
	wantRefusal(t, "no token", layerNoToken, code, stdout, stderr, srv)
	if c.role != 0 || c.mint != 0 {
		t.Errorf("flag without token fell back to custody: %+v", c)
	}
}

// The custody path is unchanged: same seams called once, same bytes minus the additive identity.
func TestCustodyPathUnchangedByCITransport(t *testing.T) {
	srv := newRecSrv(t)
	c := ciSetup(t, srv)
	t.Setenv(ciTokenEnv, "")
	prevBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = prevBase })

	stdout, stderr, code := runCI("issues", "--repo", ciRepo)
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, stderr)
	}
	if c.role != 1 || c.mint != 1 {
		t.Errorf("custody seams called role=%d mint=%d, want exactly 1/1", c.role, c.mint)
	}
	saw := false
	for _, r := range srv.seen() {
		if strings.Contains(r.auth, testToken) {
			saw = true
		}
	}
	if !saw {
		t.Errorf("the server did not see the minted token: %+v", srv.seen())
	}
	if strings.Contains(stdout, testToken) || strings.Contains(stderr, testToken) {
		t.Error("the minted custody token appears in the output")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatal(err)
	}
	id, ok := m["identity"]
	if !ok {
		t.Fatal("custody envelope lacks the identity record")
	}
	if string(compact(t, id)) != `{"role":"worker-desk","transport":"app-custody"}` {
		t.Errorf("custody identity = %s", id)
	}
	delete(m, "identity")
	got, _ := json.Marshal(m)
	const golden = `{"kind":"issues","partial":[],"repos":[{"issues":[{"authorId":42,"authorLogin":"someone","createdAt":"2026-09-01T00:00:00Z","labels":["question"],"number":11,"state":"open","title":"first","url":"https://example.test/example-org/alpha/issues/11"}],"repo":"example-org/alpha"}],"schema":1}`
	var gm, wm any
	_ = json.Unmarshal(got, &gm)
	_ = json.Unmarshal([]byte(golden), &wm)
	g, _ := json.Marshal(gm)
	w, _ := json.Marshal(wm)
	if string(g) != string(w) {
		t.Errorf("the custody envelope minus identity moved:\n got %s\nwant %s", g, w)
	}
}

func compact(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	return b
}

// The envelope and stderr name the transport and never carry the token.
func TestCITransportRecordsIdentityNotToken(t *testing.T) {
	srv := newRecSrv(t)
	ciSetup(t, srv)
	stdout, stderr, code := runCI("issues", "--repo", ciRepo, "--ci-workflow-token")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; %s", code, stderr)
	}
	var env Envelope
	_ = json.Unmarshal([]byte(stdout), &env)
	if env.Identity == nil || env.Identity.Transport != "ci-workflow-token" || env.Identity.Repository != ciRepo || env.Identity.RunID != "9001" || env.Identity.Role != "" {
		t.Errorf("CI identity = %+v", env.Identity)
	}
	if !strings.Contains(stderr, "transport=ci-workflow-token") {
		t.Errorf("stderr does not name the transport: %s", stderr)
	}
	for _, tok := range []string{ciTestToken, "gh" + "s_ambientdoesnotcount000000000000000"} {
		if strings.Contains(stdout, tok) || strings.Contains(stderr, tok) {
			t.Errorf("a token appears in the output")
		}
	}
	// Per-item kind too.
	stdout, stderr, code = runCI("trust", "--issue", ciRepo+"#7", "--ci-workflow-token")
	if code != deskkit.ExitOK {
		t.Fatalf("trust exit=%d; %s", code, stderr)
	}
	var ie ItemEnvelope
	_ = json.Unmarshal([]byte(stdout), &ie)
	if ie.Identity == nil || ie.Identity.Transport != "ci-workflow-token" || ie.Identity.RunID != "9001" {
		t.Errorf("CI per-item identity = %+v", ie.Identity)
	}
	if strings.Contains(stdout, ciTestToken) || strings.Contains(stderr, ciTestToken) {
		t.Error("the token appears in the per-item output")
	}
}

// toolClassFor: ClassCI only with the flag as a real flag, in CI.
func TestCITransportToolClass(t *testing.T) {
	cases := []struct {
		name string
		ci   bool
		args []string
		want deskkit.ToolClass
	}{
		{"flag in CI", true, []string{"issues", "--repo", ciRepo, "--ci-workflow-token"}, deskkit.ClassCI},
		{"flag outside CI", false, []string{"issues", "--repo", ciRepo, "--ci-workflow-token"}, deskkit.ClassWrite},
		{"CI without flag", true, []string{"issues", "--repo", ciRepo}, deskkit.ClassWrite},
		{"neither", false, []string{"issues", "--repo", ciRepo}, deskkit.ClassWrite},
		{"flag as a flag value", true, []string{"issues", "--repo", "--ci-workflow-token"}, deskkit.ClassWrite},
		{"malformed line", true, []string{"issues", "--ci-workflow-token", "--bogus"}, deskkit.ClassWrite},
		{"kind only", true, []string{"issues"}, deskkit.ClassWrite},
	}
	for _, tc := range cases {
		if tc.ci {
			t.Setenv("GITHUB_ACTIONS", "true")
		} else {
			t.Setenv("GITHUB_ACTIONS", "")
		}
		if got := toolClassFor(tc.args); got != tc.want {
			t.Errorf("%s: toolClassFor = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The real binary: with an empty config home, the flag activates the CI roster class (env), and the
// refusal is the transport's own; without the flag the same environment stays inactive.
func TestCITransportActivatesWithEnvRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "deskread")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := t.TempDir()
	env := []string{
		"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"), "KUBECONFIG=/dev/null",
		"GITHUB_ACTIONS=true", "GITHUB_RUN_ID=9001", "GITHUB_REPOSITORY=" + ciRepo, "GITHUB_EVENT_NAME=push",
		ciTokenEnv + "=gh" + "p_notaninstallationtoken000000",
		deskkit.EnvRepoForges + "=" + ciRepo + "=github",
		deskkit.EnvTrustedBotSlugs + "=worker=assay-worker-app:300000006",
		deskkit.EnvAllowedRepos + "=" + ciRepo + ":ci:private",
		deskkit.EnvTrustedLogins + "=ada:2001",
		deskkit.EnvBlessLogin + "=ada:2001",
	}
	run1 := func(args ...string) (string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("run: %v", err)
		}
		return string(out), code
	}
	out, code := run1("issues", "--repo", ciRepo, "--ci-workflow-token")
	if code != deskkit.ExitRefused || !strings.Contains(out, "[ci-transport:token-shape]") || strings.Contains(out, "inactive") {
		t.Errorf("with the flag: exit=%d out=%s, want exit 5 with the token-shape tag and no inactive line", code, out)
	}
	out, code = run1("issues", "--repo", ciRepo)
	if code != deskkit.ExitUnverifiable || !strings.Contains(out, "inactive") {
		t.Errorf("without the flag: exit=%d out=%s, want exit 6 with the inactive line", code, out)
	}
}
