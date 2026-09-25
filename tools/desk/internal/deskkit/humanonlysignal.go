package deskkit

import "strings"

// humanonlysignal.go — the R-3 keyword lists classification consumes, shared by deskdigest's
// classifier and deskfile's fork-test notice-lane gate (noticelane.go). Before this file both
// lists lived unexported in cmd/deskdigest/classify.go; deskfile's `new` needs the SAME lists
// — the notice lane admits exactly the items the digest would classify reversible, and a copy
// would drift the moment either side changed. Moved here so both consult one definition.
//
// These two lists are deskdigest's DISPLAY classifier, where a miss only means a row shows as
// "unclassified" and the item keeps its label. deskfile's gate, where a miss would take an
// item OFF the driver's queue, does not rely on HumanOnlySignals alone: it adds the broader
// fail-closed OneWay check (noticelane.go) and admits the notice lane only on a POSITIVE
// ReversibleSignals match.

// Signal is one substring matcher plus the R-3 category it evidences. The needle is matched
// case-insensitively; callers lower-case the haystack before calling FirstHumanOnlySignal (or
// use MatchesHumanOnlySignal, which does it for them).
type Signal struct {
	Needle   string
	Category string
}

// HumanOnlySignals are R-3's human-only side: irreversible, mechanism, security, spend. Each
// entry is a substring matched case-insensitively against title+body. The list is
// deliberately BROADER than any "reversible" list: a false human-only costs one item staying
// in a human's queue that could have left it, and a false reversible costs a decision taken
// without them. Those are not the same mistake. The mechanism entries carry "human gate" /
// "gate question" / "gate: human", NOT a bare "gate": measured against the live queue on
// 2026-08-13, the bare needle classified 4 of 8 items `mechanism` on incidental uses ("the
// release guard gate") — a confident wrong REASON — so the narrower phrases are what ship.
// (deskfile's fail-closed OneWayPatterns, where a miss costs more, add the unspaced
// `gate:human` spelling; see noticelane.go.)
var HumanOnlySignals = []Signal{
	// irreversible
	{"irreversible", "irreversible"},
	{"cannot be undone", "irreversible"},
	{"one-way door", "irreversible"},
	{"delete the", "irreversible"},
	{"force-push", "irreversible"},
	{"publish", "irreversible (publication is not recallable)"},
	{"public repo", "irreversible (publication is not recallable)"},
	{"make it public", "irreversible (publication is not recallable)"},
	{"open source", "irreversible (publication is not recallable)"},
	{"rewrite history", "irreversible"},
	// mechanism
	{"mechanism", "mechanism"},
	{"who may", "mechanism (authority boundary)"},
	{"authority", "mechanism (authority boundary)"},
	{"delegat", "mechanism (authority boundary)"},
	{"merge to main", "mechanism"},
	{"auto-merge", "mechanism"},
	{"ruling", "mechanism"},
	{"human gate", "mechanism"},
	{"gate question", "mechanism"},
	{"gate: human", "mechanism"},
	// security
	{"security", "security"},
	{"token", "security"},
	{"credential", "security"},
	{"secret", "security"},
	{"permission", "security"},
	{"sops", "security"},
	{"private key", "security"},
	{"rotation", "security"},
	{"scope", "security"},
	// spend
	{"spend", "spend"},
	{"cost", "spend"},
	{"billing", "spend"},
	{"licence", "spend"},
	{"license", "spend"},
	{"subscription", "spend"},
	{"budget", "spend"},
	// production blast radius — an irreversible-in-practice class
	{"production", "irreversible in practice (production)"},
	{"prod deploy", "irreversible in practice (production)"},
	{"on-chain", "irreversible in practice (on-chain)"},
	{"mainnet", "irreversible in practice (on-chain)"},
}

// FirstHumanOnlySignal returns the first HumanOnlySignals entry whose needle occurs in hay,
// or nil. hay must already be lower-cased by the caller (both consumers match it against
// title+body they have already folded to lower case for their own other checks, so this
// stays a pure string-contains scan with no hidden case-folding cost).
func FirstHumanOnlySignal(hay string) *Signal {
	for i := range HumanOnlySignals {
		if strings.Contains(hay, HumanOnlySignals[i].Needle) {
			return &HumanOnlySignals[i]
		}
	}
	return nil
}

// MatchesHumanOnlySignal is FirstHumanOnlySignal over un-folded text: it lower-cases hay
// itself, for a caller that has not already folded it. Returns the matched Signal and true,
// or (nil, false).
func MatchesHumanOnlySignal(hay string) (*Signal, bool) {
	if s := FirstHumanOnlySignal(strings.ToLower(hay)); s != nil {
		return s, true
	}
	return nil, false
}

// ReversibleSignals are R-3's own four examples of a one-commit reversal and their immediate
// neighbours. Nothing is added here on a hunch: the list stays close to the ruling's text
// because widening it silently widens what the desk may decide without asking — and, since
// deskfile's notice lane is admitted ONLY on a match here, what may leave the driver's queue.
var ReversibleSignals = []Signal{
	{"docs wording", "docs wording (R-3 example)"},
	{"wording", "docs wording (R-3 example)"},
	{"typo", "docs wording (R-3 example)"},
	{"phrasing", "docs wording (R-3 example)"},
	{"lint level", "lint level (R-3 example)"},
	{"lint severity", "lint level (R-3 example)"},
	{"notice or error", "lint level (R-3 example)"},
	{"port-or-drop", "port-or-drop (R-3 example)"},
	{"port or drop", "port-or-drop (R-3 example)"},
	{"tool default", "tool default (R-3 example)"},
	{"default value", "tool default (R-3 example)"},
	{"flag default", "tool default (R-3 example)"},
	{"rename the", "one-commit reversal (rename)"},
	{"table column", "one-commit reversal (report layout)"},
}

// FirstReversibleSignal returns the first ReversibleSignals entry whose needle occurs in hay,
// or nil. hay must already be lower-cased by the caller, as for FirstHumanOnlySignal.
func FirstReversibleSignal(hay string) *Signal {
	for i := range ReversibleSignals {
		if strings.Contains(hay, ReversibleSignals[i].Needle) {
			return &ReversibleSignals[i]
		}
	}
	return nil
}
