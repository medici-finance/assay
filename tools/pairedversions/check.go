package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

// errNoRelease is what a releaseSource returns when the tag names NO release of
// the release home. It is distinct from a transport error on purpose: "the tag
// does not exist" is a checked-FAILED verdict, while "the release home could not
// be read" is could-not-check. Both are non-green, but they are reported as
// themselves and never collapsed into one another.
var errNoRelease = errors.New("no such release")

// release is as much of a published release as the assertions need.
type release struct {
	Tag        string
	Draft      bool
	Prerelease bool
	// Assets names every file published under the release.
	Assets []string
	// Checksums maps artifact name -> sha256, parsed from the release's own
	// checksums.txt. It is the authority the manifest header names: a locally
	// built binary lacks the release -ldflags stamp and hashes differently, so
	// nothing else may stand in for it.
	Checksums map[string]string
}

// releaseSource resolves one published release of a release home.
type releaseSource interface {
	Release(home, tag string) (*release, error)
}

// checker accumulates problems. It never short-circuits: one run reports every
// disagreement it can see, so a re-pin is a single edit rather than a sequence
// of one-failure-at-a-time CI rounds.
type checker struct {
	src      releaseSource
	problems []string
	notes    []string
	// cache keeps one fetch per (home, tag) — statusgen and desk-tools are cut
	// from the SAME release, so the common case is one network read, not two.
	cache map[string]*release
	errs  map[string]error
}

func (c *checker) failf(format string, a ...any) {
	c.problems = append(c.problems, fmt.Sprintf(format, a...))
}

func (c *checker) notef(format string, a ...any) {
	c.notes = append(c.notes, fmt.Sprintf(format, a...))
}

// Check runs every assertion against the tree rooted at root, writes a report to
// out, and returns true only when the front door is provably consistent.
//
// FAIL-CLOSED. Every non-OK outcome — a disagreement, a missing field, an
// unreadable file, a release home that could not be reached — returns false. A
// check that did not look has cleared nothing (docs/three-state-instrument-rule.md),
// so could-not-check is reported as itself and still reddens the run; it is never
// rounded up to a pass.
func Check(root string, src releaseSource, out io.Writer) bool {
	c := &checker{src: src, cache: map[string]*release{}, errs: map[string]error{}}

	pluginVer, errPlugin := readPluginVersion(root)
	if errPlugin != nil {
		c.failf("could-not-check: %v", errPlugin)
	}
	m, errManifest := readManifest(root)
	if errManifest != nil {
		c.failf("could-not-check: %v", errManifest)
	}

	if errPlugin == nil && errManifest == nil {
		c.checkPluginPairing(pluginVer, m)
		c.checkSection("statusgen", m.Statusgen)
		c.checkSection("desk-tools", m.DeskTools)
	}

	for _, n := range c.notes {
		fmt.Fprintf(out, "pairedversions: note — %s\n", n)
	}
	if len(c.problems) > 0 {
		for _, p := range c.problems {
			fmt.Fprintf(out, "pairedversions: FAIL — %s\n", p)
		}
		fmt.Fprintf(out, "pairedversions: %d problem(s). The front door is not consistent: re-pin %s (bump the tag and refresh EVERY sha256 from the published release's checksums.txt) rather than editing a value in place.\n", len(c.problems), manifestPath)
		return false
	}
	fmt.Fprintf(out, "pairedversions: OK — plugin %s; statusgen %s @ %s; desk-tools %s @ %s; every pinned sha256 matches the published release's checksums.txt.\n",
		pluginVer, m.Statusgen.Tag, m.Statusgen.ReleaseHome, m.DeskTools.Tag, m.DeskTools.ReleaseHome)
	return true
}

// checkPluginPairing is assertion (a): the plugin manifest's version and the
// pairing manifest's `plugin` field name the SAME plugin version.
//
// This is the assertion the 0.5.0 bump skipped. `assay:install` resolves the
// statusgen tag from the pairing manifest on behalf of the running plugin, so a
// pairing manifest still declaring the previous plugin version is a pairing for a
// plugin nobody is running — and the adopter silently gets that older plugin's
// tool.
func (c *checker) checkPluginPairing(pluginVer string, m *manifest) {
	if m.Plugin == "" {
		c.failf("%s carries no `plugin` field — the LEFT side of the pairing is absent, so nothing pins the manifest to a plugin version", manifestPath)
		return
	}
	if !semverRe.MatchString(pluginVer) {
		c.failf("%s version %q is not a bare X.Y.Z semver", pluginJSONPath, pluginVer)
	}
	if !semverRe.MatchString(m.Plugin) {
		c.failf("%s `plugin: %q` is not a bare X.Y.Z semver", manifestPath, m.Plugin)
	}
	if pluginVer != m.Plugin {
		c.failf("%s version (%s) != %s `plugin` (%s) — the shipped pairing is for a plugin version nobody is running, so `assay:install` resolves the WRONG statusgen tag for the skills it ships with. On a plugin bump, re-pin.",
			pluginJSONPath, pluginVer, manifestPath, m.Plugin)
	}
}

