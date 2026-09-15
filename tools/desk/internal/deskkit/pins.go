package deskkit

// pins.go — the reader and validator for the `.assay-versions` pin file.
//
// The pin file is the single record of which release a consumer runs; it lives
// at the consumer repo root. Its published contract is
// `docs/distribution.md § The `.assay-versions` pin file`. This file is the code
// half of that contract: one generic reader (ArtifactPin) that every artifact
// line goes through, the optional umbrella line (UmbrellaPin), and the validator
// CheckPins that `deskpins --check` drives.
//
// Two load-bearing properties, both from the spec:
//
//   - TRAILING-SPACE PREFIX MATCH. A line is selected by `<artifact> ` — the
//     trailing space is the disambiguator, and it is why a lookup for
//     `desk-tools` never matches `desk-tools-linux-amd64`, and vice versa. The
//     consumers' CI selects the same way (`grep '^statusgen '`). Removing the
//     space would collapse the bare and per-platform lines into one match.
//   - FAIL-CLOSED. A missing file, an unreadable file, or a matching line with
//     fewer than three fields is Unverifiable (exit 6), never a default: "a
//     consumer that cannot read its pin cannot claim to be pinned."

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// AssayVersionsFile is the pin file's fixed name at a consumer repo root.
const AssayVersionsFile = ".assay-versions"

// UmbrellaArtifact is the artifact name of the OPTIONAL umbrella line. `assay`
// is reserved to mean the suite, so it never collides with a
// per-tool artifact name. The umbrella line names a suite composition, not a
// downloadable asset, so it is the one line that carries no sha256 — see
// UmbrellaPin and the spec's "Umbrella line" subsection.
const UmbrellaArtifact = "assay"

// artifactTagPattern is the grammar every pin tag must match. It accepts BOTH
// tag shapes a `.assay-versions` line can legitimately carry:
//
//   - a plain `vMAJOR.MINOR.PATCH` repo tag — the shape the public `assay`
//     release home actually cuts (`v0.9.0`, `v0.9.1`, …). A 2026-08-15 ruling
//     established that `medici-finance/assay` publishes plain, repo-level
//     `vX.Y.Z` tags, NOT a component-prefixed `assay/vX` or per-tool
//     `statusgen/vX` — see that
//     repo's `release.yml` (`tags: ["v*"]`, "plain repo-level semver tags") and
//     the version-scheme spec. A consumer pinning against the real
//     downloadable `assay` release therefore writes `statusgen v0.9.1 <sha>`, and
//     the pin's artifact NAME (field 1) already carries the component — the tag
//     does not need to repeat it.
//   - a legacy `<component>/vMAJOR.MINOR.PATCH` tag — the per-tool release scheme
//     the source repo still cuts (`statusgen/v0.8.2`, `desk-tools/v0.2.6`,
//     `daily-harvest/v0.1.0`) and the umbrella `assay/vX.Y.Z` form. Kept accepted
//     so existing consumers are not broken mid-migration; the component prefix is
//     generic over the name (never a fixed allow-list) so a new artifact line
//     a later brief adds is not wrongly rejected.
//
// The `<component>/` prefix is therefore OPTIONAL. The character class is NOT
// loosened ("add namespaces, keep the character class"): both
// forms are still STRICT semver — no leading zeros, exactly three dotted numbers.
//
// This deliberately DIVERGES from `deskrelease`'s manifest.go `artifactTagPattern`,
// which stays `<component>/vX.Y.Z`-only: that pattern screens what tags `deskrelease`
// may CUT from this repo (still the per-tool scheme), a different concern from what a
// consumer PIN may reference. TestPinTagGrammar asserts the shape.
var artifactTagPattern = regexp.MustCompile(`^([a-z][a-z0-9-]*/)?v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// sha256Pattern / commitShaPattern are the two content-address shapes field 3
// can take: a 64-hex sha256 of a published release asset for a binary line, or
// a 40-hex git commit sha for a `<artifact>-source` line (an immutable id the
// tag cannot be re-pointed away from — same role, different object).
var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// stripComment removes an inline `# …` comment (and any surrounding space)
// from a pin line, returning the payload. A pin's tag and digest never contain
// `#`, so the first `#` unambiguously starts the comment. This is how the
// validator counts FIELDS without a trailing comment inflating the count; the
// reader (ArtifactPin) does not need it, because it only reads fields[1..2].
func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		line = line[:i]
	}
	return strings.TrimSpace(line)
}

