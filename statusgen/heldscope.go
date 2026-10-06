package main

import (
	"regexp"
	"strconv"
	"strings"
)

// The read scope of the held/could-not-check scan (#1894).
//
// unroutedHeldLine used to read every line of a brief's whole Evidence
// section. Evidence accumulates: each verifier run appends its own `### `
// entry, and a run's table carries Command and Expect cells beside its
// result. So three kinds of text that are not a live row disposition kept
// refusing a clean pass:
//
//   - a row HELD/could-not-check in an EARLIER entry that a later run has
//     re-run and recorded with a strict PASS;
//   - an Expect (or Command) cell that names could-not-check as the
//     behaviour under test ("unreadable diff → could-not-check exit 6");
//   - prose beside the table that states what an exit code means
//     ("could-not-check→exit 6").
//
// heldScanScope decides what the scan reads. Every rule narrows only where
// the text is positively classified, and falls back to the old whole-text
// read where it is not:
//
//  1. Result cells. In an entry that holds an Evidence results table — a
//     header naming a row key ("#", "Row", "Command", exact names) and at
//     least one column that is not a key, Expect, Date or Runner column
//     (heldColumn) — each data row of that table is read in its result
//     cells only. A row whose cell count differs from its header's (an
//     unescaped "|" in a command) cannot be aligned and is read whole. A
//     table of any other shape is read whole.
//  2. Exit-code prose. In an entry that holds such a results table, a line
//     outside every table is still read, with one exception: an occurrence
//     written as an exit-code mapping — "could-not-check → exit 6",
//     "could-not-check = exit 6", "exit 6 (could-not-check)" — on a line
//     that names no row (heldExitMapping). It says what an exit code means,
//     not how a row came out: that is recorded in the row's own result cell,
//     which rule 1 reads. A mapping line that names a row ("row 2
//     could-not-check → exit 6") is read. In an entry with no results table
//     there is no result cell to fall back on, so every occurrence in its
//     prose is read. Any other prose occurrence — a verdict-line summary
//     "row 4 HELD", a note — is read.
//  3. Supersession, row by row. The Evidence body is split into entries at
//     `### ` headings (level exactly three, column 0) that render as live
//     headings (heldRender); text before the first such heading is the
//     first entry. A held occurrence in an earlier entry is dropped only
//     when ALL of these hold, and otherwise every entry is read:
//     - the LAST entry carries a live strict **VERIFY: PASS** marker
//     (heldEntryPass): at column 0, opening its own paragraph, rendered
//     live, and with no VERIFY token after it anywhere in the entry, quoted
//     or not;
//     - every held occurrence in the earlier entries sits in the result cell
//     of an aligned row of a results table (rule 1) with a non-empty row key
//     (heldRowKey). One held occurrence anywhere else — prose, a misaligned
//     row, a table of another shape — leaves every entry read;
//     - the last entry has live results tables (heldCovered) with a row of
//     the same key (heldRowKey: key-column name and key text) for every one
//     of those rows, and EVERY row of that key in the last entry is clean:
//     its Exit and Result-type columns (heldCoverColumn) hold at least one
//     cell that reads, as a whole, as a recognised clean outcome (heldOutcome:
//     ok, PASS, exit 0 or the like, followed only by neutral details such as
//     a non-zero test count or a duration; a bare 0 only as a whole Exit
//     cell), and no other cell of them or of any unrecognised column holds
//     visible text. An expectation column (Expected exit, Pass criteria) is
//     never read for the outcome. A placeholder (—, n/a), a carry-forward
//     (same as Run 1), a note after the outcome (PASS (no re-run)), a zero
//     count (0 checks run), markup or a format character, or any text outside
//     that closed grammar is not clean. A later run that re-ran nothing,
//     re-ran other rows, or wrote anything but a clean outcome for a row
//     replaces nothing; so does a last entry with a row it cannot align.
//     Supersession is never inferred inside one entry: a later table under
//     the same heading does not clear an earlier row in it. It is
//     POSITIONAL, not dated: the entry written last in the file is the
//     latest, whatever date its heading carries, because Evidence is
//     appended and a heading's date is free text the scan does not trust.
//     The held occurrences of the last entry itself are always read.
//
// The hygiene (fences, blockquotes, struck and inline code), the routing rule
// and the negation rule in unroutedHeldLine apply unchanged to what is read.
type heldLineRead struct {
	skip  bool     // the line is outside the scan's read scope
	prose bool     // a line outside every table, in an entry with a results table (rule 2)
	blank [][2]int // byte ranges of the line (cell contents) the scan must not read
	key   string   // the row key of an aligned results-table data row; "" for any other line
}

