package deskkit

// raisedby.go — the `raised-by:<role>` provenance stamp.
//
// WHAT THIS IS. When a desk files an issue, nothing on the issue records WHICH desk
// noticed the problem. The author is whatever `gh` credential the filer happened to be
// holding, which answers "which App posted" and not "which loop found it" — those are
// different questions, and only the second one tells you whether one loop is generating
// all the churn. This file declares the label that answers the second question, and the
// READER contract every consumer of it must use.
//
// ONE DECLARED SOURCE (derive-or-diff). The role vocabulary is NOT a list written down
// here. It is DERIVED from the roster's role-bindings — the `role=` prefixes on
// ASSAY_TRUSTED_BOT_SLUGS, which is already the single place a desk role exists at all
// (rosterconfig.go, Config.RoleBots; RoleAppLogin reads the same map to decide whose
// action counts). A second hand-maintained list of desk names would drift from that one
// the first time a role was added, and the drift would be silent: the metric would keep
// reporting confidently against a vocabulary nobody was emitting. So there is no list
// here to drift — RaisedByRoles() is a projection of the roster, and the label vocabulary
// is a pure function of it.
//
// The consequence is worth stating plainly, because it changes the strings: the roles are
// the ROSTER's role names (`reviewer`, `verifier`, `worker`, `issue-loop`, `intake-loop`,
// `desk`), not the SKILL file names (`pr-review-desk`, `verify-desk`, `worker-desk`,
// `intake-desk`). The skill names are a second naming of the same six things, and they
// are hand-maintained prose. Deriving from the roster is what makes the vocabulary
// checkable at all.
//
// THREE-STATE, and it is the load-bearing part. A metric over this
// label has THREE answers per issue, never two:
//
//	RaisedByStamped        a `raised-by:<role>` label is present and names one role.
//	RaisedByUnknown        NO stamp. This is the ZERO VALUE, deliberately.
//	RaisedByIndeterminate  the issue carries CONFLICTING provenance — two or more
//	                       `raised-by:` labels, or one with an empty role.
//
// UNKNOWN IS NOT "HUMAN-RAISED". An unstamped issue is an issue whose provenance was
// never recorded; a human-raised issue is a claim about who raised it. A reader that
// collapses the first into the second produces a confident wrong number — and it produces
// it in the direction that flatters the machine's least flattering metric, because the
// entire standing corpus is unstamped (421 of 421 issues on the home repo at the time
// this shipped) and would land in whichever bucket the default chose. There is no default
// here to choose. The zero value is Unknown, so a consumer that forgets to handle the
// state gets the non-answer rather than an answer.
//
// SAME RULE FOR INDETERMINATE. Two `raised-by:` labels is not "pick the first one" and it
// is not "pick the alphabetically smallest". It is a could-not-check: the issue asserts
// two different origins and this package cannot adjudicate between them.
//
// CONSUMER BOUNDARY. The self-improvement metric is the reader
// this exists for, and it is NOT implemented here. What that reader needs from this file is: the
// stamp, its vocabulary, and the two non-answer states as first-class values it can
// report separately rather than fold into its denominator. Everything past that — the
// self-healed/human-touched classification, the rate, the series — is the reader's.
//
// READING A ROLE THE ROSTER NO LONGER BINDS. RaisedByOf does NOT validate the role
// against the current roster. A stamp is a historical fact about a filing; re-validating
// it at read time would silently turn every past issue from a retired role into Unknown
// the moment the roster changed, which is a rewrite of history disguised as a lookup. The
// WRITE path validates (RaisedByLabel); the read path reports what is there.

import (
	"sort"
	"strings"
)

// RaisedByPrefix is the label prefix carrying the provenance stamp. It is the ONE spelling
// of it: writers build labels through RaisedByLabel and readers match through RaisedByOf,
// so no consumer needs to restate the string.
const RaisedByPrefix = "raised-by:"

// RaisedByState is the three-state answer to "which desk raised this issue?".
type RaisedByState int

const (
	// RaisedByUnknown is the ZERO VALUE and means NO stamp was found. It is a
	// non-answer, never a bucket: see the file comment. A consumer that treats it as
	// "human-raised" is asserting something the data does not say.
	RaisedByUnknown RaisedByState = iota
	// RaisedByStamped means exactly one `raised-by:<role>` label was present and it
	// named a non-empty role.
	RaisedByStamped
	// RaisedByIndeterminate is the could-not-check state: two or more conflicting
	// `raised-by:` labels, or one whose role half is empty. The issue asserts a
	// provenance this package cannot resolve to one role.
	RaisedByIndeterminate
)

func (s RaisedByState) String() string {
	switch s {
	case RaisedByStamped:
		return "stamped"
	case RaisedByIndeterminate:
		return "indeterminate"
	default:
		return "unknown"
	}
}

// Answered reports whether the state names a role. It exists so a consumer writes
// `if !state.Answered()` rather than comparing against one of the two non-answers and
// forgetting the other — the enumeration that omits Indeterminate is the bug this
// prevents.
func (s RaisedByState) Answered() bool { return s == RaisedByStamped }

