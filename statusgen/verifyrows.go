package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Unfailable-Verify-row lint.
//
// A Verify row exists to answer "does it actually work?". A row whose command is
// STRUCTURALLY INCAPABLE OF FAILING is worse than no row at all: it manufactures
// evidence. The table shows green, the Evidence cell gets a real command and a
// real exit code, and a reader has every reason to believe something was checked.
// Nothing was.
//
// Scope discipline. verifySectionProblems is deliberately a
// presence/structure check — "is the row any GOOD?" is the review gate's job, not
// this lint's, and that boundary stands. These checks do NOT judge quality: they
// flag rows that cannot discriminate between pass and fail for ANY input. That is
// not a weak test, it is a non-test — a structural property of the command text,
// decidable without judgement. "This assertion is shallow" stays with the
// reviewer; "this assertion is inert" belongs here.
//
// Severity: NOTICE, not PROBLEM — see unfailableRowNotices.

// ---------------------------------------------------------------------------
// Markdown table cells
// ---------------------------------------------------------------------------

// splitRowEscaped splits a markdown table row into cells on UNESCAPED pipes.
//
// The `\|` escapes are PRESERVED VERBATIM in the returned cells. Resolving them
// to `|` here would erase the exact signal rule 1 detects — the lint would be
// structurally unable to see the defect it reports on, which is the failure this
// whole check exists to prevent. The escape decides cell boundaries and nothing
// else; interpreting it is tokenizeCommand's job, which has the quoting context
// needed to tell a shell pipe from pattern content.
//
// It exists because the package-wide splitRow splits on every `|` byte, which
// shreds exactly the rows this lint hunts: a Command cell reading
//
//	`ls x \| grep -cE "a\|b"`
//
// splits into five cells and truncates the command at the first `\|` — the lint
// would be blind to the very text it is looking for. splitRow is left alone
// (every other check depends on its current behaviour); this is a local,
// escape-aware variant.
//
// In GitHub Flavored Markdown, `\|` is the ONLY way to put a pipe inside a table
// cell — the escape is processed before inline parsing, so it applies even inside
// a `code span`. That is what makes the raw text ambiguous, and is the whole
// subject of escapedPipeOutsideBracket below.
func splitRowEscaped(line string) []string {
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

// codeSpan lifts the content of the first `backtick` code span in a cell, or the
// trimmed cell text when there is none. Verify commands are authored as code
// spans; the surrounding prose ("(post out-of-repo apply)") is not part of the
// command.
func codeSpan(cell string) string {
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
// Shell-ish tokenizer
// ---------------------------------------------------------------------------

// shellTok is one token of a Verify command cell. quoted records whether the
// token came from inside quotes (a quoted `\|` is pattern content; an unquoted
// one is a pipeline operator — see tokenizeCommand).
type shellTok struct {
	text   string
	quoted bool
	op     bool
}

// tokenizeCommand splits a Verify command into shell-ish tokens.
//
// The one markdown-aware decision: an UNQUOTED `\|` is a pipeline operator, not
// an escaped literal pipe. Inside a GFM table cell that is how a shell pipe must
// be authored, and reading it as an escape would merge a whole pipeline into one
// token — every `… \| grep …` row would be missed. A QUOTED `\|` is preserved
// verbatim as token text: that is the ambiguous regex case this lint is for.
//
// This is not a POSIX shell parser and does not try to be. It resolves quoting,
// pipelines and command separators — enough to find a grep invocation and its
// pattern argument, which is all the rules below need.
func tokenizeCommand(s string) []shellTok {
	var out []shellTok
	var cur strings.Builder
	quoted, started := false, false

	flush := func() {
		if started {
			out = append(out, shellTok{text: cur.String(), quoted: quoted})
			cur.Reset()
			quoted, started = false, false
		}
	}
	emitOp := func(op string) {
		flush()
		out = append(out, shellTok{text: op, op: true})
	}

	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			flush()
			i++

		// `\|` / `\|\|` unquoted → pipeline operator (markdown-escaped shell pipe).
		case c == '\\' && i+1 < len(s) && s[i+1] == '|':
			if i+3 < len(s) && s[i+2] == '\\' && s[i+3] == '|' {
				emitOp("||")
				i += 4
			} else if i+2 < len(s) && s[i+2] == '|' {
				emitOp("||")
				i += 3
			} else {
				emitOp("|")
				i += 2
			}

		case c == '|':
			if i+1 < len(s) && s[i+1] == '|' {
				emitOp("||")
				i += 2
			} else {
				emitOp("|")
				i++
			}
		case c == ';':
			emitOp(";")
			i++
		case c == '&' && i+1 < len(s) && s[i+1] == '&':
			emitOp("&&")
			i += 2

		case c == '\'':
			started, quoted = true, true
			j := i + 1
			for j < len(s) && s[j] != '\'' {
				j++
			}
			cur.WriteString(s[i+1 : min(j, len(s))])
			i = j + 1

		case c == '"':
			started, quoted = true, true
			j := i + 1
			for j < len(s) && s[j] != '"' {
				if s[j] == '\\' && j+1 < len(s) {
					j += 2 // keep `\"` from ending the string early
					continue
				}
				j++
			}
			cur.WriteString(s[i+1 : min(j, len(s))])
			i = j + 1

		default:
			started = true
			cur.WriteByte(c)
			i++
		}
	}
	flush()
	return out
}