// ArtifactPin returns the tag and field-3 value (sha256, or a commit sha for a
// `-source` line) pinned for artifact in the repo root's `.assay-versions`.
//
// Selection is the trailing-space prefix match: a lookup for `desk-tools` will
// not match a `desk-tools-linux-amd64` line. Fail-closed: an unreadable file, no
// matching line, or a matching line with fewer than three fields is Unverifiable
// (exit 6) — never a default.
func ArtifactPin(root, artifact string) (tag, sha string, err error) {
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", "", Unverifiable("cannot read "+path+" (the "+artifact+" pin)", rerr)
	}
	tag, sha, found, err := lookupPin(raw, path, artifact)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", Unverifiable("no "+artifact+" pin in "+path, nil)
	}
	return tag, sha, nil
}

// lookupPin is the one selector every reader goes through: the trailing-space
// prefix match over raw, returning (tag, field3, true, nil) for the first matching
// line, (‑, ‑, false, nil) when NO line matches, and a fail-closed Unverifiable when
// the matching line is malformed (fewer than three fields). Keeping "absent" and
// "malformed" distinct here is what lets a fallback reader (PlatformPin) fall back
// on absence only — a malformed line is never skipped over in search of a better one.
func lookupPin(raw []byte, path, artifact string) (tag, sha string, found bool, err error) {
	prefix := artifact + " "
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return "", "", true, Unverifiable(fmt.Sprintf("malformed %s pin in %s: %q", artifact, path, line), nil)
		}
		return fields[1], fields[2], true, nil
	}
	return "", "", false, nil
}

// HostPlatformAssets returns the per-platform pin-line names artifact can carry for
// THIS host, most specific first: `<artifact>-<GOOS>-<GOARCH>` everywhere, preceded
// on Windows by the `.exe` form (`statusgen-windows-amd64.exe`) that a single-binary
// Windows asset is pinned under (docs/adopting-assay.md § Pin the Windows assets).
// The tarball artifacts (`desk-tools-<platform>`) are pinned without the `.tar.gz`
// suffix on every platform, which is why the bare `-<os>-<arch>` form is always in
// the list.
func HostPlatformAssets(artifact string) []string {
	base := artifact + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		return []string{base + ".exe", base}
	}
	return []string{base}
}

// PlatformPin reads artifact's pin with a HOST-PLATFORM FALLBACK: the bare
// `<artifact> ` line is preferred and read exactly as ArtifactPin reads it; only
// when that line is ABSENT does the reader try the host platform's own line
// (HostPlatformAssets — `statusgen-<GOOS>-<GOARCH>`, `.exe` first on Windows).
//
// This is the reader the desk tools use for the artifact THEY run on this host:
// an adopter's pin file written per docs/adopting-assay.md § install-statusgen
// carries only `statusgen-<platform>` lines (the install and CI paths select by
// platform), so a bare-only reader refused "no statusgen pin" on a file the
// install path itself accepts. The bare line stays authoritative when present.
//
// Fail-closed exactly as ArtifactPin: an unreadable file is Unverifiable; a
// MALFORMED bare line is Unverifiable and is NOT skipped in favour of a platform
// line (a broken record is a refusal, never a hint to look elsewhere); a malformed
// platform line likewise refuses; and a file with neither the bare line nor any
// host-platform line refuses, naming every line it looked for.
func PlatformPin(root, artifact string) (tag, sha string, err error) {
	tag, sha, found, err := PlatformPinLookup(root, artifact)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", Unverifiable(fmt.Sprintf(
			"no %s pin in %s (looked for a bare `%s ` line, then this host's platform line %s)",
			artifact, filepath.Join(root, AssayVersionsFile), artifact,
			strings.Join(HostPlatformAssets(artifact), " / ")), nil)
	}
	return tag, sha, nil
}

