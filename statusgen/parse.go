package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/streamview"
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
	Spec          string  `yaml:"spec"`           // optional; the repo-relative scoping doc this stream was scaffolded FROM (attention-budget/04). An active stream must cite one whose §8.1 header is `**Status:** approved` (the `stream-source` lint); a parked stream may cite a draft.
	// Mission is the optional authored mission block, kept as a raw node so a
	// malformed block never fails the README parse (the board must keep
	// working); parseMissionBlock validates it and returns diagnostics.
	// Kind == 0 when the key is absent.
	Mission yaml.Node `yaml:"mission"`
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
			rawCell := get("brief")
			title := unwrapTitleLink(rawCell)
			b := Brief{
				Num:      get("#"),
				Title:    title,
				RawCell:  rawCell,
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
	mission, missionDiags := parseMissionBlock(&fm.Mission)
	return &Stream{
		Name:               fm.Stream,
		Dir:                filepath.Dir(path),
		Status:             fm.Status,
		Priority:           fm.Priority,
		Track:              fm.Track,
		Issues:             fm.Issues,
		External:           fm.External,
		Tiering:            fm.Tiering,
		MaxConcurrent:      fm.MaxConcurrent,
		Serves:             fm.Serves,
		Theme:              strings.TrimSpace(fm.Theme),
		Owner:              fm.Owner,
		Repo:               strings.TrimSpace(fm.Repo),
		Board:              strings.TrimSpace(fm.Board),
		Traced:             fm.Traced != nil && *fm.Traced,
		Spec:               strings.TrimSpace(fm.Spec),
		Mission:            mission,
		MissionDiagnostics: missionDiags,
		Briefs:             briefs,
	}, nil
}

// missionVersion is the one authored mission-block version this statusgen
// reads. Any other value is diagnosed, never read best-effort.
const missionVersion = 1

var (
	missionForgeRefRe = regexp.MustCompile(`^([A-Za-z0-9._-]+/[A-Za-z0-9._-]+)#([1-9][0-9]*)$`)
	missionSchemeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// parseMissionBlock validates the optional `mission:` frontmatter block.
//
//	mission:
//	  version: 1                 # required; any other value is refused
//	  outcome: <text>            # required, non-empty
//	  success:                   # optional; authored order is preserved
//	    - criterion: <text>      # required per entry
//	      evidence:              # optional
//	        - docs/report.md                  # repository-relative path
//	        - <owner>/<name>#<number>         # issue or pull request
//	        - https://example.org/page        # https only
//	        - {path: docs/later.md, planned: true}
//	  commitments: [<text>, ...] # optional
//	  exclusions: [<text>, ...]  # optional
//
// It returns (nil, nil) when the block is absent, (mission, nil) when it is
// valid, and (nil, diagnostics) when it is present but invalid — every
// defect found, not just the first. Unknown keys are defects: a typo must not
// silently drop an authored field.
func parseMissionBlock(n *yaml.Node) (*streamview.Mission, []string) {
	if n == nil || n.Kind == 0 {
		return nil, nil
	}
	var diags []string
	bad := func(format string, a ...any) { diags = append(diags, fmt.Sprintf(format, a...)) }
	if n.Kind != yaml.MappingNode {
		return nil, []string{"mission: must be a mapping with version, outcome and optional success/commitments/exclusions"}
	}
	m := &streamview.Mission{Origin: streamview.OriginAuthored}
	sawVersion, sawOutcome := false, false
	scalarText := func(where string, v *yaml.Node) (string, bool) {
		if v.Kind != yaml.ScalarNode || v.Tag == "!!null" {
			bad("%s: must be text", where)
			return "", false
		}
		t := strings.TrimSpace(v.Value)
		if t == "" {
			bad("%s: is empty", where)
			return "", false
		}
		return t, true
	}
	textList := func(where string, v *yaml.Node) []string {
		if v.Kind != yaml.SequenceNode {
			bad("%s: must be a list of text", where)
			return nil
		}
		var out []string
		for i, item := range v.Content {
			if t, ok := scalarText(fmt.Sprintf("%s[%d]", where, i), item); ok {
				out = append(out, t)
			}
		}
		return out
	}
	// dup reports a repeated key at one mapping level: decoding into a raw
	// node is last-wins with no error, which would silently drop a field.
	dup := func(where string, seen map[string]bool, key string) bool {
		if seen[key] {
			bad("%s: duplicate key %q", where, key)
			return true
		}
		seen[key] = true
		return false
	}
	seenTop := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i].Value, n.Content[i+1]
		if dup("mission", seenTop, key) {
			continue
		}
		switch key {
		case "version":
			sawVersion = true
			// A plain YAML integer only: a quoted "1" is text, not the version.
			v, err := strconv.Atoi(strings.TrimSpace(val.Value))
			if val.Kind != yaml.ScalarNode || val.ShortTag() != "!!int" || err != nil {
				bad("mission.version: must be the integer %d", missionVersion)
			} else if v != missionVersion {
				bad("mission.version: unsupported mission version %d (this statusgen reads version %d)", v, missionVersion)
			}
		case "outcome":
			sawOutcome = true
			if t, ok := scalarText("mission.outcome", val); ok {
				m.Outcome = t
			}
		case "success":
			if val.Kind != yaml.SequenceNode {
				bad("mission.success: must be a list of {criterion, evidence}")
				continue
			}
			for ci, c := range val.Content {
				where := fmt.Sprintf("mission.success[%d]", ci)
				if c.Kind != yaml.MappingNode {
					bad("%s: must be a mapping with criterion and optional evidence", where)
					continue
				}
				var sc streamview.SuccessCriterion
				sawCriterion := false
				seenItem := map[string]bool{}
				for k := 0; k+1 < len(c.Content); k += 2 {
					ck, cv := c.Content[k].Value, c.Content[k+1]
					if dup(where, seenItem, ck) {
						continue
					}
					switch ck {
					case "criterion":
						sawCriterion = true
						if t, ok := scalarText(where+".criterion", cv); ok {
							sc.Criterion = t
						}
					case "evidence":
						if cv.Kind != yaml.SequenceNode {
							bad("%s.evidence: must be a list", where)
							continue
						}
						for ei, ev := range cv.Content {
							if ref, ok := parseMissionEvidence(fmt.Sprintf("%s.evidence[%d]", where, ei), ev, bad); ok {
								sc.Evidence = append(sc.Evidence, ref)
							}
						}
					default:
						bad("%s: unknown key %q (want criterion, evidence)", where, ck)
					}
				}
				if !sawCriterion {
					bad("%s.criterion: is required", where)
				}
				m.Success = append(m.Success, sc)
			}
		case "commitments":
			m.Commitments = textList("mission.commitments", val)
		case "exclusions":
			m.Exclusions = textList("mission.exclusions", val)
		default:
			bad("mission: unknown key %q (want version, outcome, success, commitments, exclusions)", key)
		}
	}
	if !sawVersion {
		bad("mission.version: is required (version: %d)", missionVersion)
	}
	if !sawOutcome {
		bad("mission.outcome: is required")
	}
	if len(diags) > 0 {
		return nil, diags
	}
	return m, nil
}

