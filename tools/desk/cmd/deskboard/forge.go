package main

// forge.go — the resolver seam every board read authenticates through.
//
// As of the forge-neutral migration, deskboard's reads ALL reach the forge as TYPED ops on
// deskkit's Forge seam: the CENTRAL reads (the bulk open-PR read and the two trust-gate reads)
// alongside the former-peripheral reads (PR search, commit history, combined status,
// workflow-directory listing, single-commit reads, compare, raw diff) that used to shell `gh`
// through the now-deleted ghRun choke point — so cmd/deskboard carries no forge-CLI literal.
// Every op resolves on any configured forge (could-not-check where a backend cannot serve one).
// ForgeFor is handed this session's minted App token and REFUSES a client without one: a read
// therefore never degrades onto an ambient identity — a session with no resolvable role gets a
// refusal the board surfaces as could-not-check.

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

// forgeFor resolves the forge serving repo for the board's reads. It is a package var so a
// test can inject a recorded backend for the typed reads without a network or a minted
// credential.
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

// forgeForOwner resolves the forge serving an OWNER, for the one owner-wide read
// (SearchOpenChanges). The forge and its minted token are resolved per account, so this reuses
// forgeFor on a WATCHED repo under the owner — the scope-reconciliation verb only asks about
// owners derived from the watched set, so such a repo always exists. It refuses (could-not-check)
// when the owner has no watched repo to resolve a coordinate from, rather than guessing one.
var forgeForOwner = func(owner string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	for _, repo := range deskkit.AllowedRepos() {
		if o, _, ok := strings.Cut(repo, "/"); ok && o == owner {
			return forgeFor(repo)
		}
	}
	return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable(
		"cannot resolve a forge for owner "+owner+" — no watched repo under it to bind the account's App token", nil)
}