// heldScope is heldScanScope's decision: what to read of each line, and
// which earlier-entry rows the last entry re-ran (rule 3).
type heldScope struct {
	lines   []heldLineRead
	from    int             // the first line of the superseding last entry, or -1
	covered map[string]bool // the row keys the superseding entry re-ran
}

// supersedes reports whether the held occurrences found on the given lines
// (the scan's hits, in any order) that lie before the last entry are all
// replaced by it: every one is a keyed results-table row, and the last entry
// re-ran that key. A single hit it cannot classify keeps every hit read.
func (s heldScope) supersedes(hits []int) bool {
	if s.from < 0 {
		return false
	}
	for _, i := range hits {
		if i >= s.from {
			continue
		}
		if k := s.lines[i].key; k == "" || !s.covered[k] {
			return false
		}
	}
	return true
}

// heldEntryHeading reports whether line opens an Evidence entry: an ATX
// heading of level exactly three at column 0. An indented heading is not an
// entry (it may sit in a list item or be indented code).
func heldEntryHeading(line string) bool {
	t := strings.TrimRight(line, "\r")
	return t == "###" || strings.HasPrefix(t, "### ") || strings.HasPrefix(t, "###\t")
}

// heldFenceToggle is the fence test unroutedHeldLine applies, so the scope
// and the scan agree on which lines are inside fenced code.
func heldFenceToggle(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

// heldTableLine reports whether line is a table line: it starts with "|".
func heldTableLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "|")
}

// heldFence is an open CommonMark fence: its character and run length.
type heldFence struct {
	char byte
	n    int
}

// heldFenceCloses reports whether line closes the open fence f as CommonMark
// reads it: at most three spaces of indentation, a run of f's character at
// least as long as its opener, and nothing but spaces and tabs after.
func heldFenceCloses(f heldFence, line string) bool {
	t := strings.TrimRight(line, "\r")
	i := 0
	for i < len(t) && t[i] == ' ' {
		i++
	}
	if i > 3 {
		return false
	}
	n := 0
	for i+n < len(t) && t[i+n] == f.char {
		n++
	}
	return n >= f.n && strings.Trim(t[i+n:], " \t") == ""
}

// heldFenceOpens returns the fence line opens as CommonMark reads it, if any.
func heldFenceOpens(line string) (heldFence, bool) {
	t := strings.TrimRight(line, "\r")
	if !fenceLineValid(t) {
		return heldFence{}, false
	}
	t = strings.TrimLeft(t, " ")
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	return heldFence{t[0], n}, true
}

// heldHTMLOpen reports whether line starts an HTML block as CommonMark can
// read one (a "<" after leading whitespace, read here at ANY indentation —
// the stricter choice), and, for the block kinds that run across blank lines
// (htmlBlockEnds, a declaration), the end marker still to come: "" when the
// block ends at the next blank line or already ended on this line.
func heldHTMLOpen(line string) (html bool, end string) {
	t := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(t, "<") {
		return false, ""
	}
	lower := strings.ToLower(t)
	for _, k := range htmlBlockEnds {
		if !strings.HasPrefix(lower, k.open) {
			continue
		}
		if k.end[1] == '/' { // type 1: the tag name must end here
			if n := len(k.open); n < len(t) && strings.IndexByte(" \t>\r", t[n]) < 0 {
				continue
			}
		}
		if strings.Contains(lower[len(k.open):], k.end) {
			return true, ""
		}
		return true, k.end
	}
	if len(t) > 2 && t[1] == '!' && isASCIILetter(t[2]) { // a declaration, type 4
		if strings.Contains(t[2:], ">") {
			return true, ""
		}
		return true, ">"
	}
	return true, "" // types 6 and 7, or a paragraph that starts with "<": ends at a blank line
}

