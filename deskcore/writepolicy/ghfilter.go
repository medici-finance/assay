package writepolicy

import "strings"

// --- gh output filter that reads outside the command's output (#5078) ---
//
// gh evaluates a --jq / -q value in-process, and that language can reach past
// the JSON it was handed. ghFilterReads reports whether a Bash command's text
// holds gh and a literal filter that names one of the jq names that do so:
// `env` and `$ENV` (the process environment, so any token passed to the
// command), `now` (the clock), `localtime` and `strflocaltime` (the machine's
// time zone). Selection and formatting filters stay allowed, whatever they
// spell in a string, a field or a jq variable of their own.
//
// As with forgeTokenRead the text is not parsed as a shell program: a flag
// anywhere in the text counts, in any command, as long as a gh word is
// somewhere in the text. So `gh pr list | grep -q env` is refused (an accepted
// false refusal; `grep --quiet` is the way through). A filter the shell
// computes is not read: `--jq "$f"` is a routine recipe, so its text cannot be
// held against it, and `--template` is not checked because gh's template
// functions reach no variable or file. docs/write-policy.md states these limits.
//
// Cost: one pass for the gh word, one over the text, and at most two over each
// filter value, which the text pass then steps past.
func ghFilterReads(s string) bool {
	if !forgeHasWord(s, "gh", true) {
		return false
	}
	for p := 0; p < len(s); {
		start := ghFilterValueAt(s, p)
		if start < 0 {
			p++
			continue
		}
		v, exact, end := shellWordAt(s, start)
		if jqReadsOutside(v, exact) {
			return true
		}
		p = max(end, p+1)
	}
	return false
}

// ghFilterValueAt returns where the value of a filter flag starting at p
// begins, or -1 when no filter flag starts there. The flag spellings are the
// ones gh accepts: `--jq x`, `--jq=x`, `-q x`, `-q=x`, `-qx`, and -q ending a
// short-flag cluster (`-iq x`, `-iqx`). The flag may itself sit in quotes.
func ghFilterValueAt(s string, p int) int {
	if s[p] != '-' || p > 0 && !forgeDelim(s[p-1]) && s[p-1] != '\'' && s[p-1] != '"' {
		return -1
	}
	a := p + 1
	if strings.HasPrefix(s[p:], "--jq") {
		a = p + 4
		if a < len(s) && strings.IndexByte(" \t\r\n=\\'\"", s[a]) < 0 {
			return -1
		}
	} else {
		for a < len(s) && s[a] != 'q' && ('a' <= s[a]|0x20 && s[a]|0x20 <= 'z') {
			a++
		}
		if a == len(s) || s[a] != 'q' {
			return -1
		}
		a++
	}
	if a < len(s) && s[a] == '=' {
		return a + 1
	}
	for a < len(s) {
		switch {
		case s[a] == ' ' || s[a] == '\t':
			a++
		case s[a] == '\\' && a+1 < len(s) && s[a+1] == '\n':
			a += 2
		default:
			return a
		}
	}
	return a
}

// shellWordAt reads one shell word starting at p with its quotes removed and
// returns it with the offset just past it. exact is false when the word holds
// a bare backslash or $'…', where the quoting seen here may not be the quoting
// the filter ends up with (a command nested in a string): the caller then
// gives string literals no credit.
func shellWordAt(s string, p int) (word string, exact bool, end int) {
	var b strings.Builder
	exact = true
	for p < len(s) {
		c := s[p]
		switch {
		case c == '\'':
			j := strings.IndexByte(s[p+1:], '\'')
			if j < 0 {
				b.WriteString(s[p+1:])
				return b.String(), exact, len(s)
			}
			b.WriteString(s[p+1 : p+1+j])
			p += j + 2
		case c == '$' && p+1 < len(s) && s[p+1] == '\'':
			exact = false
			for p += 2; p < len(s) && s[p] != '\''; p++ {
				if s[p] == '\\' && p+1 < len(s) {
					p++
				}
				b.WriteByte(s[p])
			}
			p++
		case c == '$' && p+1 < len(s) && s[p+1] == '"':
			p++
		case c == '"':
			for p++; p < len(s) && s[p] != '"'; p++ {
				if s[p] == '\\' && p+1 < len(s) {
					switch s[p+1] {
					case '\n':
						p++
						continue
					case '"', '\\', '$', '`':
						p++
					}
				}
				b.WriteByte(s[p])
			}
			p++
		case c == '\\':
			exact = false
			if p+1 < len(s) && s[p+1] != '\n' {
				b.WriteByte(s[p+1])
			}
			p += 2
		case strings.IndexByte(" \t\r\n;&|()<>`", c) >= 0:
			return b.String(), exact, p
		default:
			b.WriteByte(c)
			p++
		}
	}
	return b.String(), exact, len(s)
}

// jqReadsOutside reports whether filter q names a jq function or variable that
// reads outside the JSON gh hands it. With strs set, the body of a string
// literal is skipped, except for its \(…) interpolations, which are code. A
// backslash or `#` outside a string means this reading of the quotes cannot be
// trusted (a nested command, a comment holding a quote), so the scan restarts
// without string credit. A name right after one `.` is a field, not a call.
func jqReadsOutside(q string, strs bool) bool {
	var open []int
	depth, inStr := 0, false
	for i := 0; i < len(q); {
		c := q[i]
		switch {
		case inStr && c == '\\':
			if i+1 < len(q) && q[i+1] == '(' {
				open, depth, inStr = append(open, depth), 0, false
			}
			i += 2
		case inStr:
			inStr = c != '"'
			i++
		case strs && (c == '\\' || c == '#'):
			return jqReadsOutside(q, false)
		case strs && c == '"':
			inStr = true
			i++
		case strs && c == '(':
			depth++
			i++
		case strs && c == ')':
			if depth == 0 && len(open) > 0 {
				depth, open, inStr = open[len(open)-1], open[:len(open)-1], true
			} else {
				depth--
			}
			i++
		case jqNameByte(c) && (c < '0' || c > '9'):
			j := i
			for j < len(q) && jqNameByte(q[j]) {
				j++
			}
			if jqNameReads(q, i, j) {
				return true
			}
			i = j
		default:
			i++
		}
	}
	return false
}

func jqNameByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c|0x20 && c|0x20 <= 'z'
}

// jqNameReads judges the name q[i:j] by what comes before it. The list is what
// gh 2.102.0's `builtins` shows reads outside its input and actually runs;
// input, inputs and module loading are compiled out there, so they read nothing.
func jqNameReads(q string, i, j int) bool {
	name := q[i:j]
	if i > 0 && q[i-1] == '$' {
		return name == "ENV"
	}
	if i > 0 && q[i-1] == '.' && (i < 2 || q[i-2] != '.') {
		return false
	}
	switch name {
	case "env", "now", "localtime", "strflocaltime":
		return true
	}
	return false
}
