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
	Board         string  `yaml:"board"`          // optional; "generated" opts the Briefs table into the marker-wrapped generated region (derived-board/04).
	Traced        *bool   `yaml:"traced"`         // optional; true opts the stream INTO the untraced-brief traceability check (registers-v1 §6.5). nil/false = out (the default): the check never fires over a corpus that has not opted in.
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

// unwrapTitleLink returns the link TEXT of the first well-formed `[text](url)` inline link in a
// README status-table title cell, so the Next-up row renders a bare title like every other row.
// It matches brackets by DEPTH rather than by regexp: a title whose link text itself contains a
// bracket — a backticked tag such as “ [`[assay]` …](./brief-…md) “ — presents an inner `]`
// that the old `\[([^\]]+)\]\(…\)` form stopped at, so the outer link never unwrapped and the
// raw `./brief-…` target rode into STATUS.md, dead from the repo root and reddening any snapshot
// that copied it (#591). A cell with no well-formed inline link is returned unchanged.
func unwrapTitleLink(cell string) string {
	i := 0
	for {
		j := strings.Index(cell[i:], "](")
		if j < 0 {
			return cell
		}
		j += i
		open, okOpen := matchOpenBracket(cell, j)
		_, okEnd := matchCloseParen(cell, j+1)
		if okOpen && okEnd {
			return cell[open+1 : j]
		}
		// Not a well-formed link at this `](`; keep scanning past it.
		i = j + 2
	}
}

// matchOpenBracket walks BACKWARD from the `]` at index close to the `[` that opens it, counting
// nested bracket pairs so an inner `[...]` inside the link text does not steal the match.
func matchOpenBracket(s string, close int) (int, bool) {
	depth := 0
	for k := close - 1; k >= 0; k-- {
		switch s[k] {
		case ']':
			depth++
		case '[':
			if depth == 0 {
				return k, true
			}
			depth--
		}
	}
	return 0, false
}

// matchCloseParen walks FORWARD from the `(` at index open to the `)` that closes it, counting
// nested parens so a `(...)` inside the URL does not close the match early.
func matchCloseParen(s string, open int) (int, bool) {
	depth := 0
	for k := open; k < len(s); k++ {
		switch s[k] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return k, true
			}
		}
	}
	return 0, false
}

// splitRow splits a markdown table row into its cells on UNESCAPED pipes.
//
// GFM resolves `\|` before inline parsing, so a backslash-escaped pipe is cell
// CONTENT, never a delimiter — it is the only way to put a pipe inside a table
// cell, and it applies even inside a `code span` (a `curl … \| bash` note).
// Splitting on every `|` byte therefore invents a cell in a perfectly legal row;
// parseBriefTable's exact cell-count check then rejects that row, and because a
// stream README parse error aborts the whole load, one such row turns every
// other check in the run into could-not-check.
//
// The escape sequence is PRESERVED verbatim in the returned cell. The escape
// decides cell boundaries and nothing else, so a row that is parsed and
// re-rendered is byte-identical to the one that was read — the re-render paths
// in readmetable.go and transcribeverdict.go write these cells straight back.
//
// Outer-delimiter handling is unchanged: the empty cells produced by the row's
// leading and trailing delimiter runs are dropped, so for any row carrying no
// `\|` this returns exactly what the previous `strings.Trim(line, "|")` plus
// `strings.Split` returned.
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			cur.WriteString(`\|`) // escaped pipe: cell content, not a delimiter
			i++
		case s[i] == '|':
			cells = append(cells, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	cells = append(cells, cur.String())
	// Only ZERO-LENGTH cells are dropped: a blank-but-spaced cell (`|  |`) is a
	// real empty column — a `— `-less Verified/Reviewed cell — and must survive,
	// or every row would come up short and be rejected.
	start := 0
	for start < len(cells) && cells[start] == "" {
		start++
	}
	end := len(cells)
	for end > start && cells[end-1] == "" {
		end--
	}
	if start >= end {
		return []string{""}
	}
	return cells[start:end]
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
			if len(cells) != len(cols) {
				return nil, fmt.Errorf("row has %d cells, header has %d: %q — a briefs-table row must have exactly one cell per column. The usual cause of extra cells is a PR reference written into the Status cell (e.g. `implemented (#80)`) or a stray `||`, which prepends a cell and shifts every column right. The Status cell takes ONLY a bare lifecycle token (todo / in-progress / implemented / verified / done / blocked); the PR association belongs in the PR body or the `Brief:` trailer, never in the cell", len(cells), len(cols), row)
			}
			get := func(name string) string { return strings.TrimSpace(cells[idx[name]]) }
			wave, err := strconv.Atoi(get("wave"))
			if err != nil {
				return nil, fmt.Errorf("brief %s: wave %q is not an integer", get("#"), get("wave"))
			}
			title := unwrapTitleLink(get("brief"))
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
		Board:         strings.TrimSpace(fm.Board),
		Traced:        fm.Traced != nil && *fm.Traced,
		Briefs:        briefs,
	}, nil
}
