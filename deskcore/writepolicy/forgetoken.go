package writepolicy

import (
	"path"
	"strings"
)

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command may RUN the gh CLI's token
// read. It is a lexical backstop, not a shell. Its verdict is two-layered:
// the scan models a small set of shell constructs exactly and judges the
// commands they spell; at the first construct it does not model, ONE decision
// point (unmodeled) decides the rest of that script by a conservative word
// trigger. A false refuse there is accepted; a false allow is not.
//
// The read is a gh invocation whose first positional is `auth` and a later
// positional `token` (case-insensitive: the match fails closed on case),
// `gh auth status` asked to print the token (--show-token, any true spelling
// of it, or a short cluster with -t), `gh auth git-credential`, or `gh config
// get … oauth_token`. A gh flag's VALUE (-u, -h, …) after the subcommand is
// not a positional.
//
// Modeled, so judged command by command: words and blanks; `\` escapes and
// line continuations; `'…'`; `$'…'` with its escapes decoded, when it is
// closed and holds no `\'` (where a POSIX sh ends it elsewhere); `$"…"`;
// `"…"` holding only text, escapes, `$name` and plain expansions; `$name`;
// a plain `${…}` (no quote, backslash, backquote, `$`, brace, paren or
// newline inside); a plain `$( … )` (plainSubst: ended by the first `)` in
// every shell, its text lexed as a script); comments; `;`, `&`, `|`, `&&`,
// `||`, newlines, subshell parentheses and group braces; redirections and
// here-strings; and a heredoc whose delimiter word holds no substitution and
// whose delimiter line comes, with, under an unquoted delimiter, a body
// holding no line continuation and only plain expansions.
//
// What counts as executed, among the modeled commands:
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
//   - the text of a plain `$( … )`, lexed as a script;
//   - the body of a heredoc whose introducing command line names a shell
//     anywhere (piped, wrapped or after an assignment), lexed as a script; the
//     plain substitutions in the body of a heredoc under an unquoted
//     delimiter.
//
// Quoting a word (`gh "auth" 'token'`, `$'…'`, `$"…"`), escaping it (`\gh`)
// or splitting it with a line continuation does not hide it: the shell
// removes all of those before gh sees its arguments. A word with a part
// computed at run time (parameter expansion, a substitution, brace expansion,
// a glob) is DYNAMIC: it may stand for any one of the command word, `auth` or
// the subcommand, so one such component with the rest literal is judged a
// read; two or more are not judged.
//
// Inert data: a comment, a heredoc body written to a non-shell under a quoted
// delimiter, and a quoted argument to a non-shell command are prose.
//
// Not modeled, so decided by unmodeled: any other `$( … )` (one holding a
// paren, a backslash, a backquote, a `#`, a `<<`, `case`, a bracket, a brace
// other than a plain `${…}`, an unquoted `$` other than `$name`, or a quote
// holding any of those), a backquote, `$((…))`, `((…))`, `$[…]`, a
// non-plain `${…}`, a `$name[` subscript inside double quotes, a `[` subscript
// holding blanks or shell syntax, `<(…)` and `>(…)`, a `(` written against a
// word (a function, an array, a zsh glob qualifier, an extglob), any quote
// left open, a `$'…'` holding `\'`, a heredoc delimiter holding a
// substitution, a heredoc whose delimiter line never comes, and an unquoted
// heredoc body holding a line continuation or a non-plain expansion. No
// function in the scan places the end of any of these: the scan stops there.
//
// unmodeled refuses iff the trigger appears anywhere from the start of the
// pipeline holding the construct to the end of the script; otherwise nothing
// there can be the read and the script is allowed. The trigger
// (forgeTrigger) is a whole word `auth`, `token`, `oauth_token` or
// `git-credential`, or the words `status` and `gh` both, in any spelling the
// shells reduce to that word: quotes, `$` before a quote, backslashes, line
// continuations and the escapes `$'…'`, `echo -e` and printf decode are
// looked through. Every read the judge refuses holds one of them literally,
// so the trigger misses no read the judge would have refused.
//
// Cost: every pass over a script's bytes is charged to one work budget
// (forgeBudgetPerByte per input byte plus forgeBudgetBase): a script is
// charged its length before it is lexed, the lexer walks each of its bytes a
// bounded number of times, the trigger is charged once per word it looks
// for, and every per-word step (a judged position, an examined argument, a
// re-read of an already-lexed script) is charged the length of the word it
// reads. So the scan's own work is at most a constant times the budget, which
// is linear in the command's length. This bounds the forge scan only: the
// shared masking Classify runs before it is a separate cost (#4854).
//
// It fails CLOSED where it cannot finish (a desk-decided departure from
// allow-and-log, recorded on the PR that added this): nesting deeper than
// maxForgeTokenDepth or a scan that runs out of its work budget is judged a
// read outright, whatever the text says.
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
	if depth > maxForgeTokenDepth {
		// Nesting too deep to lex is not evidence of absence, and the text
		// cannot be judged by its spelling either: refuse.
		return true
	}
	if !st.spend(len(cmd) + 1) {
		return true
	}
	l := forgeLexer{st: st, raw: cmd, depth: depth}
	return l.scan()
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
	dash   bool // `<<-`: leading tabs are removed from each body line
	from   int  // where the pipeline holding the `<<` began
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
	// cmdStart is where the current pipeline began: the start of what
	// unmodeled decides. stop: unmodeled has decided the rest of the script.
	cmdStart int
	stop     bool
}

