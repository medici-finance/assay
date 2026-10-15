package main

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// currencyGrace is how long a newer published release may go un-pinned before
// assertion (d) fails. A re-pin needs the release's checksums.txt AND the
// harness image digest from the registry, and the image is published by a
// separate, human-dispatched run, so a pin cannot be current the minute a
// release lands. Inside the window the lag is reported as a note; past it the
// run is red. The window is measured from the FIRST release the pin missed, not
// the newest, so a steady stream of releases can never keep restarting it.
const currencyGrace = 7 * 24 * time.Hour

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

// releaseMeta is one entry of a release home's release list: enough to order
// it against the pinned tag and to say how long the pin has lagged it.
type releaseMeta struct {
	Tag         string
	Draft       bool
	Prerelease  bool
	PublishedAt time.Time
}

// releaseSource resolves one published release of a release home, and lists
// every release of a release home.
type releaseSource interface {
	Release(home, tag string) (*release, error)
	Releases(home string) ([]releaseMeta, error)
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
	// lists keeps one release-list read per release home, for the same reason.
	lists    map[string][]releaseMeta
	listErrs map[string]error
	// now is the clock assertion (d) measures a pin's lag against.
	now time.Time
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
func Check(root string, src releaseSource, now time.Time, out io.Writer) bool {
	c := &checker{
		src: src, now: now,
		cache: map[string]*release{}, errs: map[string]error{},
		lists: map[string][]releaseMeta{}, listErrs: map[string]error{},
	}

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
	fmt.Fprintf(out, "pairedversions: OK — plugin %s; statusgen %s @ %s; desk-tools %s @ %s; every pinned sha256 matches the published release's checksums.txt, and no newer release has gone un-pinned past the grace window.\n",
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

// checkSection runs assertions (b), (c) and (d) over one half of the manifest:
//
//	(b) the paired tag is a PUBLISHED release of the named release home;
//	(c) every per-platform sha256 equals that release's checksums.txt entry; and
//	(d) no newer published release of that home has gone un-pinned for longer
//	    than currencyGrace.
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

	c.checkCurrency(name, s)

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

// checkCurrency is assertion (d): the paired tag has not been left behind by a
// newer published release for longer than currencyGrace.
//
// (a)-(c) prove the manifest is internally consistent and that what it pins
// exists. None of them notices a pin that is consistent and real but STALE: a
// release workflow that bumps `plugin:` at every cut while the tags stay put
// passes all three, and every new adopter installs the old tools. This is the
// assertion that reddens on that state. Drafts and pre-releases are not
// releases an adopter should be moved to, and a tag that is not a plain
// vX.Y.Z (a per-artifact tag) is not an umbrella release, so both are ignored.
func (c *checker) checkCurrency(name string, s section) {
	pinned, ok := parseTag(s.Tag)
	if !ok {
		c.failf("could-not-check: %s: section `%s` pins tag %q, which is not a plain vX.Y.Z tag, so it cannot be ordered against the releases of %s",
			manifestPath, name, s.Tag, s.ReleaseHome)
		return
	}
	list, err := c.releases(s.ReleaseHome)
	if err != nil {
		c.failf("could-not-check: %s: section `%s`: could not list the releases of %s: %v — whether %s is still the latest release is unknown, so this run is not green",
			manifestPath, name, s.ReleaseHome, err, s.Tag)
		return
	}

	var newest, firstMissed *releaseMeta
	var newestV [3]int
	sawPinned, missed := false, 0
	for i := range list {
		r := &list[i]
		if r.Tag == s.Tag {
			sawPinned = true
		}
		if r.Draft || r.Prerelease {
			continue
		}
		v, ok := parseTag(r.Tag)
		if !ok || !tagLess(pinned, v) {
			continue
		}
		missed++
		if newest == nil || tagLess(newestV, v) {
			newest, newestV = r, v
		}
		if firstMissed == nil || r.PublishedAt.Before(firstMissed.PublishedAt) {
			firstMissed = r
		}
	}
	if !sawPinned {
		// (b) just found the pinned release, so a list without it is not the
		// complete list, and "nothing newer" read off it would be a guess.
		c.failf("could-not-check: %s: section `%s`: the release list of %s does not include the pinned %s, so it is not a complete list and cannot show whether a newer release exists",
			manifestPath, name, s.ReleaseHome, s.Tag)
		return
	}
	if newest == nil {
		return
	}
	if firstMissed.PublishedAt.IsZero() {
		c.failf("could-not-check: %s: section `%s`: release %s of %s, newer than the pinned %s, carries no publish time, so how long the pin has lagged it is unknown",
			manifestPath, name, firstMissed.Tag, s.ReleaseHome, s.Tag)
		return
	}
	lag := c.now.Sub(firstMissed.PublishedAt)
	if lag <= currencyGrace {
		c.notef("section `%s` pins %s; %s is the latest published release of %s (%d newer). The pin has lagged since %s was published %s, %s ago, inside the %s grace window. Re-pin before %s, when this becomes a failure.",
			name, s.Tag, newest.Tag, s.ReleaseHome, missed, firstMissed.Tag, firstMissed.PublishedAt.UTC().Format(time.RFC3339), days(lag), days(currencyGrace), firstMissed.PublishedAt.Add(currencyGrace).UTC().Format(time.RFC3339))
		return
	}
	c.failf("%s: section `%s` pins %s, but %s is the latest published release of %s (%d newer). The pin has lagged since %s was published %s, %s ago, past the %s grace window, so `assay:install` hands every new adopter %s. Re-pin: set every tag in %s to %s, take each per-platform sha256 from that release's checksums.txt and the harness digest from the registry, then run `make paired-versions` and `bash plugins/assay/scripts/check-paired-versions.sh`.",
		manifestPath, name, s.Tag, newest.Tag, s.ReleaseHome, missed, firstMissed.Tag, firstMissed.PublishedAt.UTC().Format(time.RFC3339), days(lag), days(currencyGrace), s.Tag, manifestPath, newest.Tag)
}

// parseTag reads a plain umbrella tag `vX.Y.Z`. Anything else — a
// component-prefixed per-artifact tag, a pre-release suffix — is not one.
func parseTag(tag string) ([3]int, bool) {
	var v [3]int
	rest, ok := strings.CutPrefix(tag, "v")
	if !ok || !semverRe.MatchString(rest) {
		return v, false
	}
	for i, part := range strings.SplitN(rest, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// tagLess orders two parsed tags numerically, so v1.10.0 sorts after v1.9.0.
func tagLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func days(d time.Duration) string {
	return fmt.Sprintf("%.1f days", d.Hours()/24)
}

// releases lists (and caches) every release of one release home.
func (c *checker) releases(home string) ([]releaseMeta, error) {
	if l, ok := c.lists[home]; ok {
		return l, nil
	}
	if e, ok := c.listErrs[home]; ok {
		return nil, e
	}
	l, err := c.src.Releases(home)
	if err != nil {
		c.listErrs[home] = err
		return nil, err
	}
	c.lists[home] = l
	return l, nil
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
