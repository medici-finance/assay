package deskkit

// umbrellainfer.go — adopting an umbrella version from VERIFIED per-artifact pins.
//
// A pin file can carry per-artifact lines and no `assay <vX.Y.Z>` umbrella line. A
// stock install writes exactly that file. The version marker reads such a file as
// could-not-determine ("no umbrella pin"), and before this reader nothing wrote the
// line, so `upgrade-assay` had no supported way through.
//
// The evidence to name the umbrella is already in the file. Under the bare `vX.Y.Z`
// scheme every asset on umbrella release T was built at T, and each per-artifact line
// pins the sha256 of one published asset. When every eligible line names the same
// bare tag T AND every pinned digest equals the digest T's checksums.txt publishes for
// that asset, the file is a pin of release T, not a guess about it.
// InferUmbrellaFromPins performs that check and refuses whenever any part of it fails.
//
// It is an EXPLICIT act. The version marker never calls it: ReadMarkerFrom keeps its
// three states, and `deskversion` still reports could-not-determine for this file.
// Only a caller that opted in (`upgrade-assay --adopt-umbrella-from-pins`) reaches
// it, and that caller prints the evidence it returns.
//
// ELIGIBLE LINES. Every non-comment line except:
//   - the umbrella line itself. If one is present this reader refuses: inference
//     applies only when the line is absent;
//   - a `<artifact>-source` line (SourcePinSuffix), which pins a commit rather than a
//     release asset, so no published digest can confirm it;
//   - a line whose trailing comment carries the `same-tag: exempt` marker
//     (docs/distribution.md § One tag, one tree), which is declared to be on a
//     different cadence than the umbrella.
//
// THE DIGEST RULE matches upgrade-assay's re-pin. A platform line
// (`statusgen-darwin-arm64`) compares with that exact asset. A bare `<component>`
// line is the CI-facing pin, so it compares with `<component>-linux-amd64`
// (CIPinPlatform), selected by name.
//
// REFUSALS, each naming its own reason:
//   - exit 6 (could-not-determine): an unreadable file, an umbrella line already
//     present, a malformed eligible line, a legacy `<component>/vX.Y.Z` tag, mixed
//     tags, zero eligible lines, a composition that cannot be loaded, a composition
//     with no per-asset digests (a hand-authored manifest), or an eligible line
//     whose asset T's checksums.txt does not list;
//   - exit 5 (known-inconsistent): any pinned digest that differs from the published
//     one. The refusal names the line, the pinned digest and the published digest.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CIPinPlatform is the platform a BARE pin line (`statusgen <tag> <sha256>`, no
// platform suffix) is the digest of. The bare line is the CI-facing pin, and CI
// verifies it against the linux-amd64 asset.
const CIPinPlatform = "linux-amd64"

// SameTagExemptMarker is the token a pin line's trailing comment carries to opt that
// line out of the one-tag-one-tree grouping (docs/distribution.md). statusgen's
// same-tag lint reads the same token.
const SameTagExemptMarker = "same-tag: exempt"

// bareUmbrellaTag is a strict bare vMAJOR.MINOR.PATCH with no `<component>/` prefix.
var bareUmbrellaTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// UmbrellaInference is what InferUmbrellaFromPins proved.
type UmbrellaInference struct {
	// Umbrella is the inferred bare umbrella tag T.
	Umbrella string
	// Source names where T's published digests came from: a local checksums file or
	// the release-home URL.
	Source string
	// Verified has one line per eligible pin line, recording which published asset it
	// matched.
	Verified []string
	// Skipped names each line left out of the inference and the reason.
	Skipped []string
	// Composition is T's composition as loaded, so the caller need not load (or
	// fetch) it a second time.
	Composition Composition
}

// Evidence renders the inference for a human: the headline count and source, then
// each verified and skipped line.
func (u UmbrellaInference) Evidence() string {
	var b strings.Builder
	fmt.Fprintf(&b, "umbrella %s inferred from the pin file: %d pin(s) verified against %s\n",
		u.Umbrella, len(u.Verified), u.Source)
	for _, v := range u.Verified {
		fmt.Fprintf(&b, "  verified: %s\n", v)
	}
	for _, s := range u.Skipped {
		fmt.Fprintf(&b, "  skipped:  %s\n", s)
	}
	return b.String()
}

// pinLineSameTagExempt reports whether a trailing comment carries the exemption marker.
func pinLineSameTagExempt(comment string) bool {
	return strings.Contains(comment, SameTagExemptMarker)
}

