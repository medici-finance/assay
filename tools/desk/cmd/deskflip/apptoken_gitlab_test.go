package main

// apptoken_gitlab_test.go — the app-token condition resolves the repo's forge BEFORE it
// mints a GitHub App installation token, so a GitLab adopter's ready-flip authenticates from
// the GitLab PAT custody path instead of dying with `no App ID for role "reviewer": set
// REVIEWER_APP_ID` — a GitHub credential error on a repo that uses a PAT
// (medici-finance/assay#772).
//
// deskflip's reads and writes are already forge-neutral (every one goes through the resolved
// deskkit.Forge); checkAppToken's unconditional GitHub-App mint was the last GitHub-shaped
// step, and this pins that it is gone for a GitLab-resolved repo.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantForgeRoster writes a roster binding gl-group/gl-repo to GitLab (and medici-finance/assay
// to GitHub, the control) under a private HOME, then reloads the cached config.
func plantForgeRoster(t *testing.T) {
	t.Helper()
	const roster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=desk=assay-desk-app:300000001,intake-loop=assay-intake-loop-app:300000002,issue-loop=assay-issue-loop-app:300000003,reviewer=assay-reviewer-app:300000004,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006
ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:private,gl-group/gl-repo:ci:private
ASSAY_REPO_FORGES=medici-finance/assay=github,gl-group/gl-repo=gitlab
`
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir config home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// On a GitLab-resolved repo, checkAppToken must NOT mint a GitHub App token: it goes straight
// to the resolver, which reads the GitLab PAT custody file. The spy proves the GitHub mint was
// not called, and the resulting refusal (no PAT provisioned in the test) names the GitLab
// custody file — never `REVIEWER_APP_ID`.
func TestAppTokenGitLabRepoSkipsGitHubMint(t *testing.T) {
	plantForgeRoster(t)
	t.Setenv("DESK_LOOP", flipRole)

	minted := false
	oldMint := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		minted = true
		// Mimic the reported failure: the GitHub App mint dies for lack of REVIEWER_APP_ID.
		return "", "", errors.New(`no App ID for role "reviewer": set REVIEWER_APP_ID`)
	}
	t.Cleanup(func() { mintTokenFn = oldMint })

	fr := deskkit.ForgeRepo{Owner: "gl-group", Name: "gl-repo"}
	_, _, err := checkAppToken(flipOpts{pr: 7, repo: fr.Slug()}, fr)

	if minted {
		t.Fatal("checkAppToken minted a GitHub App token for a GitLab-resolved repo — the pre-772 bug " +
			"(medici-finance/assay#772): the GitLab lane must never touch the GitHub App mint")
	}
	if err == nil {
		t.Fatal("expected a GitLab custody refusal (no PAT provisioned in the test), got nil")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "gitlab") {
		t.Errorf("refusal does not name the GitLab custody path: %s", msg)
	}
	if strings.Contains(msg, "REVIEWER_APP_ID") {
		t.Errorf("refusal is the misleading GitHub App-ID error (medici-finance/assay#772): %s", msg)
	}
}

// The GitHub control is unchanged: a GitHub-resolved repo still mints through mintTokenFn, so
// the role-and-path refusal message the identity tests rely on is preserved.
func TestAppTokenGitHubRepoStillMints(t *testing.T) {
	plantForgeRoster(t)
	t.Setenv("DESK_LOOP", flipRole)

	minted := false
	oldMint := mintTokenFn
	mintTokenFn = func(role, repo string) (string, string, error) {
		minted = true
		return "", "/config/home/reviewer-token-1", errors.New("private key not found")
	}
	t.Cleanup(func() { mintTokenFn = oldMint })

	fr := deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}
	_, _, err := checkAppToken(flipOpts{pr: 7, repo: fr.Slug()}, fr)

	if !minted {
		t.Fatal("checkAppToken did not mint through mintTokenFn on a GitHub-resolved repo — the GitHub " +
			"path must be byte-identical to before #772")
	}
	if err == nil {
		t.Fatal("expected the GitHub App-mint refusal, got nil")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitRefused {
		t.Errorf("GitHub mint failure exit code = %d, want %d (refused)", got, deskkit.ExitRefused)
	}
}
