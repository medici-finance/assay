package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// refusal.go — every refused inbound writes ONE journal line (#1165).
//
// Before this file, a refusal was visible only on the client receipt: the
// daily lane-violation sweep (../commsloop sweep.go) reads journal.log, and
// nothing about a refused attempt ever reached it, so a sustained probe —
// forged peers, out-of-lane submits, replayed nonces — left no out-of-band
// trace and the sweep's "no violations" was could-not-check rather than a
// measurement. Now both transports (socket.go's handleSubmit, a2a.go's
// Execute) call journalRefusal on every refusal, and the sweep counts the
// lines per sender and per lane pair against a threshold.
//
// What the line carries and what it never carries is decided in
// internal/commsqueue/refusal.go (the shared shape): the refusal KIND, the
// lane pair and sender identity AS PRESENTED, the gateway's own cell, a
// timestamp, and a digest of the raw bytes. NEVER the payload, never the
// free-text error detail (which can echo untrusted field values).

// errCarrierMalformed marks a refusal that happened BEFORE PreCheck: the
// transport carrier (a socket request line that does not decode, an A2A
// message with no usable part) never yielded envelope bytes. It is a
// distinct kind so the sweep can tell "garbage on the wire" from "a
// well-carried envelope that failed a stage".
var errCarrierMalformed = errors.New("commsgw: transport carrier is malformed")

// ClassifyRefusal maps a PreCheck (or carrier) refusal onto the closed
// commsqueue.RefusalKind vocabulary. Finer kinds are matched FIRST (the
// comms identity errors PreCheck wraps under ErrAssertionInvalid), then the
// stage-level kinds; anything unmatched is RefusalOther — a member of the
// vocabulary, so an unclassifiable refusal is still journalled and counted,
// never dropped for want of a label.
func ClassifyRefusal(err error) commsqueue.RefusalKind {
	switch {
	case err == nil:
		return commsqueue.RefusalOther
	case errors.Is(err, ErrPeerUnauthenticated):
		return commsqueue.RefusalPeerUnauthenticated
	case errors.Is(err, errCarrierMalformed):
		return commsqueue.RefusalCarrierMalformed
	case errors.Is(err, ErrEnvelopeParse):
		return commsqueue.RefusalEnvelopeParse
	case errors.Is(err, comms.ErrUnknownCell):
		return commsqueue.RefusalUnknownCell
	case errors.Is(err, comms.ErrBadSignature):
		return commsqueue.RefusalBadSignature
	case errors.Is(err, comms.ErrExpired):
		return commsqueue.RefusalExpired
	case errors.Is(err, comms.ErrNotYetValid):
		return commsqueue.RefusalNotYetValid
	case errors.Is(err, comms.ErrReplay):
		return commsqueue.RefusalReplay
	case errors.Is(err, comms.ErrIdentityMismatch):
		return commsqueue.RefusalIdentityMismatch
	case errors.Is(err, ErrAssertionInvalid):
		return commsqueue.RefusalAssertionInvalid
	case errors.Is(err, ErrLaneDenied):
		return commsqueue.RefusalLaneDenied
	case errors.Is(err, ErrCrossCellPair):
		return commsqueue.RefusalCrossCellPair
	case errors.Is(err, ErrCrossCellVerb):
		return commsqueue.RefusalCrossCellVerb
	case errors.Is(err, ErrDuplicateMessage):
		return commsqueue.RefusalDuplicate
	case errors.Is(err, ErrBudgetExhausted):
		return commsqueue.RefusalBudgetExhausted
	case errors.Is(err, ErrRateLimiterUnconfigured):
		return commsqueue.RefusalRateLimiterUnconfigured
	case errors.Is(err, ErrKillSwitch):
		return commsqueue.RefusalKillSwitch
	default:
		return commsqueue.RefusalOther
	}
}

// presentedAddressing is the LENIENT read of the addressing fields a refused
// message claimed. It is not a parse in ParseEnvelope's sense — unknown
// fields, wrong-typed siblings and size caps are all ignored, and a decode
// error just leaves the fields empty — because the question here is only
// "what did these bytes SAY they were", for attribution, never "are they a
// valid envelope" (they are not; they were refused). Nothing read here is
// trusted, and nothing beyond these four fields is ever decoded.
type presentedAddressing struct {
	ID   string         `json:"id"`
	From comms.SenderID `json:"from"`
	To   comms.Lane     `json:"to"`
	Verb string         `json:"verb"`
}

func presentedIdentity(raw []byte) presentedAddressing {
	var p presentedAddressing
	_ = json.Unmarshal(raw, &p)         // lenient by design; see the type doc
	p.From.App, p.From.Session = "", "" // advisory, unsigned metadata — never journalled
	return p
}

// RefusalRecordFor builds the journal line for a refusal of raw at now. A
// peer-unauthenticated refusal reads NOTHING from the bytes (precheck.go:
// "refused before a single byte of it is parsed") — its line carries only
// the digest, the gateway cell and the kind. Every other kind attributes
// from the presented addressing, falling back to a synthetic
// `refused-<digest16>` id when none was presented so the line is never
// id-less (the sweep's reader treats that as corrupt).
func RefusalRecordFor(cell string, raw []byte, err error, now time.Time) commsqueue.RefusalRecord {
	kind := ClassifyRefusal(err)
	digest := deskkit.Sha256Hex(raw)
	rec := commsqueue.RefusalRecord{
		Time:      now,
		Kind:      commsqueue.JournalKindRefused,
		Cell:      cell,
		Refusal:   kind,
		RawDigest: digest,
	}
	if kind != commsqueue.RefusalPeerUnauthenticated && kind != commsqueue.RefusalCarrierMalformed {
		p := presentedIdentity(raw)
		rec.ID, rec.From, rec.To, rec.Verb = p.ID, p.From, p.To, p.Verb
	}
	if rec.ID == "" {
		rec.ID = "refused-" + digest[:16]
	}
	return rec
}

// journalRefusal appends the refusal line for (raw, err) to root's
// journal.log. The REFUSAL itself is never conditional on the write: a
// caller has already refused by the time it calls this, and a journal write
// failure is returned so the caller can append it to the receipt detail —
// surfaced to the sender, never silently swallowed — while the refusal
// stands exactly as it was.
func journalRefusal(root, cell string, raw []byte, err error, now time.Time) error {
	if werr := commsqueue.AppendRefusal(root, RefusalRecordFor(cell, raw, err, now)); werr != nil {
		return fmt.Errorf("commsgw: refusal journal write failed: %w", werr)
	}
	return nil
}

// refusalDetail is the receipt detail for a refusal: the refusal's own text,
// plus — only when the journal write failed — that failure, so the sender
// sees that the gateway refused AND could not record it.
func refusalDetail(refusal, journalErr error) string {
	if journalErr == nil {
		return refusal.Error()
	}
	return refusal.Error() + "; " + journalErr.Error()
}
