package main

// claimauthgitlab_test.go — issue 1203. The claim-acquire step (resolveClaimAuth) minted a
// GitHub App installation token UNCONDITIONALLY when no GH_TOKEN was exported. On a repo whose
// forge resolves to GitLab (ASSAY_REPO_FORGES=<slug>=gitlab) there is no App to mint against,
// so claim-acquire failed closed with "no App ID for App \"reviewer-app\"" before any claim was
// taken — even though deskboot had already cached the GitLab role PAT and every other write
// verb (deskpost/deskflip via deskkit.ResolveForge) uses it.
//
// THE FIX. resolveClaimAuth now follows the resolved forge: a GitLab-served repo hands the
// claim child the same role PAT custody the other verbs use (deskkit.GitLabRoleToken →
// gitlab-<role>.token on the App-credential search path), NEVER the GitHub App minter; GitHub
// keeps the App mint unchanged.
//
// THE HARNESS reuses stampgitlab_test.go's fixture (a roster binding the project to GitLab and
// the role PAT custody files the resolver reads) and claimauth_test.go's REAL fake claim child
// (a shell script that records the credential it could see). The assertion surface is the mint
// SEAM (mintTokenFn) — the one recorded here is the claim step's and nobody else's, because the
// model stamp reads its credential inside deskkit.ResolveForge, not this seam. Nothing here
// reaches a live GitLab: the roster is a fixture, the claim child never dials out.
//
// The tests are named TestGitLab* so the forge-gitlab mutation map's -run selector picks them up.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glClaimRun drives a dispatch onto the GitLab project through the REAL fake claim child and
// returns the record path the child appended to and the recorder of mint-seam calls. The origin
// remote is GitLab-shaped; the roster entry is what actually resolves the forge.
func glClaimRun(t *testing.T, useGoBinary bool, extra ...string) (record string, mints *[]mintCall, rc int) {
	t.Helper()
	s := &stub{}
	home, root := s.install(t)
	installGLStamp(t, home)
	if useGoBinary {
		plantGoClaimChild(t)
	} else {
		plantScriptClaimChild(t, root)
	}
	record = installClaimChild(t, s)
	// The mint seam is bound to a RECORDING stub: on the unfixed code the GitLab claim runs
	// through it (that is the defect); on the fixed code it must never be reached, so a mint
	// recorded here fails the test.
	mints = stubMint(t, stubMintedToken, nil)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@gitlab.com:" + glProject + ".git"},
		{match: "deskwt add", stdout: "/private/tmp/worker-home"},
	}
	args := append([]string{"item-1", "--root", root, "--repo", glProject,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}, extra...)
	rc = run(args)
	return record, mints, rc
}

// A REVIEW dispatch on a GitLab-served project, Go binary: the claim child is handed the
// reviewer's GitLab PAT via --token-file, and the GitHub App minter is NEVER called. On the
// unfixed code the minter IS called (and the child would get the App token, not the PAT), so
// this fails first on the mint-count assertion.
func TestGitLabReviewDispatchClaimUsesGitLabPATNotTheAppMinter(t *testing.T) {
	t.Setenv("DESK_LOOP", "pr-review-desk")
	record, mints, rc := glClaimRun(t, true,
		"--kit", "review", "--pr", glMRIID, "--model", "example-model-1", "--tier", "strong")
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab review dispatch rc = %d, want 0 — the claim must authenticate under the GitLab "+
			"role PAT custody, not require a GitHub App ID", rc)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitLab-served project: %v — on a "+
			"GitLab-only roster there is no App to mint against, which is exactly the exit-6 field defect "+
			"(issue 1203)", len(*mints), *mints)
	}
	r := acquireRecord(t, record)
	if strings.TrimSpace(r.tokenFileContent) != glPATPlaceholder {
		t.Fatalf("the claim child read token-file content %q, want the reviewer GitLab PAT %q (token-file %q, "+
			"argv %s)", r.tokenFileContent, glPATPlaceholder, r.tokenFile, r.argv)
	}
	if strings.TrimSpace(r.tokenFileContent) == stubMintedToken {
		t.Fatalf("the claim child got the GitHub App token, not the GitLab PAT — the App minter served the " +
			"GitLab claim")
	}
	if r.ghToken != "" {
		t.Errorf("GH_TOKEN=%q was injected into the Go binary's environment; the binary takes --token-file", r.ghToken)
	}
}

