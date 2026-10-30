package writepolicy

import "strings"

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command's text holds one of the gh
// CLI's token-read trigger words ANYWHERE. It does not parse the shell: no
// lexer, no command positions, no quoting or heredoc model, so nothing here
// decides that a trigger word is data. Prose that names a trigger word is
// refused too (an accepted false refusal: put that text in a file and pass
// -F <file> / --body-file <file> / a pattern file).
//
// The trigger (forgeTrigger) is a whole word `auth`, `token`, `oauth_token` or
// `git-credential`, or the words `status` and `gh` both, matched without case.
// Before matching, the text is normalized the cheap way, in the same single
// pass: quotes, a `$` before a quote, backslashes and line continuations are
// dropped, and the escapes `$'…'`, `echo -e` and printf decode (`\xHH`,
// `\uHHHH`, `\UHHHHHHHH`, octal) are read as the byte they stand for. A word
// is delimited by the start or end of the text, blanks, shell operators,
// braces and commas (forgeDelim), so `GH_TOKEN`, `desktoken` and
// `--token-file` hold no trigger word.
//
// Cost: one pass over the text per trigger word, linear in its length.
func forgeTokenRead(raw string) bool { return forgeTrigger(raw) }

// forgeTriggerWords: the words of every gh token read the guard refuses
// (`gh auth token`, `gh auth status` printing the token, `gh auth
// git-credential`, `gh config get oauth_token`), plus `status` and `gh`, which
// trigger only together.
var forgeTriggerWords = []string{"auth", "token", "oauth_token", "git-credential", "status", "gh"}

// forgeTrigger reports whether s holds one of the trigger words as a whole
// word after normalization (forgeHasWord). It over-approximates on purpose.
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

// forgeFiller is how many bytes at p normalization drops from a word: a
// quote, a `$` before one, a backslash, or a backslash and newline (a line
// continuation); 0 for any other byte.
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

// forgeHasWord reports whether s may hold w as a whole word once quotes,
// backslashes and line continuations are dropped and escapes decoded
// (case-insensitively). A word starts at the start of s or after a forgeDelim
// byte and ends at one; cmd also allows a command path (`/x/gh`, zsh `=gh`)
// and a `.exe` suffix. One pass over s: the set of prefixes of w matched so
// far is kept per position.
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
