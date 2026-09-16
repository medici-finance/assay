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
// found" and makes the whole stop/blocked-timeout/reclaim instrument could-not-check). The
// credential is dialed at the RESOLVED KIND's canonical instance host, never a host read from
// the checkout the desk runs in or was pointed at (#1197 security review S1).

// TestLiveClaimListNeverHardcodesGitHubHost is the source-level guard: live.go must not
// carry a `https://github.com/` literal. The host comes from deskkit's forge resolution
// (ForgeGitEndpointFor), never a compiled-in literal — a hardcoded SaaS host is the
// self-hosted-adopter failure #727 retired from the claim layer.
func TestLiveClaimListNeverHardcodesGitHubHost(t *testing.T) {
	src, err := os.ReadFile("live.go")
	if err != nil {
		t.Fatalf("read live.go: %v", err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.Contains(line, `"https://github.com/`) {
			t.Fatalf("live.go:%d hardcodes the GitHub host in a git URL: %s\n"+
				"— the host must come from the forge resolver (deskkit.ForgeGitEndpointFor), never a literal", i+1, strings.TrimSpace(line))
		}
	}
}

// withForgeFixture installs a roster naming slug's forge into a private config home (the
// REAL loader, file + permissions + parse), pins the session loop to the-desk (role "desk"),
// and reloads. No git checkout and no network: WHICH forge comes from the roster, and WHERE
// the instance lives comes from the forge kind's canonical host (github.com) or, for GitLab,
// GITLAB_API_BASE — never a checkout origin.
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

// TestClaimListOptsForPrivateSlugCarriesAuthAndCanonicalHost is the issue's Verify row: the
// ListOpts built for a non-public GitHub slug carries a non-nil Auth (the session role's token
// as the forge's git-basic credential) and the GitHub canonical host — never an anonymous or
// hardcoded endpoint.
func TestClaimListOptsForPrivateSlugCarriesAuthAndCanonicalHost(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "fixture-installation-token", nil)

	opts, err := claimListOpts("example-org/private")
	if err != nil {
		t.Fatalf("claimListOpts: %v", err)
	}
	if want := "https://github.com/example-org/private.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q (kind-canonical host, never a checkout origin)", opts.URL, want)
	}
	ba := basicAuthOf(t, opts)
	if ba.Username != gitcore.GitHubGitUsername {
		t.Fatalf("git username = %q, want %q for a github-resolved repo", ba.Username, gitcore.GitHubGitUsername)
	}
	if ba.Password != "fixture-installation-token" {
		t.Fatalf("the credential is not the session role's token")
	}
}

// TestClaimListOptsGitLabPairsTokenWithOauthUsername: a gitlab-resolved slug pairs the role's
// provisioned PAT with GitLab's required "oauth2" username, dialed at the GITLAB_API_BASE
// instance host — never a checkout origin, never a gitlab.com default.
func TestClaimListOptsGitLabPairsTokenWithOauthUsername(t *testing.T) {
	dir := withForgeFixture(t, "example-org/gitlab-pilot", "gitlab")
	if err := os.WriteFile(filepath.Join(dir, "gitlab-desk.token"), []byte("glpat-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.test")

	opts, err := claimListOpts("example-org/gitlab-pilot")
	if err != nil {
		t.Fatalf("claimListOpts: %v", err)
	}
	if want := "https://gitlab.example.test/example-org/gitlab-pilot.git"; opts.URL != want {
		t.Fatalf("ListOpts.URL = %q, want %q", opts.URL, want)
	}
	ba := basicAuthOf(t, opts)
	if ba.Username != gitcore.GitLabGitUsername || ba.Password != "glpat-fixture" {
		t.Fatalf("git credential = %q:<%d bytes>, want %q:<%d bytes>", ba.Username, len(ba.Password), gitcore.GitLabGitUsername, len("glpat-fixture"))
	}
}

// TestClaimListOptsGitLabWithoutInstanceHostIsCouldNotCheck: a gitlab-resolved slug with no
// GITLAB_API_BASE configured must refuse (exit 6), never default to gitlab.com and present the
// PAT to a host the repo may not live on (#727 / #1197 security review S1).
func TestClaimListOptsGitLabWithoutInstanceHostIsCouldNotCheck(t *testing.T) {
	dir := withForgeFixture(t, "example-org/gitlab-pilot", "gitlab")
	if err := os.WriteFile(filepath.Join(dir, "gitlab-desk.token"), []byte("glpat-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITLAB_API_BASE", "")

	opts, err := claimListOpts("example-org/gitlab-pilot")
	if err == nil {
		t.Fatalf("expected a could-not-check refusal with no GITLAB_API_BASE, got ListOpts %+v", opts)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
	if strings.Contains(opts.URL, "gitlab.com") {
		t.Fatalf("a SaaS host was defaulted into the URL: %q", opts.URL)
	}
}

// TestClaimListOptsMissingTokenIsCouldNotCheck: no token means no listing — exit 6, never an
// anonymous attempt whose empty/404 answer could read as "no claims held".
func TestClaimListOptsMissingTokenIsCouldNotCheck(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "", errors.New("no App credential provisioned in this fixture"))

	opts, err := claimListOpts("example-org/private")
	if err == nil {
		t.Fatalf("expected a could-not-check refusal, got ListOpts %+v", opts)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
}

// TestClaimListOptsUnlistedRepoIsCouldNotCheck: a slug the roster does not name has no
// resolvable forge — refuse (exit 6), never guess from an unrelated origin.
func TestClaimListOptsUnlistedRepoIsCouldNotCheck(t *testing.T) {
	withForgeFixture(t, "example-org/private", "github")
	withFixtureGitHubMinter(t, "fixture-installation-token", nil)

	opts, err := claimListOpts("example-org/not-in-roster")
	if err == nil {
		t.Fatalf("expected a could-not-check refusal for an unlisted repo, got ListOpts %+v", opts)
	}
	if code := deskkit.ExitCodeOf(err); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (Unverifiable): %v", code, deskkit.ExitUnverifiable, err)
	}
}
