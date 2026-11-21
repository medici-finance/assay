package briefdoc

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Verify-row compilation (tool-refactoring/03).
//
// This file is the ONE home of the pure, lexical reading of a brief's `## Verify`
// table: how a row is split into cells, how a row ID, a Class cell, a Shell and
// an Env cell are read, and how the command a Command cell names is lifted
// (explicit `cmd:` marker, first code span, prose-led refusal, shell comment and
// no-op refusal). statusgen's verifyrun, rowclass and lint code and the
// tools/desk executors (internal/verifycmd) call these functions; none keeps a
// local copy of the lexical rules. Everything here is a pure function of the
// text: no file, process, forge or clock is touched, so a row this package
// reads is never executed by it.
//
// Meaning that needs statusgen's own constants (the known execution classes,
// the known obligation words, the per-shell command grammar) stays in
// statusgen: this package returns the RAW class, shell and env cells and the
// obligation words as written.

// SplitRowEscaped splits a markdown table row into cells on UNESCAPED pipes.
//
// The `\|` escapes are PRESERVED VERBATIM in the returned cells. Resolving them
// to `|` here would erase the exact signal rule 1 detects — the lint would be
// structurally unable to see the defect it reports on, which is the failure this
// whole check exists to prevent. The escape decides cell boundaries and nothing
// else; interpreting it is tokenizeCommand's job, which has the quoting context
// needed to tell a shell pipe from pattern content.
//
// It exists because the package-wide splitRow ONCE split on every `|` byte,
// which shredded exactly the rows this lint hunts: a Command cell reading
//
//	`ls x \| grep -cE "a\|b"`
//
// split into five cells and truncated the command at the first `\|` — the lint
// would be blind to the very text it is looking for. splitRow is escape-aware
// now too, so the two agree on where the cell boundaries are; they still differ
// in how they trim the row's outer delimiters (splitRow drops every zero-length
// leading/trailing cell, matching the `strings.Trim(line, "|")` it grew out of,
// while this drops at most one blank cell at each end), so this local variant
// stays rather than being folded into it.
//
// In GitHub Flavored Markdown, `\|` is the ONLY way to put a pipe inside a table
// cell — the escape is processed before inline parsing, so it applies even inside
// a `code span`. That is what makes the raw text ambiguous, and is the whole
// subject of escapedPipeOutsideBracket below.
func SplitRowEscaped(line string) []string {
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
	// A well-formed row starts and ends with a delimiter, yielding empty
	// leading/trailing cells; drop them to match splitRow's contract.
	if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
		cells = cells[1:]
	}
	if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
		cells = cells[:len(cells)-1]
	}
	return cells
}

