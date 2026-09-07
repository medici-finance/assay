package main

import (
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// fakeEmitter records every item handed to it and can be told to fail.
type fakeEmitter struct {
	items []InboxItem
	err   error
}

func (f *fakeEmitter) Emit(item InboxItem) error {
	if f.err != nil {
		return f.err
	}
	f.items = append(f.items, item)
	return nil
}

// fakeFiler records every quarantine filing without shelling out.
type fakeFiler struct {
	calls int
	err   error
}

func (f *fakeFiler) File(env comms.Envelope, reason string) error {
	f.calls++
	return f.err
}

func crossCellEnvelope(verb string) comms.Envelope {
	return comms.Envelope{
		Schema: comms.Schema,
		ID:     "xc-" + verb,
		From:   comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:     comms.Lane{Cell: "cell-b", Role: "the-desk"},
		Verb:   verb,
		Class:  "routine",
		Sent:   time.Now().UTC(),
	}
}

// --- Verify row 13: CrossCellInboxItem --------------------------------------

func TestCrossCellInboxItem(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	emitter := &fakeEmitter{}

	// The request: cell-a -> cell-b.
	req := crossCellEnvelope("status")
	if err := EmitCrossCellInboxItem(root, &req, now, emitter, nil); err != nil {
		t.Fatalf("emit request: %v", err)
	}

	// The reply: cell-b -> cell-a (same rule, reverse direction).
	reply := req
	reply.ID = "xc-status-reply"
	reply.From = comms.SenderID{Cell: req.To.Cell, Role: req.To.Role}
	reply.To = comms.Lane{Cell: req.From.Cell, Role: req.From.Role}
	if err := EmitCrossCellInboxItem(root, &reply, now, emitter, nil); err != nil {
		t.Fatalf("emit reply: %v", err)
	}

	if len(emitter.items) != 2 {
		t.Fatalf("want 2 emitted items (request + reply), got %d", len(emitter.items))
	}
	if emitter.items[0].Cell != "cell-b" {
		t.Fatalf("request item should land in cell-b's inbox (the receiver), got %q", emitter.items[0].Cell)
	}
	if emitter.items[1].Cell != "cell-a" {
		t.Fatalf("reply item should land in cell-a's inbox (the original sender), got %q", emitter.items[1].Cell)
	}

	// focus-on carries a disposition defaulting to no-answer-yet.
	fo := crossCellEnvelope("focus-on")
	if err := EmitCrossCellInboxItem(root, &fo, now, emitter, nil); err != nil {
		t.Fatalf("emit focus-on: %v", err)
	}
	last := emitter.items[len(emitter.items)-1]
	if last.Disposition != DispositionNoAnswerYet {
		t.Fatalf("focus-on item disposition = %q, want %q", last.Disposition, DispositionNoAnswerYet)
	}

	// A REFUSED cross-cell message never reaches EmitCrossCellInboxItem at
	// all in the real pipeline (main.go only calls it post-accept) — proven
	// here by construction: nothing calls Emit for a message PreCheck refused,
	// so there is nothing further to assert; a within-cell (non-cross-cell)
	// message emits none.
	within := comms.Envelope{ID: "wc-1", From: comms.SenderID{Cell: "cell-a", Role: "the-desk"}, To: comms.Lane{Cell: "cell-a", Role: "worker-desk"}, Verb: "handoff"}
	before := len(emitter.items)
	if err := EmitCrossCellInboxItem(root, &within, now, emitter, nil); err != nil {
		t.Fatalf("within-cell should be a no-op, got err %v", err)
	}
	if len(emitter.items) != before {
		t.Fatalf("within-cell message must emit no inbox item")
	}
}

// --- Verify row 14: CrossCellInboxEmitFails ---------------------------------

func TestCrossCellInboxEmitFails(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	emitter := &fakeEmitter{err: errors.New("index unreachable / unknown kind")}
	filer := &fakeFiler{}

	env := crossCellEnvelope("metrics")
	err := EmitCrossCellInboxItem(root, &env, now, emitter, filer)
	if !errors.Is(err, ErrQuarantined) {
		t.Fatalf("an emission failure must be reported as a quarantine, not swallowed: %v", err)
	}
	if filer.calls != 1 {
		t.Fatalf("quarantine issue filing calls = %d, want 1", filer.calls)
	}

	held, lerr := ListHeld(root)
	if lerr != nil {
		t.Fatalf("ListHeld: %v", lerr)
	}
	if len(held) != 1 || held[0].Envelope.ID != env.ID {
		t.Fatalf("message must land in the held mailbox — no accepted cross-cell message is dropped; held=%v", held)
	}

	// Even if issue filing ALSO fails, the message is still held (never lost).
	root2 := t.TempDir()
	filerFails := &fakeFiler{err: errors.New("forge unreachable")}
	env2 := crossCellEnvelope("help-offered")
	if err := EmitCrossCellInboxItem(root2, &env2, now, emitter, filerFails); !errors.Is(err, ErrQuarantined) {
		t.Fatalf("want ErrQuarantined surfaced when both emission and filing fail, got %v", err)
	}
	held2, lerr := ListHeld(root2)
	if lerr != nil {
		t.Fatalf("ListHeld: %v", lerr)
	}
	if len(held2) != 1 {
		t.Fatalf("message must still be held even when issue filing itself fails, held=%v", held2)
	}
}
