package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rulings.go — reports R-3's sign-off standing, and delegates the PARSING to the house's
// one reader so the digest cannot answer that question differently from every other gate.
//
// A rulings register records process authority no agent holds. deskkit.ReadSignOff is the
// single parser for "what does the register CLAIM about this ruling". deskclose reads it to
// ACT — it fetches the artifact and verifies its author before closing anything — and
// deskdigest reads it to REPORT — it prints the claim and never fetches. Those are two
// different verify steps stacked on ONE parse, not a licence for a second parser. A local
// reader that answered the sign-off question its own way would be the second answer the
// shared file forbids, and the two would drift silently in the dangerous direction: the
// shared reader is strictly more careful — a truncated block, two sign-off blocks in one
// section, or two URLs in one block are each could-not-read rather than a false positive —
// and a hand-rolled copy re-opens each of those as a wrong answer, which is the exact
// class of failure ("reported a signed register as unsigned") this command was sent back
// to fix.
//
// The three local states map one-to-one onto the shared reader's three. The local
// vocabulary is kept — `claimed` rather than `granted` — because it is the more honest
// word for what a REPORTING tool knows: the file claims a sign-off and deskdigest has not
// verified it. That is a naming choice at this mapping layer, not a re-parse.

const defaultRulingsPath = "docs/streams/issue-flow/rulings.md"

// r3ID is the ruling deskdigest reports on: the reversible-decision default whose
// records and veto window this digest IS.
const r3ID = "R-3"

type signOffState string

const (
	signOffUnsigned signOffState = "unsigned"
	signOffClaimed  signOffState = "claimed"
	signOffUnknown  signOffState = "could-not-read"
)

type signOff struct {
	State signOffState
	URL   string
	Note  string
}

// readR3SignOff reports R-3's sign-off standing by delegating the parse to the house
// reader (deskkit.ReadSignOff) and mapping its three states onto the digest's own trio.
func readR3SignOff(path string) signOff {
	r := deskkit.ReadSignOff(path, r3ID)
	switch r.State {
	case deskkit.SignOffGranted:
		return signOff{State: signOffClaimed, URL: r.URL,
			Note: "the register records a sign-off URL on file. deskdigest does NOT fetch or verify it — " +
				"it reports; the tool that acts on " + r3ID + " is the one that must verify the artifact's author."}
	case deskkit.SignOffUnsigned:
		return signOff{State: signOffUnsigned,
			Note: r3ID + " carries no sign-off URL, so the desk holds NO delegated decision authority. " +
				"Every row below is still yours, whatever its classification column says: the classification " +
				"is a proposal for what could be delegated once " + r3ID + " is signed, not a claim that it has been."}
	default: // deskkit.SignOffCouldNotRead — reported verbatim, never collapsed to a finding about the human.
		return signOff{State: signOffUnknown, Note: r.Detail}
	}
}
