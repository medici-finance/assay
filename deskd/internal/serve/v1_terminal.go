// v1_terminal.go — GET /v1/desks/{id}/terminal/stream, the terminal SSE stream
// (the service contract): the READ half of the terminal seam (spec §5.1).
//
// The stream is the desk's scrollback from a cursor, then its live output, as
// server-sent events over the same typed HTTP seam every other read uses —
// protocol option 1: plain SSE, never a socket upgrade. It follows the conventions `/v1/events`
// (events.go) set: the [contract.Event] envelope framed by
// [contract.Event.EncodeSSE], the same four response headers, a flush per
// frame, `Last-Event-ID` as the resume key, [contract.EventResync] when the
// resume point cannot be honoured, and the request context as the one
// disconnect signal.
//
// # The cursor is the single point of failure
//
// A wrong cursor on resume either drops bytes or replays them, and both look
// fine on a quiet desk. So the id space is not invented here: a frame's id IS
// the ring's cumulative byte offset (internal/terminal/ring.go), and the
// handler keeps exactly one number — the cursor it has delivered up to — which
// it advances only by bytes it has successfully written. Replay and live are
// the same read ([terminal.Follower.ReadFrom] from that cursor), so there is no
// hand-off between a "replay phase" and a "live phase" for a byte to fall into.
//
// # What happens when the cursor cannot be honoured
//
// A `Last-Event-ID` older than the ring's retained window, beyond anything the
// ring has issued (a service restart — scrollback is memory-only), or not a
// decimal cursor at all, gets exactly ONE `resync` frame whose id is the cursor
// the stream restarts from (the ring's base), followed by the whole retained
// window and then live output. The client discards what it rendered and
// redraws from there. The stream never resumes "best effort from the oldest
// byte" without saying so, and never waits silently on a cursor it cannot
// reach. The same applies mid-stream to a client that reads so slowly the ring
// evicts past it: it is told, by a `resync`, that bytes were lost.
//
// # Read-only, and never the desk's owner
//
// The handler holds a [terminal.Follower], which has no reference to the
// driver session: there is no code path from this route to a write, a resize
// or a close. The request body is never read. A client going away deregisters
// its follower and nothing else (spec §2: attach, never host).
package serve

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// terminalStreamSuffix is the path tail that selects the stream under the
// /v1/desks/ subtree; [Server.handleV1Terminal] dispatches on it.
const terminalStreamSuffix = "/terminal/stream"

// terminalFrameBytes caps the raw bytes in one `terminal.output` frame. A
// resume can owe the client the whole ring (~1 MiB); one frame that size would
// be a ~1.4 MiB `data:` line, past the 1 MiB line limit [contract.EventStream]
// scans with. 16 KiB keeps a frame at ~22 KiB on the wire and gives a slow
// client a frame boundary — and so a resumable id — every 16 KiB.
const terminalFrameBytes = 16 << 10

// Stream timings. They are variables only so the tests can shorten them; the
// brief fixes the heartbeat at 15 s.
var (
	// terminalHeartbeat is the interval between `: heartbeat` SSE comments,
	// which keep an idle desk's connection open through proxies and let the
	// server notice a dead peer on a quiet stream (a write fails).
	terminalHeartbeat = 15 * time.Second
	// terminalStatePoll is how often the handler re-derives the desk's agent
	// state while no output is flowing. It must poll because the state changes
	// WITHOUT output: a prompt is "settled" only once the idle window has
	// elapsed since the last byte.
	terminalStatePoll = 500 * time.Millisecond
	// terminalWriteTimeout bounds ONE frame write. A peer that stops reading
	// without closing (a suspended laptop, a wedged proxy) would otherwise
	// park this goroutine in Write forever, holding the follower and the
	// connection. The pump is never affected either way — see
	// [terminal.Follower] — this bound is about reclaiming the handler.
	terminalWriteTimeout = 30 * time.Second
)

