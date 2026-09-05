package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// coverageFixture describes one installation and the repository pages it serves.
type coverageFixture struct {
	id        int64
	login     string
	acctType  string   // "Organization" | "User"
	selection string   // "all" | "selected"
	repos     []string // full_name each, in the order the API returns them
	token     string   // the installation token minted for it (test-secret)
	failPage  int      // if >0, this 1-based repo page returns HTTP 500
}

// coverageServer stands in for the GitHub API base: it serves
// GET /app/installations, POST /app/installations/{id}/access_tokens (returning
// each installation's fixture token), and the paginated
// GET /installation/repositories keyed off the installation token in the
// Authorization header. Nothing contacts the real forge.
func coverageServer(t *testing.T, fixtures []coverageFixture) *httptest.Server {
	t.Helper()
	byToken := map[string]coverageFixture{}
	byID := map[string]coverageFixture{}
	for _, f := range fixtures {
		byToken[f.token] = f
		byID[strconv.FormatInt(f.id, 10)] = f
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /app/installations", func(w http.ResponseWriter, r *http.Request) {
		type acct struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		}
		type inst struct {
			ID                  int64  `json:"id"`
			Account             acct   `json:"account"`
			RepositorySelection string `json:"repository_selection"`
		}
		out := make([]inst, 0, len(fixtures))
		for _, f := range fixtures {
			out = append(out, inst{
				ID:                  f.id,
				Account:             acct{Login: f.login, Type: f.acctType},
				RepositorySelection: f.selection,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("POST /app/installations/{id}/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		f, ok := byID[id]
		if !ok {
			http.Error(w, "no such installation", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": f.token, "expires_at": "2124-01-01T00:00:00Z"})
	})

	mux.HandleFunc("GET /installation/repositories", func(w http.ResponseWriter, r *http.Request) {
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "token ")
		f, ok := byToken[auth]
		if !ok {
			http.Error(w, "bad installation token", 401)
			return
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage <= 0 {
			perPage = 100
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page <= 0 {
			page = 1
		}
		if f.failPage == page {
			http.Error(w, "server exploded", 500)
			return
		}
		start := (page - 1) * perPage
		end := start + perPage
		if start > len(f.repos) {
			start = len(f.repos)
		}
		if end > len(f.repos) {
			end = len(f.repos)
		}
		type repo struct {
			FullName string `json:"full_name"`
		}
		out := struct {
			Repositories []repo `json:"repositories"`
		}{}
		for _, full := range f.repos[start:end] {
			out.Repositories = append(out.Repositories, repo{FullName: full})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})

	return httptest.NewServer(mux)
}

// setupCoverage points HOME at a temp dir, disables the kill switch, installs a
// role PEM + App ID, and redirects httpClient at srv. It returns the config-home
// head directory (~/.config/assay under the temp HOME).
func setupCoverage(t *testing.T, role string, srv *httptest.Server) string {
	t.Helper()
	homeDir := setupTest(t)
	t.Setenv(roleEnvPrefix(role)+"_APP_ID", "12345")
	cfgHome := filepath.Join(homeDir, ".config", "assay")
	writeFileMode(t, filepath.Join(cfgHome, role+"-app.pem"), makePEM(t), 0o600)

	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = oldClient })

	oldPerPage := coveragePerPage
	coveragePerPage = 2
	t.Cleanup(func() { coveragePerPage = oldPerPage })
	return cfgHome
}

// twoInstallFixtures is the shared two-installation fixture. Installation A
// ("medici-finance", all) carries three repos across two pages (per_page=2);
// installation B ("example-org", selected) carries two, exercising the
// exactly-full-page-then-empty-page boundary. "tracker" appears under BOTH owners
// so a full_name filter can be shown not to cross owners.
func twoInstallFixtures() []coverageFixture {
	return []coverageFixture{
		{
			id: 200000001, login: "medici-finance", acctType: "Organization", selection: "all",
			repos: []string{"medici-finance/repo-a", "medici-finance/repo-b", "medici-finance/tracker"},
			token: "SYNTHETIC-INSTALL-TOKEN-A-must-not-leak",
		},
		{
			id: 200000002, login: "example-org", acctType: "Organization", selection: "selected",
			repos: []string{"example-org/tracker", "example-org/other"},
			token: "SYNTHETIC-INSTALL-TOKEN-B-must-not-leak",
		},
	}
}

// TestCoverageListsEveryInstallation: both installations render, sorted by
// account.login (example-org before medici-finance), with repo counts matching
// the fixture — including the paginated one.
func TestCoverageListsEveryInstallation(t *testing.T) {
	srv := coverageServer(t, twoInstallFixtures())
	defer srv.Close()
	setupCoverage(t, "reviewer", srv)

	rc, stdout, stderr := runCap(t, []string{"coverage", "reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
	}

	// Order: the example-org block must appear before the medici-finance block.
	idxB := strings.Index(stdout, "installation 200000002")
	idxA := strings.Index(stdout, "installation 200000001")
	if idxB < 0 || idxA < 0 {
		t.Fatalf("both installation blocks must appear; got:\n%s", stdout)
	}
	if idxB > idxA {
		t.Errorf("installations not sorted by account.login (example-org must precede medici-finance):\n%s", stdout)
	}

	// Header lines carry account/type/selection and the correct repo counts.
	wantHeaders := []string{
		"installation 200000002 account=example-org type=Org selection=selected repos=2",
		"installation 200000001 account=medici-finance type=Org selection=all repos=3",
	}
	for _, h := range wantHeaders {
		if !strings.Contains(stdout, h) {
			t.Errorf("missing header line %q in:\n%s", h, stdout)
		}
	}

	// The paginated installation shows ALL three repos, sorted.
	for _, full := range []string{"medici-finance/repo-a", "medici-finance/repo-b", "medici-finance/tracker"} {
		if !strings.Contains(stdout, "  "+full+"\n") {
			t.Errorf("missing repo line %q in:\n%s", full, stdout)
		}
	}

	// Repo lines under installation A must be sorted.
	block := stdout[idxA:]
	ra := strings.Index(block, "medici-finance/repo-a")
	rb := strings.Index(block, "medici-finance/repo-b")
	rt := strings.Index(block, "medici-finance/tracker")
	if !(ra < rb && rb < rt) {
		t.Errorf("repositories not sorted within the installation block:\n%s", block)
	}
}

// TestCoverageRepoFilterExitCodes: a seen repo exits 0 and names its
// installation; an unseen repo exits 5. The match is on full_name, so
// medici-finance/tracker resolves to installation A and never to B's
// example-org/tracker.
func TestCoverageRepoFilterExitCodes(t *testing.T) {
	srv := coverageServer(t, twoInstallFixtures())
	defer srv.Close()
	setupCoverage(t, "reviewer", srv)

	// Hit: full_name under installation A.
	rc, stdout, stderr := runCap(t, []string{"coverage", "reviewer", "--repo", "medici-finance/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("hit rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if !strings.Contains(stdout, "installation 200000001") {
		t.Errorf("hit output must name the matching installation A; got:\n%s", stdout)
	}
	if strings.Contains(stdout, "installation 200000002") {
		t.Errorf("hit output must print ONLY the matching installation, not B; got:\n%s", stdout)
	}

	// full_name discrimination: example-org/tracker resolves to B, not A.
	rc, stdout, _ = runCap(t, []string{"coverage", "reviewer", "--repo", "example-org/tracker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("hit-B rc = %d, want 0", rc)
	}
	if !strings.Contains(stdout, "installation 200000002") || strings.Contains(stdout, "installation 200000001") {
		t.Errorf("example-org/tracker must resolve to installation B only; got:\n%s", stdout)
	}

	// Miss: a repo no installation sees exits 5.
	rc, _, stderr = runCap(t, []string{"coverage", "reviewer", "--repo", "nobody/nothing"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("miss rc = %d, want 5; stderr: %s", rc, stderr)
	}
}

// TestCoveragePageFailureIsUnverifiable: a 500 on page two exits 6 and the output
// carries NO repository lines — a failed page is could-not-check, never a short
// list read as complete.
func TestCoveragePageFailureIsUnverifiable(t *testing.T) {
	fx := twoInstallFixtures()
	// Make installation A's SECOND repo page fail.
	fx[0].failPage = 2
	srv := coverageServer(t, fx)
	defer srv.Close()
	setupCoverage(t, "reviewer", srv)

	rc, stdout, stderr := runCap(t, []string{"coverage", "reviewer"})
	if rc != deskkit.ExitUnverifiable {
		t.Fatalf("rc = %d, want 6; stderr: %s", rc, stderr)
	}
	// The refusal names the installation whose page failed.
	if !strings.Contains(stderr, "200000001") {
		t.Errorf("exit-6 message must name the failing installation; stderr: %s", stderr)
	}
	// NO repository line may appear on stdout — not a single indented owner/name.
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "  ") && strings.Contains(line, "/") {
			t.Errorf("a partial repository list leaked to stdout: %q", line)
		}
	}
}

// TestCoverageWritesNoCacheAndPrintsNoToken: the config-home head directory is
// unchanged after the run, and no installation token value appears on stdout,
// stderr, or the audit line. HOME (where the audit log lives) is kept SEPARATE
// from ASSAY_CONFIG_HOME (the search-path head) so the audit write does not
// disturb the byte-identical assertion on the head directory.
func TestCoverageWritesNoCacheAndPrintsNoToken(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "test-session")

	cfgHome := t.TempDir()
	t.Setenv(deskkit.EnvConfigHome, cfgHome)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(cfgHome, "reviewer-app.pem"), makePEM(t), 0o600)

	fx := twoInstallFixtures()
	srv := coverageServer(t, fx)
	defer srv.Close()
	oldClient := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	defer func() { httpClient = oldClient }()
	oldPerPage := coveragePerPage
	coveragePerPage = 2
	defer func() { coveragePerPage = oldPerPage }()

	before := snapshotDir(t, cfgHome)

	rc, stdout, stderr := runCap(t, []string{"coverage", "reviewer"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0; stderr: %s", rc, stderr)
	}

	after := snapshotDir(t, cfgHome)
	if !reflectDirEqual(before, after) {
		t.Errorf("config-home head directory changed under coverage:\nbefore=%v\nafter=%v", before, after)
	}

	// No token value anywhere in output.
	for _, f := range fx {
		if strings.Contains(stdout, f.token) {
			t.Errorf("installation token %q leaked to stdout", f.token)
		}
		if strings.Contains(stderr, f.token) {
			t.Errorf("installation token %q leaked to stderr", f.token)
		}
	}
	// No token value in the audit line.
	for _, e := range auditEntries(t) {
		blob, _ := json.Marshal(e)
		for _, f := range fx {
			if strings.Contains(string(blob), f.token) {
				t.Errorf("installation token %q leaked into audit entry: %s", f.token, string(blob))
			}
		}
	}
}

// TestCoverageRefusesGitLabForge: --forge gitlab is refused with exit 5 BEFORE
// any network call. httpClient is pointed at a transport that fails every request
// so a stray call would surface as a non-5 exit.
func TestCoverageRefusesGitLabForge(t *testing.T) {
	homeDir := setupTest(t)
	t.Setenv("REVIEWER_APP_ID", "12345")
	writeFileMode(t, filepath.Join(homeDir, ".config", "assay", "reviewer-app.pem"), makePEM(t), 0o600)

	oldClient := httpClient
	httpClient = &http.Client{Transport: failTransport{}}
	defer func() { httpClient = oldClient }()

	rc, _, stderr := runCap(t, []string{"coverage", "reviewer", "--forge", "gitlab"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want 5; stderr: %s", rc, stderr)
	}
	if !strings.Contains(strings.ToLower(stderr), "github-only") {
		t.Errorf("refusal must name the GitHub-only scope; stderr: %s", stderr)
	}
}

// failTransport fails every request, proving a code path made no network call.
type failTransport struct{}

func (failTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errNoNetworkExpected
}

var errNoNetworkExpected = errorString("coverage made a network call on a path that must not")

type errorString string

func (e errorString) Error() string { return string(e) }

// --- small dir-snapshot helpers -------------------------------------------------

// snapshotDir returns a map of relative-path → content for every regular file
// under root, so two snapshots can be compared byte-for-byte.
func snapshotDir(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return out
}

// reflectDirEqual compares two directory snapshots.
func reflectDirEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}
