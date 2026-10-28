package main

import (
	"regexp"
	"strings"
)

// renameTokens groups literal text with any adjacent quoted text, so a quoted
// path followed by .bak is still one token. This only recognizes literal spellings;
// it neither interprets shell expansions nor decodes escapes. Unmatched quoting or
// another unsupported spelling leaves the text unchanged. In particular, spaces
// inside a quoted filename never become boundaries for a substring replacement.
var renameTokens = regexp.MustCompile("(?:[^\\s'\"`;|&()<>\\\\]|\\\\.|'[^']*'|\"(?:\\\\.|[^\"\\\\])*\"|`[^`]*`)+")
var renameAssign = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|--?[A-Za-z0-9][A-Za-z0-9_-]*)=$`)

// renamed replaces only a whole literal path token, optionally quoted or the value
// of an option/variable assignment. The proof and --open use this one matcher for
// both Command and Expect, so the row proven is the row written. Similar pathnames,
// quoted concatenations and unsupported shell spellings are never rewritten.
func renamed(s string, f RowFacts) string {
	if f.PathRef == "" || f.RenameHop == "" {
		return s
	}
	spans := renameTokens.FindAllStringIndex(s, -1)
	end := 0
	for _, span := range spans {
		if strings.Trim(s[end:span[0]], " \t\r\n;|&()<>") != "" {
			return s
		}
		end = span[1]
	}
	if strings.Trim(s[end:], " \t\r\n;|&()<>") != "" {
		return s
	}
	return renameTokens.ReplaceAllStringFunc(s, func(token string) string {
		prefix, value := "", token
		if i := strings.IndexByte(token, '='); i >= 0 && renameAssign.MatchString(token[:i+1]) {
			prefix, value = token[:i+1], token[i+1:]
		}
		quote := ""
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"' || value[0] == '`') && value[len(value)-1] == value[0] {
			quote, value = value[:1], value[1:len(value)-1]
		}
		if value != f.PathRef {
			return token
		}
		return prefix + quote + f.RenameHop + quote
	})
}
