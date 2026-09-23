package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// brief.go — load a brief, read its OWN risk declaration, and parse its `## Verify` table
// into rows. The risk read reuses the audited deskkit.RiskFromBriefContent (the same
// frontmatter predicate the ready-flip risk gate uses), so the verb's risk refusal cannot
// drift from the rest of the fleet's notion of "risk-bearing".

// verifyRow is one parsed `## Verify` row: its number, Class, Command and Expect cells.
// Columns are located by header NAME, never position, mirroring the runner's parser
// (cmd/verifyloop/verifyrows.go) and the statusgen table reader.
type verifyRow struct {
	Num     int
	Class   string
	Command string
	Expect  string
}

// loadedBrief is a brief resolved to a file, with its risk declaration and Verify rows.
type loadedBrief struct {
	Path         string
	Content      string
	RiskBearing  bool
	RiskReason   string
	Unverifiable bool // true when the brief has no parseable frontmatter fence (fail closed)
	Rows         []verifyRow
}

// row returns the Verify row with the given 1-based number, or ok=false when the table has
// no such row.
func (b loadedBrief) row(k int) (verifyRow, bool) {
	for _, r := range b.Rows {
		if r.Num == k {
			return r, true
		}
	}
	return verifyRow{}, false
}

// loadBrief resolves brief (a path, or a `<stream>/<NN>` id resolved under repo's configured
// stream root) to a file, reads its risk declaration, and parses its Verify table. An
// unresolvable id, an unreadable file or a brief with no frontmatter fence is UNVERIFIABLE —
// fail closed, treated as risk-bearing so the verb refuses rather than re-baselining a brief
// it could not read (mirrors deskkit.BriefRiskFromBody's fail-closed contract).
func loadBrief(repo, repoRoot, brief string) (loadedBrief, error) {
	path, err := resolveBriefPath(repo, repoRoot, brief)
	if err != nil {
		return loadedBrief{}, err
	}
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return loadedBrief{}, fmt.Errorf("owning brief %q resolved to %s but could not be read: %w", brief, path, rerr)
	}
	content := string(raw)
	classed, reason, hasFrontmatter := deskkit.RiskFromBriefContent(content)
	lb := loadedBrief{Path: path, Content: content, Rows: parseVerifyRows(content)}
	if !hasFrontmatter {
		lb.RiskBearing = true
		lb.Unverifiable = true
		lb.RiskReason = "brief " + path + " has no parseable frontmatter fence — its gate/risk declaration cannot be read (unverifiable, fail closed)"
		return lb, nil
	}
	lb.RiskBearing = classed
	lb.RiskReason = reason
	return lb, nil
}

// resolveBriefPath treats brief as a file path when one exists, else as a `<stream>/<NN>` id
// resolved under repo's configured stream root — the same glob deskkit.BriefRiskFromBody uses.
func resolveBriefPath(repo, repoRoot, brief string) (string, error) {
	if fi, err := os.Stat(brief); err == nil && !fi.IsDir() {
		return brief, nil
	}
	stream, nn, ok := splitStreamNN(brief)
	if !ok {
		return "", fmt.Errorf("brief %q is neither an existing file nor a resolvable <stream>/<NN> id", brief)
	}
	// Prefer the configured stream root for repo; fall back to repoRoot when this process has
	// no roots configured (the common single-checkout case).
	roots := []string{}
	if r := deskkit.RootForRepo(repo); r != "" {
		roots = append(roots, r)
	}
	if repoRoot != "" {
		roots = append(roots, repoRoot)
	}
	for _, root := range roots {
		matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md"))
		if len(matches) > 0 {
			return matches[0], nil
		}
	}
	return "", fmt.Errorf("brief %s/%s does not resolve under any configured stream root for %s", stream, nn, repo)
}

// splitStreamNN reduces a `<stream>/<NN>` (or colon-form) brief id to (stream, NN). NN must
// be numeric. Mirrors deskkit's splitBriefRef, kept local so this command imports no
// unexported helper.
func splitStreamNN(v string) (stream, nn string, ok bool) {
	v = strings.TrimSpace(v)
	var parts []string
	if strings.Contains(v, ":") {
		parts = strings.Split(v, ":")
	} else {
		parts = strings.Split(v, "/")
	}
	if len(parts) < 2 {
		return "", "", false
	}
	stream, nn = strings.TrimSpace(parts[len(parts)-2]), strings.TrimSpace(parts[len(parts)-1])
	if stream == "" || nn == "" {
		return "", "", false
	}
	for _, c := range nn {
		if c < '0' || c > '9' {
			return "", "", false
		}
	}
	return stream, nn, true
}

var sepRe = regexp.MustCompile(`^\s*\|?[\s:|-]+\|?\s*$`)

// parseVerifyRows extracts the rows of the `## Verify` pipe table. Columns are located by
// header NAME (#, class, command, expect), so a re-ordered or extended table parses the
// same. A table with no Class column defaults every row to `check`.
func parseVerifyRows(content string) []verifyRow {
	section := extractSection(content, "## Verify")
	if section == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := headerIndex(line)
		numCol, hasNum := idx["#"]
		cmdCol, hasCmd := idx["command"]
		if !hasNum || !hasCmd {
			continue
		}
		classCol, hasClass := idx["class"]
		expectCol, hasExpect := idx["expect"]
		var rows []verifyRow
		for _, row := range lines[i+2:] {
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			if sepRe.MatchString(row) {
				continue
			}
			cells := splitRow(row)
			cell := func(j int) string {
				if j >= 0 && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			num, err := strconv.Atoi(cell(numCol))
			if err != nil {
				continue
			}
			class := "check"
			if hasClass {
				if c := cell(classCol); c != "" {
					class = c
				}
			}
			expect := ""
			if hasExpect {
				expect = stripInlineCode(cell(expectCol))
			}
			rows = append(rows, verifyRow{
				Num:     num,
				Class:   class,
				Command: stripInlineCode(cell(cmdCol)),
				Expect:  expect,
			})
		}
		return rows
	}
	return nil
}

// verifySectionBounds returns the [lo, hi) line-index window of the `## Verify` section within
// lines — lo is the first line after the `## Verify` heading, hi is the next `## ` heading (or
// len(lines)). It returns lo=-1 when there is no `## Verify` section. It is the index-preserving
// twin of extractSection: applyRebaseline needs the section's bounds in the FULL line slice so
// its in-place rewrite lands on the right line, not a copy of just the section.
func verifySectionBounds(lines []string) (lo, hi int) {
	lo = -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Verify" {
			lo = i + 1
			break
		}
	}
	if lo < 0 {
		return -1, -1
	}
	hi = len(lines)
	for i := lo; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			hi = i
			break
		}
	}
	return lo, hi
}

// extractSection returns the body between an exact `## <heading>` line and the next `## `.
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// headerIndex maps lower-cased header cell names to their column positions.
func headerIndex(headerLine string) map[string]int {
	idx := map[string]int{}
	for j, c := range splitRow(headerLine) {
		name := strings.ToLower(strings.TrimSpace(c))
		if name != "" {
			idx[name] = j
		}
	}
	return idx
}

// splitRow splits a pipe-table row into its cells, dropping the empty leading/trailing cells
// the outer pipes produce.
func splitRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	return strings.Split(row, "|")
}

// stripInlineCode unwraps a single `…` inline-code span from a cell.
func stripInlineCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		return strings.TrimSpace(strings.Trim(s, "`"))
	}
	return s
}
