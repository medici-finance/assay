// Package main is a FIXTURE for TestGitHubMinterGuardCatchesPlantedCallers: it plants the
// shapes a forge-blind caller of the GitHub App minter takes. It lives under testdata, so it is
// never built and never scanned by the real-tree guard.
package main

import "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// A function VALUE bound to a package var — the shape most pre-#1573 sites used.
var mintTokenFn = deskkit.RoleTokenForRepo

// A direct call with no forge resolution in front of it — the #1573 role-init shape.
func forgeBlindRoleInit(role, owner string) string {
	_, path, _ := deskkit.RoleTokenForOwner(role, owner)
	return path
}

func main() { _, _ = mintTokenFn, forgeBlindRoleInit }
