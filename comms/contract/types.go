package contract

import (
	"errors"
	"fmt"
	"time"
)

// types.go — the typed, transport-neutral view of a cellmsg-v1 message. These
// types name the contract so a client or server need not know the legacy JSON
// field layout; the legacy wire mapping is Message.Envelope / MessageFromEnvelope
// and is lossless for every field cellmsg-v1 carries.

// Address names a mailbox: a {cell, mailbox} pair. In cellmsg-v1 the mailbox is
// the role's inbox (the legacy Lane.Role); a future transport may address
// mailboxes that are not roles, which is why the contract type is not "Role".
type Address struct {
	Cell    string
	Mailbox string
}

// String renders the address as cell/mailbox for logs and error text.
func (a Address) String() string { return a.Cell + "/" + a.Mailbox }

// AddressOfLane maps a legacy Lane to an Address.
func AddressOfLane(l Lane) Address { return Address{Cell: l.Cell, Mailbox: l.Role} }

// Lane maps an Address back to the legacy Lane.
func (a Address) Lane() Lane { return Lane{Cell: a.Cell, Role: a.Mailbox} }

// Message is the transport-neutral typed message. It carries exactly the
// information a cellmsg-v1 envelope carries — no more — so the mapping to and
// from the legacy wire loses nothing and invents nothing.
type Message struct {
	ID      string
	Cell    string
	From    SenderID
	To      Address
	Verb    string
	Class   string
	Refs    []string
	Payload []byte
	Sent    time.Time
	Sig     Assertion
}

// MessageFromEnvelope maps a parsed legacy Envelope to the typed Message.
func MessageFromEnvelope(e *Envelope) Message {
	if e == nil {
		return Message{}
	}
	return Message{
		ID: e.ID, Cell: e.Cell, From: e.From, To: AddressOfLane(e.To),
		Verb: e.Verb, Class: e.Class, Refs: append([]string(nil), e.Refs...),
		Payload: append([]byte(nil), e.Payload...), Sent: e.Sent, Sig: e.Sig,
	}
}

// Envelope maps the typed Message back to the legacy wire struct (Schema is set
// to the one schema this contract speaks).
func (m Message) Envelope() *Envelope {
	e := &Envelope{
		Schema: Schema, ID: m.ID, Cell: m.Cell, From: m.From, To: m.To.Lane(),
		Verb: m.Verb, Class: m.Class, Sent: m.Sent, Sig: m.Sig,
	}
	if len(m.Refs) > 0 {
		e.Refs = append([]string(nil), m.Refs...)
	}
	if len(m.Payload) > 0 {
		e.Payload = append([]byte(nil), m.Payload...)
	}
	return e
}

// ErrorCode is the stable, machine-readable class of a contract refusal. Codes
// are part of the public contract; their string values never change within a
// schema version.
type ErrorCode string

const (
	CodeOversize         ErrorCode = "oversize"
	CodeMalformed        ErrorCode = "malformed"
	CodeBadSchema        ErrorCode = "bad_schema"
	CodeAbsentTriple     ErrorCode = "absent_triple"
	CodeUnknownVerb      ErrorCode = "unknown_verb"
	CodeUnknownClass     ErrorCode = "unknown_class"
	CodeUnknownCell      ErrorCode = "unknown_cell"
	CodeBadSignature     ErrorCode = "bad_signature"
	CodeExpired          ErrorCode = "expired"
	CodeNotYetValid      ErrorCode = "not_yet_valid"
	CodeReplay           ErrorCode = "replay"
	CodeIdentityMismatch ErrorCode = "identity_mismatch"
	CodeMintInput        ErrorCode = "mint_input"
	// CodeNone is returned for a nil error; CodeUnclassified for an error that
	// is not one of the contract's sentinels (a host policy refusal, say).
	CodeNone         ErrorCode = ""
	CodeUnclassified ErrorCode = "unclassified"
)

