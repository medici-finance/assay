package deskkit

// workpad.go — the ONE upserted progress comment per PR.
//
// WHY. A worker re-dispatched onto a PR starts cold: the previous worker's plan, what it
// verified and what it found lives in a scatter of comments or nowhere, and the outward-write
// budgets exist largely to police that scatter. Symphony's agents (openai/symphony,
// elixir/WORKFLOW.md) keep exactly one marked comment per issue and edit it in place — plan,
// acceptance criteria, environment stamp, notes. This file is the marker, the template and
// the Render/Parse pair; `deskreply --workpad` (cmd/deskreply) is the upsert VERB — the
// find-or-create decision, the identity filter, and the actual GitHub write live there, not
// here, so this file stays a pure text transform with no gh/git dependency.
//
// SINGLE POINT OF FAILURE: the marker match. Behind it sit two independent controls this
// file does not implement itself — the CALLER's identity filter (only a comment authored by
// the worker identity is a candidate, so a look-alike marker in a human's comment is never
// edited) and the trust gate that already refuses every write verb on an untrusted PR. This
// file's own contribution to that safety is narrow and load-bearing: HasWorkpadMarker and
// Parse both require an EXACT line match, so a marker merely MENTIONED in prose ("the
// workpad marker is `<!-- assay:workpad -->`") never reads as a real workpad.

import (
	"path/filepath"
	"strings"
)

// WorkpadMarker is the exact-match line every upserted workpad comment carries as its own
// line — never as a substring of a longer line, never inside a fenced code block that is
// merely quoting it (that distinction is the CALLER's job: this file only recognises the
// line, never where it sits). A marker is recognised ONLY when a line, trimmed of leading
// and trailing whitespace, equals this string byte-for-byte.
const WorkpadMarker = "<!-- assay:workpad -->"

// The four sections a workpad renders, in FIXED order — this is the shape a re-dispatched
// worker (or a human reviewer) learns to scan for once and finds in the same place every
// time. Parse looks for these same four header lines, exact match, to split a body back
// into sections.
const (
	workpadPlanHeader       = "## Plan"
	workpadAcceptanceHeader = "## Acceptance criteria"
	workpadValidationHeader = "## Validation"
	workpadNotesHeader      = "## Notes"
)

// workpadHeaders is workpadPlanHeader..workpadNotesHeader in RENDER order — Parse's section
// splitter and the header recogniser both walk this list rather than repeating it.
var workpadHeaders = []string{
	workpadPlanHeader, workpadAcceptanceHeader, workpadValidationHeader, workpadNotesHeader,
}

// workpadEmptySection is what Render writes for a section nobody has filled in yet, and
// what Parse folds back to "" on read — so Render(Parse(x)) is stable and a genuinely empty
// section round-trips as empty, not as this placeholder's literal text.
const workpadEmptySection = "_none yet_"

// Workpad is the parsed or about-to-be-rendered content of one upserted PR comment.
type Workpad struct {
	// Stamp is the environment stamp line — see the package-level Stamp function. It is
	// the worktree basename and the short commit sha, joined by "@": never a machine path
	// (the public-repo self-containment rule this repo already enforces on every other
	// body), and never anything a reader outside this house could not make sense of.
	Stamp string
	// Plan is a checklist (or any markdown) of what the worker intends to do.
	Plan string
	// Acceptance mirrors the brief's or issue's own Validation/Test Plan section verbatim
	// where one exists — the reviewer's yardstick, not a fresh one the worker invents.
	Acceptance string
	// Validation is commands run and their results — the running log a re-dispatched
	// worker or the reviewer reads to see what has already been checked.
	Validation string
	// Notes carries blockers, pushbacks and hand-off context — anything that does not fit
	// the other three sections.
	Notes string
}

// Stamp renders the environment stamp line for a workpad comment: the CURRENT WORKTREE's
// BASENAME — never an absolute path, per the public-repo self-containment rule every other
// body this tooling writes already has to honour — joined to the short commit sha with "@".
//
// Only the basename survives: `filepath.Base` strips every directory component, and the
// result is additionally scrubbed of any residual path separator (a defence against a
// caller passing something that is not actually a filesystem path — e.g. a login or slug
// carrying a "/" — rather than a defence against filepath.Base itself, which never leaves
// one behind for a real path). TestWorkpadStampHasNoPath pins the "never contains a path
// separator" property directly rather than trusting the implementation's reasoning about
// it.
func Stamp(worktree, sha string) string {
	base := filepath.Base(strings.TrimRight(strings.TrimSpace(worktree), "/"))
	if base == "" || base == "." || base == string(filepath.Separator) {
		base = "worktree"
	}
	base = strings.NewReplacer("/", "-", "\\", "-").Replace(base)
	short := strings.TrimSpace(sha)
	if len(short) > 8 {
		short = short[:8]
	}
	return base + "@" + short
}

