package main

// Per-entry register file parsing: the intake (docs/streams/intake/) and
// findings (docs/streams/findings/) directories hold one YAML-frontmatter .md
// file per entry. These types and parsers back the view generator and the
// integrity checks. (The one-off single-file → per-entry migration that
// originally produced these files lived alongside this code in migrate.go;
// it was retired once the migration completed — oit#989.)

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
	DecisionIssue string `yaml:"decision-issue,omitempty"` // issue-loop/08: GitHub issue # for the needs-decision issue
	Body          string `yaml:"-"`                        // prose after the heading, before the Disposition line
}

type findingEntry struct {
	ID       string   `yaml:"id"`
	Date     string   `yaml:"date"`
	Title    string   `yaml:"title"`
	Affects  []string `yaml:"affects"`
	Ack      string   `yaml:"ack,omitempty"`
	Resolved bool     `yaml:"resolved"`
	Body     string   `yaml:"-"` // prose after the heading, before the metadata lines
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

// parseIntakeDir reads all intake entry files from docs/streams/intake/,
// sorted by id.
func parseIntakeDir(root string) ([]intakeEntry, error) {
	dir := filepath.Join(root, "docs", "streams", "intake")
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []intakeEntry
	for _, f := range files {
		// README.md is directory documentation, not a dated entry — skip it.
		// (Under the old tolerant splitFrontmatterYAML this happened to parse
		// "successfully" as an all-comment, zero-value YAML document; the
		// stricter shared splitFrontmatter now correctly errors on a file with
		// no "---" frontmatter fence, so a would-be-tolerated README must be
		// excluded explicitly instead of relying on that parser accident.)
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") || f.Name() == "README.md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading intake/%s: %w", f.Name(), err)
		}
		e, err := parseIntakeFile(raw)
		if err != nil {
			return nil, fmt.Errorf("parsing intake/%s: %w", f.Name(), err)
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