// heldRender classifies the lines of evidence for the structural reads rule 3
// makes (entry headings, the superseding marker, the covering tables): live[i]
// is true only where line i certainly renders as ordinary block text — outside
// fenced code and outside every HTML block. Every doubt is false:
//   - a fence is tracked twice, as CommonMark reads it (character, run length,
//     indentation, no info string on a closer) and as the scan reads it
//     (heldFenceToggle). From the first line where the two disagree, no line
//     is live, because the scan and the rendered page no longer agree on
//     what is code;
//   - a line that starts with "<" may open an HTML block: it, and every line
//     after it up to the next blank line, is not live; a block that runs
//     across blank lines (a comment, <pre>, …) is not live up to its end
//     marker, and a fence line inside any HTML block is a disagreement;
//   - a blank line is not live.
//
// Paragraph-level contexts (a blockquote's lazy continuation, a code span or
// strike running across lines) are judged by the callers, which accept a
// marker only where it opens its own paragraph.
func heldRender(lines []string) []bool {
	live := make([]bool, len(lines))
	var (
		lax, doubt bool
		open       *heldFence
		htmlEnd    string
		html       bool
	)
	for i, l := range lines {
		toggle := heldFenceToggle(l)
		switch {
		case doubt:
			continue
		case htmlEnd != "":
			if toggle {
				doubt = true
			} else if strings.Contains(strings.ToLower(l), htmlEnd) {
				htmlEnd = ""
			}
			continue
		case html:
			if isBlankLine(l) {
				html = false
			} else if toggle {
				doubt = true
			}
			continue
		}
		strict := false
		if open != nil {
			strict = heldFenceCloses(*open, l)
		} else if _, ok := heldFenceOpens(l); ok {
			strict = true
		}
		if toggle != strict {
			doubt = true
			continue
		}
		if toggle {
			if open != nil {
				open = nil
			} else {
				f, _ := heldFenceOpens(l)
				open = &f
			}
			lax = !lax
			continue
		}
		if lax || open != nil || isBlankLine(l) {
			continue
		}
		if h, end := heldHTMLOpen(l); h {
			html, htmlEnd = end == "", end
			continue
		}
		live[i] = true
	}
	return live
}

// heldVerdictTokenRe reads any VERIFY verdict token, including the ones
// verifyVerdictRe ignores (BLOCKED, PARTIAL, …).
var heldVerdictTokenRe = regexp.MustCompile(`VERIFY:[ \t]*[A-Za-z-]`)

// heldEntryPass reports whether the entry lines[lo:hi] (lo is its heading
// line) carries the live strict marker rule 3 requires: a line at column 0
// that begins with a strict **VERIFY: PASS…** marker (verifyVerdictBoldRe)
// holding none of "`", "<", "[", "\", "~" or "|", rendered live, and opening
// its own paragraph — the line before it is blank or the entry's heading, so
// it cannot continue a blockquote, a list item, a code span or a strike from
// the line above. No VERIFY token may follow the marker anywhere in the entry,
// in any context: a later verdict, even quoted, leaves the entry unsettled.
func heldEntryPass(lines []string, live []bool, lo, hi int) bool {
	at := -1
	for i := lo + 1; i < hi; i++ {
		m := verifyVerdictBoldRe.FindStringSubmatchIndex(lines[i])
		if m == nil || m[0] != 0 || lines[i][m[2]:m[3]] != "PASS" || !live[i] {
			continue
		}
		if strings.ContainsAny(lines[i][m[0]:m[1]], "`<[\\~|") {
			continue
		}
		if i-1 != lo && !isBlankLine(lines[i-1]) {
			continue
		}
		at = i
	}
	if at < 0 {
		return false
	}
	for i := at; i < hi; i++ {
		n := len(heldVerdictTokenRe.FindAllStringIndex(lines[i], -1))
		if i == at && n > 1 || i > at && n > 0 {
			return false
		}
	}
	return true
}

