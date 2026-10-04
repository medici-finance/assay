package main

// forge.go — the identity + resolver seam deskmerge's READS authenticate through
// (desktools-v2/03).
//
// deskmerge used to read a PR's state and its R-5 sign-off comment by shelling `gh` under
// whatever identity the operator's gh keyring held. Both reads now go through the resolved
// deskkit.Forge, handed this session's MINTED App token for the repo being read — the same
// shape every migrated read verb installs (cmd/deskread/forge.go). There is no ambient
// identity for a read to degrade onto: an unresolvable role, an unminted token or an empty
// one is a could-not-check, and deskmerge acts on nothing it could not read.
//
// The installation is derived from the repo argument at the call (repo.Slug() handed to the
// mint), never from GH_TOKEN / GH_REPO / HOME as deskmerge passes it — so even a present
// ambient token cannot redirect a read to another installation. The token is the role App's
// installation token for the repository's ACCOUNT: it is valid for every repository and
// permission of that installation, and narrowing it is desktools-v2/06, not this change. The
// minter child inherits the environment and honours its own documented role-named overrides
// (cmd/desktoken); those are out of scope here.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub returning a
	// fake token. Production binds it to the shared resolver.
	mintTokenFn = deskkit.GitHubRoleToken
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
	if !ok || owner == "" || name == "" {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("could-not-check: bad repo "+
			deskkit.StripControl(repo)+" — want owner/name", nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn(toolName)
	if err != nil {
		return nil, fr, deskkit.Unverifiable("could-not-check: cannot resolve the deskmerge App role to read "+
			deskkit.StripControl(repo)+" — the read path authenticates as a minted App token, never an "+
			"ambient forge-CLI identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
