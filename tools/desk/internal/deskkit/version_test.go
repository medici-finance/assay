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

// releaseWorkflowPath is the release workflow that builds and packages
// desk-tools: .github/workflows/release.yml, the umbrella release. (This guard
// once read a release-desk.yml this repository never carried, so it skipped on
// every run and guarded nothing — see fixture_skip_test.go's class guard.)
var releaseWorkflowPath = filepath.Join("..", "..", "..", "..", ".github", "workflows", "release.yml")

// deskToolsBuildStepName is the release.yml step that cross-compiles and
// packages the desk-tools binaries — the step whose ldflags must carry the
// ReleaseTag stamp.
const deskToolsBuildStepName = "Build and package desk-tools binaries"

// deskkitPkgFromGoMod derives the -X symbol prefix from tools/desk/go.mod, so a
// module rename that leaves the workflow stamping the OLD path (which the Go
// linker silently ignores) reddens the guard instead of passing it.
func deskkitPkgFromGoMod(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("tools/desk/go.mod not readable: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(mod) + "/internal/deskkit"
		}
	}
	t.Fatal("tools/desk/go.mod carries no module line")
	return ""
}

// workflowStep returns the text of the release.yml step named name — from its
// `- name:` line up to the next step at the same indentation — or "" when the
// workflow carries no such step.
func workflowStep(wf, name string) string {
	lines := strings.Split(wf, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed != "- name: "+name {
			continue
		}
		indent := line[:len(line)-len(trimmed)]
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], indent+"- ") {
				end = j
				break
			}
		}
		return strings.Join(lines[i:end], "\n")
	}
	return ""
}

// releaseTagStampProblems reports every way the desk-tools build step in wf
// fails to stamp deskkit.ReleaseTag from the resolved release tag. It checks
// the STEP, not the whole file: the stamp appears in comments too, and
// RELEASE_TAG feeds other steps, so a whole-file substring match stays green
// with the real stamp deleted or pointed at the wrong value.
func releaseTagStampProblems(wf, deskkitPath string) []string {
	step := workflowStep(wf, deskToolsBuildStepName)
	if step == "" {
		return []string{"release.yml has no \"" + deskToolsBuildStepName + "\" step — nothing builds desk-tools, so nothing stamps it"}
	}
	var problems []string
	// Only a non-comment line counts: a comment naming the stamp stamps nothing.
	var code []string
	for _, line := range strings.Split(step, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			code = append(code, line)
		}
	}
	body := strings.Join(code, "\n")
	stamp := "-X " + deskkitPath + ".ReleaseTag=${RELEASE_TAG}"
	if !strings.Contains(body, stamp) {
		problems = append(problems, "the desk-tools build step does not carry `"+stamp+"` — released binaries would report \"dev\" (or a value that is not the release tag) and defeat pin checks")
	}
	// The stamp is fed from the RESOLVED release tag, via env: — never a
	// ${{ }} splice inside run:, and never a literal.
	if !strings.Contains(body, "RELEASE_TAG: ${{ needs.resolve.outputs.tag }}") {
		problems = append(problems, "the desk-tools build step does not set RELEASE_TAG from needs.resolve.outputs.tag in env: — the stamped value would not be the resolved release tag")
	}
	// The stamped LDFLAGS must actually reach the build.
	if !strings.Contains(body, `-ldflags "$LDFLAGS"`) {
		problems = append(problems, "the desk-tools build step does not pass $LDFLAGS to go build — the stamp would be computed and discarded")
	}
	return problems
}

// TestVersionStampedFromReleaseWorkflow is the workflow assertion for the
// desk-tools release stamp (mirrors statusgen/version_test.go's "release
// workflow stamps the tag" subtest). The `-X …deskkit.ReleaseTag=$RELEASE_TAG`
// stamp is the whole mechanism that maps a running desk-tools binary back to
// its release; a release built without it ships binaries that answer "dev" and
// silently defeat every pin check. This test goes RED if the stamp is removed
// from release.yml's desk-tools build step, fed a value other than the
// resolved release tag, or left out of the go build.
//
// It is DELIBERATELY a distinct, named test: internal/deskkit already carries
// TestVersionUnpinned / TestVersionPinned, so a Verify row matching `-run
// Version` would pass today without this stamp ever being wired.
func TestVersionStampedFromReleaseWorkflow(t *testing.T) {
	skipIfFixtureAbsent(t, releaseWorkflowPath,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(releaseWorkflowPath)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", releaseWorkflowPath, err)
	}
	for _, p := range releaseTagStampProblems(string(raw), deskkitPkgFromGoMod(t)) {
		t.Errorf("release.yml: %s", p)
	}
}

