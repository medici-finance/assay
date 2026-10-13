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
	"sync"

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

// custodyRole is the App role the custody path resolved this run, recorded for the envelope's
// identity object. It is set from forgeFor (the one place the role is resolved) so the role
// resolver is called exactly as often as before the identity record existed.
var (
	custodyRoleMu sync.Mutex
	custodyRoleV  string
)

func recordCustodyRole(role string) {
	custodyRoleMu.Lock()
	custodyRoleV = role
	custodyRoleMu.Unlock()
}

func resetCustodyRole() { recordCustodyRole("") }

func custodyRoleSeen() string {
	custodyRoleMu.Lock()
	defer custodyRoleMu.Unlock()
	return custodyRoleV
}

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
	recordCustodyRole(role)
	f, ferr := deskkit.ForgeFor(fr, role)
	if ferr != nil {
		return nil, fr, ferr
	}
	return f, fr, nil
}

// forgeForRun picks the transport for one read: the CI workflow-token constructor when the run
// settled the opt-in, the custody path otherwise. There is no fallback from the first to the
// second: a refusal on the CI path is a refusal.
func forgeForRun(repo string, o readOpts) (deskkit.Forge, deskkit.ForgeRepo, error) {
	if o.ciT != nil {
		return ciForgeFor(repo, o.ciT)
	}
	return forgeFor(repo)
}

// ciForgeFor is the one function on the CI path that builds a backend. A repository other than
// the job's own is not read and is never sent the token (it surfaces as a "partial" entry); for
// the job's own repository it hands the token to deskkit.ReadOnlyForgeForCIToken, which re-checks
// the token shape and the repository binding and returns a read-only, outbound-checked backend.
func ciForgeFor(repo string, ci *ciTransport) (deskkit.Forge, deskkit.ForgeRepo, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, deskkit.ForgeRepo{}, deskkit.Unverifiable("bad repo "+repo, nil)
	}
	fr := deskkit.ForgeRepo{Owner: owner, Name: name}
	if !strings.EqualFold(repo, ci.repository) {
		return nil, fr, deskkit.Unverifiable("the CI workflow-token transport reads only the job's own repository", nil)
	}
	f, _, err := deskkit.ReadOnlyForgeForCIToken(fr, ci.repository, ci.token)
	if err != nil {
		return nil, fr, err
	}
	return f, fr, nil
}
