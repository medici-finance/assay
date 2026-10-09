package main

// ci_test.go — the CI workflow-token transport (forge-neutral brief 34): every refusal layer
// tested with every OTHER layer satisfied, so a test goes red only if its own layer is removed.
//
// A refused run must make zero forge requests: each row points the deskkit CI test seam at its own
// httptest server and asserts the server saw nothing. The token is a placeholder, not a credential.

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// defaultHost stands in for the real API host for this whole test binary. TestMain points both
// API-base seams at it (the custody backend's forgeAPIBase and deskkit's CI-token seam), so a
// test that forgets to point a backend at its own server reaches this stand-in instead of the
// real host, and the binary fails naming the paths it saw. It is the class guard for the
// no-default-probe convention in this package: no deskread test may address the live API host.
var defaultHost struct {
	mu    sync.Mutex
	url   string
	paths []string
}

func defaultHostHits() []string {
	defaultHost.mu.Lock()
	defer defaultHost.mu.Unlock()
	return append([]string(nil), defaultHost.paths...)
}

func TestMain(m *testing.M) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultHost.mu.Lock()
		defaultHost.paths = append(defaultHost.paths, r.Method+" "+r.URL.Path)
		defaultHost.mu.Unlock()
		http.Error(w, "a test reached the default API host stand-in", http.StatusTeapot)
	}))
	defaultHost.url = srv.URL
	forgeAPIBase = srv.URL
	restore := deskkit.SetCITokenAPIBaseForTest(srv.URL)
	code := m.Run()
	restore()
	srv.Close()
	if hits := defaultHostHits(); len(hits) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: %d request(s) reached the default API host stand-in — a test built a backend "+
			"without pointing it at its own server, which outside this binary is the live API host: %v\n", len(hits), hits)
		code = 1
	}
	os.Exit(code)
}

// TestDefaultHostStandInInstalled is the positive control for TestMain: at the start of a test
// both API-base seams name the stand-in, never the empty value that means the live host.
func TestDefaultHostStandInInstalled(t *testing.T) {
	if defaultHost.url == "" || forgeAPIBase != defaultHost.url {
		t.Fatalf("forgeAPIBase = %q, want the stand-in %q", forgeAPIBase, defaultHost.url)
	}
}

const (
	ciTestToken = "gh" + "s_placeholder_tok"
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
	t.Setenv("GH_TOKEN", ambientTestToken)
	t.Setenv("GITHUB_TOKEN", ambientTestToken)
	t.Setenv(deskkit.EnvRepoForges, ciRepo+"=github,"+ciOtherRepo+"=github")
	deskkit.SetToolClass(deskkit.ClassCI)
	t.Cleanup(func() { deskkit.SetToolClass(deskkit.ClassWrite) })
	defer deskkit.ReloadConfig()
	t.Cleanup(deskkit.SetCITokenAPIBaseForTest(srv.URL))
	// The custody backend too: a run without the opt-in reaches custody, and its reads must land on
	// this test's server where they are observed, never on the default host.
	prevBase := forgeAPIBase
	forgeAPIBase = srv.URL
	t.Cleanup(func() { forgeAPIBase = prevBase })

	c := &custodyCalls{}
	prevRole, prevMint := sessionRoleFn, mintTokenFn
	sessionRoleFn = func(string) (string, string, error) { c.role++; return "worker-desk", "", nil }
	mintTokenFn = func(string, string) (string, string, error) { c.mint++; return testToken, "", nil }
	t.Cleanup(func() { sessionRoleFn, mintTokenFn = prevRole, prevMint })
	return c
}

// runCI runs the verb capturing stdout and stderr both, and checks every run for a token leak:
// it is the one place a test in this file calls run, so no envelope, partial reason, refusal or
// log line of any run escapes the check (TestCIRunsCheckForTokenLeak pins that).
func runCI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb strings.Builder
	code = run(args, &out, &errb)
	stdout, stderr = out.String(), errb.String()
	assertNoTokenLeak(t, strings.Join(args, " "), stdout+"\n"+stderr)
	return stdout, stderr, code
}

// ambientTestToken is the value ciSetup puts in GH_TOKEN and GITHUB_TOKEN; it must never be used.
const ambientTestToken = "gh" + "s_ambient_not_used_tok"