// heldCellRanges returns the byte range of each cell's content in a table
// line: the text between unescaped "|" delimiters (a `\|` is content, as
// splitRow reads it). The whitespace-only text before the first delimiter
// and after the last one is not a cell.
func heldCellRanges(line string) [][2]int {
	var pipes []int
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) && line[i+1] == '|' {
			i++
			continue
		}
		if line[i] == '|' {
			pipes = append(pipes, i)
		}
	}
	if len(pipes) == 0 {
		return nil
	}
	var out [][2]int
	if strings.TrimSpace(line[:pipes[0]]) != "" {
		out = append(out, [2]int{0, pipes[0]})
	}
	for k := 0; k+1 < len(pipes); k++ {
		out = append(out, [2]int{pipes[k] + 1, pipes[k+1]})
	}
	if last := pipes[len(pipes)-1]; strings.TrimSpace(line[last+1:]) != "" {
		out = append(out, [2]int{last + 1, len(line)})
	}
	return out
}

// heldNorm normalises a table cell for comparison: emphasis and surrounding
// space trimmed, lower case, inner whitespace collapsed.
func heldNorm(cell string) string {
	n := strings.ToLower(strings.Trim(strings.TrimSpace(cell), "*_ \t"))
	return strings.Join(strings.Fields(n), " ")
}

// heldKeyColumns and heldOtherColumns are the exact header names (heldNorm)
// rule 1 recognises: a row-key column, and a column that holds no row's
// result. Every other header — Exit, Result, Output, Observed, a note, and
// any longer spelling such as "Command (as run)" or "Runner verdict" — is a
// result column and is read.
var (
	heldKeyColumns   = map[string]bool{"#": true, "row": true, "no.": true, "no": true, "verify row": true, "command": true, "cmd": true}
	heldOtherColumns = map[string]bool{"expect": true, "expected": true, "date": true, "runner": true,
		"date / runner": true, "date/runner": true, "date · runner": true, "date runner": true}
)

// heldColumn classifies an Evidence table header cell. key is a row-key
// column; notResult is any column that is not a row's result (the key
// columns, Expect, Date, Runner).
func heldColumn(header string) (key, notResult bool) {
	n := heldNorm(header)
	switch {
	case heldKeyColumns[n]:
		return true, true
	case heldOtherColumns[n]:
		return false, true
	}
	return false, false
}

// heldRowKey reads a row key (rule 3): the key column's header name and the
// row's key cell, each normalised (heldNorm). Rows correspond only when both
// are equal, so a `#`-keyed row never matches a Command-keyed one, and two
// rows that render alike but are written differently never match. An empty
// key cell is not a key ("").
func heldRowKey(header, cell string) string {
	if k := heldNorm(cell); k != "" {
		return heldNorm(header) + "\x1f" + k
	}
	return ""
}

// heldSeparatorRow reports whether line is a table's header separator.
func heldSeparatorRow(line string) bool {
	t := strings.Trim(strings.TrimSpace(line), "|")
	return strings.Contains(t, "-") && separatorRowRe.MatchString(t)
}

// heldTable is one results table the scope recognised (rule 1).
type heldTable struct {
	header, end int    // the header line, and the line after the last row
	keyCol      int    // the first row-key column
	notResult   []bool // per column: not a result column
}

