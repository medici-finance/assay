package main

// Per-entry register file parsing: the intake (docs/streams/intake/) and
// findings (docs/streams/findings/) directories hold one YAML-frontmatter .md
// file per entry. These types and parsers back the view generator and the
// integrity checks. (The one-off single-file → per-entry migration that
// originally produced these files lived alongside this code in migrate.go;
// it was retired once the migration completed.)

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ----- per-entry YAML frontmatter types -----

type intakeEntry struct {
	ID            string `yaml:"id"`
	Date          string `yaml:"date"`
	Title         string `yaml:"title"`
	Disposition   string `yaml:"disposition"`
	ScopedTo      string `yaml:"scoped-to,omitempty"`
	Why           string `yaml:"why,omitempty"`
	DecisionIssue string `yaml:"decision-issue,omitempty"` // GitHub issue # for the needs-decision issue
	Body          string `yaml:"-"`                        // prose after the heading, before the Disposition line
	// Subdir is the disposition-state subdirectory the entry's file lives
	// under (issue-loop/15): "new", "decision-needed", "watching",
	// "completed", "rejected", or "" for a root-level file (flat-layout
	// compat — adopter repos that have not split yet, or a not-yet-migrated
	// entry). Computed from where the file was found on disk, never read
	// from frontmatter — identity is the id, placement is a fact about the
	// filesystem, not a claim the entry makes about itself.
	Subdir string `yaml:"-"`
}

// intakeKnownSubdirs are the five triage-state subdirectories under
// docs/streams/intake/ (issue-loop/15): new (untriaged; disposition: new or
// missing), decision-needed (waiting on a human), watching, completed
// (routed out: scoped → <stream>, scoped → issue #NN, legacy adopted),
// rejected (tombstones). A directory under intake/ whose name is outside
// this set is an unknown-subdir integrity PROBLEM (registers.go), not a
// silent skip — and parseIntakeDir does not descend into it.
var intakeKnownSubdirs = []string{"new", "decision-needed", "watching", "completed", "rejected"}

// intakeSubdirForDisposition maps a disposition value to the subdirectory it
// belongs under in the split layout. ok is false for a disposition value
// with no defined mapping (an unrecognized/typo'd disposition is a separate
// quality concern the field-quality checks already cover; this mapping is
// only used to detect a dir↔disposition MISMATCH, not to validate the
// disposition value itself).
func intakeSubdirForDisposition(disposition string) (subdir string, ok bool) {
	switch strings.TrimSpace(disposition) {
	case "", "new":
		return "new", true
	case "decision-needed":
		return "decision-needed", true
	case "watching":
		return "watching", true
	case "scoped", "adopted": // adopted: legacy spelling, collapses into completed too
		return "completed", true
	case "rejected":
		return "rejected", true
	default:
		return "", false
	}
}

// intakeFileLoc locates one intake entry file on disk, tagged with its
// disposition-state subdir (empty for a root-level flat-layout file).
type intakeFileLoc struct {
	Dir    string // absolute directory containing the file
	Name   string // filename
	Subdir string // "" (root) or one of intakeKnownSubdirs
}

// listIntakeFiles enumerates every intake entry .md file under
// docs/streams/intake/ — root-level (flat-layout compat) plus each of the
// five known disposition subdirs — skipping README.md and non-.md files. It
// does NOT recurse into unknown-named directories (those are flagged
// separately as an unknown-subdir integrity PROBLEM, not silently walked).
// Used by the integrity/field-quality checks and by the tombstone-not-delete
// id-identity resolution, which both need raw file access alongside the
// parsed entries.
func listIntakeFiles(root string) []intakeFileLoc {
	base := filepath.Join(root, "docs", "streams", "intake")
	var out []intakeFileLoc
	levels := append([]string{""}, intakeKnownSubdirs...)
	for _, sub := range levels {
		dir := base
		if sub != "" {
			dir = filepath.Join(base, sub)
		}
		files, err := os.ReadDir(dir)
		if err != nil {
			continue // missing dir — not an error, just nothing at this level
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
				continue
			}
			out = append(out, intakeFileLoc{Dir: dir, Name: f.Name(), Subdir: sub})
		}
	}
	return out
}

// intakeUnreadableDirs returns a diagnostic for each intake level (the flat root
// plus each of the five known disposition subdirs) that EXISTS but could not be
// read for a reason other than absence — a permission or I/O error. An absent
// level (os.IsNotExist) is a legitimate empty and yields no diagnostic; any
// other ReadDir error does. A non-empty result is a could-not-check: the intake
// enumeration is then a floor, and a renderer reporting "0 untriaged — the front
// door is clear" over it would be reporting a clean read of a register it never
// actually read (docs/three-state-instrument-rule.md, sub-rule 1). This exists
// because listIntakeFiles deliberately swallows read errors to stay best-effort
// for the integrity checks; parseIntakeDir calls this first so the intake alarm
// path can surface the could-not-check instead.
func intakeUnreadableDirs(root string) []string {
	base := filepath.Join(root, "docs", "streams", "intake")
	var bad []string
	levels := append([]string{""}, intakeKnownSubdirs...)
	for _, sub := range levels {
		dir := base
		if sub != "" {
			dir = filepath.Join(base, sub)
		}
		if _, err := os.ReadDir(dir); err != nil && !os.IsNotExist(err) {
			bad = append(bad, fmt.Sprintf("%s: %v", dir, err))
		}
	}
	return bad
}

