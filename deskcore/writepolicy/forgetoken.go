package writepolicy

import (
	"path"
	"strings"
)

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command RUNS the gh CLI's token read:
// a gh invocation in command position whose first positional is `auth` and a
// later positional `token` (case-insensitive, as the match fails closed on
// case), or `gh auth status` asked to print the token (--show-token / -t).
//
// It is a pure lexical classification over the same masked text Classify
// freezes, so inert data follows the existing masking convention:
//
//   - a comment, and the body of a heredoc written to anything but a shell
//     under a QUOTED delimiter, are blanked by the masking and read nothing;
//   - a quoted span is one argument WORD, never a command — `git commit -m
//     "gh auth token"` and `echo 'gh auth token'` are prose — except where
//     the masking also treats it as executed: the script of `sh -c` / `bash
//     -c` / `eval`, and a `$( … )` or backtick substitution (quoted or not),
//     which are lexed again as commands of their own;
//   - an unterminated quote is lexed as a command (fail closed), mirroring
//     the masking's restore-on-doubt rule.
//
// Words are assembled from the RAW command at the masked offsets, so quoting
// a word (`gh "auth" 'token'`), escaping it (`\gh`) or splitting it with a
// line continuation does not hide it: the shell removes all three before gh
// sees its arguments. Leading NAME=value assignments, shell reserved words and
// the existing commandWrappers (env, command, exec, sudo, timeout, …) are
// skipped to find the command; after a wrapper every later word is tried as
// the wrapped command, so a wrapper's own option values cannot hide it.
func forgeTokenRead(masked, raw string, depth int) bool {
	if len(masked) != len(raw) {
		masked = maskLiterals(raw)
		if len(masked) != len(raw) {
			masked = raw
		}
	}
	if depth > maxForgeTokenDepth {
		// Nesting too deep to lex is not evidence of absence: fail closed on
		// the three words appearing at all.
		l := strings.ToLower(raw)
		return strings.Contains(l, "gh") && strings.Contains(l, "auth") && strings.Contains(l, "token")
	}
	l := forgeLexer{m: masked, raw: raw, depth: depth}
	return l.scan()
}

// maxForgeTokenDepth bounds substitution / `sh -c` recursion.
const maxForgeTokenDepth = 8

// forgeScript lexes cmd as a script of its own: the text a `sh -c`, `eval` or
// substitution executes is a fresh command string, masked fresh.
func forgeScript(cmd string, depth int) bool {
	return forgeTokenRead(maskLiterals(cmd), cmd, depth+1)
}

type forgeLexer struct {
	m, raw string
	depth  int
	words  []string
	cur    strings.Builder
	inWord bool
}

func (l *forgeLexer) flush() {
	if l.inWord {
		l.words = append(l.words, l.cur.String())
		l.cur.Reset()
		l.inWord = false
	}
}

// end closes the current simple command and judges it.
func (l *forgeLexer) end() bool {
	l.flush()
	hit := forgeTokenCommand(l.words, l.depth)
	l.words = l.words[:0]
	return hit
}

func (l *forgeLexer) word(s string) {
	l.cur.WriteString(s)
	l.inWord = true
}

