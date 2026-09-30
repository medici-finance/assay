package deskkit

// nativeread_test.go — the custody contract of the native read client, codified.
//
// "The native read client" is the resolver-built GitHub backend every desk read already goes
// through: ForgeFor(repo, role) mints the role's installation token for repo's account
// (githubCustody -> RoleTokenForRepo) and hands it to a GitHubForge, whose restClient binds
// that token explicitly onto a go-gh client (forge_github.go). These tests pin the two
// independent layers of that contract, each with the other bypassed:
//
//   - TRANSPORT FLOOR (refuse-if-unminted). An empty token is REFUSED before any request is
//     made — never resolved to the ambient gh-CLI identity the environment offers (GH_TOKEN,
//     GITHUB_TOKEN, GH_ENTERPRISE_TOKEN, a gh hosts.yml under HOME). The subtests drive the
//     backend directly, so the resolver's own checks are not what refuses.
//   - IDENTITY FLOOR (repo-derived installation). The installation the token is minted for
//     is derived from the repo coordinate the read names — never from GH_TOKEN, GH_REPO,
//     GH_HOST or HOME. The test mints through the real resolver with a non-empty token, so
//     the transport floor's refusal never fires and cannot be what passes it.
//
// FAIL-FIRST: internal/deskkit/nativeread-mutations.json (run with
// `go run ./cmd/muhar -spec internal/deskkit/nativeread-mutations.json` from tools/desk)
// plants each fault — an ambient-token fallback in the client, a resolver that reads the
// environment's token, a mint keyed on GH_REPO instead of the repo read — and every one must
// turn these tests red.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Ambient credential values planted in the environment. None of them may ever reach a
// request, and none may decide which installation a token is minted for.
const (
	ambientGHToken    = "ambient-gh-token-stub"
	ambientGitHubTok  = "ambient-github-token-stub"
	ambientEntToken   = "ambient-enterprise-token-stub"
	ambientHostsToken = "ambient-hosts-yml-token-stub"
	ambientOwner      = "example-ambient-org"
)

// plantAmbientCredentials fills the environment with every ambient identity go-gh or a gh
// child could resolve: the three token variables, a repo/host override, and a gh config
// (hosts.yml) under both GH_CONFIG_DIR and HOME. HOME is left pointing wherever the caller's
// roster fixture put it, so the roster stays loadable; the gh config is written INTO it.
func plantAmbientCredentials(t *testing.T) {
	t.Helper()
	t.Setenv("GH_TOKEN", ambientGHToken)
	t.Setenv("GITHUB_TOKEN", ambientGitHubTok)
	t.Setenv("GH_ENTERPRISE_TOKEN", ambientEntToken)
	t.Setenv("GH_REPO", ambientOwner+"/example-ambient-repo")
	t.Setenv("GH_HOST", "github.com")

	hosts := "github.com:\n    oauth_token: " + ambientHostsToken + "\n    user: example-ambient-user\n"
	cfgDir := filepath.Join(t.TempDir(), "gh")
	for _, dir := range []string{cfgDir, filepath.Join(os.Getenv("HOME"), ".config", "gh")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "hosts.yml"), []byte(hosts), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GH_CONFIG_DIR", cfgDir)
}

// seenRequest is what the fake forge recorded for one request.
type seenRequest struct {
	path string
	auth string
}

// recordingForge is an httptest server that answers a pull read and records the path and
// Authorization header of every request it receives.
func recordingForge(t *testing.T) (*httptest.Server, func() []seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, seenRequest{path: r.URL.Path, auth: r.Header.Get("Authorization")})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":7,"state":"open","head":{"sha":"abc123"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]seenRequest(nil), seen...)
	}
}

// assertNoAmbient fails if any recorded request carried an ambient credential.
func assertNoAmbient(t *testing.T, seen []seenRequest) {
	t.Helper()
	for _, r := range seen {
		for _, amb := range []string{ambientGHToken, ambientGitHubTok, ambientEntToken, ambientHostsToken} {
			if strings.Contains(r.auth, amb) {
				t.Errorf("request %s carried the AMBIENT credential %q — the read ran as an identity "+
					"nobody minted", r.path, amb)
			}
		}
	}
}

