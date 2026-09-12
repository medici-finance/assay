package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var findingHeadRe = regexp.MustCompile(`^## (F-\d+) — (\d{4}-\d{2}-\d{2}) — (.+)$`)

// intakeHeadRe matches an entry heading in the monolithic docs/streams/INTAKE.md
// view. Both `##` (the ordinary entries) and `###` (the decision-queue section)
// heading depths are accepted, matching generateIntakeView's own output. The id
// is the letter-prefixed slug form (`I-<slug>`) or the frozen numeric legacy
// form (`I-01`).
var intakeHeadRe = regexp.MustCompile(`^#{2,3} (I-[A-Za-z0-9-]+) — (\d{4}-\d{2}-\d{2}) — (.+)$`)

// allowEmptyRoot is the fail-closed opt-in for a root whose docs/streams
// exists and reads cleanly but resolves to ZERO streams. Wired
// once in main() from --allow-empty-root, before any run().
//
// Default false: emptyRootMessage becomes a hard PROBLEM, the same
// classification as the three adjacent cases it sits next to — a missing
// docs/streams, an unreadable one, and a nonexistent root — which already
// fail closed via the os.ReadDir error below. Without this, "the path is
// wrong/typo'd/mid-restructure and nothing was actually checked" is silently
// indistinguishable from "this root is fine", which is exactly the failure
// mode multi-root exists to prevent (a repo's contribution to the board
// vanishing without a trace).
//
// When true, a genuinely-empty root (adopted the methodology, has not
// authored a stream yet) still contributes — but as a NOTICE, never silence,
// so the state stays visible instead of reading as a clean pass.
var allowEmptyRoot bool

// emptyRootMessage is the diagnostic for a root whose docs/streams loaded
// with no error but produced zero streams. Its severity (PROBLEM vs NOTICE)
// is decided by the caller based on allowEmptyRoot.
func emptyRootMessage(root string) string {
	return fmt.Sprintf(
		"%s: exists and is readable but resolves to 0 streams — a typo'd, renamed, or mid-restructure docs/streams is indistinguishable from a legitimately empty bootstrap root without this diagnostic; pass --allow-empty-root if this root genuinely has none authored yet",
		filepath.Join(root, "docs", "streams"))
}

// reservedRegisterNames are directory names under docs/streams that are
// registers (not streams) and must be skipped by stream discovery.
//
// The names are the register entry directories the register spec fixes
// (spec/registers-v1.md §2.1) — they are not a convention this file invents.
// A register holds per-entry files; it has no README with a brief status table
// and no waves, so walking one as a stream produces the fabricated complaint
// "stream directory <register> has no README.md" about a directory that is
// working exactly as specified. Adding a README to silence that would be worse:
// it would make the tool's correctness depend on a file the spec never asks for,
// and the next register would hit the same wall.
//
// Skipping is not the same as ignoring. Each register is read by its own parser,
// and content a register directory holds that its parser does not recognise is
// reported there (requirements.go's requirementRegisterStrays) rather than left
// invisible by this skip.
var reservedRegisterNames = map[string]bool{
	"intake":            true,
	"findings":          true,
	requirementsDirName: true,
	decisionsDirName:    true,
}

// selfDeclaredRegisterRe matches the canonical self-declaration a register's
// README carries per spec/registers-v1.md §7 — the DECISIONS register README
// states "It is a register, not a stream — stream discovery skips it." A
// directory whose README makes this declaration is a register index, not a
// stream board.
//
// It complements reservedRegisterNames above: the name set skips the registers
// the spec fixes by directory name (findings/intake/requirements/decisions),
// while this marker skips a self-declaring register directory whose name is NOT
// in that set — a register the spec adds later, or a house-local one — so stream
// discovery degrades to a SKIP rather than aborting the whole --next-up /
// --consumers run on the register's frontmatter-free README (issue #616). The
// anchor "register, not a stream" is specific enough not to match an ordinary
// stream README that merely uses the words "not a stream" in prose.
var selfDeclaredRegisterRe = regexp.MustCompile(`(?i)register,\s+not\s+a\s+stream`)

// isSelfDeclaredRegisterREADME reports whether the README at path declares its
// directory a register per spec/registers-v1.md §7. A register README
// legitimately carries no `--- … ---` frontmatter, so it is consulted ONLY when
// a README has already failed stream-frontmatter parsing — it can therefore
// never override a well-formed stream README.
//
// An UNREADABLE README returns false (three-state discipline: a permission/I-O
// failure is could-not-check, never rounded to a silent register skip — the
// caller then surfaces the original parse error).
func isSelfDeclaredRegisterREADME(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return selfDeclaredRegisterRe.Match(raw)
}

