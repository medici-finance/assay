package main

// Matching. Three shapes — literal, word, and pattern — and NONE may be built
// from the word-boundary escape.
//
// The whole tool exists because the word-boundary escape is engine-dependent: in
// the regex `git grep` dispatches to on this machine it matches NOTHING and the
// command exits 1, which a checklist reads as "clean". The ban is total — it
// covers Go's own regexp too, where the escape DOES work — because the ban's
// value is that no reader ever has to know which engine a given pattern reaches.
// A word match is therefore expressed with explicit character classes, which
// mean the same thing in every engine; a `pattern` regex is held to the same ban
// (it must spell its boundaries as explicit classes), so the "no reader has to
// know which engine" invariant holds across all three shapes.
//
//   literal — a fixed substring, like `grep -F`. `strings.Contains`. This is the
//             safe default: a false positive costs a human one look, a false
//             negative publishes a secret.
//   word    — the token bounded by non-word characters or the ends of the text,
//             where a word character is [A-Za-z0-9_]. Built as
//             (start-or-nonword)(quoted token)(nonword-or-end) with an explicit
//             class, never the word-boundary escape.
//   pattern — the token IS a regexp, compiled verbatim (never QuoteMeta'd). This
//             is the CLASS matcher: it catches a whole shape of leak an exact set
//             cannot enumerate — an internal issue-ref `<repo>#<n>`, a bare repo
//             name that a `word` match cannot bound because the sanctioned public
//             form (`example-reconciler`, `example-org/reconciler`) reuses the
//             name after a `-` or `/` a word boundary treats as a break. A pattern
//             is DISCOVERY-STYLE (see sweep.go / tokens.go): it carries NO control
//             and ANY match is a leak, because a class rule has no single sibling
//             string that could serve as its positive control. The `\b` ban still
//             applies — a pattern spells its boundaries with explicit classes
//             (e.g. `(^|[^A-Za-z0-9_./-])`), never the escape — and the regexp
//             must COMPILE at load or the whole run refuses (fail-closed).

import (
	"regexp"
	"strings"
)

// matcher tests whether a token occurs in a body of text under one shape.
//
// A matcher is case-SENSITIVE by default — the safe, literal-byte behaviour every
// existing token relies on. `ci` opts a single token into case-insensitive
// matching, for the class of leak the assay#3 Security-Review found: a codename
// registered lower-case (`example-poc`) that ships capitalised (`Example-poc`)
// and slips a byte-exact sweep. It is per-token and off unless asked for, so no
// existing token changes behaviour.
type matcher struct {
	shape string
	token string
	ci    bool           // case-insensitive matching (default false)
	re    *regexp.Regexp // set only for word shape
}

const (
	matchLiteral = "literal"
	matchWord    = "word"
	matchPattern = "pattern"
)

// wordClass is the explicit character class this tool uses in place of the
// banned word-boundary escape. Kept as a named constant so there is exactly one
// definition of "what a word character is" and no pattern is assembled ad hoc.
const wordClass = "A-Za-z0-9_"

func newMatcher(shape, token string, ci bool) (*matcher, error) {
	m := &matcher{shape: shape, token: token, ci: ci}
	switch shape {
	case matchLiteral:
		return m, nil
	case matchWord:
		// (^|[^word]) token ([^word]|$) — explicit classes, no boundary escape.
		// `(?i)` is the case-fold flag, not the banned word-boundary escape, so it
		// is compatible with the total \b ban this tool enforces.
		pat := "(^|[^" + wordClass + "])" + regexp.QuoteMeta(token) + "([^" + wordClass + "]|$)"
		if ci {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		m.re = re
		return m, nil
	case matchPattern:
		// The token IS the regexp — compiled verbatim, never QuoteMeta'd. `(?i)`
		// is the case-fold flag (not the banned word-boundary escape), so a `ci`
		// pattern folds case exactly as the literal/word shapes do. A regexp that
		// does not compile is returned as an error, which loadTokens turns into a
		// hard parse failure: a class rule that cannot be evaluated must stop the
		// run, never silently pass (fail-closed).
		pat := token
		if ci {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, withExit(exitError, "pattern %q does not compile: %v", token, err)
		}
		m.re = re
		return m, nil
	default:
		return nil, withExit(exitError, "unknown match shape %q (want %q, %q, or %q)", shape, matchLiteral, matchWord, matchPattern)
	}
}

// found reports whether the token occurs in text under this matcher's shape.
func (m *matcher) found(text string) bool {
	switch m.shape {
	case matchLiteral:
		if m.ci {
			// Fold both sides to the same case. ToLower on ASCII slugs is exact;
			// on Unicode it is the same fold Go's regexp `(?i)` applies, so the
			// literal and word shapes agree on what "case-insensitive" means.
			return strings.Contains(strings.ToLower(text), strings.ToLower(m.token))
		}
		return strings.Contains(text, m.token)
	case matchWord:
		// A word-shaped token can sit at either end of the text or be immediately
		// adjacent to another match; the class-based pattern consumes a boundary
		// character on each side, so overlapping occurrences need FindAllIndex
		// semantics only when counting — for a boolean "present" a single match
		// is enough, but adjacency ("foo foo") can hide the second behind
		// the first's consumed separator. MatchString answers presence correctly
		// regardless, because at least one occurrence always has a boundary the
		// pattern can consume.
		return m.re.MatchString(text)
	case matchPattern:
		// The compiled regexp answers presence directly. Boundaries, if any, are
		// spelled inside the pattern with explicit classes (the `\b` ban), so this
		// is the same "no reader has to know which engine" guarantee the word shape
		// carries — the pattern just declares its own bounds rather than the fixed
		// (start-or-nonword)…(nonword-or-end) wrapper.
		return m.re.MatchString(text)
	}
	return false
}