// PlatformPinLookup is PlatformPin's THREE-STATE core, and the reader to call when
// "the release line is absent" and "the release line is malformed" must be told
// apart — a caller that has a further pin shape to try (a `-source` channel-D
// line, say) may fall through on ABSENCE only. Collapsing the two is what would
// let a broken release line be silently skipped in favour of another line, which
// pins.go's fail-closed rule forbids.
//
//   - (tag, sha, true, nil)  — the bare line, else this host's platform line.
//   - ("", "", false, nil)   — NO line of either shape is present. Not an error;
//     the caller decides what absence means.
//   - ("", "", false, err)   — fail-closed: unreadable file, or a matching line
//     with fewer than three fields. A malformed bare line is NOT skipped in
//     favour of a platform line.
func PlatformPinLookup(root, artifact string) (tag, sha string, found bool, err error) {
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", "", false, Unverifiable("cannot read "+path+" (the "+artifact+" pin)", rerr)
	}
	tag, sha, found, err = lookupPin(raw, path, artifact)
	if err != nil {
		return "", "", false, err
	}
	if found {
		return tag, sha, true, nil
	}
	for _, name := range HostPlatformAssets(artifact) {
		tag, sha, found, err := lookupPin(raw, path, name)
		if err != nil {
			return "", "", false, err
		}
		if found {
			return tag, sha, true, nil
		}
	}
	return "", "", false, nil
}

// SourcePinSuffix is the artifact-name suffix of a SOURCE-CHANNEL ("channel D")
// pin line — `statusgen-source`, `desk-tools-source`. Such a line pins the COMMIT
// a consumer builds the tool from, in place of the sha256 of a published release
// asset, and it is what an adopter writes when no release binary is published for
// the platform or forge they run on. It is a real pin: a pin file that carries
// only source lines is PINNED, not unpinned.
const SourcePinSuffix = "-source"

// SourcePinRef is one resolved `<artifact>-source` line. Both columns are kept
// raw alongside the interpretation so a caller reporting a refusal can quote what
// it actually read rather than describing it.
type SourcePinRef struct {
	Artifact string // the line's own artifact name, e.g. `statusgen-source`
	Field2   string // column 2 as written
	Field3   string // column 3 as written
	Tag      string // field 2 when it is a release tag, else ""
	Commit   string // the 40-hex commit from whichever column carries it, else ""
}

// Ref is the identity this source pin names: its release tag when it carries one
// (comparable against a stamped binary's `--version`), else its commit. "" when
// the line carries neither — see Usable.
func (r SourcePinRef) Ref() string {
	if r.Tag != "" {
		return r.Tag
	}
	return r.Commit
}

// Usable reports whether the line identifies anything at all. A source line whose
// columns hold neither a tag nor a 40-hex commit pins nothing a caller can act on;
// it is a NAMED refusal ("this is a source pin, and it is unreadable"), never the
// same verdict as an absent pin.
func (r SourcePinRef) Usable() bool { return r.Ref() != "" }

// SourcePin reads the `<artifact>-source` line from root's `.assay-versions`. It
// goes through the SAME selector every other reader uses (lookupPin's
// trailing-space prefix match), so `statusgen-source ` never matches
// `statusgen-source-notes`, and it has the same three states as PlatformPinLookup:
// found, absent (not an error), fail-closed on unreadable/malformed.
//
// TWO LEGITIMATE COLUMN LAYOUTS. A `-source` line carries its 40-hex commit in
// EITHER column, and both shapes are in the field:
//
//   - `<artifact>-source <tag> <40-hex-commit>` — commit in field 3, the shape
//     CheckPins' `-source` rule describes;
//   - `<artifact>-source <40-hex-commit> channel-D` — commit in field 2 with a
//     literal channel marker in field 3, the shape a channel-D adopter's pin file
//     (and `desksourceguard`) actually writes.
//
// Field 3 is preferred when it is a full commit, then field 2; field 2 is read as
// the Tag when it matches the pin-tag grammar. Interpretation lives HERE, once, so
// every reader of a source line agrees on what one says.
func SourcePin(root, artifact string) (ref SourcePinRef, found bool, err error) {
	name := artifact + SourcePinSuffix
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return SourcePinRef{}, false, Unverifiable("cannot read "+path+" (the "+name+" pin)", rerr)
	}
	field2, field3, hit, lerr := lookupPin(raw, path, name)
	if lerr != nil {
		return SourcePinRef{}, false, lerr
	}
	if !hit {
		return SourcePinRef{}, false, nil
	}
	ref = SourcePinRef{Artifact: name, Field2: field2, Field3: field3}
	switch {
	case commitPattern.MatchString(field3):
		ref.Commit = field3
	case commitPattern.MatchString(field2):
		ref.Commit = field2
	}
	if artifactTagPattern.MatchString(field2) {
		ref.Tag = field2
	}
	return ref, true, nil
}

