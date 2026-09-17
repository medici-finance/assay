package deskkit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// forgegit_test.go — ForgeGitEndpointFor (#1197): the ONE builder the live claim listing and
// the BranchMoved probe dial through. Fixture forge, fixture custody, no network. The
// credential's DESTINATION host is derived from the resolved forge kind's own canonical
// instance — never a checkout origin (#1197 security review S1) — and validated before it is
// interpolated into the URL authority (S2).

// fixtureGitHubToken is a clearly-fake credential value. It deliberately carries NO real
// GitHub token prefix: the outward-write secret scan reads the branch diff, so a literal
// prefixed string anywhere in a test (or a comment) would trip it (assay#1197's own PR did).
const fixtureGitHubToken = "fixture-installation-token"

func withFixtureMinter(t *testing.T, token string, err error) {
	t.Helper()
	SetGitHubCustodyMinter(func(role string, repo ForgeRepo) (string, string, error) { return token, "", err })
	t.Cleanup(func() { SetGitHubCustodyMinter(nil) })
}

// TestForgeGitEndpointFor_GitHubDialsCanonicalHost: a github-resolved slug dials github.com
// (the kind's canonical instance) with the role's token as the git-basic password.
func TestForgeGitEndpointFor_GitHubDialsCanonicalHost(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/private=github"
	withRoster(t, roster)
	withFixtureMinter(t, fixtureGitHubToken, nil)

	ep, err := ForgeGitEndpointFor("example-org/private", "desk")
	if err != nil {
		t.Fatalf("ForgeGitEndpointFor: %v", err)
	}
	if ep.Kind != ForgeGitHub || ep.Host != "github.com" {
		t.Fatalf("kind/host = %q/%q, want github/github.com", ep.Kind, ep.Host)
	}
	if want := "https://github.com/example-org/private.git"; ep.Opts.URL != want {
		t.Fatalf("URL = %q, want %q", ep.Opts.URL, want)
	}
	ba, ok := ep.Opts.Auth.(*githttp.BasicAuth)
	if !ok || ba.Username != gitcore.GitHubGitUsername || ba.Password != fixtureGitHubToken {
		t.Fatalf("Auth = %#v, want github username with the fixture token", ep.Opts.Auth)
	}
}

// TestForgeGitEndpointFor_GitHubEnterpriseDialsItsOwnHost: a GitHub Enterprise custody base
// (https://ghe.example/api/v3) yields the git host ghe.example — the credential still dials the
// instance the custody path named, not github.com.
func TestForgeGitEndpointFor_GitHubEnterpriseDialsItsOwnHost(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/private=github"
	withRoster(t, roster)
	SetGitHubCustodyMinter(func(role string, repo ForgeRepo) (string, string, error) {
		return fixtureGitHubToken, "https://ghe.example/api/v3", nil
	})
	t.Cleanup(func() { SetGitHubCustodyMinter(nil) })

	ep, err := ForgeGitEndpointFor("example-org/private", "desk")
	if err != nil {
		t.Fatalf("ForgeGitEndpointFor: %v", err)
	}
	if ep.Host != "ghe.example" {
		t.Fatalf("Host = %q, want ghe.example (the custody base's host, not github.com)", ep.Host)
	}
	if want := "https://ghe.example/example-org/private.git"; ep.Opts.URL != want {
		t.Fatalf("URL = %q, want %q", ep.Opts.URL, want)
	}
}

