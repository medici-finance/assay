package main

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #1573
func TestAuditorMintsInProcess(t *testing.T) {
	old := githubRoleTokenFn
	t.Cleanup(func() { githubRoleTokenFn = old; ghToken = "" })

	var gotRole, gotRepo string
	githubRoleTokenFn = func(role, repo string) (string, string, error) {
		gotRole, gotRepo = role, repo
		return "example-placeholder-value\n", "/nonexistent/auditor-cache", nil
	}
	ghToken = ""
	if err := mintAuditorToken("example-org/example-repo"); err != nil {
		t.Fatalf("mintAuditorToken: %v", err)
	}
	if gotRole != "auditor" || gotRepo != "example-org/example-repo" {
		t.Fatalf("minter called with (%q, %q), want (auditor, example-org/example-repo)", gotRole, gotRepo)
	}
	if ghToken != "example-placeholder-value" {
		t.Fatalf("ghToken = %q, want the trimmed minted value", ghToken)
	}
}

// regression: #1573
func TestAuditorMintRefusalsPass(t *testing.T) {
	old := githubRoleTokenFn
	t.Cleanup(func() { githubRoleTokenFn = old; ghToken = "" })

	refusal := deskkit.Refused("example refusal")
	githubRoleTokenFn = func(string, string) (string, string, error) { return "", "", refusal }
	ghToken = ""
	if err := mintAuditorToken("example-org/example-repo"); !errors.Is(err, refusal) {
		t.Fatalf("mint refusal = %v, want the minter's own refusal unchanged", err)
	}

	githubRoleTokenFn = func(string, string) (string, string, error) { return "  ", "/x", nil }
	err := mintAuditorToken("example-org/example-repo")
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || ghToken != "" {
		t.Fatalf("empty mint = %v (ghToken %q), want an unverifiable refusal and no token", err, ghToken)
	}
}