var codeTable = []struct {
	err  error
	code ErrorCode
}{
	{ErrOversize, CodeOversize},
	{ErrMalformed, CodeMalformed},
	{ErrBadSchema, CodeBadSchema},
	{ErrAbsentTriple, CodeAbsentTriple},
	{ErrUnknownVerb, CodeUnknownVerb},
	{ErrUnknownClass, CodeUnknownClass},
	{ErrUnknownCell, CodeUnknownCell},
	{ErrBadSignature, CodeBadSignature},
	{ErrExpired, CodeExpired},
	{ErrNotYetValid, CodeNotYetValid},
	{ErrReplay, CodeReplay},
	{ErrIdentityMismatch, CodeIdentityMismatch},
	{ErrMintInput, CodeMintInput},
}

// CodeOf classifies err into its stable ErrorCode. A nil error is CodeNone.
func CodeOf(err error) ErrorCode {
	if err == nil {
		return CodeNone
	}
	for _, c := range codeTable {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return CodeUnclassified
}

// Error is a typed refusal: a stable Code plus human detail. It wraps the
// underlying sentinel so errors.Is keeps working.
type Error struct {
	Code   ErrorCode
	Detail string
	Err    error
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Detail)
	}
	return string(e.Code)
}

// Unwrap exposes the underlying sentinel.
func (e *Error) Unwrap() error { return e.Err }

// AsError wraps err in a typed *Error carrying its CodeOf; nil stays nil.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: CodeOf(err), Detail: err.Error(), Err: err}
}

// Stage is how far a message has progressed through a transport. The legacy
// gateway surfaces Accepted/Refused on submit; later stages are declared for a
// transport that implements them.
type Stage string

const (
	StageAccepted  Stage = "accepted"
	StageRefused   Stage = "refused"
	StageHeld      Stage = "held"
	StageDelivered Stage = "delivered"
	StageAcked     Stage = "acked"
)

// Receipt is the typed acknowledgement of a submit. It mirrors the legacy
// gateway receipt ({id, accepted, detail}) and adds the Stage the legacy
// boolean collapses.
type Receipt struct {
	MessageID string
	Stage     Stage
	Detail    string
	At        time.Time
}

// Accepted reports whether the receipt records an accepted submit.
func (r Receipt) Accepted() bool { return r.Stage == StageAccepted }

// LegacyReceipt is the wire shape of the legacy submit receipt.
type LegacyReceipt struct {
	ID       string `json:"id"`
	Accepted bool   `json:"accepted"`
	Detail   string `json:"detail,omitempty"`
}

// ReceiptFromLegacy maps the legacy accepted/refused receipt to a typed one.
func ReceiptFromLegacy(l LegacyReceipt, at time.Time) Receipt {
	st := StageRefused
	if l.Accepted {
		st = StageAccepted
	}
	return Receipt{MessageID: l.ID, Stage: st, Detail: l.Detail, At: at}
}

// Legacy maps a typed receipt back to the legacy wire shape.
func (r Receipt) Legacy() LegacyReceipt {
	return LegacyReceipt{ID: r.MessageID, Accepted: r.Accepted(), Detail: r.Detail}
}

// Capability names a contract capability a transport may or may not provide.
type Capability string

const (
	// CapDeliveryToken — a per-delivery token that proves a mailbox read was
	// authorised for exactly one message. NOT implemented by cellmsg-v1.
	CapDeliveryToken Capability = "delivery-token/v1"
	// CapClientIdentity — independent per-client cryptographic identity beneath
	// the cell+role assertion. NOT implemented by cellmsg-v1: the legacy trust
	// store holds one public key per cell, and a role name does not establish
	// per-client identity.
	CapClientIdentity Capability = "client-identity/v1"
)

// Implemented reports whether a capability is implemented by cellmsg-v1. Both
// declared capabilities are future work and return false; a transport that
// claims them must ship its own versioned implementation.
func Implemented(c Capability) bool { return false }

// Capabilities lists the declared-but-unimplemented capabilities, so a caller
// can refuse to rely on one.
func Capabilities() []Capability {
	return []Capability{CapDeliveryToken, CapClientIdentity}
}
