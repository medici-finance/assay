package main

// roleinitgitlabcred_test.go — role-init's credential SELECTION on a GitLab-served repo (#1573).
//
// THE DEFECT. role-init answered git's credential request for a GitLab-bound role with the
// GitLab username (`oauth2`), but resolved the token FILE through the GitHub App minter
// (deskkit.RoleTokenForOwner → `desktoken <role> --repo <owner>`, no `--forge gitlab`). On a
// GitLab deployment there is no App to mint, so role-init stopped at exit 6 asking for
// DESK_APP_ID / apps.env instead of reading the role's provisioned `gitlab-<role>.token`.
//
// WHY THE EXISTING SUITE MISSED IT. withEnv stubs the credential seam for every role-init test,
// so no test ever reached the production resolver. The tests here put the PRODUCTION resolver
// back (productionRoleCredential, captured at package init before any stub — deskkit's
// forge-aware ResolveRoleCredential since the #1573 follow-up) and install a minter DETECTOR in
// deskkit's own seam: a call to the GitHub minter on a GitLab repo is a test failure, not a
// fixture token.
//
// Pinned: a GitLab-mapped repo reads the custody file and never calls the GitHub minter; a
// missing custody file is REFUSED (exit 5) naming the file, again without the GitHub minter and
// without wiring any helper; a GitHub repo still goes through the GitHub minter exactly as before.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// productionRoleCredential is roleCredential as the package initialises it — deskkit's real
// forge-aware credential resolver — captured before withEnv swaps in its fixture stub.
var productionRoleCredential = roleCredential

const (
	gitlabFixtureHost  = "gitlab.example.com"
	gitlabFixtureToken = "fake-gitlab-role-token-for-tests"
	gitlabFixtureEmail = "service_account_group_1_ab12cd@noreply.gitlab.example.com"
)

// githubMinterDetector restores the production credential resolver, wrapped in counters, and
// installs a fake deskkit token minter so nothing ever shells to a real `desktoken`. The fake
// minter fails the way a GitLab-only deployment's does (no App ID) unless mintPath is set, in
// which case it hands back that path. githubArm counts resolutions that selected the GitHub App
// arm; minterCalls counts forks of the GitHub minter itself — so a test can assert the GitHub
// path was, or was not, taken.
func githubMinterDetector(t *testing.T, mintPath string) (githubArm, minterCalls *int) {
	t.Helper()
	ga, mc := 0, 0
	prev := roleCredential
	roleCredential = func(role string, repo deskkit.ForgeRepo, originURL string) (deskkit.RoleCredential, error) {
		cred, err := productionRoleCredential(role, repo, originURL)
		if cred.Resolution.Kind == deskkit.ForgeGitHub {
			ga++
		}
		return cred, err
	}
	t.Cleanup(func() { roleCredential = prev })
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mc++
		if mintPath != "" {
			return mintPath, "", nil
		}
		return "", `no App ID for App "` + role + `-app": set DESK_APP_ID`, errors.New("exit status 6")
	})
	t.Cleanup(restore)
	return &ga, &mc
}

// gitlabRepoRoster binds the verifier role to a GitLab identity AND maps the fixture repo to
// GitLab in ASSAY_REPO_FORGES — the adopter shape from the issue — then points the origin at an
// https GitLab host so the helper has a host to be scoped to.
func gitlabRepoRoster(t *testing.T, work string) {
	t.Helper()
	r := strings.ReplaceAll(fixtureRoster,
		"verifier=assay-verifier-app:300000005",
		"verifier=gitlab:assay-verifier-sa:41987965")
	r += "ASSAY_GITLAB_SESSION_EMAILS=" + gitlabFixtureEmail + "\n"
	r += "ASSAY_REPO_FORGES=example-org/tracker=gitlab\n"
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(r), 0o600); err != nil {
		t.Fatalf("re-plant GitLab roster: %v", err)
	}
	deskkit.ReloadConfig()
	mustGit(t, work, "remote", "set-url", "origin", "https://"+gitlabFixtureHost+"/example-org/tracker.git")
}

// plantGitLabCustody writes the role's provisioned PAT custody file (0600) on the default
// App-credential search path under the fixture HOME and returns its path.
func plantGitLabCustody(t *testing.T, role, value string) string {
	t.Helper()
	p := filepath.Join(os.Getenv("HOME"), ".config", "assay", "gitlab-"+role+".token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatalf("plant gitlab custody: %v", err)
	}
	return p
}