// CodeSpan lifts the content of the first `backtick` code span in a cell, or the
// trimmed cell text when there is none. Verify commands are authored as code
// spans; the surrounding prose ("(post out-of-repo apply)") is not part of the
// command.
func CodeSpan(cell string) string {
	s := strings.TrimSpace(cell)
	start := strings.Index(s, "`")
	if start < 0 {
		return s
	}
	// Support ``…`` fences (used when the command itself contains a backtick).
	ticks := 0
	for start+ticks < len(s) && s[start+ticks] == '`' {
		ticks++
	}
	fence := strings.Repeat("`", ticks)
	rest := s[start+ticks:]
	end := strings.Index(rest, fence)
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// ---------------------------------------------------------------------------
// The explicit command marker — `cmd: <command>` (issue #1805)
// ---------------------------------------------------------------------------
//
// CodeSpan's rule — "the command is the FIRST code span" — is right for the
// cell shape the corpus mostly uses (a cell that is one code span, or one span
// followed by a parenthetical). It is WRONG for a prose cell that mentions a
// token before the command: "In `PublicRepoGate` (`repovis.go`) change … then
// `cd tools/desk && go test …`" lifts `PublicRepoGate`, and the execution
// witness runs THAT — exit 127, recorded could-not-run, and the check the row
// was written for is never witnessed on any runner.
//
// The fix is an explicit marker the author writes, never a guess the tool
// makes: a code span whose content starts `cmd:` names the command, wherever it
// sits in the cell, and VerifyCommand prefers it over the first span:
//
//	| 3 | In `PublicRepoGate` change the check, then `cmd: cd tools/desk && go test ./internal/deskkit/ -count=1` | exit 1 |
//
// WHY `cmd:` AND NOT `run:`. `run:` is the GitHub Actions step key, and this
// corpus quotes workflow lines inside Verify prose (a planted `run: echo …`
// fixture line, "never inside `run:`"). A `run:` marker would turn every such
// quotation into the command the witness executes — the #1805 defect again, by a
// different door. `cmd:` appears in no code span in the corpus and is no CI
// system's step key.
//
// WHY A MARKER RATHER THAN A SMARTER GUESS. Choosing "the span that looks most
// like a command" would make the executed text a function of a heuristic that
// can change under a row nobody edited — and the witness binds a recorded run to
// the command text, so the command must be a property of what the author wrote.
// The heuristic (ProseLedCommandWhy) never selects WHICH command runs. It
// decides only WHETHER the first span may run: the lint ADVISES the author to add
// the marker, and a row it flags prose-led is not executed — verifyrun records it
// could-not-run, the transcribe-verdict check:ci lane refuses it, and --check
// does not audit an old pass witness on it as pass (#1808 review).
//
// COMPATIBILITY. A cell with no marked span lifts exactly what CodeSpan lifts
// today, byte for byte, so no row already on main changes command. A row the
// lint flags prose-led does change outcome on its next run: whatever its witness
// recorded, the new witness is could-not-run (no flagged row on main records a
// pass). A cell that is one code span needs no marker; the marker is for cells
// that mix prose and spans.
const CommandMarker = "cmd:"

// CodeSpans returns the content of every inline code span in cell, in order,
// each trimmed. It uses CodeSpan's own delimiter rule (a run of N backticks
// closes at the next run of N) so its FIRST element is what CodeSpan lifts
// whenever the cell's first span is terminated. An unterminated span ends the
// scan: its tail is a shredded cell (rule 8), not a span.
func CodeSpans(cell string) []string {
	var out []string
	s := cell
	for {
		start := strings.Index(s, "`")
		if start < 0 {
			return out
		}
		ticks := 0
		for start+ticks < len(s) && s[start+ticks] == '`' {
			ticks++
		}
		fence := strings.Repeat("`", ticks)
		rest := s[start+ticks:]
		end := strings.Index(rest, fence)
		if end < 0 {
			return out
		}
		out = append(out, strings.TrimSpace(rest[:end]))
		s = rest[end+ticks:]
	}
}

// proseOutsideSpans returns the cell's text with every terminated code span
// removed, trimmed — "" for a cell that is nothing but code spans.
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
		ticks := 0
		for start+ticks < len(s) && s[start+ticks] == '`' {
			ticks++
		}
		rest := s[start+ticks:]
		end := strings.Index(rest, strings.Repeat("`", ticks))
		if end < 0 {
			b.WriteString(s[start:])
			break
		}
		s = rest[end+ticks:]
	}
	return strings.TrimSpace(b.String())
}