// goosAlternation is the set of operating-system tokens a platform suffix can
// carry — Go's GOOS names (the release cross-compiles with GOOS/GOARCH, so an asset
// name's os segment is always one of these). Anchoring the os segment to this set
// is what keeps a bare artifact name that happens to contain two dashes
// (`desk-tools-source`, `daily-harvest-notes`) from being read as
// `<component>-<os>-<arch>`.
const goosAlternation = `(aix|android|darwin|dragonfly|freebsd|illumos|ios|js|linux|netbsd|openbsd|plan9|solaris|wasip1|windows)`

// platformLinePattern matches a per-platform pin-line name for component:
// `<component>-<os>-<arch>` with an optional `.exe` (Windows single binary) or
// `.tar.gz` (a tarball pinned under its full asset name) suffix. It requires BOTH
// a known-os and an arch segment, so `desk-tools-source` is never mistaken for a
// platform line of `desk-tools`.
func platformLinePattern(component string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(component) + `-` + goosAlternation + `-[a-z0-9]+(\.exe|\.tar\.gz)?$`)
}

// ComponentPinTag resolves the tag the pin file records for a COMPONENT across
// every line shape that component can be pinned under: the bare `<component>`
// line when present, else the set of `<component>-<os>-<arch>[.exe|.tar.gz]`
// platform lines. It is the reader the version marker cross-checks a composition
// against, where the question is "which tag is this component on", not "which
// digest does THIS host verify" — so, unlike PlatformPin, it reads every platform
// line, not only the host's.
//
// Three outcomes, kept distinct:
//   - (tag, true, nil)   — one tag; every line that pins the component agrees on it.
//   - ("", false, nil)   — NO line of any shape pins the component. Not an error:
//     the caller decides whether an absent component is a disagreement (a
//     hand-authored manifest that names it) or simply not installed here (a
//     composition derived from everything the release ships).
//   - ("", false, err)   — fail-closed: the file is unreadable, a line is malformed,
//     or the platform lines DISAGREE on the tag (a half-moved bump); the error
//     names the lines.
func ComponentPinTag(root, component string) (tag string, present bool, err error) {
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", false, Unverifiable("cannot read "+path+" (the "+component+" pin)", rerr)
	}
	tag, _, found, err := lookupPin(raw, path, component)
	if err != nil {
		return "", false, err
	}
	if found {
		return tag, true, nil
	}
	pat := platformLinePattern(component)
	tags := map[string][]string{} // tag -> line names carrying it
	var order []string
	for _, line := range strings.Split(string(raw), "\n") {
		payload := stripComment(strings.TrimSpace(line))
		if payload == "" {
			continue
		}
		fields := strings.Fields(payload)
		if !pat.MatchString(fields[0]) {
			continue
		}
		if len(fields) < 3 {
			return "", false, Unverifiable(fmt.Sprintf("malformed %s pin in %s: %q", fields[0], path, payload), nil)
		}
		if _, seen := tags[fields[1]]; !seen {
			order = append(order, fields[1])
		}
		tags[fields[1]] = append(tags[fields[1]], fields[0])
	}
	switch len(order) {
	case 0:
		return "", false, nil
	case 1:
		return order[0], true, nil
	}
	var parts []string
	for _, t := range order {
		parts = append(parts, fmt.Sprintf("%s -> %s", strings.Join(tags[t], ","), t))
	}
	return "", false, Unverifiable(fmt.Sprintf(
		"%s platform lines in %s disagree on the tag (%s)", component, path, strings.Join(parts, "; ")), nil)
}

// ComponentOf returns the component name a pin-line name belongs to: the name
// itself for a bare line (`statusgen`, `desk-tools-source`), or the prefix before
// the `-<os>-<arch>[.exe|.tar.gz]` suffix of a platform line
// (`statusgen-windows-amd64.exe` -> `statusgen`, `desk-tools-linux-amd64` ->
// `desk-tools`). A name with no platform suffix is returned unchanged.
func ComponentOf(name string) string {
	trimmed := strings.TrimSuffix(strings.TrimSuffix(name, ".tar.gz"), ".exe")
	if m := platformSuffix.FindStringSubmatch(trimmed); m != nil {
		return m[1]
	}
	return name
}