// unmodeled is the ONE decision point for every construct the scan does not
// model (see forgeTokenRead). The scan does not trust its position after
// such a construct, so it stops; the rest of the script, from from (the start
// of the pipeline holding the construct), is a read iff it holds the trigger.
// The commands before from were already judged.
func (l *forgeLexer) unmodeled(from int) bool {
	l.stop = true
	rest := l.raw[from:]
	if !l.st.spend(len(forgeTriggerWords)*(len(rest)+1) + 2) {
		return true
	}
	return forgeTrigger(rest)
}

func (l *forgeLexer) flush() {
	if l.inWord {
		l.words = append(l.words, forgeWord{l.cur.String(), l.dyn})
		l.cur.Reset()
	}
	l.inWord, l.dyn, l.brace, l.glob = false, false, false, false
}

// end closes the current simple command and judges it. A pipe hands the
// command's arguments to the next one as the text on its stdin; any other end
// starts a new pipeline after next.
func (l *forgeLexer) end(pipe bool, next int) bool {
	l.flush()
	c := forgeCommand{words: l.words, stdin: l.stdin}
	hit := l.st.judge(&c, l.depth)
	l.stdin = nil
	if pipe {
		l.stdin = c.args()
	} else {
		l.cmdStart = next
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
	for i := 0; i < len(s); i++ {
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
			// ANSI-C quoting: bash and zsh decode the escapes, and `\'` does
			// not end the span there, but does in a POSIX sh.
			j := ansiCEnd(s, i+2)
			if j >= len(s) || strings.Contains(s[i+2:j], `\'`) {
				return l.unmodeled(l.cmdStart)
			}
			l.word(decodeANSIC(s[i+2 : j]))
			i = j
		case c == '$' && i+1 < len(s) && s[i+1] == '"':
			// Locale quoting: the `$` is dropped and the span is double-quoted.
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return l.unmodeled(l.cmdStart)
			}
			l.word(s[i+1 : i+1+j])
			i += 1 + j
		case c == '"':
			j, dyn, ok, hit := l.expansions(s, i+1, '"')
			if hit || !ok {
				return hit || l.unmodeled(l.cmdStart)
			}
			l.dyn = l.dyn || dyn
			l.word(unescapeDouble(s[i+1 : j]))
			i = j
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			k, ok := plainSubst(s, i+2)
			if !ok {
				return l.unmodeled(l.cmdStart)
			}
			if l.st.script(s[i+2:k], l.depth+1) {
				return true
			}
			l.word("$()")
			l.dyn = true
			i = k
		case c == '`', c == '$' && i+1 < len(s) && s[i+1] == '[':
			return l.unmodeled(l.cmdStart)
		case c == '$' && i+1 < len(s) && s[i+1] == '{':
			k, ok := plainBrace(s, i+2)
			if !ok {
				return l.unmodeled(l.cmdStart)
			}
			l.word("$")
			l.dyn = true
			i = k
		case c == '$' && i+1 < len(s) && isParamStart(s[i+1]):
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
			if j < len(s) && s[j] == '(' {
				return l.unmodeled(l.cmdStart) // process substitution
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
					return l.unmodeled(l.cmdStart)
				}
				if w != "" || quoted {
					l.words = append(l.words, forgeWord{s: w})
					l.pending = append(l.pending, forgeHeredoc{w, quoted, dash, l.cmdStart})
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
			if l.end(pipe, i+1) {
				return true
			}
		case c == '\n':
			if l.end(false, i+1) {
				return true
			}
			if len(l.pending) > 0 {
				next, hit := l.heredocs(s[l.lineStart:i], i+1)
				if hit || l.stop {
					return hit
				}
				i = next - 1
				l.cmdStart = next
			}
			l.lineStart = i + 1
		case c == '(' && (l.inWord || i+1 < len(s) && s[i+1] == '('):
			// `((` (arithmetic or nested subshells: bash decides by where the
			// parens close) or a `(` written against a word.
			return l.unmodeled(l.cmdStart)
		case c == ';' || c == '&' || c == '(' || c == ')',
			c == '{' && !l.inWord && (i+1 >= len(s) || strings.IndexByte(" \t\r\n", s[i+1]) >= 0),
			c == '}' && !l.inWord:
			// A group brace; `{a,b}` and `x{a,b}` are brace expansion.
			if l.end(false, i+1) {
				return true
			}
		case c == '{':
			l.word("{")
			l.brace = true
		case c == '}':
			l.word("}")
			l.dyn = l.dyn || l.brace
		case c == '[':
			if cur := l.cur.String(); l.inWord && isNameByte(cur[len(cur)-1]) {
				// `name[…]`: bash may read a subscript as one word and zsh a
				// `$name[…]` as one; plain bracket text reads the same as a glob.
				k := strings.IndexByte(s[i+1:], ']')
				if k < 0 || strings.ContainsAny(s[i+1:i+1+k], " \t\r\n'\"`$\\(){}<>;&|#") {
					return l.unmodeled(l.cmdStart)
				}
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
	return l.end(false, len(s))
}

// heredocs reads, in order, the bodies of the heredocs pending on the command
// line that just ended, the first starting at start. A body fed to a shell is
// lexed as a script; under an unquoted delimiter its plain substitutions are;
// under a quoted one it is data. It returns the index after the last body.
// A body whose delimiter line never comes, and one under an unquoted
// delimiter that holds a line continuation (the shells join the lines before
// they compare the delimiter) or a non-plain expansion, is unmodeled.
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
			return len(s), l.unmodeled(h.from)
		}
		body := s[i:end]
		if !h.quoted {
			if strings.Contains(body, "\\\n") {
				return len(s), l.unmodeled(h.from)
			}
			if _, _, ok, hit := l.expansions(body, 0, 0); hit || !ok {
				return len(s), hit || l.unmodeled(h.from)
			}
		}
		if fed && l.st.script(body, l.depth+1) {
			return len(s), true
		}
		i = next
	}
	return i, false
}

