// Package verifycmd lifts the command out of a brief's `## Verify` Command cell
// for the tools/desk executors that run Verify rows (verifyloop's deterministic
// runner and deskrebaseline's command probe).
//
// A Command cell may name its command with the explicit marker: a code span whose
// content starts `cmd:` (spec/brief-v1.md §4.4). An executor MUST prefer the first
// honoured marked span over any other text in the cell.
//
// This package holds NO lexical rule of its own (tool-refactoring/03). The
// marker scan, the first-span lift, the shell comment and no-op verdicts and the
// prose-led verdict all live in statusgen/briefdoc (verification.go), which
// statusgen's verifyrun and these executors both call; what remains here is the
// executors' could-not-run wording and the row splitter whose edge contract
// differs from briefdoc's (SplitRow). Both consumers are still held to the shared
// vector tables, statusgen/testdata/cmd-marker-vectors.json and
// shell-comment-vectors.json.
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
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

// Marker is the prefix that names a code span as the row's command.
const Marker = briefdoc.CommandMarker

// Marked returns the command the cell's first honoured `cmd:` span names, or
// ok=false when the cell carries none (briefdoc.MarkedCommands: a cell with ANY
// marker span that does not stand clear of its prose honours none).
func Marked(cell string) (string, bool) {
	if m := briefdoc.MarkedCommands(cell); len(m) > 0 {
		return m[0], true
	}
	return "", false
}

// Lift returns the command a Verify Command cell names (briefdoc.VerifyCommand):
// the first honoured `cmd:` span when there is one, otherwise the content of the
// cell's first code span, or the trimmed cell when it has no backtick. The
// executors run this string and nothing else. They once ran the whole unwrapped
// cell instead, where "`false`: confirm" ran as a command substitution and then
// the `:` builtin, so the zero exit recorded a pass for a check that failed
// (#3147).
func Lift(cell string) string { return briefdoc.VerifyCommand(cell) }

// CommentOnly reports whether a lifted command is a shell comment
// (briefdoc.ShellCommentOnly): its first non-blank character is `#`. The shell
// ignores such a command, so it exits 0 with no output and measures nothing
// (#2684); an executor MUST NOT run it and report the zero exit as a pass.
func CommentOnly(cmd string) bool { return briefdoc.ShellCommentOnly(cmd) }

// NoOp reports whether a lifted command is a shell no-op that measures nothing
// (briefdoc.ShellNoOp): the `:` builtin with no command after it, or a cmd.exe
// remark. Like a comment it exits 0 having checked nothing (#3147 A2).
func NoOp(cmd string) bool { return briefdoc.ShellNoOp(cmd) }

// CommentLed returns why the command a Verify Command cell names is only a shell
// comment, or "" when it is not. A marked cell is judged on its marked command,
// and a marked comment is refused too: the marker names the command to run, and
// a comment runs nothing (#2684 review F2). An unmarked cell is judged on its
// first code span, which is both the lift and the string these executors run
// (#3147), so the predicate and the executed string can no longer differ.
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
// (briefdoc.ProseLedMention); and a lift that is blank. An executor records a
// refused row could-not-run and never executes it. The executors and statusgen
// are held to the "cells" section of statusgen/testdata/shell-comment-vectors.json,
// verdict for verdict (#3147).
func Refused(cell string) string {
	if why := CommentLed(cell); why != "" {
		return why
	}
	lift := Lift(cell)
	if NoOp(lift) {
		return NoOpWhy
	}
	if _, ok := Marked(cell); !ok {
		if why := briefdoc.ProseLedMention(cell); why != "" {
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

// firstSpan is the unmarked lift (briefdoc.CodeSpan): the content of the cell's
// first code span, or the trimmed cell when it has no backtick.
func firstSpan(cell string) string { return briefdoc.CodeSpan(cell) }
