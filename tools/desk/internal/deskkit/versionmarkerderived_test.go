package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// derivedRoot materialises an adopter root pinned PER PLATFORM ONLY (the shape
// docs/adopting-assay.md writes) against a checksums-derived v1.0.8 composition.
func derivedRoot(t *testing.T, pins string) (string, CompositionSource) {
	t.Helper()
	root := t.TempDir()
	rel := filepath.Join(root, ReleasesDir)
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rel, "v1.0.8.checksums.txt"), []byte(checksumsFixture()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), []byte(pins), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, CompositionSource{ReleasesDir: rel}
}

// TestMarker_DerivedPerPlatformPins — the issue's adopter: an umbrella line and
// per-platform statusgen + desk-tools lines, no bare lines, no manifest. The marker
// derives the composition from checksums.txt and reads KNOWN; qualgen (shipped but
// not installed here) is provenance, not a disagreement.
func TestMarker_DerivedPerPlatformPins(t *testing.T) {
	root, src := derivedRoot(t, "assay v1.0.8\n"+
		"statusgen-darwin-arm64 v1.0.8 "+strings.Repeat("1", 64)+"\n"+
		"statusgen-linux-amd64  v1.0.8 "+strings.Repeat("3", 64)+"  # CI\n"+
		"desk-tools-linux-amd64 v1.0.8 "+strings.Repeat("9", 64)+"\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerKnown {
		t.Fatalf("state = %s, want known; reason=%s disagreements=%v", m.State, m.Reason, m.Disagreements)
	}
	if len(m.Artifacts) != 2 || m.Artifacts[0].Artifact != "desk-tools" || m.Artifacts[1].Artifact != "statusgen" {
		t.Errorf("artifacts = %+v", m.Artifacts)
	}
	if len(m.NotPinned) != 1 || m.NotPinned[0] != "qualgen" {
		t.Errorf("NotPinned = %v, want [qualgen]", m.NotPinned)
	}
	rep := m.Report()
	for _, want := range []string{"umbrella: v1.0.8", "not pinned here", "qualgen", "composition derived from"} {
		if !strings.Contains(rep, want) {
			t.Errorf("report missing %q:\n%s", want, rep)
		}
	}
	// ReadMarker (offline) reads the materialised checksums too.
	if m2 := ReadMarker(root, src.ReleasesDir); m2.State != MarkerKnown {
		t.Errorf("ReadMarker offline: state = %s", m2.State)
	}
}

// TestMarker_DerivedPlatformLinesDisagree — a half-moved bump (two platform lines
// on different tags) is inconsistent and names the lines.
func TestMarker_DerivedPlatformLinesDisagree(t *testing.T) {
	root, src := derivedRoot(t, "assay v1.0.8\n"+
		"statusgen-darwin-arm64 v1.0.8 "+strings.Repeat("1", 64)+"\n"+
		"statusgen-linux-amd64  v1.0.7 "+strings.Repeat("3", 64)+"\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerInconsistent {
		t.Fatalf("state = %s, want inconsistent; reason=%s", m.State, m.Reason)
	}
	if rep := m.Report(); !strings.Contains(rep, "statusgen-linux-amd64") || !strings.Contains(rep, "disagree") {
		t.Errorf("report must name the disagreeing lines:\n%s", rep)
	}
}

// TestMarker_DerivedPinnedAtOtherTag — platform lines agree with each other but
// not with the umbrella: the classic inconsistent state, still caught.
func TestMarker_DerivedPinnedAtOtherTag(t *testing.T) {
	root, src := derivedRoot(t, "assay v1.0.8\n"+
		"statusgen-linux-amd64 v1.0.7 "+strings.Repeat("3", 64)+"\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerInconsistent || !strings.Contains(m.Report(), "statusgen pin is v1.0.7") {
		t.Errorf("state = %s report:\n%s", m.State, m.Report())
	}
}

// TestMarker_DerivedNothingPinned — an umbrella line no artifact line backs is a
// disagreement, never a "known" answer resting on nothing.
func TestMarker_DerivedNothingPinned(t *testing.T) {
	root, src := derivedRoot(t, "assay v1.0.8\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerInconsistent || !strings.Contains(m.Report(), "pins none of them") {
		t.Errorf("state = %s report:\n%s", m.State, m.Report())
	}
}

// TestMarker_DerivedNoSourceAnywhere — no manifest, no checksums, no fetch: still
// could-not-determine, and the report names where it looked.
func TestMarker_DerivedNoSourceAnywhere(t *testing.T) {
	root, src := derivedRoot(t, "assay v1.0.9\nstatusgen-linux-amd64 v1.0.9 "+strings.Repeat("3", 64)+"\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerCouldNotDetermine {
		t.Fatalf("state = %s, want could-not-determine", m.State)
	}
	if !strings.Contains(m.Reason, "v1.0.9.checksums.txt") {
		t.Errorf("reason must name the materialisation path it looked for: %s", m.Reason)
	}
	if lc := strings.ToLower(m.Report()); strings.Contains(lc, "assum") || strings.Contains(lc, "latest") {
		t.Errorf("no assume/latest wording:\n%s", m.Report())
	}
	// With a fetch that can serve the tag, the same root reads known.
	src.Fetch = fakeFetch(map[string]string{ChecksumsURL(DefaultReleaseHome, "v1.0.9"): checksumsFixture()})
	if m := ReadMarkerFrom(root, src); m.State != MarkerKnown || !strings.Contains(m.Origin, "v1.0.9/checksums.txt") {
		t.Errorf("fetched: state=%s origin=%q reason=%s", m.State, m.Origin, m.Reason)
	}
}

// TestMarker_HandAuthoredMissingLineStillDisagrees — under a hand-authored
// manifest an un-pinned named artifact is still a disagreement (unchanged), while
// per-platform-only lines now resolve the tag instead of reading as "no line".
func TestMarker_HandAuthoredMissingLineStillDisagrees(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join(root, ReleasesDir)
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "umbrella: v0.11.0\nartifacts:\n  - artifact: statusgen\n    tag: v0.11.0\n  - artifact: desk-tools\n    tag: v0.11.0\n"
	if err := os.WriteFile(filepath.Join(rel, "v0.11.0.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(pins string) {
		if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), []byte(pins), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("assay v0.11.0\nstatusgen-linux-amd64 v0.11.0 " + strings.Repeat("a", 64) + "\n")
	m := ReadMarker(root, rel)
	if m.State != MarkerInconsistent || !strings.Contains(m.Report(), "no desk-tools line of any shape") {
		t.Errorf("missing desk-tools under a hand-authored manifest must disagree: state=%s\n%s", m.State, m.Report())
	}
	write("assay v0.11.0\nstatusgen-linux-amd64 v0.11.0 " + strings.Repeat("a", 64) + "\ndesk-tools-linux-amd64 v0.11.0 " + strings.Repeat("b", 64) + "\n")
	if m := ReadMarker(root, rel); m.State != MarkerKnown {
		t.Errorf("per-platform-only pins under a hand-authored manifest: state=%s reason=%s", m.State, m.Reason)
	}
}