func (l *forgeLexer) scan() bool {
	m, raw := l.m, l.raw
	for i := 0; i < len(m); i++ {
		c := m[i]
		switch {
		case c == '\\':
			if i+1 < len(m) {
				i++
				if m[i] != '\n' { // a line continuation joins the two words
					l.word(raw[i : i+1])
				}
			}
		case c == '\'':
			j := strings.IndexByte(m[i+1:], '\'')
			if j < 0 {
				if forgeScript(raw[i+1:], l.depth) {
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
				if forgeScript(raw[i+1:], l.depth) {
					return true
				}
			}
			l.word(unescapeDouble(raw[i+1 : min(j, len(raw))]))
			i = j
		case c == '$' && i+1 < len(m) && m[i+1] == '(':
			k := matchParen(m, i+2)
			if forgeScript(raw[i+2:k], l.depth) {
				return true
			}
			l.word("$()")
			i = k
		case c == '`':
			k := nextBacktick(m, i+1)
			if forgeScript(raw[i+1:k], l.depth) {
				return true
			}
			l.word("``")
			i = k
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
			l.words = append(l.words, fd+m[i:j])
			i = j - 1
		case c == '\n' || c == ';' || c == '|' || c == '&' || c == '(' || c == ')':
			if l.end() {
				return true
			}
		case (c == '{' || c == '}') && !l.inWord:
			if l.end() {
				return true
			}
		default:
			l.word(raw[i : i+1])
		}
	}
	return l.end()
}

// double scans a double-quoted span opening before `from`, lexing every
// substitution inside it as a command. It returns the closing quote's index
// (len(m) when unterminated) and whether a substitution ran the token read.
func (l *forgeLexer) double(from int) (int, bool) {
	m, raw := l.m, l.raw
	for j := from; j < len(m); j++ {
		switch {
		case m[j] == '\\':
			j++
		case m[j] == '"':
			return j, false
		case m[j] == '$' && j+1 < len(m) && m[j+1] == '(':
			k := matchParen(m, j+2)
			if forgeScript(raw[j+2:k], l.depth) {
				return j, true
			}
			j = k
		case m[j] == '`':
			k := nextBacktick(m, j+1)
			if forgeScript(raw[j+1:k], l.depth) {
				return j, true
			}
			j = k
		}
	}
	return len(m), false
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

// forgeTokenCommand judges one simple command's words.
func forgeTokenCommand(w []string, depth int) bool {
	i := 0
lead:
	for i < len(w) {
		switch {
		case isRedirection(w[i]):
			i += 2 // `>out gh auth token` still runs gh
		case envAssignRe.MatchString(w[i]) || shellReservedWords[w[i]]:
			i++
		default:
			break lead
		}
	}
	if i >= len(w) {
		return false
	}
	name, args := forgeCommandName(w[i]), w[i+1:]
	switch {
	case name == "gh":
		return ghTokenRead(args)
	case name == "eval":
		return forgeScript(strings.Join(args, " "), depth)
	case shellNameRe.MatchString(name):
		if script, ok := shellCScript(args); ok {
			return forgeScript(script, depth)
		}
	case commandWrappers[name]:
		for j := range args {
			if forgeTokenCommand(args[j:], depth) {
				return true
			}
		}
	}
	return false
}

// shellCScript returns the script argument of a shell invoked with -c (alone
// or in a short-flag cluster: -c, -lc, -ec).
func shellCScript(args []string) (string, bool) {
	for k, a := range args {
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 'c') {
			for _, s := range args[k+1:] {
				if !strings.HasPrefix(s, "-") {
					return s, true
				}
			}
			return "", false
		}
	}
	return "", false
}

// isRedirection reports whether a lexed word is a redirection operator (the
// lexer emits each as a word of its own, fd number included).
func isRedirection(w string) bool {
	return strings.ContainsAny(w, "<>") && strings.Trim(w, "0123456789<>&") == ""
}

// ghTokenRead judges a gh invocation's arguments.
func ghTokenRead(args []string) bool {
	var pos []string
	for k := 0; k < len(args); k++ {
		a := args[k]
		switch {
		case isRedirection(a):
			k++ // a redirection operator and its target are not arguments
		case strings.HasPrefix(a, "-"):
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 2 || !strings.EqualFold(pos[0], "auth") {
		return false
	}
	for _, p := range pos[1:] {
		if strings.EqualFold(p, "token") {
			return true
		}
	}
	if strings.EqualFold(pos[1], "status") {
		for _, a := range args {
			la := strings.ToLower(a)
			switch {
			case la == "--show-token", la == "--show-token=true":
				return true
			case strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a[1:], 't'):
				return true
			}
		}
	}
	return false
}
