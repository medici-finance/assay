package main

// github.go — deskpr's forge wiring.
//
// SINCE THE WRITE-VERBS-C MIGRATION deskpr no longer shells `gh`. Every change read and write
// — the existing-PR-for-branch lookup, the create, the mergeable read, the body/title edit and
// the re-review comment — goes through the resolved deskkit.Forge under the session-role App's
// custody. This is the same shape deskevidence uses: the tool mints its own App installation
// token (mintWorkerToken → ghToken), installs it as the GitHub custody step deskkit.ForgeFor
// calls, and the resolver hands that already-minted token to the backend. There is no ambient
// fallback: the `--as-app=false` path is gone, and an empty token is a HARD REFUSAL, never a
// fall-through to whatever gh identity is active.

import (
	"errors"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forgeAPIBase is a TEST-ONLY override of the API base the resolved backend is pointed at. It
// is EMPTY in production ("the backend's own default"), so this tool binds no forge host literal
// of its own, and there is deliberately no flag or environment variable that sets it.
var forgeAPIBase string

// init installs deskpr's already-minted App token as the GitHub custody step deskkit.ForgeFor
// calls. mintWorkerToken must run (setting ghToken) before any forge call.
func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token deskpr has ALREADY minted (ghToken) to the backend, and
// REFUSES — never falls back — when no token has been minted. The base URL is read HERE, at call
// time, so a per-test override of forgeAPIBase still reaches the Forge the resolver produces.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — deskpr never falls back to an ambient " +
				"gh identity/keyring (the retired --as-app=false path)")
	}
	return ghToken, forgeAPIBase, nil
}

// forgeForFn resolves the forge that serves a repo under the session-role App's custody. It is a
// package var so a test can substitute a recording fake without a live forge; production binds it
// to forgeFor.
var forgeForFn = forgeFor

// forgeFor resolves the forge serving owner/name under the App role this session acts under
// (mintedRole, resolved from the loop identity when the token was minted). Which forge it is
// comes from the resolver — never from a flag, an environment variable, or a default.
func forgeFor(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name := splitOwnerRepo(repo)
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, err := deskkit.ForgeFor(fr, mintedRole)
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}