// platformSuffix captures `<component>` off a `<component>-<os>-<arch>` name: the
// os segment must be a Go GOOS name (goosAlternation), the arch segment the
// lowercase alphanumerics GOARCH uses.
var platformSuffix = regexp.MustCompile(`^([a-z][a-z0-9-]*?)-` + goosAlternation + `-[a-z0-9]+$`)

// UmbrellaPin reads the OPTIONAL umbrella line (`assay <assay/vX.Y.Z>`) from the
// repo root's `.assay-versions`. It has THREE outcomes, and the middle one is
// the point:
//
//   - present:    (tag, true, nil)   — the consumer records a suite version.
//   - no line:    ("", false, nil)   — a DISTINCT third state, not an error and
//     not a default. A pin file with no umbrella line is valid (the live
//     consumer's is exactly this), so "no umbrella pin" must be reportable
//     without failing.
//   - unreadable/malformed: ("", false, err) — the file could not be read, or an
//     `assay ` line is present but carries no tag. Fail-closed, like ArtifactPin.
//
// The umbrella line is the one line with no sha256: it names a composition, not
// an asset (see AssayVersionsFile spec). So it needs only two fields.
func UmbrellaPin(root string) (tag string, present bool, err error) {
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", false, Unverifiable("cannot read "+path+" (the umbrella pin)", rerr)
	}
	prefix := UmbrellaArtifact + " "
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", false, Unverifiable(fmt.Sprintf("malformed umbrella pin in %s: %q", path, line), nil)
		}
		return fields[1], true, nil
	}
	return "", false, nil // no umbrella line — the valid third state
}

// PinCheckState is the validator's three-state result. It is a positive
// instrument: a file it could not read is could-not-check, NEVER checked-clean.
type PinCheckState int

const (
	// PinCheckedClean — every artifact line parses and satisfies the grammar.
	PinCheckedClean PinCheckState = iota
	// PinCheckedFailed — the file was read and parsed, but a line VIOLATES a
	// rule: a bad tag, a bad/short digest, or a duplicate artifact name.
	PinCheckedFailed
	// PinMalformed — a data line does not even have the required field count
	// (fewer than three fields for a normal artifact; the umbrella line short).
	// Distinct from PinCheckedFailed: nothing could be checked on that line.
	PinMalformed
	// PinCouldNotCheck — the file is absent or unreadable. Fail-closed.
	PinCouldNotCheck
)

func (s PinCheckState) String() string {
	switch s {
	case PinCheckedClean:
		return "checked-clean"
	case PinCheckedFailed:
		return "checked-failed"
	case PinMalformed:
		return "malformed"
	case PinCouldNotCheck:
		return "could-not-check"
	default:
		return "unknown"
	}
}

// PinCheckResult carries the validator's verdict, a human-readable summary, and
// the per-line failure reasons (empty on a clean file).
type PinCheckResult struct {
	State         PinCheckState
	Summary       string
	Reasons       []string
	ArtifactLines int // artifact (non-comment) lines seen
	CommentLines  int
}

