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
	"path/filepath"
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

// Lift returns the command a Verify Command cell names, exactly as statusgen's
// verifyCommand lifts it: the first honoured `cmd:` span when there is one,
// otherwise the content of the cell's first code span (firstSpan), or the trimmed
// cell when it has no backtick. The executors run this string and nothing else.
// They once ran the whole unwrapped cell instead, where "`false`: confirm" ran as
// a command substitution and then the `:` builtin, so the zero exit recorded a
// pass for a check that failed (#3147).
func Lift(cell string) string {
	if c, ok := Marked(cell); ok {
		return c
	}
	return firstSpan(cell)
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

// NoOp reports whether a lifted command is a shell no-op that measures nothing:
// the `:` builtin with no command after it (`:` or `: confirm it`), or a cmd.exe
// remark (`REM …`, `@rem …`, `:: …`). Like a comment it exits 0 having checked
// nothing, so a zero exit is no evidence (#3147 A2). The rule does not depend on
// the shell a row names: none of these forms is a check under any shell. A `:`
// followed by a separator (`: ; test -d .`) leaves a real command, and bare
// `true` / `exit 0` are not judged here. statusgen's shellNoOp is the same rule.
func NoOp(cmd string) bool {
	c := strings.TrimSpace(cmd)
	if strings.HasPrefix(c, "::") {
		return true
	}
	f := strings.Fields(c)
	if len(f) == 0 {
		return false
	}
	if strings.ToLower(strings.TrimPrefix(f[0], "@")) == "rem" {
		return true
	}
	return f[0] == ":" && !strings.ContainsAny(c, ";&|\n")
}

// CommentLed returns why the command a Verify Command cell names is only a shell
// comment, or "" when it is not. A marked cell is judged on its marked command,
// and a marked comment is refused too: the marker names the command to run, and
// a comment runs nothing (#2684 review F2). An unmarked cell is judged on its
// first code span, which is both statusgen's lift and the string these executors
// run (#3147), so the predicate and the executed string can no longer differ
// (#2684 review S1).
func CommentLed(cell string) string {
	if c, ok := Marked(cell); ok {
		if CommentOnly(c) {
			return MarkedCommentWhy
		}
		return ""
	}
	if CommentOnly(firstSpan(cell)) {
		return UnmarkedCommentWhy
	}
	return ""
}

// Refused returns why the command a Verify Command cell names must not run, or ""
// when Lift(cell) may run. It refuses what statusgen refuses, in statusgen's
// order: a shell comment (CommentLed); a shell no-op (NoOp), marked or not; for
// an unmarked cell, a first span that is a prose mention rather than a command
// (statusgen's proseLedCommandWhy, ported); and a lift that is blank. An executor
// records a refused row could-not-run and never executes it. The executors and
// statusgen are held to the "cells" section of
// statusgen/testdata/shell-comment-vectors.json, verdict for verdict (#3147).
func Refused(cell string) string {
	if why := CommentLed(cell); why != "" {
		return why
	}
	lift := Lift(cell)
	if NoOp(lift) {
		return NoOpWhy
	}
	if _, ok := Marked(cell); !ok {
		if why := proseLed(cell); why != "" {
			return "the first code span is " + why + ", not a command; " + proseLedFix
		}
	}
	if strings.TrimSpace(lift) == "" {
		return NoCommandWhy
	}
	return ""
}

// CommandRefused judges an already-lifted command for a row that carries no
// cell: why it must not run (blank, a shell comment, or a no-op), or "".
func CommandRefused(cmd string) string {
	switch {
	case strings.TrimSpace(cmd) == "":
		return NoCommandWhy
	case CommentOnly(cmd):
		return UnmarkedCommentWhy
	case NoOp(cmd):
		return NoOpWhy
	}
	return ""
}

// The could-not-run reasons Refused returns. Each names the remedy that fits:
// a marked row is already marked, so it needs the comment replaced.
const (
	MarkedCommentWhy   = "the `cmd:`-marked command is only a shell comment, so it runs nothing; replace it with the command that performs the check"
	UnmarkedCommentWhy = "the Verify command is a shell comment (an inline heading such as `## Coverage` lifts as one), so it runs nothing; mark the real command with a `cmd:` code span, or keep the row a manual procedure with its evidence"
	NoOpWhy            = "the Verify command is a shell no-op (`:` or a cmd.exe REM / `::` remark), so it exits 0 having checked nothing; replace the no-op with the command that performs the check"
	NoCommandWhy       = "the Verify command cell names no command (its first code span is blank)"
	proseLedFix        = "mark the command with a `cmd:` code span"
)

// firstSpan is statusgen's unmarked lift (statusgen/verifyrows.go codeSpan),
// ported so these executors run the string statusgen runs: the content of the
// cell's first code span — a run of N backticks closing at the next run of N, an
// unclosed run taking the rest of the cell — or the trimmed cell when it has no
// backtick.
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

// codeSpans is statusgen's codeSpans, ported: the content of every terminated
// code span in cell, in order, each trimmed. An unterminated span ends the scan.
func codeSpans(cell string) []string {
	var out []string
	s := cell
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			return out
		}
		fence := strings.Repeat("`", backtickRun(s, start))
		rest := s[start+len(fence):]
		end := strings.Index(rest, fence)
		if end < 0 {
			return out
		}
		out = append(out, strings.TrimSpace(rest[:end]))
		s = rest[end+len(fence):]
	}
}