// assertNoTokenLeak fails if any token this file plants (the CI token, the minted custody token,
// the ambient decoys, or whatever the CI variable holds right now) appears in output.
func assertNoTokenLeak(t *testing.T, what, output string, extra ...string) {
	t.Helper()
	toks := append([]string{ciTestToken, testToken, ambientTestToken}, extra...)
	if v := os.Getenv(ciTokenEnv); v != "" && v != ciTestToken {
		toks = append(toks, v)
	}
	for _, tok := range toks {
		if strings.Contains(output, tok) {
			t.Errorf("%s: a token (%d bytes, prefix %q) appears in the output", what, len(tok), tok[:3])
		}
	}
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
	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--ci-workflow-token")
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
	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo)
	// The custody path runs: it resolves the role and mints (stubbed), and never uses the CI token.
	if code != deskkit.ExitOK {
		t.Fatalf("the custody run without the flag: exit=%d, want %d; stderr=%s", code, deskkit.ExitOK, stderr)
	}
	if c.role == 0 || c.mint == 0 {
		t.Errorf("the custody path did not run: role=%d mint=%d (exit %d; %s)", c.role, c.mint, code, stderr)
	}
	var env Envelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if len(env.Repos) != 1 || env.Repos[0].Repo != ciRepo || len(env.Repos[0].Issues) != 1 || len(env.Partial) != 0 {
		t.Errorf("the custody read did not answer from this test's server: repos=%+v partial=%+v", env.Repos, env.Partial)
	}
	if env.Identity == nil || env.Identity.Transport != transportCustody {
		t.Errorf("identity = %+v, want the custody transport", env.Identity)
	}
	if !strings.Contains(stderr, ciTokenEnv+" is set but ignored") {
		t.Errorf("stderr lacks the one line naming the ignored variable: %s", stderr)
	}
	if strings.Contains(stderr, ciTestToken) {
		t.Error("the ignored line carries the token value")
	}
	// The header assertion below must have requests to look at: the custody read reached this
	// test's server, carrying the minted token.
	reqs := srv.seen()
	if len(reqs) == 0 {
		t.Fatal("no request reached the test server: the header assertion below would pass over nothing")
	}
	sawMinted := false
	for _, r := range reqs {
		if strings.Contains(r.auth, testToken) {
			sawMinted = true
		}
		if strings.Contains(r.auth, ciTestToken) {
			t.Errorf("the CI token was sent without the opt-in flag (%s)", r.path)
		}
	}
	if !sawMinted {
		t.Errorf("the server never saw the minted custody token over %d requests", len(reqs))
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
		{"repository bad charset", "GITHUB_REPOSITORY", "example-org/a?x", layerOutsideCI},
		{"repository dot-segment", "GITHUB_REPOSITORY", "example-org/..", layerOutsideCI},
		{"fork-triggered event", "GITHUB_EVENT_NAME", "pull_request_target", layerEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newRecSrv(t)
			c := ciSetup(t, srv)
			t.Setenv(tc.key, tc.val)
			stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--ci-workflow-token")
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
		stdout, stderr, code := runCI(t, k, "--repo", ciRepo, "--ci-workflow-token")
		if k == "issue-list" || k == "changes" {
			// these need --state; supply it so the kind layer is the only refusal.
			stdout, stderr, code = runCI(t, k, "--repo", ciRepo, "--state", map[string]string{"issue-list": "open", "changes": "merged"}[k], "--ci-workflow-token")
		}
		wantRefusal(t, k, layerKind, code, stdout, stderr, srv)
	}
	// A kind added to readKinds but not to ciTransportKinds is refused (the closed-set property).
	readKinds["zz-test-only"] = "test-only kind absent from ciTransportKinds"
	defer delete(readKinds, "zz-test-only")
	stdout, stderr, code := runCI(t, "zz-test-only", "--repo", ciRepo, "--ci-workflow-token")
	wantRefusal(t, "zz-test-only", layerKind, code, stdout, stderr, srv)
}

