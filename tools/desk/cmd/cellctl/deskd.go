package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdDeskd stands the cell's persistent deskd.
//
// It is ATTENDED-ONLY: deskd stands live cross-org reads on freshly minted (GitHub) or
// hand-provisioned (GitLab) credentials, so the operator starts it from their own shell and a
// token act is never unattended.
//
// THE CREDENTIAL PATH IS THE WHOLE POINT OF THIS FILE, and it is the one place the port most
// deliberately does NOT follow the oracle. The shell script hand-builds a signed RS256 App
// assertion by shelling to a TLS toolkit, base64url-encodes it by hand, and exchanges it for
// per-org installation tokens with `curl` + `python3`. This file mints NOTHING of its own: it
// asks deskkit for the role credential and lets the custody code that every other desk verb uses
// answer.
//
// The port's brief asserts that at the SOURCE level — the package names no
// signing primitive, no certificate package, no bearer-assertion format and no TLS-toolkit
// shell-out anywhere — and row 10 asserts the deskkit call is really made. `deskd` is outside the
// parity matrix by construction (the oracle gives it no DRY_RUN plan to diff), so those two rows
// ARE its proof, together with the live-mint check the brief hands to the online lane.
func cmdDeskd(cell string) {
	c := loadCell(cell)
	if c.Kind == "container" {
		die("container cells do not run host deskd")
	}
	if c.Env.GetOr("CELL_ATTENDED", "0") != "1" {
		die("deskd stands live cross-org reads — run from YOUR shell with CELL_ATTENDED=1")
	}
	bin := filepath.Join(c.Dir, "bin", "deskd")
	if !isExecFile(bin) {
		die("no deskd at %s", bin)
	}
	var extraEnv []string
	switch c.Forge {
	case "github":
		extraEnv = c.deskdMintGitHub()
	case "gitlab":
		extraEnv = c.deskdProvisionGitLab()
	default:
		die("cell.env: CELL_FORGE=%s is not a known forge (github|gitlab)", c.Forge)
	}
	if err := os.MkdirAll(filepath.Dir(c.DeskdIndex), 0o700); err != nil {
		die("deskd: cannot create the index directory: %v", err)
	}
	// Go/no-go BEFORE the persistent run: --once walks every slice once and exits non-zero on a
	// bad config or a missing installation/token, so a broken cell fails here rather than
	// serving stale reads.
	fmt.Println("[deskd] go/no-go: --once across the cell repos (expect: all slices checked, no owner/group 404s)")
	once := exec.Command(bin, "--config", c.CellsCfg, "--index", c.DeskdIndex, "--once")
	once.Env = append(os.Environ(), extraEnv...)
	once.Stdin, once.Stdout, once.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := once.Run(); err != nil {
		exitWith(exitStatus(err))
	}
	fmt.Printf("[deskd] standing on %s (index %s)\n", c.DeskdAddr, c.DeskdIndex)
	runForeground([]string{bin, "--config", c.CellsCfg, "--index", c.DeskdIndex, "--addr", c.DeskdAddr},
		append(os.Environ(), extraEnv...), "")
}

