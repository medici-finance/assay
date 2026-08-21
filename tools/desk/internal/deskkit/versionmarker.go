package deskkit

// versionmarker.go — the adopter version marker (distribution/08). It answers the
// one question `upgrade-assay` (distribution/09) must ask before it can move an
// adopter anywhere: "what umbrella version is this adopter on, and which artifact
// versions is that made of." The answer is ASSEMBLED from records that already
// exist — the `.assay-versions` pin file (umbrella line + per-artifact lines, from
// distribution/04) cross-checked against the umbrella's composition manifest
// (distribution/02) — never invented as a fourth source of truth.
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
// The plain `vX.Y.Z` tag shape is the shipped reality (Ian's 2026-08-15 ruling;
// docs/distribution.md § The umbrella line). The marker reads it through
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
	// Reason is a one-line human summary, always set.
	Reason string
}

// ReadMarker assembles the marker for the consumer repo at root, reading its
// composition manifests from releasesDir (conventionally <root>/releases). It is
// pure and offline: it reads only the two files and never the platform install
// cache, so it can never mutate an adopter's environment.
func ReadMarker(root, releasesDir string) Marker {
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

	// 2. The composition manifest for that umbrella version. Fail-closed if it
	//    cannot be read — a "known" answer must never rest on a manifest we could
	//    not open.
	comp, err := LoadComposition(releasesDir, umbrella)
	if err != nil {
		return Marker{
			State:    MarkerCouldNotDetermine,
			Umbrella: umbrella,
			Reason:   "could-not-determine: " + err.Error(),
		}
	}

	// 3. Cross-check every artifact the composition names against the pin file.
	//    A pinned tag that differs from the composition's tag is the disagreement
	//    the inconsistent state exists to surface; a missing pin for a named
	//    artifact is likewise a disagreement (the adopter is not on the composition
	//    the umbrella claims).
	var resolved []MarkerArtifactVersion
	var disagreements []string
	for _, a := range comp.Artifacts {
		name := a.ArtifactName()
		if name == "" {
			disagreements = append(disagreements,
				fmt.Sprintf("composition entry %q names no artifact and no namespaced tag", a.Tag))
			continue
		}
		pinTag, _, perr := ArtifactPin(root, name)
		if perr != nil {
			disagreements = append(disagreements, fmt.Sprintf(
				"umbrella %s names %s %s, but the pin file has no readable %s line (%v)",
				umbrella, name, a.Tag, name, perr))
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

	if len(disagreements) > 0 {
		sort.Strings(disagreements)
		return Marker{
			State:         MarkerInconsistent,
			Umbrella:      umbrella,
			Artifacts:     resolved,
			Disagreements: disagreements,
			Reason: fmt.Sprintf("known-inconsistent: %d record(s) disagree with umbrella %s",
				len(disagreements), umbrella),
		}
	}

	return Marker{
		State:     MarkerKnown,
		Umbrella:  umbrella,
		Artifacts: resolved,
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
	fmt.Fprintf(&b, "%s\n", m.Reason)
	return b.String()
}
