package main

// github.go — deskfile's forge wiring.
//
// SINCE THE WRITE-VERBS-C MIGRATION deskfile no longer shells `gh`, and no longer files under
// whatever ambient CLI credential happens to be active. It mints the SESSION-ROLE App
// installation token (DESK_LOOP-selected, worker by default) via desktoken and reaches the
// forge through the resolved deskkit.Forge under that App's custody — the token-custody decision
// the #781 ruling confirmed. `--raised-by` stays a body/label ATTRIBUTION, exactly as before: it
// stamps a `raised-by:<role>` label, it does NOT switch the acting credential.
//
// There is no ambient fallback: an empty token is a HARD REFUSAL at the custody step, and an
// unresolvable forge is a could-not-check refusal from the resolver itself — which is what
// RETAINS deskfile's historical refusal on a repo whose forge cannot be determined, while
// SUPERSEDING the interim GitLab named-refusal (#691) now the backend serves GitLab.

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

// mintedRole is the App role mintSessionToken resolved for this session (worker by default). It
// is passed to deskkit.ForgeFor so the resolver's custody binding names the acting identity.
var mintedRole = "worker"

// forgeAPIBase is a TEST-ONLY override of the API base the resolved backend is pointed at. Empty
// in production; there is no flag or environment variable that sets it.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token deskfile has ALREADY minted (ghToken) to the backend, and
// refuses — never falls back — when no token has been minted.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — deskfile never falls back to an ambient " +
				"gh identity/keyring")
	}
	return ghToken, forgeAPIBase, nil
}

// mintTokenFn mints the session-role token for a repo and sets ghToken. Package var so a test can
// inject a token without shelling desktoken; production binds it to mintSessionToken.
var mintTokenFn = mintSessionToken

// mintSessionToken resolves the App role this session acts under (DESK_LOOP via
// deskkit.SessionTokenRole, worker by default), calls `desktoken <role> --repo <slug>`, and sets
// the returned token as ghToken. --repo is not optional: an App installed on more than one
// account mints for the wrong installation when it is omitted (mirrors deskpr/deskevidence/#565).
func mintSessionToken(repoSlug string) error {
	role := "worker"
	if r, _, rerr := deskkit.SessionTokenRole("deskfile"); rerr == nil {
		role = r
	} else {
		fmt.Fprintf(os.Stderr, "deskfile: no App role resolved for this session — defaulting to the worker App token (%v)\n", rerr)
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

// forgeFor mints the session token (if not already minted) and resolves the forge serving
// owner/name. Which forge it is comes from the resolver — the roster binding, else an
// unambiguous origin host, else a could-not-check REFUSAL (which retains deskfile's historical
// refusal on an unresolvable forge). A repo affirmatively resolved to GitLab is now SERVED, not
// refused (#691 superseded).
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
