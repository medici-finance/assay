package main

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

// live_test.go — the offline tests behind #1197: readLiveClaims' ref listing must be
// AUTHENTICATED and FORGE-RESOLVED, never an anonymous read against a hardcoded github.com
// (which 404s on every private board root as "authentication required: Repository not
// found" and makes the whole stop/blocked-timeout/reclaim instrument could-not-check).

// TestLiveClaimListNeverHardcodesGitHubHost is the source-level guard: live.go must not
// carry a `https://github.com/` literal. The host is the forge resolver's answer
// (deskkit.ForgeKindFromSlugAndHost, fed the --root checkout's origin host), the same way
// cmd/deskclaim-ref's newForgeStore builds its URL — a compiled-in SaaS host is exactly the
// self-hosted-adopter failure #727 retired from the claim layer.
func TestLiveClaimListNeverHardcodesGitHubHost(t *testing.T) {
	src, err := os.ReadFile("live.go")
	if err != nil {
		t.Fatalf("read live.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `"https://github.com/`) {
			t.Fatalf("live.go:%d hardcodes the GitHub host in a git URL: %s\n"+
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

func basicAuthOf(t *testing.T, opts gitcore.ListOpts) *githttp.BasicAuth {
	t.Helper()
	if opts.Auth == nil {
		t.Fatalf("ListOpts.Auth is nil — the listing would go out anonymous, which is the #1197 404 on every private repo")
	}
	ba, ok := opts.Auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("ListOpts.Auth is %T, want *githttp.BasicAuth", opts.Auth)
	}
	return ba
}

// TestClaimListOptsForPrivateSlugCarriesAuthAndResolvedHost is the issue's Verify row: the
// ListOpts built for a non-public slug carries a non-nil Auth (the session role's token as
// the forge's git-basic credential) and a host the forge resolver derived from the checkout's
// origin — here a fixture host that is NOT github.com, so a hardcoded SaaS host cannot pass.
func TestClaimListOptsForPrivateSlugCarriesAuthAndResolvedHost(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "ghs_fixture_token", nil)

	opts, err := claimListOptsWithHost("example-org/private", "git.example.test")
	if err != nil {
		t.Fatalf("claimListOptsWithHost: %v", err)
	}
	if want := "https://git.example.test/example-org/private.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q (host from the resolver, never a literal)", opts.URL, want)
	}
	ba := basicAuthOf(t, opts)
	if ba.Username != gitcore.GitHubGitUsername {
		t.Fatalf("git username = %q, want %q for a github-resolved repo", ba.Username, gitcore.GitHubGitUsername)
	}
	if ba.Password != "ghs_fixture_token" {
		t.Fatalf("the credential is not the session role's token")
	}
}

// TestClaimListOptsGitLabPairsPATWithOAuthUsername: a gitlab-resolved slug pairs the role's
// provisioned PAT with GitLab's required "oauth2" username — the forge-neutral half of the
// fix, read from custody exactly as ForgeFor's GitLab branch does (a 0600 token file).
func TestClaimListOptsGitLabPairsPATWithOAuthUsername(t *testing.T) {
	dir := withForgeFixture(t, "example-org/gitlab-pilot", "gitlab")
	if err := os.WriteFile(filepath.Join(dir, "gitlab-desk.token"), []byte("glpat-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts, err := claimListOptsWithHost("example-org/gitlab-pilot", "gitlab.example.test")
	if err != nil {
		t.Fatalf("claimListOptsWithHost: %v", err)
	}
	if want := "https://gitlab.example.test/example-org/gitlab-pilot.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q", opts.URL, want)
	}
	ba := basicAuthOf(t, opts)
	if ba.Username != gitcore.GitLabGitUsername || ba.Password != "glpat-fixture" {
		t.Fatalf("git credential = %q:<%d bytes>, want %q:<%d bytes>", ba.Username, len(ba.Password), gitcore.GitLabGitUsername, len("glpat-fixture"))
	}
}

// TestClaimListOptsMissingTokenIsCouldNotCheck: no token means no listing — exit 6, never an
// anonymous attempt whose empty/404 answer could read as "no claims held".
func TestClaimListOptsMissingTokenIsCouldNotCheck(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "", errors.New("no App credential provisioned in this fixture"))

	opts, err := claimListOptsWithHost("example-org/private", "git.example.test")
	if err == nil {
		t.Fatalf("expected a could-not-check refusal, got ListOpts %+v", opts)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
}

// TestClaimListOptsNoOriginHostIsCouldNotCheck: with no readable origin host the resolver
// refuses rather than defaulting to the SaaS instance (#727) — the roster names the forge
// SOFTWARE, never WHERE it is.
func TestClaimListOptsNoOriginHostIsCouldNotCheck(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "ghs_fixture_token", nil)

	opts, err := claimListOptsWithHost("example-org/private", "")
	if err == nil {
		t.Fatalf("expected a could-not-check refusal for an unknown instance host, got ListOpts %+v", opts)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
	if strings.Contains(opts.URL, "github.com") {
		t.Fatalf("a SaaS host was defaulted into the URL: %q", opts.URL)
	}
}
