package main

// forge.go — the identity + resolver seam the trust gate authenticates through, replacing the
// ambient `gh` CLI the probe used to shell.
//
// The trust gate reads repos where ARBITRARY external accounts author issues. Reaching the
// forge through deskkit.ForgeFor hands the backend this session's minted App token — the
// backend REFUSES a client without one — so the gate's reads never silently degrade onto an
// ambient identity, which on a trust gate is the difference between a read and a guess.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub. Production
	// binds it to the shared resolver.
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

// forgeFor resolves the forge serving repo. ForgeFor never falls back to an ambient identity;
// a session with no resolvable role gets a refusal the trust gate surfaces as could-not-check.
func forgeFor(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo slug "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("scanloop")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the scanloop App role to read "+repo+
			" — the trust gate authenticates as a minted App token, never an ambient gh identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
