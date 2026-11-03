package contract

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// TestEventSSERoundTrip: an event encoded as an SSE frame decodes back through
// EventStream with its type, id, and payload intact.
func TestEventSSERoundTrip(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	ev, err := NewEvent("42", at, EventTransition, TransitionData{ID: "esc-1", From: "working", To: "waiting", At: at})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	var buf bytes.Buffer
	if err := ev.EncodeSSE(&buf); err != nil {
		t.Fatalf("EncodeSSE: %v", err)
	}
	st := NewEventStream(&buf, nil)
	got, err := st.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Type != EventTransition || got.ID != "42" {
		t.Fatalf("decoded event = %+v, want type=%s id=42", got, EventTransition)
	}
	if st.LastEventID() != "42" {
		t.Fatalf("LastEventID = %q, want 42 (needed for resume)", st.LastEventID())
	}
}

// TestEventStreamMultipleFramesAndEOF: two frames read in order, then io.EOF;
// heartbeat comment lines are skipped.
func TestEventStreamMultipleFramesAndEOF(t *testing.T) {
	at := time.Now().UTC()
	var buf bytes.Buffer
	e1, _ := NewEvent("1", at, EventBadge, BadgeData{Open: 3, Urgent: 1, Gold: 0})
	e2, _ := NewEvent("2", at, EventIndexHealth, IndexHealthData{CouldNotCheck: []string{"o/r"}})
	_ = e1.EncodeSSE(&buf)
	buf.WriteString(": heartbeat\n\n") // a comment/keepalive line between frames
	_ = e2.EncodeSSE(&buf)

	st := NewEventStream(&buf, nil)
	first, err := st.Next()
	if err != nil || first.ID != "1" || first.Type != EventBadge {
		t.Fatalf("first frame = %+v err=%v, want id=1 badge", first, err)
	}
	second, err := st.Next()
	if err != nil || second.ID != "2" || second.Type != EventIndexHealth {
		t.Fatalf("second frame = %+v err=%v, want id=2 index.health", second, err)
	}
	if _, err := st.Next(); err != io.EOF {
		t.Fatalf("third Next err = %v, want io.EOF", err)
	}
	if st.LastEventID() != "2" {
		t.Fatalf("LastEventID = %q, want 2", st.LastEventID())
	}
}
