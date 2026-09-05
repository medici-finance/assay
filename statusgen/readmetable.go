package main

// readmetable.go — the generated Briefs table in a stream README (derived-board/04).
//
// A stream README whose frontmatter carries `board: generated` opts its Briefs
// table into a marker-wrapped GENERATED region:
//
//	<!-- statusgen:briefs:begin -->
//	| # | Brief | Wave | Effort | Status | Verified | Reviewed |
//	|---|-------|------|--------|--------|----------|----------|
//	| 01 | [title](brief-01-….md) | 0 | M | done | … | … |
//	<!-- statusgen:briefs:end -->
//
// `statusgen regen --readmes` (re)writes the region; a hand edit to it is a
// `statusgen --lint` PROBLEM (rule 47). Everything OUTSIDE the two markers is
// left byte-for-byte untouched.
//
// INTERIM behaviour (governing ruling, 2026-09-04 — "make drift visible as a
// NOTICE first, don't hard-flip behavior"): only the AUTHORING
// columns (#, Brief link+title, Wave, Effort) are written from the brief
// frontmatter and enforced offline; those are the deterministic, tree-only
// columns. The LIFECYCLE columns (Status, Verified, Reviewed) are PRESERVED from
// the region as it stands — they are not overwritten from a derivation, because
// the PR/witness fold that derives them (DeriveLifecycle) needs an online read
// the offline `--lint`/branch path cannot make. Drift between the preserved
// (asserted) lifecycle cell and what the witnesses derive is surfaced as a
// NOTICE by `regen --readmes` when a reconcile read is available
// (assertedVsDerivedNotices) — the de-risking comparator the ruling asks for,
// BEFORE the fleet-wide hard flip to a fully witness-written cell.
//
// Because the lifecycle cells are copied through on every render, a hand edit to
// one of THEM is not caught here (offline it cannot be told from a legitimate
// board-row flip); only a hand edit to an AUTHORING cell diverges from a fresh
// render and reddens `--lint` (brief-04 Context: "the lifecycle columns are not
// compared offline").

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	briefsMarkerBegin = "<!-- statusgen:briefs:begin -->"
	briefsMarkerEnd   = "<!-- statusgen:briefs:end -->"

	// briefTableHead is the fixed header + separator of a generated Briefs table.
	// The region is always header, separator, then one row per brief.
	briefTableHead = "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|"
)

// lifecycleCells are the three hand-asserted/derived columns a render preserves.
type lifecycleCells struct {
	status, verified, reviewed string
}

// genRow is one brief's authoring columns, rendered from its frontmatter.
type genRow struct {
	num, title, file string
	wave             int
	effort           string
}

// streamBriefRows returns the authoring columns for every parseable brief file in
// a stream, ordered by brief number. Legacy (unparseable / non-brief-shaped)
// files are skipped — a board: generated stream is expected to be all brief-v1/v2,
// and a stray file must not abort the render.
func streamBriefRows(s *Stream) []genRow {
	var rows []genRow
	seen := map[string]bool{}
	for _, path := range briefFilePaths(s) {
		bf, ok, err := parseBriefFile(path)
		if err != nil || !ok || bf.Brief == "" {
			continue
		}
		_, num, okName := expectedBriefID(path)
		if !okName || num == "" || seen[num] {
			continue
		}
		seen[num] = true
		rows = append(rows, genRow{
			num:    num,
			title:  bf.Title,
			file:   fileBase(path),
			wave:   bf.Wave,
			effort: bf.Effort,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].num < rows[j].num })
	return rows
}

