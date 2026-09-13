package main

// forge.go — deskdisposition's forge wiring for the `read` verb.
//
// THE DEFECT (#984). `read` used to shell out to `gh pr view -R <repo> <N> --json
// labels,comments` under whatever identity `gh` resolved AMBIENTLY. That is fine for a
// human at a terminal with their own `gh auth login`, and it was this tool's original
// design (see main.go's doc comment) — but `deskclose superseded`'s confirm path calls
// `deskdisposition read` as a CHILD of an already-minted desk session, and that session's
// environment is deliberately isolated from a usable ambient `gh` identity (the same
// isolation the write-verbs-C migration gave deskclose's OWN forge calls: no ambient
// fallback, ever). The child inherited that isolated environment, `gh pr view` came back
// `HTTP 401: Requires authentication`, and `read` had no OTHER way to answer — it never
// minted a credential of its own, so there was nothing to fall back to.
//
// THE FIX mirrors deskclose/deskfile's own forge wiring exactly: `read` mints the
// SESSION-ROLE App installation token (DESK_LOOP-selected, worker by default — read has
// no lane-role concept the way deskclose's superseded lane does, so it defaults the way
// deskfile's every-verb does) via `desktoken` and reaches the forge through the resolved
// deskkit.Forge under that App's custody, via REST (go-gh's client), never `gh`. There is
// no ambient fallback here either: an empty token is a hard refusal at the custody step,
// same as every other migrated verb. `set` and `sweep` are UNCHANGED — they still shell to
// `gh` under the ambient identity, per the tool's original design; only `read` is the
// authenticated-by-another-tool's-child path this defect was found on.

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

// mintedRole is the App role mintSessionToken resolved for this session (worker by default,
// mirroring deskfile — `read` has no two-role concept of its own to key on).
var mintedRole = "worker"

// forgeAPIBase is a TEST-ONLY override of the API base the resolved backend is pointed at. Empty
// in production; there is no flag or environment variable that sets it.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint hands the token this process has ALREADY minted (ghToken) to the backend,
// and refuses — never falls back — when no token has been minted.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted App token — deskdisposition never falls back to " +
				"an ambient gh identity/keyring for an authenticated read")
	}
	return ghToken, forgeAPIBase, nil
}

// mintTokenFn mints the session-role token for a repo and sets ghToken. Package var so a test can
// inject a token without shelling desktoken; production binds it to mintSessionToken.
var mintTokenFn = mintSessionToken

// mintSessionToken resolves the App role this session acts under (DESK_LOOP via
// deskkit.SessionTokenRole, worker by default), calls `desktoken <role> --repo <slug>`, and sets
// the returned token as ghToken. --repo is not optional: an App installed on more than one
// account mints for the wrong installation when it is omitted.
func mintSessionToken(repoSlug string) error {
	role := "worker"
	if r, _, rerr := deskkit.SessionTokenRole(toolName); rerr == nil {
		role = r
	} else {
		fmt.Fprintf(os.Stderr, "deskdisposition: no App role resolved for this session — defaulting to the worker App token (%v)\n", rerr)
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
