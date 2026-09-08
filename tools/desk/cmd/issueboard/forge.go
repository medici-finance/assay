package main

// forge.go — the identity + resolver seam the read path authenticates through, replacing the
// ambient `gh` CLI this board used to shell.
//
// THE DEFECT THE OLD PATH CARRIED (the deskboard token.go lesson, applied here). Every read
// shelled `gh` with no minted token, so it authenticated as whatever account the ambient
// keyring held. On a machine whose desk config home is not the operator's own that account
// cannot authenticate, and a GraphQL read has been observed to come back EMPTY rather than
// error — an absence that reads like an answer, so it never trips a fail-closed path. Reaching
// the forge through deskkit.ForgeFor removes the whole class: the backend is handed this
// session's minted App token and REFUSES to construct a client without one, so there is no
// ambient identity for a read to silently degrade onto.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub returning a
	// fake token. Production binds it to the shared resolver.
	mintTokenFn = deskkit.RoleTokenForRepo
	// forgeAPIBase redirects the resolved GitHub backend's API host at an httptest server in
	// tests; empty in production means the real host. Read at call time via the custody minter.
	forgeAPIBase string
	// sessionRoleFn resolves this session's App role, swapped in tests.
	sessionRoleFn = deskkit.SessionTokenRole
)

func init() {
	// Route ForgeFor's GitHub custody through this binary's own token lookup — the seam every
	// migrated desk command installs — so a test can stub the token and redirect the base URL
	// without a real App credential.
	deskkit.SetGitHubCustodyMinter(func(role string, repo deskkit.ForgeRepo) (token, baseURL string, err error) {
		tok, _, merr := mintTokenFn(role, repo.Slug())
		if merr != nil {
			return "", "", merr
		}
		return tok, forgeAPIBase, nil
	})
}

// forgeFor resolves the forge serving repo and returns the backend plus the coordinate. The
// role is this session's App role; issueboard is a READ path, but ForgeFor never falls back to
// an ambient identity — a session with no resolvable role gets a refusal the caller surfaces
// as could-not-check, never a silent ambient read. It is a package var so a test can inject a
// recorded backend without a network or a minted credential.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("issueboard")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the issueboard App role to read "+repo+
			" — the read path authenticates as a minted App token, never an ambient gh identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
