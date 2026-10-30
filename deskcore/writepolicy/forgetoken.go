package writepolicy

import "strings"

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command's text holds one of the gh
// CLI's token-read trigger words ANYWHERE. It does not parse the shell: no
// lexer, no command positions, no quoting or heredoc model. Its only shell
// knowledge is the normalization below, and that keeps every reading it
// models rather than choosing one: a backslash sequence is read both as part
// of a word and as a word boundary (forgeBreaks), and any reading that
// completes a trigger word refuses. Prose that names a trigger word is
// refused too (an accepted false refusal: put that text in a file and pass
// -F <file> / --body-file <file> / a pattern file).
//
// The trigger (forgeTrigger) is a whole word `auth`, `token`, `oauth_token` or
// `git-credential`, or the words `status` and `gh` together with a token-print
// flag (forgePrintFlags), matched without case.
// Before matching, the text is normalized the cheap way, in the same single
// pass: quotes, a `$` before a quote, backslashes and line continuations are
// dropped, and the escapes `$'…'`, `echo -e` and printf decode (`\xHH`,
// `\uHHHH`, `\UHHHHHHHH`, octal) are read as the byte they stand for. A word
// is delimited by the start or end of the text, blanks, shell operators,
// braces and commas (forgeDelim), and also by a backslash sequence that a
// shell, echo -e or printf may read as a boundary (forgeBreaks), so
// `GH_TOKEN`, `desktoken` and `--token-file` hold no trigger word.
//
// Cost: one pass over the text per word and flag, linear in its length.
func forgeTokenRead(raw string) bool { return forgeTrigger(raw) }

// forgeTriggerWords: the words of every gh token read the guard refuses
// (`gh auth token`, `gh auth status`, `gh auth git-credential`, `gh config get
// oauth_token`).
var forgeTriggerWords = []string{"auth", "token", "oauth_token", "git-credential"}

// forgePrintFlags: the flag that makes `gh auth status` print the token, alone
// (`-t`, also matching `-ta` and `-t=1`), after the only boolean short that can
// lead a cluster (`-at`), and in long form (`--show-token`, also with `=value`).
// A status check prints no token without one of them. `auth` missing from the
// text means the subcommand is computed, so a form the flag does not spell out
// as well is deliberate evasion, which this guard leaves to other layers.
var forgePrintFlags = []string{"-t", "-at", "--show-token"}

// forgeTrigger reports whether s holds one of the trigger words as a whole
// word after normalization (forgeHasWord), or `gh`, `status` and a token-print
// flag together. It over-approximates on purpose: `status` may be anyone's
// word and the flag anyone's flag.
func forgeTrigger(s string) bool {
	for _, w := range forgeTriggerWords {
		if forgeHasWord(s, w, false, false) {
			return true
		}
	}
	if !forgeHasWord(s, "status", false, false) || !forgeHasWord(s, "gh", true, false) {
		return false
	}
	for _, f := range forgePrintFlags {
		if forgeHasWord(s, f, true, true) {
			return true
		}
	}
	return false
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

// forgeBreaks calls f(m) for each length m of a backslash sequence at p that
// a shell, echo -e or printf may read as a word boundary. forgeFiller drops
// the same bytes as part of a word, so both readings are kept:
//   - a line continuation: a shell joins it mid-input but drops it at the end
//     of its input (the last body line of a quoted heredoc fed to a shell),
//     which ends the word there;
//   - an escape that decodes to a blank or a newline: `\n`, `\t`, `\r`, the
//     control escapes `\cI`, `\cJ`, `\cM`, or a numeric escape (forgeEscapes)
//     whose byte is a forgeDelim byte.
func forgeBreaks(s string, p int, f func(m int)) {
	if s[p] != '\\' || p+1 >= len(s) {
		return
	}
	switch s[p+1] {
	case '\n', 'n', 't', 'r':
		f(2)
	case 'c':
		if p+2 < len(s) && strings.IndexByte("ijmIJM", s[p+2]) >= 0 {
			f(3)
		}
	}
	forgeEscapes(s, p, func(m, v int) {
		if v < 0x80 && forgeDelim(byte(v)) {
			f(m)
		}
	})
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
// (case-insensitively). A word starts at the start of s, after a forgeDelim
// byte or after a forgeBreaks sequence, and ends at either; cmd also allows
// a start after `/` or `=` (`/x/gh`, zsh `=gh`, `f=-t`) and a `.exe` suffix;
// prefix lets w end anywhere, so w only has to start the word. One pass over s:
// the set of prefixes of w matched so far is kept per position.
func forgeHasWord(s, w string, cmd, prefix bool) bool {
	n, full := len(s), uint32(1)<<len(w)
	act := make([]uint32, n+1)
	for p := 0; p <= n; p++ {
		if p == 0 || forgeDelim(s[p-1]) || cmd && (s[p-1] == '/' || s[p-1] == '=') {
			act[p] |= 1
		}
		brk := false
		if p < n {
			forgeBreaks(s, p, func(m int) { brk, act[p+m] = true, act[p+m]|1 })
		}
		a := act[p]
		if a == 0 {
			continue
		}
		if a&full != 0 && (prefix || p == n || brk || forgeDelim(s[p]) || cmd && strings.HasPrefix(strings.ToLower(s[p:min(p+4, n)]), ".exe")) {
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