// expansions reads the expansions in s from from: in a double-quoted span when
// term is `"` (returning the closing quote's index), or in a heredoc body
// under an unquoted delimiter when term is 0. The text of each plain `$( … )`
// is lexed as a script (hit when it runs the read); dyn reports any
// expansion. ok is false at the first construct the scan does not model
// there: a non-plain substitution or `${…}`, a backquote, `$[`, a zsh
// `$name[` subscript, or a span left open.
func (l *forgeLexer) expansions(s string, from int, term byte) (end int, dyn, ok, hit bool) {
	for j := from; j < len(s); j++ {
		switch {
		case s[j] == '\\':
			j++
		case term != 0 && s[j] == term:
			return j, dyn, true, false
		case s[j] == '$' && j+1 < len(s) && s[j+1] == '(':
			k, plain := plainSubst(s, j+2)
			if !plain {
				return j, dyn, false, false
			}
			if l.st.script(s[j+2:k], l.depth+1) {
				return j, dyn, true, true
			}
			dyn, j = true, k
		case s[j] == '`', s[j] == '$' && j+1 < len(s) && s[j+1] == '[':
			return j, dyn, false, false
		case s[j] == '$' && j+1 < len(s) && s[j+1] == '{':
			k, plain := plainBrace(s, j+2)
			if !plain {
				return j, dyn, false, false
			}
			dyn, j = true, k
		case s[j] == '$' && j+1 < len(s) && isParamStart(s[j+1]):
			dyn = true
			k := j + 1
			for k < len(s) && isNameByte(s[k]) {
				k++
			}
			if k < len(s) && s[k] == '[' {
				return j, dyn, false, false
			}
		}
	}
	return len(s), dyn, term == 0, false
}

