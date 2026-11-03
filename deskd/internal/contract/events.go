package contract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// EventType names a server-sent event on GET /v1/events. There are exactly
// three, one event per STATE TRANSITION never per poll (the service contract
// dedup rule lives on the producer).
type EventType string

const (
	// EventBadge carries the inbox counts {open, urgent, gold}.
	EventBadge EventType = "badge"
	// EventTransition carries one escalation's state change {id, from, to, at}.
	EventTransition EventType = "escalation.transition"
	// EventIndexHealth carries the per-repo could-not-check set.
	EventIndexHealth EventType = "index.health"
	// EventResync tells a client that fell off the ring buffer to re-read
	// /v1/inbox. It carries no data.
	EventResync EventType = "resync"

	// EventTerminalOutput and EventTerminalState are the two frames of the
	// per-desk terminal stream, GET /v1/desks/{id}/terminal/stream
	// (the service contract). They never appear on /v1/events, and the three
	// events above never appear on a terminal stream; the two streams share
	// the [Event] envelope, [Event.EncodeSSE] and [EventResync], nothing else.
	//
	// On a terminal stream the event ID is the desk's scrollback CURSOR — a
	// cumulative byte offset — rendered as a decimal string, so Last-Event-ID
	// IS the cursor and there is no second id space. Every frame's id is the
	// cursor the client holds AFTER that frame: an output frame's id is the
	// offset at the end of its bytes, a state frame's id is the offset already
	// delivered (it moves no bytes), and a resync frame's id is the cursor the
	// stream restarts from.
	EventTerminalOutput EventType = "terminal.output"
	// EventTerminalState carries the desk's derived agent state and OSC title.
	EventTerminalState EventType = "terminal.state"
)

// Event is the SSE envelope: {type, id, at, data}. ID is monotonic and is what
// a client sends back as Last-Event-ID to resume from the ring buffer.
type Event struct {
	Type EventType       `json:"type"`
	ID   string          `json:"id"`
	At   time.Time       `json:"at"`
	Data json.RawMessage `json:"data,omitempty"`
}

// BadgeData is the payload of an [EventBadge].
type BadgeData struct {
	Open   int `json:"open"`
	Urgent int `json:"urgent"`
	Gold   int `json:"gold"`
}

// TransitionData is the payload of an [EventTransition].
type TransitionData struct {
	ID   string    `json:"id"`
	From string    `json:"from"`
	To   string    `json:"to"`
	At   time.Time `json:"at"`
}

// IndexHealthData is the payload of an [EventIndexHealth].
type IndexHealthData struct {
	CouldNotCheck []string `json:"could_not_check"`
}

// TerminalOutputData is the payload of an [EventTerminalOutput]: a run of the
// desk's raw terminal bytes. SSE is a text protocol and a terminal stream is
// not text (escape sequences, split UTF-8 sequences at a frame boundary), so
// the bytes ride base64 — []byte's JSON encoding — and the client decodes them
// before handing them to the terminal emulator. The ~33% overhead is accepted
// and stated (protocol option 1: SSE, never a socket upgrade).
//
// Cursor is the cumulative byte offset at the END of Bytes (it equals the
// frame's id). A client that tracks its own cursor can therefore check every
// frame: cursor - len(bytes) must equal the cursor it already holds, or a byte
// was skipped or replayed.
type TerminalOutputData struct {
	Cursor uint64 `json:"cursor"`
	Bytes  []byte `json:"bytes"`
}

// TerminalStateData is the payload of an [EventTerminalState]: the desk's
// derived agent state (live | busy | waiting | idle | could-not-check), its
// last OSC title, and when the state was derived. It is the same two-signal
// derivation [TerminalHandle.State] reports, pushed on change instead of
// polled.
type TerminalStateData struct {
	State string    `json:"state"`
	Title string    `json:"title"`
	At    time.Time `json:"at"`
}

// NewEvent builds an Event with data marshalled to JSON.
func NewEvent(id string, at time.Time, typ EventType, data any) (Event, error) {
	e := Event{Type: typ, ID: id, At: at}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return Event{}, err
		}
		e.Data = raw
	}
	return e, nil
}

// EncodeSSE writes the event as an SSE frame:
//
//	id: <id>
//	event: <type>
//	data: <json of the whole Event>
//	<blank line>
//
// The whole Event (type/id/at/data) is the data payload so a client that does
// not parse SSE field lines can still json-decode one frame; the id and event
// lines let a browser EventSource resume and dispatch by type.
func (e Event) EncodeSSE(w io.Writer) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", e.ID); err != nil {
			return err
		}
	}
	if e.Type != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", e.Type); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", raw)
	return err
}

// EventStream reads SSE frames from an [io.Reader] (the body of GET /v1/events)
// and yields decoded [Event]s. It tracks the last id it saw so a caller can
// resume with [EventStream.LastEventID].
type EventStream struct {
	sc     *bufio.Scanner
	closer io.Closer
	last   string
}

// NewEventStream wraps an SSE body. closer may be nil (e.g. a test buffer).
func NewEventStream(r io.Reader, closer io.Closer) *EventStream {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	return &EventStream{sc: sc, closer: closer}
}

// LastEventID is the id of the most recently yielded event, for resume.
func (s *EventStream) LastEventID() string { return s.last }

// Next reads and returns the next event, or io.EOF when the stream ends. It
// accumulates `data:` lines until a blank line terminates the frame, then
// decodes the Event from the joined data payload (the whole-Event encoding
// [Event.EncodeSSE] produces).
func (s *EventStream) Next() (Event, error) {
	var data strings.Builder
	sawData := false
	for s.sc.Scan() {
		line := s.sc.Text()
		if line == "" {
			if !sawData {
				continue // blank line with no pending frame — skip
			}
			return s.decode(data.String())
		}
		if strings.HasPrefix(line, ":") {
			continue // comment/heartbeat
		}
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			sawData = true
			data.WriteString(strings.TrimPrefix(v, " "))
			continue
		}
		// id:/event: lines are redundant with the whole-Event payload; ignore.
	}
	if err := s.sc.Err(); err != nil {
		return Event{}, err
	}
	if sawData {
		return s.decode(data.String())
	}
	return Event{}, io.EOF
}

func (s *EventStream) decode(payload string) (Event, error) {
	var e Event
	if err := json.Unmarshal([]byte(payload), &e); err != nil {
		return Event{}, fmt.Errorf("event frame did not decode: %w", err)
	}
	if e.ID != "" {
		s.last = e.ID
	}
	return e, nil
}

// Close releases the underlying body if there is one.
func (s *EventStream) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}
