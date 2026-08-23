package main

// Rule compilation — the one place a token entry becomes something an external
// scanner can run, and the one place the "except a sanctioned longer identifier"
// predicate is decided.
//
// WHY THIS FILE EXISTS AT ALL
// ------------------------
// The engines are gitleaks and trufflehog. Neither knows anything about the
// house token list, so each needs the list compiled into its own config dialect.
// Doing that twice, by hand, in two dialects, is how the two configs drift and
// how one of them silently stops carrying a rule. So there is exactly ONE
// engine-neutral rule model here, and each engine's emitter is a pure rendering
// of it (gitleaks.go, trufflehog.go). A rule that exists cannot exist in only
// one engine.
//
// THE PREFIX-COLLISION SHAPE — AND WHAT WAS ACTUALLY MISSING
// ------------------------
// Some withheld identifiers are substrings of identifiers we deliberately
// publish. The house name is a prefix of the sanctioned public org
// self-reference, so a plain literal token for it matches every legitimate
// module path in the tree. It was therefore left unregistered, and the sweep
// printed `checked-clean` for a shape it had NO RULE FOR — which is
// indistinguishable from a real pass. Measured on the staged public tree
// (2026-08-14): 96 occurrences across 19 files, certified clean.
//
// It would be easy, and wrong, to conclude the old engine COULD NOT express this
// and that the external scanners can. That was checked, and it is false. RE2 —
// which is Go's regexp, and therefore this tool AND both external engines —
// rejects lookahead, but lookahead is not needed: "X not followed by a fixed
// suffix" expands into ordinary alternation, and the incumbent `pattern` shape
// carries it today, scoring 8/8 on a mixed corpus. No engine is being credited
// here with a capability the incumbent lacked. The gap was the rule.
//
// What IS different is authoring safety, measured the same way: the obvious
// hand-written extension to three sanctioned forms — one "not followed by" group
// per form, concatenated — scores 3/6, failing on all three sanctioned forms.
// The correct construction is the complement over a trie of the suffixes, which
// must be generated. `except` is that generator with a declarative surface, so a
// reviewer reads a list of sanctioned forms instead of auditing an alternation
// they cannot check by eye.
//
// The mechanism: the rule matches the MAXIMAL IDENTIFIER around the token (the
// window), and a finding is excused only when every occurrence of the token
// inside that window is accounted for by a sanctioned longer form. The predicate
// is excused(), below, and it is deliberately substring-based rather than
// anchored, because the real shapes are embedded: a module path carries the
// sanctioned org form in the middle, not at the ends.
//
// The same predicate is ALSO emitted into each engine's config as a conservative
// exact-equality allowlist. That emission is a strict SUBSET of excused() — if
// the window equals a sanctioned form exactly, removing that form leaves nothing,
// so excused() would excuse it too. A subset can only ever agree, never excuse
// something the orchestrator would have flagged, which is the direction that
// matters for a security control. The orchestrator remains the authority.

import (
	"fmt"
	"regexp"
	"strings"
)

// identChars is the character class of an IDENTIFIER for windowing purposes:
// letters, digits, and the four connectors that join identifier parts in the
// shapes this repo actually ships — `_` (Go/YAML), `.` (domains, file
// extensions), `/` (org/repo slugs, import paths) and `-` (slug words). It is
// deliberately the SAME connector set the existing bare-repo-name pattern uses
// (see the `pattern` entries in the token file), so a reader does not have to
// hold two definitions of "what counts as one identifier".
//
// `-` is last so it is a literal inside a character class in every engine, and
// `.` and `/` need no escaping inside a class. No word-boundary escape appears
// here or anywhere else in this tool — that ban is total and predates this file.
const identChars = "A-Za-z0-9_./-"

// rule is one compiled detection rule, engine-neutral.
//
// kind is "token" (the withheld string — a hit is a leak) or "control" (a string
// known present in the swept tree — a hit is PROOF THE ENGINE READ THE CORPUS,
// and its ABSENCE is could-not-check). The two are compiled the same way and run
// in the same pass, so a control cannot be checked by a different mechanism than
// the token it vouches for — which was the original insight and is preserved
// verbatim here, just moved inside the external engines.
type rule struct {
	id      string // engine-visible rule id; maps a finding back to its entry
	token   string // the entry token this rule serves
	kind    string // ruleToken | ruleControl
	regex   string // RE2 source — both engines are Go/RE2, so one source serves both
	keyword string // lowercase literal prefilter hint (trufflehog requires one)
	excepts []string
	entry   *Entry
}

const (
	ruleToken   = "token"
	ruleControl = "control"
)