// checkSection runs assertions (b) and (c) over one half of the manifest:
//
//	(b) the paired tag is a PUBLISHED release of the named release home; and
//	(c) every per-platform sha256 equals that release's checksums.txt entry.
func (c *checker) checkSection(name string, s section) {
	if s.ReleaseHome == "" {
		c.failf("%s: section `%s` names no `release_home` — there is nothing to resolve the tag against", manifestPath, name)
		return
	}
	if s.Tag == "" {
		c.failf("%s: section `%s` carries no `tag` — a pairing with no pinned tag is not a pin", manifestPath, name)
		return
	}
	if len(s.Platforms) == 0 {
		c.failf("%s: section `%s` pins no platforms — an empty pin set verifies nothing", manifestPath, name)
		return
	}

	pins, perr := s.pins()
	for _, e := range perr {
		c.failf("%s: section `%s`: %v", manifestPath, name, e)
	}

	rel, err := c.release(s.ReleaseHome, s.Tag)
	if errors.Is(err, errNoRelease) {
		// (b) checked-FAILED: the tag resolves to nothing published.
		c.failf("%s: section `%s` pins tag %s, which is NOT a published release of %s — an adopter's `gh release download` would find nothing to install",
			manifestPath, name, s.Tag, s.ReleaseHome)
		return
	}
	if err != nil {
		// could-not-check, reported as itself and still non-green.
		c.failf("could-not-check: %s: section `%s`: could not read release %s of %s: %v — an unread release home has cleared nothing, so this run is not green",
			manifestPath, name, s.Tag, s.ReleaseHome, err)
		return
	}
	if rel.Draft {
		c.failf("%s: section `%s` pins tag %s, which is a DRAFT release of %s — a draft is not published and its assets are not downloadable by an adopter",
			manifestPath, name, s.Tag, s.ReleaseHome)
		return
	}
	if rel.Prerelease {
		// Advisory, not fatal: a prerelease IS published and downloadable. Say
		// it out loud so an accidental pre-release pin is visible in the log.
		c.notef("section `%s` pins %s, a PRE-release of %s — published and installable, but flagged so a pre-release pin is never silent", name, s.Tag, s.ReleaseHome)
	}

	assets := map[string]bool{}
	for _, a := range rel.Assets {
		assets[a] = true
	}

	for _, p := range pins {
		// The pin line repeats the tag, and `assay:install` reads the tag from
		// THAT field. A line whose tag differs from the section's would resolve
		// a different release than the one whose checksums were just verified.
		if p.Tag != s.Tag {
			c.failf("%s: section `%s`, platform %s: pin line names tag %s but the section pins %s — the installer reads the tag from the pin line, so these must not disagree",
				manifestPath, name, p.Platform, p.Tag, s.Tag)
			continue
		}
		if !assets[p.Artifact] {
			c.failf("%s: section `%s`, platform %s: release %s of %s publishes no asset named %q — the pin names an artifact that does not exist in the release it points at",
				manifestPath, name, p.Platform, s.Tag, s.ReleaseHome, p.Artifact)
			continue
		}
		want, ok := rel.Checksums[p.Artifact]
		if !ok {
			c.failf("could-not-check: %s: section `%s`, platform %s: %s has no entry in the checksums.txt of release %s — the pinned hash cannot be corroborated against the release, so it is not treated as verified",
				manifestPath, name, p.Platform, p.Artifact, s.Tag)
			continue
		}
		if want != p.SHA256 {
			// (c) checked-FAILED. This is the assertion a hand-invented or
			// locally-built hash trips.
			c.failf("%s: section `%s`, platform %s: pinned sha256 for %s is %s but release %s of %s publishes %s — the installer would REFUSE the download. Harvest hashes from the published checksums.txt, never from a local build.",
				manifestPath, name, p.Platform, p.Artifact, p.SHA256, s.Tag, s.ReleaseHome, want)
		}
	}
}

// release fetches (and caches) one release. Both a value and an error are
// cached, so a single unreachable release home is reported once per section
// without a second network round trip.
func (c *checker) release(home, tag string) (*release, error) {
	k := home + "@" + tag
	if r, ok := c.cache[k]; ok {
		return r, nil
	}
	if e, ok := c.errs[k]; ok {
		return nil, e
	}
	r, err := c.src.Release(home, tag)
	if err != nil {
		c.errs[k] = err
		return nil, err
	}
	c.cache[k] = r
	return r, nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
