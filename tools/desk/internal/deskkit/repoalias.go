package deskkit

import "strings"

// Repo taxonomy: the ONE shared computation behind the boards' short-name labels
// and their per-repo → product grouping. It lives HERE in deskkit so deskboard and
// issueboard share a SINGLE resolver rather than each carrying its own copy of a
// suffix/basename switch that could — and did — drift (deskboard grew an
// example-k8s/example-reconciler/proposals label set that issueboard's private copy
// never had).
//
// GENERIC compiled default, house policy in config. The compiled default names no
// deployment's repos:
//
//	short   = the repo's last path segment (basename)
//	product = the repo's owner
//
// The house short names and product buckets that used to be hardwired into the
// boards' switches ("ledger", "recon", "examples", an org-wide catch-all, …) are
// NOT compiled here — they are adopter configuration supplied through
// ASSAY_REPO_ALIASES (rosterconfig.go), read from the same roster the trust and
// write-authorisation surfaces already come from. An adopter that ships the boards
// layers its real grouping ON TOP of the generic default through that variable, so
// no one deployment's taxonomy travels with the tool.
//
// UNSET is not an error and not a refusal: the generic default is a complete
// configuration on its own, exactly as the compiled risk-path base is (riskpath.go).
// A MALFORMED value, by contrast, refuses the whole roster (parseConfig) — grouping
// is display-only, but a configured-but-broken value that silently fell back to the
// generic derivation would be the configured-but-wrong shape the roster design
// refuses everywhere else.
//
// DISPLAY ONLY. Nothing here gates a flip, a write, a bless or a queue admission —
// the label a board prints and the product a repo is merged under are cosmetic, and
// the board re-sorts to a total order after the product merge so grouping never
// changes the board's bytes. This is why the config is a plain override map rather
// than the additive-only union the risk gate requires: there is no safety direction
// to protect, only a human-readable label to get right.

// RepoShortLabel returns the short board label for a repo: the configured
// ASSAY_REPO_ALIASES short name if one is set for this repo (matched by full slug or
// basename), otherwise the generic default — the repo's last path segment.
//
// The label must be INJECTIVE over a board's census (two repos rendering the same
// string leaves a human unable to tell which repo a row is about); collisions under
// the generic default are the alias config's job to resolve, and
// TestShortRepoLabelsAreInjective pins the property over the configured set.
func RepoShortLabel(repo string) string {
	if a, ok := lookupRepoAlias(repo); ok && a.Short != "" {
		return a.Short
	}
	return repoBasename(repo)
}

// RepoProductGroup returns the product group a repo is merged under on the boards:
// the configured ASSAY_REPO_ALIASES product if one is set for this repo (matched by
// full slug or basename), otherwise the generic default — the repo's owner.
func RepoProductGroup(repo string) string {
	if a, ok := lookupRepoAlias(repo); ok && a.Product != "" {
		return a.Product
	}
	return repoOwner(repo)
}

// lookupRepoAlias resolves a repo to its configured alias, trying the full
// `owner/name` slug FIRST (so an adopter can disambiguate one owner's repo from a
// same-named repo under another owner) and then the bare basename (the compact form
// that matches every mirror of a product across owners, as the old basename switch
// did).
func lookupRepoAlias(repo string) (RepoAlias, bool) {
	m := EffectiveConfig().RepoAliases
	if len(m) == 0 {
		return RepoAlias{}, false
	}
	if a, ok := m[repo]; ok {
		return a, true
	}
	if base := repoBasename(repo); base != repo {
		if a, ok := m[base]; ok {
			return a, true
		}
	}
	return RepoAlias{}, false
}

// repoBasename is the repo's last '/'-separated segment (the generic short label).
func repoBasename(repo string) string {
	if i := strings.LastIndex(repo, "/"); i >= 0 {
		return repo[i+1:]
	}
	return repo
}

// repoOwner is the part of a repo slug before the first '/' (the generic product
// group). A slug with no '/' is its own owner.
func repoOwner(repo string) string {
	if i := strings.IndexByte(repo, '/'); i >= 0 {
		return repo[:i]
	}
	return repo
}
