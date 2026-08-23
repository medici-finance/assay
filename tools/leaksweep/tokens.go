package main

// Token-list loading and validation for docs/leak-sweep-tokens.yaml.
//
// The list is a CONTROL list, not merely a token list. Every LITERAL or WORD
// entry pairs the withheld token with a `control` — a token known to be present
// in the tree being swept, matched by the SAME shape. The control is what turns a
// zero token count from "clean" into "checked-and-clean": if the control is not
// found, the search mechanism itself is suspect, so a zero token count proves
// nothing and the entry reports could-not-check rather than clean.
//
// A missing control is a HARD PARSE ERROR for those shapes, not a warning. The
// whole point is that the control cannot be forgotten in a hurry: an entry
// without one does not load, so the tool refuses to run rather than run a check
// that cannot fail.
//
// A `pattern` entry is the ONE exception, by design. A pattern
// matches a CLASS of leak (an internal issue-ref `<repo>#<n>`, a bare repo name
// the word shape cannot bound), and a class has no single sibling string that
// could serve as its positive control. So a pattern is DISCOVERY-STYLE: it
// carries NO control (a control on a pattern is itself a parse error), and ANY
// match is a leak. Its fail-closed guarantee is NOT weaker: (1) a pattern regexp
// that does not COMPILE is a hard parse error, so a broken class rule stops the
// run rather than passing; and (2) the run's fail-closed proof-of-search is
// supplied by the literal/word entries whose controls prove the corpus was
// actually read — a pattern can only ever ADD a leak finding, never turn a
// could-not-check run into a clean one (worstExit is worst-state-wins). So the
// token file must always carry control-bearing entries, which it does.

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const tokensSchema = "leak-sweep-tokens-v1"

// Entry is one withheld token plus its positive control.
type Entry struct {
	Token   string   `yaml:"token"`
	Match   string   `yaml:"match"`        // literal | word
	Ci      bool     `yaml:"ci,omitempty"` // case-insensitive matching (default false)
	Control string   `yaml:"control"`      // MANDATORY — a token known present in the swept tree
	Reason  string   `yaml:"reason"`
	Allow   []Allow  `yaml:"allow,omitempty"`
	Except  []Except `yaml:"except,omitempty"` // sanctioned longer identifiers that CONTAIN this token
	Sample  string   `yaml:"sample,omitempty"` // a string this entry MUST match — the per-rule positive control
	line    int      // source line, for error messages
	matcher *matcher // token matcher, built at load
	control *matcher // control matcher, same shape
}

// Except is a SANCTIONED LONGER IDENTIFIER that legitimately contains the token.
//
// It exists for the prefix-collision shape: a withheld identifier that is a
// substring of a form we deliberately publish. The house name is a prefix of the
// sanctioned public org self-reference, so a plain literal token for it matches
// every legitimate module path in the tree, and it was therefore left
// unregistered — meaning the file carried NO RULE for the house name while the
// sweep still printed `checked-clean`. Measured on the staged public tree on
// 2026-08-14: 96 occurrences across 19 files, all certified clean.
//
// BE PRECISE ABOUT WHAT WAS MISSING: it was the RULE, not the capability. This
// was checked rather than assumed. RE2 (Go's regexp, and so both this tool and
// both external engines) rejects lookahead — `medici(?!-finance)` fails to
// compile — but lookahead is not required. "X not followed by a fixed suffix"
// expands into ordinary alternation, and the existing `pattern` shape can carry
// it today:
//
//	medici($|[^-]|-[^f]|-f[^i]|-fi[^n]|-fin[^a]|-fina[^n]|-finan[^c]|-financ[^e])
//
// That scores 8/8 on a mixed corpus, including the sanctioned module path, a
// bare mention on the SAME LINE as a sanctioned form, and the genuine near-miss
// `medici-financial-times`. So no engine gets credit here for a class the
// incumbent could not express.
//
// What `except` buys is AUTHORING SAFETY, and that is a real and measured
// difference rather than a capability one. The hand-written extension to three
// sanctioned forms — concatenating one "not followed by" group per form, which
// is the obvious thing to write — scores 3/6: it fails on ALL THREE sanctioned
// forms, i.e. it is exactly the cry-wolf rule that gets a gate switched off
// in practice. Getting it right needs the complement over a TRIE of the suffixes,
// which has to be generated rather than written by hand. `except` is that
// generator, expressed declaratively: name the forms, and the predicate and both
// engine configs are derived from them.
//
// The predicate is implemented once, in excused() (rules.go), and applied
// identically to every engine's findings — see that function for the exact
// semantics and for why an exact-equality allowlist is ALSO emitted into each
// engine's config (it is a strict subset of the orchestrator predicate, so it can
// only ever agree).
//
// Like `allow`, every exception carries a mandatory reason: an unexplained
// exception is an unreviewable hole in a security control.
type Except struct {
	Form   string `yaml:"form"`
	Reason string `yaml:"reason"`
}

