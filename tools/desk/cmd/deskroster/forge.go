package main

// forge.go — the resolver seam deskroster's display reads authenticate through.
//
// deskroster's two PR reads (ghViewPR, ghListOpenPRs in roster.go) annotate the roster
// listing with each open change's state/draft/title. They reach the forge as TYPED ops on
// deskkit's Forge seam — GetPullRequest and ListOpenChanges — never by shelling `gh`, so
// cmd/deskroster carries no forge-CLI literal (the closed-surface brief, forge-gitlab/08).
//
// The reads authenticate as this session's minted App token, resolved from the loop identity
// the session presents ($DESK_LOOP), the same custody deskboard's reads use. A session with no
// resolvable role gets a refusal, which forgeFor returns as an error and the caller renders as a
// soft miss ("?"/omitted row) — the SAME graceful degradation the former `gh` shell-out gave on
// any failure. These are READ-only annotations; deskroster mints no token for and performs no
// forge WRITE, so this changes transport, not the token-custody posture of any write path.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	// mintTokenFn is the per-repo token-lookup seam, swapped in tests for a stub token.
	mintTokenFn = deskkit.RoleTokenForRepo
	// forgeAPIBase redirects the resolved GitHub backend's API host at an httptest server in
	// tests; empty in production is the real host.
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

// forgeFor resolves the forge serving repo for the roster's display reads. It is a package var
// so a test can inject a recorded backend without a network or a minted credential.
var forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	role, _, err := sessionRoleFn("deskroster")
	if err != nil {
		return nil, fr, deskkit.Unverifiable("cannot resolve the deskroster App role to read "+repo+
			" — the roster's typed PR reads authenticate as a minted App token, never an ambient gh identity", err)
	}
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}
