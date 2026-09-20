package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionUnpinned(t *testing.T) {
	oldS, oldB := SourceSHA, BuiltAt
	t.Cleanup(func() { SourceSHA, BuiltAt = oldS, oldB })

	SourceSHA, BuiltAt = "", ""
	s, b := Version()
	if s != "unpinned" || b != "unpinned" {
		t.Fatalf("Version() = %q,%q, want unpinned,unpinned", s, b)
	}
	if IsPinned() {
		t.Fatalf("IsPinned() = true for empty stamp")
	}
	var buf bytes.Buffer
	WarnIfUnpinned(&buf)
	if !strings.Contains(buf.String(), "UNPINNED") {
		t.Fatalf("WarnIfUnpinned did not warn: %q", buf.String())
	}
}

func TestVersionPinned(t *testing.T) {
	oldS, oldB := SourceSHA, BuiltAt
	t.Cleanup(func() { SourceSHA, BuiltAt = oldS, oldB })

	SourceSHA, BuiltAt = "abc1234", "2026-07-10T00:00:00Z"
	s, b := Version()
	if s != "abc1234" || b != "2026-07-10T00:00:00Z" {
		t.Fatalf("Version() = %q,%q, want the stamped values", s, b)
	}
	if !IsPinned() {
		t.Fatalf("IsPinned() = false for stamped binary")
	}
	var buf bytes.Buffer
	WarnIfUnpinned(&buf)
	if buf.Len() != 0 {
		t.Fatalf("WarnIfUnpinned wrote %q for a pinned binary", buf.String())
	}
}

// TestReleaseTagOrDev pins the pin-checkability contract for desk-tools: a build
// stamped with a release tag reports it, and an UNSTAMPED build answers "dev"
// rather than inventing a release number — mirroring statusgen's default. A
// binary that cannot say which release it is makes a stale install
// indistinguishable from a current one.
func TestReleaseTagOrDev(t *testing.T) {
	old := ReleaseTag
	t.Cleanup(func() { ReleaseTag = old })

	ReleaseTag = ""
	if got := ReleaseTagOrDev(); got != "dev" {
		t.Errorf("unstamped ReleaseTagOrDev() = %q, want dev — an unstamped build must not claim a release", got)
	}
	ReleaseTag = "desk-tools/v9.9.9"
	if got := ReleaseTagOrDev(); got != "desk-tools/v9.9.9" {
		t.Errorf("stamped ReleaseTagOrDev() = %q, want the stamped tag", got)
	}
	// The tag stamp is additive: it must not change which builds are pinned.
	SourceSHA, BuiltAt = "", ""
	if IsPinned() {
		t.Errorf("IsPinned() = true off the ReleaseTag stamp alone — the tag must not change pinned-ness")
	}
	SourceSHA, BuiltAt = "", "" // leave cleared; TestVersion* set their own
}

// TestVersionStampedFromReleaseWorkflow is the NEW workflow assertion Task 5
// requires (mirrors statusgen/version_test.go's "release workflow stamps the
// tag" subtest). The `-X …deskkit.ReleaseTag=$RELEASE_TAG` stamp is the whole
// mechanism that maps a running desk-tools binary back to its
// `desk-tools/vX.Y.Z`; a release built without it ships binaries that answer
// "dev" and silently defeat every pin check. This test goes RED if the stamp is
// ever removed from release-desk.yml.
//
// It is DELIBERATELY a distinct, named test: internal/deskkit already carries
// TestVersionUnpinned / TestVersionPinned, so a Verify row matching `-run
// Version` would pass today without this stamp ever being wired.
func TestVersionStampedFromReleaseWorkflow(t *testing.T) {
	// internal/deskkit sits at tools/desk/internal/deskkit; the repo root is four
	// levels up.
	path := filepath.Join("..", "..", "..", "..", ".github", "workflows", "release-desk.yml")
	skipIfFixtureAbsent(t, path,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", path, err)
	}
	wf := string(raw)
	if !strings.Contains(wf, "deskkit.ReleaseTag=") {
		t.Error("release-desk.yml does not stamp deskkit.ReleaseTag — released binaries would report \"dev\" and defeat pin checks")
	}
	// The stamp must be fed from the resolved release tag, not a literal.
	if !strings.Contains(wf, "RELEASE_TAG") {
		t.Error("release-desk.yml stamps ReleaseTag but not from $RELEASE_TAG — the tag would not be the resolved release tag")
	}
}

// TestCellctlPackagedInReleaseWorkflow guards #850: cellctl must ship inside
// desk-tools-<platform>.tar.gz, stamped with the release tag.
//
// The CLAIM is unchanged; the MECHANISM changed at desk-containers/10. cellctl used to be a
// hand-maintained shell script the workflow `sed`-stamped and copied in as a packaging
// EXCEPTION. It is now tools/desk/cmd/cellctl, so the generic `for cmd in cmd/*/` loop builds
// and stages it like every other verb, and `-X main.cellctlVersion` in the shared LDFLAGS stamps
// it the way -ldflags stamps the rest. This test goes RED if either half regresses: the package
// disappearing (nothing to build, nothing in the tarball) or the stamp being dropped (a released
// copy reporting "dev", which defeats `cellctl --version`).
func TestCellctlPackagedInReleaseWorkflow(t *testing.T) {
	// internal/deskkit sits at tools/desk/internal/deskkit; the repo root is four
	// levels up.
	root := filepath.Join("..", "..", "..", "..")
	path := filepath.Join(root, ".github", "workflows", "release.yml")
	skipIfFixtureAbsent(t, path,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", path, err)
	}
	wf := string(raw)
	// The package is what makes cellctl one of the cmd/*/ builds. Without it the loop has
	// nothing to build and the tarball carries no cellctl at all.
	if _, serr := os.Stat(filepath.Join(root, "tools", "desk", "cmd", "cellctl")); serr != nil {
		t.Errorf("tools/desk/cmd/cellctl is missing (%v) — cellctl would ship nowhere (#850)", serr)
	}
	if !strings.Contains(wf, "for cmd in cmd/*/") {
		t.Error("release.yml no longer builds every tools/desk/cmd/*/ — cellctl ships only because it is one of them (#850)")
	}
	if !strings.Contains(wf, "-X main.cellctlVersion=${RELEASE_TAG}") {
		t.Error("release.yml does not stamp main.cellctlVersion with the release tag — a released cellctl would report \"dev\" and defeat `cellctl --version` (#850)")
	}
	// The packaging EXCEPTION is gone and must not come back: a sed-stamped script copied in
	// beside the binaries is exactly the drift desk-containers/10 retired.
	if strings.Contains(wf, "CELLCTL_VERSION") {
		t.Error("release.yml still sed-stamps CELLCTL_VERSION — the shell-script packaging exception was removed at desk-containers/10; cellctl is stamped by -ldflags like every other verb")
	}
}
