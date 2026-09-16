package loopengine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// probes_forge_test.go — the offline tests behind #1197 for the production BranchLister:
// houseBranchLister must list AUTHENTICATED against the forge the resolver names, never an
// anonymous read against a hardcoded github.com (which 404s on a private repo, so on a
// private board root a dispatched worker's liveness was seen only through the PR probe).

// TestHouseBranchListerNeverHardcodesGitHubHost is the source-level guard: probes.go must
// not carry a `https://github.com/` literal in a git URL. The host is the forge resolver's
// answer (deskkit.ForgeKindFromSlugAndHost), the same way cmd/deskclaim-ref's newForgeStore
// builds its URL — a compiled-in SaaS host is the self-hosted-adopter failure #727 retired.
func TestHouseBranchListerNeverHardcodesGitHubHost(t *testing.T) {
	src, err := os.ReadFile("probes.go")
	if err != nil {
		t.Fatalf("read probes.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `"https://github.com/`) {
			t.Fatalf("probes.go:%d hardcodes the GitHub host in a git URL: %s\n"+
				"— the host must come from the forge resolver (deskkit.ForgeKindFromSlugAndHost), never a literal", i+1, strings.TrimSpace(line))
		}
	}
}

// withForgeFixture installs a roster naming slug's forge into a private config home (the
// REAL loader, file + permissions + parse), pins the session loop to the-desk (role "desk"),
// and reloads. No git checkout and no network: the origin host is what the test passes in.
func withForgeFixture(t *testing.T, slug, forge string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := deskkit.EnvRepoForges + "=" + slug + "=" + forge + "\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	t.Setenv("DESK_LOOP", "the-desk")
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return dir
}

// withFixtureGitHubMinter installs a custody minter that hands back a fixture token without
// forking desktoken or touching the network; err != nil makes it refuse instead.
func withFixtureGitHubMinter(t *testing.T, token string, err error) {
	t.Helper()
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (string, string, error) {
		return token, "", err
	})
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })
}

// TestHouseBranchListOpts_PrivateSlugCarriesAuthAndResolvedHost is the issue's Verify row
// for the production BranchLister: the ListOpts built for a non-public slug carries a
// non-nil Auth (the session role's token as the forge's git-basic credential) and a host
// the forge resolver derived from the origin — a fixture host that is NOT github.com, so a
// hardcoded SaaS host cannot pass.
func TestHouseBranchListOpts_PrivateSlugCarriesAuthAndResolvedHost(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "ghs_fixture_token", nil)

	opts, err := houseBranchListOptsWithHost("example-org/private", "git.example.test")
	if err != nil {
		t.Fatalf("houseBranchListOptsWithHost: %v", err)
	}
	if want := "https://git.example.test/example-org/private.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q (host from the resolver, never a literal)", opts.URL, want)
	}
	if opts.Auth == nil {
		t.Fatal("ListOpts.Auth is nil — the listing would go out anonymous, which is the #1197 404 on every private repo")
	}
	ba, ok := opts.Auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("ListOpts.Auth is %T, want *githttp.BasicAuth", opts.Auth)
	}
	if ba.Username != gitcore.GitHubGitUsername || ba.Password != "ghs_fixture_token" {
		t.Fatalf("git credential = %q:<%d bytes>, want %q:<the session role's token>", ba.Username, len(ba.Password), gitcore.GitHubGitUsername)
	}
}

// TestHouseBranchListOpts_GitLabPairsPATWithOAuthUsername: a gitlab-resolved slug pairs the role's
// provisioned PAT with GitLab's required "oauth2" username — the forge-neutral half.
func TestHouseBranchListOpts_GitLabPairsPATWithOAuthUsername(t *testing.T) {
	dir := withForgeFixture(t, "example-org/gitlab-pilot", "gitlab")
	if err := os.WriteFile(filepath.Join(dir, "gitlab-desk.token"), []byte("glpat-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := houseBranchListOptsWithHost("example-org/gitlab-pilot", "gitlab.example.test")
	if err != nil {
		t.Fatalf("houseBranchListOptsWithHost: %v", err)
	}
	if want := "https://gitlab.example.test/example-org/gitlab-pilot.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q", opts.URL, want)
	}
	ba, ok := opts.Auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("ListOpts.Auth is %T, want *githttp.BasicAuth", opts.Auth)
	}
	if ba.Username != gitcore.GitLabGitUsername || ba.Password != "glpat-fixture" {
		t.Fatalf("git credential = %q:<%d bytes>, want %q:<the provisioned PAT>", ba.Username, len(ba.Password), gitcore.GitLabGitUsername)
	}
}

// TestBranchProbe_ProductionListerMissingTokenIsCouldNotCheck runs the PRODUCTION lister
// (houseBranchLister) through NewBranchProbe with no token available: the probe must report
// could-not-check (exit 6), never a clean "no observation" that the liveness verdict would
// read as no-life — and it must do so BEFORE any transport is dialed (no network here).
func TestBranchProbe_ProductionListerMissingTokenIsCouldNotCheck(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "", errors.New("no App credential provisioned in this fixture"))

	probe := NewBranchProbe(houseBranchLister, NewMemBranchSHAStore())
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "example-org/private", PayloadBranch: "feat/x"}}
	obs, err := probe(it, mustParse(t, "2026-09-02T10:00:00Z"))
	if err == nil {
		t.Fatalf("expected a could-not-check error, got observation %+v", obs)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
}
