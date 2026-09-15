package deskkit

// versionmarker.go — the adopter version marker. It answers the one question
// `upgrade-assay` must ask before it can move an adopter anywhere: "what umbrella
// version is this adopter on, and which artifact versions is that made of." The
// answer is ASSEMBLED from records that already exist — the `.assay-versions` pin
// file (umbrella line + per-artifact lines) cross-checked against the umbrella's
// composition manifest — never invented as a fourth source of truth.
//
// THREE STATES, NEVER TWO (docs/three-state-instrument-rule.md). The marker maps
// each to a DISTINCT exit code so a caller can branch on the process result alone:
//
//   - MarkerKnown            → exit 0  — one umbrella version, and every artifact
//                                        the composition names is pinned to the tag
//                                        it names. A single, trustworthy answer.
//   - MarkerInconsistent     → exit 5  — records DISAGREE: an artifact is pinned to
//                                        a tag the umbrella's composition does not
//                                        name. The report names WHICH records
//                                        disagree and how — "inconsistent" without
//                                        the pair is unactionable.
//   - MarkerCouldNotDetermine→ exit 6  — the umbrella version cannot be positively
//                                        determined: no pin file, an unreadable one,
//                                        no umbrella line ("no umbrella pin" — a
//                                        valid, expected state, not an error), or a
//                                        composition manifest that could not be read.
//                                        A missing record is NEVER "assume latest":
//                                        that is exactly how a migration runs against
//                                        the wrong baseline.
//
// The plain `vX.Y.Z` tag shape is the shipped reality: the public release home cuts
// a plain `vX.Y.Z` umbrella tag, never `assay/vX.Y.Z`. The marker reads it through
// pins.go / composition.go, both of which accept it.

import (
	"fmt"
	"sort"
	"strings"
)

// MarkerState is the marker's three-state result.
type MarkerState int

const (
	// MarkerKnown — one umbrella version, consistent with the composition.
	MarkerKnown MarkerState = iota
	// MarkerInconsistent — records disagree; Disagreements names the pairs.
	MarkerInconsistent
	// MarkerCouldNotDetermine — the umbrella version could not be positively read.
	MarkerCouldNotDetermine
)

func (s MarkerState) String() string {
	switch s {
	case MarkerKnown:
		return "known"
	case MarkerInconsistent:
		return "known-inconsistent"
	case MarkerCouldNotDetermine:
		return "could-not-determine"
	default:
		return "unknown"
	}
}

// ExitCode returns the process exit code for this state — one distinct code each,
// reusing the deskkit contract (exitcodes.go): 0 ok, 5 refused (a determinate
// disagreement), 6 unverifiable (could not positively determine).
func (s MarkerState) ExitCode() int {
	switch s {
	case MarkerKnown:
		return ExitOK
	case MarkerInconsistent:
		return ExitRefused
	default:
		return ExitUnverifiable
	}
}

// MarkerArtifactVersion is one pinned artifact line the marker resolved, recorded
// as provenance under the umbrella answer.
type MarkerArtifactVersion struct {
	Artifact string
	Tag      string
}

// Marker is the assembled answer. State is the headline; the other fields are the
// provenance a caller (or a human reading the report) needs to act on it.
type Marker struct {
	State    MarkerState
	Umbrella string // the umbrella version, when one was read
	// Artifacts are the composition's artifact lines resolved against the pin
	// file — the "which artifact versions is that made of" half.
	Artifacts []MarkerArtifactVersion
	// Disagreements names each record pair that disagrees, on MarkerInconsistent.
	Disagreements []string
	// NotPinned lists components a DERIVED composition names that this repo pins
	// under no line of any shape — the release ships them, the adopter did not
	// install them. Provenance, never a disagreement (see ReadMarkerFrom).
	NotPinned []string
	// Origin records where a derived composition came from, when it was derived.
	Origin string
	// Reason is a one-line human summary, always set.
	Reason string
}

// ReadMarker assembles the marker for the consumer repo at root, reading its
// composition manifests from releasesDir (conventionally <root>/releases). It is
// pure and OFFLINE: it reads only local files — a hand-authored manifest or a
// materialised checksums.txt under releasesDir — and never the platform install
// cache or the network. A caller that may consult the release home for a
// composition that is not materialised passes a CompositionSource with a Fetch to
// ReadMarkerFrom instead.
func ReadMarker(root, releasesDir string) Marker {
	return ReadMarkerFrom(root, CompositionSource{ReleasesDir: releasesDir})
}