type findingEntry struct {
	ID       string   `yaml:"id"`
	Date     string   `yaml:"date"`
	Title    string   `yaml:"title"`
	Affects  []string `yaml:"affects"`
	Ack      string   `yaml:"ack,omitempty"`
	Resolved bool     `yaml:"resolved"`
	// Bounded shelving (statusgen/06). All three are REQUIRED together when the
	// finding is parked; the all-absent case is the common one (an open or
	// resolved finding) and stays valid. omitempty so an unparked entry
	// round-trips through the generated view unchanged.
	ParkedUntil  string `yaml:"parked-until,omitempty"`
	ParkedBy     string `yaml:"parked-by,omitempty"`
	ParkedReason string `yaml:"parked-reason,omitempty"`
	Body         string `yaml:"-"` // prose after the heading, before the metadata lines
}

// slugFromTitle produces a short, deterministic slug from a title — lowercase,
// non-letters/digits → hyphen, collapse runs. It is intentionally simple: the
// id in the file carries the canonical identity; the slug is a readability aid.
func slugFromTitle(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s := b.String()
	// collapse runs of hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	// truncate at a reasonable length
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// parseIntakeDir reads all intake entry files from docs/streams/intake/ —
// root-level files (flat-layout compat, for repos that have not split, or an
// entry not yet migrated) PLUS one level into each of the five known
// disposition-state subdirs (issue-loop/15) — sorted by date then id. Each
// entry's Subdir field records where it was found ("" for root-level).
// README.md (in any of those directories) is directory documentation, not a
// dated entry — skipped. (Under the old tolerant splitFrontmatterYAML this
// happened to parse "successfully" as an all-comment, zero-value YAML
// document; the stricter shared splitFrontmatter now correctly errors on a
// file with no "---" frontmatter fence, so a would-be-tolerated README must
// be excluded explicitly instead of relying on that parser accident.)
func parseIntakeDir(root string) ([]intakeEntry, error) {
	// Distinguish an UNREADABLE register from a genuinely-empty one BEFORE
	// enumerating: listIntakeFiles swallows a directory read error (it treats
	// every ReadDir failure as "nothing at this level"), so a permission/I/O
	// error on the intake register would otherwise parse to zero entries — a
	// could-not-check indistinguishable from a clean "front door is clear" read
	// (docs/three-state-instrument-rule.md, sub-rule 1). An ABSENT level
	// (os.IsNotExist) stays a legitimate empty; any other read error is a hard
	// could-not-check surfaced to the caller.
	if bad := intakeUnreadableDirs(root); len(bad) > 0 {
		return nil, fmt.Errorf("intake register unreadable: %s", strings.Join(bad, "; "))
	}
	var entries []intakeEntry
	for _, loc := range listIntakeFiles(root) {
		raw, err := os.ReadFile(filepath.Join(loc.Dir, loc.Name))
		if err != nil {
			return nil, fmt.Errorf("reading intake/%s: %w", intakeFileLabel(loc), err)
		}
		e, err := parseIntakeFile(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing intake/%s: %w", intakeFileLabel(loc), err)
		}
		e.Subdir = loc.Subdir
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// intakeFileLabel renders an intakeFileLoc as a subdir-qualified filename for
// error messages, e.g. "new/2026-07-08-x.md" or "2026-07-08-x.md" for a
// root-level file.
func intakeFileLabel(loc intakeFileLoc) string {
	if loc.Subdir == "" {
		return loc.Name
	}
	return loc.Subdir + "/" + loc.Name
}

// parseIntakeFile parses a single intake entry .md file (YAML frontmatter + body).
// If the frontmatter has no disposition key (Disposition == ""), it defaults to
// "new" — matching the brief's stated fact that Disposition defaults to new.
func parseIntakeFile(raw []byte) (*intakeEntry, error) {
	fm, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	var e intakeEntry
	if err := yaml.Unmarshal([]byte(fm), &e); err != nil {
		return nil, err
	}
	if e.Disposition == "" {
		e.Disposition = "new"
	}
	e.Body = strings.TrimSpace(body)
	return &e, nil
}

// parseFindingsDir reads all finding entry files from docs/streams/findings/,
// sorted by id.
//
// Three-state read (docs/three-state-instrument-rule.md, sub-rule 1): an ABSENT
// register (os.IsNotExist) is a legitimate empty and returns (nil, nil); an
// UNREADABLE register (any other ReadDir error, e.g. a permission failure)
// returns a non-nil error, which loadStreams propagates so the board fails
// closed rather than silently rendering zero findings as a clean read. The two
// branches below must stay separate — collapsing them into one `if err != nil`
// would turn an unreadable register into an empty one.
func parseFindingsDir(root string) ([]findingEntry, error) {
	dir := filepath.Join(root, "docs", "streams", "findings")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []findingEntry
	for _, f := range files {
		// README.md is directory documentation, not a dated entry — skip it
		// (see the matching comment in parseIntakeDir).
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading findings/%s: %w", f.Name(), err)
		}
		e, err := parseFindingFile(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing findings/%s: %w", f.Name(), err)
		}
		entries = append(entries, *e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// parseFindingFile parses a single finding entry .md file (YAML frontmatter + body).
func parseFindingFile(raw []byte) (*findingEntry, error) {
	fm, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, err
	}
	var e findingEntry
	if err := yaml.Unmarshal([]byte(fm), &e); err != nil {
		return nil, err
	}
	e.Body = strings.TrimSpace(body)
	return &e, nil
}
