package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Marketplace-catalog check (ui-craft/02, ruling #1024).
//
// The house marketplace (.claude-plugin/marketplace.json) can serve plugins
// whose source is an EXTERNAL repo pinned by commit sha — the ruled consumption
// mechanism for external skills like impeccable. That pin is provenance exactly
// like a SOURCES.yaml row, and a pin this tool cannot see is a pin that silently
// rots, so the catalog's external sources are checked here with the same
// vocabulary: in-sync / behind / moved / unreachable.
//
//	in-sync      the pinned ref (tag) still resolves to the recorded sha, and no
//	             commits have touched the pinned path since the sha's date
//	behind       ref still matches the sha, but N commits have touched the path
//	             on the default branch since — re-pin material for the cadence
//	             brief (ui-craft/04). ADVISORY: a catalog pin targets a release
//	             tag on purpose, so upstream moving past it is the expected
//	             steady state (the fork auto-syncs), not a broken pin; it is
//	             reported but never decides the drift verdict
//	moved        the ref no longer exists, or resolves to a different commit
//	             than the recorded sha — the name the pin cites no longer means
//	             what was recorded, which is worse than behind
//	unreachable  the source could not be read; an unknown, never a pass
//
// In-repo sources (a string path like "./plugins/assay") are skipped: their
// provenance story is this repo's own history plus SOURCES.yaml. External
// object sources MUST carry a 40-hex sha — an unpinned external plugin source
// is a manifest error (exit 2), per the house explicit-pin rule.

// MarketplacePin is one external plugin source lifted out of the catalog.
type MarketplacePin struct {
	Plugin string // plugin name in the catalog
	Kind   string // "github" | "git-subdir" | "url"
	Repo   string // owner/name; empty when the URL is not a readable github.com repo
	Path   string // subdirectory scope ("" = whole repo)
	Ref    string // branch or tag, optional
	SHA    string // 40-hex commit sha — required
}

type marketplaceCatalog struct {
	Plugins []marketplacePlugin `json:"plugins"`
}

type marketplacePlugin struct {
	Name   string          `json:"name"`
	Source json.RawMessage `json:"source"`
}

// externalSource is the object form of a plugin `source` field. The inner
// "source" key is the kind discriminator in the marketplace schema.
type externalSource struct {
	Kind string `json:"source"`
	Repo string `json:"repo"`
	URL  string `json:"url"`
	Path string `json:"path"`
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
}

var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

// githubSlug extracts owner/name from a github.com URL, or "" when the URL is
// not a github.com repo this tool can read through `gh`.
func githubSlug(raw string) string {
	s := strings.TrimSuffix(strings.TrimSpace(raw), ".git")
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "git@github.com:", "ssh://git@github.com/"} {
		if rest, ok := strings.CutPrefix(s, prefix); ok {
			parts := strings.Split(strings.Trim(rest, "/"), "/")
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return parts[0] + "/" + parts[1]
			}
			return ""
		}
	}
	return ""
}