// Render produces the full comment body for w: the marker line, the stamp (when set), then
// the four sections in their fixed order. An EMPTY section is still rendered with its
// header and the workpadEmptySection placeholder, not omitted — so a re-parse always sees
// the same four headers and a human editing the comment by hand always has a place to
// write, whichever section the worker left blank.
func Render(w Workpad) string {
	var b strings.Builder
	b.WriteString(WorkpadMarker)
	b.WriteString("\n")
	if stamp := strings.TrimSpace(w.Stamp); stamp != "" {
		b.WriteString(stamp)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	sections := []struct {
		header string
		body   string
	}{
		{workpadPlanHeader, w.Plan},
		{workpadAcceptanceHeader, w.Acceptance},
		{workpadValidationHeader, w.Validation},
		{workpadNotesHeader, w.Notes},
	}
	for i, s := range sections {
		b.WriteString(s.header)
		b.WriteString("\n")
		body := strings.TrimSpace(s.body)
		if body == "" {
			body = workpadEmptySection
		}
		b.WriteString(body)
		if i != len(sections)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// HasWorkpadMarker reports whether body carries the workpad marker as its OWN line — a
// line that, trimmed, equals WorkpadMarker exactly. A body that merely MENTIONS the marker
// mid-sentence, or wraps it in surrounding text on the same line, does not count: the
// exact-line-match rule is what keeps documentation and prose describing this feature from
// ever being misread as a real workpad.
func HasWorkpadMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == WorkpadMarker {
			return true
		}
	}
	return false
}

// Parse reports whether body carries the workpad marker (HasWorkpadMarker) and, when it
// does, the Workpad it decodes to: the first non-blank line after the marker that is not
// itself a section header is read as the stamp, and each of the four fixed sections is
// extracted between its own header and the next.
//
// Parse does NOT look at who authored the comment body came from — that identity check is
// the caller's (deskreply's candidate filter), never this file's, exactly as the single-
// point-of-failure note at the top of this file states.
func Parse(body string) (Workpad, bool) {
	lines := strings.Split(body, "\n")
	markerIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == WorkpadMarker {
			markerIdx = i
			break
		}
	}
	if markerIdx < 0 {
		return Workpad{}, false
	}

	w := Workpad{}
	i := markerIdx + 1
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && !isWorkpadHeaderLine(lines[i]) {
		w.Stamp = strings.TrimSpace(lines[i])
		i++
	}

	rest := strings.Join(lines[i:], "\n")
	w.Plan = extractWorkpadSection(rest, workpadPlanHeader)
	w.Acceptance = extractWorkpadSection(rest, workpadAcceptanceHeader)
	w.Validation = extractWorkpadSection(rest, workpadValidationHeader)
	w.Notes = extractWorkpadSection(rest, workpadNotesHeader)
	return w, true
}

// isWorkpadHeaderLine reports whether line, trimmed, is exactly one of the four fixed
// section headers.
func isWorkpadHeaderLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, h := range workpadHeaders {
		if trimmed == h {
			return true
		}
	}
	return false
}

// extractWorkpadSection returns the trimmed text between header's line and the next
// section header (or the end of body), or "" when header is absent or its body is the
// workpadEmptySection placeholder Render writes for a blank section.
func extractWorkpadSection(body, header string) string {
	lines := strings.Split(body, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if isWorkpadHeaderLine(lines[i]) {
			end = i
			break
		}
	}
	section := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if section == workpadEmptySection {
		return ""
	}
	return section
}

// StripWorkpadMarkerLine returns raw with every line that is an EXACT match of
// WorkpadMarker replaced by an empty line. It exists so the marker — a fixed, public,
// content-free string that will legitimately appear, quoted, in this package's own tests,
// in the worker clause kit, and in a PR body describing this feature — can never itself be
// read as a disclosure or a credential span by BodyCheck (bodycheck.go) or
// SelfContainCheck (selfcontain.go). This is the same #380 self-reference problem
// stripBenignArmorDelimiters exists for (structured.go): a guard that fires on its own
// routine mention gets routed around rather than fixed. Only the exact marker LINE is
// blanked; everything else on the surface — including the stamp line right below it and
// every section's own content — reaches both scans completely untouched.
func StripWorkpadMarkerLine(raw string) string {
	if !strings.Contains(raw, WorkpadMarker) {
		return raw
	}
	lines := strings.Split(raw, "\n")
	changed := false
	for i, line := range lines {
		if strings.TrimSpace(line) == WorkpadMarker {
			lines[i] = ""
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return strings.Join(lines, "\n")
}
