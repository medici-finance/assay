package main

// Output-containment clauses in an Expect cell (#7389).
//
// THE DEFECT. A Verify row whose Expect reads "output contains `--- PASS:
// TestName`" was credited on its exit status alone: parseExpect read the exit,
// the `output is` line and the `≥ N` floor, and nothing else. `go test -run
// '^TestName$'` exits 0 and prints "no tests to run" when TestName does not
// exist, so a renamed or deleted test passed the row at head, and the same
// reading at the fail-first base scored a base with no such test green, which
// reported a real fail-first row as non-discriminating.
//
// THE CLASS is every Expect clause of the form `<output|stdout|stderr> [line]
// [must] contain(s) …` or its negation (`does not contain`, `must not contain`,
// `never contains`) that the reader skipped and the verdict then credited on
// the exit status. This file closes it in one place, the reader both the head
// run and the fail-first base run go through (expectSpec.verdict), so the
// base reading cannot drift from the head reading.
//
// THE RULE. A clause whose literals can be read is checked against the
// combined output. A clause that names output but whose literal cannot be read
// (prose such as "output contains the named PASS line") is never decided on
// the exit status: the row is could-not-run, the same fail-closed reading a
// declared `≥ N` floor with no number already gets. The change only adds
// constraints: no row that failed before can pass now.
//
// A literal is a backtick code span, a double-quoted string, or one bare
// token (no spaces) that ends the clause. Several literals joined by `,` or
// `and` must all hold. `or` between literals is a disjunction this reader
// does not decide, so the clause is unreadable.
//
// A positive `--- PASS|FAIL|SKIP: <name>` literal is matched as go test's own
// result line, not as a substring: the line must START with the literal and
// continue with ` (`, and a top-level name (no `/`) must be unindented. An
// unanchored substring would find `    --- PASS: TestName/sub` (a passing
// subtest under a failing parent) and `--- PASS: TestNameLonger` (a different
// test), so the row would still pass without the named test passing. A
// negative literal stays an unanchored substring: for an absence claim the
// wider match is the stricter one.

import (
	"bytes"
	"regexp"
	"strings"
)

// outputClaim is one literal an Expect clause requires present or absent.
type outputClaim struct {
	literal string
	absent  bool
}

// expectContainsRe finds the start of an output-containment clause. The
// negation group is captured so the clause's polarity is read from it.
var expectContainsRe = regexp.MustCompile(`(?i)\b(?:output|stdout|stderr)\s+(?:line\s+)?(?:(?:must|should|will)\s+)?(?:(does\s+not|doesn't|do\s+not|not|never)\s+)?contains?\b`)

// goTestResultRe is a go test result line literal: `--- PASS: TestName`.
var goTestResultRe = regexp.MustCompile(`^--- (?:PASS|FAIL|SKIP): (\S+)$`)

// unreadableContainsNote is the fixed could-not-run reason. It is a constant,
// never caller text, because a could-not-run reason is written into the
// witness row.
const unreadableContainsNote = "Expect states an output-contains clause whose literal cannot be read (quote it in backticks), so the row is not decided on its exit status"

// parseOutputClaims reads every output-containment clause of an Expect cell.
// unreadable is true when any clause names output but yields no literal.
func parseOutputClaims(expect string) (claims []outputClaim, unreadable bool) {
	for _, loc := range expectContainsRe.FindAllStringSubmatchIndex(expect, -1) {
		absent := loc[2] >= 0
		lits, ok := readClaimLiterals(expect[loc[1]:])
		if !ok {
			unreadable = true
			continue
		}
		for _, l := range lits {
			claims = append(claims, outputClaim{literal: l, absent: absent})
		}
	}
	return claims, unreadable
}

// readClaimLiterals reads the literal list that follows "contains". It returns
// false when the first item is not a literal, or when the list is joined by
// `or`.
func readClaimLiterals(s string) ([]string, bool) {
	s = strings.TrimLeft(s, " \t")
	// Filler before the list: "both", "the literal line", "BOTH literal lines".
	for _, w := range []string{"both", "the", "literal", "lines", "line"} {
		if rest, ok := cutWord(s, w); ok {
			s = rest
		}
	}
	var lits []string
	for {
		lit, rest, ok := readClaimItem(s)
		if !ok {
			// The first item must be a literal. A later one that is not is
			// ordinary prose after the list ("`ok`, exit 0"), so the list
			// ends at the last literal read.
			return lits, len(lits) > 0
		}
		lits = append(lits, lit)
		rest = strings.TrimLeft(rest, " \t")
		if _, ok := cutWord(rest, "or"); ok || strings.HasPrefix(rest, "/") {
			return nil, false
		}
		next, sep := cutSeparator(rest)
		if !sep {
			return lits, true
		}
		s = next
	}
}

