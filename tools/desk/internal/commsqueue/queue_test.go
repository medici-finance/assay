package commsqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

func sampleEnvelope(id string) comms.Envelope {
	return comms.Envelope{
		Schema: comms.Schema,
		ID:     id,
		Cell:   "cell-a",
		From:   comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:     comms.Lane{Cell: "cell-a", Role: "worker-desk"},
		Verb:   "handoff",
	}
}

// --- Atomic-write invariant --------------------------------------------------

// TestWriteAcceptedNoTempLeftover proves atomicWriteJSON's write-then-rename
// shape (the package doc's "every write here is ATOMIC" claim): after one
// write, exactly the final .json file exists — no sibling .tmp file is ever
// left behind for a reader (or a later write) to trip over.
func TestWriteAcceptedNoTempLeftover(t *testing.T) {
	root := t.TempDir()
	env := sampleEnvelope("msg-1")
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)

	if err := WriteAccepted(root, env, now); err != nil {
		t.Fatalf("WriteAccepted: %v", err)
	}

	entries, err := os.ReadDir(AcceptedDir(root))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("accepted dir has %d entries, want exactly 1 (no .tmp leftover): %v", len(entries), entries)
	}
	if got := entries[0].Name(); got != "msg-1.json" {
		t.Fatalf("accepted dir entry = %q, want %q (no stray .tmp file)", got, "msg-1.json")
	}

	items, err := ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(items) != 1 || items[0].Envelope.ID != "msg-1" {
		t.Fatalf("ListAccepted = %v, want one item msg-1", items)
	}
	if !items[0].AcceptedAt.Equal(now) {
		t.Fatalf("AcceptedAt = %v, want %v", items[0].AcceptedAt, now)
	}
}

// TestAtomicWriteNeverObservesPartialContent drives TWO concurrency shapes
// against a single reader that continuously lists+parses the directory:
//
//   - many writers, each hammering its OWN id repeatedly with alternating
//     short/long payloads (varying byte count, so a torn read would very
//     likely land mid-write and fail to unmarshal) — the production shape of
//     many distinct messages arriving concurrently;
//   - one further id repeatedly overwritten IN PLACE by a single writer
//     (commsgw's own re-accept-on-retry shape), racing the same reader.
//
// Since atomicWriteJSON builds the full bytes off to the side and only
// RENAMES into place, the reader must always see either a complete prior
// write or a complete new one for every file — parsing must never fail with
// a truncated/partial-JSON error, which is exactly what a naive
// write-in-place (no temp+rename) would risk under concurrent access.
func TestAtomicWriteNeverObservesPartialContent(t *testing.T) {
	root := t.TempDir()
	const writers = 8
	const rounds = 30

	var wg sync.WaitGroup
	stop := make(chan struct{})

	writeRounds := func(id string, w int) {
		defer wg.Done()
		for r := 0; r < rounds; r++ {
			env := sampleEnvelope(id)
			if r%2 == 0 {
				env.Verb = "handoff"
			} else {
				env.Verb = "a-much-longer-verb-string-to-change-the-payload-size-materially"
			}
			if err := WriteAccepted(root, env, time.Now().UTC()); err != nil {
				t.Errorf("writer %d (id=%s): WriteAccepted: %v", w, id, err)
				return
			}
		}
	}

	// Many writers, each on its own distinct id — the ordinary production
	// concurrency shape (distinct messages arriving at once).
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go writeRounds(fmt.Sprintf("writer-%d", w), w)
	}
	// One id repeatedly overwritten in place by a single writer, racing the
	// reader below — proves the SAME-id rename-into-place path is torn-read
	// safe too.
	wg.Add(1)
	go writeRounds("hammered", -1)

	// Reader: continuously lists+parses every file until the writers finish.
	var readErr error
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := ListAccepted(root); err != nil {
				readErr = err
				return
			}
		}
	}()

	wg.Wait()
	close(stop)
	<-readerDone

	if readErr != nil {
		t.Fatalf("ListAccepted observed a torn/partial write during concurrent WriteAccepted calls: %v", readErr)
	}

	// Final state must still parse cleanly: writers distinct ids + the one
	// repeatedly-overwritten id.
	items, err := ListAccepted(root)
	if err != nil {
		t.Fatalf("final ListAccepted: %v", err)
	}
	if len(items) != writers+1 {
		t.Fatalf("final accepted queue has %d items, want %d", len(items), writers+1)
	}

	// No .tmp file left behind after the dust settles.
	entries, err := os.ReadDir(AcceptedDir(root))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf("stray temp file left behind after concurrent writes: %s", e.Name())
		}
	}
}

// --- ListAccepted / RemoveAccepted behavior ----------------------------------

func TestListAcceptedMissingDirIsEmptyNotError(t *testing.T) {
	root := t.TempDir() // accepted/ never created
	items, err := ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted on a never-written queue must not error, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ListAccepted on a never-written queue = %v, want empty", items)
	}
}

