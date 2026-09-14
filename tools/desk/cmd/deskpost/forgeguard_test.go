package main

// forgeguard_test.go — deskpost resolves the repo's forge BEFORE minting a GitHub App
// installation token, so a GitLab adopter gets an honest could-not-check naming its forge
// rather than the misleading `set REVIEWER_APP_ID` mint error (medici-finance/assay#772).
//
// deskpost's verdict/comment/flip write path reads every precondition through the
// App-authenticated ghClient, so it cannot serve GitLab yet; the correct behaviour is to
// FAIL CLOSED with a message that does not send the operator hunting apps.env for a GitHub
// App credential a PAT-backed GitLab bot never uses.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantForgeRoster writes a roster binding gl-group/gl-repo to GitLab (and keeping the
// GitHub control repo bound to GitHub) under a private HOME, then reloads the cached config.
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

// On a GitLab-resolved repo, newGHClient refuses BEFORE minting: the error names the GitLab
// forge and must NOT be the GitHub App-ID mint error. This is the whole defect — the pre-772
// path minted first and died with `set REVIEWER_APP_ID`.
func TestNewGHClientRefusesGitLabRepoWithHonestMessage(t *testing.T) {
	plantForgeRoster(t)

	_, err := newGHClient("gl-group", "gl-repo")
	if err == nil {
		t.Fatal("newGHClient returned no error on a GitLab-resolved repo — it must fail closed")
	}
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "gitlab") {
		t.Errorf("refusal does not name the gitlab forge: %s", msg)
	}
	// The misleading pre-772 error told the operator to `set REVIEWER_APP_ID` / edit apps.env
	// — a GitHub App credential a PAT-backed GitLab bot never uses. The honest refusal must
	// carry NEITHER of those remediation phrases (it may still say it is NOT that error).
	for _, bad := range []string{"set REVIEWER_APP_ID", "apps.env"} {
		if strings.Contains(msg, bad) {
			t.Errorf("refusal carries the misleading GitHub App-ID remediation %q (medici-finance/assay#772): %s", bad, msg)
		}
	}
	// Exit 6 — could-not-check, the honest three-state answer for "no GitLab write backend yet".
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Errorf("exit code = %d, want %d (unverifiable/could-not-check)", got, deskkit.ExitUnverifiable)
	}
}

// A GitHub-resolved repo is unaffected by the guard: requireGitHubForge returns nil, so the
// path is byte-identical to before #772 (the mint runs and fails only for lack of a real App
// credential in the test env — never with the forge refusal).
func TestRequireGitHubForgeAllowsGitHubRepo(t *testing.T) {
	plantForgeRoster(t)

	if err := requireGitHubForge("medici-finance", "assay"); err != nil {
		t.Fatalf("requireGitHubForge refused a GitHub-resolved repo: %v", err)
	}
}

// A repo whose forge cannot be POSITIVELY resolved (no roster entry, no known origin host) is
// could-not-check, NOT GitLab: the guard falls through so the GitHub mint keeps its exact
// pre-772 behaviour. requireGitHubForge must return nil in that case.
func TestRequireGitHubForgeFallsThroughOnUnresolvedRepo(t *testing.T) {
	plantForgeRoster(t)

	// Not in the roster; ForgeKindFor's remote-host fallback reads this package's own checkout
	// origin, which is medici-finance/assay on github.com (or nothing, off-repo). Either way the
	// result must never REFUSE this repo as non-GitHub — that is the fall-through the guard makes.
	if err := requireGitHubForge("unconfigured-org", "unconfigured-repo"); err != nil {
		// The only acceptable error here is a positive GitLab resolution, which cannot happen
		// for an unconfigured repo whose origin is not gitlab.com.
		if strings.Contains(strings.ToLower(err.Error()), "gitlab") {
			t.Fatalf("unexpectedly refused an unresolved repo AS GitLab: %v", err)
		}
	}
}