// ---------------------------------------------------------------------------
// grep invocations
// ---------------------------------------------------------------------------

// grepCall is one grep invocation lifted from a command cell.
type grepCall struct {
	extended bool     // -E / --extended-regexp / egrep
	count    bool     // -c / --count
	patterns []string // pattern arguments, in order
	last     bool     // true when this grep is the last stage of its pipeline
}

// grepFlagWithArg are grep short flags that consume the following token as their
// argument, so it is not mistaken for the pattern.
var grepFlagWithArg = map[byte]bool{'e': true, 'f': true, 'm': true, 'A': true, 'B': true, 'C': true, 'd': true, 'D': true}

// grepCalls extracts every grep/egrep invocation from a tokenized command.
func grepCalls(toks []shellTok) []grepCall {
	var out []grepCall
	// Split into simple commands on operators, remembering whether each was the
	// final stage of its pipeline (only the last stage's exit status survives).
	var cmd []shellTok
	var cmds [][]shellTok
	var lastFlags []bool
	flush := func(isLast bool) {
		if len(cmd) > 0 {
			cmds = append(cmds, cmd)
			lastFlags = append(lastFlags, isLast)
			cmd = nil
		}
	}
	for _, t := range toks {
		if t.op {
			// A `|` means a further stage follows → this command is not last.
			flush(t.text != "|")
			continue
		}
		cmd = append(cmd, t)
	}
	flush(true)

	for ci, c := range cmds {
		// Skip leading shell noise (`{`, `!`, `(`) before the command word.
		k := 0
		for k < len(c) && (c[k].text == "{" || c[k].text == "!" || c[k].text == "(") {
			k++
		}
		if k >= len(c) {
			continue
		}
		name := c[k].text
		if name != "grep" && name != "egrep" && name != "ggrep" && name != "rg" {
			continue
		}
		if name == "rg" {
			continue // ripgrep: different flag surface, not in scope
		}
		g := grepCall{extended: name == "egrep", last: lastFlags[ci]}
		var operands []string
		for j := k + 1; j < len(c); j++ {
			t := c[j]
			switch {
			case !t.quoted && strings.HasPrefix(t.text, "--"):
				flag, val, hasVal := strings.Cut(t.text[2:], "=")
				switch flag {
				case "extended-regexp":
					g.extended = true
				case "count":
					g.count = true
				case "regexp":
					if hasVal {
						g.patterns = append(g.patterns, val)
					} else if j+1 < len(c) {
						j++
						g.patterns = append(g.patterns, c[j].text)
					}
				case "file":
					if !hasVal && j+1 < len(c) {
						j++
					}
				}
			case !t.quoted && strings.HasPrefix(t.text, "-") && len(t.text) > 1:
				body := t.text[1:]
				for bi := 0; bi < len(body); bi++ {
					ch := body[bi]
					switch ch {
					case 'E':
						g.extended = true
					case 'c':
						g.count = true
					}
					if grepFlagWithArg[ch] {
						// The flag's argument is the rest of this token, or the
						// next token when the rest is empty (`-e PAT` / `-ePAT`).
						arg := body[bi+1:]
						if arg == "" && j+1 < len(c) {
							j++
							arg = c[j].text
						}
						if ch == 'e' {
							g.patterns = append(g.patterns, arg)
						}
						bi = len(body) // flag argument consumed the token
					}
				}
			default:
				operands = append(operands, t.text)
			}
		}
		// With no -e/--regexp, the first operand is the pattern; the rest are files.
		if len(g.patterns) == 0 && len(operands) > 0 {
			g.patterns = append(g.patterns, operands[0])
		}
		out = append(out, g)
	}
	return out
}

