package writepolicy

import (
	"path"
	"strings"
)

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command RUNS the gh CLI's token read
// in one of the forms this lexer understands. It is a lexical backstop, not a
// shell: the forms it judges are the ones its tests pin, and the classes it
// does not judge are listed in docs/write-policy.md ("Not covered").
//
// The read is a gh invocation whose first positional is `auth` and a later
// positional `token` (case-insensitive: the match fails closed on case),
// `gh auth status` asked to print the token (--show-token, any true spelling
// of it, or a short cluster with -t), `gh auth git-credential`, or `gh config
// get … oauth_token`. A gh flag's VALUE (-u, -h, …) after the subcommand is
// not a positional.
//
// What counts as executed:
//
//   - a command word, after leading NAME=value assignments, redirections and
//     shell reserved words; after a command wrapper (the shared
//     commandWrappers set) or runner (forgeRunners), or after a command word
//     computed at run time, every later word is tried once as the wrapped
//     command, so a wrapper's own option values cannot hide it, and the
//     number of positions judged is linear in the words. Runners outside those
//     two closed sets are not judged;
//   - the script of a shell's -c (or --command), of `eval`, of `env -S`, of a
//     runner that hands its argument to a shell (forgeArgScripts), a
//     here-string fed to a shell, and the text the IMMEDIATELY preceding pipe
//     stage passes to a shell as its arguments; each is lexed again as a
//     script of its own;
//   - a `$( … )` or backtick substitution, quoted or not, lexed as a script;
//   - the body of a heredoc whose introducing command line names a shell
//     anywhere (piped, wrapped or after an assignment), lexed as a script; the
//     substitutions in the body of a heredoc under an unquoted delimiter.
//
// Every `<<` is read as a heredoc except inside a `((…))` the scan has
// confirmed as arithmetic the way bash decides it (arithmeticEnd), where it is
// a shift and nothing runs. A `<<` the scan cannot place (after a `((` or
// `$((` it cannot confirm either way, after `$[`, or inside a `${…}` or a
// `name[…]` subscript) may be a shift in bash, so a heredoc it opens whose
// delimiter line exists is undecided. A heredoc body ends at the first line
// that is exactly its delimiter word (after its leading tabs, for `<<-`), as
// the shell compares it.
//
// A `$( … )` (unquoted, double-quoted or in a heredoc body) holding a heredoc
// ends where the shells disagree when a body holds a `)`: bash 4+ and zsh read
// the bodies to find its end (substEnd); bash 3.2, which is macOS /bin/sh,
// takes the first `)` that closes the parentheses. The scan follows the first
// reading and judges the rest of the script under the second one as well
// (fork). A heredoc in a substitution whose delimiter line is not inside it,
// or whose end the scan cannot place, is undecided.
//
// The scan reads the RAW command and does its own quote, comment and heredoc
// handling (it uses none of the shared maskers), so quoting a word
// (`gh "auth" 'token'`, ANSI-C `$'…'` with its escapes decoded, locale
// `$"…"`), escaping it (`\gh`) or splitting it with a line continuation does
// not hide it: the shell removes all of those before gh sees its arguments.
// A word with a part computed at run time (parameter expansion, a
// substitution, brace expansion, a glob) is DYNAMIC: it may stand for any one
// of the command word, `auth` or the subcommand, so one such component with
// the rest literal is judged a read; two or more are not judged.
//
// Inert data: a comment, a heredoc body written to a non-shell under a quoted
// delimiter, and a quoted argument to a non-shell command are prose.
//
// Cost: every pass over a script's bytes is charged to one work budget
// (forgeBudgetPerByte per input byte plus forgeBudgetBase): a script is
// charged its length before it is lexed, the lexer walks each of its bytes a
// bounded number of times, and every per-word step (a judged position, an
// examined argument, a re-read of an already-lexed script) is charged the
// length of the word it reads. So the scan's own work is at most a constant
// times the budget, which is linear in the command's length. This bounds the
// forge scan only: the shared masking Classify runs before it is a separate
// cost (#4854).
//
// It fails CLOSED where it cannot finish (a desk-decided departure from
// allow-and-log, recorded on the PR that added this): an unterminated quote,
// or a heredoc whose delimiter line never comes, is lexed as commands; and
// nesting deeper than maxForgeTokenDepth, a scan that runs out of its work
// budget, or a heredoc the scan cannot place (above) is judged a read
// outright, whatever the text says.
func forgeTokenRead(raw string) bool {
	hit, _ := forgeTokenReadStats(raw)
	return hit
}

// forgeStats is the work a classification did, for tests that pin its cost.
type forgeStats struct {
	judged    int  // command positions judged
	spent     int  // work units charged
	exhausted bool // the budget ran out, so the command was refused undecided
}

func forgeTokenReadStats(raw string) (bool, forgeStats) {
	budget := forgeBudgetPerByte*len(raw) + forgeBudgetBase
	st := &forgeState{budget: budget}
	// Exhaustion aborts the scan with a hit (every caller of spend returns
	// true on failure), so an undecided scan is a refusal.
	hit := st.script(raw, 0) || st.exhausted
	return hit, forgeStats{judged: st.judged, spent: budget - st.budget, exhausted: st.exhausted}
}

const (
	// maxForgeTokenDepth bounds script-in-script recursion.
	maxForgeTokenDepth = 8
	// The work budget: units per input byte, plus a floor. A script costs
	// its length plus one; a judged position, an examined argument or a
	// re-read script costs its word's length plus one.
	forgeBudgetPerByte = 32
	forgeBudgetBase    = 4096
)

type forgeState struct {
	budget    int
	judged    int
	exhausted bool
}

// spend charges n work units; false once the budget is gone. Every caller
// treats false as a hit and returns at once, so no work follows exhaustion.
func (st *forgeState) spend(n int) bool {
	if st.exhausted {
		return false
	}
	st.budget -= n
	if st.budget < 0 {
		st.exhausted = true
		return false
	}
	return true
}

