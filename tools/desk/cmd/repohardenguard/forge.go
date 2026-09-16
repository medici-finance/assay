package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// forge.go — repohardenguard's identity + forge wiring (the forge-gitlab guard-read-custody brief's migration).
//
// This program resolves and mints as the FIXED role "auditor" — never `$DESK_LOOP`.
// repohardenguard runs OUTSIDE any desk window (an operator's laptop, a cron), so there is no
// loop identity to read, and reading one would silently substitute whatever role happened to be
// exported into the invoking shell for the read-only identity this program's whole design
// depends on. This is the same FIXED-role shape deskevidence installs for "verifier"
// (cmd/deskevidence/github.go) — a tool that mints its OWN token rather than reading a desk
// window's loop var — applied here to "auditor".
//
// Forge resolution is by the roster's ASSAY_REPO_FORGES entry ONLY, via deskkit.ForgeFor —
// never the CWD's origin remote: this guard is routinely run from a checkout of a DIFFERENT
// repo than the one --repo names, so "the origin remote" would resolve the WRONG repo's forge.

// ghToken holds the auditor App installation token minted for the target repo's owner. It is
// read lazily by githubCustodyMint on the first GitHub custody call, so a repo that resolves to
// GitLab never triggers a `desktoken auditor` invocation at all. An empty value after minting is
// a hard REFUSAL, never a fallback to any ambient identity.
var ghToken string

// forgeAPIBase is a TEST-ONLY override of the API base the resolved backend is pointed at,
// empty in production (the backend's own default) — no flag or env var sets it.
var forgeAPIBase string

func init() {
	deskkit.SetGitHubCustodyMinter(githubCustodyMint)
}

// githubCustodyMint is the GitHub custody step deskkit.ForgeFor's resolver calls. It mints the
// auditor token ON FIRST USE (mintTokenFn) rather than eagerly at startup, so a repo that
// resolves to GitLab never shells `desktoken auditor` at all — only a GitHub-resolved repo
// reaches this function.
func githubCustodyMint(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
	if ghToken == "" {
		if merr := mintTokenFn(repo.Slug()); merr != nil {
			return "", "", merr
		}
	}
	if ghToken == "" {
		return "", "", fmt.Errorf(
			"refusing to reach the forge with no minted auditor token for %s — repohardenguard never falls "+
				"back to an ambient forge identity/keyring", repo.Slug())
	}
	return ghToken, forgeAPIBase, nil
}

// forgeForFn resolves the forge serving repo, as the FIXED "auditor" role, and hands back the
// RESOLUTION alongside it (deskkit.ResolveForge's provenance record) — the guard's preflight
// document is read off that value (ForgeResolution.HardeningRepoDocumentKind: `repo` on
// GitHub, `project` on GitLab), never hard-coded to one forge's kind and never chosen from a
// ForgeKind this program supplies. Package var so a test substitutes a stub Forge instead of
// a network call.
var forgeForFn = forgeFor

func forgeFor(repo string) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeResolution, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.ForgeResolution{}, deskkit.Unverifiable("repohardenguard: bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	fg, res, err := deskkit.ResolveForge(fr, "auditor")
	if err != nil {
		return nil, fr, deskkit.ForgeResolution{}, err
	}
	return fg, fr, res, nil
}

// mintTokenFn mints the auditor token for a repo and sets ghToken. Package var so a test
// injects a token without shelling desktoken; production binds it to mintAuditorToken.
var mintTokenFn = mintAuditorToken

// execCommand is the single seam through which repohardenguard starts a child process — only
// `desktoken` (the identity layer) reaches it. Tests replace it.
var execCommand = exec.Command

// mintAuditorToken calls `desktoken auditor --repo <owner/repo>` to mint (or reuse) the auditor
// App installation token scoped to the repo's owner, then sets it as ghToken so every
// subsequent forge call authenticates as the auditor App.
func mintAuditorToken(repoSlug string) error {
	cmd := execCommand("desktoken", "auditor", "--repo", repoSlug)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return deskkit.Unverifiable(fmt.Sprintf("desktoken auditor --repo %s: %v (%s)",
			repoSlug, err, strings.TrimSpace(errb.String())), err)
	}
	tokenPath := strings.TrimSpace(out.String())
	b, rerr := os.ReadFile(tokenPath)
	if rerr != nil {
		return deskkit.Unverifiable(fmt.Sprintf("read auditor token from %s", tokenPath), rerr)
	}
	ghToken = strings.TrimSpace(string(b))
	if ghToken == "" {
		return deskkit.Unverifiable(fmt.Sprintf("auditor token at %s is empty", tokenPath), nil)
	}
	return nil
}

// identity renders the auditor's known GitHub login for evidence/display purposes. It reads
// deskkit.AppBinding — the App-NAME a role mints as (apps.env / <ROLE>_APP), never the
// roster's TRUST binding: the auditor never posts, so it is never bound in the roster's bot-
// slug map, and this program's identity line must still name a real `[bot]` login even on a
// deployment that has not added an (optional) roster echo entry for it. It is DISPLAY ONLY —
// never a comparison — the same posture cmd/deskevidence's verifierBotDisplay documents for
// the "verifier" role.
func identity() string {
	return deskkit.AppBinding("auditor") + "[bot]"
}