// Allow is a path where the token is permitted, with a per-path reason. An allow
// without a reason is as unreviewable as a manifest row without one, so the
// reason is mandatory too.
type Allow struct {
	Path   string `yaml:"path"`
	Reason string `yaml:"reason"`
}

// TokenList is the whole parsed file.
type TokenList struct {
	Schema  string   `yaml:"schema"`
	Entries []*Entry `yaml:"tokens"`
	// StockAllow are paths where a finding from an ENGINE'S OWN detectors is
	// expected and reviewed — a documented fake credential in a test fixture, an
	// example token in prose.
	//
	// This list exists because the alternative to it is one of two failures. Drop
	// stock findings silently and a secret scanner can find a live credential in
	// the tree while the gate says nothing. Report every one as a hard failure and
	// the gate reddens on deliberate test placeholders until somebody switches it
	// off — the six-day-red shape. An enumerated, reasoned allowlist is the only
	// version that stays both honest and usable, and it is the same shape `allow:`
	// already uses per token, for the same reason.
	StockAllow []Allow `yaml:"stock_allow,omitempty"`
}

// loadTokens reads and validates the token file. Every failure here is exitError
// (a broken instrument), never a silent skip: a token file that half-parses is a
// sweep that half-checks.
func loadTokens(path string) (*TokenList, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, withExit(exitError, "reading token file %s: %v", path, err)
	}

	// Decode into a node first so a missing `control` can be reported with its
	// line number and so an unknown key is a parse error rather than a silent
	// discard — the same discipline the publication manifest uses, for the same
	// reason: a field written where the schema does not define it reads as
	// recorded and has no effect.
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, withExit(exitError, "parsing token file %s: %v", path, err)
	}

	var tl TokenList
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&tl); err != nil {
		return nil, withExit(exitError, "parsing token file %s: %v", path, err)
	}
	if tl.Schema != tokensSchema {
		return nil, withExit(exitError, "token file %s: schema is %q, want %q", path, tl.Schema, tokensSchema)
	}
	if len(tl.Entries) == 0 {
		return nil, withExit(exitError, "token file %s carries no tokens — an empty token list is a sweep that checks nothing", path)
	}

	// A stock_allow entry with no reason is an unreviewable hole in a security
	// control, exactly as a reasonless per-token `allow` would be.
	for i, a := range tl.StockAllow {
		if strings.TrimSpace(a.Path) == "" || strings.TrimSpace(a.Reason) == "" {
			return nil, withExit(exitError,
				"token file %s: stock_allow entry %d needs both a path and a reason", path, i+1)
		}
	}

	// Attach source lines for good error messages.
	lineOf := buildLineIndex(&doc)

	seen := map[string]bool{}
	for i, e := range tl.Entries {
		e.line = lineOf(e.Token)
		where := fmt.Sprintf("token file %s, entry %d (token %q, line %d)", path, i+1, e.Token, e.line)
		if strings.TrimSpace(e.Token) == "" {
			return nil, withExit(exitError, "%s: empty token", where)
		}
		if seen[e.Token] {
			return nil, withExit(exitError, "%s: duplicate token", where)
		}
		seen[e.Token] = true
		if e.Match == "" {
			e.Match = matchLiteral
		}
		if e.Match != matchLiteral && e.Match != matchWord && e.Match != matchPattern {
			return nil, withExit(exitError, "%s: match must be %q, %q, or %q, got %q", where, matchLiteral, matchWord, matchPattern, e.Match)
		}
		isPattern := e.Match == matchPattern
		// The control field. For literal/word it is MANDATORY — its absence is what
		// separates this from the checklist it replaces, so it stops the whole run.
		// For a `pattern` (discovery-style class rule) a control is meaningless — a
		// class has no single sibling string to prove — so a control on a pattern is
		// itself a parse error, keeping the model unambiguous: pattern ⇒ no control.
		if isPattern {
			if strings.TrimSpace(e.Control) != "" {
				return nil, withExit(exitError,
					"%s: a `pattern` entry must NOT carry a `control` — a pattern is discovery-style (any match is a leak); a control would be meaningless. Remove the control.",
					where)
			}
		} else if strings.TrimSpace(e.Control) == "" {
			return nil, withExit(exitError,
				"%s: missing mandatory `control` — a token with no positive control is a check that cannot prove it looked; add a control token known to be present in the swept tree",
				where)
		}
		if strings.TrimSpace(e.Reason) == "" {
			return nil, withExit(exitError, "%s: missing `reason`", where)
		}
		if containsBannedConstruct(e.Token) || containsBannedConstruct(e.Control) {
			return nil, withExit(exitError, "%s: token or control contains the banned word-boundary escape", where)
		}
		for _, a := range e.Allow {
			if strings.TrimSpace(a.Path) == "" || strings.TrimSpace(a.Reason) == "" {
				return nil, withExit(exitError, "%s: an allow entry needs both a path and a reason", where)
			}
		}
		// `except` — sanctioned longer identifiers. A pattern declares its own
		// bounds inside the regexp, so an exception list on one would be a second,
		// contradictory way to say the same thing; that is a parse error, exactly
		// as a control on a pattern is.
		if isPattern && len(e.Except) > 0 {
			return nil, withExit(exitError,
				"%s: a `pattern` entry must NOT carry `except` — a pattern spells its own boundaries in the regexp; express the sanctioned form there instead",
				where)
		}
		for _, x := range e.Except {
			if strings.TrimSpace(x.Form) == "" || strings.TrimSpace(x.Reason) == "" {
				return nil, withExit(exitError, "%s: an except entry needs both a form and a reason", where)
			}
			// An exception that does not CONTAIN the token excuses nothing — it is
			// a typo wearing the shape of a security decision, and it would sit in
			// the file reading as coverage. Fail the load rather than carry it.
			hay, needle := x.Form, e.Token
			if e.Ci {
				hay, needle = strings.ToLower(hay), strings.ToLower(needle)
			}
			if !strings.Contains(hay, needle) {
				return nil, withExit(exitError,
					"%s: except form %q does not contain the token — an exception that cannot apply is a hole that reads as coverage",
					where, x.Form)
			}
			if x.Form == e.Token {
				return nil, withExit(exitError,
					"%s: except form %q IS the token — that would excuse every occurrence and turn the entry into a rule that cannot fail",
					where, x.Form)
			}
		}
		// Build the token matcher. A `pattern` whose regexp does not compile is
		// returned as an error here and becomes a hard parse failure — the
		// fail-closed rule for a class rule that cannot be evaluated.
		mt, err := newMatcher(e.Match, e.Token, e.Ci)
		if err != nil {
			return nil, withExit(exitError, "%s: %v", where, err)
		}
		e.matcher = mt
		if !isPattern {
			// The control is matched by the SAME shape AND the same case-sensitivity
			// as the token, so a `ci` token's control is proven present under the
			// same folding the token is searched under. A pattern has no control, so
			// e.control stays nil and evaluate() skips the control gate for it.
			ct, err := newMatcher(e.Match, e.Control, e.Ci)
			if err != nil {
				return nil, withExit(exitError, "%s: control: %v", where, err)
			}
			e.control = ct
		}
		// `sample` — the entry's OWN positive control, and the answer to the one
		// hole the control-then-token model never closed.
		//
		// A `control` proves the corpus was READ (a known-present sibling string
		// was found). It does NOT prove the rule for THIS token is live: a rule
		// that compiles to something that can never match still reports
		// checked-clean on every tree, forever, which is the #643 dead-pipeline
		// shape one level down. The sample is a string the entry MUST match; the
		// orchestrator plants every sample in a fixture and requires each engine
		// to report it before that engine may certify anything.
		//
		// For literal/word the sample defaults to the token itself, so no existing
		// entry needs editing. For a `pattern` it is MANDATORY, because a class
		// regexp has no self-evident instance — which is precisely why patterns
		// were previously the one shape with no positive control of any kind.
		if strings.TrimSpace(e.Sample) == "" {
			if isPattern {
				return nil, withExit(exitError,
					"%s: a `pattern` entry must carry a `sample` — a string the class rule must match, so the engines can prove the rule is live; a class regexp has no self-evident instance to default to",
					where)
			}
			e.Sample = e.Token
		}
		if !e.matcher.found(e.Sample) {
			return nil, withExit(exitError,
				"%s: sample %q is NOT matched by this entry's own rule — the positive control does not fire, so the rule could never be proven live",
				where, e.Sample)
		}
		if excused(e, e.Sample) {
			return nil, withExit(exitError,
				"%s: sample %q is excused by this entry's own `except` list — the positive control would be suppressed by the exception it is meant to be tested against",
				where, e.Sample)
		}
	}
	return &tl, nil
}

// containsBannedConstruct reports whether s contains the two-character
// word-boundary escape. Written by byte value so this source file itself does
// not contain the literal it forbids — which is what lets Verify row 13a assert
// the whole tool is free of it.
func containsBannedConstruct(s string) bool {
	esc := string([]byte{'\\', 'b'})
	return strings.Contains(s, esc)
}

// buildLineIndex maps a token string to the source line of the mapping node
// whose `token:` value equals it, for human-readable errors.
func buildLineIndex(doc *yaml.Node) func(string) int {
	index := map[string]int{}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Value == "token" && v.Kind == yaml.ScalarNode {
					index[v.Value] = v.Line
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(doc)
	return func(tok string) int { return index[tok] }
}

// byToken finds an entry by token, for tests.
func (tl *TokenList) byToken(tok string) *Entry {
	for _, e := range tl.Entries {
		if e.Token == tok {
			return e
		}
	}
	return nil
}
