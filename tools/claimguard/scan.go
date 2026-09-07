// Package main implements claimguard: a heuristic check for the "named
// third-party product, no resolving citation" shape of unresolved outward
// claim (an invented or unverifiable product/vendor name asserted without a
// URL a reader could dereference).
//
// This is deliberately narrow. It does not do entity recognition and it does
// not resolve links (a separate link-resolution lint is the tool for "is this
// URL live and on the right host"). It answers one question: does a
// name-shaped token that reads like a third-party product have *any* nearby
// citation at all? A name with a citation that turns out to be wrong or
// off-host is a different, already-covered failure mode; a name with *no*
// citation at all is this one.
package main

import (
	"regexp"
	"strings"
)

// defaultWindow is how many tokens on either side of a candidate we scan for
// a resolving URL before flagging it. Chosen to comfortably cover "Product X
// (https://example.com)" and "Product X, see https://example.com for
// details" while not reaching across an unrelated paragraph.
const defaultWindow = 12

// resolvedMarker replaces a markdown link's URL half during preprocessing so
// the link's anchor text sits immediately next to a synthetic resolver
// token — a name inside `[Name](url)` is resolved regardless of window size.
const resolvedMarker = "\x00CLAIMGUARD_RESOLVED\x00"

var (
	// camelCaseRe matches an internally-capitalized word: at least one
	// lowercase letter, then another uppercase letter later in the token
	// (VerdictCI, GitHub, OAuth). Plain Title-case words ("Verdict") and
	// ALL-CAPS words are handled by the other two patterns.
	camelCaseRe = regexp.MustCompile(`^[A-Z][a-z0-9]+[A-Za-z0-9]*[A-Z][A-Za-z0-9]*$`)

	// acronymRe matches an all-caps token 3-8 letters long (SLAW, ACME,
	// FOOBAR). Two-letter acronyms (CI, ID, OK, ...) are excluded — the
	// false-positive rate there is too high to be worth flagging.
	acronymRe = regexp.MustCompile(`^[A-Z]{3,8}$`)

	urlRe    = regexp.MustCompile(`^\(?<?https?://`)
	mdLinkRe = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\n]+)\)`)
	trimRe   = regexp.MustCompile(`^[^A-Za-z0-9]+|[^A-Za-z0-9]+$`)
)

// Finding is one unresolved named-third-party-product-shaped claim.
type Finding struct {
	Line    int    // 1-based source line
	Token   string // the flagged word (or first word of a multi-word phrase)
	Context string // the source line, for a human to eyeball
}

type token struct {
	raw  string // as it appeared, punctuation included
	word string // trimmed of leading/trailing non-alphanumerics
	line int
}

// Scan finds candidate named-third-party-product tokens in text with no
// resolving URL nearby and returns them, in document order. allow holds
// exact words (case-sensitive) that are never flagged — house terms, the
// project's own name, and known-safe acronyms/products the caller has
// cleared. window overrides defaultWindow when > 0.
func Scan(text string, allow map[string]bool, window int) []Finding {
	if window <= 0 {
		window = defaultWindow
	}

	// Preprocess: fold every markdown link's anchor text and URL together so
	// the anchor sits next to a resolver token regardless of window size.
	prepped := mdLinkRe.ReplaceAllString(text, "$1 "+resolvedMarker)

	toks := tokenize(prepped)

	var findings []Finding
	seen := map[string]bool{}

	for i, t := range toks {
		if t.word == "" || allow[t.word] {
			continue
		}
		if !isCandidate(t.word) {
			continue
		}
		if hasNearbyResolver(toks, i, window) {
			continue
		}
		// Multi-word Title-Case run: only flag once, at its first word, and
		// skip the run so "Open Sky Systems" isn't reported three times.
		key := t.word + ":" + itoa(t.line)
		if seen[key] {
			continue
		}
		seen[key] = true
		findings = append(findings, Finding{
			Line:    t.line,
			Token:   t.word,
			Context: strings.TrimSpace(t.line0Context(text)),
		})
	}
	return findings
}

func isCandidate(word string) bool {
	if camelCaseRe.MatchString(word) {
		return true
	}
	if acronymRe.MatchString(word) {
		return true
	}
	// A lone Title-Case word ("Verdict") or a multi-word Title-Case phrase
	// ("Acme Corp") is deliberately NOT flagged here: distinguishing a
	// product name from an ordinary sentence-initial or headline word needs
	// more than a regex, and the false-positive rate on real prose would be
	// prohibitive. This is a documented limitation (README.md), not an
	// omission — it narrows the tool to the CamelCase/ACRONYM shape the
	// triggering evidence actually showed (`VerdictCI`, `SLAW`).
	return false
}

func hasNearbyResolver(toks []token, i, window int) bool {
	lo := i - window
	if lo < 0 {
		lo = 0
	}
	hi := i + window
	if hi > len(toks)-1 {
		hi = len(toks) - 1
	}
	for j := lo; j <= hi; j++ {
		if j == i {
			continue
		}
		if strings.Contains(toks[j].raw, resolvedMarker) {
			return true
		}
		if urlRe.MatchString(toks[j].raw) {
			return true
		}
	}
	return false
}

func tokenize(text string) []token {
	lines := strings.Split(text, "\n")
	var toks []token
	for i, line := range lines {
		for _, f := range strings.Fields(line) {
			toks = append(toks, token{
				raw:  f,
				word: trimRe.ReplaceAllString(f, ""),
				line: i + 1,
			})
		}
	}
	return toks
}

// line0Context is a small helper kept as a method purely so Scan's loop body
// reads cleanly; it re-derives the source line text for the Finding.
func (t token) line0Context(fullText string) string {
	lines := strings.Split(fullText, "\n")
	if t.line-1 < 0 || t.line-1 >= len(lines) {
		return t.raw
	}
	return lines[t.line-1]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