// handleV1TerminalStream serves GET /v1/desks/{id}/terminal/stream.
//
// Gate order, all before any frame: registry wired (503) → the desk's cell is
// readable by this viewer (403) → the desk is configured (404) → the response
// writer can stream (500). The authorization check is [Server.deskAuthorized]:
// [Server.cellAuthorized], the same per-cell, fail-closed check GET /v1/desks
// applies — no new capability — run BEFORE the desk lookup.
func (s *Server) handleV1TerminalStream(w http.ResponseWriter, r *http.Request, id string) {
	if s.Terminal == nil {
		// Not wired on this build: could-not-check, never a stream that hangs
		// open and delivers nothing (the same shape as a nil EventBus).
		s.writeV1Err(w, http.StatusServiceUnavailable, "the terminal registry is not wired on this build (could-not-check)")
		return
	}
	if !s.deskAuthorized(w, r, id) {
		return
	}
	follower, ok := s.Terminal.Follow(id)
	if !ok {
		s.writeV1Err(w, http.StatusNotFound, "no such desk "+id+" in the registry (a configured desk with no target still streams, as could-not-check — this id names no configured desk)")
		return
	}
	defer follower.Close()

	if _, ok := w.(http.Flusher); !ok {
		s.writeV1Err(w, http.StatusInternalServerError, "streaming is not supported by this response writer")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	st := &terminalStream{w: w, rc: http.NewResponseController(w), f: follower, now: s.Now}

	// The resume point. No header = cursor 0 = "everything you still have": a
	// first attach wants the whole retained scrollback, and owes nobody a
	// resync for bytes it never claimed to hold.
	header := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	resuming := header != ""
	if resuming {
		c, err := strconv.ParseUint(header, 10, 64)
		if err != nil {
			// Not a cursor this route ever issued. Same verdict as a cursor it
			// can no longer reach — say so and restart from the base — and the
			// same code path: a cursor beyond anything issued is a reset.
			c = ^uint64(0)
		}
		st.cursor = c
	}
	if !st.drain(resuming) {
		return
	}
	if !st.state(true) {
		return
	}

	heartbeat := time.NewTicker(terminalHeartbeat)
	defer heartbeat.Stop()
	poll := time.NewTicker(terminalStatePoll)
	defer poll.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-follower.Wake():
			if !st.drain(true) || !st.state(false) {
				return
			}
		case <-poll.C:
			if !st.state(false) {
				return
			}
		case <-heartbeat.C:
			if !st.write(func() error {
				_, err := st.w.Write([]byte(": heartbeat\n\n"))
				return err
			}) {
				return
			}
		}
	}
}

// deskAuthorized is the ONE per-cell gate both terminal reads under
// /v1/desks/{id}/ go through — the handle read and the stream (issue #132: a
// desk's handle and scrollback are live data about a cell whose repos the
// viewer may not be able to read). It writes the refusal and reports false.
//
// The verdict is reached from the CELL the id names, read from config, and the
// desk itself is not looked up until after it passes. So inside a cell the
// viewer may not read, a desk that exists and one that does not get the same
// 403 with the same body: 403-vs-404 cannot be used to map another cell's
// desks. (That a CELL exists is not protected — GET /v1/cells already lists
// every configured cell to the same audience.)
func (s *Server) deskAuthorized(w http.ResponseWriter, r *http.Request, id string) bool {
	cell := s.deskCell(id)
	if cell == nil {
		s.writeV1Err(w, http.StatusNotFound, "no such desk "+id+" in the registry")
		return false
	}
	if !s.cellAuthorized(r, cell) {
		s.writeV1Err(w, http.StatusForbidden, "the viewer has no read capability on cell "+cell.Name+"'s repos")
		return false
	}
	return true
}

// deskCell resolves the cell a desk id belongs to. A desk id is
// "<cell>/<role>" ([terminal.DeskID]), so the cell is everything before the
// first slash. An id this does not resolve is a 404 either way: the registry
// keys desks by the same "<cell>/<role>" string.
func (s *Server) deskCell(id string) *schema.Cell {
	name, _, ok := strings.Cut(id, "/")
	if !ok {
		return nil
	}
	return s.findCell(name)
}