// heldScanScope returns what the held scan reads of evidence (split on "\n"),
// under the rules documented above.
func heldScanScope(evidence string) heldScope {
	lines := strings.Split(evidence, "\n")
	out := make([]heldLineRead, len(lines))
	live := heldRender(lines)
	inFence := make([]bool, len(lines)) // as the scan reads it: a toggle or a line inside fenced code
	fence := false
	for i, l := range lines {
		if heldFenceToggle(l) {
			fence = !fence
			inFence[i] = true
			continue
		}
		inFence[i] = fence
	}

	// Entries, split at live level-3 headings.
	starts := []int{0}
	for i, l := range lines {
		if i > 0 && live[i] && heldEntryHeading(l) {
			starts = append(starts, i)
		}
	}

	var tables []heldTable
	for e := range starts {
		lo, hi := starts[e], len(lines)
		if e+1 < len(starts) {
			hi = starts[e+1]
		}
		// Table blocks: runs of table lines outside fenced code.
		hasResults := false
		for i := lo; i < hi; {
			if inFence[i] || !heldTableLine(lines[i]) {
				i++
				continue
			}
			j := i
			for j < hi && !inFence[j] && heldTableLine(lines[j]) {
				j++
			}
			if j-i >= 2 && heldSeparatorRow(lines[i+1]) {
				header := heldCellRanges(lines[i])
				tb := heldTable{header: i, end: j, keyCol: -1, notResult: make([]bool, len(header))}
				hasResult := false
				for c, r := range header {
					k, nr := heldColumn(lines[i][r[0]:r[1]])
					if k && tb.keyCol < 0 {
						tb.keyCol = c
					}
					tb.notResult[c] = nr
					hasResult = hasResult || !nr
				}
				if tb.keyCol >= 0 && hasResult {
					hasResults = true
					tables = append(tables, tb)
					out[i].skip, out[i+1].skip = true, true // header and separator
					for r := i + 2; r < j; r++ {
						cells := heldCellRanges(lines[r])
						if len(cells) != len(header) {
							continue // cannot align: read whole, no key (fail closed)
						}
						for c, rg := range cells {
							if tb.notResult[c] {
								out[r].blank = append(out[r].blank, rg)
							}
						}
						kc := cells[tb.keyCol]
						hc := header[tb.keyCol]
						out[r].key = heldRowKey(lines[i][hc[0]:hc[1]], lines[r][kc[0]:kc[1]])
					}
				}
			}
			i = j
		}
		if !hasResults {
			continue
		}
		for i := lo; i < hi; i++ {
			if !inFence[i] && !heldTableLine(lines[i]) {
				out[i].prose = true
			}
		}
	}

	scope := heldScope{lines: out, from: -1}
	if len(starts) > 1 {
		last := starts[len(starts)-1]
		if heldEntryPass(lines, live, last, len(lines)) {
			if covered := heldCovered(lines, live, out, tables, last); len(covered) > 0 {
				scope.from, scope.covered = last, covered
			}
		}
	}
	return scope
}

// heldCovered returns the row keys the last entry (from line last on)
// re-ran clean. A key is covered only when EVERY keyed row of it in the last
// entry's results tables is clean, so a Verify row that ran two commands, or
// a second table that lists the same key, cannot cover a row the other line
// says was skipped. A row is clean only in a table that opens its own block
// (the line before it is blank or the entry heading, so it cannot continue a
// blockquote or a list item) with every line at column 0 and rendered live;
// a keyed row of any other results table in the entry counts as not clean.
// One row in the entry's results tables that cannot be aligned with its
// header leaves nothing covered: its key cannot be read, so it cannot be ruled
// out as a second line of a held row.
//
// A row is clean when its outcome columns (heldCoverColumn) carry at least
// one recognised clean outcome and nothing else that is visible (heldOutcome).
// An expectation column ("Expected exit", "Pass criteria") is never read for
// the outcome. Any other result column must be empty: a note, a check name or
// any text the cover cannot classify keeps the row from covering.
func heldCovered(lines []string, live []bool, out []heldLineRead, tables []heldTable, last int) map[string]bool {
	clean, bad := map[string]bool{}, map[string]bool{}
	for _, tb := range tables {
		if tb.header <= last {
			continue
		}
		verified := true
		if prev := tb.header - 1; prev != last && !isBlankLine(lines[prev]) {
			verified = false
		}
		for i := tb.header; i < tb.end; i++ {
			verified = verified && live[i] && strings.HasPrefix(lines[i], "|")
		}
		header := heldCellRanges(lines[tb.header])
		for r := tb.header + 2; r < tb.end; r++ {
			if out[r].key == "" {
				return nil // a row that cannot be aligned (fail closed)
			}
			cells := heldCellRanges(lines[r])
			ran, other := false, false
			for c, rg := range cells {
				if tb.notResult[c] {
					continue
				}
				role := heldCoverColumn(lines[tb.header][header[c][0]:header[c][1]])
				if role == heldColExpect {
					continue
				}
				st := heldOutcome(lines[r][rg[0]:rg[1]], role == heldColExit)
				if role == heldColOther && st != heldCellEmpty {
					st = heldCellOther
				}
				switch st {
				case heldCellPass:
					ran = true
				case heldCellOther:
					other = true
				}
			}
			if verified && ran && !other {
				clean[out[r].key] = true
			} else {
				bad[out[r].key] = true
			}
		}
	}
	covered := map[string]bool{}
	for k := range clean {
		if !bad[k] {
			covered[k] = true
		}
	}
	return covered
}

