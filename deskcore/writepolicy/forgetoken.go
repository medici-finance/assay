package writepolicy

import (
	"regexp"
	"strings"
)

// --- ambient forge-credential read (#4821) ---
//
// forgeTokenRead reports whether a Bash command's text holds one of the gh
// CLI's token-read trigger words ANYWHERE. forgeListedReads is the one exception
// to the rest of this paragraph: it reads the raw text, splits it into commands
// and reads their first words, flags and operands, and it can clear the `status`
// and `gh` pair. Otherwise this does not parse the shell: no lexer, no command
// positions, no quoting or heredoc model. Its only other shell knowledge is the
// normalization below, and that keeps every reading it models rather than
// choosing one: a backslash sequence is read both as part of a word and as a
// word boundary (forgeBreaks), and any reading that completes a trigger word
// refuses unless forgeListedReads clears the pair. Prose that names a trigger word is
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
// clears. Their arguments stay on forgeGhFlags, and only literal field paths clear.
var forgeGhReads = map[string]bool{
	"pr view": true, "pr list": true, "pr checks": true,
	"run view": true, "run list": true,
	"issue view": true, "issue list": true,
}

// forgeGhFlags: the flags of the forgeGhReads pairs that clear, spelled exactly
// (gh 2.102.0's own flag lists). Off this list: --web and -w, which start a
// configured program, --template and -t, and any short flag with an attached value
// or in a cluster, since a cluster can end in a flag that takes a value. -R and -q
// are in forgeGhChecked. A flag that takes a value on one pair and none on another
// (-a) cannot hide a flag behind it: every word starting `-` is checked here.
var forgeGhFlags = func() map[string]bool {
	m := map[string]bool{}
	for _, f := range strings.Fields(`--all --app --assignee --attempt --author --base --branch
		--comments --commit --created --draft --event --exit-status --fail-fast --head --interval
		--job --json --label --limit --log --log-failed --mention --milestone --required --search
		--state --status --type --user --verbose --watch --workflow
		-A -B -H -L -S -a -b -c -d -e -i -j -l -m -s -u -v`) {
		m[f] = true
	}
	return m
}()

// forgeGhChecked: the flags whose value must match a pattern, in any spelling
// (`--jq x`, `--jq=x`, `-q x`, `-q=x`, `-qx`). A jq value clears only as a literal
// field path (`.status`, `.a.b`); a repo clears only as OWNER/REPO, no host. Every
// byte these patterns and forgeGhWord accept reads the same before and after shell
// expansion, so the value checked here is the value gh receives.
var forgeGhChecked = map[string]*regexp.Regexp{
	"--jq": forgeFieldPath, "-q": forgeFieldPath,
	"--repo": forgeOwnerRepo, "-R": forgeOwnerRepo,
}

var (
	forgeFieldPath = regexp.MustCompile(`^(\.[A-Za-z_][A-Za-z0-9_]*)+$`)
	forgeOwnerRepo = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	// forgeGhWord: any other word on a listed read, so no URL naming another host.
	forgeGhWord = regexp.MustCompile(`^[A-Za-z0-9._,/@-]+$`)
)

// forgeFilters: the output filters that clear, with the flags each may take and
// how many operands: grep's pattern, none for the rest, so none opens a file.
// values holds the flags that take a count (forgeFlagValue).
var forgeFilters = map[string]struct {
	flags    *regexp.Regexp
	operands int
	values   string
}{
	"grep": {regexp.MustCompile(`^-[AaBCEFHchilnoqsvwx0-9]+$`), 1, "ABC"},
	"head": {regexp.MustCompile(`^-[nc0-9]+$`), 0, "nc"},
	"tail": {regexp.MustCompile(`^-[nc0-9]+$`), 0, "nc"},
	"wc":   {regexp.MustCompile(`^-[lwcm]+$`), 0, ""},
}