// parseFindings reads findings from the docs/streams/findings/ per-entry
// directory. Returns nil, nil when the directory does not exist (empty register).
func parseFindings(path string) ([]Finding, error) {
	// Accept both old file-path form (backward compat during migration) and
	// the new directory form. When the path ends in ".md", read the old
	// single-file register. Otherwise treat it as the repo root and read
	// from the per-entry directory.
	if strings.HasSuffix(path, ".md") {
		return parseFindingsLegacy(path)
	}
	// New per-entry directory form.
	entries, err := parseFindingsDir(path)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, nil
	}
	findings := make([]Finding, len(entries))
	for i, e := range entries {
		findings[i] = Finding{
			ID:           e.ID,
			Date:         e.Date,
			Title:        e.Title,
			Affects:      e.Affects,
			Ack:          e.Ack,
			Resolved:     e.Resolved,
			ParkedUntil:  e.ParkedUntil,
			ParkedBy:     e.ParkedBy,
			ParkedReason: e.ParkedReason,
			Class:        e.Class,
			Control:      e.Control,
		}
	}
	return findings, nil
}

// parseFindingsLegacy reads from the old single-file FINDINGS.md register.
// Kept during the migration transition.
//
// Three-state read (docs/three-state-instrument-rule.md, sub-rule 1): an ABSENT
// file (os.IsNotExist) is a legitimate empty and returns (nil, nil); an
// UNREADABLE file (any other error) returns a non-nil error rather than an empty
// result, so a permission/I/O failure surfaces as could-not-check instead of
// being rendered as a clean "no findings" read. The two branches must stay
// separate.
func parseFindingsLegacy(path string) ([]Finding, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var findings []Finding
	var cur *Finding
	flush := func() {
		if cur != nil {
			findings = append(findings, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := findingHeadRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Finding{ID: m[1], Date: m[2], Title: m[3]}
			continue
		}
		if cur == nil {
			continue
		}
		if v, ok := strings.CutPrefix(line, "Affects:"); ok {
			for _, a := range strings.Split(v, ",") {
				if s := strings.TrimSpace(a); s != "" {
					cur.Affects = append(cur.Affects, s)
				}
			}
		}
		if v, ok := strings.CutPrefix(line, "Ack:"); ok {
			cur.Ack = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(line, "Resolved:"); ok {
			trimmed := strings.TrimSpace(v)
			firstWord := strings.SplitN(trimmed, " ", 2)[0]
			cur.Resolved = firstWord == "yes" || firstWord == "true"
		}
	}
	flush()
	return findings, nil
}

func loadStreams(root string) ([]*Stream, []Finding, error) {
	streamsDir := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", streamsDir, err)
	}
	var streams []*Stream
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Skip reserved register directories — they are not streams.
		if reservedRegisterNames[e.Name()] {
			continue
		}
		readme := filepath.Join(streamsDir, e.Name(), "README.md")
		if _, err := os.Stat(readme); os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("stream directory %s has no README.md", e.Name())
		}
		s, err := parseStreamREADME(readme)
		if err != nil {
			// A README that declares itself a register (spec/registers-v1.md §7)
			// is a register index, not a malformed stream: its "no frontmatter"
			// is the EXPECTED shape for a register, not an error. Skip it so
			// stream discovery does not abort the whole --next-up/--consumers run
			// on a register README whose directory name is not in
			// reservedRegisterNames (issue #616).
			if isSelfDeclaredRegisterREADME(readme) {
				continue
			}
			return nil, nil, err
		}
		// Stream→root tagging: the root a stream was
		// discovered under is what routes it to its own STATUS.md, its own
		// registers, and its own historian in a multi-root run.
		s.Root = root
		streams = append(streams, s)
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].Name < streams[j].Name })

	// Load findings from the per-entry directory (new format).
	findings, err := parseFindings(root)
	if err != nil {
		return nil, nil, err
	}
	return streams, findings, nil
}

