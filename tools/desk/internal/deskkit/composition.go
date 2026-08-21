package deskkit

// composition.go — a shared reader for the umbrella composition manifest
// (`releases/<umbrella>.yaml`), the file that answers "which per-artifact tags
// make up umbrella version X". Its format is defined in
// `docs/streams/distribution/version-scheme.md § "Composition manifest"`.
//
// This is a SECOND, deliberately more permissive reader than
// `deskrelease/manifest.go`. That one screens what tags `deskrelease` may CUT and
// stays `<component>/vX.Y.Z`-only; this one is read by the adopter marker
// (`deskversion`) over the tags a release ACTUALLY shipped, and the public
// release home cuts a plain `vX.Y.Z` umbrella tag (Ian's 2026-08-15 ruling — see
// `docs/distribution.md § The umbrella line`), never `assay/vX.Y.Z`. So it
// accepts both tag shapes, exactly as `pins.go`'s `artifactTagPattern` does, for
// the same reason: what a consumer may READ is a superset of what this repo may
// CUT. This mirrors, and does not replace, the deskrelease reader.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReleasesDir is the conventional directory name, under a marker root, that holds
// one composition manifest per umbrella version. A consumer that has fetched the
// manifests from the release home materialises them here; the marker fixtures ship
// them here directly so the reader is exercisable offline.
const ReleasesDir = "releases"

// CompositionArtifact is one entry of a composition manifest's `artifacts:` list.
//
//   - Tag is the per-artifact tag the umbrella release names — a plain `vX.Y.Z`
//     (the shipped shape) or a legacy `<component>/vX.Y.Z`. Both parse.
//   - Artifact is the component name. Under a plain `vX.Y.Z` tag the tag no longer
//     carries the component, so the name is stated explicitly. It is the
//     additional per-entry field version-scheme.md's Composition-manifest format
//     leaves "open for the brief that first needs one" — this is that brief. When
//     Artifact is empty and Tag is namespaced, ArtifactName() recovers the
//     component from the tag so an older, namespaced-only manifest still reads.
type CompositionArtifact struct {
	Artifact string `yaml:"artifact"`
	Tag      string `yaml:"tag"`
}

// ArtifactName resolves the component name for an entry: the explicit `artifact:`
// field when set, else the `<component>` prefix of a namespaced tag, else "".
func (a CompositionArtifact) ArtifactName() string {
	if a.Artifact != "" {
		return a.Artifact
	}
	if i := strings.IndexByte(a.Tag, '/'); i > 0 {
		return a.Tag[:i]
	}
	return ""
}

// Composition is one umbrella version's composition manifest: the umbrella tag it
// is for, and the per-artifact tags that make it up (authority rule 1: an umbrella
// release names exactly one tag per artifact line).
type Composition struct {
	Umbrella  string                `yaml:"umbrella"`
	Artifacts []CompositionArtifact `yaml:"artifacts"`
}

// TagFor returns the tag this composition names for artifact, and whether it names
// one at all. A component the manifest does not list is (‑, false) — the marker
// treats that as provenance-only, never as a match.
func (c Composition) TagFor(artifact string) (string, bool) {
	for _, a := range c.Artifacts {
		if a.ArtifactName() == artifact {
			return a.Tag, true
		}
	}
	return "", false
}

// LoadComposition reads and parses the composition manifest for umbrellaTag from
// releasesDir. The filename is the umbrella tag with `/` replaced by `-` (a `/`
// cannot be a single path segment), matching version-scheme.md's rule; a plain
// `vX.Y.Z` tag has no `/` and maps to `<tag>.yaml` unchanged.
//
// Fail-closed, like the pin reader: a missing or unreadable manifest, or one that
// does not parse, is Unverifiable (exit 6) — the marker must never assemble a
// "known" answer from a manifest it could not read.
func LoadComposition(releasesDir, umbrellaTag string) (Composition, error) {
	name := strings.ReplaceAll(umbrellaTag, "/", "-") + ".yaml"
	path := filepath.Join(releasesDir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Composition{}, Unverifiable(
			"cannot read composition manifest "+path+" for umbrella "+umbrellaTag, err)
	}
	var c Composition
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return Composition{}, Unverifiable(
			fmt.Sprintf("cannot parse composition manifest %s: %v", path, err), err)
	}
	if c.Umbrella == "" {
		return Composition{}, Unverifiable(
			"composition manifest "+path+" has no umbrella: field", nil)
	}
	return c, nil
}
