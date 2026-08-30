package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type frontmatter struct {
	Stream        string  `yaml:"stream"`
	Status        string  `yaml:"status"`
	Priority      string  `yaml:"priority"`
	Track         string  `yaml:"track"`
	Issues        []int   `yaml:"issues"`
	External      string  `yaml:"external"`
	Tiering       *string `yaml:"tiering"`        // optional; nil when absent, non-nil (incl. "") when present.
	MaxConcurrent *int    `yaml:"max-concurrent"` // optional; nil when absent; 1..perStreamCap when present.
	Serves        string  `yaml:"serves"`         // optional; example-app | example-service | assay | platform | "" (untagged).
	Theme         string  `yaml:"theme"`          // optional render-style selector for cadenced artifacts; unmapped values render as a visible marker.
	Owner         string  `yaml:"owner"`          // optional stream owner; "" when absent — renders "—".
	Repo          string  `yaml:"repo"`           // optional owning repo, <owner>/<name>; "" when absent.
}

// splitFrontmatter is the SINGLE canonical frontmatter splitter for the whole
// tool (stream READMEs, brief files, and — in a later revision — the
// per-entry intake/findings register files, which previously had their own
// divergent byte-prefix parser). Line-based and tolerant: the opening and
// closing fences only need to trim to "---", and CRLF is normalized to LF
// up front so a Windows-authored (CRLF) file parses identically to a
// LF file instead of silently missing the "---\n" byte-prefix and having its
// entire content — fence lines included — swallowed as "frontmatter" with an
// empty body (the exact register-entry data-loss bug).
func splitFrontmatter(content string) (string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", "", fmt.Errorf("no frontmatter: first line must be ---")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n"), nil
		}
	}
	return "", "", fmt.Errorf("unterminated frontmatter")
}

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func splitRow(line string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
}

func normalizeMark(s string) string {
	switch s {
	case "—", "-", "–":
		return ""
	}
	return s
}

func parseBriefTable(body string) ([]Brief, error) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := map[string]int{}
		cols := splitRow(line)
		for j, c := range cols {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		if _, ok := idx["brief"]; !ok {
			continue
		}
		if _, ok := idx["status"]; !ok {
			continue
		}
		for _, req := range []string{"#", "brief", "wave", "status", "verified", "reviewed"} {
			if _, ok := idx[req]; !ok {
				return nil, fmt.Errorf("briefs table missing required column %q", req)
			}
		}
		var briefs []Brief
		for _, row := range lines[i+2:] { // i+1 is the |---| separator
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			cells := splitRow(row)
			if len(cells) < len(cols) {
				return nil, fmt.Errorf("row has %d cells, header has %d: %q", len(cells), len(cols), row)
			}
			get := func(name string) string { return strings.TrimSpace(cells[idx[name]]) }
			wave, err := strconv.Atoi(get("wave"))
			if err != nil {
				return nil, fmt.Errorf("brief %s: wave %q is not an integer", get("#"), get("wave"))
			}
			title := get("brief")
			if m := linkRe.FindStringSubmatch(title); m != nil {
				title = m[1]
			}
			b := Brief{
				Num:      get("#"),
				Title:    title,
				Wave:     wave,
				Status:   strings.ToLower(get("status")),
				Verified: normalizeMark(get("verified")),
				Reviewed: normalizeMark(get("reviewed")),
			}
			if j, ok := idx["effort"]; ok {
				b.Effort = strings.TrimSpace(cells[j])
			}
			briefs = append(briefs, b)
		}
		return briefs, nil
	}
	return nil, nil
}

func parseStreamREADME(path string) (*Stream, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fmRaw, body, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return nil, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	briefs, err := parseBriefTable(body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Stream{
		Name:          fm.Stream,
		Dir:           filepath.Dir(path),
		Status:        fm.Status,
		Priority:      fm.Priority,
		Track:         fm.Track,
		Issues:        fm.Issues,
		External:      fm.External,
		Tiering:       fm.Tiering,
		MaxConcurrent: fm.MaxConcurrent,
		Serves:        fm.Serves,
		Theme:         strings.TrimSpace(fm.Theme),
		Owner:         fm.Owner,
		Repo:          strings.TrimSpace(fm.Repo),
		Briefs:        briefs,
	}, nil
}