// RenderedCodeSpans returns every inline code span a Markdown renderer
// DISPLAYS in cell, in order, each trimmed and with how it sits in the cell —
// and plain=false when the cell's prose carries a construct that can hide text
// from the rendered page or re-pair its backticks.
//
// It exists because the marker must never select text the reader of the
// rendered Verify table cannot see (#1808 review, SR-1808-1). CodeSpans reads
// raw backticks, so a `cmd:` span inside an HTML comment, or between two
// backslash-escaped backticks, would count as a marker there although the page
// shows no code at all. This scan follows CommonMark instead:
//
//   - outside a span, a backslash escapes the ASCII punctuation after it (so
//     "\`" is a literal backtick, never a span delimiter);
//   - a run of N backticks is closed only by the next run of EXACTLY N, and an
//     opener with no closer is literal text, not a span.
//
// Rather than model every construct that can carry hidden text (an HTML
// comment, a tag's attribute, a link title, an image's alt text, a footnote), a
// cell whose prose outside code spans carries an unescaped `<` or `[` is not
// plain. Nor is one whose prose carries a `$` written any way — bare,
// backslash-escaped, or as a character reference such as `&#36;`: GitHub
// renders a span wrapped in dollar signs as math, not code, and it applies
// math after escapes and references are resolved. Any character reference in
// prose makes the cell not plain, whatever it names.
//
// A code span also has to stand clear of the prose around it. GitHub's
// extended autolinks run through a backtick, and math, strikethrough and
// references glue onto one, so a span whose opening run is fused to the text
// before it may not render as the span this scan paired — and when one opener
// is swallowed, every later pairing shifts. So a cell is not plain when any
// span's opening run is preceded by anything but whitespace, the start of the
// cell, or a run of `(` that itself follows whitespace or the start. The
// marker span itself is held tighter (MarkedCommands): its opening run must
// follow whitespace or the start of the cell, and its closing run must be at
// the end of the cell or followed by whitespace or plain punctuation.
//
// MarkedCommands honours no marker in a cell that is not plain, and the lint
// says why (the cmd-marker-not-honoured rule). A cell with no marker never reaches this
// rule's consequences — VerifyCommand falls back to CodeSpan exactly as before.
//
// The same rule is implemented for tools/desk's executors in
// tools/desk/internal/verifycmd; both are held to one shared vector table,
// testdata/cmd-marker-vectors.json.
func RenderedCodeSpans(cell string) (spans []RenderedSpan, plain bool) {
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
				i += n // an unclosed opener is literal backticks
				continue
			}
			if !parenLed(cell, i) {
				plain = false // the opener is fused to the prose before it
			}
			spans = append(spans, RenderedSpan{
				Text:     strings.TrimSpace(cell[i+n : closeAt]),
				SpaceLed: i == 0 || isSpaceByte(cell[i-1]),
				CleanEnd: cleanSpanEnd(cell, closeAt+n),
			})
			i = closeAt + n
		default:
			i++
		}
	}
	return spans, plain
}

// RenderedSpan is one code span RenderedCodeSpans found: its trimmed content,
// and how it sits in the cell. SpaceLed: the opening run is at the start of
// the cell or right after whitespace. CleanEnd: the closing run is at the end
// of the cell or right before whitespace or plain punctuation.
type RenderedSpan struct {
	Text     string
	SpaceLed bool
	CleanEnd bool
}

// charRefRe matches a character reference at the start of a string — a named
// one (`&dollar;`), a decimal one (`&#36;`) or a hex one (`&#x24;`).
var charRefRe = regexp.MustCompile(`^&(#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]{1,31});`)

// isSpaceByte reports whether b is a space or a tab.
func isSpaceByte(b byte) bool { return b == ' ' || b == '\t' }

// parenLed reports whether the backtick run at cell[i] follows whitespace or
// the start of the cell, allowing a run of `(` between them: "(`x`)" stands
// clear of the prose, "a/`x`" does not.
func parenLed(cell string, i int) bool {
	j := i
	for j > 0 && cell[j-1] == '(' {
		j--
	}
	return j == 0 || isSpaceByte(cell[j-1])
}

// cleanSpanEnd reports whether a code span's closing run, ending just before
// cell[k], is at the end of the cell or followed by whitespace or plain
// punctuation.
func cleanSpanEnd(cell string, k int) bool {
	return k == len(cell) || isSpaceByte(cell[k]) || strings.IndexByte(".,;:!?)", cell[k]) >= 0
}

// backtickRun is the length of the run of backticks starting at s[i].
func backtickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// isASCIIPunct reports whether b is ASCII punctuation — the set a CommonMark
// backslash escapes.
func isASCIIPunct(b byte) bool {
	return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", b) >= 0
}

// markerContent returns the command a span names when it is a `cmd:` marker,
// or ok=false. A span that is the bare marker with nothing after it is a
// MENTION of the marker (as in this repo's own docs), not a marked command.
// The match is a case-sensitive PREFIX: `CMD:` and `echo cmd: x` are not
// markers.
func markerContent(span string) (string, bool) {
	if !strings.HasPrefix(span, CommandMarker) {
		return "", false
	}
	c := strings.TrimSpace(strings.TrimPrefix(span, CommandMarker))
	return c, c != ""
}

