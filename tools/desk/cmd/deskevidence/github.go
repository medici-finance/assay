package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// github.go — deskevidence's identity + forge wiring.
//
// SINCE THE FORGE MIGRATION deskevidence no longer builds net/http against a hardcoded
// api.github.com, no longer signs an App JWT, and no longer performs the
// installation-token exchange itself. All of that is the identity layer: the verifier App
// installation token is minted by `desktoken verifier --repo <repo>` (mintVerifierToken
// below), and every forge read and write goes through the resolved deskkit.Forge, under the
// verifier App's custody. This is the same shape deskreply uses for its worker token — the
// tool holds the already-minted token and hands it to the resolver, which never mints twice
// and never falls back to an ambient CLI identity.

// ghToken holds the verifier App installation token minted for the target repo. deskevidence
// ALWAYS mints it before any forge call, so every read and every write is performed as the
// verifier App.
//
// An EMPTY value is a HARD REFUSAL, never a fallback (custody minter below). With no minted
// token the write would land as whatever identity the ambient credential holds — the exact
// ambient-identity lane the custody rules retire, and unlike a failed read it cannot be taken
// back once an Evidence row is committed under the wrong actor.
var ghToken string

// forgeAPIBase is a TEST-ONLY override of the API base the resolved backend is pointed at. It
// is EMPTY in production, meaning "the backend's own default", so this tool binds no forge host
// literal of its own — and there is deliberately no flag or environment variable that sets it.
var forgeAPIBase string

// init installs deskevidence's already-minted verifier token as the GitHub custody step
// deskkit.ForgeFor calls.
func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint is the GitHub custody step deskkit.ForgeFor's resolver calls: it hands
// the token this tool has ALREADY minted (ghToken) to the backend, and refuses — never falls
// back — when no token has been minted. The base URL is read HERE, at call time, so a per-test
// override of forgeAPIBase still reaches the Forge the resolver produces. Named (rather than an
// init-local closure) so the empty-token refusal is directly exercised by a test.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		return "", "", errors.New(
			"refusing to reach the forge with no minted verifier token — deskevidence never falls back " +
				"to an ambient forge identity/keyring")
	}
	return ghToken, forgeAPIBase, nil
}

// forgeForFn resolves the forge that serves a repo under the verifier App's custody. It is a
// package var so a test can substitute a recording fake without a live forge; production binds
// it to forgeFor.
var forgeForFn = forgeFor

// forgeFor resolves the forge serving owner/name. Which forge it is comes from the resolver
// (the roster's binding, else an unambiguous origin host, else a refusal) — never from a flag,
// an environment variable, or a default.
func forgeFor(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, err := deskkit.ForgeFor(fr, "verifier")
	if err != nil {
		return nil, fr, err
	}
	return fg, fr, nil
}

// mintTokenFn mints the verifier token for a repo and sets ghToken. It is a package var so a
// test can inject a token without shelling desktoken; production binds it to mintVerifierToken.
var mintTokenFn = mintVerifierToken

// execCommand is the single seam through which deskevidence starts a child process — only
// `desktoken` (the identity layer) reaches it. Tests replace it.
var execCommand = exec.Command

// mintVerifierToken calls `desktoken verifier --repo <owner/repo>` to mint (or reuse) the
// verifier App installation token scoped to the repo's owner, then sets it as ghToken so every
// subsequent forge call authenticates as the verifier App.
//
// --repo is not optional (mirrors deskreply/#562): an App installed on more than one account
// mints for the wrong installation when --repo is omitted — a token with no access to the
// target repo. The repo deskevidence passes is the one it just validated against its repo set.
func mintVerifierToken(repoSlug string) error {
	cmd := execCommand("desktoken", "verifier", "--repo", repoSlug)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("desktoken verifier --repo %s: %v (%s)",
			repoSlug, err, strings.TrimSpace(errb.String())), err)
	}
	tokenPath := strings.TrimSpace(out.String())
	b, rerr := os.ReadFile(tokenPath)
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("read verifier token from %s", tokenPath), rerr)
	}
	ghToken = strings.TrimSpace(string(b))
	if ghToken == "" {
		return deskkit.Unverifiable(fmt.Sprintf("verifier token at %s is empty", tokenPath), nil)
	}
	return nil
}

// verifierBotDisplay is for MESSAGE TEXT only — never a comparison.
func verifierBotDisplay() string { return deskkit.RoleAppLoginOrEmpty("verifier") }

// isVerifierBot is a MATCHER for the Evidence commit's git author against the verifier App's
// bound bot login. An unbound role yields ok=false and an empty author matches nothing — the
// two states checkAttribution keeps distinct (an unbound role is a REFUSAL, an empty author is
// could-not-check).
func isVerifierBot(author string) bool {
	want, ok := deskkit.RoleAppLogin("verifier")
	if !ok || author == "" {
		return false
	}
	return author == want
}

// checkAttribution reads the git author the forge recorded on the Evidence write and decides
// whether it carries the unforgeable verifier identity. Four states, ordered deliberately:
//
//   - the verifier role is UNBOUND → REFUSED (exit 6). Ordered first, because it is the only
//     state in which this function does not know what the right answer looks like.
//   - isVerifierBot(author)          → proven. nil.
//   - author == ""                   → COULD NOT CHECK (warned, exit stays 0). GitLab's file
//     write reports no author at all, so this is the ordinary answer on a GitLab landing, and
//     a response-shape surprise on GitHub — either way not a verdict this tree may invent.
//   - author is some OTHER login     → PROVEN WRONG (exit 6), naming what landed. The write is
//     already on the remote (a Contents-API write is not a local commit), so this is a report
//     rather than a prevention — which is why it must be loud.
func checkAttribution(author string) (detail string, err error) {
	want, bound := deskkit.RoleAppLogin("verifier")
	switch {
	case !bound:
		return "attribution=REFUSED (verifier role unbound)", deskkit.Unverifiable(
			"cannot check the Evidence commit's attribution: "+deskkit.RequireRole("verifier").Error(), nil)
	case isVerifierBot(author):
		return "author=" + want, nil
	case author == "":
		fmt.Fprintf(stderr, "deskevidence: WARNING: the forge reported no author on the Evidence write — "+
			"could NOT verify it is attributed to %s\n", want)
		return "attribution=could-not-check", nil
	default:
		return "attribution=WRONG (" + author + ")", deskkit.Unverifiable(
			"the Evidence write landed attributed to "+author+", not "+want+
				" — the Evidence row does NOT carry the verifier identity; check the verifier App custody binding", nil)
	}
}
