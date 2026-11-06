package deskkit

import (
	"fmt"
	"io"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/medici-finance/assay/perfrec"
)

// Version stamping. SourceSHA and BuiltAt are injected at build time
// by the Makefile's desk-install / desk-build targets via:
//
//	-ldflags "-X github.com/medici-finance/assay/tools/desk/internal/deskkit.SourceSHA=<short-sha> \
//	          -X github.com/medici-finance/assay/tools/desk/internal/deskkit.BuiltAt=<rfc3339>"
//
// A `go run` / unstamped build leaves them empty; Version() then reports the
// toolchain's vcs.revision / vcs.time when the binary carries them, else "unpinned",
// and WarnIfUnpinned announces the missing stamp on stderr either way — so running
// from source (drift risk) is visible in every transcript.
var (
	SourceSHA = ""
	BuiltAt   = ""
	// ReleaseTag is the artifact-namespaced release tag (`desk-tools/vX.Y.Z`)
	// stamped by .github/workflows/release.yml (the desk-tools build step) via
	//   -X …/internal/deskkit.ReleaseTag=$RELEASE_TAG
	// mirroring release-statusgen.yml's `-X main.statusgenVersion`. It is what
	// maps a RUNNING binary back to the release it was cut from: SourceSHA alone
	// is a commit, not a version, so before this stamp nothing could say which
	// `desk-tools/vX.Y.Z` a binary is. A `go run` / unstamped build leaves it
	// empty, reported as "dev" (ReleaseTagOrDev) — never a fabricated release.
	ReleaseTag = ""
	// HotfixExpires is the RFC 3339 instant a hotfix build stops being valid, stamped by
	// `cellctl hotfix install` alone via `-X …/internal/deskkit.HotfixExpires=<rfc3339>`. A
	// release build never sets it: release.yml's -X set is unchanged, so a release binary
	// carries no `+` in its tag and an empty HotfixExpires.
	HotfixExpires = ""
)

// The ruled hotfix constants (per the driver's ruling). They are COMPILED IN: no ldflag, env
// var, flag or file changes them, and no workflow sets them. A test reaches a different value
// only through the consuming package's own unexported seams.
const (
	// HotfixSourceRule names the only source rule a hotfix may be built under: the source
	// commit is on main (the forge reports `main...<sha>` as `behind` or `identical`).
	HotfixSourceRule = "merged-main"
	// HotfixMaxTTL is the ceiling on a hotfix's lifetime (expires - built_at).
	HotfixMaxTTL = 7 * 24 * time.Hour
	// HotfixDefaultTTL is the lifetime `cellctl hotfix install` stamps when --ttl is absent.
	HotfixDefaultTTL = 3 * 24 * time.Hour
	// HotfixRequireSudo: install runs under sudo, hotfix files are root-owned, and every
	// layer refuses hotfix files owned by anyone else.
	HotfixRequireSudo = true
)

// hotfixIneligiblePaths is the compiled eligibility list behind HotfixIneligiblePaths. A
// source whose diff against the pin touches any of these is never hotfixable, whatever the
// change: schema and migration directories, permission / role-binding definitions, and
// on-disk record formats. An entry ending in `/` is a directory prefix; any other entry is
// one exact file. The hotfix identity itself (this file, hotfix.go, the pin grammar) is in
// the list, so a hotfix can never rewrite the rules it is checked against.
var hotfixIneligiblePaths = []string{
	// schema / migration directories
	"migrations/",
	"schemas/",
	"tools/desk/cmd/deskmigrate/",
	"tools/desk/internal/deskkit/migrate.go",
	// permission and role-binding definitions
	"tools/desk/internal/deskkit/custodyacl.go",
	"tools/desk/internal/deskkit/rosteracl.go",
	"tools/desk/internal/deskkit/rosterconfig.go",
	"tools/desk/internal/deskkit/roletoken.go",
	"tools/desk/internal/deskkit/appconfig.go",
	// on-disk record formats
	"tools/desk/internal/deskkit/decisionrecord.go",
	"tools/desk/internal/deskkit/dispatchrecord.go",
	// the identity and pin grammar the hotfix is checked against
	"tools/desk/internal/deskkit/pins.go",
	"tools/desk/internal/deskkit/version.go",
	"tools/desk/internal/deskkit/hotfix.go",
	"tools/desk/internal/deskkit/hotfixself.go",
	"tools/desk/internal/deskkit/hotfixseam.go",
	"tools/desk/internal/deskkit/hotfixseam_off.go",
	"tools/desk/internal/deskkit/hotfixseam_on.go",
	"tools/desk/internal/deskkit/hotfixowner_unix.go",
	"tools/desk/internal/deskkit/hotfixowner_windows.go",
	"tools/desk/internal/deskkit/hotfixexec_unix.go",
	"tools/desk/internal/deskkit/hotfixexec_windows.go",
	// statusgen's own copy of the hotfix identity (its own module), its version gate and its
	// schema migrations: a statusgen hotfix can never rewrite the rules it is checked against
	"statusgen/migrate.go",
	"statusgen/versiongate.go",
	"statusgen/hotfixself.go",
	"statusgen/hotfixseam_off.go",
	"statusgen/hotfixseam_on.go",
	"statusgen/hotfixowner_unix.go",
	"statusgen/hotfixowner_windows.go",
	"statusgen/hotfixexec_unix.go",
	"statusgen/hotfixexec_windows.go",
}