// heldColRole is what a result column of a covering table holds
// (heldCoverColumn).
type heldColRole int

const (
	heldColOther  heldColRole = iota // a note, a check name, any header the cover does not recognise
	heldColExit                      // an exit code: Exit, Exit code, rc
	heldColResult                    // the row's actual result: Result, Observed, Output, Outcome, Status
	heldColExpect                    // an expectation or criterion, never the outcome
)

var (
	// heldExitColumns and heldResultColumns are the exact header names
	// (heldNorm) the cover reads a row's outcome from.
	heldExitColumns   = map[string]bool{"exit": true, "exit code": true, "exit status": true, "rc": true}
	heldResultColumns = map[string]bool{"result": true, "results": true, "observed": true, "output": true,
		"outcome": true, "status": true, "actual": true, "actual result": true, "observed result": true, "verdict": true}
	// heldExpectColumnRe is a header that names an expectation or a criterion.
	heldExpectColumnRe = regexp.MustCompile(`\bexpect|\bcriteri`)
)

// heldCoverColumn classifies a result-column header for the cover.
func heldCoverColumn(header string) heldColRole {
	n := heldNorm(header)
	switch {
	case heldExitColumns[n]:
		return heldColExit
	case heldResultColumns[n]:
		return heldColResult
	case heldExpectColumnRe.MatchString(n):
		return heldColExpect
	}
	return heldColOther
}

// heldCellState is what a covering row's result cell says (heldOutcome).
type heldCellState int

const (
	heldCellEmpty heldCellState = iota // renders as nothing a reader could see
	heldCellPass                       // a recognised clean outcome
	heldCellOther                      // anything else: a placeholder, a carry-forward, markup, unrecognised text
)

var (
	// heldOutcomeRe is a recognised clean outcome at the start of what is
	// left of a result cell: "ok", "PASS", "passed", "green", "exit 0",
	// "rc=0", a check mark. A bare "0" is an outcome only as a whole Exit
	// cell (heldOutcome).
	heldOutcomeRe = regexp.MustCompile(`^(?:ok|pass(?:ed|es)?|green|clean|success(?:ful)?|exit(?:[ \t]+code)?[ \t]*[:=]?[ \t]*0|rc[ \t]*[:=]?[ \t]*0|✓|✔|✅)`)
	// heldDetailRes are the neutral details a clean outcome may carry after
	// it, the closed set heldDetailsClean accepts: a count of things that ran
	// (its numbers checked by heldDetailCounts), a count of zero failures, a
	// duration, or the clean outcome again ("exit 0, ok").
	heldDetailRes = []*regexp.Regexp{
		regexp.MustCompile(`^(\d+)[ \t]+(?:tests?|subtests?|checks?|cases?|files?|rows?|assertions?|packages?)(?:[ \t]+(?:passed|passing|ok|green|run|ran))?`),
		regexp.MustCompile(`^(\d+)[ \t]+(?:passed|passing)`),
		regexp.MustCompile(`^(\d+)[ \t]*/[ \t]*(\d+)(?:[ \t]+(?:passed|passing|ok|green))?`),
		regexp.MustCompile(`^(\d+)[ \t]+of[ \t]+(\d+)(?:[ \t]+(?:tests?|subtests?|checks?|cases?|rows?))?[ \t]+(?:passed|passing|ok|green)`),
		regexp.MustCompile(`^(?:0[ \t]+(?:failures?|failed|fails|errors?|findings?)|no[ \t]+(?:failures|errors|findings))`),
		regexp.MustCompile(`^(?:in[ \t]+)?\d+(?:\.\d+)?[ \t]*(?:minutes?|mins?|ms|m|seconds?|secs?|s)`),
		heldOutcomeRe,
	}
	// heldStrikeRe is a struck span as GFM renders one: the text between the
	// tildes neither starts nor ends with a space. "~~ HELD ~~" is not a
	// strike on GitHub, so the cover keeps it.
	heldStrikeRe = regexp.MustCompile(`~~[^~ \t](?:[^~]*[^~ \t])?~~`)
	// heldDetailSepRe is the punctuation, emphasis and space between details.
	heldDetailSepRe = regexp.MustCompile("^[ \t,;:.()\\[\\]·—–*_`-]+")
)