// script lexes cmd as a script of its own: a top-level command, or the text a
// `sh -c`, `eval`, here-string, pipe, heredoc or substitution executes.
func (st *forgeState) script(cmd string, depth int) bool {
	return st.lexScript(cmd, depth, false, false, false)
}

// lexScript is script with the lexer's starting state: subst marks the text of
// a `$( … )` whose end substEnd placed, ambig a script in which any `<<` may
// be a shift (the inside of a `$((` that is not confirmed either way), old a
// script read the way bash 3.2 reads it (see fork).
func (st *forgeState) lexScript(cmd string, depth int, subst, ambig, old bool) bool {
	if depth > maxForgeTokenDepth {
		// Nesting too deep to lex is not evidence of absence, and the text
		// cannot be judged by its spelling either: refuse.
		return true
	}
	if !st.spend(len(cmd) + 1) {
		return true
	}
	l := forgeLexer{st: st, raw: cmd, depth: depth, subst: subst, ambig: ambig, old: old,
		shifts: strings.Contains(cmd, "<<")}
	return l.scan()
}

// substScript lexes the text of a `$( … )`: under the bash 4+ reading its
// heredocs must all end inside it (subst); under the bash 3.2 one a heredoc
// left open runs to the end of the text, as that shell reads it.
func (l *forgeLexer) substScript(text string, ambig, old bool) bool {
	return l.st.lexScript(text, l.depth+1, !old, ambig, old)
}

// forgeLineFeedsShell reports whether a heredoc's introducing command line
// names a shell in any word (piped into one, behind a wrapper, after an
// assignment), not only the first.
func forgeLineFeedsShell(line string) bool {
	for _, f := range strings.FieldsFunc(line, func(r rune) bool {
		return strings.ContainsRune(" \t\n;&|()", r)
	}) {
		if forgeShell(forgeCommandName(strings.Trim(f, `"'`))) {
			return true
		}
	}
	return false
}

// forgeExtraShells are shells outside the shared shellNameRe that take a
// script with -c (or --command / -Command).
var forgeExtraShells = map[string]bool{
	"fish": true, "mksh": true, "yash": true, "posh": true, "oksh": true, "loksh": true,
	"rbash": true, "csh": true, "tcsh": true, "pwsh": true, "powershell": true,
}

func forgeShell(name string) bool { return shellNameRe.MatchString(name) || forgeExtraShells[name] }

// forgeRunners run the command in their later words without being in the
// shared commandWrappers; forgeArgScripts also hand an argument to a shell,
// so each of their arguments is lexed as a script too.
var (
	forgeRunners = map[string]bool{
		"doas": true, "run0": true, "busybox": true, "unbuffer": true, "script": true,
		"flock": true, "watch": true, "su": true, "runuser": true, "chronic": true,
	}
	forgeArgScripts = map[string]bool{
		"script": true, "flock": true, "watch": true, "su": true, "runuser": true,
	}
)

type forgeWord struct {
	s   string
	dyn bool // a part of the word is computed at run time
}

// forgeHeredoc is a heredoc whose body starts after the current line.
type forgeHeredoc struct {
	word   string
	quoted bool // the delimiter was quoted: the body is not expanded
	ambig  bool // the `<<` may be a shift in bash, not a heredoc
	dash   bool // `<<-`: leading tabs are removed from each body line
}

type forgeLexer struct {
	st     *forgeState
	raw    string
	depth  int
	words  []forgeWord
	cur    strings.Builder
	inWord bool
	dyn    bool // the current word has a part computed at run time
	brace  bool // the current word has an unquoted `{`
	glob   bool // the current word has an unquoted `[`
	stdin  []forgeWord
	// Heredocs read on the current line, and where that line began.
	pending   []forgeHeredoc
	lineStart int
	// ambig: a `((`, `$((`, `$[`, `${…<<…}` or `name[…<<…]` was met that the
	// scan cannot place, so any later `<<` may be a shift. subst: this script
	// is the text of a `$( … )`. shifts: the script holds a `<<` at all.
	ambig, subst, shifts bool
	// old: the script is read as bash 3.2 reads it (a substitution ends at
	// the first `)` closing its parentheses). forked: the bash 3.2 reading of
	// the rest of this script has been judged. pos: where scan starts.
	old, forked bool
	pos         int
}

// fork judges the rest of the script, from at, the way bash 3.2 reads it: a
// copy of the lexer in its current state, in which every substitution ends at
// the first `)` closing its parentheses. One fork per script covers that
// reading of everything after the first substitution the two readings end
// differently, so a lexer forks at most once, and a fork never does.
func (l *forgeLexer) fork(at int) bool {
	if l.old || l.forked {
		return false
	}
	l.forked = true
	if !l.st.spend(len(l.raw) - at + 1) {
		return true
	}
	f := forgeLexer{st: l.st, raw: l.raw, depth: l.depth,
		words: append([]forgeWord(nil), l.words...), inWord: l.inWord,
		dyn: l.dyn, brace: l.brace, glob: l.glob,
		stdin:   append([]forgeWord(nil), l.stdin...),
		pending: append([]forgeHeredoc(nil), l.pending...), lineStart: l.lineStart,
		ambig: l.ambig, subst: l.subst, shifts: l.shifts, old: true, pos: at}
	f.cur.WriteString(l.cur.String())
	return f.scan()
}

func (l *forgeLexer) flush() {
	if l.inWord {
		l.words = append(l.words, forgeWord{l.cur.String(), l.dyn})
		l.cur.Reset()
	}
	l.inWord, l.dyn, l.brace, l.glob = false, false, false, false
}

// end closes the current simple command and judges it. A pipe hands the
// command's arguments to the next one as the text on its stdin.
func (l *forgeLexer) end(pipe bool) bool {
	l.flush()
	c := forgeCommand{words: l.words, stdin: l.stdin}
	hit := l.st.judge(&c, l.depth)
	l.stdin = nil
	if pipe {
		l.stdin = c.args()
	}
	l.words = nil
	return hit
}