// fileBase returns the final path segment (the brief filename) for the table link
// target, matching how the hand-written tables already spell it (bare filename,
// relative to the README).
func fileBase(path string) string {
	if i := strings.LastIndexAny(path, "/\\"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// renderBriefsRegion builds the region text (header + separator + one row per
// brief) for a stream. Authoring columns come from the brief frontmatter;
// lifecycle columns come from preserved, defaulting a brief with no existing row
// to the honest base (`todo` / `—` / `—`). The returned text has no trailing
// newline; the caller frames it between the markers.
func renderBriefsRegion(s *Stream, preserved map[string]lifecycleCells) string {
	var b strings.Builder
	b.WriteString(briefTableHead)
	for _, r := range streamBriefRows(s) {
		lc, ok := preserved[r.num]
		if !ok {
			lc = lifecycleCells{status: "todo", verified: "—", reviewed: "—"}
		}
		if strings.TrimSpace(lc.status) == "" {
			lc.status = "todo"
		}
		if strings.TrimSpace(lc.verified) == "" {
			lc.verified = "—"
		}
		if strings.TrimSpace(lc.reviewed) == "" {
			lc.reviewed = "—"
		}
		b.WriteString(fmt.Sprintf("\n| %s | [%s](%s) | %d | %s | %s | %s | %s |",
			r.num, r.title, r.file, r.wave, r.effort, lc.status, lc.verified, lc.reviewed))
	}
	return b.String()
}

// parsePreservedLifecycle reads the lifecycle columns out of an existing region,
// keyed by brief number, so a re-render carries the hand-asserted/derived cells
// through unchanged. Rows it cannot key on are skipped.
func parsePreservedLifecycle(region string) map[string]lifecycleCells {
	out := map[string]lifecycleCells{}
	for _, line := range strings.Split(region, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			continue
		}
		cells := splitRow(line)
		// header/separator and short rows carry no lifecycle data.
		if len(cells) < 7 {
			continue
		}
		num := strings.TrimSpace(cells[0])
		if num == "" || num == "#" || strings.HasPrefix(num, "---") {
			continue
		}
		out[num] = lifecycleCells{
			status:   strings.TrimSpace(cells[4]),
			verified: strings.TrimSpace(cells[5]),
			reviewed: strings.TrimSpace(cells[6]),
		}
	}
	return out
}

// extractRegion locates the marker-wrapped region in a README's content. ok is
// false when either marker is absent or they are out of order; the returned
// prefix (through the begin marker) and suffix (from the end marker) frame the
// region on a successful rewrite.
func extractRegion(content string) (prefix, region, suffix string, ok bool) {
	bi := strings.Index(content, briefsMarkerBegin)
	if bi < 0 {
		return "", "", "", false
	}
	ei := strings.Index(content, briefsMarkerEnd)
	if ei < 0 || ei < bi+len(briefsMarkerBegin) {
		return "", "", "", false
	}
	prefix = content[:bi+len(briefsMarkerBegin)]
	region = content[bi+len(briefsMarkerBegin) : ei]
	suffix = content[ei:]
	return prefix, region, suffix, true
}

// framedRegion reassembles a README from its prefix/suffix and a freshly rendered
// region, always with exactly one newline on each side of the table so the result
// is byte-stable under repeated rewrites.
func framedRegion(prefix, region, suffix string) string {
	return prefix + "\n" + region + "\n" + suffix
}

// rewriteReadmeRegion regenerates a stream README's Briefs region in place. It is
// a no-op (changed=false) when the file already matches a fresh render, which is
// what makes `regen --readmes` idempotent. A board: generated README with no
// markers is an error — the markers are the opt-in's second half.
func rewriteReadmeRegion(s *Stream, path string) (changed bool, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(raw)
	prefix, region, suffix, ok := extractRegion(content)
	if !ok {
		return false, fmt.Errorf("%s: board: generated but no %s / %s markers around the Briefs table", path, briefsMarkerBegin, briefsMarkerEnd)
	}
	preserved := parsePreservedLifecycle(region)
	rendered := renderBriefsRegion(s, preserved)
	next := framedRegion(prefix, rendered, suffix)
	if next == content {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// checkReadmeTables is the `--lint` arm: for every board: generated stream it
// PROBLEMs a marker-wrapped table whose AUTHORING columns differ from a fresh
// render of the brief frontmatter (rule 47), and PROBLEMs a board: generated
// README that is missing its markers. It is offline and tree-only: lifecycle
// columns are preserved into the expected render, so only an authoring-cell edit
// diverges. It never writes.
func checkReadmeTables(streams []*Stream) (problems, notices []string) {
	for _, s := range streams {
		if s.Board != "generated" {
			continue
		}
		path := s.Dir + "/README.md"
		raw, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: board: generated but the README could not be read: %v", path, err))
			continue
		}
		content := string(raw)
		_, region, _, ok := extractRegion(content)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s README: board: generated but no %s / %s markers around the Briefs table — add the markers or drop board: generated", s.Name, briefsMarkerBegin, briefsMarkerEnd))
			continue
		}
		preserved := parsePreservedLifecycle(region)
		expected := renderBriefsRegion(s, preserved)
		if strings.TrimSpace(region) == strings.TrimSpace(expected) {
			continue
		}
		// Name the offending rows: any brief whose expected authoring row is not
		// present verbatim in the current region is a hand edit to a generated table.
		named := readmeDriftRows(region, expected)
		if len(named) == 0 {
			problems = append(problems, fmt.Sprintf("%s README: hand edit to a generated table — the marker-wrapped Briefs region differs from a fresh render of the brief frontmatter; regenerate with `statusgen regen --readmes`", s.Name))
			continue
		}
		for _, num := range named {
			problems = append(problems, fmt.Sprintf("%s README: hand edit to a generated table — row %s authoring cells (title/wave/effort) differ from the brief frontmatter; regenerate with `statusgen regen --readmes`", s.Name, num))
		}
	}
	return problems, notices
}

// readmeDriftRows returns the brief numbers whose expected generated row is not
// present in the current region, so the PROBLEM can name the row a hand edit
// touched. Comparison is per-row and exact: a preserved-lifecycle render means an
// unedited row matches verbatim, so only an authoring edit shows up here.
func readmeDriftRows(current, expected string) []string {
	have := map[string]bool{}
	for _, line := range strings.Split(current, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "|") {
			have[t] = true
		}
	}
	var rows []string
	for _, line := range strings.Split(expected, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			continue
		}
		cells := splitRow(line)
		if len(cells) < 7 {
			continue // header / separator
		}
		num := strings.TrimSpace(cells[0])
		if num == "" || num == "#" {
			continue
		}
		if !have[t] {
			rows = append(rows, num)
		}
	}
	return rows
}

