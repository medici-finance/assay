package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// briefRow is one parsed stream-README table row plus its resolved brief file.
type briefRow struct {
	Stream        string
	Num           string
	Status        string
	Verified      string
	Reviewed      string
	BriefPath     string // repo-relative
	fm            briefFrontmatter
	evidenceEmpty bool
}

// briefFrontmatter is the subset of brief-v1 frontmatter the verify adapter needs. It is
// parsed with a small field extractor rather than a YAML dep so the desk module stays
// self-contained (the deliberate "desk mirrors statusgen, does not import it" pattern —
// statusgen is package main and unimportable anyway).
type briefFrontmatter struct {
	Gate        string
	Risk        loopengine.RiskFlags
	Effort      string
	ExecTier    string
	Implementer string // optional `implementer:` field; empty disables the author!=runner guard for this item
}

// scanAwaiting reads every stream README under <root>/docs/streams/*/README.md, applies the
// statusgen Awaiting filter (status ∈ {implemented, verified}), resolves each row to its
// brief file + frontmatter + Evidence emptiness, and returns typed Items ORDERED tier-1
// (implemented with an EMPTY Evidence section — the real verify work) before tier-2
// (verified free-closes and Evidence-present-but-unpromoted rows), oldest-first within class.
//
// This is the deterministic board read; there is NO code path that produces a verify verdict
// without going through the engine's Dispatch — the #541 inline path is unrepresentable.
func scanAwaiting(root, targetSHA string) ([]loopengine.Item, error) {
	streamsDir := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		return nil, err
	}
	var rows []briefRow
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		readme := filepath.Join(streamsDir, e.Name(), "README.md")
		raw, err := os.ReadFile(readme)
		if err != nil {
			continue // a stream dir without a README is not a fatal scan error
		}
		stream, tableRows := parseStreamTable(string(raw), e.Name())
		for _, r := range tableRows {
			st := strings.ToLower(r.Status)
			if st != "implemented" && st != "verified" {
				continue // Awaiting filter
			}
			r.Stream = stream
			r.BriefPath, r.fm, r.evidenceEmpty = resolveBrief(root, streamsDir, e.Name(), r.Num)
			rows = append(rows, r)
		}
	}

	// Order: tier-1 (implemented + empty Evidence) first, then tier-2; oldest-first within
	// class. Age from the statusgen historian is the live signal; for a deterministic
	// reference read we order within class by (stream, brief-num), a stable proxy.
	sort.SliceStable(rows, func(i, j int) bool {
		ci, cj := workClass(rows[i]), workClass(rows[j])
		if ci != cj {
			return ci < cj
		}
		if rows[i].Stream != rows[j].Stream {
			return rows[i].Stream < rows[j].Stream
		}
		return rows[i].Num < rows[j].Num
	})

	items := make([]loopengine.Item, 0, len(rows))
	for _, r := range rows {
		items = append(items, loopengine.Item{
			ID:          r.Stream + "/" + r.Num,
			BriefPath:   r.BriefPath,
			TargetSHA:   targetSHA,
			Risk:        r.fm.Risk,
			Gate:        r.fm.Gate,
			Effort:      r.fm.Effort,
			ExecTier:    r.fm.ExecTier,
			Implementer: r.fm.Implementer,
			Payload: map[string]string{
				"status":   r.Status,
				"verified": r.Verified,
				"reviewed": r.Reviewed,
			},
		})
	}
	return items, nil
}

// workClass returns 0 for tier-1 (the real verify work: implemented with an EMPTY Evidence
// section) and 1 for tier-2 (verified free-closes + Evidence-present-but-unpromoted). Tier-1
// must never be crowded out by tier-2.
func workClass(r briefRow) int {
	if strings.ToLower(r.Status) == "implemented" && r.evidenceEmpty {
		return 0
	}
	return 1
}

var sepRe = regexp.MustCompile(`^\s*\|?\s*-{2,}`)

// parseStreamTable extracts the stream name (frontmatter `stream:`) and the brief-table rows
// from a stream README. It mirrors statusgen/parse.go's column-by-header approach: it finds
// the pipe table whose header carries both "brief" and "status", then reads rows by column
// name so column order/extra columns do not matter.
func parseStreamTable(content, dirName string) (string, []briefRow) {
	stream := dirName
	if m := regexp.MustCompile(`(?m)^stream:\s*(\S+)`).FindStringSubmatch(content); m != nil {
		stream = m[1]
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := headerIndex(line)
		if _, ok := idx["#"]; !ok {
			continue
		}
		if _, ok := idx["status"]; !ok {
			continue
		}
		var rows []briefRow
		for _, row := range lines[i+2:] { // skip the |---| separator
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			if sepRe.MatchString(row) {
				continue
			}
			cells := splitRow(row)
			get := func(name string) string {
				if j, ok := idx[name]; ok && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			num := get("#")
			if num == "" {
				continue
			}
			rows = append(rows, briefRow{
				Num:      num,
				Status:   get("status"),
				Verified: normalizeMark(get("verified")),
				Reviewed: normalizeMark(get("reviewed")),
			})
		}
		return stream, rows
	}
	return stream, nil
}

func headerIndex(line string) map[string]int {
	idx := map[string]int{}
	for j, c := range splitRow(line) {
		idx[strings.ToLower(strings.TrimSpace(c))] = j
	}
	return idx
}

func splitRow(line string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
}

func normalizeMark(s string) string {
	switch strings.TrimSpace(s) {
	case "—", "-", "–", "":
		return ""
	}
	return s
}

// resolveBrief finds the brief file for a row (docs/streams/<dir>/brief-<num>-*.md), parses
// the frontmatter subset, and reports whether its ## Evidence section is empty.
func resolveBrief(root, streamsDir, dir, num string) (relPath string, fm briefFrontmatter, evidenceEmpty bool) {
	matches, _ := filepath.Glob(filepath.Join(streamsDir, dir, "brief-"+num+"-*.md"))
	if len(matches) == 0 {
		// no resolvable brief file — treat as empty-evidence so it is not silently dropped
		return "", briefFrontmatter{}, true
	}
	path := matches[0]
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", briefFrontmatter{}, true
	}
	rel, rerr := filepath.Rel(root, path)
	if rerr != nil || rel == "" {
		rel = path
	}
	fm = parseFrontmatter(string(raw))
	evidenceEmpty = !evidenceHasContent(extractEvidence(string(raw)))
	return rel, fm, evidenceEmpty
}