func (l *forgeLexer) word(s string) {
	l.cur.WriteString(s)
	l.inWord = true
}

// isParamStart reports whether b, after an unquoted or double-quoted `$`,
// starts a parameter expansion.
func isParamStart(b byte) bool {
	return b == '_' || b == '{' || ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') ||
		('0' <= b && b <= '9') || strings.IndexByte("@*#?$!-", b) >= 0
}

func (l *forgeLexer) scan() bool {
	s := l.raw
	for i := l.pos; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			if i+1 < len(s) {
				i++
				if s[i] != '\n' { // a line continuation joins the two words
					l.word(s[i : i+1])
				}
			}
		case c == '$' && i+1 < len(s) && s[i+1] == '\'':
			// ANSI-C quoting: the shell decodes the escapes, and `\'` does
			// not end the span.
			j := ansiCEnd(s, i+2)
			body := s[i+2 : j]
			l.word(decodeANSIC(body))
			if j >= len(s) && l.st.script(body, l.depth+1) {
				return true
			}
			i = j
		case c == '$' && i+1 < len(s) && s[i+1] == '"':
			// Locale quoting: the `$` is dropped and the span is double-quoted.
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				if l.st.script(s[i+1:], l.depth+1) {
					return true
				}
				l.word(s[i+1:])
				i = len(s)
				continue
			}
			l.word(s[i+1 : i+1+j])
			i += 1 + j
		case c == '"':
			j, hit := l.double(i)
			if hit {
				return true
			}
			if j >= len(s) && l.st.script(s[i+1:], l.depth+1) {
				return true
			}
			l.word(unescapeDouble(s[i+1 : min(j, len(s))]))
			i = j
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			ambig := false
			if i+2 < len(s) && s[i+2] == '(' {
				// `$((`: arithmetic when bash confirms it, where nothing
				// runs; nested subshells in a substitution when it does not.
				k, kind := l.arithmeticEnd(i + 3)
				if kind == arithExhausted {
					return true
				}
				if kind == arithYes {
					l.word("$(())")
					l.dyn = true
					i = k + 1
					continue
				}
				ambig = kind == arithUnplaced
			}
			k, split, refuse := l.substEnd(s, i+2, l.old)
			if refuse || split && l.fork(i) || l.substScript(s[i+2:k], ambig, l.old) {
				return true
			}
			l.word("$()")
			l.dyn = true
			i = k
		case c == '`':
			k := nextBacktick(s, i+1)
			if l.st.script(s[i+1:k], l.depth+1) {
				return true
			}
			l.word("``")
			l.dyn = true
			i = k
		case c == '$' && i+1 < len(s) && s[i+1] == '[':
			// Legacy `$[…]` arithmetic: a `<<` from here on may be a shift.
			l.ambig = true
			l.word("$")
			l.dyn = true
		case c == '$' && i+1 < len(s) && isParamStart(s[i+1]):
			if s[i+1] == '{' && l.shifts && !l.ambig {
				// Inside `${…}` a `<<` is a word's text, not a heredoc.
				hit, shift := l.spanShift(i+2, '{', '}')
				if hit {
					return true
				}
				l.ambig = shift
			}
			l.word("$")
			l.dyn = true
		case c == '#' && !l.inWord:
			for i+1 < len(s) && s[i+1] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\r':
			l.flush()
		case c == '<' || c == '>' || (c == '&' && i+1 < len(s) && s[i+1] == '>'):
			// A redirection operator is a word of its own (`token>/dev/null`
			// is the argument `token`), and its target is skipped later. A
			// file-descriptor number written against it (`2>`) belongs to it.
			fd := ""
			if l.inWord && strings.Trim(l.cur.String(), "0123456789") == "" {
				fd = l.cur.String()
				l.cur.Reset()
				l.inWord = false
			}
			l.flush()
			j := i
			for j < len(s) && (s[j] == '<' || s[j] == '>' || s[j] == '&') {
				j++
			}
			op := s[i:j]
			l.words = append(l.words, forgeWord{s: fd + op})
			i = j - 1
			if op == "<<" {
				// A heredoc: its delimiter is the operator's target word, and
				// its body starts after this command line.
				dash := j < len(s) && s[j] == '-'
				if dash {
					j++
				}
				w, quoted, end, odd := heredocWord(s, j)
				if odd {
					// A substitution or backquote in the delimiter word: the
					// shells read its end by their own rules, so the body's
					// end cannot be placed.
					return true
				}
				if w != "" || quoted {
					l.words = append(l.words, forgeWord{s: w})
					l.pending = append(l.pending, forgeHeredoc{w, quoted, l.ambig, dash})
					i = end - 1
				}
			}
		case c == '|':
			pipe := true
			if i+1 < len(s) && s[i+1] == '|' {
				pipe = false
				i++
			} else if i+1 < len(s) && s[i+1] == '&' {
				i++
			}
			if l.end(pipe) {
				return true
			}
		case c == '\n':
			if l.end(false) {
				return true
			}
			if len(l.pending) > 0 {
				next, hit := l.heredocs(s[l.lineStart:i], i+1)
				if hit {
					return true
				}
				i = next - 1
			}
			l.lineStart = i + 1
		case c == ';' || c == '&' || c == '(' || c == ')':
			if c == '(' && i+1 < len(s) && s[i+1] == '(' && !(i > 0 && (s[i-1] == '<' || s[i-1] == '>')) {
				// `((` (a `<(` or `>(` before it is a process substitution
				// whose first word is a subshell, read on the next pass).
				if !l.commandStart() {
					// Not where bash starts a command: `((` cannot be placed.
					l.ambig = true
					i++
				} else {
					k, kind := l.arithmeticEnd(i + 2)
					switch kind {
					case arithExhausted:
						return true
					case arithYes:
						i = k + 1 // confirmed arithmetic: nothing runs, `<<` is a shift
					case arithNo:
						i++ // nested subshells, as bash reads them
					default:
						l.ambig = true
						i++
					}
				}
			}
			if l.end(false) {
				return true
			}
		case c == '{' && !l.inWord && (i+1 >= len(s) || strings.IndexByte(" \t\r\n", s[i+1]) >= 0),
			c == '}' && !l.inWord:
			// A group brace; `{a,b}` and `x{a,b}` are brace expansion.
			if l.end(false) {
				return true
			}
		case c == '{':
			l.word("{")
			l.brace = true
		case c == '}':
			l.word("}")
			l.dyn = l.dyn || l.brace
		case c == '[':
			if l.shifts && !l.ambig && l.inWord && isName(l.cur.String()) {
				// `name[…]`: bash may read the subscript as one word, so a
				// `<<` in it is not a heredoc.
				hit, shift := l.spanShift(i+1, '[', ']')
				if hit {
					return true
				}
				l.ambig = shift
			}
			l.word("[")
			l.glob = true
		case c == ']':
			l.word("]")
			l.dyn = l.dyn || l.glob
		case c == '*' || c == '?':
			l.word(s[i : i+1])
			l.dyn = true
		default:
			l.word(s[i : i+1])
		}
	}
	if l.subst && len(l.pending) > 0 {
		// A heredoc opened on a substitution's last line has its body past the
		// substitution's end: that end cannot be trusted.
		return true
	}
	return l.end(false)
}

