package deskkit

import "strings"

// humanonlysignal.go — the ONE human-only keyword list R-3 classification consumes, shared
// by deskdigest's classifier and deskfile's fork-test one-way override
// (attention-budget/13). Before this file the list lived unexported in
// cmd/deskdigest/classify.go; deskfile's `new` needed the SAME list — a filer must never be
// able to talk a one-way item onto the notice lane by claiming a catching gate — and a copy
// would drift the moment either side's list changed. Moved here so both consult exactly one
// definition.

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
// without them. Those are not the same mistake — see deskdigest's classify.go for the fuller
// discussion of "human gate" vs bare "gate" (assay-toolkit's 2026-08-13 measurement of 4/8
// items misclassified motivated the narrower phrase there; unchanged here).
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
