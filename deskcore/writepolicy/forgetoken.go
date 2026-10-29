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
//     shell reserved words; after a command wrapper or runner (env, sudo,
//     timeout, doas, flock, …) or a command word computed at run time, every
//     later word is tried once as the wrapped command, so a wrapper's own
//     option values cannot hide it, and the scan stays linear in the words;
//   - the script of a shell's -c (or --command), of `eval`, of `env -S`, of a
//     runner that hands its argument to a shell (watch, su -c, …), a
//     here-string fed to a shell, and the text a pipe feeds a shell from the
//     command before it; each is lexed again as a script of its own;
//   - a `$( … )` or backtick substitution, quoted or not, lexed as a script;
//   - the body of a heredoc whose introducing line names a shell anywhere
//     (piped, wrapped or after an assignment), which stays visible as lines of
//     commands.
//
// A word is assembled from the RAW command at the masked offsets, so quoting
// it (`gh "auth" 'token'`, ANSI-C `$'…'` with its escapes decoded, locale
// `$"…"`), escaping it (`\gh`) or splitting it with a line continuation does
// not hide it: the shell removes all of those before gh sees its arguments.
// A word with a part computed at run time (parameter expansion, a
// substitution, brace expansion, a glob) is DYNAMIC: it may stand for any one
// of the command word, `auth` or the subcommand, so one such component with
// the rest literal is judged a read; two or more are not judged.
//
// Inert data follows the existing masking convention: a comment, a heredoc
// body written to a non-shell under a quoted delimiter, and a quoted argument
// to a non-shell command are prose.
//
// It fails CLOSED where it cannot finish (a desk-decided departure from
// allow-and-log, recorded on the PR that added this): an unterminated quote
// is lexed as a command, and nesting deeper than maxForgeTokenDepth or a scan
// that runs out of its work budget is judged by whether the text names gh,
// auth and token at all. The budget is linear in the command's length, so the
// hook always reaches a decision.
func forgeTokenRead(raw string) bool {
	hit, _ := forgeTokenReadStats(raw)
	return hit
}

// forgeStats is the work a classification did, for tests that pin its cost.
type forgeStats struct {
	judged    int  // command positions judged
	spent     int  // work units charged
	exhausted bool // the budget ran out and the fail-closed rule decided
}

func forgeTokenReadStats(raw string) (bool, forgeStats) {
	budget := forgeBudgetPerByte*len(raw) + forgeBudgetBase
	st := &forgeState{budget: budget}
	hit := st.script(raw, 0)
	if !hit && st.exhausted {
		hit = forgeWordsPresent(raw)
	}
	return hit, forgeStats{judged: st.judged, spent: budget - st.budget, exhausted: st.exhausted}
}

const (
	// maxForgeTokenDepth bounds script-in-script recursion.
	maxForgeTokenDepth = 8
	// The work budget: units per input byte, plus a floor. One unit is one
	// byte lexed, one command position judged or one argument examined.
	forgeBudgetPerByte = 32
	forgeBudgetBase    = 4096
)

// forgeWordsPresent is the fail-closed rule for text the lexer could not
// finish: it names gh, auth and a token or credential at all.
func forgeWordsPresent(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "gh") && strings.Contains(l, "auth") &&
		(strings.Contains(l, "token") || strings.Contains(l, "credential"))
}

type forgeState struct {
	budget    int
	judged    int
	exhausted bool
}

// spend charges n work units; false once the budget is gone.
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
// `sh -c`, `eval`, here-string, pipe or substitution executes, masked fresh.
func (st *forgeState) script(cmd string, depth int) bool {
	if depth > maxForgeTokenDepth {
		// Nesting too deep to lex is not evidence of absence: fail closed.
		return forgeWordsPresent(cmd)
	}
	if !st.spend(len(cmd) + 1) {
		return false
	}
	l := forgeLexer{st: st, m: forgeMask(cmd), raw: cmd, depth: depth}
	return l.scan()
}

// forgeMask is maskLiterals with the forge-local heredoc rule: a body stays
// visible when ANY word of its introducing line names a shell (piped into
// one, behind a wrapper, after an assignment), not only the first.
func forgeMask(cmd string) string {
	m := maskQuotedKeepShellArg(maskHeredocBodiesFed(cmd, forgeShellFed))
	if len(m) != len(cmd) {
		return cmd
	}
	return m
}