// CheckPins validates the pin file at root/.assay-versions against the published
// grammar. It rejects, per the pin-file spec:
//   - a missing sha256 (a line short of three fields → PinMalformed);
//   - a malformed tag — neither a plain `vX.Y.Z` nor `<component>/vX.Y.Z`, or
//     with a leading zero / loose semver (PinCheckedFailed);
//   - a field-3 that is neither a 64-hex sha256 (binary line) nor, for a
//     `-source` line, a 40-hex commit sha (PinCheckedFailed);
//   - a DUPLICATE artifact name (PinCheckedFailed).
//
// THE NOT-A-DUPLICATE TRAP: `statusgen` and `statusgen-linux-amd64` legitimately
// carry an identical tag AND an identical sha256 in the live file — they are
// distinct artifact NAMES, not a duplicate line. The duplicate rule therefore
// keys on field 1 (the artifact name) and nothing else, never on the (tag, hash)
// pair.
func CheckPins(root string) PinCheckResult {
	path := filepath.Join(root, AssayVersionsFile)
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return PinCheckResult{
			State:   PinCouldNotCheck,
			Summary: "could-not-check: cannot read " + path + ": " + rerr.Error(),
		}
	}

	var reasons []string
	seen := map[string]int{} // artifact name -> first line number
	malformedCount, semanticCount := 0, 0
	artifacts, comments := 0, 0

	// semantic records a rule violation on a line whose field COUNT was fine
	// (bad tag, bad digest, duplicate name) — a checked-failed reason.
	semantic := func(msg string) { reasons = append(reasons, msg); semanticCount++ }
	// malformed records a field-COUNT problem — a line that does not parse as a
	// pin at all — kept distinct so `absent` and `malformed` are separable.
	malformed := func(msg string) { reasons = append(reasons, msg); malformedCount++ }

	for i, rawLine := range strings.Split(string(raw), "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			comments++
			continue
		}
		payload := stripComment(trimmed)
		if payload == "" {
			comments++ // a line that is only a trailing comment
			continue
		}
		fields := strings.Fields(payload)
		artifacts++
		name := fields[0]

		// Duplicate name — keyed on the NAME alone (the not-a-duplicate trap).
		if first, dup := seen[name]; dup {
			semantic(fmt.Sprintf(
				"line %d: duplicate artifact %q (first seen line %d)", lineNo, name, first))
		} else {
			seen[name] = lineNo
		}

		if name == UmbrellaArtifact {
			// Umbrella line: `assay <assay/vX.Y.Z>` — two fields, no digest.
			if len(fields) != 2 {
				malformed(fmt.Sprintf(
					"line %d: umbrella line must be `assay <tag>` (2 fields), got %d", lineNo, len(fields)))
				continue
			}
			if !artifactTagPattern.MatchString(fields[1]) {
				semantic(fmt.Sprintf(
					"line %d: umbrella tag %q is not vX.Y.Z (or <component>/vX.Y.Z)", lineNo, fields[1]))
			}
			continue
		}

		// Every other artifact line: `<artifact> <tag> <field3>`.
		if len(fields) < 3 {
			malformed(fmt.Sprintf(
				"line %d: %q has %d field(s), want <artifact> <tag> <sha256> (missing sha256)", lineNo, name, len(fields)))
			continue
		}
		if len(fields) > 3 {
			malformed(fmt.Sprintf(
				"line %d: %q has %d fields after the comment is stripped, want exactly 3", lineNo, name, len(fields)))
			continue
		}
		tag, field3 := fields[1], fields[2]
		if !artifactTagPattern.MatchString(tag) {
			semantic(fmt.Sprintf(
				"line %d: tag %q is not vX.Y.Z or <component>/vX.Y.Z (no leading zeros)", lineNo, tag))
		}
		if strings.HasSuffix(name, "-source") {
			if !commitPattern.MatchString(field3) {
				semantic(fmt.Sprintf(
					"line %d: %q source pin %q is not a 40-hex commit sha", lineNo, name, field3))
			}
		} else if !sha256Pattern.MatchString(field3) {
			semantic(fmt.Sprintf(
				"line %d: %q digest %q is not a 64-hex sha256", lineNo, name, field3))
		}
	}

	res := PinCheckResult{ArtifactLines: artifacts, CommentLines: comments, Reasons: reasons}
	switch {
	case len(reasons) == 0:
		res.State = PinCheckedClean
		res.Summary = fmt.Sprintf("checked-clean: %d artifact line(s), %d comment line(s) in %s",
			artifacts, comments, path)
	case semanticCount == 0 && malformedCount > 0:
		// Every failure is a field-count problem: report the distinct malformed
		// state so `absent` (could-not-check) and `malformed` are separable.
		res.State = PinMalformed
		res.Summary = fmt.Sprintf("malformed: %d line(s) do not parse as a pin in %s", malformedCount, path)
	default:
		// Any semantic violation (bad tag/digest/duplicate) — the stronger claim
		// even when a malformed line is also present.
		res.State = PinCheckedFailed
		res.Summary = fmt.Sprintf("checked-failed: %d rule violation(s) in %s", len(reasons), path)
	}
	return res
}