// terminalStream is one connection's state: the writer, the follower, and the
// ONE cursor — the offset of the last byte successfully written to this client.
type terminalStream struct {
	w      http.ResponseWriter
	rc     *http.ResponseController
	f      *terminal.Follower
	now    func() time.Time
	cursor uint64

	sentState bool
	lastState string
	lastTitle string
}

// write runs one frame write under the per-frame deadline and flushes it. It
// reports false when the client is gone or stalled — the caller returns, which
// closes the follower.
func (st *terminalStream) write(fn func() error) bool {
	// A writer that cannot take a deadline (a test recorder, an exotic
	// middleware) still streams; it just is not protected from a stalled peer.
	_ = st.rc.SetWriteDeadline(time.Now().Add(terminalWriteTimeout))
	if err := fn(); err != nil {
		return false
	}
	return st.rc.Flush() == nil
}

func (st *terminalStream) emit(typ contract.EventType, data any) bool {
	evt, err := contract.NewEvent(strconv.FormatUint(st.cursor, 10), st.now(), typ, data)
	if err != nil {
		return false
	}
	return st.write(func() error { return evt.EncodeSSE(st.w) })
}

// drain writes everything the ring holds after the stream's cursor, in
// [terminalFrameBytes] frames, advancing the cursor frame by frame. announce
// controls whether a reset is announced with a `resync`: it is for every case
// except a first attach with no Last-Event-ID, where the client holds nothing
// to invalidate.
func (st *terminalStream) drain(announce bool) bool {
	b, start, reset := st.f.ReadFrom(st.cursor)
	// The ring, not this handler, says where the bytes begin. Without a reset
	// start IS the cursor asked for; taking it from the ring regardless means a
	// frame's cursor is always the true offset of its bytes, so even a ring that
	// answered wrongly would be caught by the client's own arithmetic
	// (data.cursor - len(bytes) == the cursor it holds) instead of being
	// relabelled into a plausible-looking stream here.
	st.cursor = start
	if reset && announce && !st.emit(contract.EventResync, nil) {
		return false
	}
	for len(b) > 0 {
		n := min(len(b), terminalFrameBytes)
		// The cursor moves BEFORE the frame is built because the frame's id and
		// data.cursor are the offset at the END of these bytes. If the write
		// fails the connection is over and this value is never reused — the
		// client resumes from the last id it actually received.
		st.cursor += uint64(n)
		if !st.emit(contract.EventTerminalOutput, contract.TerminalOutputData{Cursor: st.cursor, Bytes: b[:n]}) {
			return false
		}
		b = b[n:]
	}
	return true
}

// state emits a `terminal.state` frame when the derived state or the title
// changed (or unconditionally when force is set — once, after the initial
// replay, so every client learns the state it attached into).
//
// It is the ONLY place a `terminal.state` frame is built, and it delivers the
// bytes the state came from BEFORE the frame. The frame's id is the cursor
// delivered so far, and a client resuming from that id must already hold every
// byte the state describes. The pump writes the ring and the title under one
// lock ([terminal.Handle.Ingest]), so every byte the snapshot was derived from
// is in the ring by the time [terminal.Follower.State] returns, and the drain
// below puts it on the wire first. Without that drain, the poll tick could
// win the select against a pending wake and send a new title stamped with the
// cursor from before the bytes that set it.
func (st *terminalStream) state(force bool) bool {
	state, title, at := st.f.State()
	if !force && st.sentState && string(state) == st.lastState && title == st.lastTitle {
		return true
	}
	if !st.drain(true) {
		return false
	}
	st.sentState, st.lastState, st.lastTitle = true, string(state), title
	if at.IsZero() {
		at = st.now()
	}
	return st.emit(contract.EventTerminalState, contract.TerminalStateData{State: string(state), Title: title, At: at})
}
