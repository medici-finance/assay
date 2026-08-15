package main

// deskboard's grouping now flows through the ONE shared deskkit resolver
// (deskkit.RepoShortLabel / deskkit.RepoProductGroup). These tests pin that the
// board keeps NO private switch of its own (delegation identity), and that with
// ASSAY_REPO_ALIASES UNSET the board groups by the generic default alone — proving
// the house labels the other suites assert come from CONFIG, not compiled residue.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// minimalRosterNoAliases is a valid roster with the census but NO ASSAY_REPO_ALIASES,
// so the resolver falls to its generic default.
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

// TestShortRepoDelegatesToSharedResolver: the board's shortRepo is a thin reading of
// the shared resolver, with no private label switch. If a copy were reintroduced the
// two would diverge on some census repo.
func TestShortRepoDelegatesToSharedResolver(t *testing.T) {
	for _, repo := range deskkit.AllowedRepos() {
		if got, want := shortRepo(repo), deskkit.RepoShortLabel(repo); got != want {
			t.Errorf("shortRepo(%q) = %q but deskkit.RepoShortLabel = %q — deskboard is not "+
				"routing through the shared resolver", repo, got, want)
		}
	}
}

// TestProductDelegatesToSharedResolver: same, for the product grouping.
func TestProductDelegatesToSharedResolver(t *testing.T) {
	for _, repo := range deskkit.AllowedRepos() {
		if got, want := product(repo), deskkit.RepoProductGroup(repo); got != want {
			t.Errorf("product(%q) = %q but deskkit.RepoProductGroup = %q — deskboard is not "+
				"routing through the shared resolver", repo, got, want)
		}
	}
}

// TestGroupingGenericDefaultWhenAliasesUnset: with ASSAY_REPO_ALIASES unset the labels
// and products are the generic derivation (last path segment / owner) — NO house label
// like "ledger" or "recon" appears, and the labels are still injective over the census.
func TestGroupingGenericDefaultWhenAliasesUnset(t *testing.T) {
	plantRosterNoAliases(t)

	generic := map[string]string{
		"example-org/example-k8s":        "example-k8s", // NOT "ledger" — that lived only in config
		"example-org/example-reconciler": "example-reconciler",
		"example-org/proposals":          "proposals",
		"medici-finance/assay":           "assay",
	}
	for repo, want := range generic {
		if got := shortRepo(repo); got != want {
			t.Errorf("with aliases UNSET, shortRepo(%q) = %q, want generic %q", repo, got, want)
		}
		if got := product(repo); got != repoOwnerForTest(repo) {
			t.Errorf("with aliases UNSET, product(%q) = %q, want generic owner %q", repo, got, repoOwnerForTest(repo))
		}
	}

	seen := map[string]string{}
	for _, repo := range deskkit.AllowedRepos() {
		label := shortRepo(repo)
		if other, dup := seen[label]; dup {
			t.Fatalf("generic-default label collision: %q and %q both render %q", other, repo, label)
		}
		seen[label] = repo
	}
}

func repoOwnerForTest(repo string) string {
	for i := 0; i < len(repo); i++ {
		if repo[i] == '/' {
			return repo[:i]
		}
	}
	return repo
}
