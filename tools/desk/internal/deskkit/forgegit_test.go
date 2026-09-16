package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// forgegit_test.go — ForgeGitEndpointFor (#1197): the ONE builder the live claim listing and
// the BranchMoved probe dial through. Fixture forge, fixture custody, no network.

// fixtureGitHubToken is a clearly-fake credential value. It deliberately carries NO real
// GitHub token prefix: the outward-write secret scan reads the branch diff, so a literal
// prefixed string anywhere in a test (or a comment) would trip it (assay#1197's own PR did).
const fixtureGitHubToken = "fixture-installation-token"

func withFixtureMinter(t *testing.T, token string, err error) {
	t.Helper()
	SetGitHubCustodyMinter(func(role string, repo ForgeRepo) (string, string, error) { return token, "", err })
	t.Cleanup(func() { SetGitHubCustodyMinter(nil) })
}

func TestForgeGitEndpointFor_GitHubCarriesRoleTokenOnResolvedHost(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/private=github"
	withRoster(t, roster)
	withFixtureMinter(t, fixtureGitHubToken, nil)

	ep, err := ForgeGitEndpointFor("example-org/private", "desk", "ghe.example.test")
	if err != nil {
		t.Fatalf("ForgeGitEndpointFor: %v", err)
	}
	if ep.Kind != ForgeGitHub || ep.Host != "ghe.example.test" {
		t.Fatalf("kind/host = %q/%q, want github/ghe.example.test", ep.Kind, ep.Host)
	}
	if want := "https://ghe.example.test/example-org/private.git"; ep.Opts.URL != want {
		t.Fatalf("URL = %q, want %q", ep.Opts.URL, want)
	}
	ba, ok := ep.Opts.Auth.(*githttp.BasicAuth)
	if !ok || ba.Username != gitcore.GitHubGitUsername || ba.Password != fixtureGitHubToken {
		t.Fatalf("Auth = %#v, want github username with the fixture token", ep.Opts.Auth)
	}
}

func TestForgeGitEndpointFor_GitLabUsesOauthUsernameAndToken(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/gitlab-pilot=gitlab"
	home := withRoster(t, roster)
	tokenFile := filepath.Join(home, ".config", "assay", gitlabTokenFileName("desk"))
	if err := os.WriteFile(tokenFile, []byte("glpat-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureTestRosterPaths(tokenFile); err != nil {
		t.Fatal(err)
	}

	ep, err := ForgeGitEndpointFor("example-org/gitlab-pilot", "desk", "gitlab.example.test")
	if err != nil {
		t.Fatalf("ForgeGitEndpointFor: %v", err)
	}
	if ep.Kind != ForgeGitLab || ep.Opts.URL != "https://gitlab.example.test/example-org/gitlab-pilot.git" {
		t.Fatalf("kind/URL = %q/%q", ep.Kind, ep.Opts.URL)
	}
	ba, ok := ep.Opts.Auth.(*githttp.BasicAuth)
	if !ok || ba.Username != gitcore.GitLabGitUsername || ba.Password != "glpat-fixture" {
		t.Fatalf("Auth = %#v, want oauth2 with the provisioned PAT", ep.Opts.Auth)
	}
}

// TestForgeGitEndpointFor_FailsClosed: every gap is an error, never an anonymous or
// SaaS-defaulted endpoint.
func TestForgeGitEndpointFor_FailsClosed(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/private=github"
	withRoster(t, roster)

	t.Run("no origin host is never a SaaS default", func(t *testing.T) {
		withFixtureMinter(t, fixtureGitHubToken, nil)
		ep, err := ForgeGitEndpointFor("example-org/private", "desk", "")
		if err == nil {
			t.Fatalf("expected a refusal, got %+v", ep)
		}
		if ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("exit = %d, want %d: %v", ExitCodeOf(err), ExitUnverifiable, err)
		}
	})
	t.Run("no credential is an error, not an anonymous endpoint", func(t *testing.T) {
		withFixtureMinter(t, "", errors.New("nothing provisioned"))
		ep, err := ForgeGitEndpointFor("example-org/private", "desk", "ghe.example.test")
		if err == nil {
			t.Fatalf("expected a refusal, got %+v", ep)
		}
		if ep.Opts.Auth != nil {
			t.Fatalf("an endpoint was built without a credential: %+v", ep)
		}
	})
	t.Run("unresolvable forge", func(t *testing.T) {
		withFixtureMinter(t, fixtureGitHubToken, nil)
		if _, err := ForgeGitEndpointFor("example-org/unlisted", "desk", "ghe.example.test"); err == nil {
			t.Fatal("expected a refusal for a repo neither the roster nor the host table resolves")
		}
	})
	t.Run("bad slug", func(t *testing.T) {
		withFixtureMinter(t, fixtureGitHubToken, nil)
		for _, slug := range []string{"", "nameonly", "a/b/c", "/x", "x/"} {
			if _, err := ForgeGitEndpointFor(slug, "desk", "ghe.example.test"); err == nil {
				t.Errorf("slug %q: expected a refusal", slug)
			}
		}
	})
}

// TestOriginRemoteHost reads the origin host of a real (local, throwaway) checkout in-process
// and reports the git host, never the owner/name — no network is contacted.
func TestOriginRemoteHost(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "https://ghe.example.test/example-org/private.git"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	host, err := OriginRemoteHost(dir)
	if err != nil {
		t.Fatalf("OriginRemoteHost: %v", err)
	}
	if host != "ghe.example.test" {
		t.Fatalf("host = %q, want ghe.example.test", host)
	}
	if _, err := OriginRemoteHost(t.TempDir()); err == nil {
		t.Fatal("a directory with no repository must not yield a host")
	}
}
