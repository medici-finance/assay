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
// honoured marker at all. Neither has a cell whose prose carries a `$` in any
// spelling (bare, backslash-escaped, or a character reference such as `&#36;`;
// any character reference counts): GitHub renders a span wrapped in dollar signs
// as math, not code, after resolving escapes and references.
//
// Every code span must also stand clear of the prose before it: its opening run
// follows whitespace, the start of the cell, or a run of `(` that does. An
// opener fused to the text before it (a URL, a dollar, `~~`) may not render as
// the span this scan paired, and a swallowed opener shifts every later pairing,
// so such a cell honours no marker. The marker span is held tighter: its opening
// run follows whitespace or the start of the cell, and its closing run ends the
// cell or is followed by whitespace or plain punctuation.
package verifycmd

import (
	"regexp"
	"strings"
)

// Marker is the prefix that names a code span as the row's command.
const Marker = "cmd:"

// Marked returns the command the cell's first honoured `cmd:` span names, or
// ok=false when the cell carries none. A cell with ANY marker span that does
// not stand clear of its prose honours none, even after an earlier good one.
func Marked(cell string) (string, bool) {
	spans, plain := renderedCodeSpans(cell)
	if !plain {
		return "", false
	}
	// Every marker span is checked, not just the first: one that does not
	// stand clear of its prose refuses the whole cell, exactly as statusgen's
	// markedCommands does, so the two scanners never pick different commands.
	first, found := "", false
	for _, sp := range spans {
		if !strings.HasPrefix(sp.text, Marker) {
			continue
		}
		c := strings.TrimSpace(strings.TrimPrefix(sp.text, Marker))
		if c == "" {
			continue
		}
		if !sp.spaceLed {
			return "", false
		}
		if !sp.cleanEnd {
			return "", false
		}
		if !found {
			first, found = c, true
		}
	}
	return first, found
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

// CommentOnly reports whether a lifted command is a shell comment: its first
// non-blank character is `#`. The shell ignores such a command, so it exits 0
// with no output and measures nothing (#2684, an inline `## Coverage` heading
// lifted as the command). An executor MUST NOT run it and report the zero exit
// as a pass. statusgen's shellCommentOnly is the same rule; both are held to
// statusgen/testdata/shell-comment-vectors.json. A `#` later in the command (a
// trailing comment, a quoted argument) leaves a real command.
func CommentOnly(cmd string) bool {
	return strings.HasPrefix(strings.TrimSpace(cmd), "#")
}

// CommentLed returns why the command a Verify Command cell names is only a shell
// comment, or "" when it names a real one. An executor MUST NOT run a comment-led
// cell: the shell ignores the comment, and what is left exits 0 with no output (a
// whole raw cell such as "`## Coverage`: confirm it" runs as the `:` no-op once the
// comment-only substitution expands to nothing), so the zero exit would record a
// pass for a check that never looked (#2684, review S1).
//
// A marked cell is judged on its marked command, and a marked comment is refused
// too: the marker names the command to run, and a comment runs nothing (review
// F2; statusgen refuses it the same way). An unmarked cell is judged on BOTH
// lifts: statusgen's (the first code span, firstSpan) and this package's legacy
// Lift (the whole unwrapped cell, which is what runs). Either one comment-only
// refuses the cell, so a cell statusgen records could-not-run is never one these
// executors run. Both modules are held to the "cells" section of
// statusgen/testdata/shell-comment-vectors.json.
func CommentLed(cell string) string {
	if c, ok := Marked(cell); ok {
		if CommentOnly(c) {
			return MarkedCommentWhy
		}
		return ""
	}
	if CommentOnly(firstSpan(cell)) || CommentOnly(stripInlineCode(cell)) {
		return UnmarkedCommentWhy
	}
	return ""
}

// The could-not-run reasons CommentLed returns. Each names the remedy that fits:
// a marked row is already marked, so it needs the comment replaced.
const (
	MarkedCommentWhy   = "the `cmd:`-marked command is only a shell comment, so it runs nothing; replace it with the command that performs the check"
	UnmarkedCommentWhy = "the Verify command is a shell comment (an inline heading such as `## Coverage` lifts as one), so it runs nothing; mark the real command with a `cmd:` code span, or keep the row a manual procedure with its evidence"
)

// firstSpan is statusgen's unmarked lift (statusgen/verifyrows.go codeSpan),
// ported so CommentLed judges the string statusgen judges: the content of the
// cell's first code span — a run of N backticks closing at the next run of N, an
// unclosed run taking the rest of the cell — or the trimmed cell when it has no
// backtick. It is a judging lift only; Lift stays the executed one.
func firstSpan(cell string) string {
	s := strings.TrimSpace(cell)
	start := strings.Index(s, "`")
	if start < 0 {
		return s
	}
	fence := strings.Repeat("`", backtickRun(s, start))
	rest := s[start+len(fence):]
	end := strings.Index(rest, fence)
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
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

// renderedCodeSpans returns every code span a renderer displays in cell, trimmed
// and with how it sits in the cell, and plain=false when the cell's prose
// carries an unescaped `<` or `[`, a `$` in any spelling, a character reference,
// or a span whose opening run is fused to the text before it.
func renderedCodeSpans(cell string) (spans []renderedSpan, plain bool) {
	plain = true
	i := 0
	for i < len(cell) {
		c := cell[i]
		switch {
		case c == '\\' && i+1 < len(cell) && isASCIIPunct(cell[i+1]):
			if cell[i+1] == '$' {
				plain = false // an escaped dollar still opens math on GitHub
			}
			i += 2
		case c == '<' || c == '[':
			plain = false
			i++
		case c == '$':
			plain = false
			i++
		case c == '&' && charRefRe.MatchString(cell[i:]):
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
			if !parenLed(cell, i) {
				plain = false // the opener is fused to the prose before it
			}
			spans = append(spans, renderedSpan{
				text:     strings.TrimSpace(cell[i+n : closeAt]),
				spaceLed: i == 0 || isSpaceByte(cell[i-1]),
				cleanEnd: cleanSpanEnd(cell, closeAt+n),
			})
			i = closeAt + n
		default:
			i++
		}
	}
	return spans, plain
}

// renderedSpan is one code span: its trimmed content; spaceLed, the opening run
// is at the start of the cell or right after whitespace; cleanEnd, the closing
// run is at the end of the cell or right before whitespace or plain punctuation.
type renderedSpan struct {
	text     string
	spaceLed bool
	cleanEnd bool
}

// charRefRe matches a character reference at the start of a string: named
// (`&dollar;`), decimal (`&#36;`) or hex (`&#x24;`).
var charRefRe = regexp.MustCompile(`^&(#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)

func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' }

// parenLed reports whether the backtick run at cell[i] follows whitespace or the
// start of the cell, allowing a run of `(` between them.
func parenLed(cell string, i int) bool {
	j := i
	for j > 0 && cell[j-1] == '(' {
		j--
	}
	return j == 0 || isSpaceByte(cell[j-1])
}

// cleanSpanEnd reports whether a closing run ending just before cell[k] is at the
// end of the cell or followed by whitespace or plain punctuation.
func cleanSpanEnd(cell string, k int) bool {
	return k == len(cell) || isSpaceByte(cell[k]) || strings.IndexByte(".,;:!?)", cell[k]) >= 0
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
