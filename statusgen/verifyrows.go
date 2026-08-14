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
	perl     bool     // -P / --perl-regexp (GNU-only; see the portability rule)
	fixed    bool     // -F / --fixed-strings / fgrep — a pipe is then LITERAL BY INTENT
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
		if name != "grep" && name != "egrep" && name != "fgrep" && name != "ggrep" && name != "rg" {
			continue
		}
		if name == "rg" {
			continue // ripgrep: different flag surface, not in scope
		}
		g := grepCall{extended: name == "egrep", fixed: name == "fgrep", last: lastFlags[ci]}
		var operands []string
		for j := k + 1; j < len(c); j++ {
			t := c[j]
			switch {
			case !t.quoted && strings.HasPrefix(t.text, "--"):
				flag, val, hasVal := strings.Cut(t.text[2:], "=")
				switch flag {
				case "extended-regexp":
					g.extended = true
				case "perl-regexp":
					g.perl = true
				case "fixed-strings":
					g.fixed = true
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
					case 'P':
						g.perl = true
					case 'F':
						g.fixed = true
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

// goTestPattern is one RE2 selector argument lifted from a `go test`
// invocation, with the flag it came from so the notice can name it.
type goTestPattern struct {
	flag string // "-run" or "-bench"
	pat  string
}

// goTestRegexpFlags are the `go test` flags whose argument is compiled as an
// RE2 regexp against test/benchmark names. Both share the defect: a pattern
// that matches nothing exits 0 with "no tests to run" / "no benchmarks to
// run" — a green row that ran nothing. `-bench` was the half #374 named and
// the original rule missed.
var goTestRegexpFlags = map[string]bool{"run": true, "bench": true, "fuzz": true}

// goTestRunPatterns extracts every `-run` / `-bench` / `-fuzz` regexp argument
// (in the `-run PAT`, `-run=PAT` and `--run=PAT` forms) from `go test`
// invocations in a tokenized command.
//
// Unlike grepCalls this does not track pipeline position: a `-run` regexp
// that matches no test name makes `go test` print "no tests to run" and exit
// 0 wherever it sits — there is no "not the last stage, so it doesn't matter"
// case to exclude.
func goTestRunPatterns(toks []shellTok) []goTestPattern {
	var patterns []goTestPattern
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
			// The quoting flag is NOT a filter here: `-run='A\|B'` tokenizes as
			// a single token that is marked quoted because its VALUE was, and
			// that is the exact form the rule must see.
			if !strings.HasPrefix(t.text, "-") {
				continue
			}
			name, val, hasVal := strings.Cut(strings.TrimLeft(t.text, "-"), "=")
			if !goTestRegexpFlags[name] {
				continue
			}
			switch {
			case hasVal:
				patterns = append(patterns, goTestPattern{flag: "-" + name, pat: val})
			case j+1 < len(c):
				j++
				patterns = append(patterns, goTestPattern{flag: "-" + name, pat: c[j].text})
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
// Rule 6 — `go run` under an expectation of a SPECIFIC non-zero exit (#493)
// ---------------------------------------------------------------------------
//
// `go run` does not propagate the program's exit status. When the program
// exits non-zero, `go run` prints `exit status N` to stderr and itself exits
// **1** — measured, not inferred: a `main` calling `os.Exit(5)` under `go run`
// yields 1.
//
// So EVERY non-zero exit flattens to 1, and a row asserting a specific failure
// code through `go run` asserts nothing of the kind: it cannot tell 3
// (disabled) from 4 (rate-limited) from 5 (refused) from 6 (precondition
// unverifiable) — the desk-tools scoping contract's whole point. The rows
// meant to prove fail-closed behaviour are the exact rows this silently
// disarms, and they go green either way, so the defect is not discoverable
// from a passing run.
//
// A row expecting `1` is flagged too, and for a sharper reason than
// bookkeeping: `go run` also exits 1 when the package DOES NOT COMPILE. Such a
// row passes on a tree whose code never built — the strongest possible
// instance of a check that cannot fail.
//
// A row expecting exit 0 is unaffected and stays silent.

// exitExprRe matches an exit-code assertion in an Expect cell: `rc=3`,
// `rc = 3`, `exit 5`, `exit code 6`, `exit status 4`, `exits 2`, `$? = 3`.
// The code is capture group 1.
var exitExprRe = regexp.MustCompile(`(?i)(?:\brc\b|\bexit(?:s|ed)?(?:[ \t-]*(?:code|status))?|\$\?)[ \t]*[:=]?[ \t]*(\d+)`)

// expectClauseBreaks end the ASSERTION at the head of an Expect cell. What
// follows is commentary, and commentary in this corpus routinely discusses
// exit codes it does not assert — publication/05 row 6b is the built-binary
// FIX for #493 and its Expect cell explains the defect, naming 1, 5 and 6.
// Reading those as the row's expectation would make the rule fire on the very
// row that demonstrates its remedy.
var expectClauseBreaks = []string{". ", ".\n", " — ", " – ", " -- ", ";", "\n"}

// expectClause returns the leading assertion of an Expect cell, with markdown
// emphasis stripped. Same narrowness as expectsZeroCount, and for the same
// reason: an Expect cell is an assertion followed by prose, and only the
// assertion binds.
func expectClause(expect string) string {
	s := strings.NewReplacer("`", "", "*", "", "**", "").Replace(expect)
	cut := len(s)
	for _, brk := range expectClauseBreaks {
		if i := strings.Index(s, brk); i >= 0 && i < cut {
			cut = i
		}
	}
	return strings.TrimSpace(s[:cut])
}

// expectedNonZeroExits reports the specific non-zero exit codes an Expect cell
// asserts. Empty when the cell asserts none (prose, a count, or exit 0).
func expectedNonZeroExits(expect string) []string {
	s := expectClause(expect)
	var out []string
	seen := map[string]bool{}
	for _, m := range exitExprRe.FindAllStringSubmatch(s, -1) {
		code := strings.TrimLeft(m[1], "0")
		if code == "" || seen[m[1]] { // "0", "00" → zero, not a failure code
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// goRunInvoked reports whether a command runs a Go program through `go run`.
// `go run` anywhere in the command is enough: the row's status is the last
// stage's, and every shape seen in the corpus puts the `go run` there.
func goRunInvoked(toks []shellTok) bool {
	prevGo := false
	for _, t := range toks {
		if t.op {
			prevGo = false
			continue
		}
		if prevGo && t.text == "run" {
			return true
		}
		prevGo = t.text == "go" && !t.quoted
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule 7 — a pipe in a BASIC-regex grep pattern (#262)
// ---------------------------------------------------------------------------
//
// Rule 1 catches `\|` carried INTO an ERE. This is the mirror: alternation
// written for a grep that has no `-E`.
//
// In a POSIX basic regular expression `|` is an ordinary character, so
// `grep -c "alpha|beta|gamma" doc.md` searches for the single 12-character
// string `alpha|beta|gamma`. In a brief, the one line in the document
// containing that string is THE VERIFY ROW ITSELF — the row returns 1, exits
// 0, and against a `≥1` bar reports PASS having measured nothing. That is
// #262's measured failure, not a hypothetical: #257's §6 table
// returned 1,1,1,1,1,1,1 against thresholds 3,3,1,3,2,3,3, each match being
// the row's own line.
//
// The markdown escape makes it worse rather than better. A GFM table cell can
// only carry a pipe as `\|`, and GFM resolves the escape before inline
// parsing — so the source says `grep -c "a\|b"` while the RENDERED page a
// verifier copies from says `grep -c "a|b"`. Those are two different commands:
// GNU-compatible greps read `\|` as alternation (a GNU extension, absent from
// POSIX BRE), and every grep reads a bare `|` in a BRE as a literal. The row
// therefore means one thing to whoever reads the source and another to whoever
// runs the rendered text, and neither reading is stable across platforms.
//
// The fix is the same either way, and is unambiguous in both forms: add `-E`,
// or write the alternatives as separate `-e` patterns. `-F`/`fgrep` and a
// `[\|]` bracket class are the escape hatches for a genuinely literal pipe.

// pipeOutsideBracket reports whether a pattern contains a pipe — escaped or
// bare — outside a bracket expression. Bracket content is exempt for the same
// reason as escapedPipeOutsideBracket: `[\|]` and `[|]` are the sanctioned way
// to mean a literal pipe.
func pipeOutsideBracket(pat string) bool {
	if escapedPipeOutsideBracket(pat) {
		return true
	}
	inBracket := false
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch {
		case !inBracket && c == '[':
			inBracket = true
			j := i + 1
			if j < len(pat) && pat[j] == '^' {
				j++
			}
			if j < len(pat) && pat[j] == ']' {
				i = j
			}
		case inBracket && c == ']':
			inBracket = false
		case !inBracket && c == '|':
			return true
		case c == '\\' && i+1 < len(pat):
			i++
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule 8 — a Command cell shredded by a RAW pipe (#374)
// ---------------------------------------------------------------------------
//
// A raw `|` inside a GFM table cell is a CELL DELIMITER, wherever it sits — a
// code span does not protect it, because the escape is resolved before inline
// parsing. So `go test -run 'A|B'` written raw in a Verify row does not
// survive as a command at all: the cell ends at the first pipe, the rest
// becomes extra columns, and the Expect column shifts.
//
// Three things break at once, which is why this is its own rule rather than a
// rendering nit: the brief shows a command nobody can run; the row's Expect
// cell is no longer the row's expectation; and every other rule here goes
// BLIND past the cut, so a shredded row is silently exempt from the whole
// lint. #374's "cell-splitter truncation hazard" is exactly this.
//
// Detection is structural: a Command cell whose backticks do not pair up has
// had its code span cut by a delimiter.

// truncatedCodeSpan reports whether a table cell's code span was cut by a cell
// delimiter — an odd number of backticks means the span never closed.
func truncatedCodeSpan(cell string) bool {
	return strings.Count(cell, "`")%2 == 1
}

// ---------------------------------------------------------------------------
// Rule 9 — a diff base pinned to a MOVING ref (#639)
// ---------------------------------------------------------------------------
//
// A row whose base is a branch name is not a function of the tree under test.
// Measured on methodology-metrics/21: the identical
// `statusgen --consumers --base origin/main` returned exit 1 on one invocation
// and exit 2 on the next, purely because background commits advanced
// `origin/main` between them (7c4b752 → c51bb06) and the three-dot diff base
// moved underneath it.
//
// That is a flappy gate — a check that passes or "could-not-check" depending
// on WHEN it runs relative to another branch's motion, not on the content it
// claims to judge — and it is worse than a check that simply fails, because
// the green run and the red run are equally unreproducible. Re-running it
// proves nothing, so a reviewer cannot settle a disagreement about it.
//
// `refs/remotes/origin/main` is NOT an exemption: it is the same ref by its
// full name, and it moves on every fetch.
//
// The exemptions are the two forms that ARE pure functions of the PR: an
// explicit merge-base computation (`$(git merge-base …)` — #639's own
// suggested fix), and a pinned SHA or shell variable holding one.

// movingRefRe matches a branch-shaped ref whose tip moves independently of the
// tree under test. Bare `HEAD` is deliberately excluded — it names the commit
// under test, which is the fixed end of the comparison, not the moving one.
var movingRefRe = regexp.MustCompile(`^(?:refs/(?:remotes|heads)/)?(?:[A-Za-z0-9._-]+/)?(?:main|master|develop|trunk|HEAD)$`)

// baseFlags are flags whose value names the other end of a comparison.
var baseFlags = map[string]bool{"base": true, "since": true, "from": true, "against": true, "baseline": true}

// movingRef reports whether a ref name moves independently of the commit under
// test. A `$` (command substitution or variable) means the value is computed,
// which is the sanctioned fix, and a hex string is a pin.
func movingRef(s string) bool {
	if s == "" || s == "HEAD" || strings.ContainsAny(s, "$`") {
		return false
	}
	return movingRefRe.MatchString(s)
}

// movingRefBases reports every moving ref used as a comparison endpoint in a
// command: as the value of a base-shaped flag, or as an endpoint of a git
// `A..B` / `A...B` range.
//
// A command that computes a merge-base is exempt wholesale — `git merge-base
// origin/main HEAD` READS the moving ref precisely to pin it, which is the fix
// this rule asks for, not the defect.
func movingRefBases(toks []shellTok) []string {
	for _, t := range toks {
		if !t.op && strings.Contains(t.text, "merge-base") {
			return nil
		}
	}
	var out []string
	seen := map[string]bool{}
	addOnce := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for i, t := range toks {
		if t.op || t.quoted {
			continue
		}
		// `--base <ref>` / `--base=<ref>` and friends.
		if strings.HasPrefix(t.text, "-") {
			name, val, hasVal := strings.Cut(strings.TrimLeft(t.text, "-"), "=")
			if !baseFlags[name] {
				continue
			}
			if !hasVal && i+1 < len(toks) && !toks[i+1].op {
				val = toks[i+1].text
			}
			if movingRef(val) {
				addOnce(val)
			}
			continue
		}
		// `origin/main..HEAD` / `origin/main...HEAD` range endpoints.
		if !strings.Contains(t.text, "..") {
			continue
		}
		sep := "..."
		if !strings.Contains(t.text, "...") {
			sep = ".."
		}
		lhs, rhs, _ := strings.Cut(t.text, sep)
		for _, end := range []string{lhs, rhs} {
			if movingRef(end) {
				addOnce(end)
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Rule 10 — GNU-only constructs in a row meant to run on macOS too (#650)
// ---------------------------------------------------------------------------
//
// Verify rows are run by whoever verifies, on whatever machine they have —
// macOS desks and ubuntu CI both. A row that only works on one of them is not
// portable evidence: it produces a different verdict per platform, and the
// platform is invisible in the Evidence cell.
//
// The catalogued instance is #650's, and it is the dangerous kind because it
// fails QUIETLY: desk-hardening/06 rows 1,4,5,6,7 fed `<(sed …)` to grep, and
// under BSD grep `/dev/fd/N` reads as EMPTY — so the rows returned count 0 /
// rc 1 with the content plainly present (the `sed … | grep` pipe form returned
// the real counts 18/13/4/7/10). An empty read is indistinguishable from a
// genuine absence, so the failure looks like a finding about the code rather
// than a finding about the row.
//
// Severity NOTICE, and deliberately advisory-first: unlike the rules above,
// these rows are not vacuous — they are correct on one platform. The fix is
// mechanical in every case (a pipe instead of a process substitution, `-E`
// instead of `-P`, `sed -i ''`), so the notice names the substitute.

// gnuOnlyCommandFlags maps a command name to the flags of it that are GNU-only.
var gnuOnlyCommandFlags = map[string]map[string]string{
	"date":     {"-d": "`date -d`", "--date": "`date --date`"},
	"stat":     {"-c": "`stat -c`", "--format": "`stat --format`"},
	"readlink": {"-f": "`readlink -f`"},
	"xargs":    {"-r": "`xargs -r`", "--no-run-if-empty": "`xargs --no-run-if-empty`"},
	"sort":     {"-V": "`sort -V`"},
}

// gnuOnlySubstitute names the portable replacement for each construct, so the
// notice tells the author what to write instead of only what not to.
var gnuOnlySubstitute = map[string]string{
	"`<(…)` process substitution":    "pipe instead (`sed … | grep -cE …`) — BSD `grep` reads `/dev/fd/N` as EMPTY, so the row returns 0 matches with the content plainly present (#650, measured on desk-hardening/06 rows 1,4,5,6,7)",
	"`grep -P`":                      "use `grep -E`; BSD `grep` has no `-P` and exits 2 with a usage error",
	"`sed -i` with no backup suffix": "write `sed -i ''` (BSD requires the suffix argument; GNU accepts `-i ''` too), or `sed … > tmp && mv tmp file`",
	"`date -d`":                      "use `date -j -f` or compute the date in the tool under test",
	"`date --date`":                  "use `date -j -f` or compute the date in the tool under test",
	"`stat -c`":                      "use `wc -c < file` for a size, or `stat -f` guarded by an OS check",
	"`stat --format`":                "use `wc -c < file` for a size, or `stat -f` guarded by an OS check",
	"`readlink -f`":                  "use `cd \"$(dirname x)\" && pwd -P`, or `python3 -c 'import os,sys;print(os.path.realpath(sys.argv[1]))'`",
	"`xargs -r`":                     "guard the pipeline instead (`[ -s list ] && xargs …`); BSD `xargs` skips an empty input already",
	"`xargs --no-run-if-empty`":      "guard the pipeline instead (`[ -s list ] && xargs …`); BSD `xargs` skips an empty input already",
	"`sort -V`":                      "sort the fields explicitly (`sort -t. -k1,1n -k2,2n -k3,3n`)",
	"`tac`":                          "use `tail -r` guarded by an OS check, or reverse in the tool under test",
	"`mapfile`/`readarray`":          "read the lines with a `while read` loop; these are bash 4+ builtins and macOS ships bash 3.2",
}

// gnuOnlyConstructs reports the GNU-only constructs a command uses.
func gnuOnlyConstructs(cmd string, toks []shellTok, greps []grepCall) []string {
	var out []string
	seen := map[string]bool{}
	addOnce := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if strings.Contains(stripQuotedRegions(cmd), "<(") {
		addOnce("`<(…)` process substitution")
	}
	for _, g := range greps {
		if g.perl {
			addOnce("`grep -P`")
		}
	}
	// Per-simple-command flag scan.
	var cmds [][]shellTok
	var cur []shellTok
	for _, t := range toks {
		if t.op {
			if len(cur) > 0 {
				cmds = append(cmds, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		cmds = append(cmds, cur)
	}
	for _, c := range cmds {
		k := 0
		for k < len(c) && (c[k].text == "{" || c[k].text == "!" || c[k].text == "(") {
			k++
		}
		if k >= len(c) || c[k].quoted {
			continue
		}
		name := c[k].text
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		switch name {
		case "tac":
			addOnce("`tac`")
		case "mapfile", "readarray":
			addOnce("`mapfile`/`readarray`")
		case "sed":
			for j := k + 1; j < len(c); j++ {
				if c[j].quoted || !strings.HasPrefix(c[j].text, "-i") {
					continue
				}
				// GNU form: `-i` with no suffix, or `-i.bak` glued on.
				// BSD needs the suffix as a SEPARATE argument, `-i ''`.
				if c[j].text == "-i" && j+1 < len(c) && c[j+1].quoted && c[j+1].text == "" {
					break // `sed -i ''` — the portable spelling
				}
				addOnce("`sed -i` with no backup suffix")
				break
			}
		}
		flags, ok := gnuOnlyCommandFlags[name]
		if !ok {
			continue
		}
		for j := k + 1; j < len(c); j++ {
			if c[j].quoted {
				continue
			}
			flag, _, _ := strings.Cut(c[j].text, "=")
			if label, hit := flags[flag]; hit {
				addOnce(label)
			}
		}
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

	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported elsewhere; legacy/opted-out exempt
			}
			verifyRowTable(bf.Verify, func(num, cmdCell, expect string) {
				where := "a Verify row"
				if num != "" {
					where = "Verify row " + num
				}
				for _, f := range rowFindings(cmdCell, expect) {
					notices = append(notices, fmt.Sprintf("%s: %s [%s] %s", path, where, f.rule, f.msg))
				}
			})
		}
	}
	sort.Strings(notices)
	return notices
}

// rowFinding is one defect found in one Verify row. rule is a stable,
// greppable tag — CI steps and the row-audit inventory select on it, so it is
// part of the interface and does not change with the wording of msg.
type rowFinding struct {
	rule string
	msg  string
}

// Rule tags. Stable identifiers, one per shape.
const (
	ruleERELiteralPipe = "ere-literal-pipe"      // #509 — `\|` inside grep -E
	ruleGrepZeroCount  = "grep-zero-count"       // #509 — grep -c gated on 0
	ruleExitSwallowed  = "pipeline-exit-sunk"    // #509 — pipeline sink eats the status
	ruleRE2LiteralPipe = "rE2-literal-pipe"      // #374 — `\|` inside go test -run/-bench
	ruleMetavar        = "unsubstituted-metavar" // #509 — the row cannot run
	ruleGoRunExit      = "gorun-exit"            // #493 — go run flattens the exit code
	ruleBREAlternation = "bre-alternation"       // #262 — a pipe in a BRE grep pattern
	ruleShreddedCell   = "shredded-cell"         // #374 — raw `|` cut the Command cell
	ruleMovingRef      = "moving-ref"            // #639 — diff base on a moving ref
	rulePortability    = "gnu-only"              // #650 — GNU-only construct
)

// rowFindings applies every row rule to one Verify row's Command and Expect
// cells. It is the single implementation: unfailableRowNotices formats its
// output, and the tests drive it directly, so a rule cannot be exercised in
// one and silently absent from the other.
func rowFindings(cmdCell, expect string) []rowFinding {
	var out []rowFinding
	add := func(rule, format string, a ...any) {
		out = append(out, rowFinding{rule: rule, msg: fmt.Sprintf(format, a...)})
	}

	// Rule 8 first: a shredded cell means every rule below reads a TRUNCATED
	// command, so the finding has to say the row was cut before it reports
	// anything about what survived.
	if truncatedCodeSpan(cmdCell) {
		add(ruleShreddedCell, "the Command cell's code span is unterminated — a RAW `|` in the command was read as a table-cell delimiter, cutting the command at the pipe and shifting every column after it (the Expect cell shown is another fragment of the command, not an expectation). The brief therefore prints a command nobody can run, and every other row check goes blind past the cut. Escape shell pipes and regex alternations as `\\|` inside the table cell")
	}

	cmd := codeSpan(cmdCell)
	if cmd == "" {
		return out
	}
	toks := tokenizeCommand(cmd)
	greps := grepCalls(toks)

	for _, g := range greps {
		// Rule 1: `\|` in an ERE pattern.
		if g.extended && !g.fixed {
			for _, p := range g.patterns {
				if escapedPipeOutsideBracket(p) {
					add(ruleERELiteralPipe, "uses `\\|` inside a `grep -E` pattern (%q) — in an extended regex `\\|` is a LITERAL pipe, not alternation, so the row matches almost nothing and passes whatever the file contains. The raw text is also ambiguous: GFM renders `\\|` in a table cell as `|`, so this command means one thing copied from the rendered page and another copied from the source. Write the alternatives as separate patterns (`grep -E -e alpha -e beta`), which reads identically in both. For a genuine literal pipe use a bracket class (`[\\|]`)", p)
					break
				}
			}
		}
		// Rule 7: a pipe in a BASIC-regex grep pattern.
		if !g.extended && !g.perl && !g.fixed {
			for _, p := range g.patterns {
				if pipeOutsideBracket(p) {
					add(ruleBREAlternation, "uses a pipe inside a grep pattern (%q) with no `-E`/`-P` — that grep compiles a BASIC regex, where `|` is an ORDINARY CHARACTER. The pattern therefore searches for one long literal string, and in a brief the single line containing that string is THE VERIFY ROW ITSELF: the row returns 1, exits 0, and passes a `≥1` bar having measured nothing (#257 returned 1,1,1,1,1,1,1 against thresholds 3,3,1,3,2,3,3 this way). The `\\|` spelling is no safer — GFM renders it as a bare `|`, so the source and the rendered page are different commands, and `\\|` alternation is a GNU extension absent from POSIX BRE. Write the alternatives as separate `-e` patterns (`grep -cE -e alpha -e beta`), which carries no pipe at all and so reads identically in the source and on the rendered page. Adding `-E` alone to a `\\|` pattern only trades this defect for the mirror one, where `\\|` becomes a literal pipe in the extended regex. For a genuinely literal pipe use `-F` or a `[\\|]` bracket class", p)
					break
				}
			}
		}
		// Rule 2: `grep -c` gated on an expected count of zero.
		if g.count && g.last && expectsZeroCount(expect) && !forcesSuccess(toks) {
			add(ruleGrepZeroCount, "expects a count of `0` from `grep -c`, but grep exits 1 when it matches nothing — on the success path the row FAILS, and it only passes when it finds what it was meant to prove absent. Gate on the exit status instead (`! grep -qE …`), or keep the count as output and neutralise the status (`grep -cE … || true`)")
		}
	}

	// Rule 3: exit status swallowed by an always-zero sink.
	if sink := pipelineSwallowsExit(toks); sink != "" {
		add(ruleExitSwallowed, "pipes into `%s`, so the row reports `%s`'s exit status, not the check's — the command before it can fail and the row still passes. Assert on the real command (write to a file, then check it), or set `set -o pipefail`", sink, sink)
	}

	// Rule 4: `\|` in a `go test -run` / `-bench` pattern.
	for _, p := range goTestRunPatterns(toks) {
		if escapedPipeOutsideBracket(p.pat) {
			add(ruleRE2LiteralPipe, "uses `\\|` inside a `go test %s` pattern (%q) — `go test %s` compiles its argument as RE2, same as `grep -E`: `\\|` there is a LITERAL pipe, not alternation, so the pattern matches no name, `go test` reports \"no tests to run\", and the row passes having run nothing. The raw text is also ambiguous: GFM renders `\\|` in a table cell as `|`, so this command means one thing copied from the rendered page and another copied from the source. Inside a table cell there is no spelling of an RE2 alternation that is unambiguous — a raw `|` is a cell delimiter and shreds the row — so do not try to fix the pattern: use a single unambiguous token (`%s Dora`), or chain single-pattern runs (`go test %s A ./... && go test %s B ./...`), or move the command into a fenced block outside the table", p.flag, p.pat, p.flag, p.flag, p.flag, p.flag)
			break
		}
	}

	// Rule 5: an unsubstituted metavariable — the row cannot run.
	if mv := unsubstitutedMetavars(cmd); len(mv) > 0 {
		add(ruleMetavar, "carries the unsubstituted placeholder(s) %s — the command cannot run as literally written. A verifier either gets an error instead of a verdict, or silently substitutes a value, in which case the command recorded in the brief is not the command that produced the Evidence and the row is no longer reproducible. Substitute a concrete value, or derive it in the command (a `$(…)` lookup); if the row is genuinely manual, move the placeholder out of the code span. Only bracket and ellipsis placeholder shapes are decidable from the text — one spelled as a plain word is NOT — so this check is a LOWER BOUND on the class, never the complete set", strings.Join(mv, ", "))
	}

	// Rule 6: `go run` under an expectation of a specific non-zero exit code.
	if codes := expectedNonZeroExits(expect); len(codes) > 0 && goRunInvoked(toks) {
		add(ruleGoRunExit, "expects exit code %s from a `go run` invocation — but `go run` does NOT propagate the program's status: it prints `exit status N` to stderr and itself exits 1, so every non-zero code flattens to 1 and the row cannot tell 3 from 4 from 5 from 6. `go run` also exits 1 when the package fails to COMPILE, so the row passes on a tree whose code never built. Build once and assert on the binary: `go build -o /tmp/tool ./cmd/tool && /tmp/tool …; echo $?`", strings.Join(codes, "/"))
	}

	// Rule 9: a comparison base pinned to a moving ref.
	if refs := movingRefBases(toks); len(refs) > 0 {
		add(ruleMovingRef, "compares against the moving ref(s) %s — the row is then a function of another branch's tip, not of the tree under test, so the identical command returns different answers as that ref advances (measured: `--consumers --base origin/main` returned exit 1 and exit 2 on consecutive runs while background commits moved main). A green run and a red run are equally unreproducible, so re-running settles nothing. Pin the base: `--base $(git merge-base origin/main HEAD)`, an explicit SHA, or the PR's own `base.sha`. `refs/remotes/origin/main` is the same moving ref by its long name, not a fix", strings.Join(refs, ", "))
	}

	// Rule 10: GNU-only constructs — the row answers differently per platform.
	for _, c := range gnuOnlyConstructs(cmd, toks, greps) {
		add(rulePortability, "uses %s, which is GNU-only — the row is run by whoever verifies, on macOS desks as well as ubuntu CI, and it does not mean the same thing on both. Write it %s", c, gnuOnlySubstitute[c])
	}

	return out
}
