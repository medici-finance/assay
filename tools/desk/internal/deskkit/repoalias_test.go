package deskkit

// Tests for the shared repo-taxonomy resolver (repoalias.go) and its ASSAY_REPO_ALIASES
// config (rosterconfig.go). The properties pinned here:
//
//   - GENERIC default when unset: short = last path segment, product = owner. No house
//     name is compiled in, so an unconfigured tree groups by owner/basename and nothing else.
//   - CONFIGURED override unions on top: a repo named in ASSAY_REPO_ALIASES takes the
//     configured short/product; an EMPTY facet keeps the generic default for that facet.
//   - FAIL-CLOSED parse: a malformed entry refuses the WHOLE roster, exactly as a bad
//     ASSAY_ALLOWED_REPOS entry does — never a silent fall-back to generic derivation.
//   - P3 echo carries the effective alias set (a settings-only change is visible on the run).

import (
	"bytes"
	"strings"
	"testing"
)

// TestRepoAliasGenericDefaultWhenUnset: with no ASSAY_REPO_ALIASES set, the resolver
// derives short = last path segment and product = owner, and names no house bucket.
func TestRepoAliasGenericDefaultWhenUnset(t *testing.T) {
	withRoster(t, goldenRoster()) // roster present, but NO ASSAY_REPO_ALIASES

	cases := []struct {
		repo, short, product string
	}{
		{"example-org/example-k8s", "example-k8s", "example-org"},
		{"medici-finance/assay", "assay", "medici-finance"},
		{"one/two", "two", "one"},
		{"noslug", "noslug", "noslug"}, // no '/': owner and basename are the slug itself
	}
	for _, c := range cases {
		if got := RepoShortLabel(c.repo); got != c.short {
			t.Errorf("RepoShortLabel(%q) = %q, want generic %q", c.repo, got, c.short)
		}
		if got := RepoProductGroup(c.repo); got != c.product {
			t.Errorf("RepoProductGroup(%q) = %q, want generic %q", c.repo, got, c.product)
		}
	}
}

// TestRepoAliasConfiguredOverride: a configured alias wins over the generic default, an
// EMPTY facet keeps the generic default for that facet, and a full owner/name key takes
// precedence over a bare-basename key for the same repo.
func TestRepoAliasConfiguredOverride(t *testing.T) {
	r := goldenRoster()
	r[EnvRepoAliases] = strings.Join([]string{
		"example-k8s=ledger:ledger",            // both facets overridden, keyed by basename
		"example-reconciler=recon:",            // short overridden, product stays generic (owner)
		"proposals=:demo",                      // product overridden, short stays generic (basename)
		"example-org/console=cons:consoleprod", // full-slug key
		"console=WRONG:WRONG",                  // basename key for the same name — full slug must win
	}, ",")
	withRoster(t, r)

	if !EffectiveConfig().Configured() {
		t.Fatalf("roster with a valid ASSAY_REPO_ALIASES refused: %v", EffectiveConfig().Problems)
	}

	check := func(repo, short, product string) {
		t.Helper()
		if got := RepoShortLabel(repo); got != short {
			t.Errorf("RepoShortLabel(%q) = %q, want %q", repo, got, short)
		}
		if got := RepoProductGroup(repo); got != product {
			t.Errorf("RepoProductGroup(%q) = %q, want %q", repo, got, product)
		}
	}
	// basename key, both facets overridden.
	check("example-org/example-k8s", "ledger", "ledger")
	// short overridden, product empty → generic owner.
	check("example-org/example-reconciler", "recon", "example-org")
	// product overridden, short empty → generic basename.
	check("example-org/proposals", "proposals", "demo")
	// full-slug key beats the bare-basename key for the same repo.
	check("example-org/console", "cons", "consoleprod")
	// a repo whose basename matches the "console" key but under a DIFFERENT owner still
	// resolves via basename (no full-slug entry for it), proving basename lookup is live.
	check("other-org/console", "WRONG", "WRONG")
}

// TestRepoAliasMalformedRefuses is the fail-closed property (brief-05 row-shape / brief-07
// row 6): a malformed ASSAY_REPO_ALIASES value collapses the WHOLE configuration to
// unconfigured with a loud reason, rather than silently dropping the bad entry and
// grouping by the generic default while broken config is present.
func TestRepoAliasMalformedRefuses(t *testing.T) {
	for _, bad := range []struct{ name, value string }{
		{"no equals", "example-k8s:ledger"},     // missing '=' between repo and spec
		{"no colon", "example-k8s=ledger"},      // missing ':' between short and product
		{"empty key", "=ledger:ledger"},         // empty repo key
		{"configures nothing", "example-k8s=:"}, // both facets empty
		{"duplicate repo", "k8s=a:b,k8s=c:d"},   // same repo aliased twice
	} {
		t.Run(bad.name, func(t *testing.T) {
			r := goldenRoster()
			r[EnvRepoAliases] = bad.value
			withRoster(t, r)
			cfg := EffectiveConfig()
			if cfg.Configured() {
				t.Fatalf("a malformed ASSAY_REPO_ALIASES (%q) did NOT refuse — a broken grouping "+
					"config must fail closed, never fall back to generic derivation", bad.value)
			}
			joined := strings.Join(cfg.Problems, "\n")
			if !strings.Contains(joined, EnvRepoAliases) {
				t.Errorf("refusal did not name %s; problems: %s", EnvRepoAliases, joined)
			}
		})
	}
}

// TestRepoAliasEchoed is P3 for the grouping surface (brief-05 Verify row 3): the
// effective alias set appears in the run echo, so a settings-only regrouping is visible.
func TestRepoAliasEchoed(t *testing.T) {
	r := goldenRoster()
	r[EnvRepoAliases] = "example-k8s=ledger:ledger,proposals=props:demo"
	withRoster(t, r)

	var b bytes.Buffer
	EchoEffectiveConfig(&b)
	out := b.String()
	if !strings.Contains(out, "assay-config: "+EnvRepoAliases+"=") {
		t.Fatalf("the run echo carries no %s line:\n%s", EnvRepoAliases, out)
	}
	for _, want := range []string{"example-k8s=ledger:ledger", "proposals=props:demo"} {
		if !strings.Contains(out, want) {
			t.Errorf("the %s echo omits %q:\n%s", EnvRepoAliases, want, out)
		}
	}
}
