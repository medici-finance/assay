package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The desk-tools provenance reader against the pin line the installer actually writes (#4842).
//
// `assay-install.sh pin --kind desk-tools` and `plugins/assay/paired-versions.yaml` both spell
// the per-platform line under the tarball's full asset name, `desk-tools-<os>-<arch>.tar.gz`.
// The reader asked only for `desk-tools` and `desk-tools-<os>-<arch>`, so deskboot's
// first-sweep-prereqs refused a pin file that the installer had just written, and deskboard's
// drift banner read the same file as could-not-check.
//
// Mutation spec: internal/deskkit/desktoolspin-mutations.json (run from tools/desk with
// `go run ./cmd/muhar -spec internal/deskkit/desktoolspin-mutations.json`).

// manifestDeskToolsLine is the linux-amd64 desk-tools line exactly as the v1.0.64 manifest
// publishes it and the installer copies it into `.assay-versions`.
const manifestDeskToolsLine = "desk-tools-linux-amd64.tar.gz v1.0.64 1e9b2269e3238bfff13949343ac44f2cc45cac22e8e44e5c9f442d7f8a79ce71"

// pinDigest is a well-formed sha256 for lines whose digest the reader never inspects.
const pinDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// writeDeskPins writes a `.assay-versions` holding exactly lines into a fresh dir.
func writeDeskPins(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// (a) The exact manifest line, beside the umbrella and statusgen lines a consumer carries
// for the same release, is desk-tools provenance for linux-amd64.
//
// regression: #4842
func TestDeskToolsPinTarballLine(t *testing.T) {
	dir := writeDeskPins(t,
		"assay v1.0.64",
		"statusgen v1.0.64 "+pinDigest,
		"statusgen-linux-amd64 v1.0.64 "+pinDigest,
		manifestDeskToolsLine,
	)
	tag, artifact, found := DeskToolsReleasePin(dir, "linux-amd64")
	if !found {
		t.Fatalf("the installer's line %q is not desk-tools provenance for linux-amd64", manifestDeskToolsLine)
	}
	if tag != "v1.0.64" {
		t.Errorf("tag = %q, want v1.0.64", tag)
	}
	if want := "desk-tools-linux-amd64.tar.gz"; artifact != want {
		t.Errorf("artifact = %q, want %q (the line the verdict came from)", artifact, want)
	}
}

// (b) The bare `desk-tools` lookup never matches a per-platform line of either spelling.
// With no platform token only the bare name is asked for, so a file carrying only platform
// lines has no pin.
//
// regression: #4842
func TestDeskToolsPinBareNoPlatform(t *testing.T) {
	dir := writeDeskPins(t,
		"desk-tools-linux-amd64 v1.0.64 "+pinDigest,
		manifestDeskToolsLine,
	)
	for _, platform := range []string{"", "   "} {
		if tag, artifact, found := DeskToolsReleasePin(dir, platform); found {
			t.Errorf("platform %q: the bare lookup matched %s %s — `desk-tools ` must not match a platform line",
				platform, artifact, tag)
		}
	}
}

// (c) A lookup for one platform never takes another platform's `.tar.gz` line, never takes a
// different component's line, and a token that is not `<os>-<arch>` selects nothing at all.
//
// regression: #4842
func TestDeskToolsPinOtherPlatform(t *testing.T) {
	t.Run("another platform's tarball line", func(t *testing.T) {
		dir := writeDeskPins(t,
			"desk-tools-darwin-arm64.tar.gz v1.0.64 "+pinDigest,
			"desk-tools-windows-amd64.tar.gz v1.0.64 "+pinDigest,
		)
		if tag, artifact, found := DeskToolsReleasePin(dir, "linux-amd64"); found {
			t.Errorf("linux-amd64 took %s %s, a foreign platform's line", artifact, tag)
		}
	})
	t.Run("another component's tarball line", func(t *testing.T) {
		dir := writeDeskPins(t,
			"desk-tools-guard-linux-amd64.tar.gz v1.0.64 "+pinDigest,
			"statusgen-linux-amd64.tar.gz v1.0.64 "+pinDigest,
		)
		if tag, artifact, found := DeskToolsReleasePin(dir, "linux-amd64"); found {
			t.Errorf("linux-amd64 took %s %s, another component's line", artifact, tag)
		}
	})
	t.Run("a non-platform token", func(t *testing.T) {
		dir := writeDeskPins(t,
			"desk-tools-source v1.0.64 0123456789abcdef0123456789abcdef01234567",
			"desk-tools-linux-amd64.tar.gz.tar.gz v1.0.64 "+pinDigest,
		)
		for _, platform := range []string{"source", "linux-amd64.tar.gz", "linux"} {
			if tag, artifact, found := DeskToolsReleasePin(dir, platform); found {
				t.Errorf("platform token %q took %s %s; only a `<os>-<arch>` token names a release line",
					platform, artifact, tag)
			}
		}
	})
}

// (d) The suffixless per-platform spelling and the bare line keep the verdicts they had, and
// precedence is unchanged: bare first, then `desk-tools-<platform>`, then the tarball name.
//
// regression: #4842
func TestDeskToolsPinNoRegression(t *testing.T) {
	cases := []struct {
		name, wantTag, wantArtifact string
		lines                       []string
	}{
		{"suffixless platform line only", "v1.0.60", "desk-tools-linux-amd64",
			[]string{"desk-tools-linux-amd64 v1.0.60 " + pinDigest}},
		{"bare line only", "v1.0.61", "desk-tools",
			[]string{"desk-tools v1.0.61 " + pinDigest}},
		{"bare line beats both platform spellings", "v1.0.61", "desk-tools",
			[]string{manifestDeskToolsLine, "desk-tools-linux-amd64 v1.0.60 " + pinDigest, "desk-tools v1.0.61 " + pinDigest}},
		{"suffixless platform line beats the tarball name", "v1.0.60", "desk-tools-linux-amd64",
			[]string{manifestDeskToolsLine, "desk-tools-linux-amd64 v1.0.60 " + pinDigest}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeDeskPins(t, tc.lines...)
			tag, artifact, found := DeskToolsReleasePin(dir, "linux-amd64")
			if !found || tag != tc.wantTag || artifact != tc.wantArtifact {
				t.Errorf("DeskToolsReleasePin = (%q, %q, %v), want (%q, %q, true)",
					tag, artifact, found, tc.wantTag, tc.wantArtifact)
			}
		})
	}
}