func forgeShellFed(before, after string) bool {
	for _, f := range strings.FieldsFunc(before+" "+after, func(r rune) bool {
		return strings.ContainsRune(" \t;&|()", r)
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

type forgeLexer struct {
	st     *forgeState
	m, raw string
	depth  int
	words  []forgeWord
	cur    strings.Builder
	inWord bool
	dyn    bool // the current word has a part computed at run time
	brace  bool // the current word has an unquoted `{`
	glob   bool // the current word has an unquoted `[`
	stdin  []forgeWord
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
	raw := l.raw
	for i := 0; i < len(l.m); i++ {
		m := l.m
		c := m[i]
		switch {
		case c == '\\':
			if i+1 < len(m) {
				i++
				if m[i] != '\n' { // a line continuation joins the two words
					l.word(raw[i : i+1])
				}
			}
		case c == '$' && i+1 < len(m) && m[i+1] == '\'':
			// ANSI-C quoting: the shell decodes the escapes, and `\'` does
			// not end the span (the shared masking, which ends it there, is
			// redone from the real end).
			j := ansiCEnd(raw, i+2)
			body := raw[i+2 : j]
			l.word(decodeANSIC(body))
			if j >= len(raw) {
				if l.st.script(body, l.depth+1) {
					return true
				}
				i = len(m)
				continue
			}
			if strings.Contains(body, `\'`) {
				if rest := forgeMask(raw[j+1:]); len(rest) == len(raw)-j-1 {
					l.m = m[:j+1] + rest
				}
			}
			i = j
		case c == '$' && i+1 < len(m) && m[i+1] == '"':
			// Locale quoting: the `$` is dropped and the span is double-quoted.
		case c == '\'':
			j := strings.IndexByte(m[i+1:], '\'')
			if j < 0 {
				if l.st.script(raw[i+1:], l.depth+1) {
					return true
				}
				l.word(raw[i+1:])
				i = len(m)
				continue
			}
			l.word(raw[i+1 : i+1+j])
			i += 1 + j
		case c == '"':
			j, hit := l.double(i + 1)
			if hit {
				return true
			}
			if j >= len(m) {
				if l.st.script(raw[i+1:], l.depth+1) {
					return true
				}
			}
			l.word(unescapeDouble(raw[i+1 : min(j, len(raw))]))
			i = j
		case c == '$' && i+1 < len(m) && m[i+1] == '(':
			k := matchParen(m, i+2)
			if l.st.script(raw[i+2:k], l.depth+1) {
				return true
			}
			l.word("$()")
			l.dyn = true
			i = k
		case c == '`':
			k := nextBacktick(m, i+1)
			if l.st.script(raw[i+1:k], l.depth+1) {
				return true
			}
			l.word("``")
			l.dyn = true
			i = k
		case c == '$' && i+1 < len(m) && isParamStart(m[i+1]):
			l.word("$")
			l.dyn = true
		case c == '#' && !l.inWord:
			for i+1 < len(m) && m[i+1] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\r':
			l.flush()
		case c == '<' || c == '>' || (c == '&' && i+1 < len(m) && m[i+1] == '>'):
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
			for j < len(m) && (m[j] == '<' || m[j] == '>' || m[j] == '&') {
				j++
			}
			l.words = append(l.words, forgeWord{s: fd + m[i:j]})
			i = j - 1
		case c == '|':
			pipe := true
			if i+1 < len(m) && m[i+1] == '|' {
				pipe = false
				i++
			} else if i+1 < len(m) && m[i+1] == '&' {
				i++
			}
			if l.end(pipe) {
				return true
			}
		case c == '\n' || c == ';' || c == '&' || c == '(' || c == ')':
			if l.end(false) {
				return true
			}
		case c == '{' && !l.inWord && (i+1 >= len(m) || strings.IndexByte(" \t\r\n", m[i+1]) >= 0),
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
			l.word("[")
			l.glob = true
		case c == ']':
			l.word("]")
			l.dyn = l.dyn || l.glob
		case c == '*' || c == '?':
			l.word(raw[i : i+1])
			l.dyn = true
		default:
			l.word(raw[i : i+1])
		}
	}
	return l.end(false)
}

// double scans a double-quoted span opening before `from`, lexing every
// substitution inside it as a command and marking the word dynamic on any
// expansion. It returns the closing quote's index (len(m) when unterminated)
// and whether a substitution ran the token read.
func (l *forgeLexer) double(from int) (int, bool) {
	m, raw := l.m, l.raw
	for j := from; j < len(m); j++ {
		switch {
		case m[j] == '\\':
			j++
		case m[j] == '"':
			return j, false
		case m[j] == '$' && j+1 < len(m) && m[j+1] == '(':
			l.dyn = true
			k := matchParen(m, j+2)
			if l.st.script(raw[j+2:k], l.depth+1) {
				return j, true
			}
			j = k
		case m[j] == '`':
			l.dyn = true
			k := nextBacktick(m, j+1)
			if l.st.script(raw[j+1:k], l.depth+1) {
				return j, true
			}
			j = k
		case raw[j] == '$' && j+1 < len(raw) && isParamStart(raw[j+1]) && !oddBackslashesBefore(raw, j):
			l.dyn = true // the masking blanks a quoted `$name`; the raw text has it
		}
	}
	return len(m), false
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
// from, depth-counted and quote-aware; len(s) when unterminated.
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
		return false
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
		for j := i + 1; j < len(w) && !st.exhausted; j++ {
			if st.judgeAt(c, j, depth) {
				return true
			}
		}
	}
	return false
}

// judgeAt judges c's words from index i as one command.
func (st *forgeState) judgeAt(c *forgeCommand, i, depth int) bool {
	if !st.spend(1) {
		return false
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
		for _, s := range st.shellScripts(args) {
			if c.lex(st, s, depth) {
				return true
			}
		}
		for k := 0; k+1 < len(c.words) && st.spend(1); k++ {
			if strings.HasSuffix(c.words[k].s, "<<<") && c.lex(st, c.words[k+1].s, depth) {
				return true
			}
		}
		return st.stdinScript(c, depth)
	case name == "env":
		// env's own options end at its first other word (the command, or a
		// NAME=value); -S / --split-string hands a script to the command line.
		for k := 0; k < len(args) && st.spend(1); k++ {
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
			if c.lex(st, c.words[k].s, depth) {
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
// the real one. One backward pass keeps it linear.
func (st *forgeState) shellScripts(args []forgeWord) []string {
	var out []string
	next := "" // the nearest later word that is not an option
	have := false
	for k := len(args) - 1; k >= 0 && st.spend(1); k-- {
		a := args[k].s
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
	return out
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
		if !st.spend(1) {
			return false
		}
		a := args[k]
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