// heredocs reads, in order, the bodies of the heredocs pending on the command
// line that just ended, the first starting at start. A body fed to a shell is
// lexed as a script; under an unquoted delimiter its substitutions are; under
// a quoted one it is data. A body whose delimiter line never comes is lexed
// as commands with all that follows it (fail closed: a delimiter read
// differently from the shell cannot hide them). It returns the index after
// the last body. Inside a `$( … )` read the bash 4+ way a missing delimiter
// line is undecided instead (the substitution's end and the heredoc disagree),
// and so is a heredoc whose `<<` may be a shift: the lines this would read as
// its body may be commands.
func (l *forgeLexer) heredocs(line string, start int) (int, bool) {
	s := l.raw
	if !l.st.spend(len(line) + 1) {
		return len(s), true
	}
	fed := forgeLineFeedsShell(line)
	pend := l.pending
	l.pending = nil
	i := start
	for _, h := range pend {
		end, next := heredocEnd(s, i, h.word, h.dash)
		if end < 0 {
			if l.subst {
				return len(s), true
			}
			return len(s), l.st.script(s[i:], l.depth+1)
		}
		if h.ambig {
			return len(s), true
		}
		body := s[i:end]
		if fed && l.st.script(body, l.depth+1) || !fed && !h.quoted && l.expansions(body, 0, l.old) {
			return len(s), true
		}
		i = next
	}
	return i, false
}

// arithKind is how bash reads a `((`: arithmetic, nested subshells, or (for
// the scan) neither can be confirmed.
type arithKind int

const (
	arithNo        arithKind = iota // nested subshells (or no closing paren at all)
	arithYes                        // arithmetic: no command runs, `<<` is a shift
	arithUnplaced                   // the scan cannot confirm either reading
	arithExhausted                  // the work budget ran out
)

// arithmeticEnd decides a `((` (or `$((`) whose text starts at from the way
// bash does: it is arithmetic iff the first `)` that closes the text's
// parentheses is followed at once by another `)`. The scan confirms that only
// where it is a parenthesis count: plain text, `$#`, and a `${…}` or a
// double-quoted span holding no parenthesis, quote, backslash, backtick or
// nested expansion (which then cannot change the count, however bash reads
// it). Any other quote, backslash, backtick or comment character, or a `$(`,
// `$[`, `$'` or `$"` (which bash reads by their own rules) leaves it unplaced.
// It returns the index of that first closing `)`.
func (l *forgeLexer) arithmeticEnd(from int) (int, arithKind) {
	s := l.raw
	depth, j, kind := 0, from, arithNo
	plain := func(k int, end byte) int { // index of end after a plain run, or -1
		for ; k < len(s); k++ {
			switch c := s[k]; {
			case c == end:
				return k
			case c == '$' && k+1 < len(s) && strings.IndexByte("({['\"", s[k+1]) < 0:
			case strings.IndexByte("(){}'\"`\\$", c) >= 0:
				return -1
			}
		}
		return -1
	}
scan:
	for ; j < len(s); j++ {
		switch c := s[j]; {
		case c == '(':
			depth++
		case c == ')' && depth > 0:
			depth--
		case c == ')':
			if j+1 < len(s) && s[j+1] == ')' {
				kind = arithYes
			}
			break scan
		case c == '$' && j+1 < len(s) && s[j+1] == '#':
			j++ // the argument count, not a comment
		case c == '$' && j+1 < len(s) && s[j+1] == '{':
			k := plain(j+2, '}')
			if k < 0 {
				kind = arithUnplaced
				break scan
			}
			j = k
		case c == '"':
			k := plain(j+1, '"')
			if k < 0 {
				kind = arithUnplaced
				break scan
			}
			j = k
		case strings.IndexByte("'\"`\\#", c) >= 0,
			c == '$' && j+1 < len(s) && strings.IndexByte("({['\"", s[j+1]) >= 0:
			kind = arithUnplaced
			break scan
		}
	}
	if !l.st.spend(j - from + 1) {
		return j, arithExhausted
	}
	return j, kind
}

// commandStart reports whether the lexer stands where bash starts a command:
// no word yet on this command, or only reserved words (and `for`, `time`).
func (l *forgeLexer) commandStart() bool {
	if l.inWord {
		return false
	}
	for _, w := range l.words {
		if !shellReservedWords[w.s] && w.s != "for" && w.s != "time" {
			return false
		}
	}
	return true
}

