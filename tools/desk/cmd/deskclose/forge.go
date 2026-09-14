package main

// forge.go — deskclose's forge wiring.
//
// SINCE THE WRITE-VERBS-C MIGRATION deskclose no longer shells `gh` under an ambient credential.
// It mints the SESSION-ROLE App installation token (DESK_LOOP-selected) via desktoken and reaches
// the forge through the resolved deskkit.Forge under that App's custody — the token-custody
// decision the #781 ruling confirmed. The blessing-authority model is UNCHANGED: authority is read
// from the ITEM's authorizing comment and checked against the roster-pinned human, never from the
// acting identity. What changes is only WHO performs the close (ambient → App), which is the
// point. The `viewer{login}` whoami the supersession lane used to read is replaced by the minted
// role's known login (mintedRole / RoleAppLogin), an identity-layer answer, not a forge op.
//
// There is no ambient fallback: an empty token is a HARD REFUSAL at the custody step.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ghToken holds the session-role App installation token minted before any forge call. The GitHub
// custody minter hands it to the resolved backend; an empty value is a hard refusal.
var ghToken string

// mintedRole is the App role this session acts under (DESK_LOOP-selected). It is the role the
// supersession lane keys the caller on, and the login of every close is RoleAppLogin(mintedRole).
var mintedRole string

// forgeAPIBase is a TEST-ONLY override of the resolved backend's API base. Empty in production.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token deskclose has ALREADY minted (ghToken) to the backend, and
// refuses — never falls back — when no token has been minted.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — deskclose never falls back to an ambient " +
				"gh identity/keyring")
	}
	return ghToken, forgeAPIBase, nil
}

// mintTokenFn mints the session-role token for a repo and sets ghToken. Package var so a test can
// inject a token without shelling desktoken; production binds it to mintSessionToken.
var mintTokenFn = mintSessionToken

// mintSessionToken resolves the App role this session acts under (DESK_LOOP via
// deskkit.SessionTokenRole; deskclose has NO worker default — an unresolvable role is a refusal,
// because deskclose closes other people's items and must never do so under a guessed identity),
// calls `desktoken <role> --repo <slug>`, and sets ghToken. It also records the role in mintedRole.
func mintSessionToken(repoSlug string) error {
	role, _, rerr := deskkit.SessionTokenRole("deskclose")
	if rerr != nil {
		return deskkit.Refused(
			"refused: deskclose could not resolve which App role this session acts under (" + rerr.Error() +
				") — it closes other people's items and will not do so under a guessed identity. Run inside a " +
				"booted desk window (DESK_LOOP set) whose role the roster binds to an App.")
	}
	mintedRole = role
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

// forgeForFn resolves the forge that serves a repo under the session-role App's custody. Package
// var so a test can substitute a recording fake; production binds it to forgeFor.
var forgeForFn = forgeFor

// forgeFor mints the session token (once) and resolves the forge serving owner/name. The forge
// comes from the resolver — the roster binding, else an unambiguous origin host, else a
// could-not-check refusal.
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
