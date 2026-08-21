package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// markerRoot returns the testdata root and its releases dir for a fixture.
func markerRoot(name string) (root, releases string) {
	root = filepath.Join("testdata", name)
	return root, filepath.Join(root, ReleasesDir)
}

// TestMarker_Known — a consistent fixture resolves to one umbrella version at
// exit 0, and the report names the umbrella and the artifacts it is made of.
func TestMarker_Known(t *testing.T) {
	root, rel := markerRoot("marker-known")
	m := ReadMarker(root, rel)
	if m.State != MarkerKnown {
		t.Fatalf("state = %s, want known; reason=%s", m.State, m.Reason)
	}
	if m.State.ExitCode() != ExitOK {
		t.Errorf("known exit = %d, want 0", m.State.ExitCode())
	}
	if m.Umbrella != "v0.11.0" {
		t.Errorf("umbrella = %q, want v0.11.0", m.Umbrella)
	}
	if len(m.Artifacts) != 2 {
		t.Fatalf("artifacts = %+v, want 2", m.Artifacts)
	}
	rep := m.Report()
	if !strings.Contains(rep, "umbrella") {
		t.Errorf("report does not mention umbrella:\n%s", rep)
	}
}

// TestMarker_Inconsistent — a fixture whose statusgen pin names a version the
// umbrella composition does not resolves to known-inconsistent at a DISTINCT
// non-zero exit, and NAMES both disagreeing records.
func TestMarker_Inconsistent(t *testing.T) {
	root, rel := markerRoot("marker-inconsistent")
	m := ReadMarker(root, rel)
	if m.State != MarkerInconsistent {
		t.Fatalf("state = %s, want inconsistent; reason=%s", m.State, m.Reason)
	}
	if m.State.ExitCode() == ExitOK {
		t.Errorf("inconsistent exit must be non-zero, got %d", m.State.ExitCode())
	}
	rep := m.Report()
	if !strings.Contains(rep, "statusgen") || !strings.Contains(rep, "umbrella") {
		t.Errorf("inconsistent report must name statusgen AND umbrella:\n%s", rep)
	}
}

// TestMarker_CouldNotDetermine_NoFile — a root with no pin file is
// could-not-determine, at a third exit code distinct from both known (0) and
// inconsistent, with NO "assume"/"latest" wording.
func TestMarker_CouldNotDetermine_NoFile(t *testing.T) {
	src, _ := markerRoot("marker-known")
	tmp := t.TempDir()
	// Copy only the releases dir; deliberately omit .assay-versions.
	if err := os.MkdirAll(filepath.Join(tmp, ReleasesDir), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(src, ReleasesDir, "v0.11.0.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ReleasesDir, "v0.11.0.yaml"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	m := ReadMarker(tmp, filepath.Join(tmp, ReleasesDir))
	if m.State != MarkerCouldNotDetermine {
		t.Fatalf("state = %s, want could-not-determine", m.State)
	}
	inconsistent := MarkerInconsistent.ExitCode()
	if c := m.State.ExitCode(); c == ExitOK || c == inconsistent {
		t.Errorf("could-not exit %d must differ from known(0) and inconsistent(%d)", c, inconsistent)
	}
	if lc := strings.ToLower(m.Report()); strings.Contains(lc, "assum") || strings.Contains(lc, "latest") {
		t.Errorf("could-not-determine must not say assume/latest:\n%s", m.Report())
	}
}

// TestMarker_NoUmbrellaPin — the live consumer's golden pin file carries no
// umbrella line; the marker reports "no umbrella pin" (could-not-determine),
// never an error over the valid file and never a default.
func TestMarker_NoUmbrellaPin(t *testing.T) {
	tmp := t.TempDir()
	raw, err := os.ReadFile(goldenRel)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, AssayVersionsFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m := ReadMarker(tmp, filepath.Join(tmp, ReleasesDir))
	if m.State != MarkerCouldNotDetermine {
		t.Fatalf("state = %s, want could-not-determine (no umbrella pin)", m.State)
	}
	if !strings.Contains(m.Report(), "no umbrella pin") {
		t.Errorf("report must say 'no umbrella pin':\n%s", m.Report())
	}
}

// TestMarker_MissingComposition — an umbrella line present but no composition
// manifest to check it against is fail-closed (could-not-determine), never a
// silently-known answer.
func TestMarker_MissingComposition(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, AssayVersionsFile),
		[]byte("assay v0.11.0\nstatusgen v0.11.0 "+strings.Repeat("a", 64)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := ReadMarker(tmp, filepath.Join(tmp, ReleasesDir)) // no releases dir at all
	if m.State != MarkerCouldNotDetermine {
		t.Fatalf("state = %s, want could-not-determine (no composition manifest)", m.State)
	}
	if m.Umbrella != "v0.11.0" {
		t.Errorf("umbrella still readable = %q, want v0.11.0", m.Umbrella)
	}
}