// MarkedCommands returns the command of every `cmd:`-marked span the RENDERED
// cell shows as code (RenderedCodeSpans), marker stripped and trimmed — and
// none at all for a cell that is not plain, whose marker the reader of the
// rendered table might not see. A marker span that does not stand clear of its
// prose — opening run after anything but whitespace or the start of the cell,
// or closing run before anything but whitespace, plain punctuation or the end
// — makes the whole cell honour none: the rendered page may not show it as
// code.
func MarkedCommands(cell string) []string {
	spans, plain := RenderedCodeSpans(cell)
	if !plain {
		return nil
	}
	var out []string
	for _, sp := range spans {
		c, ok := markerContent(sp.Text)
		if !ok {
			continue
		}
		if !sp.SpaceLed {
			return nil
		}
		if !sp.CleanEnd {
			return nil
		}
		out = append(out, c)
	}
	return out
}

// RawMarkerPresent reports whether a raw-backtick scan of cell finds a
// `cmd:`-marked span — including one the rendered cell does not show as code.
// It is the lint's half of the hidden-marker check: raw marker present, but
// MarkedCommands empty, means the author wrote a marker verifyrun will not
// honour.
func RawMarkerPresent(cell string) bool {
	for _, sp := range CodeSpans(cell) {
		if _, ok := markerContent(sp); ok {
			return true
		}
	}
	return false
}

// MarkerOverridesCommandSpan returns the cell's first rendered code span when a
// honoured marker overrides it AND that first span is itself command-shaped
// (multi-word), or "" otherwise. The shape the marker exists for has one-token
// MENTIONS ahead of it; a multi-word first span that the marker silently
// replaces is what a reader of the rendered table would take for the command
// (#1808 review, SR-1808-1).
func MarkerOverridesCommandSpan(cell string) string {
	if len(MarkedCommands(cell)) == 0 {
		return ""
	}
	spans, _ := RenderedCodeSpans(cell)
	if len(spans) == 0 {
		return ""
	}
	first := spans[0].Text
	if _, ok := markerContent(first); ok {
		return ""
	}
	if !strings.ContainsAny(first, " \t") {
		return ""
	}
	return first
}

// VerifyCommand lifts the command a Verify row's Command cell names. It is the
// ONE place a Verify command is lifted out of its cell — every site that runs,
// re-executes, or lints the command goes through it (the class guard,
// TestVerifyCommandLiftHasOneChokePoint, fails on a new site that calls CodeSpan
// on a Verify cell directly). The first `cmd:`-marked span wins; with no marked
// span the result is CodeSpan's, unchanged. A cell carrying more than one marked
// span is ambiguous and the lint says so (the cmd-marker-ambiguous rule); the first
// still runs, so the row's behaviour never depends on the lint having been read.
func VerifyCommand(cell string) string {
	if m := MarkedCommands(cell); len(m) > 0 {
		return m[0]
	}
	return CodeSpan(cell)
}

// ---------------------------------------------------------------------------
// A prose-led Command cell whose first span is not a command (#1805)
// ---------------------------------------------------------------------------

// nonExecExtensions are file extensions a bare command word never carries: a
// source, doc or data file. A first span ending in one of these is a file the
// prose MENTIONS. Script extensions (.sh, .py, .ps1, .exe, …) are deliberately
// absent — a script path can be the command.
var nonExecExtensions = map[string]bool{
	".go": true, ".md": true, ".yaml": true, ".yml": true, ".json": true, ".jsonl": true,
	".toml": true, ".txt": true, ".mod": true, ".sum": true, ".ts": true,
	".tsx": true, ".js": true, ".html": true, ".css": true, ".csv": true, ".lock": true,
	".xml": true, ".proto": true, ".rs": true, ".tf": true, ".svg": true, ".png": true,
	".pdf": true,
}

// scriptExtensions are the extensions that keep a path-shaped span from being
// read as a mere mention: `scripts/check.sh` can be the whole command.
var scriptExtensions = map[string]bool{
	".sh": true, ".bash": true, ".py": true, ".ps1": true, ".exe": true, ".rb": true, ".pl": true,
}