// proseOutsideSpans is statusgen's proseOutsideSpans, ported: the cell with every
// terminated code span removed, trimmed.
func proseOutsideSpans(cell string) string {
	var b strings.Builder
	s := cell
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		fence := strings.Repeat("`", backtickRun(s, start))
		rest := s[start+len(fence):]
		end := strings.Index(rest, fence)
		if end < 0 {
			b.WriteString(s[start:])
			break
		}
		s = rest[end+len(fence):]
	}
	return strings.TrimSpace(b.String())
}

// proseLed is statusgen's proseLedCommandWhy for an unmarked cell whose lift is
// neither a comment nor a no-op, ported: why the first code span is a prose
// MENTION rather than a command, or "". A cell that is one span and nothing else,
// and a first span with whitespace in it, are never flagged.
func proseLed(cell string) string {
	spans := codeSpans(cell)
	if len(spans) == 0 {
		return ""
	}
	if len(spans) == 1 && proseOutsideSpans(cell) == "" {
		return ""
	}
	first := spans[0]
	if first == "" || strings.ContainsAny(first, " \t") {
		return ""
	}
	return proseLedShape(first, spans[1:])
}

// proseLedShape is statusgen's proseLedShape, ported verbatim in behaviour.
func proseLedShape(first string, later []string) string {
	ext := strings.ToLower(filepath.Ext(first))
	switch {
	case first == Marker:
		return "a bare mention of the `cmd:` marker (a marker needs a command after it)"
	case strings.HasPrefix(first, ".") && !strings.HasPrefix(first, "./") && !strings.HasPrefix(first, "../") && first != ".":
		return "a file extension or dotfile name"
	case nonExecExtensions[ext]:
		return "a " + ext + " file name"
	case strings.Contains(first, "/") && !scriptExtensions[ext] &&
		!strings.HasPrefix(first, "./") && !strings.HasPrefix(first, "../") &&
		!strings.HasPrefix(first, "/") && !strings.HasPrefix(first, "~") && !strings.HasPrefix(first, "$"):
		return "a path or `owner/repo` / `stream/NN` reference"
	case codeIdentRe.MatchString(first) && (strings.ToLower(first) != first || strings.HasSuffix(first, "()")):
		return "a code identifier"
	case bareWordRe.MatchString(first):
		for _, sp := range later {
			if strings.ContainsAny(strings.TrimSpace(sp), " \t") {
				return "a lone word mentioned ahead of the command span"
			}
		}
	}
	return ""
}

// nonExecExtensions, scriptExtensions, codeIdentRe and bareWordRe are statusgen's
// (statusgen/verifyrows.go), ported for proseLedShape.
var nonExecExtensions = map[string]bool{
	".go": true, ".md": true, ".yaml": true, ".yml": true, ".json": true, ".jsonl": true,
	".toml": true, ".txt": true, ".mod": true, ".sum": true, ".ts": true,
	".tsx": true, ".js": true, ".html": true, ".css": true, ".csv": true, ".lock": true,
	".xml": true, ".proto": true, ".rs": true, ".tf": true, ".svg": true, ".png": true,
	".pdf": true,
}

var scriptExtensions = map[string]bool{
	".sh": true, ".bash": true, ".py": true, ".ps1": true, ".exe": true, ".rb": true, ".pl": true,
}

var (
	codeIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*(\(\))?$`)
	bareWordRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

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
