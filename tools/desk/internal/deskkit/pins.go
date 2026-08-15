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
	"strings"
)

// AssayVersionsFile is the pin file's fixed name at a consumer repo root.
const AssayVersionsFile = ".assay-versions"

// UmbrellaArtifact is the artifact name of the OPTIONAL umbrella line. `assay`
// is reserved to mean the suite (distribution/02), so it never collides with a
// per-tool artifact name. The umbrella line names a suite composition, not a
// downloadable asset, so it is the one line that carries no sha256 — see
// UmbrellaPin and the spec's "Umbrella line" subsection.
const UmbrellaArtifact = "assay"

// artifactTagPattern is the namespace grammar every pin tag must match:
// `<component>/vMAJOR.MINOR.PATCH`, no leading zeros. It is a verbatim copy of
// `deskrelease`'s manifest.go `artifactTagPattern` (distribution/02's grammar) —
// deliberately generic over the component name rather than a fixed allow-list,
// so a new artifact line (`assay-plugin/…`, a container-image line) a later
// brief adds is not wrongly rejected. It is the same string as deskrelease's
// (package-local) manifest.go pattern; TestPinTagGrammar asserts the shape.
var artifactTagPattern = regexp.MustCompile(`^([a-z][a-z0-9-]*)/v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

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
	prefix := artifact + " "
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return "", "", Unverifiable(fmt.Sprintf("malformed %s pin in %s: %q", artifact, path, line), nil)
		}
		return fields[1], fields[2], nil
	}
	return "", "", Unverifiable("no "+artifact+" pin in "+path, nil)
}

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
// grammar. It rejects, per Task 4 of distribution/04:
//   - a missing sha256 (a line short of three fields → PinMalformed);
//   - a malformed / non-namespaced tag (PinCheckedFailed);
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
					"line %d: umbrella tag %q is not <component>/vX.Y.Z", lineNo, fields[1]))
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
				"line %d: tag %q is not <component>/vX.Y.Z (no leading zeros)", lineNo, tag))
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
