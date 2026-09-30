// Package verifycmd lifts the command out of a brief's `## Verify` Command cell
// for the tools/desk executors that run Verify rows (verifyloop's deterministic
// runner and deskrebaseline's command probe).
//
// A Command cell may name its command with the explicit marker: a code span whose
// content starts `cmd:` (spec/brief-v1.md §4.4). An executor MUST prefer the first
// honoured marked span over any other text in the cell. statusgen implements the
// same rule for verifyrun and the check:ci lane (statusgen/verifyrows.go,
// markedCommands); the two implementations live in different Go modules, so both
// are held to ONE shared vector table, statusgen/testdata/cmd-marker-vectors.json.
//
// A marker is honoured only where the rendered brief shows it as code. The scan
// follows CommonMark: outside a code span a backslash escapes the ASCII
// punctuation after it, and a run of N backticks closes only on the next run of
// exactly N (an opener with no closer is literal text). A cell whose prose
// outside code spans carries an unescaped `<` or `[` — raw HTML, an HTML comment,
// a link or an image, any of which can hide text from the rendered table — has no
// honoured marker at all. Neither has a cell whose prose carries an unescaped
// `$`: GitHub renders a span wrapped in dollar signs as math, not code.
package verifycmd

import "strings"

// Marker is the prefix that names a code span as the row's command.
const Marker = "cmd:"

// Marked returns the command the cell's first honoured `cmd:` span names, or
// ok=false when the cell carries none.
func Marked(cell string) (string, bool) {
	spans, plain := renderedCodeSpans(cell)
	if !plain {
		return "", false
	}
	for _, sp := range spans {
		if !strings.HasPrefix(sp, Marker) {
			continue
		}
		if c := strings.TrimSpace(strings.TrimPrefix(sp, Marker)); c != "" {
			return c, true
		}
	}
	return "", false
}

// Lift returns the command a Verify Command cell names: the first honoured
// `cmd:` span when there is one, otherwise the tools/desk legacy lift — the
// cell with one wrapping pair of backticks removed, or the cell unchanged when it
// is not wrapped. A cell with no marker therefore lifts exactly what these
// executors lifted before the marker existed.
func Lift(cell string) string {
	if c, ok := Marked(cell); ok {
		return c
	}
	return stripInlineCode(cell)
}

// stripInlineCode unwraps a `…` inline-code span that wraps the whole cell; a
// cell with no wrapping backticks passes through unchanged.
func stripInlineCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		return strings.TrimSpace(strings.Trim(s, "`"))
	}
	return s
}

// renderedCodeSpans returns the trimmed content of every code span a renderer
// displays in cell, and plain=false when the cell's prose carries an unescaped
// `<`, `[` or `$`.
func renderedCodeSpans(cell string) (spans []string, plain bool) {
	plain = true
	i := 0
	for i < len(cell) {
		c := cell[i]
		switch {
		case c == '\\' && i+1 < len(cell) && isASCIIPunct(cell[i+1]):
			i += 2
		case c == '<' || c == '[':
			plain = false
			i++
		case c == '$':
			plain = false
			i++
		case c == '`':
			n := backtickRun(cell, i)
			closeAt := -1
			for j := i + n; j < len(cell); {
				if cell[j] != '`' {
					j++
					continue
				}
				m := backtickRun(cell, j)
				if m == n {
					closeAt = j
					break
				}
				j += m
			}
			if closeAt < 0 {
				i += n
				continue
			}
			spans = append(spans, strings.TrimSpace(cell[i+n:closeAt]))
			i = closeAt + n
		default:
			i++
		}
	}
	return spans, plain
}

func backtickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

func isASCIIPunct(b byte) bool {
	return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", b) >= 0
}