// deskdMintGitHub obtains ONE credential PER ORG and returns them as env assignments for the
// deskd process. An installation token is scoped to its own installation, so a single org's
// token returns 404 for every other org's slice — which reads as "repo missing", not as "wrong
// token", and is the failure this loop exists to prevent.
//
// Every token comes from deskkit.RoleTokenForRepo. Nothing here signs, encodes or transports a
// key: the App private key is read, held and used only by deskkit's custody code, the same path
// every other desk verb's credential travels, under the same roster bindings.
func (c *Cell) deskdMintGitHub() []string {
	orgs := c.Env.Get("ORGS")
	if orgs == "" {
		die("cell.env: ORGS is not set")
	}
	// The forge the cell's own repo resolves to, asked of deskkit rather than derived from
	// cell.env's CELL_FORGE a second time. This is also the agreement check: a roster entry
	// naming another forge for this repo is refused HERE, before any credential is read.
	repo := forgeRepoOf(c.Env.Get("CELL_REPO_SLUG"), orgs)
	_, res, err := deskkit.ResolveForge(repo, "desk")
	if err != nil {
		die("deskd: could not resolve the forge for %s: %v", repo.Slug(), err)
	}
	fmt.Printf("[deskd] forge resolved for %s: %s (%s)\n", repo.Slug(), res.Kind, res.Source)

	var out []string
	var minted []string
	for _, org := range strings.Fields(strings.ReplaceAll(orgs, ",", " ")) {
		// RoleTokenForRepo answers per REPO coordinate; an installation is per OWNER, so the
		// owner is what varies and the name is the cell's own repo under that owner.
		slug := org + "/" + repo.Name
		tok, _, terr := deskkit.RoleTokenForRepo("desk", slug)
		if terr != nil {
			die("deskd: no credential for %s — %v (install the App there first, or bind the role in the cell's roster)", org, terr)
		}
		v := "DESKD_GITHUB_TOKEN_" + strings.ToUpper(strings.ReplaceAll(org, "-", "_"))
		out = append(out, v+"="+tok)
		minted = append(minted, org)
	}
	// The values are never echoed — only which orgs were covered.
	fmt.Printf("[deskd] per-org credentials resolved (%s) — not echoed\n", strings.Join(minted, ","))
	return out
}

// deskdProvisionGitLab verifies the hand-provisioned role token store and points deskd at the
// cell's GitLab endpoint. It MINTS NOTHING: GitLab role tokens rotate deliberately and by hand,
// so this path only asserts the store is present and readable. No signed assertion, no
// installation token, no private key — on either forge, in this port.
func (c *Cell) deskdProvisionGitLab() []string {
	if c.Env.Get("GITLAB_GROUP") == "" {
		die("cell.env: GITLAB_GROUP is not set (the GitLab group this cell reads)")
	}
	store := c.Env.Get("GITLAB_TOKEN_STORE")
	if store == "" || !isDir(store) {
		die("cell.env: GITLAB_TOKEN_STORE is not a directory: %s — provision the role token store first", orDefault(store, "unset"))
	}
	tokFile := c.Env.GetOr("DESKD_GITLAB_TOKEN_FILE", "/nonexistent")
	if !isReadable(tokFile) {
		die("deskd GitLab read token not readable: %s — place gitlab-deskd.token in the role token store (mode 0600); it is not minted for you", c.Env.GetOr("DESKD_GITLAB_TOKEN_FILE", "unset"))
	}
	fmt.Printf("[deskd] GitLab cell: reading group %s via %s from role token store %s (no token minted — rotation is a hand step)\n",
		c.Env.Get("GITLAB_GROUP"), c.ForgeAPIBase, store)
	return []string{"GITLAB_API_BASE=" + c.ForgeAPIBase}
}

// forgeRepoOf turns the cell's own repo coordinate into deskkit's ForgeRepo. A cell that names
// CELL_REPO_SLUG (every scrubbed cell, and any other cell that sets it) is taken at its word;
// otherwise the first org in ORGS supplies the owner and the checkout's directory name the
// repository name, which is the same pair the oracle's per-org loop walks.
func forgeRepoOf(slug, orgs string) deskkit.ForgeRepo {
	if i := strings.IndexByte(slug, '/'); i > 0 {
		return deskkit.ForgeRepo{Owner: slug[:i], Name: slug[i+1:]}
	}
	first := ""
	if f := strings.Fields(strings.ReplaceAll(orgs, ",", " ")); len(f) > 0 {
		first = f[0]
	}
	if first == "" {
		die("deskd: neither CELL_REPO_SLUG nor ORGS names an owner to resolve the forge against")
	}
	name := slug
	if name == "" {
		name = filepath.Base(mustWD())
	}
	return deskkit.ForgeRepo{Owner: first, Name: name}
}

func mustWD() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
