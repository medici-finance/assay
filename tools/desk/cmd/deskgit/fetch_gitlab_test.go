package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fetch_gitlab_test.go — `deskgit fetch --as <role>` on a GitLab-served repo (#5028).
//
// The forge map (ASSAY_REPO_FORGES) decides which custody the role credential comes from.
// GitLab: the role's provisioned gitlab-<role>.token, offered as "oauth2" basic auth and only
// to the configured GitLab instance (GITLAB_API_BASE host and port). GitHub: unchanged. A
// forge neither the roster nor the origin host names: still refused, exit 5. The transport is
// the fetch seam (fetchFn) recording what it was handed, so nothing dials out.

const gitlabOrigin = "https://gitlab.example.invalid:8443/" + allowedSlug + ".git"

const gitlabFixture = "gitlab-fetch-fixture"

// gitlabFetchEnv points work's origin at origin, maps the repo to GitLab in the roster (with
// a GitLab-qualified worker identity), provisions the worker's GitLab custody file and counts
// every GitHub App mint. The fetch seam records and does not dial.
func gitlabFetchEnv(t *testing.T, origin, apiBase string) (calls *[][]string, mints *int) {
	t.Helper()
	work := newRepo(t, allowedSlug)
	calls = withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	roster := strings.ReplaceAll(fixtureRoster, "worker=assay-worker-app:300000006", "worker=gitlab:example-worker:300000006")
	roster += "ASSAY_REPO_FORGES=" + allowedSlug + "=gitlab\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Setenv("GITLAB_API_BASE", apiBase)
	custody := t.TempDir()
	t.Setenv(deskkit.EnvConfigHome, custody)
	if err := os.WriteFile(filepath.Join(custody, "gitlab-worker.token"), []byte(gitlabFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, work, "remote", "set-url", "origin", origin)
	n := 0
	mints = &n
	t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		n++
		return "", "no GitHub App for this fixture", errors.New("exit status 6")
	}))
	fetchFn = func(_ string, opts gitcore.FetchOpts) error {
		fetchRecorded = append(fetchRecorded, opts)
		*calls = append(*calls, []string{"gitcore", "fetch", opts.URL})
		return nil
	}
	return calls, mints
}

// regression: #5028
func TestFetchAsGitLabRole(t *testing.T) {
	_, mints := gitlabFetchEnv(t, gitlabOrigin, "https://gitlab.example.invalid:8443/api/v4")
	if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("fetch --as worker on a GitLab-served origin exit = %d, want 0", code)
	}
	got := onlyFetch(t)
	if got.URL != gitlabOrigin {
		t.Fatalf("fetch URL = %q, want the gated origin %q", got.URL, gitlabOrigin)
	}
	if len(got.RefSpecs) != 1 || got.RefSpecs[0] != trackingRefspec {
		t.Fatalf("fetch refspecs = %v, want only %s", got.RefSpecs, trackingRefspec)
	}
	auth, ok := got.Auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatal("fetch carried no basic-auth credential for the GitLab role")
	}
	if auth.Username != gitcore.GitLabGitUsername {
		t.Fatalf("basic-auth username = %q, want %q", auth.Username, gitcore.GitLabGitUsername)
	}
	if auth.Password != gitlabFixture {
		t.Fatal("fetch credential is not the worker's GitLab custody token")
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter ran %d time(s) for a GitLab-served origin", *mints)
	}
}

// The GitLab credential goes only to the configured instance: another host or port, or no
// configured instance at all, is refused before any fetch — never offered the PAT.
//
// regression: #5028
func TestFetchGitLabHostBound(t *testing.T) {
	cases := []struct {
		name, origin, base string
		want               int
	}{
		{"host", "https://elsewhere.invalid:8443/" + allowedSlug + ".git", "https://gitlab.example.invalid:8443/api/v4", deskkit.ExitRefused},
		{"port", "https://gitlab.example.invalid/" + allowedSlug + ".git", "https://gitlab.example.invalid:8443/api/v4", deskkit.ExitRefused},
		{"github", githubOrigin, "https://gitlab.example.invalid:8443/api/v4", deskkit.ExitRefused},
		{"cleartext", "http://gitlab.example.invalid:8443/" + allowedSlug + ".git", "https://gitlab.example.invalid:8443/api/v4", deskkit.ExitRefused},
		{"no-instance", gitlabOrigin, "", deskkit.ExitUnverifiable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls, mints := gitlabFetchEnv(t, c.origin, c.base)
			if code := run([]string{"fetch", "--as", "worker"}); code != c.want {
				t.Fatalf("fetch --as worker origin %s exit = %d, want %d", c.origin, code, c.want)
			}
			if fetchArgv(*calls) != nil {
				t.Fatalf("fetch reached the transport for origin %s", c.origin)
			}
			if *mints != 0 {
				t.Fatalf("the GitHub App minter ran %d time(s)", *mints)
			}
		})
	}
}

// A forge nobody recognises — roster silent, origin host not a well-known forge — keeps the
// refusal: exit 5, no fetch, no credential read.
//
// regression: #5028
func TestFetchUnknownForgeRefused(t *testing.T) {
	work := newRepo(t, allowedSlug)
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	mustGit(t, work, "remote", "set-url", "origin", "https://forge.example.invalid/"+allowedSlug+".git")
	mints := 0
	t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mints++
		return "", "no GitHub App for this fixture", errors.New("exit status 6")
	}))
	if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitRefused {
		t.Fatalf("fetch --as worker on an unrecognised forge exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("fetch reached the transport for an unrecognised forge")
	}
	if mints != 0 {
		t.Fatalf("a credential was read %d time(s) for an unrecognised forge", mints)
	}
}

// GitHub behaviour is unchanged: the GitHub App token, GitHub's basic-auth username, the
// origin URL verbatim.
//
// regression: #5028
func TestFetchGitHubUnchanged(t *testing.T) {
	work := newRepo(t, allowedSlug)
	mustGit(t, work, "remote", "set-url", "origin", githubOrigin)
	withEnv(t, work)
	asWorker(t)
	roleTokenForRepo = func(role, repo string, dests []string) (string, string, error) {
		if role != "worker" || repo != allowedSlug || len(dests) != 1 || dests[0] != githubOrigin {
			t.Fatalf("token bound to %s/%s for %v", role, repo, dests)
		}
		return fixtureToken, "", nil
	}
	fetchFn = func(_ string, opts gitcore.FetchOpts) error {
		fetchRecorded = append(fetchRecorded, opts)
		return nil
	}
	if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("fetch --as worker on a GitHub origin exit = %d, want 0", code)
	}
	got := onlyFetch(t)
	auth, ok := got.Auth.(*githttp.BasicAuth)
	if got.URL != githubOrigin || !ok || auth.Username != gitcore.GitHubGitUsername || auth.Password != fixtureToken {
		t.Fatal("GitHub fetch destination, username or credential changed")
	}
}