// HotfixIneligiblePaths returns a copy of the compiled ineligible-path list.
func HotfixIneligiblePaths() []string {
	return append([]string(nil), hotfixIneligiblePaths...)
}

// HotfixIneligibleMatch returns the first ineligible entry the repo-relative path p falls
// under, or "" when p is eligible.
func HotfixIneligibleMatch(p string) string {
	for _, e := range hotfixIneligiblePaths {
		if len(e) > 0 && e[len(e)-1] == '/' {
			if len(p) >= len(e) && p[:len(e)] == e {
				return e
			}
			continue
		}
		if p == e {
			return e
		}
	}
	return ""
}

// hotfixTagPattern is the ONLY hotfix grammar: a bare release tag, `+hotfix-`, and exactly
// twelve lowercase hex digits of the source commit. Any other `+…` suffix is neither a
// hotfix nor a pin.
var hotfixTagPattern = regexp.MustCompile(`^(v[0-9]+\.[0-9]+\.[0-9]+)\+hotfix-([0-9a-f]{12})$`)

// IsHotfix reports whether tag is exactly `vX.Y.Z+hotfix-<12 hex>`.
func IsHotfix(tag string) bool { return hotfixTagPattern.MatchString(tag) }

// HotfixBase returns the release tag a hotfix tag was built over (`v1.0.41` for
// `v1.0.41+hotfix-0123456789ab`); ok=false for anything that is not a hotfix tag.
func HotfixBase(tag string) (base string, ok bool) {
	m := hotfixTagPattern.FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// HotfixSHA returns the 12-hex source commit a hotfix tag names, or "" when tag is not one.
func HotfixSHA(tag string) string {
	m := hotfixTagPattern.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[2]
}

// HotfixTag spells the tag a hotfix of source over base carries.
func HotfixTag(base, sourceSHA string) string {
	if len(sourceSHA) > 12 {
		sourceSHA = sourceSHA[:12]
	}
	return base + "+hotfix-" + sourceSHA
}

const unpinned = "unpinned"

// readBuildInfo is the build-info reader Version falls back to; a var so a test can present
// a binary with or without VCS information.
var readBuildInfo perfrec.BuildInfoFunc = debug.ReadBuildInfo

// devRelease is what an unstamped build answers for its release tag. It mirrors
// statusgen's `dev` default: a binary that was not cut from a release must say so
// rather than claim a version, so a source build can never be mistaken for a pin.
const devRelease = "dev"

// Version returns (sourceSHA, builtAt). A value not stamped in at link time falls back to
// the toolchain-embedded vcs.revision (short) / vcs.time — so a build path that omits the
// ldflags stamp still names its commit — and to "unpinned" when the binary carries no VCS
// information either (`go run`, -buildvcs=false). Both are echoed in every audit record and
// in each tool's --version output. IsPinned is NOT affected: it keys on the link-time stamp
// alone, so an unstamped build still warns.
func Version() (sourceSHA, builtAt string) {
	s := perfrec.RevisionFrom(SourceSHA, readBuildInfo)
	b := perfrec.BuildTimeFrom(BuiltAt, readBuildInfo)
	if s == "" {
		s = unpinned
	}
	if b == "" {
		b = unpinned
	}
	return s, b
}

// ReleaseTagOrDev returns the stamped release tag, or "dev" for an unstamped
// build. Callers print this on `--version` alongside SourceSHA/BuiltAt so a
// running desk-tools binary can be mapped back to its `desk-tools/vX.Y.Z`.
func ReleaseTagOrDev() string {
	if ReleaseTag == "" {
		return devRelease
	}
	return ReleaseTag
}

// IsPinned reports whether the binary was stamped (installed via `sudo make
// desk-install`) rather than run from source. It keys on SourceSHA/BuiltAt
// only — the ReleaseTag stamp is additive and does NOT change which builds are
// pinned, so every build that reported IsPinned() before this stamp still does.
func IsPinned() bool { return SourceSHA != "" && BuiltAt != "" }

// WarnIfUnpinned writes a one-line WARNING to w when the binary is unpinned, so an
// operator sees drift risk in the transcript. It is a no-op for a stamped binary.
func WarnIfUnpinned(w io.Writer) {
	if IsPinned() {
		return
	}
	fmt.Fprintln(w, "desk-tools WARNING: running UNPINNED (go run / unstamped build) — "+
		"sourceSHA/builtAt not embedded, drift is invisible. Install via `sudo make desk-install`.")
}