// TestReleaseTagStampMissingIsCaught is the mutation control for the guard
// above, in TestReleaseAuthorizerStampMissingIsCaught's shape: each guarded
// piece of the stamp is broken in a copy of release.yml, and EVERY break must
// make releaseTagStampProblems report a problem. A guard that still passes with
// its guarded text removed guards nothing; this proves it can fail on every
// suite execution, not only inside a mutation harness.
func TestReleaseTagStampMissingIsCaught(t *testing.T) {
	skipIfFixtureAbsent(t, releaseWorkflowPath,
		".github/ is not part of this repository's published file set")
	raw, err := os.ReadFile(releaseWorkflowPath)
	if err != nil {
		t.Fatalf("release workflow not readable at %s: %v", releaseWorkflowPath, err)
	}
	wf := string(raw)
	pkg := deskkitPkgFromGoMod(t)

	// Positive control on the intact tree first, or the mutations prove nothing.
	if problems := releaseTagStampProblems(wf, pkg); len(problems) != 0 {
		t.Fatalf("the intact release.yml already reports problems (%v) — fix the stamp before proving the check can catch its absence", problems)
	}

	// Every mutation but the rename is applied INSIDE the desk-tools build step:
	// RELEASE_TAG and -ldflags "$LDFLAGS" also appear in other steps, so a
	// whole-file replace could hit a sibling step and leave this one intact.
	step := workflowStep(wf, deskToolsBuildStepName)
	stamp := "-X " + pkg + ".ReleaseTag=${RELEASE_TAG}"
	for _, m := range []struct {
		name, from, to string
		wholeFile      bool
	}{
		{name: "stamp removed", from: stamp, to: ""},
		{name: "stamp fed the commit, not the tag", from: stamp, to: "-X " + pkg + ".ReleaseTag=${SHA_SHORT}"},
		{name: "stamp fed a literal", from: stamp, to: "-X " + pkg + ".ReleaseTag=dev"},
		{name: "stamp on the wrong package", from: stamp, to: "-X " + pkg + "x.ReleaseTag=${RELEASE_TAG}"},
		{name: "stamp commented out", from: "                   " + stamp, to: "#                  " + stamp},
		{name: "RELEASE_TAG not from the resolved tag", from: "RELEASE_TAG: ${{ needs.resolve.outputs.tag }}", to: "RELEASE_TAG: v0.0.0"},
		{name: "LDFLAGS not passed to go build", from: `go build -ldflags "$LDFLAGS" \`, to: `go build \`},
		{name: "desk-tools build step renamed away", from: "- name: " + deskToolsBuildStepName, to: "- name: Build something else", wholeFile: true},
	} {
		src := step
		if m.wholeFile {
			src = wf
		}
		if !strings.Contains(src, m.from) {
			t.Errorf("%s: guarded text %q is not in the desk-tools build step — the mutation control and the workflow have drifted apart", m.name, m.from)
			continue
		}
		mutated := strings.Replace(src, m.from, m.to, 1)
		if !m.wholeFile {
			mutated = strings.Replace(wf, step, mutated, 1)
		}
		if problems := releaseTagStampProblems(mutated, pkg); len(problems) == 0 {
			t.Errorf("%s was NOT caught — the release-stamp guard passes with the stamp broken, so it guards nothing", m.name)
		}
	}
}

// TestCellctlPackagedInReleaseWorkflow guards #850: cellctl must ship inside
// desk-tools-<platform>.tar.gz, stamped with the release tag.
//
// The CLAIM is unchanged; the MECHANISM changed when cellctl was ported to Go. It used to be a
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
	// beside the binaries is exactly the drift the Go port retired.
	if strings.Contains(wf, "CELLCTL_VERSION") {
		t.Error("release.yml still sed-stamps CELLCTL_VERSION — the shell-script packaging exception was removed when cellctl was ported to Go; it is stamped by -ldflags like every other verb")
	}
}