// InferUmbrellaFromPins infers the umbrella tag the pin file at root is a pin of, and
// verifies every eligible line's digest against that release's published checksums.
// src supplies the composition (CompositionSource.Load). See the file comment for the
// eligible lines, the digest rule and the refusals.
func InferUmbrellaFromPins(root string, src CompositionSource) (UmbrellaInference, error) {
	path := filepath.Join(root, AssayVersionsFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		return UmbrellaInference{}, Unverifiable("cannot infer an umbrella: cannot read "+path, err)
	}

	type pinLine struct {
		lineNo         int
		name, tag, sha string
	}
	var eligible []pinLine
	var skipped []string
	byTag := map[string][]string{}
	var tagOrder []string
	for i, rawLine := range strings.Split(string(raw), "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		comment := ""
		if j := strings.IndexByte(trimmed, '#'); j >= 0 {
			comment = trimmed[j+1:]
		}
		payload := stripComment(trimmed)
		if payload == "" {
			continue
		}
		fields := strings.Fields(payload)
		name := fields[0]
		switch {
		case name == UmbrellaArtifact:
			return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
				"cannot infer an umbrella: %s line %d already carries an umbrella line (%q); inference applies only when that line is absent",
				path, lineNo, payload), nil)
		case strings.HasSuffix(name, SourcePinSuffix):
			skipped = append(skipped, fmt.Sprintf("line %d %s (a source pin names a commit, not a release asset)", lineNo, name))
			continue
		case pinLineSameTagExempt(comment):
			skipped = append(skipped, fmt.Sprintf("line %d %s (marked %q)", lineNo, name, SameTagExemptMarker))
			continue
		}
		if len(fields) != 3 {
			return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
				"cannot infer an umbrella: %s line %d is not `<artifact> <tag> <sha256>`: %q", path, lineNo, payload), nil)
		}
		tag, sha := fields[1], fields[2]
		if !bareUmbrellaTag.MatchString(tag) {
			if artifactTagPattern.MatchString(tag) {
				return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
					"cannot infer an umbrella: %s line %d pins %s at the legacy per-artifact tag %q; an umbrella is inferred only from bare vX.Y.Z tags",
					path, lineNo, name, tag), nil)
			}
			return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
				"cannot infer an umbrella: %s line %d pins %s at %q, which is not a vX.Y.Z tag", path, lineNo, name, tag), nil)
		}
		if !sha256Pattern.MatchString(sha) {
			return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
				"cannot infer an umbrella: %s line %d pins %s with %q, which is not a 64-hex sha256", path, lineNo, name, sha), nil)
		}
		eligible = append(eligible, pinLine{lineNo: lineNo, name: name, tag: tag, sha: sha})
		if _, seen := byTag[tag]; !seen {
			tagOrder = append(tagOrder, tag)
		}
		byTag[tag] = append(byTag[tag], name)
	}

	if len(eligible) == 0 {
		why := "the file has no per-artifact lines"
		if len(skipped) > 0 {
			why = "every per-artifact line is excluded: " + strings.Join(skipped, "; ")
		}
		return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
			"cannot infer an umbrella: %s has no eligible per-artifact pin line (%s)", path, why), nil)
	}
	if len(tagOrder) > 1 {
		var parts []string
		for _, t := range tagOrder {
			parts = append(parts, fmt.Sprintf("%s (%s)", t, strings.Join(byTag[t], ", ")))
		}
		return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
			"cannot infer an umbrella: the per-artifact lines in %s carry mixed tags: %s; one umbrella needs one tag",
			path, strings.Join(parts, "; ")), nil)
	}
	umbrella := tagOrder[0]

	comp, found, err := src.Load(umbrella)
	if err != nil {
		return UmbrellaInference{}, Unverifiable("cannot infer umbrella "+umbrella+": its composition could not be loaded", err)
	}
	if !found {
		return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
			"cannot infer umbrella %s: no composition for it was found (looked in: %s)", umbrella, src.Describe(umbrella)), nil)
	}
	if len(comp.AssetSHA256) == 0 {
		return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
			"cannot infer umbrella %s: its composition is a hand-authored manifest (%s) that carries no per-asset digests; inference needs the release's checksums.txt",
			umbrella, filepath.Join(src.ReleasesDir, ManifestFileName(umbrella))), nil)
	}
	source := comp.Origin
	if source == "" {
		source = "the " + umbrella + " composition"
	}

	var verified, mismatches, unlisted []string
	for _, l := range eligible {
		component := ComponentOf(l.name)
		asset := l.name
		if l.name == component {
			asset = component + "-" + CIPinPlatform
		}
		published, listed := comp.AssetSHA256[asset]
		switch {
		case !listed:
			unlisted = append(unlisted, fmt.Sprintf("line %d %s (needs asset %s)", l.lineNo, l.name, asset))
		case published != l.sha:
			mismatches = append(mismatches, fmt.Sprintf(
				"line %d %s %s: pinned sha256 %s, but %s publishes %s for %s",
				l.lineNo, l.name, l.tag, l.sha, source, published, asset))
		default:
			verified = append(verified, fmt.Sprintf("line %d %s %s matches %s", l.lineNo, l.name, l.tag, asset))
		}
	}
	if len(mismatches) > 0 {
		sort.Strings(mismatches)
		return UmbrellaInference{}, Refused(fmt.Sprintf(
			"known-inconsistent: %d pin line(s) in %s disagree with the published %s digests: %s",
			len(mismatches), path, umbrella, strings.Join(mismatches, "; ")))
	}
	if len(unlisted) > 0 {
		return UmbrellaInference{}, Unverifiable(fmt.Sprintf(
			"cannot infer umbrella %s: %s lists no asset for %s, so those pins cannot be verified",
			umbrella, source, strings.Join(unlisted, "; ")), nil)
	}
	return UmbrellaInference{
		Umbrella:    umbrella,
		Source:      source,
		Verified:    verified,
		Skipped:     skipped,
		Composition: comp,
	}, nil
}
