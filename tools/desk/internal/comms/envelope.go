package comms

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Schema is the one envelope schema this reader accepts. A payload declaring
// anything else is REFUSED rather than read on a best-effort basis: a reader that
// guesses at an unknown schema is how a silently-changed meaning ships. It
// mirrors topology.Schema's fail-closed posture.
const Schema = "cellmsg-v1"

// Size caps. The whole point of a typed, capped envelope is that the gateway
// parses one bounded, well-known shape — an unbounded body is an unbounded parse,
// which is an amplification surface. The caps are deliberately generous for real
// desk messages and far below anything that would strain the gateway; they are a
// floor against abuse, not a tuning knob.
const (
	// MaxEnvelopeBytes bounds the raw wire payload before any parse is attempted.
	MaxEnvelopeBytes = 64 * 1024
	// MaxPayloadBytes bounds the structured body carried inside the envelope.
	MaxPayloadBytes = 16 * 1024
	// MaxFieldBytes bounds every scalar string field (id, cell, role, verb, …).
	MaxFieldBytes = 256
	// MaxRefs bounds how many cross-references one message may carry.
	MaxRefs = 32
	// MaxRefBytes bounds each individual ref string.
	MaxRefBytes = 256
)

// Typed refusal errors. Each is DISTINCT so a caller (and the refusal battery)
// can tell them apart with errors.Is — a single "bad envelope" error would make
// "it was too big" indistinguishable from "it named an unknown verb", and the
// downstream sweep needs to know which.
var (
	// ErrOversize — the raw payload, the structured body, or a field exceeds its
	// size cap. Checked on the raw bytes BEFORE unmarshalling, so an oversize
	// payload is never parsed.
	ErrOversize = errors.New("comms: envelope exceeds a size cap")
	// ErrMalformed — the payload is not well-formed cellmsg-v1 JSON (bad JSON, an
	// unknown field, a wrong-typed field). Refused before any field is trusted.
	ErrMalformed = errors.New("comms: payload does not parse as cellmsg-v1")
	// ErrBadSchema — the payload parses but declares a schema this reader does not
	// accept.
	ErrBadSchema = errors.New("comms: unrecognised envelope schema")
	// ErrAbsentTriple — the message is missing part of the {from, to, verb} triple
	// that every routable message must carry. Absent addressing is refused, never
	// defaulted.
	ErrAbsentTriple = errors.New("comms: envelope is missing part of the required {from, to, verb} triple")
	// ErrUnknownVerb — the verb is not a member of the compiled lane-ACL
	// vocabulary. An unknown verb is refused at parse, never routed on a guess.
	ErrUnknownVerb = errors.New("comms: verb is not in the lane-ACL vocabulary")
	// ErrUnknownClass — the class is not a recognised handling class.
	ErrUnknownClass = errors.New("comms: class is not a recognised handling class")
)

// SenderID is the identity of a message's sender. Its {cell, role} are the
// AUTHENTICATED identity: they are covered by the signed assertion (identity.go),
// and VerifyEnvelope refuses any envelope whose declared From.{Cell,Role}
// disagrees with its verified assertion — so {cell, role} are never self-claimed.
// App and Session, by contrast, are NOT in the signed canonical bytes: they are
// advisory metadata (the app/session the token was minted for) and are forgeable
// within a legitimate cell+role. The lane ACL keys only on {cell, role, verb}, so
// this does not weaken the boundary — but do NOT attribute trust or audit to
// From.App / From.Session until they are bound into the signed bytes.
type SenderID struct {
	Cell    string `json:"cell"`
	Role    string `json:"role"`
	App     string `json:"app,omitempty"`
	Session string `json:"session,omitempty"`
}

// Lane is a destination: a {cell, role} inbox. `to` addresses a LANE, never a
// session — a session is a transient process that may have exited by delivery
// time, while a lane (a role's inbox at a cell) is the durable, ACL-checkable
// unit the lane matrix keys on.
type Lane struct {
	Cell string `json:"cell"`
	Role string `json:"role"`
}