// codeIdentRe is a code identifier — `PublicRepoGate`, `runSupersededLane`,
// `pkg.Func`, `Foo()` — as opposed to a command word. It only counts as one when
// it carries an upper-case letter or a call's `()`: command names are lower case.
var codeIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*(\(\))?$`)

// bareWordRe is a lone lower-case word — `release-runner`, `make`, `grep`.
var bareWordRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// ProseLedCommandWhy reports why a Command cell's lifted command is a prose
// MENTION rather than a command — returning that first span too, for the
// NOTICE text, so the lint never re-lifts it with CodeSpan — or two empty
// strings when the cell is not flagged.
//
// It is deliberately CONSERVATIVE — a NOTICE that fires on a real command is
// noise an author learns to skip, so every arm needs positive evidence the span
// is not a command:
//
//   - the cell carries a `cmd:` marker → never flagged (the author said which);
//   - the cell is ONE code span and nothing else → never flagged (the span is,
//     unambiguously, what the author wrote as the command);
//   - the first span has whitespace in it → never flagged (a multi-word span is
//     read as a command line; judging it is the other rules' job).
//
// ONE arm runs ahead of those exemptions, the marker's included, because it needs
// no heuristic: a lifted command whose first non-blank character is `#` is a shell
// comment (#2684 — an inline `## Coverage` heading in a procedure). It is flagged
// whatever surrounds it, including as the cell's only span and as the command a
// `cmd:` marker names: a marker names the command to run, and a comment runs
// nothing, so a marked comment is refused exactly as the tools/desk executors
// (verifyloop, deskrebaseline) refuse it (#2684 review F2). A lift that is a
// shell no-op (`:`, or a cmd.exe `REM` / `::` remark) is refused the same way and
// at the same point, marked or not (#3147 A2; see ShellNoOp).
//
// Otherwise a single-token first span is flagged when it is: a dotted extension
// or dotfile (`.exe`); a file with a source/doc/data extension (`repovis.go`); a
// relative path or `owner/repo` / `stream/NN` reference (`windows-port/00`); a
// code identifier (`PublicRepoGate`, `Foo()`); or a lone lower-case word that a
// LATER span in the same cell follows with a multi-word command (`release-runner`
// … `cd tools/desk && go test …`) — that last arm needs the later span, so a
// lone `make` in prose is never flagged on its own. A bare `cmd:` span ahead of
// the command is flagged too: it is a mention of the marker, not a marker, so
// verifyrun would run the text `cmd:` itself.
func ProseLedCommandWhy(cell string) (span, why string) {
	// A command that is nothing but a shell comment is never a command, however
	// many spans or words surround it (#2684): the shell ignores it, exits 0 with
	// no output, and a witness would record a pass for a check that never ran. It
	// is judged on what the lift returns — the marked command, else the first
	// span, else the whole cell — and BEFORE the marker, one-span and whitespace
	// exemptions below, because each of those assumes a command line.
	if m := MarkedCommands(cell); len(m) > 0 {
		if ShellCommentOnly(m[0]) {
			return m[0], ShellCommentMarkedWhy
		}
		if ShellNoOp(m[0]) {
			return m[0], ShellNoOpMarkedWhy
		}
		return "", "" // a marked real command: the author named it
	}
	lifted := VerifyCommand(cell)
	if ShellCommentOnly(lifted) {
		return lifted, ShellCommentWhy
	}
	// A lift that is a shell no-op runs nothing for the same reason (#3147 A2):
	// `:` exits 0 whatever its arguments, and a cmd.exe `REM` / `::` remark is
	// that shell's comment. Judged like the comment arm, ahead of the exemptions.
	if ShellNoOp(lifted) {
		return lifted, ShellNoOpWhy
	}
	// The shape test has one home, ProseLedMention; a flagged cell always has a
	// first span, which is the span this reports.
	if w := ProseLedMention(cell); w != "" {
		return CodeSpans(cell)[0], w
	}
	return "", ""
}

// ShellCommentWhy is the reason text ProseLedCommandWhy gives a comment-led
// span; the NOTICE and the could-not-run note read "...is <ShellCommentWhy>, not a command".
// ShellCommentMarkedWhy is the same reason for a `cmd:`-marked command.
const (
	ShellCommentDetail    = "(a command starting `#` is ignored by the shell: it exits 0 with no output and measures nothing)"
	ShellCommentWhy       = "a shell comment " + ShellCommentDetail
	ShellCommentMarkedWhy = "a `cmd:`-marked shell comment " + ShellCommentDetail
)

// ShellNoOpWhy is the reason ProseLedCommandWhy gives a lift that is a shell
// no-op (#3147 A2); ShellNoOpMarkedWhy is the same reason for a `cmd:`-marked one.
const (
	ShellNoOpDetail    = "(`:` exits 0 whatever its arguments, and a cmd.exe `REM` / `::` remark is that shell's comment: it runs nothing and measures nothing)"
	ShellNoOpWhy       = "a shell no-op " + ShellNoOpDetail
	ShellNoOpMarkedWhy = "a `cmd:`-marked shell no-op " + ShellNoOpDetail
)

