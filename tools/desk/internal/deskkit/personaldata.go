package deskkit

// personaldata.go — the generic PERSONAL-DATA pass of the outbound-write check (outbound.go).
//
// Two shapes, both GENERIC: an e-mail address and a telephone number. Nothing here names a
// deployment, a person or a domain a deployment uses; the only compiled values are the forge
// no-reply domains and the reserved documentation names (RFC 2606 / RFC 6761), which are
// public by definition. A deployment that needs to allow more says so through its own
// configuration layer, never through this list.
//
// THE SPLIT BETWEEN REFUSE AND NOTICE follows the same three-state rule selfcontain.go does.
//
//   - pii.email REFUSES: an address outside the allow-list is unambiguous.
//   - pii.phone REFUSES: a `+` followed by 8-15 digits (optional space, dot, hyphen and
//     parenthesis separators) is the international (E.164) shape, and nothing else in a desk
//     body is spelled that way.
//   - pii.phone-ambiguous NOTICES: a separator-grouped 10-11 digit run with no `+` is also
//     what an order number, a build id or a formatted count looks like. Refusing it would
//     strand legitimate work on a false positive, so it is reported, never blocked.

import (
	"regexp"
	"strings"
)

var (
	// reEmail matches an e-mail address. The local part admits `[` and `]` so a forge
	// bot's no-reply address (`<id>+<slug>[bot]@users.noreply.github.com`) is matched
	// WHOLE and passes through the allow-list below, rather than escaping the pattern by
	// accident of its brackets.
	reEmail = regexp.MustCompile(`[A-Za-z0-9._%+\[\]-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	// rePhoneE164 matches the international shape: `+`, a digit, then digits and single
	// separators, ending on a digit. The digit count is checked separately (8-15).
	rePhoneE164 = regexp.MustCompile(`\+[0-9](?:[ .()-]?[0-9]){5,20}`)
	// rePhoneGrouped matches a separator-grouped 10-11 digit run: an optional leading `1`
	// group, then 3-3-4 with separators.
	rePhoneGrouped = regexp.MustCompile(`(?:1[-. ])?\(?[0-9]{3}\)?[-. ][0-9]{3}[-. ][0-9]{4}`)
)

// emailAllowedDomains are the compiled no-reply and documentation domains. A domain is
// allowed when it EQUALS an entry or is a subdomain of one.
var emailAllowedDomains = []string{
	"users.noreply.github.com",
	"noreply.gitlab.com",
	"example.com",
	"example.org",
	"example.net",
}

// emailAllowedTLDs are the reserved top-level names (RFC 2606 / RFC 6761).
var emailAllowedTLDs = []string{"example", "invalid", "test"}

// emailAllowedExact are single addresses allowed verbatim.
var emailAllowedExact = []string{"noreply@github.com"}

// emailAllowed reports whether addr is on the compiled allow-list or is the no-reply
// address of one of the roster's own bot identities.
func emailAllowed(addr string) bool {
	a := strings.ToLower(addr)
	for _, e := range emailAllowedExact {
		if a == e {
			return true
		}
	}
	at := strings.LastIndexByte(a, '@')
	if at < 0 {
		return false
	}
	domain := a[at+1:]
	for _, d := range emailAllowedDomains {
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	if dot := strings.LastIndexByte(domain, '.'); dot >= 0 {
		tld := domain[dot+1:]
		for _, t := range emailAllowedTLDs {
			if tld == t {
				return true
			}
		}
	}
	for _, b := range EffectiveConfig().BotIdents {
		if b.CommitEmailSpec().Accepts(a) {
			return true
		}
	}
	return false
}

// piiFinding is one personal-data match: its rule id, the 1-based line and the span.
type piiFinding struct {
	rule string
	line int
	span string
}

// personalDataScan returns every personal-data finding in s, refusals and notices alike,
// in text order per rule. It is pure: no I/O beyond the roster accessor emailAllowed reads.
func personalDataScan(s string) []piiFinding {
	var out []piiFinding
	for _, loc := range reEmail.FindAllStringIndex(s, -1) {
		span := s[loc[0]:loc[1]]
		if emailAllowed(span) || isGitRemoteUserinfo(s, loc[0], loc[1]) {
			continue
		}
		out = append(out, piiFinding{rule: RulePIIEmail, line: lineOf(s, loc[0]), span: span})
	}
	var e164 [][]int
	for _, loc := range rePhoneE164.FindAllStringIndex(s, -1) {
		// A `+` glued to a word or number ("1+2", a semver build suffix) is arithmetic or
		// metadata, not a dialling prefix.
		if loc[0] > 0 && isAlnumByte(s[loc[0]-1]) {
			continue
		}
		// A digit straight after the match means the run is longer than the pattern took;
		// such a run is not a telephone number either.
		if loc[1] < len(s) && isAlnumByte(s[loc[1]]) {
			continue
		}
		span := s[loc[0]:loc[1]]
		if n := countDigits(span); n < 8 || n > 15 {
			continue
		}
		e164 = append(e164, loc)
		out = append(out, piiFinding{rule: RulePIIPhone, line: lineOf(s, loc[0]), span: span})
	}
	for _, loc := range rePhoneGrouped.FindAllStringIndex(s, -1) {
		if loc[0] > 0 && (isAlnumByte(s[loc[0]-1]) || s[loc[0]-1] == '+') {
			continue
		}
		if loc[1] < len(s) && isAlnumByte(s[loc[1]]) {
			continue
		}
		if insideAny(loc, e164) {
			continue
		}
		out = append(out, piiFinding{rule: RulePIIPhoneAmbiguous, line: lineOf(s, loc[0]), span: s[loc[0]:loc[1]]})
	}
	return out
}

// isGitRemoteUserinfo reports whether the address-shaped span s[start:end] is the user@host
// half of a git remote rather than an e-mail address: the scp form `user@host:path`, or
// the userinfo of a URL (`scheme://user@host/...`). Both are transport syntax that every
// clone instruction carries, and neither addresses a person.
func isGitRemoteUserinfo(s string, start, end int) bool {
	if end+1 < len(s) && s[end] == ':' {
		c := s[end+1]
		if c == '/' || c == '~' || c == '_' || c == '.' || c == '-' || isAlnumByte(c) {
			return true
		}
	}
	tokStart := start
	for tokStart > 0 && !piiDelimByte(s[tokStart-1]) {
		tokStart--
	}
	return strings.Contains(s[tokStart:start], "://")
}

func piiDelimByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '"' || c == '\'' || c == '(' || c == '<' || c == '`'
}

func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n++
		}
	}
	return n
}

func insideAny(loc []int, spans [][]int) bool {
	for _, sp := range spans {
		if loc[0] < sp[1] && sp[0] < loc[1] {
			return true
		}
	}
	return false
}