// spanShift reports whether the span opened before from (by open, closed by
// the matching close) holds a `<<`, and whether the budget ran out reading it.
// The close is found as the shell finds it: an escaped byte, a quoted span and
// a substitution do not end the span or nest in it. A span the scan cannot
// read that way (an unterminated quote, an ANSI-C span, or a substitution or
// backquote that is unterminated or holds a `<<`) is reported as holding one:
// the scan then cannot place a later `<<` either.
func (l *forgeLexer) spanShift(from int, open, close byte) (exhausted, shift bool) {
	s := l.raw
	// skipSubst returns the index ending a `$(` or backquote at k, or -1.
	skipSubst := func(k int) int {
		var e int
		if s[k] == '`' {
			e = nextBacktick(s, k+1)
		} else {
			e = matchParen(s, k+2)
		}
		if e >= len(s) || strings.Contains(s[k:e], "<<") {
			return -1
		}
		return e
	}
	depth, j := 1, from
scan:
	for ; j < len(s) && depth > 0; j++ {
		switch c := s[j]; {
		case c == '\\':
			j++
		case c == '\'':
			k := strings.IndexByte(s[j+1:], '\'')
			if k < 0 {
				shift = true
				break scan
			}
			j += k + 1
		case c == '"':
			k := j + 1
			for ; k < len(s) && s[k] != '"'; k++ {
				switch {
				case s[k] == '\\':
					k++
				case s[k] == '`' || s[k] == '$' && k+1 < len(s) && s[k+1] == '(':
					if k = skipSubst(k); k < 0 {
						shift = true
						break scan
					}
				}
			}
			if k >= len(s) {
				shift = true
				break scan
			}
			j = k
		case c == '$' && j+1 < len(s) && s[j+1] == '\'':
			shift = true
			break scan
		case c == '`' || c == '$' && j+1 < len(s) && s[j+1] == '(':
			k := skipSubst(j)
			if k < 0 {
				shift = true
				break scan
			}
			j = k
		case c == open:
			depth++
		case c == close:
			depth--
		case c == '<':
			shift = shift || j+1 < len(s) && s[j+1] == '<'
		}
	}
	return !l.st.spend(j - from + 1), shift
}

// isName reports whether w is a shell variable name.
func isName(w string) bool {
	for i := 0; i < len(w); i++ {
		c := w[i]
		if c != '_' && !('a' <= c && c <= 'z') && !('A' <= c && c <= 'Z') && (i == 0 || !('0' <= c && c <= '9')) {
			return false
		}
	}
	return w != ""
}

// heredocEnd finds the delimiter line for a body starting at from: it returns
// the body's end and the index after that line, or -1 when no line matches.
// The line must be exactly the word, as the shell compares it: with dash
// (`<<-`) after its leading tabs are removed, and never with other blanks
// around it, since a line the shell keeps as body text would end the body
// early here and leave the rest of it to be lexed as commands.
func heredocEnd(s string, from int, word string, dash bool) (int, int) {
	for i := from; i < len(s); {
		line, next := s[i:], len(s)
		if le := strings.IndexByte(s[i:], '\n'); le >= 0 {
			line, next = s[i:i+le], i+le+1
		}
		if dash {
			line = strings.TrimLeft(line, "\t")
		}
		if line == word {
			return i, next
		}
		i = next
	}
	return -1, len(s)
}

// heredocWord reads a heredoc delimiter from j (leading blanks skipped) with
// the shell's quote removal (bash decodes `$'…'` there too), reporting
// whether any part was quoted, the index after the word, and odd when the word
// holds a substitution or backquote (whose end the shells read by their own
// rules, so the word and the index after it are not certain).
func heredocWord(s string, j int) (string, bool, int, bool) {
	for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		j++
	}
	var b strings.Builder
	quoted, odd := false, false
	for j < len(s) && strings.IndexByte(" \t\n;&|()<>", s[j]) < 0 {
		switch c := s[j]; {
		case c == '\\':
			quoted = true
			if j+1 < len(s) && s[j+1] != '\n' {
				b.WriteByte(s[j+1])
			}
			j += 2
		case c == '$' && j+1 < len(s) && s[j+1] == '\'':
			quoted = true
			k := ansiCEnd(s, j+2)
			b.WriteString(decodeANSIC(s[j+2 : k]))
			j = k + 1
		case c == '$' && j+1 < len(s) && s[j+1] == '"':
			j++
		case c == '\'':
			quoted = true
			k := strings.IndexByte(s[j+1:], '\'')
			if k < 0 {
				b.WriteString(s[j+1:])
				j = len(s)
				continue
			}
			b.WriteString(s[j+1 : j+1+k])
			j += k + 2
		case c == '"':
			quoted = true
			k := j + 1
			for k < len(s) && s[k] != '"' {
				if s[k] == '\\' {
					k++
				}
				k++
			}
			b.WriteString(unescapeDouble(s[j+1 : min(k, len(s))]))
			j = k + 1
		case c == '$' && j+1 < len(s) && s[j+1] == '(':
			odd = true
			k := matchParen(s, j+2)
			b.WriteString(s[j:min(k+1, len(s))])
			j = k + 1
		case c == '`':
			odd = true
			k := nextBacktick(s, j+1)
			b.WriteString(s[j:min(k+1, len(s))])
			j = k + 1
		default:
			b.WriteByte(c)
			j++
		}
	}
	return b.String(), quoted, min(j, len(s)), odd
}

// expansions lexes, as scripts, the substitutions in the body of a heredoc
// under an unquoted delimiter, from from: the shell runs them before the
// command reads the body. old reads them as bash 3.2 does; from the first
// substitution whose two ends differ, the rest of the body is also read that
// way.
func (l *forgeLexer) expansions(s string, from int, old bool) bool {
	forked := old // the bash 3.2 reading of the rest is judged once
	for j := from; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '$' && j+1 < len(s) && s[j+1] == '(':
			k, split, refuse := l.substEnd(s, j+2, old)
			if refuse {
				return true
			}
			if split && !forked {
				forked = true
				if !l.st.spend(len(s)-j+1) || l.expansions(s, j, true) {
					return true
				}
			}
			if l.substScript(s[j+2:k], false, old) {
				return true
			}
			j = k
		case s[j] == '`':
			k := nextBacktick(s, j+1)
			if l.st.script(s[j+1:k], l.depth+1) {
				return true
			}
			j = k
		}
	}
	return false
}