// loadArchivedStreams reads the streams that have been moved off the active
// board into docs/archive/<stream>/ (the whole-stream archival act described in
// streamarchive.go; checks.go rejects a `done` stream still under docs/streams/).
//
// WHY IT EXISTS. loadStreams walks ONLY docs/streams/, so an archived stream is
// absent from the active stream set by design — it must never reappear on the
// board, in the per-stream checks, or in the append-only history. But an archived
// stream is still real, completed work: a brief may legitimately `depends:` on a
// done brief, and an OPEN finding may still `affects:` a completed stream. Those
// inbound edges must stay resolvable after the referenced stream is archived —
// otherwise archiving a referenced stream silently converts every valid inbound
// edge into a hard `references unknown stream` PROBLEM (rc=1). The returned
// streams join ONLY the edge-resolution universe (the allStreams argument of
// checkScoped and checkBriefFiles), so docs/archive/<stream> resolves for an edge
// EXACTLY as docs/streams/<stream> does — same README, same brief table, so a
// per-brief ref (<stream>/<NN>) resolves too, not just the bare stream name.
//
// THE BOUNDARY IS PRESERVED. A genuinely-unknown stream — present under neither
// docs/streams/ nor docs/archive/ — is still absent from this set and still
// PROBLEMs. This function only ADDS known targets; it suppresses nothing.
//
// THREE-STATE READ (docs/three-state-instrument-rule.md). An ABSENT docs/archive/
// directory is a legitimate empty (a repo that has never archived a stream) and
// returns (nil, nil). An UNREADABLE directory returns a non-nil error — a
// permission/I-O failure is could-not-check, never rounded to "no archived
// streams". A subdirectory without a README.md is not a stream (docs/archive/ may
// hold other completed artifacts) and is skipped rather than hard-erroring the run
// — unlike docs/streams/, where a missing README is a malformed ACTIVE stream. A
// README that fails to parse is likewise skipped: archived content is frozen and a
// single malformed archived README must not abort the whole active board; an edge
// into that one stream degrades to the pre-existing `unknown stream` PROBLEM
// (still surfaced, never rounded to a pass), which is the safe direction.
func loadArchivedStreams(root string) ([]*Stream, error) {
	archiveDir := filepath.Join(root, "docs", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", archiveDir, err)
	}
	var streams []*Stream
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Registers are never streams, archived or active.
		if reservedRegisterNames[e.Name()] {
			continue
		}
		readme := filepath.Join(archiveDir, e.Name(), "README.md")
		if _, statErr := os.Stat(readme); statErr != nil {
			continue // not a stream directory — skip, don't hard-error
		}
		s, parseErr := parseStreamREADME(readme)
		if parseErr != nil {
			continue // frozen archived content: degrade gracefully, never abort the board
		}
		s.Root = root
		s.Archived = true
		streams = append(streams, s)
	}
	sort.Slice(streams, func(i, j int) bool { return streams[i].Name < streams[j].Name })
	return streams, nil
}

// edgeResolutionUniverse returns the stream set against which cross-stream
// depends:/unblocks:/affects: edges resolve: the active streams PLUS the archived
// streams that no active stream already shadows by name. Active always wins a name
// collision (a stream mid-transition may momentarily exist under both trees), so
// the active copy — the one with the live brief table — is the one an edge
// resolves against. The active `streams` slice is never mutated: a fresh backing
// array is allocated so an append here cannot scribble into it.
func edgeResolutionUniverse(streams, archived []*Stream) []*Stream {
	universe := append([]*Stream{}, streams...)
	if len(archived) == 0 {
		return universe
	}
	activeNames := make(map[string]bool, len(streams))
	for _, s := range streams {
		activeNames[s.Name] = true
	}
	for _, a := range archived {
		if activeNames[a.Name] {
			continue
		}
		universe = append(universe, a)
	}
	return universe
}

// loadHydratedStreams is the load path every score/consumer view MUST go
// through. It loads the stream READMEs (loadStreams), attaches placeholder
// briefs (attachPlaceholders), and — the load-bearing step — hydrates each
// opted-in brief's frontmatter fields (Depends/Value/ExecTier/Gate/BlockedBy/
// Measures/Evidence) from its brief file via checkBriefFiles.
//
// Those frontmatter fields are populated ONLY as a SIDE EFFECT of
// checkBriefFiles (brieffile.go, "Wire BriefFile data into the Brief row"):
// loadStreams alone leaves Depends nil and Value "" (scored as med). A
// subcommand that hand-assembled loadStreams + attachPlaceholders and skipped
// the check therefore walked empty Depends and default Value — the gate-scores
// bug of issue #266, where --gate-scores dropped the value weight and the
// dominant unblocks term relative to the STATUS.md write path for identical
// input. Routing through one constructor closes that omission and the ~latent
// next one.
//
// checkBriefFiles' problems/notices are intentionally discarded here: a
// diagnostic/consumer view HYDRATES, it does not re-run the validation gate.
// The STATUS.md build path in run() still calls checkBriefFiles directly and
// surfaces those problems as the --lint verdict; this helper is for the paths
// that need the hydrated rows but not the validation report.
//
// The []Finding returned is loadStreams' stream/README findings (NOT
// checkBriefFiles' hydration problems, which are discarded) — a consumer view
// that also renders those findings (e.g. --roadmap health rules) takes them
// here; views that don't (--gate-scores, --launch) discard them with `_`. One
// constructor thus serves both without forcing a caller to re-open loadStreams
// and re-lose the hydration step.
func loadHydratedStreams(root string) ([]*Stream, []Finding, error) {
	streams, findings, err := loadStreams(root)
	if err != nil {
		return nil, nil, err
	}
	attachPlaceholders(streams)
	checkBriefFiles(streams, streams) // for its hydration side effect only
	return streams, findings, nil
}

