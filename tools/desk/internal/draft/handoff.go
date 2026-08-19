package draft

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// THE HAND-OFF SURFACE
// --------------------
// §7.5 phase 2 says a draft is handed to the human commit path: "review in
// inbox" → the human confirms and commits. That path does not exist in any
// tree reachable from here (measured; see Present below), and the honest
// consequence is that this layer's hand-off is a REFUSAL surface, not a
// delivery one.
//
// That is not a placeholder. It is the design. A hand-off that delivers is a
// write path, and a write path that is merely dormant is one commit from being
// live. So the register below enumerates every target a hand-off could name,
// says for each which identity a transmission through it would carry and what
// authority that transmission would require — and [Draft.HandOff] refuses
// every transmitting one UNCONDITIONALLY.
//
// ORDER OF CHECKS IS LOAD-BEARING
// -------------------------------
// Transmission is checked BEFORE existence. If existence were checked first,
// a target becoming real would move it from "refused because absent" to
// "permitted", and the whole guarantee would rest on a substrate staying
// missing. Refusal here does not depend on anything staying missing.
//
// WHY THIS FILE NAMES THE POSTING TOOLS IN PROSE
// ----------------------------------------------
// A neighbouring read-only package forbids the posting tools' NAMES in its own
// source. That is the right check there and the wrong one here: a register
// whose job is to say "a hand-off through this route would post as the worker
// role" cannot do its job while forbidden from naming the route. Naming is
// safe here for a structural reason rather than a stylistic one — this package
// imports nothing that can execute, connect or write (see the package header,
// rule 1), so a tool name in it is text and can never be a call.

// The hand-off refusal classes.
var (
	// ErrNoAutoPost is the invariant. It is returned for every destination
	// that would transmit, always, with no input that changes it.
	ErrNoAutoPost = errors.New("refused: this layer never posts — a draft's only exit is a human")

	// ErrNoSubstrate is could-not-check, not failure: the destination is
	// non-transmitting and legitimate, but the thing that would receive the
	// draft does not exist in this tree.
	ErrNoSubstrate = errors.New("could-not-check: the hand-off target does not exist in this tree")

	// ErrUnknownDestination is default-deny. An ID that is not in the register
	// is refused rather than treated as a pass-through.
	ErrUnknownDestination = errors.New("refused: undeclared hand-off destination")
)

// Destination is one place a draft could be handed to. Every field is a
// statement a reviewer can check, and the register is closed.
type Destination struct {
	// ID is the register key.
	ID string
	// What the destination is, in the operator's words.
	What string
	// Substrate is what would have to exist for this destination to receive
	// anything, named concretely enough to look for.
	Substrate string
	// Present records whether that substrate exists in THIS tree. It is a
	// measurement, restated by TestAuthorityPresentFlagsMatchTheTree, and it
	// grants nothing: see the order-of-checks note above.
	Present bool
	// Transmits records whether reaching this destination would send anything
	// anywhere. Every true here is an unconditional refusal.
	Transmits bool
	// Identity is the identity a transmission through this destination would
	// carry. Tool identity in this suite is NOT uniform, and a hand-off that
	// does not say which identity it would wear is a hand-off nobody can
	// review.
	Identity string
	// Authority is what would have to authorize a transmission, and from whom.
	Authority string
	// Note is why this destination is in the state it is.
	Note string
}

// destinations is the closed register. Exactly one entry does not transmit;
// it is the only supported exit, and it is the one that ends at a human's
// eyes.
var destinations = []Destination{
	{
		ID:        "operator-render",
		What:      "show the draft to the operator reading the pane",
		Substrate: "none beyond this package — Draft.Render produces the text",
		Present:   true,
		Transmits: false,
		Identity:  "none — nothing leaves the process. The operator is the transport",
		Authority: "none required: reading is not acting",
		Note:      "THE ONLY SUPPORTED EXIT. §6.2 in one sentence — an agent may propose, only the human commits",
	},
	{
		ID:        "inbox-answer-commit",
		What:      `the "review in inbox" hand-off: the human answer/commit path, pre-filled`,
		Substrate: "the console web shell and its two-step commit gate (§7.6), owned by the console shell",
		Present:   false,
		Transmits: true,
		Identity:  "the human-authority credential named in design §6.4 — held by the desktop app only, never by a loop, a worker or a server",
		Authority: "NONE EXISTS. The five standing rulings R-1..R-5 (signed) cover delegated close lanes, label taxonomy, the reversible-decision default, drain-weighting and desk merge-currency. Not one of them grants an unattended post. A hand-off that transmits would need a NEW ruling accepted by a human in a comment whose URL lands on a Sign-off line",
		Note:      "this is the destination the design names. It is refused for transmitting, and separately its substrate is absent: the console shell is not built",
	},
	{
		ID:        "priority-steering-write",
		What:      "pre-fill the Priority pane's steering write with a proposed reorder",
		Substrate: "the steering write path over the Roadmap render, owned by the console shell",
		Present:   false,
		Transmits: true,
		Identity:  "the human-authority credential named in design §6.4 — steering mutates sources, so it is the human's own identity or it is nothing",
		Authority: "NONE EXISTS — same finding as inbox-answer-commit. A priority change is a source mutation, so its bar is at least as high",
		Note:      "refused for transmitting; substrate absent, the owning shell is not built",
	},
	{
		ID:        "issue-comment-reply",
		What:      "reply on the escalation issue directly, skipping the human",
		Substrate: "the suite's reply verb at cmd/deskreply — MEASURED PRESENT in this module",
		Present:   true,
		Transmits: true,
		Identity:  "the WORKER App role. cmd/deskreply/exec.go mints its credential with the worker role; the concrete bot login is resolved from roster config at run time, so it is not fixed in source",
		Authority: "NONE EXISTS. This is the destination the invariant is actually about: the tool is present, it works, and a single line here would use it",
		Note:      "REFUSED. Present is true, and it changes nothing — the transmit check runs first precisely so that a working tool nearby cannot become an exit",
	},
	{
		ID:        "pr-comment",
		What:      "post the draft as a comment on the pull request under discussion",
		Substrate: "the suite's comment verb at cmd/deskpost — MEASURED PRESENT in this module",
		Present:   true,
		Transmits: true,
		Identity:  "the REVIEWER App role — a DIFFERENT identity from the reply route above. cmd/deskpost/github.go resolves the reviewer role; the concrete bot login comes from roster config",
		Authority: "NONE EXISTS. And note the identity hazard: two hand-off routes, two different App identities, so 'the desk said so' is not one claim but two with different standing",
		Note:      "REFUSED. Recorded because a drafting layer that hides the identity split invites a reader to treat the two routes as interchangeable",
	},
	{
		ID:        "chat-transcript-persist",
		What:      "keep the composed draft across navigation and failure (§7.6 draft-persistence)",
		Substrate: "a store owned by the console shell. This package supplies the round-trippable FORM — see Encode/Decode — and holds no store, because it cannot write one",
		Present:   false,
		Transmits: false,
		Identity:  "none — a store is not an identity. Whatever holds the bytes inherits the shell's",
		Authority: "none required for the form. The store's own access rules are the shell's to state",
		Note:      "NOT a refusal for transmitting — this destination is legitimate and simply has nowhere to land yet. Encode/Decode is the half that could be built and falsified here",
	},
}