// double scans a double-quoted span whose opening quote is at open, lexing
// every substitution inside it as a command and marking the word dynamic on
// any expansion. It returns the closing quote's index (len(s) when
// unterminated) and whether a substitution ran the token read.
func (l *forgeLexer) double(open int) (int, bool) {
	s := l.raw
	for j := open + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case s[j] == '"':
			return j, false
		case s[j] == '$' && j+1 < len(s) && s[j+1] == '(':
			l.dyn = true
			k, split, refuse := l.substEnd(s, j+2, l.old)
			if refuse || split && l.fork(open) || l.substScript(s[j+2:k], false, l.old) {
				return j, true
			}
			j = k
		case s[j] == '`':
			l.dyn = true
			k := nextBacktick(s, j+1)
			if l.st.script(s[j+1:k], l.depth+1) {
				return j, true
			}
			j = k
		case s[j] == '$' && j+1 < len(s) && isParamStart(s[j+1]):
			l.dyn = true
		}
	}
	return len(s), false
}

// substEnd finds the `)` closing a `$(` whose text starts at from in s. It
// returns the end the scan follows, split when the other reading ends the
// substitution elsewhere (the caller then judges that reading as well), and
// refuse when the end cannot be placed. Bash 4+ and zsh read the heredocs
// inside to find it (heredocParen); bash 3.2 (old) takes the first `)` that
// closes the parentheses, as matchParen does. Without a `<<` before that
// first `)` the two readings agree. Where the bash 4+ end cannot be found (a
// delimiter line that never comes) the first `)` stands, and the text's own
// lexing refuses the open heredoc; where it is found after a `<<` the scan
// cannot place, the end is undecided.
func (l *forgeLexer) substEnd(s string, from int, old bool) (k int, split, refuse bool) {
	k = matchParen(s, from)
	if old || !strings.Contains(s[from:k], "<<") {
		return k, false, false
	}
	k2, found, sure, spent := heredocParen(s, from)
	if !l.st.spend(spent) {
		return k, false, true
	}
	switch {
	case !found || k2 == k:
		return k, false, false
	case !sure:
		return k, false, true
	}
	return k2, true, false
}

// heredocParen is matchParen that also reads heredocs, as bash 4+ and zsh
// do: at the end of a line on which a `<<` opened a heredoc, the body up to
// its delimiter line is skipped. It returns the closing `)`, found false when
// a delimiter line (or the close) never comes or a delimiter word holds a
// substitution, sure false when a heredoc was read after a `((`, `$[`, `${`
// or `name[` (where its `<<` may be a shift or text), and the bytes it read.
// Every unquoted `<<` outside a comment is read as a heredoc: one that is
// not can only make the end later than the shell's or undecided.
func heredocParen(s string, from int) (k int, found, sure bool, spent int) {
	depth, unsure := 1, false
	sure = true
	var quote byte
	var pend []forgeHeredoc
	for j := from; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			if c == quote && !(quote == '"' && oddBackslashesBefore(s, j)) {
				quote = 0
			}
		case c == '\\':
			j++
		case c == '#' && (j == from || strings.IndexByte(" \t\n;&|(", s[j-1]) >= 0):
			for j+1 < len(s) && s[j+1] != '\n' {
				j++
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
			unsure = unsure || j+1 < len(s) && s[j+1] == '('
		case c == ')':
			if depth--; depth == 0 {
				return j, true, sure, j - from + 1
			}
		case c == '$' && j+1 < len(s) && (s[j+1] == '[' || s[j+1] == '{'),
			c == '[' && j > from && isName(s[j-1:j]):
			unsure = true
		case c == '<' && j+1 < len(s) && s[j+1] == '<':
			if j+2 < len(s) && s[j+2] == '<' {
				j += 2 // a here-string
				continue
			}
			n := j + 2
			dash := n < len(s) && s[n] == '-'
			if dash {
				n++
			}
			w, quoted, end, odd := heredocWord(s, n)
			if odd {
				return 0, false, false, end - from
			}
			if w != "" || quoted {
				pend = append(pend, forgeHeredoc{word: w, dash: dash})
				sure = sure && !unsure
			}
			j = end - 1
		case c == '\n' && len(pend) > 0:
			i := j + 1
			for _, h := range pend {
				e, next := heredocEnd(s, i, h.word, h.dash)
				if e < 0 {
					return 0, false, false, len(s) - from
				}
				i = next
			}
			pend = nil
			j = i - 1
		}
	}
	return 0, false, false, len(s) - from
}

// ansiCEnd returns the index of the quote closing a `$'…'` body that starts
// at from (a backslash escapes the next byte); len(s) when unterminated.
func ansiCEnd(s string, from int) int {
	for j := from; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '\'':
			return j
		}
	}
	return len(s)
}

