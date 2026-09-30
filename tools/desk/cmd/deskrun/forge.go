package main

// forge.go — deskrun's custody and resolver seam.
//
// deskrun acts under ONE identity, the release-runner role, and only after
// deskkit.ResolveRunCredential has said the repo is bound to it. It never acts under the
// session's own desk role (a worker/reviewer/verifier App must not carry actions: write), and
// there is no flag that selects a role. The backend comes from deskkit.ResolveForge — the one
// construction site — so the forge serving the repo is the roster's / origin host's answer,
// never deskrun's choice.
//
// GitHub custody: the installed minter below obtains the release-runner App token through
// deskkit.GitHubRoleToken (the `desktoken release-runner --repo <slug>` mint-or-reuse path, which refuses
// a repo the roster binds to another forge before any mint).
// GitLab custody: ForgeFor reads the already-provisioned `gitlab-release-runner.token` file —
// the pipeline trigger token — and never rotates it. Neither path falls back to an ambient
// gh/glab credential: an absent or empty token is a refusal at the custody step, and the
// backends refuse an unminted token again at the transport floor.

import (
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the GitHub mint seam, swapped in tests for a stub. Production binds it to
	// the shared desktoken mint-or-reuse resolver.
	mintTokenFn = deskkit.GitHubRoleToken
	// forgeAPIBase is a TEST-ONLY override of the resolved GitHub backend's API base. Empty in
	// production means the real host.
	forgeAPIBase string
	// resolveCredFn resolves the repo's run-credential binding; swapped only by the mutation
	// harness's reach (the control itself lives in deskkit.ResolveRunCredential).
	resolveCredFn = deskkit.ResolveRunCredential
	// forgeForFn resolves the backend serving the repo under the release-runner role's custody.
	// Package var so a test can substitute a recording fake.
	forgeForFn = func(fr deskkit.ForgeRepo) (deskkit.Forge, deskkit.ForgeResolution, error) {
		return deskkit.ResolveForge(fr, deskkit.ReleaseRunnerRole)
	}
)

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint mints the release-runner App token — and ONLY that role's. A request for
// any other role is refused: deskrun has no business minting a desk role's credential.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if role != deskkit.ReleaseRunnerRole {
		return "", "", fmt.Errorf("deskrun mints only the %s credential, never %q", deskkit.ReleaseRunnerRole, role)
	}
	tok, _, merr := mintTokenFn(role, repo.Slug())
	if merr != nil {
		return "", "", merr
	}
	return tok, forgeAPIBase, nil
}