// Envelope is the parsed cellmsg-v1 message. It is the in-memory result of a
// successful ParseEnvelope; a partially-populated Envelope is never returned
// alongside an error.
type Envelope struct {
	Schema  string          `json:"schema"`
	ID      string          `json:"id"`
	Cell    string          `json:"cell"`
	From    SenderID        `json:"from"`
	To      Lane            `json:"to"`
	Verb    string          `json:"verb"`
	Class   string          `json:"class"`
	Refs    []string        `json:"refs,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Sent    time.Time       `json:"sent"`
	// Sig is the signed identity assertion over this message (§4). It is the
	// per-desk signature that proves From; VerifyEnvelope checks it before any
	// other trust decision.
	Sig Assertion `json:"sig"`
}

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

// ParseEnvelope parses raw wire bytes into an Envelope, refusing anything that is
// not well-formed cellmsg-v1. It is PARSE-OR-REFUSE: every failure is a typed
// error and no partially-populated Envelope is ever returned with one. The order
// is deliberate — cheapest, earliest refusals first:
//
//  1. size cap on the raw bytes (an oversize payload is never unmarshalled);
//  2. strict JSON decode with unknown fields disallowed (ErrMalformed);
//  3. schema match (ErrBadSchema);
//  4. the required {from, to, verb} triple is present (ErrAbsentTriple);
//  5. per-field size caps, ref caps, payload cap (ErrOversize);
//  6. verb is in the lane-ACL vocabulary, class is a known class
//     (ErrUnknownVerb / ErrUnknownClass).
//
// It does NOT verify the signature or the ACL — those are separate, later
// decisions (VerifyEnvelope, ACL.Allow). This function answers only "is this a
// well-formed cellmsg-v1 message at all"; a caller that treats a parse success as
// an authenticated or authorised message has skipped the two checks that matter.
func ParseEnvelope(raw []byte) (*Envelope, error) {
	if len(raw) > MaxEnvelopeBytes {
		return nil, fmt.Errorf("%w: payload is %d bytes, cap is %d", ErrOversize, len(raw), MaxEnvelopeBytes)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var e Envelope
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	// Reject trailing content after the first JSON value: a payload that is a
	// valid envelope followed by junk is not a valid envelope.
	if dec.More() {
		return nil, fmt.Errorf("%w: trailing content after the envelope", ErrMalformed)
	}

	if e.Schema != Schema {
		return nil, fmt.Errorf("%w: %q is not the %q this reader accepts", ErrBadSchema, e.Schema, Schema)
	}

	// The required triple. Absent addressing is refused, never defaulted to a
	// least-authority lane — a message with no stated sender or destination is not
	// a low-privilege message, it is an unroutable one.
	switch {
	case e.From.Cell == "" || e.From.Role == "":
		return nil, fmt.Errorf("%w: from.{cell,role} is empty", ErrAbsentTriple)
	case e.To.Cell == "" || e.To.Role == "":
		return nil, fmt.Errorf("%w: to.{cell,role} is empty", ErrAbsentTriple)
	case e.Verb == "":
		return nil, fmt.Errorf("%w: verb is empty", ErrAbsentTriple)
	case e.ID == "":
		return nil, fmt.Errorf("%w: id is empty", ErrAbsentTriple)
	}

	if err := capFields(&e); err != nil {
		return nil, err
	}

	if !KnownVerb(e.Verb) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownVerb, e.Verb)
	}
	if e.Class != "" && !KnownClass(e.Class) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownClass, e.Class)
	}
	return &e, nil
}

// capFields enforces the per-field, ref, and payload size caps. It is separate so
// the caps read as one list rather than being scattered through the parse.
func capFields(e *Envelope) error {
	for _, f := range []struct {
		name string
		val  string
	}{
		{"id", e.ID},
		{"cell", e.Cell},
		{"from.cell", e.From.Cell},
		{"from.role", e.From.Role},
		{"from.app", e.From.App},
		{"from.session", e.From.Session},
		{"to.cell", e.To.Cell},
		{"to.role", e.To.Role},
		{"verb", e.Verb},
		{"class", e.Class},
	} {
		if len(f.val) > MaxFieldBytes {
			return fmt.Errorf("%w: field %s is %d bytes, cap is %d", ErrOversize, f.name, len(f.val), MaxFieldBytes)
		}
	}
	if len(e.Refs) > MaxRefs {
		return fmt.Errorf("%w: %d refs, cap is %d", ErrOversize, len(e.Refs), MaxRefs)
	}
	for i, r := range e.Refs {
		if len(r) > MaxRefBytes {
			return fmt.Errorf("%w: refs[%d] is %d bytes, cap is %d", ErrOversize, i, len(r), MaxRefBytes)
		}
	}
	if len(e.Payload) > MaxPayloadBytes {
		return fmt.Errorf("%w: payload is %d bytes, cap is %d", ErrOversize, len(e.Payload), MaxPayloadBytes)
	}
	return nil
}

// VerifyEnvelope authenticates a parsed envelope: it refuses any message whose
// declared From disagrees with the identity its assertion proves, then runs the
// full assertion verification (signature, TTL, clock skew, single-use nonce)
// against the trust store. It is the verify-or-refuse gate that must pass BEFORE
// the message is handed to the ACL, the router, or any handler.
//
// The assertion binds {cell, role, msg-id}; comparing those to the envelope's own
// From and ID is what stops a validly-signed assertion for one message being
// replayed on the envelope of another. A signature that verifies against a
// mismatched msg-id cannot exist without the signer's key, so the two checks
// together bind the identity to THIS envelope.
func VerifyEnvelope(e *Envelope, now time.Time, trust TrustStore, skew time.Duration, replay ReplayGuard) error {
	if e == nil {
		return fmt.Errorf("%w: nil envelope", ErrMalformed)
	}
	if e.Sig.Cell != e.From.Cell || e.Sig.Role != e.From.Role || e.Sig.MsgID != e.ID {
		return fmt.Errorf("%w: assertion binds {cell=%q role=%q msg=%q}, envelope declares {cell=%q role=%q id=%q}",
			ErrIdentityMismatch, e.Sig.Cell, e.Sig.Role, e.Sig.MsgID, e.From.Cell, e.From.Role, e.ID)
	}
	return Verify(e.Sig, now, trust, skew, replay)
}