// ReadMarkerFrom is ReadMarker with the composition's provenance made explicit:
// src decides where the umbrella's composition comes from (compositionsource.go —
// hand-authored manifest first, then a materialised checksums.txt, then the
// release home when src carries a Fetch). The marker's three states are unchanged.
//
// HOW A PIN IS MATCHED AGAINST THE COMPOSITION. Each component the composition
// names is resolved through ComponentPinTag: the bare `<component>` line when
// present, else the `<component>-<os>-<arch>` platform lines (which must agree).
// An adopter pinned per platform, as docs/adopting-assay.md writes the file, is
// therefore read at the tag it is on, rather than reported as "no readable line".
//
// AN UN-PINNED COMPONENT is a disagreement under a HAND-AUTHORED manifest (its
// author named that artifact deliberately; the adopter is not on the composition
// the umbrella claims) but provenance-only under a DERIVED composition, which
// names EVERY component the release ships — an adopter who never installed the
// quality report pack is still on umbrella T. Derived + nothing pinned at all is
// still inconsistent: an umbrella line no artifact line backs is a disagreement,
// not a "known" answer resting on nothing.
func ReadMarkerFrom(root string, src CompositionSource) Marker {
	// 1. The umbrella line. Absent-but-valid is the "no umbrella pin" state, an
	//    unreadable file is fail-closed — both are could-not-determine, with
	//    distinct wording so "no file" and "no umbrella line" stay separable.
	umbrella, present, err := UmbrellaPin(root)
	if err != nil {
		return Marker{
			State:  MarkerCouldNotDetermine,
			Reason: "could-not-determine: " + err.Error(),
		}
	}
	if !present {
		return Marker{
			State: MarkerCouldNotDetermine,
			Reason: "no umbrella pin: the pin file carries no `assay` umbrella line " +
				"(a valid, expected state — the per-artifact lines stay authoritative; " +
				"the consumer is simply not recorded against a suite version)",
		}
	}

	// 2. The composition for that umbrella version. Fail-closed if it cannot be
	//    read or found — a "known" answer must never rest on a composition we
	//    could not open.
	comp, found, err := src.Load(umbrella)
	if err != nil {
		return Marker{
			State:    MarkerCouldNotDetermine,
			Umbrella: umbrella,
			Reason:   "could-not-determine: " + err.Error(),
		}
	}
	if !found {
		return Marker{
			State:    MarkerCouldNotDetermine,
			Umbrella: umbrella,
			Reason: "could-not-determine: no composition for umbrella " + umbrella +
				" — no manifest, no materialised checksums, and the release home does not " +
				"publish it (looked in: " + src.Describe(umbrella) + ")",
		}
	}

	// 3. Cross-check every artifact the composition names against the pin file.
	//    A pinned tag that differs from the composition's tag is the disagreement
	//    the inconsistent state exists to surface.
	var resolved []MarkerArtifactVersion
	var disagreements, notPinned []string
	for _, a := range comp.Artifacts {
		name := a.ArtifactName()
		if name == "" {
			disagreements = append(disagreements,
				fmt.Sprintf("composition entry %q names no artifact and no namespaced tag", a.Tag))
			continue
		}
		pinTag, pinned, perr := ComponentPinTag(root, name)
		if perr != nil {
			disagreements = append(disagreements, fmt.Sprintf(
				"umbrella %s names %s %s, but the pin file has no readable %s line (%v)",
				umbrella, name, a.Tag, name, perr))
			continue
		}
		if !pinned {
			if comp.Derived {
				notPinned = append(notPinned, name)
				continue
			}
			disagreements = append(disagreements, fmt.Sprintf(
				"umbrella %s names %s %s, but the pin file has no %s line of any shape",
				umbrella, name, a.Tag, name))
			continue
		}
		resolved = append(resolved, MarkerArtifactVersion{Artifact: name, Tag: pinTag})
		if pinTag != a.Tag {
			disagreements = append(disagreements, fmt.Sprintf(
				"%s pin is %s but umbrella %s composition names %s %s",
				name, pinTag, umbrella, name, a.Tag))
		}
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i].Artifact < resolved[j].Artifact })
	sort.Strings(notPinned)
	if comp.Derived && len(resolved) == 0 && len(notPinned) > 0 {
		disagreements = append(disagreements, fmt.Sprintf(
			"umbrella %s names %s, but the pin file pins none of them under any line shape",
			umbrella, strings.Join(notPinned, ", ")))
		notPinned = nil
	}

	if len(disagreements) > 0 {
		sort.Strings(disagreements)
		return Marker{
			State:         MarkerInconsistent,
			Umbrella:      umbrella,
			Artifacts:     resolved,
			Disagreements: disagreements,
			NotPinned:     notPinned,
			Origin:        comp.Origin,
			Reason: fmt.Sprintf("known-inconsistent: %d record(s) disagree with umbrella %s",
				len(disagreements), umbrella),
		}
	}

	return Marker{
		State:     MarkerKnown,
		Umbrella:  umbrella,
		Artifacts: resolved,
		NotPinned: notPinned,
		Origin:    comp.Origin,
		Reason:    "known: umbrella " + umbrella,
	}
}

// Report renders the marker as human-readable lines for stdout. The word
// "umbrella" always appears (the marker's subject), the disagreeing records are
// named on the inconsistent state, and NO "assume"/"latest" wording ever appears —
// a could-not-determine answer says only what it could not determine.
func (m Marker) Report() string {
	var b strings.Builder
	fmt.Fprintf(&b, "state: %s\n", m.State)
	switch m.State {
	case MarkerKnown:
		fmt.Fprintf(&b, "umbrella: %s\n", m.Umbrella)
		fmt.Fprintln(&b, "made of:")
		for _, a := range m.Artifacts {
			fmt.Fprintf(&b, "  - %s %s\n", a.Artifact, a.Tag)
		}
	case MarkerInconsistent:
		fmt.Fprintf(&b, "umbrella: %s\n", m.Umbrella)
		fmt.Fprintln(&b, "disagreements:")
		for _, d := range m.Disagreements {
			fmt.Fprintf(&b, "  - %s\n", d)
		}
	case MarkerCouldNotDetermine:
		if m.Umbrella != "" {
			fmt.Fprintf(&b, "umbrella: %s\n", m.Umbrella)
		}
	}
	if len(m.NotPinned) > 0 {
		fmt.Fprintf(&b, "not pinned here (the release ships them; this repo installs none): %s\n",
			strings.Join(m.NotPinned, ", "))
	}
	if m.Origin != "" {
		fmt.Fprintf(&b, "composition derived from: %s\n", m.Origin)
	}
	fmt.Fprintf(&b, "%s\n", m.Reason)
	return b.String()
}