func TestRoleInitGitLabReadsCustodyNeverGitHubMinter(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv(deskkit.EnvConfigHome, "")
	gitlabRepoRoster(t, work)
	custody := plantGitLabCustody(t, "verifier", gitlabFixtureToken)
	githubArm, minterCalls := githubMinterDetector(t, "")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "glcred", "--no-fetch"})
	if *githubArm != 0 || *minterCalls != 0 {
		t.Fatalf("GitHub App token resolver called %d time(s) (minter forked %d) for a GitLab-mapped repo; rc=%d stderr: %s",
			*githubArm, *minterCalls, rc, stderr)
	}
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on a GitLab-mapped repo rc = %d, want 0; stderr: %s", rc, stderr)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-glcred")
	scopedKey := "credential.https://" + gitlabFixtureHost + ".helper"
	helper, err := exec.Command("git", "-C", target, "config", "--worktree", "--get-all", scopedKey).Output()
	if err != nil {
		t.Fatalf("get-all %s: %v", scopedKey, err)
	}
	if !strings.Contains(string(helper), "username=oauth2") || !strings.Contains(string(helper), "'"+custody+"'") {
		t.Fatalf("%s = %q; want the oauth2 helper reading the GitLab custody file %s", scopedKey, helper, custody)
	}
	if strings.Contains(string(helper), gitlabFixtureToken) {
		t.Fatalf("the token VALUE was written into config: %q", helper)
	}
	if fill := credentialFill(t, target, "https", gitlabFixtureHost); !strings.Contains(fill, "username=oauth2") ||
		!strings.Contains(fill, "password="+gitlabFixtureToken) {
		t.Fatalf("git credential fill for https://%s did not answer with the GitLab custody token:\n%s", gitlabFixtureHost, fill)
	}
	// Still host-scoped: a foreign host gets nothing.
	if fill := credentialFill(t, target, "https", "example.invalid"); strings.Contains(fill, gitlabFixtureToken) {
		t.Fatalf("a foreign host received the GitLab token:\n%s", fill)
	}
}

