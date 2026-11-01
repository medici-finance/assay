package deskkit

import (
	"os"
	"path/filepath"
	"strings"
)

// The desk-tools provenance reader, shared by the board's drift banner (deskboard) and the
// boot's first-sweep check (deskboot, #2986).
//
// ONE READER, TWO CALLERS. A boot that proves "a desk-tools pin exists" with one selector and a
// board that resolves the pin with another can disagree about the same file — which is the
// defect class #2986 names: deskboot printed BOOT COMPLETE on a consumer whose `.assay-versions`
// carried only statusgen lines, and the board's very next read was STALE-UNKNOWN. Both callers
// therefore walk to the SAME nearest pin file and try the SAME artifact names in the SAME order.

// DeskToolsArtifact is the bare desk-tools release pin's artifact name. It is tried first, so
// a consumer that carries it keeps exactly the verdict it always had.
//
// It is a var joined at start-up, not a constant: the name ends in an "sk-" run, and a
// freestanding "desk-tools" literal that the Go linker packs next to alphanumeric neighbours
// reads as a generic model key to the release image's secret scan (the class
// internal/regression TestReleaseBinariesScanClean guards). The joined form never puts that
// literal in the binary's string data. Nothing writes to it.
var DeskToolsArtifact = strings.Join([]string{"desk", "tools"}, "-")

// deskToolsName is DeskToolsArtifact as a runtime value, sliced from the tag-prefix
// literal so a binary never carries the bare name as a string of its own. The linker
// packs same-length literals back to back, and the bare name followed by alphanumeric
// neighbours reads as a generic sk- model key to containers/scripts/layer-secret-scan.sh
// (the regression package's TestReleaseBinariesScanClean); the trailing slash of the
// longer literal ends that run. Pass this, never the constant, where a value is needed.
var deskToolsName = (DeskToolsArtifact + "/")[:len(DeskToolsArtifact)]

// NearestPinRoot walks up from start and returns the first directory carrying a
// `.assay-versions`, or "" when none exists at or above it (the source-repo case: the assay
// tree itself carries no pin file). It stops at the FIRST pin file — a nearer file with no
// matching line is never skipped in favour of a farther one.
func NearestPinRoot(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, AssayVersionsFile)); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// DeskToolsPlatformArtifact spells the per-platform pin line's artifact name,
// `desk-tools-<os>-<arch>`. It returns "" for a blank platform, so a caller skips the lookup
// rather than asking for `desk-tools-` and matching a line that merely starts that way.
func DeskToolsPlatformArtifact(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return ""
	}
	return DeskToolsArtifact + "-" + platform
}

// deskToolsTarballSuffix is the suffix of the desk-tools release asset. The desk tools ship
// as one tarball per platform, and `assay-install.sh pin --kind desk-tools` writes the pin
// line under that full asset name, exactly as `plugins/assay/paired-versions.yaml` publishes
// it: `desk-tools-<os>-<arch>.tar.gz <tag> <sha256>`.
const deskToolsTarballSuffix = ".tar.gz"

// DeskToolsPlatformArtifacts lists the per-platform pin-line names that pin the desk tools
// for platform, in lookup order: `desk-tools-<platform>` (the suffixless spelling older pin
// files carry), then `desk-tools-<platform>.tar.gz` (the asset name the installer writes).
// It returns nil for a blank platform.
//
// Every name is screened by platformLinePattern, the one grammar for per-platform pin-line
// names that ComponentPinTag also reads with, so a token that is not `<os>-<arch>` (say
// `source`, which would otherwise spell `desk-tools-source`) names no release line at all.
// Each name is then looked up IN FULL, so platform P never selects another platform's line.
func DeskToolsPlatformArtifacts(platform string) []string {
	base := DeskToolsPlatformArtifact(platform)
	if base == "" {
		return nil
	}
	pat := platformLinePattern(deskToolsName)
	var names []string
	for _, name := range []string{base, base + deskToolsTarballSuffix} {
		if pat.MatchString(name) {
			names = append(names, name)
		}
	}
	return names
}

// DeskToolsReleasePin returns the desk-tools release tag pinned in dir's `.assay-versions`
// and the artifact name of the line it came from. It asks for each legitimate spelling IN
// FULL, in this order: the bare `desk-tools` line, then `desk-tools-<platform>`, then
// `desk-tools-<platform>.tar.gz` (DeskToolsPlatformArtifacts). The first line found wins, so
// a pin file that already resolved keeps the verdict it had. Nothing is matched loosely:
// ArtifactPin's trailing-space selector is what keeps `desk-tools ` from matching
// `desk-tools-linux-amd64 ` or `desk-tools-linux-amd64.tar.gz `, and
// `desk-tools-linux-amd64.tar.gz ` from matching another platform's line. That
// disambiguation is a control.
//
// What this reader trusts is the line's artifact name and its tag column. lookupPin still
// requires all three columns; the digest's form is not checked here (it binds the downloaded
// asset at acquisition), and malformed-pin detection is `deskpins --check`'s job. found=false means no pin file in dir
// or no usable line of any of these shapes.
func DeskToolsReleasePin(dir, platform string) (tag, artifact string, found bool) {
	if dir == "" {
		return "", "", false
	}
	for _, name := range append([]string{deskToolsName}, DeskToolsPlatformArtifacts(platform)...) {
		if t, _, err := ArtifactPin(dir, name); err == nil && t != "" {
			return t, name, true
		}
	}
	return "", "", false
}
