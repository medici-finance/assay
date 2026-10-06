package main

import (
	"regexp"
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
//   - a HELD/could-not-check line in an EARLIER entry that a later run has
//     replaced with a recorded strict PASS;
//   - an Expect (or Command) cell that names could-not-check as the
//     behaviour under test ("unreadable diff → could-not-check exit 6");
//   - prose beside the table that states what an exit code means
//     ("could-not-check→exit 6").
//
// heldScanScope decides, line by line, what the scan reads. Every rule
// narrows only where the text is positively classified, and falls back to
// the old whole-text read where it is not:
//
//  1. Entries. The Evidence body is split at `### ` headings outside fenced
//     code (a `####` heading stays inside its entry). Text before the first
//     such heading is the first entry. When there are two or more entries
//     and the LAST one supersedes (heldEntrySupersedes: a live strict
//     **VERIFY: PASS** marker, and its last verdict token is PASS), only that
//     entry is read. Otherwise every entry is read, as before. Supersession
//     is never inferred inside one entry: a later table under the same
//     heading does not clear an earlier row in it.
//  2. Result cells. In an entry that holds an Evidence results table — a
//     header naming a row key ("#", "row", "Command") and at least one other
//     column that is not Command, Expect, Date or Runner — each data row of
//     that table is read in its result cells only. A row whose cell count
//     differs from its header's (an unescaped "|" in a command) cannot be
//     aligned and is read whole. A table of any other shape is read whole.
//  3. Exit-code prose. In an entry that holds such a results table, a line
//     outside every table is still read, with one exception: an occurrence
//     written as an exit-code mapping — "could-not-check → exit 6",
//     "could-not-check = exit 6", "exit 6 (could-not-check)" — names what an
//     exit code means (heldExitMapping). It is not a row's disposition: that
//     is recorded in the row's own result cell, which rule 2 reads. In an
//     entry with no results table there is no result cell to fall back on,
//     so every occurrence in its prose is read, as before. Any other prose
//     occurrence — a verdict-line summary "row 4 HELD", a note — is read.
//
// The hygiene (fences, blockquotes, struck and inline code), the routing rule
// and the negation rule in unroutedHeldLine apply unchanged to what is read.
type heldLineRead struct {
	skip  bool     // the line is outside the scan's read scope
	prose bool     // a line outside every table, in an entry with a results table (rule 3)
	blank [][2]int // byte ranges of the line (cell contents) the scan must not read
}

// heldEntryHeading reports whether line opens an Evidence entry: an ATX
// heading of level exactly three, indented at most three spaces.
func heldEntryHeading(line string) bool {
	t := strings.TrimRight(line, "\r")
	i := 0
	for i < len(t) && i < 4 && t[i] == ' ' {
		i++
	}
	if i > 3 {
		return false
	}
	t = t[i:]
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

// heldAnyVerdictRe reads any VERIFY verdict token, including the ones
// verifyVerdictRe ignores (BLOCKED, PARTIAL, …), so an entry whose last word
// is not PASS never supersedes the entries before it.
var heldAnyVerdictRe = regexp.MustCompile(`VERIFY:[ \t]*([A-Za-z-]+)`)

// heldEntrySupersedes reports whether an Evidence entry carries a live strict
// **VERIFY: PASS** marker — outside fenced code, blockquotes and struck spans
// — and its last verdict token of any kind is PASS, with no FAIL after the
// strict marker. Only such an entry replaces the entries before it in the
// held scan. A loose-form PASS, a struck or quoted one, or a later BLOCKED,
// PARTIAL or FAIL leaves every entry in scope.
func heldEntrySupersedes(entry string) bool {
	strict := false
	last := ""
	inFence := false
	for _, line := range strings.Split(entry, "\n") {
		if heldFenceToggle(line) {
			inFence = !inFence
			continue
		}
		if inFence || strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		line = strikethroughRe.ReplaceAllString(line, "")
		for _, m := range verifyVerdictBoldRe.FindAllStringSubmatch(line, -1) {
			if m[1] == "PASS" {
				strict = true
			}
		}
		for _, m := range heldAnyVerdictRe.FindAllStringSubmatch(line, -1) {
			last = m[1]
		}
	}
	return strict && last == "PASS" && !verdictFailAfterStrictPass(entry)
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

// heldColumn classifies an Evidence table header cell. key is a row-key
// column ("#", "row", "Command"); notResult is any column that is not a
// row's result (the key columns, Expect, Date, Runner). Every other column —
// Exit, Result, Output, Observed, a note — is a result column and is read.
func heldColumn(header string) (key, notResult bool) {
	n := strings.ToLower(strings.Trim(strings.TrimSpace(header), "*_` "))
	switch {
	case n == "#" || n == "row" || n == "no." || strings.HasPrefix(n, "command") || strings.HasPrefix(n, "cmd"):
		return true, true
	case strings.HasPrefix(n, "expect") || strings.HasPrefix(n, "date") || strings.HasPrefix(n, "runner"):
		return false, true
	}
	return false, false
}

// heldSeparatorRow reports whether line is a table's header separator.
func heldSeparatorRow(line string) bool {
	t := strings.Trim(strings.TrimSpace(line), "|")
	return strings.Contains(t, "-") && separatorRowRe.MatchString(t)
}

// heldScanScope returns, for each line of evidence (split on "\n"), what the
// held scan reads of it, under the rules documented above.
func heldScanScope(evidence string) []heldLineRead {
	lines := strings.Split(evidence, "\n")
	out := make([]heldLineRead, len(lines))
	inFence := make([]bool, len(lines)) // a fence toggle or a line inside fenced code

	// Entries, split at level-3 headings outside fenced code.
	starts := []int{0}
	fence := false
	for i, l := range lines {
		if heldFenceToggle(l) {
			fence = !fence
			inFence[i] = true
			continue
		}
		inFence[i] = fence
		if !fence && i > 0 && heldEntryHeading(l) {
			starts = append(starts, i)
		}
	}
	first := 0
	if len(starts) > 1 {
		last := starts[len(starts)-1]
		if heldEntrySupersedes(strings.Join(lines[last:], "\n")) {
			first = len(starts) - 1
			for i := 0; i < last; i++ {
				out[i].skip = true
			}
		}
	}

	for e := first; e < len(starts); e++ {
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
				notResult := make([]bool, len(header))
				hasKey, hasResult := false, false
				for c, r := range header {
					k, nr := heldColumn(lines[i][r[0]:r[1]])
					hasKey = hasKey || k
					notResult[c] = nr
					hasResult = hasResult || !nr
				}
				if hasKey && hasResult {
					hasResults = true
					out[i].skip, out[i+1].skip = true, true // header and separator
					for r := i + 2; r < j; r++ {
						cells := heldCellRanges(lines[r])
						if len(cells) != len(header) {
							continue // cannot align: read whole (fail closed)
						}
						for c, rg := range cells {
							if notResult[c] {
								out[r].blank = append(out[r].blank, rg)
							}
						}
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
	return out
}

var (
	// heldExitAfterRe is an exit-code mapping right after the marker:
	// "could-not-check → exit 6", "-> exit code 2", "= exit 6".
	heldExitAfterRe = regexp.MustCompile(`(?i)^[ \t*_]*(?:→|->|=>|=)[ \t*_]*exit(?:[ \t]+code)?[ \t]*\d`)
	// heldExitBeforeRe is an exit code whose meaning the marker names in
	// parentheses, right before it: "exit 6 (could-not-check".
	heldExitBeforeRe = regexp.MustCompile(`(?i)\bexit(?:[ \t]+code)?[ \t]*\d+[ \t]*\([ \t*_]*$`)
)

// heldExitMapping reports whether the occurrence at clean[h[0]:h[1]] is
// written as an exit-code mapping (rule 3 above) rather than a disposition.
func heldExitMapping(clean string, h []int) bool {
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