// TestForgeGitEndpointFor_GitLabDialsInstanceFromCustody: a gitlab-resolved slug dials the
// GITLAB_API_BASE instance host with the "oauth2" username and the provisioned PAT.
func TestForgeGitEndpointFor_GitLabDialsInstanceFromCustody(t *testing.T) {
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
	t.Setenv("GITLAB_API_BASE", "https://gitlab.example.test")

	ep, err := ForgeGitEndpointFor("example-org/gitlab-pilot", "desk")
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
	roster[EnvRepoForges] = "example-org/private=github,example-org/gitlab-pilot=gitlab"
	home := withRoster(t, roster)

	t.Run("no credential is an error, not an anonymous endpoint", func(t *testing.T) {
		withFixtureMinter(t, "", errors.New("nothing provisioned"))
		ep, err := ForgeGitEndpointFor("example-org/private", "desk")
		if err == nil {
			t.Fatalf("expected a refusal, got %+v", ep)
		}
		if ep.Opts.Auth != nil {
			t.Fatalf("an endpoint was built without a credential: %+v", ep)
		}
	})
	t.Run("unresolvable forge (unlisted repo)", func(t *testing.T) {
		withFixtureMinter(t, fixtureGitHubToken, nil)
		if _, err := ForgeGitEndpointFor("example-org/unlisted", "desk"); err == nil {
			t.Fatal("expected a refusal for a repo the roster does not name")
		}
	})
	t.Run("gitlab with no configured instance host is could-not-check", func(t *testing.T) {
		tokenFile := filepath.Join(home, ".config", "assay", gitlabTokenFileName("desk"))
		if err := os.WriteFile(tokenFile, []byte("glpat-fixture\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := secureTestRosterPaths(tokenFile); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GITLAB_API_BASE", "")
		ep, err := ForgeGitEndpointFor("example-org/gitlab-pilot", "desk")
		if err == nil {
			t.Fatalf("expected a refusal with no GITLAB_API_BASE, got %+v", ep)
		}
		if ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("exit = %d, want %d: %v", ExitCodeOf(err), ExitUnverifiable, err)
		}
		if ep.Opts.URL != "" {
			t.Fatalf("a URL was built without a known GitLab host: %q", ep.Opts.URL)
		}
	})
	t.Run("bad slug", func(t *testing.T) {
		withFixtureMinter(t, fixtureGitHubToken, nil)
		for _, slug := range []string{"", "nameonly", "a/b/c", "/x", "x/"} {
			if _, err := ForgeGitEndpointFor(slug, "desk"); err == nil {
				t.Errorf("slug %q: expected a refusal", slug)
			}
		}
	})
}

// --- #1197 security review (S1/S2): the credential's DESTINATION host ---------------------

// TestForgeGitEndpointFor_HostIsKindCanonicalNotAnUnrelatedForge is S1: the URL host — where
// the custody credential is presented as an HTTP Basic password — is the RESOLVED KIND's own
// canonical instance, so a GitHub repo dials github.com even when a GitLab instance is
// configured in-process. The GitHub App token can never be dialed at the GitLab host (the
// #1206 crossing shape the reviewer flagged, re-introduced via the host). Since the builder no
// longer reads any checkout origin, there is no unrelated-host input left to leak through.
func TestForgeGitEndpointFor_HostIsKindCanonicalNotAnUnrelatedForge(t *testing.T) {
	roster := goldenRoster()
	roster[EnvRepoForges] = "example-org/private=github"
	withRoster(t, roster)
	withFixtureMinter(t, fixtureGitHubToken, nil)
	// A GitLab instance is configured in this process; it must NOT capture a GitHub repo.
	t.Setenv("GITLAB_API_BASE", "https://gitlab.unrelated.example")

	ep, err := ForgeGitEndpointFor("example-org/private", "desk")
	if err != nil {
		t.Fatalf("ForgeGitEndpointFor: %v", err)
	}
	if ep.Host != "github.com" {
		t.Fatalf("Host = %q, want github.com — a GitHub App token must never be dialed at a GitLab host", ep.Host)
	}
}

// TestValidateGitHostRejectsAuthorityInjection is S2: validateGitHost is the one gate before a
// host is interpolated into the URL authority. It rejects any host carrying userinfo, a path,
// a port, a query/fragment, or whitespace — the scp-like `user@a@evil.test:o/n` shape whose
// authority would otherwise resolve to evil.test with the credential presented to it.
func TestValidateGitHostRejectsAuthorityInjection(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "private"}
	for _, ok := range []string{"github.com", "gitlab.example.test", "ghe.example"} {
		if err := validateGitHost(ok, repo); err != nil {
			t.Errorf("validateGitHost(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{
		"", "  ", "git@a.example@evil.test:o/n", "a@evil.test", "evil.test:8080",
		"evil.test/../x", "evil.test?x=1", "evil.test#frag", "evil.test\\x", "host with space",
	} {
		if err := validateGitHost(bad, repo); err == nil {
			t.Errorf("validateGitHost(%q) = nil, want a refusal", bad)
		} else if ExitCodeOf(err) != ExitUnverifiable {
			t.Errorf("validateGitHost(%q) exit = %d, want %d", bad, ExitCodeOf(err), ExitUnverifiable)
		}
	}
}

// TestForgeGitEndpointFor_MalformedGitLabBaseRefuses is S2 end-to-end: a GITLAB_API_BASE that
// resolves to a non-bare host (no scheme, scp-like) is refused rather than dialed.
func TestForgeGitEndpointFor_MalformedGitLabBaseRefuses(t *testing.T) {
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
	t.Setenv("GITLAB_API_BASE", "git@a.example@evil.test:o/n")

	ep, err := ForgeGitEndpointFor("example-org/gitlab-pilot", "desk")
	if err == nil {
		t.Fatalf("expected a refusal for a malformed GITLAB_API_BASE, got URL %q", ep.Opts.URL)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("exit = %d, want %d: %v", ExitCodeOf(err), ExitUnverifiable, err)
	}
}