// ShellNoOp reports whether a lifted command is a shell no-op: a cmd.exe `::`
// remark, a `REM` / `@rem` remark (any case), or `:` as the first word with no
// `;`, `&`, `|` or newline after it (`: ; test -d .` still runs its test). Bare
// `true` and `exit 0` are deliberately left to the cmd-marker-vacuous notice: a
// fixture command of `true` is a common, intended pass. Kept identical to
// tools/desk's verifycmd.NoOp; the shared cells vectors hold both to it.
func ShellNoOp(cmd string) bool {
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

// ProseLedRemedy is the fix a prose-led NOTICE, could-not-run note or check:ci
// refusal names for its reason: a comment-led cell needs the real command marked
// (or stays a manual procedure); a marked comment needs the comment REPLACED, since
// the row is already marked; any other mention needs the marker.
func ProseLedRemedy(why string) string {
	switch {
	case strings.HasSuffix(why, ShellNoOpDetail):
		return "Replace the no-op with the command that performs the check"
	case strings.HasSuffix(why, ShellCommentMarkedWhy):
		return "Replace the cmd:-marked comment with the command that performs the check"
	case strings.HasSuffix(why, ShellCommentWhy):
		return "Mark the real command with a cmd: code span where one exists; otherwise keep the row a manual procedure with its evidence, never a command that merely exits 0"
	}
	return "Mark the command with a cmd: code span"
}

// ShellCommentOnly reports whether a lifted command is a shell comment: its first
// non-blank character is `#`. A `#` later in the command is untouched — a trailing
// ` # comment`, a quoted `'#'` or `a#b` all leave a real command.
func ShellCommentOnly(cmd string) bool {
	return strings.HasPrefix(strings.TrimSpace(cmd), "#")
}

// proseLedShape is ProseLedCommandWhy's shape test on a single-token first span.
func proseLedShape(first string, later []string) string {
	ext := strings.ToLower(filepath.Ext(first))
	switch {
	case first == CommandMarker:
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

// separatorRow matches a markdown table separator row once its outer pipes are
// trimmed (`|---|:--:|`).
var separatorRow = regexp.MustCompile(`^[\s:|-]+$`)

// NormalizeRowID trims a row's "#" cell to a bare identifier ("`3`" -> "3").
func NormalizeRowID(cell string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(cell), "`*_"))
}

// ObligationSeparator is the prefix that marks an obligation word in a Class
// cell (`check:ci +mutation`).
const ObligationSeparator = "+"

// SplitRowClassCell splits a Verify row's Class cell into its execution class
// and its obligation words: the first whitespace-separated word is the
// execution class unless it begins with the obligation separator, and every
// later word is an obligation (separator stripped, lower-cased). A blank cell
// yields ("", nil). Whether a word is a KNOWN class or obligation is the
// caller's judgment; this function only reads the cell.
func SplitRowClassCell(raw string) (execCell string, obligations []string) {
	s := strings.ToLower(strings.TrimSpace(NormalizeRowID(raw)))
	if s == "" {
		return "", nil
	}
	fields := strings.Fields(s)
	i := 0
	if !strings.HasPrefix(fields[0], ObligationSeparator) {
		execCell = fields[0]
		i = 1
	}
	for _, f := range fields[i:] {
		obligations = append(obligations, strings.TrimPrefix(f, ObligationSeparator))
	}
	return execCell, obligations
}

// RowCells is one Verify-table data row as located by VerifyTable: every cell
// is the RAW text, markdown intact. Classed and EnvCol record whether the TABLE
// carried a Class or Env column at all (an explicit column and a legacy-default
// table mean different things to the callers).
type RowCells struct {
	Num     string
	Class   string
	Command string
	Expect  string
	Classed bool
	Shell   string
	Env     string
	EnvCol  bool
}

// VerifyTable walks the table(s) in a Verify section and calls fn for each data
// row. Columns are located by header NAME (`#`, `Class`, `Shell`, `Env`,
// `Command`, `Expect`), never by position; a table without a `Command` column
// yields no rows; a row shorter than the Command column is skipped; leaving
// the table (a non-pipe line) resets the header.
func VerifyTable(section string, fn func(RowCells)) {
	cmdIdx, expIdx, numIdx, classIdx, shellIdx, envIdx := -1, -1, -1, -1, -1, -1
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			cmdIdx, expIdx, numIdx, classIdx, shellIdx, envIdx = -1, -1, -1, -1, -1, -1 // left the table
			continue
		}
		if separatorRow.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := SplitRowEscaped(line)
		if cmdIdx < 0 {
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				case "#":
					numIdx = j
				case "class":
					classIdx = j
				case "shell":
					shellIdx = j
				case "env":
					envIdx = j
				}
			}
			continue // the header row is not a data row
		}
		if cmdIdx >= len(cells) {
			continue
		}
		cell := func(idx int) string {
			if idx >= 0 && idx < len(cells) {
				return strings.TrimSpace(cells[idx])
			}
			return ""
		}
		fn(RowCells{
			Num: cell(numIdx), Class: cell(classIdx), Command: cells[cmdIdx], Expect: cell(expIdx),
			Classed: classIdx >= 0, Shell: cell(shellIdx), Env: cell(envIdx), EnvCol: envIdx >= 0,
		})
	}
}