var (
	// forgeRoleEnv: the role credential for one command, from the file variable the
	// role mint assigned earlier in the same text (forgeListedReads checks that).
	forgeRoleEnv = regexp.MustCompile(`^GH_TOKEN=\$\(cat "\$(TF?)"\)[ \t]+`)
	// forgeRoleMint: the sanctioned mint, assigned to GH_TOKEN or to the file variable
	// forgeRoleEnv reads, and to no name that a shell or gh gives a meaning.
	forgeRoleMint = regexp.MustCompile(`^(GH_TOKEN|TF?)=\$\(desktoken( [-A-Za-z0-9_ ./]+)?( 2>/dev/null)?\)$`)
	// forgeSeparator: what ends a command, with the blanks around the next one.
	forgeSeparator = regexp.MustCompile(`[;&|\n][;&|\n \t]*`)
	// forgeMintBefore and forgeMintAfter: the separators on each side of a mint that
	// always runs it in this shell, so the next command sees what it assigned.
	forgeMintBefore = regexp.MustCompile(`^[;\n \t]*$`)
	forgeMintAfter  = regexp.MustCompile(`^([;\n \t]*|&&[\n \t]*)$`)
)

// forgeListedReads reports whether every command in s is one listed read:
// `git status`, a forgeGhReads pair, a forgeFilters filter, or the role mint,
// spelled literally. It decides only for the `status` and `gh` pair, and anything
// else refuses, as that pair did before #5046: a computed word, an alias, another
// program, a wrapper, a redirection. Quotes play no part in the split, so they cannot
// hide a command: a shell ends a command only at a byte split on here, so every command
// it runs starts a piece checked here. A quoted separator costs a false refusal,
// and so does a quoted blank (forgeQuotedBlank), which words here would misread.
// Inside a piece, each word is read as the shell reads it (forgeUnquote).
// A GH_TOKEN prefix clears only after a mint of its variable that is sure to run:
// one no earlier `#` could comment out and no separator could skip or fork.
func forgeListedReads(s string) bool {
	minted := map[string]bool{}
	seps := forgeSeparator.FindAllStringIndex(s, -1)
	for i := 0; i <= len(seps); i++ {
		start, end, before, after := 0, len(s), "", ""
		if i > 0 {
			start, before = seps[i-1][1], s[seps[i-1][0]:seps[i-1][1]]
		}
		if i < len(seps) {
			end, after = seps[i][0], s[seps[i][0]:seps[i][1]]
		}
		c := strings.Trim(s[start:end], " \t")
		if c == "" {
			continue
		}
		if m := forgeRoleMint.FindStringSubmatch(c); m != nil {
			if forgeMintBefore.MatchString(before) && forgeMintAfter.MatchString(after) && !strings.Contains(s[:start], "#") {
				minted[m[1]] = true
			}
			continue
		}
		if m := forgeRoleEnv.FindStringSubmatch(c); m != nil {
			if !minted[m[1]] {
				return false
			}
			c = c[len(m[0]):]
		}
		if strings.ContainsAny(c, "$`\\<>()") || forgeQuotedBlank(c) {
			return false
		}
		f := strings.FieldsFunc(c, func(r rune) bool { return r == ' ' || r == '\t' })
		switch {
		case len(f) >= 2 && f[0] == "git" && f[1] == "status":
		case len(f) >= 3 && f[0] == "gh" && forgeGhReads[f[1]+" "+f[2]] && forgeGhArgs(f[3:]):
		case len(f) >= 1 && forgeFilterArgs(f[0], f[1:]):
		default:
			return false
		}
	}
	return true
}

// forgeQuotedBlank reports whether a quote in c is left open or holds a blank, or
// whether a single-quoted span closes against another `'`, which zsh's RC_QUOTES reads
// as a literal quote in the span.
// c holds no backslash here, so a quote byte opens or closes a span unless it sits
// inside a span of the other kind, where it is a literal byte. Words are
// split on blanks without modeling quotes, so `grep "x -A" 123` would otherwise
// read as a flag and a count where the shell sees a pattern and a file.
func forgeQuotedBlank(c string) bool {
	var open rune
	for i, r := range c {
		switch {
		case open == 0 && (r == '"' || r == '\''):
			open = r
		case r == open:
			if r == '\'' && strings.HasPrefix(c[i+1:], "'") {
				return true
			}
			open = 0
		case open != 0 && (r == ' ' || r == '\t'):
			return true
		}
	}
	return open != 0
}