func TestRoleInitGitLabMissingCustodyFailsClosed(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv(deskkit.EnvConfigHome, "")
	gitlabRepoRoster(t, work)
	githubArm, minterCalls := githubMinterDetector(t, "")
	pfRan := false
	roleInitPreflight = func(deskkit.PreflightRequest) error { pfRan = true; return nil }

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "glnone", "--no-fetch"})
	if *githubArm != 0 || *minterCalls != 0 {
		t.Fatalf("missing GitLab custody fell through to the GitHub App token resolver (%d call(s), %d fork(s)); rc=%d stderr: %s",
			*githubArm, *minterCalls, rc, stderr)
	}
	if rc != deskkit.ExitRefused {
		t.Fatalf("role-init with no GitLab custody file rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	if !strings.Contains(stderr, "gitlab-verifier.token") {
		t.Fatalf("the refusal must name the expected custody file; stderr: %s", stderr)
	}
	if pfRan {
		t.Fatalf("the preflight must not run when the credential could not be wired")
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-glnone")
	if out, _ := exec.Command("git", "-C", target, "config", "--worktree", "--list").Output(); strings.Contains(string(out), "credential.https://") {
		t.Fatalf("a credential helper was wired despite the missing custody file:\n%s", out)
	}
}

func TestRoleInitGitHubStillUsesAppMinter(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	t.Setenv(deskkit.EnvConfigHome, "")
	mustGit(t, work, "remote", "set-url", "origin", "https://github.com/example-org/tracker.git")
	// A GitLab custody file for the SAME role is present: the GitHub path must not read it.
	plantGitLabCustody(t, "verifier", gitlabFixtureToken)
	ghPath := filepath.Join(os.Getenv("HOME"), "verifier-app-token-fixture")
	if err := os.WriteFile(ghPath, []byte(fixtureTokenValue+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	githubArm, minterCalls := githubMinterDetector(t, ghPath)

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "ghcred", "--no-fetch"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on a GitHub repo rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if *githubArm != 1 {
		t.Fatalf("GitHub App arm selected %d time(s), want exactly 1", *githubArm)
	}
	// The minter fork itself may be a memo hit from an earlier test in this process; the
	// resolver call above is the load-bearing assertion. It must never be more than one.
	if *minterCalls > 1 {
		t.Fatalf("GitHub minter forked %d times, want at most 1", *minterCalls)
	}
	target := filepath.Join(tmpBaseDir, "tracker-verify-desk-ghcred")
	fill := credentialFill(t, target, "https", "github.com")
	if !strings.Contains(fill, "username=x-access-token") || !strings.Contains(fill, "password="+fixtureTokenValue) {
		t.Fatalf("git credential fill for https://github.com did not answer with the App token:\n%s", fill)
	}
	if strings.Contains(fill, gitlabFixtureToken) {
		t.Fatalf("the GitHub path read the GitLab custody file:\n%s", fill)
	}
}

// TestRoleInitNonGitHubOriginHostRefusedBeforeMint is sec-1587-S1 at the role-init binding.
// role-init scopes its helper to the ORIGIN's host, so a GitHub App token resolved for an origin
// whose host is not exactly github.com would be wired under — and offered to — that host. With
// the roster silent (the unresolved default) or naming the forge "github", such an origin is
// REFUSED (exit 5) before the GitHub minter runs, and no helper is wired.
func TestRoleInitNonGitHubOriginHostRefusedBeforeMint(t *testing.T) {
	for i, tc := range []struct{ rosterForge, origin string }{
		{"", "https://" + gitlabFixtureHost + "/example-org/tracker.git"},
		{"", "https://github.com.evil.test/example-org/tracker.git"},
		{"", "https://github.com@evil.test/example-org/tracker.git"},
		{"github", "https://github.example.com/example-org/tracker.git"},
	} {
		session := fmt.Sprintf("hostref%d", i)
		t.Run(session, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			t.Setenv(deskkit.EnvConfigHome, "")
			if tc.rosterForge != "" {
				r := fixtureRoster + "ASSAY_REPO_FORGES=example-org/tracker=" + tc.rosterForge + "\n"
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(r), 0o600); err != nil {
					t.Fatal(err)
				}
				deskkit.ReloadConfig()
			}
			mustGit(t, work, "remote", "set-url", "origin", tc.origin)
			ghPath := filepath.Join(os.Getenv("HOME"), "verifier-app-token-fixture")
			if err := os.WriteFile(ghPath, []byte(fixtureTokenValue+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, minterCalls := githubMinterDetector(t, ghPath)
			pfRan := false
			roleInitPreflight = func(deskkit.PreflightRequest) error { pfRan = true; return nil }

			rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", session, "--no-fetch"})
			if rc != deskkit.ExitRefused || !strings.Contains(stderr, deskkit.EnvRepoForges) {
				t.Fatalf("role-init on origin %s rc = %d, want 5 (refused, naming %s); stderr: %s",
					tc.origin, rc, deskkit.EnvRepoForges, stderr)
			}
			if *minterCalls != 0 {
				t.Fatalf("the GitHub minter forked %d time(s) for origin %s", *minterCalls, tc.origin)
			}
			if pfRan {
				t.Fatalf("the preflight must not run when the credential could not be wired")
			}
			target := filepath.Join(tmpBaseDir, "tracker-verify-desk-"+session)
			if out, _ := exec.Command("git", "-C", target, "config", "--worktree", "--list").Output(); strings.Contains(string(out), "credential.https://") ||
				strings.Contains(string(out), fixtureTokenValue) {
				t.Fatalf("a credential helper was wired for origin %s:\n%s", tc.origin, out)
			}
		})
	}
}

// TestRoleInitHostlessOriginResolvesNoCredential: a hostless (local-transport) origin consults
// no credential helper, so role-init wires nothing — and, with the PRODUCTION resolver in place,
// resolves nothing either: the GitHub minter never runs, and the forge-aware resolver's refusal
// of an origin with no host is never reached, so a local clone still provisions (rc 0).
func TestRoleInitHostlessOriginResolvesNoCredential(t *testing.T) {
	work := newRepo(t) // origin is a local bare PATH, no host
	withEnv(t, work)
	t.Setenv(deskkit.EnvConfigHome, "")
	githubArm, minterCalls := githubMinterDetector(t, "")

	rc, stderr := runCapErr(t, []string{"role-init", "--role", "verifier", "--session", "hostless", "--no-fetch"})
	if rc != deskkit.ExitOK {
		t.Fatalf("role-init on a hostless origin rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if *githubArm != 0 || *minterCalls != 0 {
		t.Fatalf("a credential was resolved for a hostless origin (GitHub arm %d, minter forks %d)", *githubArm, *minterCalls)
	}
}