// RaisedByRoles returns the desk roles the roster BINDS, lowercased and sorted. It is the
// whole vocabulary: a role with no `role=` binding on ASSAY_TRUSTED_BOT_SLUGS is not a
// stampable desk, because nothing else in the tools would recognise its identity either.
//
// An UNCONFIGURED roster returns nil, not a fallback set. There is no compiled-in default
// vocabulary anywhere in this file, on purpose: a shipped default would be exactly the
// second copy the derive-or-diff rule exists to prevent, and it would let a tool stamp a
// role the deployment does not have.
func RaisedByRoles() []string {
	c := EffectiveConfig()
	if !c.Configured() {
		return nil
	}
	roles := make([]string, 0, len(c.RoleBots))
	for role, slug := range c.RoleBots {
		if strings.TrimSpace(role) == "" || strings.TrimSpace(slug) == "" {
			continue
		}
		roles = append(roles, strings.ToLower(strings.TrimSpace(role)))
	}
	sort.Strings(roles)
	return roles
}

// RaisedByLabel validates role against the roster and returns the label to apply. It is
// the ONLY admissible way to construct a stamp: a caller that formats the string itself
// can stamp a role that does not exist, and a metric grouping by a role nobody is bound to
// reports a category with one member forever.
//
// The error names the enumerated vocabulary, because "unknown role" without the list is a
// refusal the caller cannot act on.
func RaisedByLabel(role string) (string, error) {
	r, err := boundRole(role)
	if err != nil {
		return "", err
	}
	return RaisedByPrefix + r, nil
}

// boundRole is the SHARED desk-role resolver behind BOTH the `--raised-by` provenance
// flag (RaisedByLabel) and the `--to` addressee flag (AddressedToLabel, addressto.go):
// one resolver, two label prefixes. It validates role against the roster's DERIVED
// vocabulary (RaisedByRoles — the `role=` prefixes on ASSAY_TRUSTED_BOT_SLUGS) and
// returns the canonical lowercased role, or a typed refusal that names the bound set so
// a caller has the fix in hand. Keeping the vocabulary in ONE place is why `--to` cannot
// address a role `--raised-by` could not stamp, and vice versa.
func boundRole(role string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(role))
	if want == "" {
		return "", Refused("desk role: an empty role is not a stamp — " +
			"omit the flag entirely if the provenance/addressee is genuinely unknown, so nothing " +
			"is recorded rather than a blank role")
	}
	roles := RaisedByRoles()
	if len(roles) == 0 {
		return "", Refused("desk role: no desk role is bound in the roster, " +
			"so there is no vocabulary to resolve from. Bind roles with the `role=` prefix on " +
			EnvTrustedBotSlugs + " (e.g. `reviewer=<slug>:<id>`); an unconfigured roster resolves nothing")
	}
	for _, r := range roles {
		if r == want {
			return r, nil
		}
	}
	return "", Refused("desk role: " + want +
		" is not a desk role bound in the roster. Bound roles: " + strings.Join(roles, ", ") +
		". The vocabulary is DERIVED from " + EnvTrustedBotSlugs + "'s `role=` prefixes — add the " +
		"binding there rather than inventing a label, or the metric/inbox groups by a role nothing emits")
}

// RaisedByOf is the READER contract: given an issue's label names, answer which desk
// raised it, in three states. See the file comment for why the non-answers are two
// distinct values and why neither is a bucket.
//
// The role is returned lowercased and trimmed, verbatim from the label — it is NOT
// re-validated against the current roster (a stamp is a historical fact; see the file
// comment). role is "" for every state except RaisedByStamped.
func RaisedByOf(labels []string) (role string, state RaisedByState) {
	return stampOf(labels, RaisedByPrefix)
}

// stampOf is the SHARED three-state reader behind RaisedByOf and AddressedToOf
// (addressto.go): given an issue's label names and a stamp PREFIX, it answers which
// role the stamp names, in the three states RaisedByState enumerates. The non-answers
// (Unknown / Indeterminate) are first-class here for the SAME reason they are on the
// raised-by metric: a `to:` inbox that collapses "no addressee" and "two conflicting
// addressees" into one bucket routes on a guess.
func stampOf(labels []string, prefix string) (role string, state RaisedByState) {
	found := ""
	count := 0
	empty := false
	for _, l := range labels {
		name := strings.ToLower(strings.TrimSpace(l))
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		count++
		r := strings.TrimSpace(strings.TrimPrefix(name, prefix))
		if r == "" {
			empty = true
			continue
		}
		if found != "" && found != r {
			// Two DIFFERENT roles: conflicting provenance/addressee, adjudicating is not
			// this package's call.
			return "", RaisedByIndeterminate
		}
		found = r
	}
	switch {
	case count == 0:
		return "", RaisedByUnknown
	case empty:
		// A stamp with no role half is malformed. It is not Unknown — something DID try
		// to record a role and produced an unreadable answer, and those two need
		// different remedies.
		return "", RaisedByIndeterminate
	case found == "":
		return "", RaisedByIndeterminate
	default:
		// count may exceed 1 here only when every stamp named the SAME role (a
		// duplicate label is not a conflict).
		return found, RaisedByStamped
	}
}
