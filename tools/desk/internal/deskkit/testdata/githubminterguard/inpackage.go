// Package deskkit is a FIXTURE for TestGitHubMinterGuardCatchesPlantedCallers: it plants, INSIDE
// the minter's own package, the shapes that reach the GitHub App mint chain around the exported
// names — a pass-through through the resolver's GitHub arm with no forge check in front of it,
// a direct call to the primitive, and the primitive's seam taken as a value or called. It lives
// under testdata, so it is never built and never scanned by the real-tree guard.
package deskkit

// An exported, forge-blind entry point that aliases the GitHub arm (#1587 review F1).
func PlantedForgeBlind(role, repo string) (string, string, error) {
	return githubAppRoleToken(role, repo)
}

// Pass-throughs one link up from the arm (#1587 review F1, round 2): custody and its GitHub
// branch resolve no forge, so calling either is the same forge-blind mint.
func PlantedViaGitHubCustody(role string, r ForgeRepo) (string, string, error) {
	return githubCustody(role, r)
}

func PlantedViaCustody(role string, r ForgeRepo) (string, string, error) {
	return custody(ForgeGitHub, role, r)
}

// The per-process memo, read directly: an already-minted token with no fork and no forge check.
func plantedMemoRead(k roleOwnerKey) string {
	e, _ := lookupRoleTokenMemo(k)
	return e.token
}

func plantedMemoMap(k roleOwnerKey) string {
	return roleTokenMemo[k].token
}

// The primitive beneath the exported minters, reached directly.
func plantedPrimitive(role, owner string) string {
	path, _, _ := mintRoleToken(role, owner)
	return path
}

// The seam itself, called.
func plantedSeamCall(role, owner string) string {
	path, _, _ := tokenMinter(role, owner)
	return path
}

// The seam taken as a package-level value.
var plantedSeamValue = tokenMinter

// NOT references: a declared name that merely shares a link's spelling, a field and a method
// of that name, and a local that shadows nothing the guard confines.
type plantedHolder struct{ mintRoleToken func() }

func (plantedHolder) githubAppRoleToken() {}

var githubAppRoleTokenCount = 0

func plantedNotReferences(h plantedHolder) {
	h.mintRoleToken()
	h.githubAppRoleToken()
	githubAppRoleTokenCount++
}
