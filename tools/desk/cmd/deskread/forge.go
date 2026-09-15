package main

// forge.go — the identity + resolver seam deskread authenticates through.
//
// Byte-for-byte the shape every migrated read verb installs (see cmd/issueboard/forge.go for the
// defect it removes): the backend is handed this session's MINTED App token and refuses to be
// constructed without one, so there is no ambient identity for a read to silently degrade onto.
// A read that cannot resolve a role is a could-not-check the caller reports, never an anonymous
// read that comes back empty and looks like an answer.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub returning a
	// fake token. Production binds it to the shared resolver.
	mintTokenFn = deskkit.RoleTokenForRepo
	// forgeAPIBase redirects the resolved GitHub backend's API host at an httptest server in
	// tests; empty in production means the real host.
	forgeAPIBase string
	// sessionRoleFn resolves this session's App role, swapped in tests.
	sessionRoleFn = deskkit.SessionTokenRole
)

func init() {
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
		tok, _, merr := mintTokenFn(role, repo.Slug())
		if merr != nil {
			return "", "", merr
		}
		return tok, forgeAPIBase, nil
	})
}

// forgeFor resolves the forge serving repo and returns the backend plus the coordinate. It is a
// package var so a test can inject a recorded backend without a network or a minted credential.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("deskread")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the deskread App role to read "+repo+
			" — the read path authenticates as a minted App token, never an ambient forge-CLI identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