// ---------------------------------------------------------------------------
// go test -run invocations
// ---------------------------------------------------------------------------

// goTestRunPatterns extracts every `-run <pattern>` (or `-run=<pattern>` /
// `--run=<pattern>`) argument from `go test` invocations in a tokenized
// command.
//
// Unlike grepCalls this does not track pipeline position: a `-run` regexp
// that matches no test name makes `go test` print "no tests to run" and exit
// 0 wherever it sits — there is no "not the last stage, so it doesn't matter"
// case to exclude.
func goTestRunPatterns(toks []shellTok) []string {
	var patterns []string
	var cmd []shellTok
	var cmds [][]shellTok
	flush := func() {
		if len(cmd) > 0 {
			cmds = append(cmds, cmd)
			cmd = nil
		}
	}
	for _, t := range toks {
		if t.op {
			flush()
			continue
		}
		cmd = append(cmd, t)
	}
	flush()

	for _, c := range cmds {
		k := 0
		for k < len(c) && (c[k].text == "{" || c[k].text == "!" || c[k].text == "(") {
			k++
		}
		if k >= len(c) || c[k].text != "go" {
			continue
		}
		// Find the `test` subcommand token. Verify rows write `go test
		// [./path...] [flags...]`; scanning forward for a literal "test" is
		// enough to cover every shape this lint sees without parsing `go`'s
		// full flag surface.
		testAt := -1
		for j := k + 1; j < len(c); j++ {
			if c[j].text == "test" {
				testAt = j
				break
			}
		}
		if testAt < 0 {
			continue
		}
		for j := testAt + 1; j < len(c); j++ {
			t := c[j]
			switch {
			case t.text == "-run" || t.text == "--run":
				if j+1 < len(c) {
					j++
					patterns = append(patterns, c[j].text)
				}
			case strings.HasPrefix(t.text, "-run="):
				patterns = append(patterns, strings.TrimPrefix(t.text, "-run="))
			case strings.HasPrefix(t.text, "--run="):
				patterns = append(patterns, strings.TrimPrefix(t.text, "--run="))
			}
		}
	}
	return patterns
}

// ---------------------------------------------------------------------------
// Rule 1 — `\|` inside a grep -E pattern
// ---------------------------------------------------------------------------

