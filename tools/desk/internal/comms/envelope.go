package comms

import (
	"errors"
	"fmt"
	"time"

	"github.com/medici-finance/assay/comms/contract"
)

// envelope.go — the Assay POLICY ADAPTER over the public cellmsg-v1 contract.
//
// The envelope's structure (shape, caps, parse order, identity assertion) lives
// in github.com/medici-finance/assay/comms/contract. This file is the one place
// the Assay vocabulary and lane policy are bolted onto it:
//
//   - the verb and handling-class vocabulary (KnownVerb, KnownClasses) that the
//     contract parser asks about through its Vocabulary port;
//   - ParseEnvelope, which is the contract parse WITH that vocabulary — the entry
//     every existing consumer calls, accepting exactly what it always accepted;
//   - Admit / ACL.AllowEnvelope, which apply the compiled lane ACL to a parsed
//     message, so a structurally valid but lane-forbidden message is refused here
//     and nowhere in the contract.
//
// The names below that are aliases or forwarders carry no behaviour of their own;
// they exist so the consumers (cmd/commsgw, cmd/commsloop, cmd/deskcomms,
// internal/commsqueue) compile unchanged. There is ONE parser: contract's.

// Schema is the one envelope schema this reader accepts.
const Schema = contract.Schema

// Size caps (see contract).
const (
	MaxEnvelopeBytes = contract.MaxEnvelopeBytes
	MaxPayloadBytes  = contract.MaxPayloadBytes
	MaxFieldBytes    = contract.MaxFieldBytes
	MaxRefs          = contract.MaxRefs
	MaxRefBytes      = contract.MaxRefBytes
)

// Typed parse refusals (distinct; see contract).
var (
	ErrOversize     = contract.ErrOversize
	ErrMalformed    = contract.ErrMalformed
	ErrBadSchema    = contract.ErrBadSchema
	ErrAbsentTriple = contract.ErrAbsentTriple
	ErrUnknownVerb  = contract.ErrUnknownVerb
	ErrUnknownClass = contract.ErrUnknownClass
)

// ErrLaneForbidden — the message is structurally valid cellmsg-v1 but the
// compiled lane ACL does not permit its (from, verb, to) tuple. Absent is
// refused: there is no default-allow. This is an Assay POLICY refusal; the
// contract parser never returns it.
var ErrLaneForbidden = errors.New("comms: lane ACL does not permit this (from, verb, to)")

// Wire types (aliases of the contract's).
type (
	SenderID = contract.SenderID
	Lane     = contract.Lane
	Envelope = contract.Envelope
)

// KnownClasses is the deterministic handling-class vocabulary the envelope
// validates against. The (action, class, risk) assignment table that keys on
// these is a follow-up brief's deliverable; this package ships the MINIMAL
// fail-closed set so an envelope carrying an unrecognised class is refused at
// parse rather than routed on a best-effort guess. Extending it is a recorded
// decision, not a silent addition — which is why it is a closed set here and not
// a free string.
var KnownClasses = map[string]bool{
	// routine — an ordinary desk message with no elevated handling requirement.
	"routine": true,
	// sensitive — a message the downstream assign table must route at its
	// conservative tier. Shipping this member (rather than only "routine") keeps
	// the fail-closed direction available to a sender without waiting on the full
	// table.
	"sensitive": true,
}

// KnownClass reports whether class is a recognised handling class.
func KnownClass(class string) bool { return KnownClasses[class] }

// vocabulary is the Assay verb/class vocabulary handed to the contract parser.
type vocabulary struct{}

func (vocabulary) KnownVerb(verb string) bool   { return KnownVerb(verb) }
func (vocabulary) KnownClass(class string) bool { return KnownClass(class) }

// Vocabulary returns the Assay vocabulary as the contract's Vocabulary port, for
// a caller that parses through contract.ParseEnvelopeWith directly.
func Vocabulary() contract.Vocabulary { return vocabulary{} }

// ParseEnvelope parses raw wire bytes into an Envelope, refusing anything that is
// not well-formed cellmsg-v1 or that names a verb or class outside the Assay
// vocabulary. It is the contract's one parser with the Assay vocabulary applied,
// so its accept set is exactly the legacy accept set. It does NOT verify the
// signature or the ACL — those are separate, later decisions (VerifyEnvelope,
// ACL.Allow / Admit).
func ParseEnvelope(raw []byte) (*Envelope, error) {
	return contract.ParseEnvelopeWith(raw, vocabulary{})
}

// VerifyEnvelope authenticates a parsed envelope against its assertion; see
// contract.VerifyEnvelope.
func VerifyEnvelope(e *Envelope, now time.Time, trust TrustStore, skew time.Duration, replay ReplayGuard) error {
	return contract.VerifyEnvelope(e, now, trust, skew, replay)
}

// AllowEnvelope reports whether the ACL permits the envelope's lane tuple. It is
// Allow keyed on a parsed message, so a caller cannot pass a verb from one
// message and a lane from another.
func (a *ACL) AllowEnvelope(e *Envelope) bool {
	if e == nil {
		return false
	}
	return a.Allow(e.From.Cell, e.From.Role, e.Verb, e.To.Cell, e.To.Role)
}

// Admit is the policy adapter entry: parse raw as cellmsg-v1 with the Assay
// vocabulary, then require the lane ACL to permit it. It returns the parsed
// envelope only when BOTH hold; a structurally valid message the ACL forbids
// returns ErrLaneForbidden. Admit does not verify the assertion — that is
// VerifyEnvelope, which a gateway runs before the ACL.
func Admit(raw []byte, acl *ACL) (*Envelope, error) {
	e, err := ParseEnvelope(raw)
	if err != nil {
		return nil, err
	}
	if !acl.AllowEnvelope(e) {
		return nil, fmt.Errorf("%w: %s/%s -> %s/%s verb %q",
			ErrLaneForbidden, e.From.Cell, e.From.Role, e.To.Cell, e.To.Role, e.Verb)
	}
	return e, nil
}