// Every ciTransportKinds entry is a real read kind, and the set is exactly the three the brief
// names: a kind added to it is a reviewed change to this test as well, never a silent widening.
func TestCITransportKindsAreReads(t *testing.T) {
	if len(ciTransportKinds) == 0 {
		t.Fatal("ciTransportKinds is empty")
	}
	if got := strings.Join(sortedCIKinds(), ","); got != "comments,issues,trust" || len(ciTransportKinds) != 3 {
		t.Errorf("ciTransportKinds = %v, want exactly comments, issues, trust", ciTransportKinds)
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
		stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--ci-workflow-token")
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
	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--repo", ciOtherRepo, "--ci-workflow-token")
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
	// This run's envelope carries a partial entry: its reason, like every other byte of stdout and
	// stderr, must not carry the token (runCI checks too; this run is the one with a partial).
	if strings.Contains(stdout, ciTestToken) || strings.Contains(stderr, ciTestToken) {
		t.Errorf("the token appears in the output of the run with a partial entry")
	}

	srv2 := newRecSrv(t)
	ciSetup(t, srv2)
	stdout, stderr, code = runCI(t, "trust", "--issue", ciOtherRepo+"#7", "--ci-workflow-token")
	wantRefusal(t, "trust other repo", layerRepository, code, stdout, stderr, srv2)

	// With the verb bypassed: the constructor re-checks the binding.
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "beta"}
	if _, _, err := deskkit.ReadOnlyForgeForCIToken(fr, ciRepo, ciTestToken); err == nil || !deskkit.IsRefused(err) {
		t.Errorf("constructor built a backend for another repository: %v", err)
	}
}

// The comments kind over the CI transport reads the thread on the target the address flag names:
// the decorator hands the target kind through, so the item is read rather than landing in partial.
func TestCITransportServesComments(t *testing.T) {
	srv := newRecSrv(t)
	ciSetup(t, srv)
	stdout, stderr, code := runCI(t, "comments", "--issue", ciRepo+"#7", "--ci-workflow-token")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, stderr)
	}
	var ie ItemEnvelope
	if err := json.Unmarshal([]byte(stdout), &ie); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if len(ie.Partial) != 0 || len(ie.Items) != 1 || ie.Items[0].Comments == nil || len(*ie.Items[0].Comments) != 1 ||
		(*ie.Items[0].Comments)[0].AuthorLogin != "authority" {
		t.Fatalf("comments over the CI transport: items=%+v partial=%+v", ie.Items, ie.Partial)
	}
	reqs := srv.seen()
	if len(reqs) == 0 {
		t.Fatal("no request reached the server")
	}
	for _, r := range reqs {
		if r.auth != "Bearer "+ciTestToken && r.auth != "token "+ciTestToken {
			t.Errorf("request %s did not carry the CI token", r.path)
		}
	}
}

// The per-item repository binding compares owner/name case-insensitively, as GitHub does: the
// job's own repository spelled in another case is not refused as "another repository".
func TestCIPerItemRepoCaseFold(t *testing.T) {
	srv := newRecSrv(t)
	ciSetup(t, srv)
	_, stderr, code := runCI(t, "trust", "--issue", strings.ToUpper(ciRepo)+"#7", "--ci-workflow-token")
	if code == deskkit.ExitRefused || strings.Contains(stderr, "[ci-transport:"+layerRepository+"]") {
		t.Errorf("the job's own repository in upper case was refused as another repository: exit=%d %s", code, stderr)
	}
}

// Layer: no-token. The flag alone is not enough; ambient tokens never substitute.
func TestCITransportFlagWithoutTokenRefused(t *testing.T) {
	srv := newRecSrv(t)
	c := ciSetup(t, srv)
	t.Setenv(ciTokenEnv, "")
	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--ci-workflow-token")
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

	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo)
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
	stdout, stderr, code := runCI(t, "issues", "--repo", ciRepo, "--ci-workflow-token")
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
	for _, tok := range []string{ciTestToken, ambientTestToken} {
		if strings.Contains(stdout, tok) || strings.Contains(stderr, tok) {
			t.Errorf("a token appears in the output")
		}
	}
	// Per-item kind too.
	stdout, stderr, code = runCI(t, "trust", "--issue", ciRepo+"#7", "--ci-workflow-token")
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
	binToken := "gh" + "p_not_an_install_tok"
	bin := filepath.Join(t.TempDir(), "deskread")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := t.TempDir()
	env := []string{
		"HOME=" + home, "USERPROFILE=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"), "KUBECONFIG=/dev/null",
		"GITHUB_ACTIONS=true", "GITHUB_RUN_ID=9001", "GITHUB_REPOSITORY=" + ciRepo, "GITHUB_EVENT_NAME=push",
		ciTokenEnv + "=" + binToken,
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
		assertNoTokenLeak(t, strings.Join(args, " "), string(out), binToken)
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