func isNameByte(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}

// plainSubst returns the `)` ending a `$(` whose text starts at from, and
// whether the substitution is plain: its text holds no paren at all, and no
// construct that a shell could read as going on past that first `)` or as
// opening another context (a backslash, a backquote, a `#`, a `<<`, `case`, a
// bracket or brace other than a plain `${…}`, a `$` before a paren, quote or
// bracket, an unclosed quote, a quote holding the other quote, a paren, a
// bracket or a backquote, or a `$` in a double quote other than `$name`). Then every shell
// (bash 3.2 and 4+, zsh, a POSIX sh) ends it at that `)`. Anything else is
// not plain, and the scan does not place its end.
func plainSubst(s string, from int) (int, bool) {
	var quote byte
	for j := from; j < len(s); j++ {
		c := s[j]
		if quote != 0 {
			switch {
			case c == quote:
				quote = 0
			case strings.IndexByte("'\"`()[]", c) >= 0,
				quote == '"' && (c == '\\' || c == '$' && !(j+1 < len(s) && isNameByte(s[j+1]))):
				return 0, false
			}
			continue
		}
		switch {
		case c == ')':
			return j, !strings.Contains(s[from:j], "case")
		case c == '\'' || c == '"':
			quote = c
		case c == '$' && j+1 < len(s) && s[j+1] == '{':
			k, ok := plainBrace(s, j+2)
			if !ok {
				return 0, false
			}
			j = k
		case c == '$' && j+1 < len(s) && strings.IndexByte("('\"[", s[j+1]) >= 0,
			strings.IndexByte("(\\`#{}[]", c) >= 0,
			c == '<' && j+1 < len(s) && s[j+1] == '<':
			return 0, false
		}
	}
	return 0, false
}

// plainBrace returns the `}` ending a `${` whose text starts at from, and
// whether that text is plain: no quote, backslash, backquote, `$`, brace,
// paren or newline, so every shell ends it at that first `}`.
func plainBrace(s string, from int) (int, bool) {
	for j := from; j < len(s); j++ {
		switch c := s[j]; {
		case c == '}':
			return j, true
		case strings.IndexByte("'\"`\\${()\n", c) >= 0:
			return 0, false
		}
	}
	return 0, false
}

// forgeTriggerWords: every read the judge refuses holds one of these as a
// literal word, or holds `status` with a literal gh command word (`gh <dynamic>
// status -t`). See ghTokenRead: a dynamic first positional leaves `token`,
// `git-credential` or `status`; a dynamic command word leaves `auth` or
// `oauth_token`; with neither dynamic, `auth` or `oauth_token` is literal.
var forgeTriggerWords = []string{"auth", "token", "oauth_token", "git-credential", "status", "gh"}

// forgeTrigger reports whether s may spell one of the trigger words as a
// word the shell would hand gh (forgeHasWord). It over-approximates: prose
// that mentions a trigger word triggers.
func forgeTrigger(s string) bool {
	for _, w := range forgeTriggerWords[:4] {
		if forgeHasWord(s, w, false) {
			return true
		}
	}
	return forgeHasWord(s, "status", false) && forgeHasWord(s, "gh", true)
}

// forgeDelim reports whether b may end a shell word (blanks and operators),
// or split one by brace expansion.
func forgeDelim(b byte) bool { return strings.IndexByte(" \t\r\n;&|()<>`{},", b) >= 0 }

// forgeFiller is how many bytes at p the shell may remove from a word
// without a trace: a quote, a `$` before one, a backslash, or a backslash and
// newline (a line continuation); 0 for any other byte.
func forgeFiller(s string, p int) int {
	switch {
	case s[p] == '\'' || s[p] == '"':
		return 1
	case s[p] == '$' && p+1 < len(s) && (s[p+1] == '\'' || s[p+1] == '"'):
		return 1
	case s[p] == '\\' && p+1 < len(s) && s[p+1] == '\n':
		return 2
	case s[p] == '\\':
		return 1
	}
	return 0
}