// parseMissionEvidence classifies one authored evidence reference. Path refs
// are returned with an empty Repo — the stream-view identity qualifies them.
func parseMissionEvidence(where string, n *yaml.Node, bad func(string, ...any)) (streamview.EvidenceRef, bool) {
	planned := false
	raw := ""
	switch n.Kind {
	case yaml.ScalarNode:
		raw = strings.TrimSpace(n.Value)
	case yaml.MappingNode:
		sawPath := false
		seen := map[string]bool{}
		for k := 0; k+1 < len(n.Content); k += 2 {
			key, val := n.Content[k].Value, n.Content[k+1]
			if seen[key] {
				bad("%s: duplicate key %q", where, key)
				return streamview.EvidenceRef{}, false
			}
			seen[key] = true
			switch key {
			case "path":
				sawPath = true
				raw = strings.TrimSpace(val.Value)
			case "planned":
				b, err := strconv.ParseBool(strings.TrimSpace(val.Value))
				if val.Kind != yaml.ScalarNode || err != nil {
					bad("%s.planned: must be true or false", where)
					return streamview.EvidenceRef{}, false
				}
				planned = b
			default:
				bad("%s: unknown key %q (want path, planned)", where, key)
				return streamview.EvidenceRef{}, false
			}
		}
		if !sawPath {
			bad("%s.path: is required in the mapping form", where)
			return streamview.EvidenceRef{}, false
		}
	default:
		bad("%s: must be a reference string or {path, planned}", where)
		return streamview.EvidenceRef{}, false
	}
	if raw == "" {
		bad("%s: is empty", where)
		return streamview.EvidenceRef{}, false
	}
	isPath := n.Kind == yaml.MappingNode
	switch {
	case !isPath && strings.HasPrefix(raw, "https://"):
		ref := streamview.EvidenceRef{Kind: streamview.EvidenceURL, URL: raw}
		if err := ref.Validate(); err != nil {
			bad("%s: %v", where, err)
			return streamview.EvidenceRef{}, false
		}
		return ref, true
	case missionSchemeRe.MatchString(raw):
		bad("%s: %q — only https URLs, <owner>/<name>#<number> and repository-relative paths are accepted", where, raw)
		return streamview.EvidenceRef{}, false
	case strings.HasPrefix(raw, "#"):
		bad("%s: bare %q is ambiguous — qualify it as <owner>/<name>%s", where, raw, raw)
		return streamview.EvidenceRef{}, false
	case !isPath && strings.Contains(raw, "#"):
		mm := missionForgeRefRe.FindStringSubmatch(raw)
		if mm == nil {
			bad("%s: %q is not <owner>/<name>#<number>", where, raw)
			return streamview.EvidenceRef{}, false
		}
		num, _ := strconv.Atoi(mm[2])
		ref := streamview.EvidenceRef{Kind: streamview.EvidenceForge, Repo: mm[1], Number: num}
		if err := ref.Validate(); err != nil {
			bad("%s: %v", where, err)
			return streamview.EvidenceRef{}, false
		}
		return ref, true
	}
	// A path: validate its form against a placeholder repo; the real owning
	// repo is attached by the stream-view identity.
	probe := streamview.EvidenceRef{Kind: streamview.EvidencePath, Repo: "owner/name", Path: raw, Planned: planned}
	if err := probe.Validate(); err != nil {
		bad("%s: %v", where, err)
		return streamview.EvidenceRef{}, false
	}
	return streamview.EvidenceRef{Kind: streamview.EvidencePath, Path: raw, Planned: planned}, true
}