func TestListAcceptedSortedByID(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	for _, id := range []string{"z-msg", "a-msg", "m-msg"} {
		if err := WriteAccepted(root, sampleEnvelope(id), now); err != nil {
			t.Fatalf("WriteAccepted(%s): %v", id, err)
		}
	}
	items, err := ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	want := []string{"a-msg", "m-msg", "z-msg"}
	for i, w := range want {
		if items[i].Envelope.ID != w {
			t.Fatalf("items[%d].ID = %q, want %q (must be sorted)", i, items[i].Envelope.ID, w)
		}
	}
}

func TestRemoveAcceptedIdempotent(t *testing.T) {
	root := t.TempDir()
	// Removing an id that was never written must be a no-op, not an error —
	// commsloop's Land calls RemoveAccepted unconditionally on every landing
	// path and must never wedge on a double-remove.
	if err := RemoveAccepted(root, "never-existed"); err != nil {
		t.Fatalf("RemoveAccepted on a nonexistent id must be a no-op, got %v", err)
	}

	if err := WriteAccepted(root, sampleEnvelope("m1"), time.Now().UTC()); err != nil {
		t.Fatalf("WriteAccepted: %v", err)
	}
	if err := RemoveAccepted(root, "m1"); err != nil {
		t.Fatalf("RemoveAccepted: %v", err)
	}
	if err := RemoveAccepted(root, "m1"); err != nil {
		t.Fatalf("second RemoveAccepted of an already-removed id must still be a no-op, got %v", err)
	}
	items, err := ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("ListAccepted after remove = %v, want empty", items)
	}
}

// --- safeSeg path-escape hardening -------------------------------------------

// TestSafeSegNeverEscapesRoot pins the sanitizer every write path in this
// package routes IDs through: a hostile or malformed envelope ID (carrying
// path-traversal characters — IDs arrive over the network, unlike a
// hand-typed filename) can never place a file outside the queue root.
func TestSafeSegNeverEscapesRoot(t *testing.T) {
	root := t.TempDir()
	hostile := "../../../etc/passwd"
	if err := WriteAccepted(root, sampleEnvelope(hostile), time.Now().UTC()); err != nil {
		t.Fatalf("WriteAccepted with a hostile id: %v", err)
	}

	// The written file must land INSIDE AcceptedDir(root), never above it.
	entries, err := os.ReadDir(AcceptedDir(root))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want exactly 1 written inside the queue root: %v", len(entries), entries)
	}
	for _, e := range entries {
		if filepath.Dir(e.Name()) != "." {
			t.Fatalf("entry name %q carries a path separator — escaped its directory", e.Name())
		}
	}
}

// --- Mailbox: deliver / poll / ack --------------------------------------------

func TestMailboxDeliverPollAck(t *testing.T) {
	root := t.TempDir()
	notice := Notice{ID: "n1", From: comms.SenderID{Cell: "cell-a", Role: "the-desk"}, Verb: "status", Sent: time.Now().UTC()}

	if err := DeliverToMailbox(root, "cell-b", "worker-desk", notice); err != nil {
		t.Fatalf("DeliverToMailbox: %v", err)
	}

	polled, err := PollMailbox(root, "cell-b", "worker-desk")
	if err != nil {
		t.Fatalf("PollMailbox: %v", err)
	}
	if len(polled) != 1 || polled[0].ID != "n1" {
		t.Fatalf("PollMailbox = %v, want one notice n1", polled)
	}

	if err := AckMailbox(root, "cell-b", "worker-desk", "n1"); err != nil {
		t.Fatalf("AckMailbox: %v", err)
	}

	// Acked notice must no longer poll...
	polledAfter, err := PollMailbox(root, "cell-b", "worker-desk")
	if err != nil {
		t.Fatalf("PollMailbox after ack: %v", err)
	}
	if len(polledAfter) != 0 {
		t.Fatalf("PollMailbox after ack = %v, want empty (acked notice must not repoll)", polledAfter)
	}

	// ...but MUST still exist, moved (not deleted) into the acked partition —
	// acknowledgement moves, it never deletes (queue.go's own doc).
	ackedPath := filepath.Join(MailboxAckedDir(root, "cell-b", "worker-desk"), "n1.json")
	raw, err := os.ReadFile(ackedPath)
	if err != nil {
		t.Fatalf("acked copy missing at %s: %v", ackedPath, err)
	}
	var got Notice
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("acked copy does not parse: %v", err)
	}
	if got.ID != "n1" {
		t.Fatalf("acked copy ID = %q, want n1", got.ID)
	}
}

func TestAckMailboxUnknownIDErrors(t *testing.T) {
	root := t.TempDir()
	if err := AckMailbox(root, "cell-a", "worker-desk", "never-delivered"); err == nil {
		t.Fatalf("AckMailbox on an id never delivered must error, not silently no-op")
	}
}
