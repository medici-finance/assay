package main

import kit "github.com/medici-finance/assay/tools/desk/internal/deskkit"

// An aliased import must not hide the minter.
func mintViaAlias(role, owner string) error {
	_, _, err := kit.RoleTokenForOwner(role, owner)
	return err
}

type notTheMinter struct{}

// A method that merely shares the name is not the package minter and must NOT be reported.
func (notTheMinter) RoleTokenForRepo(role, repo string) {}

func useNotTheMinter() { notTheMinter{}.RoleTokenForRepo("role", "example-org/example-repo") }
