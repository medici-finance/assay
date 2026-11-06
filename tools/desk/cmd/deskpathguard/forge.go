package main

// forge.go — deskpathguard's forge wiring: the desklabel precedent, verbatim in shape.
//
// deskpathguard mints the SESSION-ROLE App installation token (DESK_LOOP-selected) via
// read in process (deskkit.WriteVerbRoleCredential, #5457) and reaches the forge through the resolved deskkit.Forge under that App's
// custody. There is no ambient fallback and no worker default: an empty token is a hard
// refusal at the custody step, and an unresolvable role refuses before the mint is even
// attempted.

import (
	"errors"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ghToken holds the session-role App installation token minted before any forge call.
var ghToken string

// mintedRole is the App role this session acts under (DESK_LOOP-selected). cmdCheck sets it
// from roleFn before any forge call; the mint below reuses it rather than resolving twice.
var mintedRole string

// forgeAPIBase is a TEST-ONLY override of the resolved backend's API base. Empty in production.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token deskpathguard has ALREADY minted (ghToken) to the
// backend, and refuses — never falls back — when no token has been minted.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — deskpathguard never falls back to an " +
				"ambient gh identity/keyring")
	}
	return ghToken, forgeAPIBase, nil
}

// mintTokenFn mints the session-role token for a repo and sets ghToken. Package var so a
// test can inject a token without a real credential read; production binds it to mintSessionToken.
var mintTokenFn = mintSessionToken

// mintSessionToken reads the credential of the role cmdCheck already resolved (mintedRole)
// READ-ONLY through deskkit.WriteVerbRoleCredential (never the desktoken binary, so never its
// rotation path — #5457) and sets ghToken. The repo is not optional: an App installed on more
// than one account resolves the wrong installation when the owner is omitted.
func mintSessionToken(repoSlug string) error {
	if mintedRole == "" {
		role, rerr := roleFn()
		if rerr != nil {
			return rerr
		}
		mintedRole = role
	}
	cred, err := deskkit.WriteVerbRoleCredential("deskpathguard", mintedRole, repoSlug)
	if err != nil {
		return err
	}
	ghToken = cred.Token
	return nil
}

// forgeForFn resolves the forge that serves a repo under the session-role App's custody.
// Package var so a test can substitute a recording fake; production binds it to forgeFor.
var forgeForFn = forgeFor

// forgeFor mints the session token (once) and resolves the forge serving owner/name.
func forgeFor(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, _ := strings.Cut(repo, "/")
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	if ghToken == "" {
		if merr := mintTokenFn(repo); merr != nil {
			return nil, fr, merr
		}
	}
	fg, err := deskkit.ForgeFor(fr, mintedRole)
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}