// Destinations returns the register, ID-sorted, as a copy.
func Destinations() []Destination {
	out := append([]Destination(nil), destinations...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LookupDestination returns a declared destination.
func LookupDestination(id string) (Destination, bool) {
	for _, d := range destinations {
		if d.ID == id {
			return d, true
		}
	}
	return Destination{}, false
}

// AuthorityRequired reports what would have to authorize a transmission
// through a destination, and from whom. It is a question this package answers
// and never acts on: there is no type here that can hold a granted authority,
// deliberately, because an authority token plus a transmitting code path is a
// post waiting for a flag flip.
func AuthorityRequired(id string) (string, bool) {
	d, ok := LookupDestination(id)
	if !ok {
		return "", false
	}
	return d.Authority, true
}

// Packet is what a permitted hand-off yields: inert text and the destination
// it was prepared for. It has no method that transmits, and
// TestAuthorityNoTransmitShapedExportedAPI holds that true against the whole
// package rather than against this type alone.
type Packet struct {
	// Destination is the register ID this packet was prepared for.
	Destination string
	// Body is the rendered draft. It is a string. Moving it anywhere is the
	// caller's act, under the caller's identity.
	Body string
}

// Zero reports whether this is the empty packet — what every refused hand-off
// returns, so that a caller which drops the error is left holding nothing
// rather than something plausible.
func (p Packet) Zero() bool { return p.Destination == "" && p.Body == "" }

// HandOff prepares a draft for a declared destination, or refuses.
//
// The check order is the guarantee:
//
//  1. an undeclared destination is REFUSED (default-deny);
//  2. a TRANSMITTING destination is REFUSED, unconditionally, before anything
//     else is considered — presence, kind, content and caller are all
//     irrelevant to this branch, and there is no parameter, field or build tag
//     that reaches past it;
//  3. an absent substrate is COULD-NOT-CHECK;
//  4. what is left is rendered into an inert packet.
//
// A refusal always returns the zero [Packet].
func (d Draft) HandOff(destID string) (Packet, error) {
	dest, ok := LookupDestination(strings.TrimSpace(destID))
	if !ok {
		return Packet{}, fmt.Errorf("%w: %q is not in the hand-off register (%s)", ErrUnknownDestination, destID, declaredIDs())
	}
	if dest.Transmits {
		return Packet{}, fmt.Errorf("%w. Destination %s would transmit as: %s. Authority: %s",
			ErrNoAutoPost, dest.ID, dest.Identity, dest.Authority)
	}
	if !dest.Present {
		return Packet{}, fmt.Errorf("%w: %s needs %s", ErrNoSubstrate, dest.ID, dest.Substrate)
	}
	if d.Zero() {
		return Packet{}, fmt.Errorf("%w: there is no draft to hand off", ErrMalformed)
	}
	return Packet{Destination: dest.ID, Body: d.Render()}, nil
}

func declaredIDs() string {
	ids := make([]string, 0, len(destinations))
	for _, d := range destinations {
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	return strings.Join(ids, " ")
}

// UnsupportedTargets reports every hand-off target this layer does not
// support, with the reason, so that "no silent caps" holds for the hand-off
// surface too: a caller is told what it cannot do, by name, rather than
// discovering it as an empty result.
func UnsupportedTargets() []string {
	var out []string
	for _, d := range Destinations() {
		switch {
		case d.Transmits:
			out = append(out, fmt.Sprintf("%s — REFUSED, would transmit as %s; authority: %s", d.ID, d.Identity, d.Authority))
		case !d.Present:
			out = append(out, fmt.Sprintf("%s — COULD-NOT-CHECK, substrate absent: %s", d.ID, d.Substrate))
		}
	}
	return out
}
