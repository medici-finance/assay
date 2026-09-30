package main

// Row-parse comment stripping for `## Evidence` and `## Verify` sections (#1939).
//
// Every reader that parses rows out of those sections strips the HTML comments
// first — the brief template ships the Evidence contract comment, and it is not
// evidence. Those readers used to share htmlCommentRe, whose `(?:-->|$)`
// alternative treats an UNTERMINATED `<!--` as running to end of input. That is
// the right call for evidenceHasContent (an opener with no closer must not let
// the rest of the section pass as content), but applied before ROW parsing it
// silently dropped everything after a stray opener — a later witness table, an
// independent re-run, an UNRUN row — out of the witness, attribution and unrun
// checks, and nothing reported it.
//
// How the page renders depends on where the opener sits (CommonMark):
//
//   - A mid-line `<!--` with no closer is not a comment. It renders as literal
//     text, so the old strip disagreed with what a reader of the brief sees.
//   - A line-start `<!--` opens an HTML block. With no `-->` after it, the block
//     runs to the end of the document and HIDES the rest of the section. For this
//     shape the row readers below deliberately read rows the rendered page does
//     not show. That is safe only because of the lint PROBLEM and the closure
//     refusal below: a section in this shape cannot pass lint or be closed, so
//     the hidden rows never back a landed closure.
//
// The fix has two halves that only work together:
//
//   - stripRowComments strips COMPLETE comments only. An unterminated opener and
//     everything after it stay in place, so the rows after it are still parsed.
//   - unterminatedCommentProblems makes an unterminated opener in either section
//     a lint PROBLEM, and closureWitnesses (used by every closure and audit verb:
//     `brief --check-verified`, `verifyclosure`, `verifyrun --check`) refuses
//     one, because none of those verbs runs the lint.
//
// This covers an unterminated opener inside Verify or Evidence. It does not make
// the parser and the renderer agree on every input: an abruptly closed `<!-->`
// or `<!--->` with a later `-->`, and an opener in another section that closes
// inside Evidence, still read differently, as they did before #1939.
//
// htmlCommentRe (brieffile.go) is kept for evidenceHasContent ONLY;
// TestCommentStripSitesAllowList pins that every other use goes through
// stripRowComments.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// htmlCommentClosedRe matches one COMPLETE HTML comment, `<!--` through the
// nearest `-->`. Unlike htmlCommentRe it has no end-of-input alternative.
var htmlCommentClosedRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// stripRowComments removes every complete HTML comment from an Evidence or
// Verify section body before its rows are parsed. It never strips an
// unterminated opener or the text after it; instead it returns the byte offset
// (into the ORIGINAL section) of the first unterminated `<!--`, or -1 when
// there is none, so a caller that needs to refuse can.
//
// Only the text after the last complete comment can hold an unterminated
// opener: any `<!--` with a `-->` somewhere after it is the start of (or inside)
// a complete match, because the regexp is leftmost-first.
func stripRowComments(section string) (stripped string, unterminatedAt int) {
	matches := htmlCommentClosedRe.FindAllStringIndex(section, -1)
	tail := 0
	if n := len(matches); n > 0 {
		tail = matches[n-1][1]
	}
	unterminatedAt = -1
	if i := strings.Index(section[tail:], "<!--"); i >= 0 {
		unterminatedAt = tail + i
	}
	if len(matches) == 0 {
		return section, unterminatedAt
	}
	return htmlCommentClosedRe.ReplaceAllString(section, ""), unterminatedAt
}

// unterminatedCommentLine returns an excerpt of the source line holding the
// opener at offset — up to 40 runes before it and 60 from it — for a one-line
// diagnostic that always shows the opener itself.
func unterminatedCommentLine(section string, offset int) string {
	start := strings.LastIndex(section[:offset], "\n") + 1
	end := len(section)
	if i := strings.Index(section[offset:], "\n"); i >= 0 {
		end = offset + i
	}
	before := []rune(section[start:offset])
	after := []rune(section[offset:end])
	pre, post := "", ""
	if len(before) > 40 {
		before, pre = before[len(before)-40:], "…"
	}
	if len(after) > 60 {
		after, post = after[:60], "…"
	}
	return strings.TrimSpace(pre + string(before) + string(after) + post)
}

// unterminatedCommentIn reports the first unterminated `<!--` in a brief's
// Verify or Evidence section as a one-line description, or "" when both are
// clean. The shared wording keeps the lint PROBLEM and the --check-verified
// refusal saying the same thing.
func unterminatedCommentIn(verify, evidence string) string {
	for _, sec := range []struct{ name, body string }{{"Verify", verify}, {"Evidence", evidence}} {
		if _, at := stripRowComments(sec.body); at >= 0 {
			return fmt.Sprintf("the `## %s` section has an HTML comment opener `<!--` with no closing `-->` (line: %q)", sec.name, unterminatedCommentLine(sec.body, at))
		}
	}
	return ""
}

// closureWitnesses is the one witness audit for the closure and audit verbs:
// `statusgen brief --check-verified`, `statusgen verifyclosure` and
// `statusgen verifyrun --check`. None of them runs the lint, so each would
// otherwise pass a witness table that a line-start unterminated opener hides
// on the rendered page. It refuses such a section (refusal != "", no findings)
// and otherwise returns checkWitnesses' findings. TestWitnessCallerAllowList
// pins that those verbs reach checkWitnesses only through here.
func closureWitnesses(verify, evidence string) (findings []checkFinding, refusal string) {
	if what := unterminatedCommentIn(verify, evidence); what != "" {
		return nil, what
	}
	return checkWitnesses(verify, evidence), ""
}

// unterminatedCommentProblems is the #1939 lint: a PROBLEM for every brief file
// whose `## Verify` or `## Evidence` section carries an unterminated `<!--`.
// Every brief file is checked, legacy ones included, because the row parsers
// read legacy Evidence too. An unreadable file is left to the checks that own
// file readability.
func unterminatedCommentProblems(streams []*Stream) []string {
	var problems []string
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			verify, evidence, err := briefSections(path)
			if err != nil {
				continue
			}
			if what := unterminatedCommentIn(verify, evidence); what != "" {
				problems = append(problems, fmt.Sprintf("%s: %s — close it, or quote the literal without a raw opener (outside a code span, write it as `&lt;!--`); the row parsers read past it but a rendered page may hide everything after it, so the two views of the section disagree. An opener inside a code span or fenced block renders literally and is a known false positive: reword it (#1939)", path, what))
			}
		}
	}
	sort.Strings(problems)
	return problems
}
