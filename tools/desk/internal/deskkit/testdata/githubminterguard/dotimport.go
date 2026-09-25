package main

import . "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// A dot-import makes the minter a bare identifier; it must still be reported.
func mintViaDot(role, repo string) error {
	_, _, err := RoleTokenForRepo(role, repo)
	return err
}
