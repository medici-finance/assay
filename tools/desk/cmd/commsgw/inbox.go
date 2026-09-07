package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// inbox.go — deskd inbox emission for cross-cell traffic (#1896 addendum). On
// every ACCEPTED cross-cell message the gateway emits one inbox item of kind
// "cross-cell" into the receiving cell's inbox.
//
// ONE RULE COVERS BOTH DIRECTIONS. "The item lands in the receiving cell's
// inbox, and the reply in the sender's" is not two rules: a reply IS an
// accepted cross-cell envelope, just in the reverse direction, so "emit into
// env.To.Cell's inbox for every accepted cross-cell envelope" alone produces
// the request landing at the original receiver and the reply landing at the
// original sender, with no special-casing of "is this a reply".
//
// The `cross-cell` item KIND belongs to a SEPARATE deskd inbox index this
// gateway does not own or ship. This gateway emits items of that kind through
// the InboxEmitter seam below and must not silently drop one when the index
// does not yet recognise the kind: an emission failure quarantines the
// message, it is never a dropped send.
type InboxItem struct {
	Kind      string         `json:"kind"` // always "cross-cell"
	Cell      string         `json:"cell"` // the inbox this item lands in (env.To.Cell)
	MessageID string         `json:"messageId"`
	From      comms.SenderID `json:"from"`
	To        comms.Lane     `json:"to"`
	Verb      string         `json:"verb"`
	// Payload is carried opaquely, the same way Notice carries it — the inbox
	// index re-interprets it, this gateway does not.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Disposition is populated ONLY for a focus-on item (#1896 addendum): the
	// receiving desk's response to the request, defaulting to
	// DispositionNoAnswerYet on emit.
	Disposition string    `json:"disposition,omitempty"`
	Emitted     time.Time `json:"emitted"`
}

// Disposition values for a focus-on InboxItem.
const (
	DispositionNoAnswerYet = "no-answer-yet"
	DispositionTakenUp     = "taken-up"
	DispositionDeclined    = "declined"
)

// InboxEmitter delivers one InboxItem to the deskd inbox index. It is an
// interface because the index itself is a separate, external system this
// gateway does not own: a real emitter posts to whatever transport that index
// turns out to expose; a fake emitter drives the failure/quarantine path in
// tests without either existing yet.
type InboxEmitter interface {
	Emit(InboxItem) error
}

// InboxItemFor builds the InboxItem for an accepted cross-cell envelope. It
// does not decide whether env is cross-cell — call it only after confirming
// env.From.Cell != env.To.Cell (main.go's accept path does this once, at the
// point it already knows).
func InboxItemFor(env *comms.Envelope, now time.Time) InboxItem {
	item := InboxItem{
		Kind:      "cross-cell",
		Cell:      env.To.Cell,
		MessageID: env.ID,
		From:      env.From,
		To:        env.To,
		Verb:      env.Verb,
		Payload:   env.Payload,
		Emitted:   now,
	}
	if env.Verb == "focus-on" {
		item.Disposition = DispositionNoAnswerYet
	}
	return item
}

// IsCrossCell reports whether env crosses a cell boundary.
func IsCrossCell(env *comms.Envelope) bool {
	return env.From.Cell != env.To.Cell
}

// NoOpInboxEmitter accepts every item without delivering it anywhere. It is
// the INERT default while the deskd inbox index does not exist yet: emission
// "succeeds" in the sense that nothing to fail has been wired, but it is never
// the correct choice once a real index endpoint exists — main.go's
// construction site is where a real emitter (or an explicitly-failing one, to
// prove the quarantine path) replaces it.
type NoOpInboxEmitter struct{}

func (NoOpInboxEmitter) Emit(InboxItem) error { return nil }

// EmitCrossCellInboxItem builds and emits the inbox item for an accepted
// cross-cell envelope. On emission failure it quarantines the message (WriteHeld
// + fileIssue) rather than dropping it — Verify row 14. It returns the
// quarantine error, if any; a nil return means either "not cross-cell" (no
// item to emit) or "emitted (or intentionally no-op'd) successfully".
func EmitCrossCellInboxItem(root string, env *comms.Envelope, now time.Time, emitter InboxEmitter, filer IssueFiler) error {
	if !IsCrossCell(env) {
		return nil
	}
	if emitter == nil {
		emitter = NoOpInboxEmitter{}
	}
	item := InboxItemFor(env, now)
	if err := emitter.Emit(item); err != nil {
		reason := fmt.Sprintf("commsgw: cross-cell inbox emission failed for %s: %v", env.ID, err)
		return Quarantine(root, *env, reason, now, filer)
	}
	return nil
}
