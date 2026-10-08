package main

// forge.go — deskrun's custody and resolver seam.
//
// deskrun's WRITES (dispatch, approve, retry) and its status read act under ONE identity, the
// release-runner role, and only after deskkit.ResolveRunCredential has said the repo is bound to
// it. They never act under the session's own desk role (a worker/reviewer/verifier App must not
// carry actions: write). The one exception is the log read: it acts under the CALLING session's
// own role (worker-desk or pr-review-desk, per the loop identity), never the release-runner's,
// and refuses every other loop. No flag selects a role on any verb. The backend comes from deskkit.ResolveForge — the one
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
	// logForgeFn resolves the backend serving `deskrun log` under the CALLING role's own custody
	// (a read: worker or reviewer, never the release-runner). Package var so a test can
	// substitute a recording fake.
	logForgeFn = func(fr deskkit.ForgeRepo, role string) (deskkit.Forge, deskkit.ForgeResolution, error) {
		return deskkit.ResolveForge(fr, role)
	}
)

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint mints the release-runner App token for every run verb, and for `log` ALONE
// the calling worker or reviewer role's own token (logRoles). A request for any other role is
// refused: deskrun has no business minting any other desk role's credential, and only
// cmdLog ever asks for a logRoles role (the write verbs are hard-wired to the release-runner
// by forgeForFn).
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if role != deskkit.ReleaseRunnerRole && !logRoles[role] {
		return "", "", fmt.Errorf("deskrun mints only the %s credential (and, for log, the worker or reviewer role's own), never %q", deskkit.ReleaseRunnerRole, role)
	}
	tok, _, merr := mintTokenFn(role, repo.Slug())
	if merr != nil {
		return "", "", merr
	}
	return tok, forgeAPIBase, nil
}
