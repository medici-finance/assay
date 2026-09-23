package deskkit

// rolecredential_test.go — the forge-aware role-credential entry points (#1573 follow-up):
// ResolveRoleCredential selects the custody by forge, and GitHubRoleToken /
// GitHubRoleTokenForRemote refuse a repo another forge serves instead of minting a GitHub App
// token for it. Each test installs a minter DETECTOR in the real token seam, so "the GitHub
// minter never ran" is a counted fact, not an inference.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minterDetector resets the per-process memo (so an earlier test's mint cannot hide a call as
// a memo hit) and installs a fake minter that counts forks. When path is empty the fake fails
// the way a GitLab-only deployment's minter does (no App ID).
func minterDetector(t *testing.T, path string) *int {
	t.Helper()
	resetRoleTokenMemo()
	t.Cleanup(resetRoleTokenMemo)
	n := 0
	t.Cleanup(SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		n++
		if path == "" {
			return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
		}
		return path, "", nil
	}))
	return &n
}

// plantCustody writes a role's GitLab PAT custody file (0600) in a private credential dir and
// points the App-credential search path at it.
func plantCustody(t *testing.T, role, value string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gitlab-"+role+".token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigHome, dir)
	return p
}

func plantAppToken(t *testing.T, value string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worker-app-token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func rosterWithForge(t *testing.T, slug, kind string) {
	t.Helper()
	roster := goldenRoster()
	if slug != "" {
		roster[EnvRepoForges] = slug + "=" + kind
	}
	withRoster(t, roster)
}

func TestRoleCredential_GitLabReadsCustody_NoMint(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	custody := plantCustody(t, "worker", "gitlab-pat-stub")
	mints := minterDetector(t, "")

	cred, err := ResolveRoleCredential("worker", repo, "")
	if err != nil {
		t.Fatalf("ResolveRoleCredential on a GitLab-mapped repo: %v", err)
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter forked %d time(s) for a GitLab-mapped repo", *mints)
	}
	if cred.Resolution.Kind != ForgeGitLab || cred.Path != custody || cred.Token != "gitlab-pat-stub" {
		t.Fatalf("got kind=%q path=%q; want gitlab custody %s", cred.Resolution.Kind, cred.Path, custody)
	}
}

func TestRoleCredential_GitLabMissingCustody_Refused(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	t.Setenv(EnvConfigHome, t.TempDir()) // an empty search path: nothing provisioned
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

	_, err := ResolveRoleCredential("worker", repo, "")
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(errString(err), "gitlab-worker.token") {
		t.Fatalf("missing custody: err = %v (exit %d); want a refusal naming gitlab-worker.token", err, ExitCodeOf(err))
	}
	if *mints != 0 {
		t.Fatalf("missing GitLab custody fell through to the GitHub App minter (%d fork(s))", *mints)
	}
}

func TestRoleCredential_GitHubAndUnresolved_MintAppToken(t *testing.T) {
	for _, tc := range []struct{ name, slugForge, origin string }{
		{"roster says github", "github", ""},
		{"origin host says github", "", "https://github.com/example-org/tracker.git"},
		{"unresolved: historical GitHub default", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
			if tc.slugForge != "" {
				rosterWithForge(t, repo.Slug(), tc.slugForge)
			} else {
				rosterWithForge(t, "", "")
			}
			plantCustody(t, "worker", "gitlab-pat-stub") // present, and must NOT be read
			app := plantAppToken(t, "app-token-stub")
			mints := minterDetector(t, app)

			cred, err := ResolveRoleCredential("worker", repo, tc.origin)
			if err != nil {
				t.Fatalf("ResolveRoleCredential: %v", err)
			}
			if *mints != 1 || cred.Resolution.Kind != ForgeGitHub || cred.Path != app || cred.Token != "app-token-stub" {
				t.Fatalf("got kind=%q path=%q mints=%d; want the GitHub App token %s from one mint",
					cred.Resolution.Kind, cred.Path, *mints, app)
			}
		})
	}
}

func TestGitHubRoleToken_RefusesOtherForge_NoMint(t *testing.T) {
	// Roster-mapped GitLab repo, via the roster-only form.
	rosterWithForge(t, "example-org/gitlab-pilot", "gitlab")
	plantCustody(t, "worker", "gitlab-pat-stub")
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))
	tok, path, err := GitHubRoleToken("worker", "example-org/gitlab-pilot")
	if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
		t.Fatalf("GitHubRoleToken on a GitLab-mapped repo: tok=%q path=%q err=%v; want a refusal and nothing handed back", tok, path, err)
	}
	if strings.Contains(errString(err), "gitlab-pat-stub") {
		t.Fatalf("the refusal carries a token value: %v", err)
	}
	// Roster silent, but the origin host is unambiguously GitLab.
	tok, path, err = GitHubRoleTokenForRemote("worker", "example-org/other", "https://gitlab.com/example-org/other.git")
	if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
		t.Fatalf("GitHubRoleTokenForRemote on a gitlab.com origin: tok=%q path=%q err=%v; want a refusal", tok, path, err)
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter forked %d time(s) for a repo another forge serves", *mints)
	}
	// And a GitHub repo still gets its App token through the same entry point.
	tok, _, err = GitHubRoleToken("worker", "example-org/tracker")
	if err != nil || tok != "app-token-stub" || *mints != 1 {
		t.Fatalf("GitHubRoleToken on an unmapped repo: tok=%q err=%v mints=%d; want the App token from one mint", tok, err, *mints)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
