package writepolicy

import (
	"regexp"
	"strings"
)

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
// `git-credential`, matched without case, or the words `status` and `gh` both,
// unless every command in the text is a listed read (forgeListedReads).
// Before matching, the text is normalized the cheap way, in the same single
// pass: quotes, a `$` before a quote, backslashes and line continuations are
// dropped, and the escapes `$'…'`, `echo -e` and printf decode (`\xHH`,
// `\uHHHH`, `\UHHHHHHHH`, octal) are read as the byte they stand for. A word
// is delimited by the start or end of the text, blanks, shell operators,
// braces and commas (forgeDelim), and also by a backslash sequence that a
// shell, echo -e or printf may read as a boundary (forgeBreaks), so
// `GH_TOKEN`, `desktoken` and `--token-file` hold no trigger word.
//
// Cost: one pass over the text per trigger word, linear in its length.
func forgeTokenRead(raw string) bool { return forgeTrigger(raw) }

// forgeTriggerWords: the words of every gh token read the guard refuses
// (`gh auth token`, `gh auth status` printing the token, `gh auth
// git-credential`, `gh config get oauth_token`), plus `status` and `gh`, which
// trigger only together.
var forgeTriggerWords = []string{"auth", "token", "oauth_token", "git-credential", "status", "gh"}

// forgeTrigger reports whether s holds one of the trigger words as a whole
// word after normalization (forgeHasWord), or `status` and `gh` in a text that
// forgeListedReads does not clear. It over-approximates on purpose.
func forgeTrigger(s string) bool {
	for _, w := range forgeTriggerWords[:4] {
		if forgeHasWord(s, w, false) {
			return true
		}
	}
	return forgeHasWord(s, "status", false) && forgeHasWord(s, "gh", true) && !forgeListedReads(s)
}

// forgeGhReads: the gh reads that clear `status`. Each is a pair, because a gh
// alias can sit under a command (`gh alias set 'issue mine' …`) but cannot
// replace one of these, even written straight into gh's config (checked against
// gh 2.102.0). `gh <one word>` and `gh api <path>` could be an alias, so neither
// clears. None of these prints the stored credential or reads a local file.
var forgeGhReads = map[string]bool{
	"pr view": true, "pr list": true, "pr checks": true,
	"run view": true, "run list": true,
	"issue view": true, "issue list": true,
}

// forgeFilters: the output filters that clear, with the flags each may take and
// how many operands: grep's pattern, none for the rest, so none opens a file.
var forgeFilters = map[string]struct {
	flags    *regexp.Regexp
	operands int
}{
	"grep": {regexp.MustCompile(`^-[AaBCEFHchilnoqsvwx0-9]+$`), 1},
	"head": {regexp.MustCompile(`^-[nc0-9]+$`), 0},
	"tail": {regexp.MustCompile(`^-[nc0-9]+$`), 0},
	"wc":   {regexp.MustCompile(`^-[lwcm]+$`), 0},
}

var (
	forgeDropQuotes = strings.NewReplacer(`"`, "", "'", "")
	// forgeRoleEnv: the role credential for one command, from a file variable.
	forgeRoleEnv = regexp.MustCompile(`^GH_TOKEN=\$\(cat "\$[A-Za-z_][A-Za-z0-9_]*"\)[ \t]+`)
	// forgeRoleMint: the sanctioned mint, assigned to a variable.
	forgeRoleMint = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=\$\(desktoken( [-A-Za-z0-9_ ./]+)?( 2>/dev/null)?\)$`)
)

// forgeListedReads reports whether every command in s is one listed read:
// `git status`, a forgeGhReads pair, a forgeFilters filter, or the role mint,
// spelled literally. It decides only for the `status` and `gh` pair, and anything
// else refuses, as that pair did before #5046: a computed word, an alias, another
// program, a wrapper, a redirection. Quotes are not modeled, and they cannot hide
// a command: a shell ends a command only at a byte split on here, so every command
// it runs starts a piece checked here. A quoted separator costs a false refusal.
func forgeListedReads(s string) bool {
	for _, c := range strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(";&|\n", r) }) {
		c = strings.Trim(c, " \t")
		if c == "" || forgeRoleMint.MatchString(c) {
			continue
		}
		c = forgeRoleEnv.ReplaceAllString(c, "")
		if strings.ContainsAny(c, "$`\\<>()") {
			return false
		}
		f := strings.FieldsFunc(c, func(r rune) bool { return r == ' ' || r == '\t' })
		switch {
		case len(f) >= 2 && f[0] == "git" && f[1] == "status":
		case len(f) >= 3 && f[0] == "gh" && forgeGhReads[f[1]+" "+f[2]] && !forgeOpensBrowser(f[3:]):
		case len(f) >= 1 && forgeFilterArgs(f[0], f[1:]):
		default:
			return false
		}
	}
	return true
}

// forgeOpensBrowser reports whether args hold gh's `--web` or `-w`, alone or in a
// cluster: the one flag on a listed read that starts a configured program.
func forgeOpensBrowser(args []string) bool {
	for _, a := range args {
		a = forgeDropQuotes.Replace(a)
		if a == "--web" || !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.Contains(a, "w") {
			return true
		}
	}
	return false
}

// forgeFilterArgs reports whether name is a listed filter and args stay inside
// its flags and operand count. Quotes are dropped first, so `"-r"` reads as a
// flag, and a glob byte refuses, since it may expand to more operands. A bare
// number does not count: it is a count for `head -n` or `grep -A`.
func forgeFilterArgs(name string, args []string) bool {
	flt, ok := forgeFilters[name]
	if !ok {
		return false
	}
	operands := 0
	for _, a := range args {
		a = forgeDropQuotes.Replace(a)
		switch {
		case strings.ContainsAny(a, "*?[]{}~"):
			return false
		case strings.HasPrefix(a, "-"):
			if !flt.flags.MatchString(a) {
				return false
			}
		case strings.Trim(a, "0123456789") != "":
			operands++
		}
	}
	return operands <= flt.operands
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
// a command path (`/x/gh`, zsh `=gh`) and a `.exe` suffix. One pass over s: the set of prefixes of w matched so
// far is kept per position.
func forgeHasWord(s, w string, cmd bool) bool {
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
		if a&full != 0 && (p == n || brk || forgeDelim(s[p]) || cmd && strings.HasPrefix(strings.ToLower(s[p:min(p+4, n)]), ".exe")) {
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
