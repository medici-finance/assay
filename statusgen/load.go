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

// reservedRegisterNames are directory names under docs/streams that are
// registers (not streams) and must be skipped by stream discovery.
var reservedRegisterNames = map[string]bool{
	"intake":   true,
	"findings": true,
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
			ID:       e.ID,
			Date:     e.Date,
			Title:    e.Title,
			Affects:  e.Affects,
			Ack:      e.Ack,
			Resolved: e.Resolved,
		}
	}
	return findings, nil
}

// parseFindingsLegacy reads from the old single-file FINDINGS.md register.
// Kept during the migration transition.
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

// loadIntake reads all intake entries from the per-entry directory.
func loadIntake(root string) ([]intakeEntry, error) {
	return parseIntakeDir(root)
}
