package main

// Composition-manifest reader for the umbrella version scheme (see version-scheme.md
// § "Composition manifest" for the format this pins): one YAML file per umbrella
// version, a top-level `umbrella:` tag and an `artifacts:` list whose entries each
// carry the full per-artifact `tag:`.
//
// This package does not cut any tag from a manifest — it does not cut releases — and it
// is not wired into `planCut`. It exists so the format is machine-readable by a second
// party (the fixture in releases/example.yaml, and future consumer lines) and so the two
// negative properties required here have somewhere to live: the umbrella's own line never
// regresses across releases, and a per-artifact tag is validated independently of whether
// any umbrella tag exists.

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// artifactTagPattern is the SHAPE check for one manifest entry's tag: a component name
// (lowercase, digits, hyphens — never empty, never starting with a digit or hyphen) followed
// by "/vX.Y.Z". It is deliberately component-agnostic, not a 3-way allow-list: the
// artifact-line set is left open on purpose, since a future plugin line and a future
// container-image line each add one, and this check must accept any well-formed line
// they add, not just today's three.
//
// The numeric fields are `(0|[1-9][0-9]*)`, NOT `[0-9]+`, because version-scheme.md's "Tag
// grammar" states the rule normatively — "three non-negative integers, no leading zeros" —
// and names `tools/releaseguard/version.go`'s `versionRE` as the canonical parse. A `[0-9]+`
// field contradicted both. The concrete harm is not pedantry: `[0-9]+` accepted
// `statusgen/v0.8.03` and mapped it to the SAME [0 8 3] tuple as `statusgen/v0.8.3`, so two
// distinct git tags collapsed to one version, and checkUmbrellaMonotonic below — the one
// ordering check this tool ships — would then read a real bump as "does not sort above" (or
// a duplicate as a bump). Rejecting the shape at the door is what keeps the tuple a faithful
// stand-in for the tag.
//
// This is deliberately a STRICT SUBSET of releaseguard's versionRE, which additionally
// accepts an optional `-<pre-release>` suffix. A composition manifest names RELEASED
// artifact lines (authority rule 2: a version never named by an umbrella release is
// unsupported), and ordering pre-releases would require precedence rules this scheme does
// not define — so a pre-release tag is refused here rather than mis-ordered. Widening to
// accept one is a scheme change, not a regex change: it needs the precedence rule written
// into version-scheme.md first.
var artifactTagPattern = regexp.MustCompile(`^([a-z][a-z0-9-]*)/v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Artifact is one entry of a composition manifest's `artifacts:` list — the full
// per-artifact tag an umbrella release names for that line, e.g. "statusgen/v0.8.2".
type Artifact struct {
	Tag string `yaml:"tag"`
}

// Manifest is the composition-manifest format: one file per umbrella version, naming
// exactly one tag per artifact line (authority rule 1 in version-scheme.md).
type Manifest struct {
	Umbrella  string     `yaml:"umbrella"`
	Artifacts []Artifact `yaml:"artifacts"`
}

// loadManifest reads and parses a composition manifest from disk. It performs no shape
// validation beyond what YAML decoding itself enforces; callers that need well-formed tags
// use splitArtifactTag on the entries they care about.
func loadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return m, nil
}

// splitArtifactTag splits one artifact-line tag into its component and numeric version,
// e.g. "statusgen/v0.8.2" -> ("statusgen", [0,8,2], true). It reports ok=false for anything
// that does not match the fixed "<component>/vX.Y.Z" shape version-scheme.md requires of
// every manifest entry — the same shape a plain grep checks.
func splitArtifactTag(tag string) (component string, ver [3]int, ok bool) {
	m := artifactTagPattern.FindStringSubmatch(tag)
	if m == nil {
		return "", [3]int{}, false
	}
	major, err1 := strconv.Atoi(m[2])
	minor, err2 := strconv.Atoi(m[3])
	patch, err3 := strconv.Atoi(m[4])
	if err1 != nil || err2 != nil || err3 != nil {
		return "", [3]int{}, false
	}
	return m[1], [3]int{major, minor, patch}, true
}

// versionLess reports whether a sorts strictly below b under (major, minor, patch)
// precedence — the same ordering releaseguard's version.go uses for the numeric fields.
func versionLess(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// checkUmbrellaMonotonic enforces authority rule 4 ("the umbrella is its own line") at the
// manifest level: a NEXT manifest's own umbrella version must sort strictly above the
// PREVIOUS manifest's umbrella version. This is a new check over the composition manifest
// itself — it is deskrelease-side, not tools/releaseguard, because releaseguard's existing
// per-component gatherExisting scoping is out of scope here (it already orders each
// per-ARTIFACT line correctly; nothing here duplicates that).
func checkUmbrellaMonotonic(prev, next Manifest) error {
	_, prevVer, ok := splitArtifactTag(prev.Umbrella)
	if !ok {
		return fmt.Errorf("previous manifest's umbrella %q does not match <component>/vX.Y.Z", prev.Umbrella)
	}
	// The component name is deliberately not compared — both sides are always "assay" in
	// practice, but this check is written against the version alone so it is never coupled
	// to a hardcoded umbrella component name.
	_, nextVer, ok := splitArtifactTag(next.Umbrella)
	if !ok {
		return fmt.Errorf("next manifest's umbrella %q does not match <component>/vX.Y.Z", next.Umbrella)
	}
	if !versionLess(prevVer, nextVer) {
		return fmt.Errorf(
			"refused: umbrella %s does not sort above the previous umbrella %s — "+
				"the umbrella version must sort strictly above every earlier umbrella release "+
				"(version-scheme.md § the sorts-above-its-components invariant)",
			next.Umbrella, prev.Umbrella)
	}
	return nil
}
