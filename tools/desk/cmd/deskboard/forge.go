package main

// forge.go — the resolver seam the board's TYPED reads authenticate through.
//
// deskboard's peripheral reads (PR search, commit history, combined status, workflow-directory
// listing, single-commit reads) still shell `gh` through ghRun, behind the one narrowed permit
// row that names them and their follow-up brief. The board's CENTRAL reads — the bulk open-PR
// read and the two trust-gate reads — reach the forge through deskkit.ForgeFor instead, as
// TYPED ops that resolve on any configured forge (could-not-check where a backend cannot serve
// one). Unlike the ambient-fallback ghRun read path, ForgeFor is handed this session's minted
// App token and REFUSES a client without one: these three reads therefore do NOT degrade onto
// an ambient identity — a session with no resolvable role gets a refusal the board surfaces as
// could-not-check (the same fail-closed direction the peripheral reads already take on a 401).

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub token.
	mintTokenFn = deskkit.RoleTokenForRepo
	// forgeAPIBase redirects the resolved GitHub backend's API host at an httptest server in
	// tests; empty in production is the real host. Read at call time via the custody minter.
	forgeAPIBase string
	// sessionRoleFn resolves this session's App role, swapped in tests.
	sessionRoleFn = deskkit.SessionTokenRole
)

func init() {
	// Route ForgeFor's GitHub custody through this binary's own token lookup — the seam every
	// migrated desk command installs — so a test can stub the token and redirect the base URL.
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
		tok, _, merr := mintTokenFn(role, repo.Slug())
		if merr != nil {
			return "", "", merr
		}
		return tok, forgeAPIBase, nil
	})
}

// forgeFor resolves the forge serving repo for the board's typed reads. It is a package var
// so a test can inject a recorded backend for the three typed reads without a network or a
// minted credential, while the peripheral ghRun reads keep their own PATH-shim.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("deskboard")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the deskboard App role to read "+repo+
			" — the typed board reads authenticate as a minted App token, never an ambient gh identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