// cutSeparator consumes a list separator: `,`, `and`, or `, and`.
func cutSeparator(s string) (string, bool) {
	sep := false
	if strings.HasPrefix(s, ",") {
		s, sep = strings.TrimLeft(s[1:], " \t"), true
	}
	if r, ok := cutWord(s, "and"); ok {
		s, sep = r, true
	}
	return s, sep
}

// cutWord consumes word (case-insensitive) when s starts with it followed by
// a space.
func cutWord(s, word string) (string, bool) {
	if len(s) > len(word) && strings.EqualFold(s[:len(word)], word) && (s[len(word)] == ' ' || s[len(word)] == '\t') {
		return strings.TrimLeft(s[len(word)+1:], " \t"), true
	}
	return s, false
}

// readClaimItem reads one literal off the front of s.
func readClaimItem(s string) (lit, rest string, ok bool) {
	s = strings.TrimLeft(s, " \t")
	switch {
	case strings.HasPrefix(s, "`"):
		n := len(s) - len(strings.TrimLeft(s, "`"))
		fence := s[:n]
		end := strings.Index(s[n:], fence)
		if end < 0 {
			return "", s, false
		}
		body := s[n : n+end]
		if n > 1 && len(body) >= 2 && body[0] == ' ' && body[len(body)-1] == ' ' {
			body = body[1 : len(body)-1]
		}
		if strings.TrimSpace(body) == "" {
			return "", s, false
		}
		return body, s[n+end+n:], true
	case strings.HasPrefix(s, `"`), strings.HasPrefix(s, "“"):
		open, closer := `"`, `"`
		if strings.HasPrefix(s, "“") {
			open, closer = "“", "”"
		}
		end := strings.Index(s[len(open):], closer)
		if end < 0 || strings.TrimSpace(s[len(open):len(open)+end]) == "" {
			return "", s, false
		}
		return s[len(open) : len(open)+end], s[len(open)+end+len(closer):], true
	}
	// One bare token. It is a literal only when it ends the clause or the
	// list: prose ("the named PASS line") is not.
	i := strings.IndexAny(s, " \t")
	tok, after := s, ""
	if i >= 0 {
		tok, after = s[:i], s[i:]
	}
	if strings.ContainsAny(tok, "`\"“”|") {
		return "", s, false
	}
	switch trimmed := strings.TrimRight(tok, ".;,)"); {
	case trimmed == "" || proseWords[strings.ToLower(trimmed)]:
		return "", s, false
	case trimmed != tok:
		// Trailing punctuation ends the token; a comma keeps the list open.
		return trimmed, tok[len(trimmed):] + after, true
	default:
		next := strings.TrimLeft(after, " \t")
		if next == "" || strings.HasPrefix(next, "(") || strings.HasPrefix(next, ";") || strings.HasPrefix(strings.ToLower(next), "<br") {
			return tok, after, true
		}
		if _, ok := cutWord(next, "and"); ok {
			return tok, after, true
		}
		if _, ok := cutWord(next, "or"); ok {
			return tok, after, true
		}
		return "", s, false
	}
}

// proseWords are bare words that refer to a literal rather than state one
// ("output contains it."), so they are never read as the literal itself.
var proseWords = map[string]bool{
	"it": true, "this": true, "that": true, "them": true, "these": true, "those": true,
	"no": true, "nothing": true, "a": true, "an": true, "one": true, "two": true,
	"each": true, "every": true, "all": true, "only": true, "exactly": true, "some": true,
}

// outputHasClaim reports whether output satisfies a positive literal.
func outputHasClaim(output []byte, lit string) bool {
	m := goTestResultRe.FindStringSubmatch(lit)
	if m == nil {
		return bytes.Contains(output, []byte(lit))
	}
	topLevel := !strings.Contains(m[1], "/")
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimRight(line, "\r")
		if !topLevel {
			line = strings.TrimLeft(line, " \t")
		}
		if strings.HasPrefix(line, lit+" (") {
			return true
		}
	}
	return false
}