// escapedPipeOutsideBracket reports whether an ERE contains a `\|` that is NOT
// inside a bracket expression.
//
// Inside `[...]` a pipe is already literal, so `\|` there is not a mis-escaped
// alternation — and `[\|]` is the only way to write a literal-pipe class inside a
// GFM table cell. That is the escape hatch, so it must not be flagged.
func escapedPipeOutsideBracket(pat string) bool {
	inBracket := false
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch {
		case !inBracket && c == '[':
			inBracket = true
			// POSIX: `]` immediately after `[` or `[^` is a literal, not the close.
			j := i + 1
			if j < len(pat) && pat[j] == '^' {
				j++
			}
			if j < len(pat) && pat[j] == ']' {
				i = j
			}
		case inBracket && c == ']':
			inBracket = false
		case !inBracket && c == '\\' && i+1 < len(pat) && pat[i+1] == '|':
			return true
		case c == '\\' && i+1 < len(pat):
			i++ // skip the escaped character
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule 2 — `grep -c` gated on an expected count of zero
// ---------------------------------------------------------------------------

// expectsZeroCount reports whether an Expect cell asserts a count of exactly
// zero. Deliberately narrow: the cell's first token, stripped of markdown, must
// be exactly "0" ("0", "0 (F-12)", "`0` — forbidden numbers absent"). Anything
// relational ("≥ 3", "0 or more") is not a zero-count assertion.
func expectsZeroCount(expect string) bool {
	s := strings.TrimSpace(expect)
	s = strings.NewReplacer("`", "", "*", "", "**", "").Replace(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	first := strings.Fields(s)[0]
	first = strings.TrimRight(first, ".,;:")
	return first == "0"
}

// forcesSuccess reports whether a command neutralises its own exit status with a
// trailing `|| true` / `|| echo …` — the sanctioned fix for the grep -c
// contradiction (the count is then read from stdout, not the exit code).
func forcesSuccess(toks []shellTok) bool {
	for i, t := range toks {
		if t.op && t.text == "||" && i+1 < len(toks) {
			switch toks[i+1].text {
			case "true", "echo", ":":
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule 3 — pipeline exit status swallowed by an always-zero sink
// ---------------------------------------------------------------------------

// exitSwallowingSinks are commands that virtually always exit 0, so a pipeline
// ending in one reports THEIR status, not the real check's. `make test | tee log`
// passes however hard the tests fail — a defect this pipeline shape can hide in a
// CI gate. Kept to `tee` alone: it is the real-world instance and it has no
// meaningful failure mode short of an unwritable file. Filters whose exit status
// can legitimately be the gate (grep, jq, test) are NOT listed.
var exitSwallowingSinks = map[string]bool{"tee": true}

// pipelineSwallowsExit reports the sink name when a command is a multi-stage
// pipeline whose LAST stage is an always-zero sink, and the row is gated on the
// exit status. Returns "" when there is nothing to flag.
func pipelineSwallowsExit(toks []shellTok) string {
	var stages [][]shellTok
	var cur []shellTok
	piped := false
	for _, t := range toks {
		if t.op && t.text == "|" {
			piped = true
			stages = append(stages, cur)
			cur = nil
			continue
		}
		if t.op { // && || ; — a new command, not a pipeline stage
			stages = nil
			cur = nil
			piped = false
			continue
		}
		cur = append(cur, t)
	}
	stages = append(stages, cur)
	if !piped || len(stages) < 2 || len(stages[len(stages)-1]) == 0 {
		return ""
	}
	name := stages[len(stages)-1][0].text
	if exitSwallowingSinks[name] {
		return name
	}
	return ""
}

// ---------------------------------------------------------------------------
// Rule 4 — `\|` inside a `go test -run` pattern
// ---------------------------------------------------------------------------
//
// Identical defect to rule 1, different command surface. `go test
// -run` compiles its argument as an RE2 regexp exactly like `grep -E` — `\|`
// there is a LITERAL pipe too, not alternation, so a row like
// `go test -run 'Dora\|Weekly\|Artifact'` matches no test name, `go test`
// prints "no tests to run", and exits 0: a vacuous PASS on the row meant to
// prove the requirement. escapedPipeOutsideBracket is reused unchanged from
// rule 1 — both commands consume the same RE2 syntax, brackets included.
//
// This is the more dangerous instance of the two: `go test -run` is the most
// common Verify command for Go tooling briefs, and — unlike a `grep -E` typo,
// which usually still matches SOMETHING — a mismatched `-run` pattern matches
// nothing at all, so the "no tests to run" output looks like an empty-but-fine
// result rather than an obvious failure.

// ---------------------------------------------------------------------------
// Rule 5 — unsubstituted metavariable (the row cannot run as written)
// ---------------------------------------------------------------------------
//
// Rules 1-4 catch rows that always PASS. This one catches the mirror defect:
// a row that can never RUN. Both are non-tests — a command that errors before
// it reaches the thing under test discriminates no better than one that
// matches nothing — and both surfaced from the same class of unproven claim:
// smoke rows that passed a bare `url` placeholder the implementation rejected
// before the testing seam.
//
// An unrunnable row is not merely noise. It corrodes the record twice: it
// reaches `implemented` with a Verify table nobody could have executed, and
// when a verifier eventually DOES run it they must silently substitute a
// value — so the command in the brief is not the command that produced the
// Evidence, and the row stops being reproducible.
//
// SCOPE, stated plainly. Two metavariable SHAPES are decidable from the
// command text and are flagged here:
//
//   - an angle-bracket metavariable — `<N>`, `<PR#>`, `<mm/22 workflow file>`;
//   - an ellipsis elision — `... merged-fixture ...`, `…`.
//
// A third shape is NOT decidable and is deliberately not attempted: a
// metavariable spelled as an ordinary bare word (`deskpushguard origin url`).
// Nothing in the command text distinguishes a stand-in named `url` from a
// literal argument that happens to read like one — deciding it requires
// knowing the callee's argument contract. That is the exact shape of the
// bare-word placeholder case, so this rule does NOT subsume the finding that
// motivated it; it narrows the gap and leaves a named remainder. The lint
// output is a LOWER BOUND on this class, never the complete set.
//
// Quoted text is exempt throughout. `grep -Eiq -e "<script[^>]*src="` is a
// regex, not a placeholder, and the quoting is what tells them apart.

// metavarRe matches an angle-bracket metavariable. The negated class keeps it
// off shell syntax that legitimately uses the brackets: `<<EOF` heredocs and
// `>>` appends (a second `<`/`>`), `<(…)` process substitution (a paren), and
// `<$VAR>` expansions (a dollar). The 60-char ceiling keeps a stray `<` and a
// distant `>` from pairing up across a whole command.
var metavarRe = regexp.MustCompile(`<[^<>()$\n]{1,60}>`)

// stripQuotedRegions blanks out single- and double-quoted spans, preserving
// length and every other byte, so a scan over the result sees only what the
// shell would treat as unquoted. Regex patterns, format strings and grep
// arguments live inside quotes; placeholders a verifier must substitute do
// not.
func stripQuotedRegions(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		q := b[i]
		if q != '\'' && q != '"' {
			continue
		}
		j := i + 1
		for j < len(b) && b[j] != q {
			if q == '"' && b[j] == '\\' && j+1 < len(b) {
				j += 2
				continue
			}
			j++
		}
		for k := i; k < min(j+1, len(b)); k++ {
			b[k] = ' '
		}
		i = j
	}
	return string(b)
}

// unsubstitutedMetavars reports the metavariable placeholders in a command
// that a verifier would have to substitute by hand before the row could run.
func unsubstitutedMetavars(cmd string) []string {
	bare := stripQuotedRegions(cmd)
	var out []string
	seen := map[string]bool{}
	addOnce := func(m string) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for _, loc := range metavarRe.FindAllStringIndex(bare, -1) {
		lo, hi := loc[0], loc[1]
		// `cat <<EOF > f.txt` — the match opens on the second `<` of a heredoc.
		if lo > 0 && bare[lo-1] == '<' {
			continue
		}
		// `wc -l < in.txt > out.txt` — two redirections, not one placeholder. A
		// real metavariable closes tight against its last word (`<that-date>`);
		// whitespace before the `>` means the `>` belongs to a separate operator.
		if prev := bare[hi-2]; prev == ' ' || prev == '\t' {
			continue
		}
		addOnce(bare[lo:hi])
	}
	// Ellipsis elision. `/...` is Go's package wildcard (`go test ./x/...`) and
	// `[…]`/`{…}` inside a regex are handled by the quote strip above; an
	// ellipsis anywhere else is an author eliding the real arguments.
	if strings.Contains(bare, "…") {
		addOnce("…")
	}
	for i := 0; i+2 < len(bare)+1; i++ {
		if i+3 > len(bare) || bare[i:i+3] != "..." {
			continue
		}
		if i > 0 && bare[i-1] == '/' {
			continue // ./... — Go package wildcard
		}
		addOnce("...")
		break
	}
	return out
}

// ---------------------------------------------------------------------------
// The lint
// ---------------------------------------------------------------------------

// verifyRowTable locates the Command (and, when present, Expect) columns of a
// Verify section's table and yields each data row's cells.
func verifyRowTable(section string, fn func(num, cmd, expect string)) {
	cmdIdx, expIdx, numIdx := -1, -1, -1
	for _, raw := range strings.Split(section, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "|") {
			cmdIdx, expIdx, numIdx = -1, -1, -1 // left the table
			continue
		}
		if separatorRowRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		cells := splitRowEscaped(line)
		if cmdIdx < 0 {
			for j, c := range cells {
				switch strings.ToLower(strings.TrimSpace(c)) {
				case "command":
					cmdIdx = j
				case "expect":
					expIdx = j
				case "#":
					numIdx = j
				}
			}
			continue // the header row is not a data row
		}
		if cmdIdx >= len(cells) {
			continue
		}
		num := ""
		if numIdx >= 0 && numIdx < len(cells) {
			num = strings.TrimSpace(cells[numIdx])
		}
		expect := ""
		if expIdx >= 0 && expIdx < len(cells) {
			expect = strings.TrimSpace(cells[expIdx])
		}
		fn(num, cells[cmdIdx], expect)
	}
}

// unfailableRowNotices flags Verify rows whose command cannot fail.
//
// SEVERITY — NOTICE, not PROBLEM, deliberately. The two rules below fire on a
// substantial number of briefs already on main, most of them verified/done with
// recorded Evidence. A hard PROBLEM would red-CI main on merge, and the only way
// to green it would be to rewrite the Verify tables of briefs that are already
// closed — retroactively editing the recorded basis of a past sign-off, which is
// precisely the falsification this lint exists to prevent. So: NOTICE now, visible
// on every run, backfill the ACTIVE streams, then flip to a hard add(...) once
// main is clean — the same phased path gate-why took and
// the one the `why:` NOTICE is on today.
//
// Correcting a CLOSED brief's row is not in scope for the backfill: the row is a
// historical record of what was actually run. Note the defect in Evidence instead.
func unfailableRowNotices(streams []*Stream) []string {
	var notices []string
	add := func(format string, a ...any) { notices = append(notices, fmt.Sprintf(format, a...)) }

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported elsewhere; legacy/opted-out exempt
			}
			verifyRowTable(bf.Verify, func(num, cmdCell, expect string) {
				cmd := codeSpan(cmdCell)
				if cmd == "" {
					return
				}
				toks := tokenizeCommand(cmd)
				where := "a Verify row"
				if num != "" {
					where = "Verify row " + num
				}

				for _, g := range grepCalls(toks) {
					// Rule 1: `\|` in an ERE pattern.
					if g.extended {
						for _, p := range g.patterns {
							if escapedPipeOutsideBracket(p) {
								add("%s: %s uses `\\|` inside a `grep -E` pattern (%q) — in an extended regex `\\|` is a LITERAL pipe, not alternation, so the row matches almost nothing and passes whatever the file contains. The raw text is also ambiguous: GFM renders `\\|` in a table cell as `|`, so this command means one thing copied from the rendered page and another copied from the source. Write the alternatives as separate patterns (`grep -E -e alpha -e beta`), which reads identically in both. For a genuine literal pipe use a bracket class (`[\\|]`)", path, where, p)
								break
							}
						}
					}
					// Rule 2: `grep -c` gated on an expected count of zero.
					if g.count && g.last && expectsZeroCount(expect) && !forcesSuccess(toks) {
						add("%s: %s expects a count of `0` from `grep -c`, but grep exits 1 when it matches nothing — on the success path the row FAILS, and it only passes when it finds what it was meant to prove absent. Gate on the exit status instead (`! grep -qE …`), or keep the count as output and neutralise the status (`grep -cE … || true`)", path, where)
					}
				}

				// Rule 3: exit status swallowed by an always-zero sink.
				if sink := pipelineSwallowsExit(toks); sink != "" {
					add("%s: %s pipes into `%s`, so the row reports `%s`'s exit status, not the check's — the command before it can fail and the row still passes. Assert on the real command (write to a file, then check it), or set `set -o pipefail`", path, where, sink, sink)
				}

				// Rule 4: `\|` in a `go test -run` pattern.
				for _, p := range goTestRunPatterns(toks) {
					if escapedPipeOutsideBracket(p) {
						add("%s: %s uses `\\|` inside a `go test -run` pattern (%q) — `go test -run` compiles its argument as RE2, same as `grep -E`: `\\|` there is a LITERAL pipe, not alternation, so the pattern matches no test name, `go test` reports \"no tests to run\", and the row passes having run nothing. The raw text is also ambiguous: GFM renders `\\|` in a table cell as `|`, so this command means one thing copied from the rendered page and another copied from the source. Use a single unambiguous token (`-run Dora`) or write the alternation unescaped (`-run 'Dora|Weekly|Artifact'`)", path, where, p)
						break
					}
				}

				// Rule 5: an unsubstituted metavariable — the row cannot run.
				if mv := unsubstitutedMetavars(cmd); len(mv) > 0 {
					add("%s: %s carries the unsubstituted placeholder(s) %s — the command cannot run as literally written. A verifier either gets an error instead of a verdict, or silently substitutes a value, in which case the command recorded in the brief is not the command that produced the Evidence and the row is no longer reproducible. Substitute a concrete value, or derive it in the command (a `$(…)` lookup); if the row is genuinely manual, move the placeholder out of the code span. Only bracket and ellipsis placeholder shapes are decidable from the text — one spelled as a plain word is NOT — so this check is a LOWER BOUND on the class, never the complete set", path, where, strings.Join(mv, ", "))
				}
			})
		}
	}
	sort.Strings(notices)
	return notices
}
