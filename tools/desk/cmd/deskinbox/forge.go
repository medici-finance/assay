package main

// forge.go — the identity + resolver seam deskinbox's reads authenticate through, the same
// seam cmd/deskboard and cmd/issueboard install (deskkit.ForgeFor, handed this session's
// minted App token). deskinbox never shells `gh` and never falls back to an ambient
// credential: a session with no resolvable loop identity gets a refusal it surfaces as
// could-not-check, exactly like every other migrated desk read.
//
// The bash oracle (assay-inbox.sh) authenticated as whatever the ambient `gh` keyring held —
// fine for a human's own terminal, but a source of the exact silent-empty-read defect
// deskboard's token.go documents elsewhere in this tree. Routing through the resolved forge
// is a deliberate IMPROVEMENT over the oracle's identity model, not a behavioural drift the
// parity tests need to track: the oracle's OWN doc says nothing about which account gh uses.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub token.
	mintTokenFn = deskkit.RoleTokenForRepo
	// forgeAPIBase redirects the resolved GitHub backend's API host at an httptest server in
	// tests; empty in production means the real host. Read at call time via the custody
	// minter, and shared with the package-local comment-detail client (detail.go) so both
	// paths point at the same fake backend under test.
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

// forgeFor resolves the forge serving repo, and the coordinate. A package var so a test can
// inject a recorded backend without a network or a minted credential.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("deskinbox")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the deskinbox App role to read "+repo+
			" — the read path authenticates as a minted App token, never an ambient gh identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
