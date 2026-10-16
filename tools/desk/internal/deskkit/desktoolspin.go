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
const DeskToolsArtifact = "desk-tools"

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

// DeskToolsReleasePin returns the desk-tools release tag pinned in dir's `.assay-versions`
// and the artifact name of the line it came from. It asks for each legitimate spelling IN
// FULL — the bare `desk-tools` line, then `desk-tools-<platform>` — and never matches loosely:
// ArtifactPin's trailing-space selector is what keeps `desk-tools ` from matching
// `desk-tools-linux-amd64 `, and that disambiguation is a control. found=false means no pin
// file in dir or no usable line of either shape; malformed-pin detection is `deskpins
// --check`'s job, not this reader's.
func DeskToolsReleasePin(dir, platform string) (tag, artifact string, found bool) {
	if dir == "" {
		return "", "", false
	}
	for _, name := range []string{DeskToolsArtifact, DeskToolsPlatformArtifact(platform)} {
		if name == "" {
			continue
		}
		if t, _, err := ArtifactPin(dir, name); err == nil && t != "" {
			return t, name, true
		}
	}
	return "", "", false
}