// ParseMarketplace lifts the external pins out of a marketplace.json. In-repo
// string sources are skipped. A malformed catalog, or an external source with
// no 40-hex sha, is an error — never a silently unchecked entry.
func ParseMarketplace(data []byte) ([]MarketplacePin, error) {
	var cat marketplaceCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("cannot parse marketplace catalog: %w", err)
	}
	var pins []MarketplacePin
	for _, p := range cat.Plugins {
		if p.Name == "" {
			return nil, errors.New("catalog plugin with no name")
		}
		if len(p.Source) == 0 {
			return nil, fmt.Errorf("%s: plugin has no source", p.Name)
		}
		// String source = a path inside this repo; provenance is this repo.
		var asString string
		if err := json.Unmarshal(p.Source, &asString); err == nil {
			continue
		}
		var src externalSource
		if err := json.Unmarshal(p.Source, &src); err != nil {
			return nil, fmt.Errorf("%s: source is neither a path nor a source object: %w", p.Name, err)
		}
		pin := MarketplacePin{Plugin: p.Name, Kind: src.Kind, Path: src.Path, Ref: src.Ref, SHA: src.SHA}
		switch src.Kind {
		case "github":
			pin.Repo = src.Repo
			if !strings.Contains(pin.Repo, "/") {
				return nil, fmt.Errorf("%s: github source needs an owner/name repo, got %q", p.Name, src.Repo)
			}
		case "git-subdir", "url":
			pin.Repo = githubSlug(src.URL)
		default:
			return nil, fmt.Errorf("%s: unknown source kind %q (want github, git-subdir, or url)", p.Name, src.Kind)
		}
		// The sha is the pin. ref alone names a moving target (tags can be
		// re-pointed), and an external executable source with no recorded
		// commit is exactly the silent-rot shape this tool exists to refuse.
		if !shaRE.MatchString(pin.SHA) {
			return nil, fmt.Errorf("%s: external source needs a 40-hex sha pin (got %q); record the tag's commit with `git ls-remote <repo> 'refs/tags/<ref>^{}'`", p.Name, pin.SHA)
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

// CheckMarketplace reports one Result per external pin.
func CheckMarketplace(c SourceClient, pins []MarketplacePin, maxPages int) []Result {
	var out []Result
	for _, pin := range pins {
		r := checkMarketplacePin(c, pin, maxPages)
		r.File, r.Origin = "marketplace:"+pin.Plugin, "catalog"
		out = append(out, r)
	}
	return out
}

func checkMarketplacePin(c SourceClient, pin MarketplacePin, maxPages int) Result {
	if pin.Repo == "" {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: %s source is not a readable github.com repo — drift cannot be measured through gh", pin.Plugin, pin.Kind),
		}
	}

	// The ref is the human name for the pin; verify it still means the recorded
	// sha. A moved tag is a broken provenance claim even when the sha itself is
	// still fetchable.
	if pin.Ref != "" {
		tagCommit, err := c.TagCommit(pin.Repo, pin.Ref)
		switch {
		case errors.Is(err, ErrNotFound):
			return Result{
				Status:  StatusMoved,
				Commits: -1,
				Detail:  fmt.Sprintf("%s: tag %s no longer exists in %s (pin records %s)", pin.Plugin, pin.Ref, pin.Repo, short(pin.SHA)),
			}
		case err != nil:
			return Result{
				Status:  StatusUnreachable,
				Commits: -1,
				Detail:  fmt.Sprintf("%s: tag %s in %s could not be read — %v", pin.Plugin, pin.Ref, pin.Repo, err),
			}
		case tagCommit != pin.SHA:
			return Result{
				Status:  StatusMoved,
				Commits: -1,
				Detail:  fmt.Sprintf("%s: tag %s in %s now points at %s, pin records %s — re-tagged upstream, re-pin deliberately", pin.Plugin, pin.Ref, pin.Repo, short(tagCommit), short(pin.SHA)),
			}
		}
	}

	branch, err := c.DefaultBranch(pin.Repo)
	if err != nil {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: %s default branch: %v", pin.Plugin, pin.Repo, err),
		}
	}

	stamp, err := c.CommitDate(pin.Repo, pin.SHA)
	switch {
	case errors.Is(err, ErrNotFound):
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: pinned commit %s does not resolve in %s — the catalog pin may be wrong", pin.Plugin, short(pin.SHA), pin.Repo),
		}
	case err != nil:
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: pinned commit date: %v", pin.Plugin, err),
		}
	}
	since, err := afterInstant(stamp)
	if err != nil {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: pinned commit date %q: %v", pin.Plugin, stamp, err),
		}
	}

	n, capped, err := c.CommitsSince(pin.Repo, pin.Path, branch, since, maxPages)
	if err != nil {
		return Result{
			Status:  StatusUnreachable,
			Commits: -1,
			Detail:  fmt.Sprintf("%s: commit history: %v", pin.Plugin, err),
		}
	}

	scope := pin.Path
	if scope == "" {
		scope = "(whole repo)"
	}
	if n == 0 {
		return Result{
			Status:  StatusInSync,
			Commits: 0,
			Detail:  fmt.Sprintf("%s: %s@%s pinned at %s; nothing has touched %s on %s since", pin.Plugin, pin.Repo, orUnknown(pin.Ref), short(pin.SHA), scope, branch),
		}
	}
	atLeast := ""
	if capped {
		atLeast = "at least "
	}
	return Result{
		Status:   StatusBehind,
		Commits:  n,
		Advisory: true,
		Detail: fmt.Sprintf("%s: %s%d commit(s) have touched %s in %s on %s since the pin %s — re-pin material for the cadence runbook",
			pin.Plugin, atLeast, n, scope, pin.Repo, branch, short(pin.SHA)),
	}
}