// forgeEscapes lists, for a backslash at p, each escape the shells or
// echo -e / printf may decode there (`\xHH`, `\uHHHH`, `\UHHHHHHHH`, `\NNN`,
// `\0NNN`, with fewer digits too) as its length and the byte it stands for.
func forgeEscapes(s string, p int, f func(n int, v int)) {
	try := func(skip, max, base int) {
		v := 0
		for n := 1; n <= max && p+skip+n <= len(s); n++ {
			d := strings.IndexByte("0123456789abcdef", s[p+skip+n-1]|0x20)
			if d < 0 || d >= base {
				return
			}
			v = v*base + d
			f(skip+n, v)
		}
	}
	if p+1 >= len(s) {
		return
	}
	switch s[p+1] {
	case 'x':
		try(2, 2, 16)
	case 'u':
		try(2, 4, 16)
	case 'U':
		try(2, 8, 16)
	case '0':
		try(2, 3, 8)
		try(1, 3, 8)
	default:
		try(1, 3, 8)
	}
}

// forgeHasWord reports whether s may hold w as a whole word once the shell
// has removed quotes, backslashes and line continuations and decoded escapes
// (case-insensitively: the judge matches gh's words that way). A word starts
// at the start of s or after a forgeDelim byte and ends at one; cmd also
// allows a command path (`/x/gh`, zsh `=gh`) and a `.exe` suffix. One pass
// over s: the set of prefixes of w matched so far is kept per position.
func forgeHasWord(s, w string, cmd bool) bool {
	n, full := len(s), uint32(1)<<len(w)
	act := make([]uint32, n+1)
	for p := 0; p <= n; p++ {
		if p == 0 || forgeDelim(s[p-1]) || cmd && (s[p-1] == '/' || s[p-1] == '=') {
			act[p] |= 1
		}
		a := act[p]
		if a == 0 {
			continue
		}
		if a&full != 0 && (p == n || forgeDelim(s[p]) || cmd && strings.HasPrefix(strings.ToLower(s[p:min(p+4, n)]), ".exe")) {
			return true
		}
		if p == n {
			continue
		}
		if f := forgeFiller(s, p); f > 0 {
			act[p+f] |= a
		}
		c := s[p]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		for k := 0; k < len(w); k++ {
			if a&(1<<k) == 0 {
				continue
			}
			if c == w[k] {
				act[p+1] |= 1 << (k + 1)
			}
			if s[p] == '\\' {
				forgeEscapes(s, p, func(m, v int) {
					if 'A' <= v && v <= 'Z' {
						v += 'a' - 'A'
					}
					if v == int(w[k]) {
						act[p+m] |= 1 << (k + 1)
					}
				})
			}
		}
	}
	return false
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
// whether any part was quoted and the index after the word. odd reports a
// word the scan does not model: one holding a substitution, a backquote, an
// open quote or a `$'…'` holding `\'`.
func heredocWord(s string, j int) (string, bool, int, bool) {
	for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		j++
	}
	var b strings.Builder
	quoted := false
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
			if k >= len(s) || strings.Contains(s[j+2:k], `\'`) {
				return "", quoted, j, true
			}
			b.WriteString(decodeANSIC(s[j+2 : k]))
			j = k + 1
		case c == '$' && j+1 < len(s) && s[j+1] == '"':
			j++
		case c == '\'':
			quoted = true
			k := strings.IndexByte(s[j+1:], '\'')
			if k < 0 {
				return "", quoted, j, true
			}
			b.WriteString(s[j+1 : j+1+k])
			j += k + 2
		case c == '"':
			quoted = true
			k := j + 1
			for k < len(s) && s[k] != '"' {
				if s[k] == '`' || s[k] == '$' && k+1 < len(s) && s[k+1] == '(' {
					return "", quoted, j, true
				}
				if s[k] == '\\' {
					k++
				}
				k++
			}
			if k >= len(s) {
				return "", quoted, j, true
			}
			b.WriteString(unescapeDouble(s[j+1 : k]))
			j = k + 1
		case c == '`', c == '$' && j+1 < len(s) && s[j+1] == '(':
			return "", quoted, j, true
		default:
			b.WriteByte(c)
			j++
		}
	}
	return b.String(), quoted, min(j, len(s)), false
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
// lowercased, without a Windows executable suffix or the `=` of a zsh
// `=name` path lookup.
func forgeCommandName(w string) string {
	return strings.TrimLeft(strings.TrimSuffix(strings.ToLower(path.Base(w)), ".exe"), "=")
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