// loadIntake reads all intake entries for the untriaged-intake alarm and board
// line. It reads the per-entry docs/streams/intake/ directory when that directory
// exists — the source of truth. When the per-entry directory is ABSENT it falls
// back to the monolithic docs/streams/INTAKE.md view (parseIntakeLegacy), the
// same legacy fallback the findings register has (parseFindingsLegacy), so a repo
// whose intake still lives in the single-file register is READ rather than
// silently rounded to zero. When NEITHER register exists the untriaged set is
// genuinely undetermined and this returns an error, which the caller renders as
// could-not-check.
//
// WHY THE FALLBACK LIVES HERE, not in parseIntakeDir. A missing per-entry
// directory made parseIntakeDir return (nil, nil) — an empty set — which the
// board rendered as "the front door is clear": a confident negative over a
// register that was never read, a three-state-instrument-rule violation
// (docs/three-state-instrument-rule.md, sub-rule 1). But parseIntakeDir's other
// callers — the register-view generator (generateIntakeView) and the per-entry
// integrity/tamper checks — operate specifically on the per-entry files and MUST
// keep the per-entry semantics (an absent directory is a legitimate empty for a
// view that regenerates FROM those files). Only the alarm/board path wants the
// monolithic fallback, so it is applied here and nowhere else.
func loadIntake(root string) ([]intakeEntry, error) {
	intakeDir := filepath.Join(root, "docs", "streams", "intake")
	info, err := os.Stat(intakeDir)
	switch {
	case err == nil && info.IsDir():
		// Per-entry register present — the source of truth. An EMPTY directory
		// here is a legitimate empty (the adopter uses the per-entry register and
		// has no open entries), not a could-not-check.
		return parseIntakeDir(root)
	case err == nil:
		// A non-directory at docs/streams/intake is a malformed register: surface
		// it as a read error, never a silent zero.
		return nil, fmt.Errorf("intake register unreadable: %s exists but is not a directory", intakeDir)
	case os.IsNotExist(err):
		// Per-entry directory ABSENT — fall back to the monolithic INTAKE.md view.
		return parseIntakeLegacy(filepath.Join(root, "docs", "streams", "INTAKE.md"))
	default:
		// Stat failed for a reason other than absence (permission/I-O) —
		// could-not-check, never a clean zero.
		return nil, fmt.Errorf("intake register unreadable: %s: %w", intakeDir, err)
	}
}

// parseIntakeLegacy reads intake dispositions from the monolithic
// docs/streams/INTAKE.md register — the fallback loadIntake takes when the
// per-entry docs/streams/intake/ directory is absent. It is the intake twin of
// parseFindingsLegacy: the same heading + field-line scan.
//
// Three-state read (docs/three-state-instrument-rule.md, sub-rule 1): reaching
// here already means the per-entry directory was ABSENT. If the monolithic view
// is ALSO absent (os.IsNotExist) then there is NO intake register to read at all,
// so the untriaged set is genuinely undetermined — this returns a non-nil error
// (could-not-check), never (nil, nil), because an empty result here would render
// as a clean "the front door is clear" over a register that was never read. An
// UNREADABLE file (any other error) is likewise could-not-check.
//
// Each entry's Disposition carries the raw text after "Disposition:" (matched
// case-insensitively with leading and trailing whitespace trimmed around the
// key and value, e.g. "new", "scoped → <stream>", "new — proposed …");
// isUntriagedDisposition then classifies it exactly as it does a per-entry
// file's, so the monolithic and per-entry paths count untriaged identically.
// A missing Disposition line defaults to "new" — parseIntakeFile's rule for a
// per-entry file with no disposition key.
func parseIntakeLegacy(path string) ([]intakeEntry, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("no intake register found: neither the per-entry docs/streams/intake/ directory nor the docs/streams/INTAKE.md view exists — the untriaged set cannot be determined, so this is could-not-check, not a clear front door")
	}
	if err != nil {
		return nil, err
	}
	var entries []intakeEntry
	var cur *intakeEntry
	flush := func() {
		if cur != nil {
			if strings.TrimSpace(cur.Disposition) == "" {
				cur.Disposition = "new"
			}
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if m := intakeHeadRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &intakeEntry{ID: m[1], Date: m[2], Title: m[3]}
			continue
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= len("Disposition:") && strings.EqualFold(trimmed[:len("Disposition:")], "Disposition:") {
			cur.Disposition = strings.TrimSpace(trimmed[len("Disposition:"):])
		}
	}
	flush()
	return entries, nil
}