// VerificationRow is one compiled Verify row: its identity, the command its
// Command cell names, and the lexical verdict on whether that command may run.
type VerificationRow struct {
	// ID is the row's `#` cell, normalised, or its 1-based ordinal among the
	// kept rows when the table has no `#` cell for it; Numbered records which.
	ID       string
	Numbered bool
	// Command is the lifted command (VerifyCommand); CommandCell the trimmed raw
	// Command cell; Expect the raw Expect cell.
	Command     string
	CommandCell string
	Expect      string
	// ClassCell, ShellCell and EnvCell are the raw, trimmed cells. Classed and
	// EnvCol record whether the table carried those columns.
	ClassCell string
	ShellCell string
	EnvCell   string
	Classed   bool
	EnvCol    bool
	// ExecClass and Obligations are the Class cell read by SplitRowClassCell:
	// raw words, unvalidated. An unclassed table leaves both empty.
	ExecClass   string
	Obligations []string
	// ProseLed, when non-empty, says why the lifted command is a prose MENTION,
	// a shell comment or a shell no-op, not a command: the row must not run.
	ProseLed string
}

// CompileVerification compiles a Verify section into its rows. A row with
// neither a lifted command nor an Expect is dropped. It is the ONE reading of
// row identity, class cell, command lift and prose-led refusal: the statusgen
// verify code and the tools/desk executors consume this record and keep no
// grammar of their own.
func CompileVerification(section string) []VerificationRow {
	var rows []VerificationRow
	ordinal := 0
	VerifyTable(section, func(r RowCells) {
		cmd := VerifyCommand(r.Command)
		if strings.TrimSpace(cmd) == "" && strings.TrimSpace(r.Expect) == "" {
			return
		}
		ordinal++
		id := NormalizeRowID(r.Num)
		numbered := id != ""
		if !numbered {
			id = strconv.Itoa(ordinal)
		}
		row := VerificationRow{
			ID: id, Numbered: numbered,
			Command: cmd, CommandCell: strings.TrimSpace(r.Command), Expect: r.Expect,
			ClassCell: strings.TrimSpace(r.Class), ShellCell: r.Shell, EnvCell: r.Env,
			Classed: r.Classed, EnvCol: r.EnvCol,
		}
		if r.Classed {
			row.ExecClass, row.Obligations = SplitRowClassCell(r.Class)
		}
		if first, why := ProseLedCommandWhy(r.Command); why != "" {
			what := "first span "
			if why == ShellCommentMarkedWhy || why == ShellNoOpMarkedWhy {
				what = "marked command "
			}
			row.ProseLed = what + strings.ReplaceAll(first, "|", "\\|") + " is " + why
		}
		rows = append(rows, row)
	})
	return rows
}

// ProseLedMention is ProseLedCommandWhy's shape test on an UNMARKED cell whose
// lift is neither a shell comment nor a no-op: why the first code span is a
// prose mention rather than a command, or "". A cell that is one span and
// nothing else, and a first span with whitespace in it, are never flagged.
func ProseLedMention(cell string) string {
	spans := CodeSpans(cell)
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