// compileRules turns the token list into the engine-neutral rule set. Every
// control-bearing entry yields TWO rules; a pattern entry yields one (it is
// discovery-style and has no control — see tokens.go).
func compileRules(tl *TokenList) ([]rule, error) {
	var out []rule
	for i, e := range tl.Entries {
		re, err := entryRegex(e)
		if err != nil {
			return nil, withExit(exitError, "entry %d (token %q): %v", i+1, e.Token, err)
		}
		var ex []string
		for _, x := range e.Except {
			ex = append(ex, x.Form)
		}
		out = append(out, rule{
			id:      fmt.Sprintf("house-token-%03d", i+1),
			token:   e.Token,
			kind:    ruleToken,
			regex:   re,
			keyword: keywordFor(e.Sample),
			excepts: ex,
			entry:   e,
		})
		if e.control == nil {
			continue
		}
		cre, err := shapeRegex(e.Match, e.Control, e.Ci, false)
		if err != nil {
			return nil, withExit(exitError, "entry %d (token %q): control: %v", i+1, e.Token, err)
		}
		out = append(out, rule{
			id:      fmt.Sprintf("house-control-%03d", i+1),
			token:   e.Token,
			kind:    ruleControl,
			regex:   cre,
			keyword: keywordFor(e.Control),
			entry:   e,
		})
	}
	return out, nil
}

// entryRegex is the token-side regex for an entry: the plain shape, or — when the
// entry carries exceptions — the maximal-identifier WINDOW around the token, so
// the orchestrator (and the engine allowlist) can see the whole identifier a hit
// sits inside and decide whether a sanctioned longer form accounts for it.
func entryRegex(e *Entry) (string, error) {
	return shapeRegex(e.Match, e.Token, e.Ci, len(e.Except) > 0)
}

// shapeRegex renders one of the three match shapes as RE2 source.
//
// window=true wraps the token in the maximal-identifier class on both sides. It
// is only ever set for except-bearing entries, so no existing entry's matched
// text changes shape.
func shapeRegex(shape, token string, ci, window bool) (string, error) {
	var core string
	switch shape {
	case matchLiteral:
		core = regexp.QuoteMeta(token)
	case matchWord:
		core = "(^|[^" + wordClass + "])" + regexp.QuoteMeta(token) + "([^" + wordClass + "]|$)"
	case matchPattern:
		core = token
	default:
		return "", fmt.Errorf("unknown match shape %q", shape)
	}
	if window {
		// A word shape already consumes a boundary character on each side; the
		// window subsumes that job (a maximal identifier IS bounded by non-
		// identifier characters), so window mode always wraps the QUOTED token,
		// never the word wrapper, or the two boundaries would fight.
		core = "[" + identChars + "]*" + regexp.QuoteMeta(token) + "[" + identChars + "]*"
	}
	if ci {
		core = "(?i)" + core
	}
	// Compile here so a rule that cannot be evaluated stops the run at load time
	// rather than becoming a silently-absent rule inside an external scanner —
	// where "no findings" and "no rule" print identically.
	if _, err := regexp.Compile(core); err != nil {
		return "", fmt.Errorf("regex %q does not compile: %v", core, err)
	}
	return core, nil
}

// keywordFor extracts a lowercase alphanumeric run to serve as trufflehog's
// required keyword prefilter. trufflehog only runs a custom detector on a chunk
// containing one of its keywords, so a wrong keyword is a rule that never fires —
// a dead rule that reports clean. The keyword is therefore derived from the same
// SAMPLE the liveness check plants, and callers must treat an empty result as a
// hard error rather than emitting a keywordless detector.
func keywordFor(s string) string {
	best := ""
	cur := strings.Builder{}
	flush := func() {
		if cur.Len() > len(best) {
			best = cur.String()
		}
		cur.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return best
}

// excused reports whether a matched window is fully accounted for by the entry's
// sanctioned longer forms — the prefix-collision predicate.
//
// The rule is: strike out every occurrence of every sanctioned form from the
// window; if no occurrence of the token survives, the window is excused. This is
// exactly "this identifier, EXCEPT where it appears as part of this other
// sanctioned identifier", and it gets the mixed cases right in the direction that
// matters:
//
//   - a module path made only of the sanctioned org form → excused
//   - the sanctioned org form AND a bare house name on the same line → the bare
//     one survives the strike-out → NOT excused (the engines report per match,
//     not per line, so the two are separate findings anyway)
//   - the sanctioned form with the token repeated inside a DIFFERENT compound
//     (org-form/token-compound) → the compound's occurrence survives → NOT excused
//
// An entry with no `except` list is never excused, so this cannot loosen any
// existing entry.
func excused(e *Entry, window string) bool {
	if len(e.Except) == 0 {
		return false
	}
	hay, tok := window, e.Token
	if e.Ci {
		hay, tok = strings.ToLower(hay), strings.ToLower(tok)
	}
	if tok == "" {
		return false
	}
	for _, x := range e.Except {
		form := x.Form
		if e.Ci {
			form = strings.ToLower(form)
		}
		// Replace rather than delete, so two adjacent sanctioned forms cannot be
		// spliced into a fresh occurrence of the token by their own removal.
		hay = strings.ReplaceAll(hay, form, "\x00")
	}
	return !strings.Contains(hay, tok)
}
