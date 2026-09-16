package main

// forge.go — desklabel's forge wiring: the deskclose precedent, verbatim in shape.
//
// desklabel mints the SESSION-ROLE App installation token (DESK_LOOP-selected) via desktoken
// and reaches the forge through the resolved deskkit.Forge under that App's custody. The role
// is the same value the vocabulary check keyed on (mintedRole, set from roleFn before any
// forge call), so the identity that WRITES the label is the identity that was ENTITLED to —
// there is no path on which the two differ.
//
// There is no ambient fallback and no worker default: an empty token is a HARD REFUSAL at the
// custody step, and an unresolvable role refuses before the mint is even attempted.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ghToken holds the session-role App installation token minted before any forge call. The
// GitHub custody minter hands it to the resolved backend; an empty value is a hard refusal.
var ghToken string

// mintedRole is the App role this session acts under (DESK_LOOP-selected). runLabel sets it
// from roleFn BEFORE the ownership check; the mint below reuses it rather than resolving a
// second time, so the checked role and the minted role cannot drift.
var mintedRole string

// forgeAPIBase is a TEST-ONLY override of the resolved backend's API base. Empty in production.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token desklabel has ALREADY minted (ghToken) to the backend, and
// refuses — never falls back — when no token has been minted.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — desklabel never falls back to an ambient " +
				"gh identity/keyring")
	}
	return ghToken, forgeAPIBase, nil
}

// mintTokenFn mints the session-role token for a repo and sets ghToken. Package var so a test
// can inject a token without shelling desktoken; production binds it to mintSessionToken.
var mintTokenFn = mintSessionToken

// mintSessionToken calls `desktoken <role> --repo <slug>` for the role runLabel already
// resolved (mintedRole; resolved here only if a caller reached the forge first) and sets
// ghToken. --repo is not optional: an App installed on more than one account mints for the
// wrong installation when it is omitted.
func mintSessionToken(repoSlug string) error {
	if mintedRole == "" {
		role, rerr := roleFn()
		if rerr != nil {
			return rerr
		}
		mintedRole = role
	}
	role := mintedRole
	cmd := execCommand("desktoken", role, "--repo", repoSlug)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("desktoken %s --repo %s: %v (%s)",
			role, repoSlug, err, strings.TrimSpace(errb.String())), err)
	}
	tokenPath := strings.TrimSpace(out.String())
	b, rerr := os.ReadFile(tokenPath)
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("read %s token from %s", role, tokenPath), rerr)
	}
	ghToken = strings.TrimSpace(string(b))
	if ghToken == "" {
		return deskkit.Unverifiable(fmt.Sprintf("%s token at %s is empty", role, tokenPath), nil)
	}
	return nil
}

// forgeForFn resolves the forge that serves a repo under the session-role App's custody.
// Package var so a test can substitute a recording fake; production binds it to forgeFor.
var forgeForFn = forgeFor

// forgeFor mints the session token (once) and resolves the forge serving owner/name. The
// forge comes from the resolver — the roster binding, else an unambiguous origin host, else
// a could-not-check refusal. There is no parameter, flag or environment variable by which a
// caller supplies the forge.
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
