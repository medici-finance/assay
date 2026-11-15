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
	return gitlabFetchEnvWith(t, origin, apiBase, gitlabEnvOpts{})
}

// gitlabEnvOpts varies the GitLab fixture for the negative-path rows. The zero value is the
// well-formed fixture gitlabFetchEnv builds.
type gitlabEnvOpts struct {
	identity    string      // the worker roster entry; "" is the GitLab-qualified default
	noCustody   bool        // provision no gitlab-worker.token at all
	custodyMode os.FileMode // the custody file's mode; 0 is the owner-only 0600
}

func gitlabFetchEnvWith(t *testing.T, origin, apiBase string, o gitlabEnvOpts) (calls *[][]string, mints *int) {
	t.Helper()
	work := newRepo(t, allowedSlug)
	calls = withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	identity := o.identity
	if identity == "" {
		identity = "worker=gitlab:example-worker:300000006"
	}
	roster := strings.ReplaceAll(fixtureRoster, "worker=assay-worker-app:300000006", identity)
	roster += "ASSAY_REPO_FORGES=" + allowedSlug + "=gitlab\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Setenv("GITLAB_API_BASE", apiBase)
	custody := t.TempDir()
	t.Setenv(deskkit.EnvConfigHome, custody)
	if !o.noCustody {
		tokenFile := filepath.Join(custody, "gitlab-worker.token")
		if err := os.WriteFile(tokenFile, []byte(gitlabFixture), 0o600); err != nil {
			t.Fatal(err)
		}
		if o.custodyMode != 0 {
			// Chmod, not WriteFile's mode: the umask would otherwise mask the loose bits.
			if err := os.Chmod(tokenFile, o.custodyMode); err != nil {
				t.Fatal(err)
			}
		}
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
// configured instance at all, is refused before any fetch — never offered the PAT. The rows
// after "no-instance" pin at the verb what CheckoutGitRead already refuses: a lookalike or
// suffix host, a userinfo origin, a malformed GITLAB_API_BASE, a forge-mismatched identity,
// and a missing or loose custody file. Each also asserts the custody value is nowhere in the
// tool's output or under its HOME (audit ledger included).
//
// regression: #5028
func TestFetchGitLabHostBound(t *testing.T) {
	const base = "https://gitlab.example.invalid:8443/api/v4"
	const path = "/" + allowedSlug + ".git"
	cases := []struct {
		name, origin, base string
		opts               gitlabEnvOpts
		want               int
		why                string // a fragment of the refusal, so a row fails on the wrong reason
	}{
		{"host", "https://elsewhere.invalid:8443" + path, base, gitlabEnvOpts{}, deskkit.ExitRefused, ""},
		{"port", "https://gitlab.example.invalid" + path, base, gitlabEnvOpts{}, deskkit.ExitRefused, ""},
		{"github", githubOrigin, base, gitlabEnvOpts{}, deskkit.ExitRefused, ""},
		{"cleartext", "http://gitlab.example.invalid:8443" + path, base, gitlabEnvOpts{}, deskkit.ExitRefused, ""},
		{"no-instance", gitlabOrigin, "", gitlabEnvOpts{}, deskkit.ExitUnverifiable, ""},
		{"suffix-host", "https://gitlab.example.invalid.attacker.invalid:8443" + path, base,
			gitlabEnvOpts{}, deskkit.ExitRefused, "origin host differs"},
		{"lookalike-host", "https://gitlab-example.invalid:8443" + path, base,
			gitlabEnvOpts{}, deskkit.ExitRefused, "origin host differs"},
		{"userinfo-origin", "https://someone@gitlab.example.invalid:8443" + path, base,
			gitlabEnvOpts{}, deskkit.ExitRefused, "without userinfo"},
		{"base-cleartext", gitlabOrigin, "http://gitlab.example.invalid:8443/api/v4",
			gitlabEnvOpts{}, deskkit.ExitRefused, "HTTPS custody instance"},
		{"base-no-scheme", gitlabOrigin, "gitlab.example.invalid:8443/api/v4",
			gitlabEnvOpts{}, deskkit.ExitUnverifiable, "must be a bare hostname"},
		{"base-userinfo", gitlabOrigin, "https://someone@gitlab.example.invalid:8443/api/v4",
			gitlabEnvOpts{}, deskkit.ExitRefused, "HTTPS custody instance"},
		{"forge-mismatch", gitlabOrigin, base,
			gitlabEnvOpts{identity: "worker=github:assay-worker-app:300000006"}, deskkit.ExitRefused, "forge-mismatched identity"},
		{"no-custody", gitlabOrigin, base,
			gitlabEnvOpts{noCustody: true}, deskkit.ExitRefused, "file not found"},
		{"loose-custody", gitlabOrigin, base,
			gitlabEnvOpts{custodyMode: 0o644}, deskkit.ExitRefused, "must be 0600"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls, mints := gitlabFetchEnvWith(t, c.origin, c.base, c.opts)
			var code int
			out := captureStdoutStderr(t, func() { code = run([]string{"fetch", "--as", "worker"}) })
			if code != c.want {
				t.Fatalf("fetch --as worker origin %s exit = %d, want %d\n%s", c.origin, code, c.want, out)
			}
			if c.why != "" && !strings.Contains(out, c.why) {
				t.Fatalf("refusal does not carry %q:\n%s", c.why, out)
			}
			if fetchArgv(*calls) != nil {
				t.Fatalf("fetch reached the transport for origin %s", c.origin)
			}
			if *mints != 0 {
				t.Fatalf("the GitHub App minter ran %d time(s)", *mints)
			}
			if strings.Contains(out, gitlabFixture) {
				t.Fatal("the GitLab custody value appeared in the tool's output")
			}
			assertNoCustodyUnder(t, os.Getenv("HOME"))
		})
	}
}

// assertNoCustodyUnder fails when the GitLab custody value appears in any file under dir (the
// tool's HOME: roster, audit ledger, kill-switch state). The custody file itself lives in a
// separate config home, so a hit here is a leak.
func assertNoCustodyUnder(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(b), gitlabFixture) {
			t.Errorf("the GitLab custody value was written to %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The GitLab arm refuses a credential CheckoutGitRead bound to a URL other than the one the
// repo gate decided on. The production binding re-reads the same remote.origin.url, so only a
// stubbed seam reaches this branch: the stub binds a different URL and must be refused with
// no transport call and no credential in the output.
//
// regression: #5028
func TestFetchGitLabBoundURLMismatch(t *testing.T) {
	calls, mints := gitlabFetchEnv(t, gitlabOrigin, "https://gitlab.example.invalid:8443/api/v4")
	seamCalls := 0
	old := checkoutGitRead
	checkoutGitRead = func(root, expectedRepo, role string) (gitcore.ListOpts, error) {
		seamCalls++
		return gitcore.ListOpts{
			URL:  "https://gitlab.example.invalid:8443/other-org/other-repo.git",
			Auth: gitcore.BasicAuthAs(gitcore.GitLabGitUsername, gitlabFixture),
		}, nil
	}
	t.Cleanup(func() { checkoutGitRead = old })
	var code int
	out := captureStdoutStderr(t, func() { code = run([]string{"fetch", "--as", "worker"}) })
	if code != deskkit.ExitRefused {
		t.Fatalf("fetch --as worker with a mismatched bound URL exit = %d, want %d\n%s", code, deskkit.ExitRefused, out)
	}
	if seamCalls != 1 {
		t.Fatalf("the GitLab binding seam ran %d time(s), want 1", seamCalls)
	}
	if !strings.Contains(out, "bound to a different origin URL") {
		t.Fatalf("refusal does not name the URL mismatch:\n%s", out)
	}
	if fetchArgv(*calls) != nil || len(fetchRecorded) != 0 {
		t.Fatal("fetch reached the transport with a credential bound to another URL")
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter ran %d time(s)", *mints)
	}
	if strings.Contains(out, gitlabFixture) {
		t.Fatal("the GitLab custody value appeared in the tool's output")
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
