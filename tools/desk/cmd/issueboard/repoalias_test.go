package main

// issueboard used to keep its OWN copy of the short-name switch — a strict subset of
// deskboard's that had already drifted (it lacked ledger/recon/props). That private
// copy is the antipattern removed here: issueboard now renders
// through the SAME shared deskkit resolver deskboard uses. These tests pin the
// delegation, the generic default when ASSAY_REPO_ALIASES is unset, and the
// cross-board shared-value property (the same repo renders the SAME label whether the
// call goes through issueboard's path or the resolver deskboard also calls).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const minimalRosterNoAliases = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private,example-org/agents:ci:private,example-org/examples:no-ci:private,example-org/console:ci:private,medici-finance/assay:ci:private,example-org/example-k8s:ci:public,example-org/example-reconciler:ci:private,example-org/org-slides:no-ci:private,example-org/proposals:no-ci:public,example-org/platform:ci:private,example-org/demo-slides:no-ci:private,example-org/assay-slides:no-ci:private,example-org/example-reconciler-slides:no-ci:private
`

func plantRosterNoAliases(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(minimalRosterNoAliases), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
}

// TestShortRepoDelegatesToSharedResolver: issueboard's shortRepo is a thin reading of
// the shared resolver with no private switch of its own.
func TestShortRepoDelegatesToSharedResolver(t *testing.T) {
	for _, repo := range deskkit.AllowedRepos() {
		if got, want := shortRepo(repo), deskkit.RepoShortLabel(repo); got != want {
			t.Errorf("shortRepo(%q) = %q but deskkit.RepoShortLabel = %q — issueboard is not "+
				"routing through the shared resolver (a private copy has crept back)", repo, got, want)
		}
	}
}

// TestSharedResolverAcrossBoards is brief-07 Verify row 4 (FLOW + NEIGHBOUR): the SAME
// repo, resolved through issueboard's shortRepo and through the resolver deskboard also
// calls, yields the SAME label against ONE alias fixture (the package's TestMain roster).
// Grouping flows through the one mechanism, not two copies. deskboard is the adjacent
// consumer; because both delegate to deskkit.RepoShortLabel, matching it here is the
// same value deskboard prints.
func TestSharedResolverAcrossBoards(t *testing.T) {
	// The configured fixture is live (TestMain); example-k8s is a repo where the house
	// alias differs from the generic default, so a divergence would show here.
	for _, repo := range deskkit.AllowedRepos() {
		issueboardLabel := shortRepo(repo)
		sharedLabel := deskkit.RepoShortLabel(repo) // the exact call deskboard's shortRepo makes
		if issueboardLabel != sharedLabel {
			t.Fatalf("issueboard and the shared resolver disagree on %q: %q vs %q — the boards are "+
				"not grouping through one mechanism", repo, issueboardLabel, sharedLabel)
		}
	}
	// Positive control: the fixture actually EXERCISES an override, so equality above is
	// not vacuous (both sides generic).
	if got := deskkit.RepoShortLabel("example-org/example-k8s"); got == "example-k8s" {
		t.Fatalf("the alias fixture did not override example-k8s (still generic %q) — the "+
			"cross-board equality proves nothing without a live override", got)
	}
}

// TestGroupingGenericDefaultWhenAliasesUnset: with ASSAY_REPO_ALIASES unset issueboard's
// labels are the generic derivation, and no house label survives from compiled residue.
func TestGroupingGenericDefaultWhenAliasesUnset(t *testing.T) {
	plantRosterNoAliases(t)

	generic := map[string]string{
		"example-org/example-k8s":        "example-k8s",
		"example-org/example-reconciler": "example-reconciler",
		"example-org/proposals":          "proposals",
	}
	for repo, want := range generic {
		if got := shortRepo(repo); got != want {
			t.Errorf("with aliases UNSET, shortRepo(%q) = %q, want generic %q", repo, got, want)
		}
	}
}