// forgeGhArgs reports whether every argument of a listed gh read is a listed flag
// or a plain word. A flag in forgeGhChecked must carry a matching value. It does
// not track which other flags take a value, and needs not to: a word starting `-`
// is checked wherever it sits, so a value position cannot hide a flag.
func forgeGhArgs(args []string) bool {
	for i := 0; i < len(args); i++ {
		a := forgeUnquote(args[i])
		name, val, hasVal := strings.Cut(a, "=")
		if len(a) > 2 && a[0] == '-' && a[1] != '-' && forgeGhChecked[a[:2]] != nil {
			name, val, hasVal = a[:2], strings.TrimPrefix(a[2:], "="), true
		}
		if !strings.HasPrefix(a, "-") {
			if !forgeGhWord.MatchString(a) {
				return false
			}
			continue
		}
		if re := forgeGhChecked[name]; re != nil {
			if !hasVal {
				if i++; i == len(args) {
					return false
				}
				val = forgeUnquote(args[i])
			}
			if !re.MatchString(val) {
				return false
			}
			continue
		}
		if !forgeGhFlags[name] || hasVal && !forgeGhWord.MatchString(val) {
			return false
		}
	}
	return true
}

// forgeFilterArgs reports whether name is a listed filter and args stay inside
// its flags and operand count. A word forgeUnquotedExpands flags refuses. The word
// is read next as the shell reads it (forgeUnquote), so `"-r"` reads as a flag, and
// a glob byte refuses, quoted or not, since it may expand to more operands. A number
// is a count only straight after a flag that takes one (`head -n 5`, `grep -A 3`);
// anywhere else it is an operand, since `head 123` opens a file named 123.
func forgeFilterArgs(name string, args []string) bool {
	flt, ok := forgeFilters[name]
	if !ok {
		return false
	}
	operands, value := 0, false
	for _, a := range args {
		if forgeUnquotedExpands(a) {
			return false
		}
		a = forgeUnquote(a)
		switch {
		case strings.ContainsAny(a, "*?[]{}~"):
			return false
		case value:
			if a == "" || strings.Trim(a, forgeDigits) != "" {
				return false
			}
			value = false
		case strings.HasPrefix(a, "-"):
			next, ok := forgeFlagValue(a[1:], flt.values)
			if !flt.flags.MatchString(a) || !ok {
				return false
			}
			value = next
		default:
			operands++
		}
	}
	return !value && operands <= flt.operands
}

// forgeUnquotedExpands reports whether a word holds `^` or `#` outside quotes, the
// glob bytes of a shell's extended globbing, which would widen a word to file names.
// The word holds no backslash and no quoted blank here (forgeListedReads), so a
// quote byte opens or closes a span unless it sits inside one of the other kind, and
// each span ends inside this word.
func forgeUnquotedExpands(a string) bool {
	var open rune
	for _, r := range a {
		switch {
		case open == 0 && (r == '"' || r == '\''):
			open = r
		case r == open:
			open = 0
		case open == 0 && strings.ContainsRune("^#", r):
			return true
		}
	}
	return false
}

// forgeUnquote reads a word as the shell does: a quote outside a span opens one, the
// same quote closes it, and a quote of the other kind inside a span stays in the
// word. Every matcher check on a word reads this value, since it is what the program
// receives. The word holds no backslash here (forgeListedReads).
func forgeUnquote(a string) string {
	var open byte
	b := make([]byte, 0, len(a))
	for i := 0; i < len(a); i++ {
		switch c := a[i]; {
		case open == 0 && (c == '"' || c == '\''):
			open = c
		case c == open:
			open = 0
		default:
			b = append(b, c)
		}
	}
	return string(b)
}

const forgeDigits = "0123456789"

// forgeFlagValue reads a flag cluster for a flag in values. next says the count
// is the following argument; ok is false for a spelling read two ways by
// different tools: a number before the flag (`-5c 123` reads file 123 under some
// head and tail builds) or anything but a number after it (`-n5c`).
func forgeFlagValue(cluster, values string) (next, ok bool) {
	i := strings.IndexAny(cluster, values)
	if i < 0 {
		return false, true
	}
	if strings.ContainsAny(cluster[:i], forgeDigits) || strings.Trim(cluster[i+1:], forgeDigits) != "" {
		return false, false
	}
	return i+1 == len(cluster), true
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