// assertedVsDerivedNotices is the INTERIM drift comparator the governing ruling
// asks for: it compares each brief's ASSERTED README Status cell against
// the cell DeriveLifecycle computes from the witnesses, and returns one NOTICE per
// disagreement. A derived `unknown` (the offline / could-not-look state) never
// produces a NOTICE — a could-not-check is not a drift (the three-state
// invariant). This makes board-vs-witness drift VISIBLE before the fleet-wide hard
// flip to a witness-written cell, without changing what the board renders.
func assertedVsDerivedNotices(s *Stream, derived []BriefCell) []string {
	asserted := map[string]string{}
	for i := range s.Briefs {
		asserted[s.Name+"/"+s.Briefs[i].Num] = strings.ToLower(strings.TrimSpace(s.Briefs[i].Status))
	}
	var notices []string
	for _, c := range derived {
		if c.Cell == "unknown" || c.Cell == "" {
			continue
		}
		a, ok := asserted[c.ID]
		if !ok || a == "" {
			continue
		}
		if a == c.Cell {
			continue
		}
		detail := c.Witness
		if detail == "" {
			detail = c.Reason
		}
		notices = append(notices, fmt.Sprintf("%s README: board cell for %s asserts %q but the witnesses derive %q (%s) — reconcile the row", s.Name, c.ID, a, c.Cell, detail))
	}
	return notices
}
