package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestShortRepoLabelsAreInjective — every census repo must render a DISTINCT board label.
//
// This is a real read hazard, not a style point: the boards print `shortRepo(repo)` as
// the only repo identifier on the row, so two repos sharing a label means a human cannot
// tell which repository an item belongs to. The live hazard this guards is the family of
// `<grouping>-slides` repos: a single `slides` suffix case would render every one of them
// as "slides", so the per-repo forms must each fall through to a distinct label.
//
// It is driven off deskkit.AllowedRepos() rather than a copied list on purpose: a repo
// added to the census with a label that collides fails HERE, with no second list to
// update. That is the drift anti-pattern shape — a parallel hand-maintained list is what goes stale.
func TestShortRepoLabelsAreInjective(t *testing.T) {
	seen := map[string]string{}
	for _, repo := range deskkit.AllowedRepos() {
		label := shortRepo(repo)
		if label == "" {
			t.Fatalf("shortRepo(%q) is empty", repo)
		}
		if other, dup := seen[label]; dup {
			t.Fatalf("shortRepo collision: %q and %q both render %q — a board row cannot "+
				"say which repo it is about; add a case to shortRepo", other, repo, label)
		}
		seen[label] = repo
	}
}

// TestShortRepoKnownLabels pins the specific labels the collision fix introduced, so a
// later reordering of shortRepo's switch (where a shorter suffix shadows a longer one)
// fails on the value and not only on the injectivity property.
func TestShortRepoKnownLabels(t *testing.T) {
	cases := map[string]string{
		"medici-finance/assay":                  "assay",
		"example-org/tracker":                   "tracker",
		"example-org/agents":                    "agents",
		"example-org/examples":                  "examples",
		"example-org/console":                   "console",
		"example-org/example-k8s":               "ledger",
		"example-org/example-reconciler":        "recon",
		"example-org/proposals":                 "props",
		"example-org/platform":                  "platform",
		"example-org/org-slides":                "org-slides",
		"example-org/medici-slides":             "medici-slides",
		"example-org/assay-slides":              "assay-slides",
		"example-org/example-reconciler-slides": "example-reconciler-slides",
	}
	for repo, want := range cases {
		t.Run(repo, func(t *testing.T) {
			if got := shortRepo(repo); got != want {
				t.Fatalf("shortRepo(%q) = %q, want %q", repo, got, want)
			}
		})
	}
}