// heldOutcome classifies one outcome cell of a covering row by what it renders
// as; exit says the cell is in an Exit column. Struck spans that render as a
// strike (heldStrikeRe) are removed first.
// The WHOLE rest of the cell must then read as a clean outcome: a bare "0" as
// a whole Exit cell, or a recognised outcome (heldOutcomeRe) followed only by
// neutral details (heldDetailsClean). The grammar is closed — letters, digits,
// spaces, a few punctuation marks and the check marks — so anything else is
// heldCellOther: a placeholder (—, n/a), a carry-forward (same as Run 1), a
// note after the outcome ("PASS (no re-run)"), a zero count ("0 checks run"),
// unrecognised text, and markup or an invisible format character that may
// render as nothing or hide text (<!-- -->, <br>, &nbsp;, a backslash escape,
// a zero-width space), none of which the grammar admits.
func heldOutcome(cell string, exit bool) heldCellState {
	cell = heldStrikeRe.ReplaceAllString(cell, "")
	n := strings.Trim(heldNorm(cell), "`*_ ")
	if n == "" {
		return heldCellEmpty
	}
	if exit && n == "0" {
		return heldCellPass
	}
	m := heldOutcomeRe.FindString(n)
	if m == "" || !heldDetailsClean(n[len(m):]) {
		return heldCellOther
	}
	return heldCellPass
}

// heldDetailsClean reports whether t, the text after a clean outcome, is
// nothing but neutral details (heldDetailRes), optionally separated by
// punctuation, emphasis and space. Whatever is left after the last detail
// must itself be a detail, so text outside the closed set never passes.
func heldDetailsClean(t string) bool {
	for {
		t = heldDetailSepRe.ReplaceAllString(t, "")
		if t == "" {
			return true
		}
		matched := false
		for _, re := range heldDetailRes {
			m := re.FindStringSubmatch(t)
			if m == nil {
				continue
			}
			if !heldDetailCounts(m[1:]) {
				return false
			}
			t, matched = t[len(m[0]):], true
			break
		}
		if !matched {
			return false
		}
	}
}

// heldDetailCounts checks the numbers a count detail carries: every count of
// things that ran is above zero, and a ratio ("3/3", "3 of 3 passed") passed
// all it ran.
func heldDetailCounts(nums []string) bool {
	for _, s := range nums {
		if n, err := strconv.Atoi(s); err != nil || n == 0 {
			return false
		}
	}
	return len(nums) < 2 || nums[0] == nums[1]
}

var (
	// heldExitAfterRe is an exit-code mapping right after the marker:
	// "could-not-check → exit 6", "-> exit code 2", "= exit 6".
	heldExitAfterRe = regexp.MustCompile(`(?i)^[ \t*_]*(?:→|->|=>|=)[ \t*_]*exit(?:[ \t]+code)?[ \t]*\d`)
	// heldExitBeforeRe is an exit code whose meaning the marker names in
	// parentheses, right before it: "exit 6 (could-not-check".
	heldExitBeforeRe = regexp.MustCompile(`(?i)\bexit(?:[ \t]+code)?[ \t]*\d+[ \t]*\([ \t*_]*$`)
	// heldNamesRowRe is a line that names a row: "row 2", "rows", "#4".
	heldNamesRowRe = regexp.MustCompile(`(?i)\brows?\b|#[ \t]*\d`)
)

// heldExitMapping reports whether the occurrence at clean[h[0]:h[1]] is
// written as an exit-code mapping on a line that names no row (rule 2) rather
// than a disposition.
func heldExitMapping(clean string, h []int) bool {
	if heldNamesRowRe.MatchString(clean) {
		return false
	}
	return heldExitAfterRe.MatchString(clean[h[1]:]) || heldExitBeforeRe.MatchString(clean[:h[0]])
}

// heldBlankRanges overwrites the given byte ranges of s with spaces. The
// length is unchanged, so offsets into s still agree with the line's.
func heldBlankRanges(s string, rs [][2]int) string {
	if len(rs) == 0 {
		return s
	}
	b := []byte(s)
	for _, r := range rs {
		for k := r[0]; k < r[1] && k < len(b); k++ {
			b[k] = ' '
		}
	}
	return string(b)
}