// tokenLeakUnchecked is the class guard for "a token reaches output undetected": it returns
// "<file>:<line> <func>" for every call to run, or to an exec command's CombinedOutput/Output, whose
// innermost enclosing function does not also call assertNoTokenLeak. A test that drives the verb
// any other way than through a checked helper is a run whose envelope, partial reasons and log
// lines no token check reads.
func tokenLeakUnchecked(fset *token.FileSet, f *ast.File) []string {
	var out []string
	check := func(fn ast.Node, name string, body *ast.BlockStmt) {
		if body == nil {
			return
		}
		var hazards []token.Pos
		checked := false
		ast.Inspect(body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.FuncLit); ok && lit != fn {
				return false // its own function; checked on its own
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if fun.Name == "run" {
					hazards = append(hazards, call.Pos())
				}
				if fun.Name == "assertNoTokenLeak" {
					checked = true
				}
			case *ast.SelectorExpr:
				if (fun.Sel.Name == "CombinedOutput" || fun.Sel.Name == "Output") && !isGoToolchainCall(fun.X) {
					hazards = append(hazards, call.Pos())
				}
			}
			return true
		})
		if checked {
			return
		}
		for _, p := range hazards {
			pos := fset.Position(p)
			out = append(out, fmt.Sprintf("%s:%d %s", filepath.Base(pos.Filename), pos.Line, name))
		}
	}
	var outer string
	ast.Inspect(f, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			outer = fn.Name.Name
			check(fn, fn.Name.Name, fn.Body)
		case *ast.FuncLit:
			check(fn, outer+".func", fn.Body)
		}
		return true
	})
	sort.Strings(out)
	return out
}

// isGoToolchainCall reports whether x is exec.Command("go", ...): building the binary under test
// runs the toolchain, not the verb, and is the one exec call the guard does not count.
func isGoToolchainCall(x ast.Expr) bool {
	call, ok := x.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Command" {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "exec" {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Value == `"go"`
}

// TestCIRunsCheckForTokenLeak applies tokenLeakUnchecked to every test file in this package that
// handles the CI token, and to a planted fixture it must flag (the positive control: a matcher that
// stopped matching would otherwise report clean).
func TestCIRunsCheckForTokenLeak(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("could-not-check: no test files found (%v)", err)
	}
	scanned := 0
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("could-not-check: %v", err)
		}
		if !strings.Contains(string(src), "ciTokenEnv") && !strings.Contains(string(src), "DESKREAD_CI_WORKFLOW_TOKEN") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("could-not-check: parse %s: %v", name, err)
		}
		scanned++
		for _, off := range tokenLeakUnchecked(fset, f) {
			t.Errorf("%s drives the verb without a token-leak check; route it through runCI or call assertNoTokenLeak on its output", off)
		}
	}
	if scanned == 0 {
		t.Fatal("could-not-check: no test file handles the CI token, so the guard scanned nothing")
	}

	const planted = `package main
func TestPlanted(t *testing.T) { var a, b strings.Builder; _ = run([]string{"issues"}, &a, &b) }
func TestPlantedBin(t *testing.T) { f := func() { _, _ = exec.Command("x").CombinedOutput() }; f() }
func runChecked(t *testing.T) { var a, b strings.Builder; _ = run(nil, &a, &b); assertNoTokenLeak(t, "", "") }
func TestBuildOnly(t *testing.T) { _, _ = exec.Command("go", "build").CombinedOutput() }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted_test.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := tokenLeakUnchecked(fset, f)
	want := []string{"planted_test.go:2 TestPlanted", "planted_test.go:3 TestPlantedBin.func"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("the guard over the planted fixture reported %v, want %v", got, want)
	}
}