// decodeANSIC decodes the escapes bash decodes inside `$'…'`.
func decodeANSIC(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'e', 'E':
			b.WriteByte(27)
		case 'f':
			b.WriteByte(12)
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte(11)
		case 'c':
			if i+1 < len(s) {
				i++
				b.WriteByte(s[i] & 0x1f)
			}
		case 'x', 'u', 'U':
			width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[c]
			v, n := digitsAt(s, i+1, width, 16)
			if n == 0 {
				b.WriteByte('\\')
				b.WriteByte(c)
				continue
			}
			i += n
			if c == 'x' {
				b.WriteByte(byte(v))
			} else if v <= 0x10FFFF && (v < 0xD800 || v > 0xDFFF) {
				b.WriteRune(rune(v))
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v, n := digitsAt(s, i, 3, 8)
			i += n - 1
			b.WriteByte(byte(v))
		case '\\', '\'', '"', '?':
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}

// digitsAt reads up to max digits of base from s[i:], returning the value and
// how many it read.
func digitsAt(s string, i, max, base int) (int, int) {
	v, n := 0, 0
	for ; n < max && i+n < len(s); n++ {
		d := strings.IndexByte("0123456789abcdef", s[i+n]|0x20)
		if d < 0 || d >= base {
			break
		}
		v = v*base + d
	}
	return v, n
}

// matchParen returns the index of the `)` closing a `$(` whose body starts at
// from, depth-counted, quote-aware and comment-aware (a quote or paren in a
// comment is prose); len(s) when unterminated.
func matchParen(s string, from int) int {
	depth := 1
	var quote byte
	for j := from; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			if c == quote && !(quote == '"' && oddBackslashesBefore(s, j)) {
				quote = 0
			}
		case c == '\\':
			j++
		case c == '#' && (j == from || strings.IndexByte(" \t\n;&|(", s[j-1]) >= 0):
			for j+1 < len(s) && s[j+1] != '\n' {
				j++
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return len(s)
}

// nextBacktick returns the index of the next unescaped backtick at or after
// from; len(s) when unterminated.
func nextBacktick(s string, from int) int {
	for j := from; j < len(s); j++ {
		if s[j] == '`' && !oddBackslashesBefore(s, j) {
			return j
		}
	}
	return len(s)
}

// unescapeDouble removes the backslashes the shell removes inside double
// quotes.
func unescapeDouble(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("\"\\$`\n", s[i+1]) >= 0 {
			i++
			if s[i] == '\n' {
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// shellReservedWords precede a command without being one.
var shellReservedWords = map[string]bool{
	"!": true, "if": true, "then": true, "elif": true, "else": true,
	"while": true, "until": true, "do": true, "coproc": true,
}

// forgeCommandName normalises a command word for comparison: its path base,
// lowercased, without a Windows executable suffix.
func forgeCommandName(w string) string {
	return strings.TrimSuffix(strings.ToLower(path.Base(w)), ".exe")
}

// forgeCommand is one simple command under judgement.
type forgeCommand struct {
	words []forgeWord
	stdin []forgeWord // the piped-in previous command's arguments, if any
	lead  int
	seen  map[string]bool
	// Later candidate positions see a suffix of what an earlier one saw, so
	// the shell feeds and runner arguments are read once, from the FIRST
	// position that names a shell or such a runner: what any later one would
	// read is already covered, and the scan stays linear.
	shellFed bool
	argsFrom int // runner arguments from this index on are lexed (0: none)
}

// args returns a copy of the arguments after the command word.
func (c *forgeCommand) args() []forgeWord {
	if c.lead+1 >= len(c.words) {
		return nil
	}
	return append([]forgeWord(nil), c.words[c.lead+1:]...)
}

// lex lexes s as a script once per simple command: several candidate
// positions in one command can name the same script word.
func (c *forgeCommand) lex(st *forgeState, s string, depth int) bool {
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[s] {
		return !st.spend(len(s) + 1) // a re-read is charged; it is not lexed
	}
	c.seen[s] = true
	return st.script(s, depth+1)
}

// judge judges one simple command's words. The command word is the first
// after the lead; behind a wrapper, a runner or a computed command word each
// later word is judged ONCE as the wrapped command (judgeAt never re-enters
// this scan), so the cost is linear in the words.
func (st *forgeState) judge(c *forgeCommand, depth int) bool {
	w := c.words
	i := 0
lead:
	for i < len(w) {
		switch {
		case isRedirection(w[i].s):
			i += 2 // `>out gh auth token` still runs gh
		case envAssignRe.MatchString(w[i].s) || shellReservedWords[w[i].s]:
			i++
		default:
			break lead
		}
	}
	c.lead = i
	if i >= len(w) {
		return false
	}
	if st.judgeAt(c, i, depth) {
		return true
	}
	if name := forgeCommandName(w[i].s); w[i].dyn || commandWrappers[name] || forgeRunners[name] {
		for j := i + 1; j < len(w); j++ {
			if st.judgeAt(c, j, depth) {
				return true
			}
		}
	}
	return false
}

// judgeAt judges c's words from index i as one command.
func (st *forgeState) judgeAt(c *forgeCommand, i, depth int) bool {
	if !st.spend(1 + len(c.words[i].s)) {
		return true // past the budget: undecided, refused
	}
	st.judged++
	name, args := forgeCommandName(c.words[i].s), c.words[i+1:]
	switch {
	case c.words[i].dyn:
		return st.ghTokenRead(args, true)
	case name == "gh":
		return st.ghTokenRead(args, false)
	case name == "eval":
		return c.lex(st, joinForgeWords(args), depth)
	case forgeShell(name):
		if c.shellFed {
			return false
		}
		c.shellFed = true
		// Every script the shell can be handed: a -c / --command script, a
		// here-string anywhere in the command, and piped-in text. All are
		// lexed even when a -c script is present (fail closed: the piped text
		// may be read by that script).
		scripts, ok := st.shellScripts(args)
		if !ok {
			return true
		}
		for _, s := range scripts {
			if c.lex(st, s, depth) {
				return true
			}
		}
		for k := 0; k+1 < len(c.words); k++ {
			if !st.spend(1) || strings.HasSuffix(c.words[k].s, "<<<") && c.lex(st, c.words[k+1].s, depth) {
				return true
			}
		}
		return st.stdinScript(c, depth)
	case name == "env":
		// env's own options end at its first other word (the command, or a
		// NAME=value); -S / --split-string hands a script to the command line.
		for k := 0; k < len(args); k++ {
			if !st.spend(1) {
				return true
			}
			a := args[k].s
			var s string
			switch {
			case a == "-S" || a == "--split-string":
				if k+1 < len(args) {
					s = args[k+1].s
				}
			case strings.HasPrefix(a, "--split-string="):
				s = a[len("--split-string="):]
			case strings.HasPrefix(a, "-S"):
				s = a[2:]
			case a == "-u" || a == "-C" || a == "--unset" || a == "--chdir":
				k++
				continue
			case !strings.HasPrefix(a, "-"):
				return false
			}
			if s != "" && c.lex(st, s, depth) {
				return true
			}
		}
	case forgeArgScripts[name]:
		end := len(c.words)
		if c.argsFrom > 0 {
			end = c.argsFrom
		}
		for k := i + 1; k < end; k++ {
			if !st.spend(1) || c.lex(st, c.words[k].s, depth) {
				return true
			}
		}
		if c.argsFrom == 0 || i+1 < c.argsFrom {
			c.argsFrom = i + 1
		}
	}
	return false
}

// stdinScript lexes the text a pipe feeds a shell: the previous command's
// arguments, joined and one by one, with `\n` read as a newline (echo -e /
// printf), and with its leading options dropped.
func (st *forgeState) stdinScript(c *forgeCommand, depth int) bool {
	if len(c.stdin) == 0 {
		return false
	}
	nl := func(s string) string { return strings.ReplaceAll(s, `\n`, "\n") }
	k := 0
	for k < len(c.stdin) && strings.HasPrefix(c.stdin[k].s, "-") {
		k++
	}
	cands := []string{joinForgeWords(c.stdin), joinForgeWords(c.stdin[k:])}
	for _, a := range c.stdin {
		cands = append(cands, a.s)
	}
	for _, s := range cands {
		if c.lex(st, nl(s), depth) {
			return true
		}
	}
	return false
}

func joinForgeWords(w []forgeWord) string {
	s := make([]string, len(w))
	for k, x := range w {
		s[k] = x.s
	}
	return strings.Join(s, " ")
}

// shellScripts returns the script arguments of a shell invoked with -c (alone
// or in a short-flag cluster: -c, -lc, -ec; -Command), --command or
// --command=…: for each such flag, the next word that is not an option. Every
// candidate flag is tried, so an earlier option spelled with a `c` cannot hide
// the real one. One backward pass, each word charged to the budget; false
// when the budget ran out (the caller refuses).
func (st *forgeState) shellScripts(args []forgeWord) ([]string, bool) {
	var out []string
	next := "" // the nearest later word that is not an option
	have := false
	for k := len(args) - 1; k >= 0; k-- {
		a := args[k].s
		if !st.spend(1 + len(a)) {
			return nil, false
		}
		la := strings.ToLower(a)
		switch {
		case strings.HasPrefix(la, "--command="):
			out = append(out, a[len("--command="):])
		case la == "--command" || (strings.HasPrefix(la, "-") && !strings.HasPrefix(la, "--") && strings.ContainsRune(la[1:], 'c')):
			if have {
				out = append(out, next)
			}
		}
		if !strings.HasPrefix(a, "-") {
			next, have = a, true
		}
	}
	return out, true
}

// isRedirection reports whether a lexed word is a redirection operator (the
// lexer emits each as a word of its own, fd number included).
func isRedirection(w string) bool {
	return strings.ContainsAny(w, "<>") && strings.Trim(w, "0123456789<>&") == ""
}

// ghValueFlags take a value as the next word once gh's subcommand is known.
var ghValueFlags = map[string]bool{
	"-u": true, "--user": true, "-h": true, "--hostname": true, "-s": true, "--scopes": true,
	"-p": true, "--git-protocol": true, "-r": true, "--remove-scopes": true,
}

// isShowToken reports whether a gh flag asks `auth status` to print the
// token: --show-token, any true spelling pflag accepts after `=`, or a short
// cluster with t.
func isShowToken(a string) bool {
	la := strings.ToLower(a)
	if la == "--show-token" {
		return true
	}
	if v, ok := strings.CutPrefix(la, "--show-token="); ok {
		return v == "1" || v == "t" || v == "true"
	}
	return strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 't')
}

// ghTokenRead judges a gh invocation's arguments; nameDyn says the command
// word was computed at run time and may be gh. At most one of the command
// word, the first positional and the subcommand may be dynamic.
func (st *forgeState) ghTokenRead(args []forgeWord, nameDyn bool) bool {
	var pos []forgeWord
	show := false
	for k := 0; k < len(args); k++ {
		a := args[k]
		if !st.spend(1 + len(a.s)) {
			return true // past the budget: undecided, refused
		}
		switch {
		case isRedirection(a.s):
			k++ // a redirection operator and its target are not arguments
		case strings.HasPrefix(a.s, "-"):
			show = show || isShowToken(a.s)
			if len(pos) >= 2 && ghValueFlags[a.s] {
				k++ // `gh auth switch -u <value>`: the value is not a positional
			}
		default:
			pos = append(pos, a)
			if len(pos) == 1 && (a.dyn && nameDyn ||
				!a.dyn && !strings.EqualFold(a.s, "auth") && !strings.EqualFold(a.s, "config")) {
				return false // nothing further can match
			}
		}
	}
	if len(pos) < 2 {
		return false
	}
	dyn := 0
	if nameDyn {
		dyn++
	}
	if pos[0].dyn {
		dyn++
	}
	if dyn > 1 {
		return false
	}
	if pos[0].dyn || strings.EqualFold(pos[0].s, "auth") {
		for _, p := range pos[1:] {
			if !p.dyn && strings.EqualFold(p.s, "token") {
				return true
			}
		}
		if pos[1].dyn {
			return dyn == 0
		}
		switch strings.ToLower(pos[1].s) {
		case "status":
			return show
		case "git-credential":
			return true
		}
		return false
	}
	if len(pos) >= 3 && strings.EqualFold(pos[1].s, "get") {
		for _, p := range pos[2:] {
			if strings.EqualFold(p.s, "oauth_token") {
				return true
			}
		}
	}
	return false
}
