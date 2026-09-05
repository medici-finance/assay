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

// loadIntake reads all intake entries from the per-entry directory.
func loadIntake(root string) ([]intakeEntry, error) {
	return parseIntakeDir(root)
}