// TestNativeReadClientRefusesUnmintedToken is the transport-floor negative path: with every
// ambient credential present, a read handed no minted token is refused before any request —
// never resolved to the ambient identity.
func TestNativeReadClientRefusesUnmintedToken(t *testing.T) {
	withRoster(t, goldenRoster())
	plantAmbientCredentials(t)
	repo := ForgeRepo{Owner: "example-org", Name: "read-repo"}

	t.Run("empty_token_refused_before_any_request", func(t *testing.T) {
		srv, seen := recordingForge(t)
		g := &GitHubForge{Token: "", BaseURL: srv.URL, Client: srv.Client()}

		pr, err := g.GetPullRequest(repo, 7)
		if err == nil {
			t.Fatalf("an unminted read succeeded (%+v) — it resolved an identity nobody minted", pr)
		}
		if got := ExitCodeOf(err); got != ExitUnverifiable {
			t.Fatalf("exit = %d, want %d (could-not-check) for an unminted read: %v", got, ExitUnverifiable, err)
		}
		if !strings.Contains(err.Error(), "explicitly minted token") {
			t.Errorf("refusal does not name the missing minted token, so an operator cannot act on it: %v", err)
		}
		if n := len(seen()); n != 0 {
			t.Fatalf("%d request(s) reached the forge for an unminted read — the refusal must come BEFORE "+
				"the wire, not after an ambient-authenticated call", n)
		}
	})

	t.Run("minted_token_is_the_one_sent", func(t *testing.T) {
		// Positive control for the subtest above: the same read, handed a minted token, DOES
		// reach the forge — so the refusal is about the missing token, not a broken fixture —
		// and what it carries is the minted token, never an ambient one.
		srv, seen := recordingForge(t)
		g := &GitHubForge{Token: "minted-installation-stub", BaseURL: srv.URL, Client: srv.Client()}

		if _, err := g.GetPullRequest(repo, 7); err != nil {
			t.Fatalf("a minted read failed against the fixture: %v", err)
		}
		got := seen()
		if len(got) != 1 {
			t.Fatalf("minted read made %d requests, want 1", len(got))
		}
		if !strings.Contains(got[0].auth, "minted-installation-stub") {
			t.Fatalf("Authorization = %q, want the minted token", got[0].auth)
		}
		assertNoAmbient(t, got)
	})

	t.Run("empty_mint_refused_by_resolver", func(t *testing.T) {
		// The resolver path: a minter that answers with an empty token file yields NO client
		// at all, rather than a client that would go on to resolve an ambient identity.
		empty := filepath.Join(t.TempDir(), "worker-token-empty")
		if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		roster := goldenRoster()
		roster[EnvRepoForges] = repo.Slug() + "=github"
		withRoster(t, roster)
		plantAmbientCredentials(t)
		stubMinter(t, empty, "", nil)

		f, err := ForgeFor(repo, "worker")
		if err == nil {
			t.Fatalf("an empty mint produced a client: %#v", f)
		}
		if f != nil {
			t.Fatalf("ForgeFor returned a non-nil Forge alongside an error: %#v", f)
		}
		if got := ExitCodeOf(err); got != ExitRefused {
			t.Fatalf("exit = %d, want %d (refused) — an empty mint is a custody failure: %v", got, ExitRefused, err)
		}
	})
}

// TestNativeReadClientInstallationFromRepoNotEnv is the identity-floor negative path: with an
// ambient GH_TOKEN, GH_REPO naming another account, and a gh login under HOME, the
// installation a read's token is minted for is the account of the repo being READ, and the
// token the read carries is that mint — for each of two repos in two different accounts.
func TestNativeReadClientInstallationFromRepoNotEnv(t *testing.T) {
	repoA := ForgeRepo{Owner: "example-org", Name: "read-repo"}
	repoB := ForgeRepo{Owner: "example-other-org", Name: "other-repo"}
	roster := goldenRoster()
	roster[EnvRepoForges] = repoA.Slug() + "=github," + repoB.Slug() + "=github"
	withRoster(t, roster)
	plantAmbientCredentials(t)

	// One minted token file per account; the minter records which account it was asked for.
	dir := t.TempDir()
	tokenFor := map[string]string{
		repoA.Owner:  "minted-for-example-org",
		repoB.Owner:  "minted-for-example-other-org",
		ambientOwner: "minted-for-the-ambient-org",
	}
	var asked []string
	resetRoleTokenMemo()
	restore := SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		asked = append(asked, role+" "+owner)
		tok, ok := tokenFor[owner]
		if !ok {
			tok = "minted-for-unknown-" + owner
		}
		p := filepath.Join(dir, role+"-token-"+owner)
		if err := os.WriteFile(p, []byte(tok+"\n"), 0o600); err != nil {
			return "", "", err
		}
		return p, "", nil
	})
	t.Cleanup(func() {
		restore()
		resetRoleTokenMemo()
	})

	for _, repo := range []ForgeRepo{repoA, repoB} {
		asked = nil
		f, err := ForgeFor(repo, "worker")
		if err != nil {
			t.Fatalf("ForgeFor(%s): %v", repo.Slug(), err)
		}
		g, ok := f.(*GitHubForge)
		if !ok {
			t.Fatalf("ForgeFor(%s) returned %T, want *GitHubForge", repo.Slug(), f)
		}
		if len(asked) != 1 || asked[0] != "worker "+repo.Owner {
			t.Fatalf("minter asked for %v while reading %s — the installation must be the READ repo's "+
				"account (%s), never one the environment names", asked, repo.Slug(), repo.Owner)
		}
		want := tokenFor[repo.Owner]
		if g.Token != want {
			t.Fatalf("client token for %s = %q, want the mint for %s (%q)", repo.Slug(), g.Token, repo.Owner, want)
		}

		// Read through the client: the request targets the repo named, carrying its mint.
		srv, seen := recordingForge(t)
		g.BaseURL, g.Client = srv.URL, srv.Client()
		if _, err := g.GetPullRequest(repo, 7); err != nil {
			t.Fatalf("read of %s failed: %v", repo.Slug(), err)
		}
		got := seen()
		if len(got) != 1 {
			t.Fatalf("read of %s made %d requests, want 1", repo.Slug(), len(got))
		}
		if wantPath := "/repos/" + repo.Owner + "/" + repo.Name + "/pulls/7"; !strings.HasSuffix(got[0].path, wantPath) {
			t.Errorf("read of %s hit %q, want a path ending %q", repo.Slug(), got[0].path, wantPath)
		}
		if !strings.Contains(got[0].auth, want) {
			t.Errorf("read of %s carried Authorization %q, want its own mint %q", repo.Slug(), got[0].auth, want)
		}
		assertNoAmbient(t, got)
	}
}