// A WORKER (default kit) dispatch on a GitLab-served project, Go binary: same property one lane
// over. The worker path leaves plan.forgeKind unset, so resolveClaimAuth resolves the forge
// itself at claim time — this proves the worker claim child gets the desk role's GitLab PAT and
// the App minter is not called.
func TestGitLabWorkerDispatchClaimUsesGitLabPATNotTheAppMinter(t *testing.T) {
	record, mints, rc := glClaimRun(t, true)
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab worker dispatch rc = %d, want 0", rc)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitLab-served worker dispatch: %v", len(*mints), *mints)
	}
	r := acquireRecord(t, record)
	if strings.TrimSpace(r.tokenFileContent) != glPATPlaceholder {
		t.Fatalf("the worker claim child read token-file content %q, want the desk role's GitLab PAT %q "+
			"(argv %s)", r.tokenFileContent, glPATPlaceholder, r.argv)
	}
}

// The LEGACY SCRIPT (Go binary absent) on a GitLab project shells its own transport and reads
// the token from the environment: the GitLab PAT is placed in GH_TOKEN (and GITLAB_TOKEN), not
// handed as --token-file, and again the App minter is never called.
func TestGitLabClaimLegacyScriptGetsTheGitLabPATInTheEnvironment(t *testing.T) {
	t.Setenv("DESK_LOOP", "pr-review-desk")
	record, mints, rc := glClaimRun(t, false,
		"--kit", "review", "--pr", glMRIID, "--model", "example-model-1", "--tier", "strong")
	if rc != deskkit.ExitOK {
		t.Fatalf("GitLab review dispatch (legacy script) rc = %d, want 0", rc)
	}
	if len(*mints) != 0 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitLab-served project (legacy script): %v", len(*mints), *mints)
	}
	r := acquireRecord(t, record)
	if r.ghToken != glPATPlaceholder {
		t.Fatalf("the legacy script saw GH_TOKEN=%q, want the reviewer GitLab PAT %q (argv %s)", r.ghToken, glPATPlaceholder, r.argv)
	}
	if r.tokenFile != "" || strings.Contains(r.argv, "--token-file") {
		t.Errorf("--token-file was passed to the legacy script, which has no such flag: %s", r.argv)
	}
}

// KEEP THE GITHUB PATH GREEN: a dispatch onto a GitHub-served repo still mints the App
// installation token for the claim, under the desk dispatcher role. The App path — the mint
// seam — is unchanged; this is the half the fix must NOT disturb. (The token VALUE reaching the
// child as --token-file is covered by TestClaimChildReceivesTheMintedTokenAsTokenFileForTheGoBinary.)
func TestGitHubDispatchClaimStillMintsTheAppToken(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantGoClaimChild(t)
	record := installClaimChild(t, s)
	mints := stubMint(t, stubMintedToken, nil)
	s.replies = happyReplies("/private/tmp/worker-home")

	rc := run([]string{"item-1", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("GitHub dispatch rc = %d, want 0", rc)
	}
	if len(*mints) != 1 {
		t.Fatalf("the GitHub App minter was called %d time(s) on a GitHub-served repo, want exactly 1 — the "+
			"App path must stay the credential custody for GitHub", len(*mints))
	}
	if (*mints)[0].role != deskkit.DispatcherRole {
		t.Errorf("the claim minted under role %q, want the desk dispatcher role %q", (*mints)[0].role, deskkit.DispatcherRole)
	}
	// The claim child ran under the minted App token via --token-file (GitHub takes no GH_TOKEN
	// env on the binary), never a GitLab PAT path.
	r := acquireRecord(t, record)
	if r.ghToken != "" {
		t.Errorf("GH_TOKEN=%q was injected into the Go binary on the GitHub path", r.ghToken)
	}
	if strings.Contains(r.tokenFile, "gitlab-") {
		t.Errorf("the GitHub claim child was handed a GitLab custody file: %q", r.tokenFile)
	}
}
