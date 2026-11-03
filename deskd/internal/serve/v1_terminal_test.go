package serve_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/serve"
	"github.com/medici-finance/assay/deskd/internal/terminal"
)

// ── the fake target: the driver stand-in (no docker, no kubectl, no PTY) ──────

// streamTarget implements [terminal.Target]; its one session is fed by the
// test. It records every Write/Resize/Close so a test can prove the stream
// route never reached the desk's session.
type streamTarget struct{ sess *streamSession }

func (f *streamTarget) Attach(context.Context, *terminal.TargetSpec, terminal.Size) (terminal.Session, error) {
	f.sess = &streamSession{ch: make(chan []byte)}
	return f.sess, nil
}

type streamSession struct {
	ch      chan []byte
	mu      sync.Mutex
	writes  int
	resizes int
	closed  bool
}

func (s *streamSession) Read() <-chan []byte { return s.ch }
func (s *streamSession) Write([]byte) error  { s.mu.Lock(); s.writes++; s.mu.Unlock(); return nil }
func (s *streamSession) Resize(int, int) error {
	s.mu.Lock()
	s.resizes++
	s.mu.Unlock()
	return nil
}
func (s *streamSession) Alive() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.closed }
func (s *streamSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *streamSession) touched() (writes, resizes int, closed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes, s.resizes, s.closed
}

const streamDesk = "assay/dispatch"

// streamFixture is a real HTTP server over the /v1 surface with a fake target
// attached to assay/dispatch. feed pushes bytes through the driver seam exactly
// as a real driver would (session channel → registry pump → ring).
type streamFixture struct {
	srv  *httptest.Server
	reg  *terminal.Registry
	sess *streamSession
}

func newStreamFixture(t *testing.T, authz serve.SurfaceAuthorizer, opts ...func(*http.Server)) *streamFixture {
	t.Helper()
	cfg := desksFixtureCfg()
	reg := terminal.NewRegistry(cfg)
	tgt := &streamTarget{}
	if err := reg.Attach(context.Background(), streamDesk, tgt, terminal.Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("attach the fake target: %v", err)
	}
	srv := httptest.NewUnstartedServer(serve.NewV1(nil, cfg, authz, nil, nil, reg, nil, nil))
	for _, o := range opts {
		o(srv.Config)
	}
	srv.Start()
	t.Cleanup(srv.Close)
	// Cleanups run last-in-first-out: drop the client connections BEFORE Close,
	// which otherwise waits forever on a stream a FAILING test left open — a
	// red test must end in its assertion, not in the 10-minute test timeout.
	t.Cleanup(srv.CloseClientConnections)
	return &streamFixture{srv: srv, reg: reg, sess: tgt.sess}
}

func (f *streamFixture) feed(b []byte) { f.sess.ch <- append([]byte(nil), b...) }

func (f *streamFixture) client() *contract.Client {
	// No client-side timeout: a stream is long-lived by design.
	return contract.NewClient(f.srv.URL, contract.WithHTTPClient(&http.Client{}))
}

func (f *streamFixture) cursor(t *testing.T) uint64 {
	t.Helper()
	h, ok := f.reg.Resolve(streamDesk)
	if !ok {
		t.Fatal("the fixture desk is not configured")
	}
	return h.Cursor
}

func waitFor(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s", within, what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// sourceBytes is deterministic, deliberately hostile terminal output: random
// bytes (so escape bytes, NULs and split UTF-8 all occur) with a newline about
// every 200 bytes so the ring's LINE cap (10 000) is not what bounds it.
func sourceBytes(n int) []byte {
	rng := rand.New(rand.NewSource(17))
	b := make([]byte, n)
	rng.Read(b)
	for i := range b {
		if b[i] == '\n' {
			b[i] = 'n'
		}
	}
	for i := 199; i < n; i += 200 {
		b[i] = '\n'
	}
	return b
}

// outputOf decodes a terminal.output frame and enforces the two per-frame
// invariants a client can check: id == data.cursor, and the frame BEGINS where
// the client's cursor already is (no gap, no replay).
func outputOf(t *testing.T, e contract.Event, have uint64) contract.TerminalOutputData {
	t.Helper()
	var d contract.TerminalOutputData
	if err := json.Unmarshal(e.Data, &d); err != nil {
		t.Fatalf("terminal.output data did not decode: %v", err)
	}
	if e.ID != strconv.FormatUint(d.Cursor, 10) {
		t.Fatalf("frame id %q != data.cursor %d — Last-Event-ID must BE the cursor", e.ID, d.Cursor)
	}
	if start := d.Cursor - uint64(len(d.Bytes)); start != have {
		t.Fatalf("frame begins at %d but the client holds cursor %d (%+d bytes: >0 is a GAP, <0 is a REPLAY)", start, have, int64(start)-int64(have))
	}
	if len(d.Bytes) > serve.TerminalFrameBytes {
		t.Fatalf("frame carries %d bytes, over the %d cap", len(d.Bytes), serve.TerminalFrameBytes)
	}
	return d
}

// TestTerminalStreamResumeIsByteExact is brief Verify row 2 — the spike's
// ANSWER 1 reproduced on the real route, over real HTTP, through
// contract.Client: 5 MB through a fake target while the client is disconnected
// 20 times and resumes with Last-Event-ID; the concatenation must sha256-equal
// the source. Then a 21st resume with a cursor older than the ring must get
// exactly one resync, and what follows must be the ring from the resync id.
func TestTerminalStreamResumeIsByteExact(t *testing.T) {
	const total = 5_000_000
	const disconnects = 20
	// The producer may run at most this far ahead of what the client has
	// received, so a resume always finds its cursor still in the 1 MiB ring.
	// (What happens when it does NOT is the 21st resume, below.)
	const maxLead = 256 << 10

	f := newStreamFixture(t, nil)
	src := sourceBytes(total)
	var received atomic.Int64

	go func() {
		rng := rand.New(rand.NewSource(18))
		for off := 0; off < total; {
			for int64(off)-received.Load() > maxLead {
				time.Sleep(time.Millisecond)
			}
			n := min(1+rng.Intn(9000), total-off) // odd chunk sizes: frame and chunk boundaries never line up
			f.feed(src[off : off+n])
			off += n
		}
	}()

	started := time.Now()
	var got []byte
	lastID := ""
	frames, resumes := 0, 0
	for cycle := 0; cycle <= disconnects; cycle++ {
		// Each cycle reads up to the next 1/21st boundary, then drops the
		// connection mid-stream. The last cycle reads to the end.
		stopAt := total * (cycle + 1) / (disconnects + 1)
		ctx, cancel := context.WithCancel(context.Background())
		es, err := f.client().TerminalStream(ctx, streamDesk, lastID)
		if err != nil {
			t.Fatalf("cycle %d: open stream (Last-Event-ID %q): %v", cycle, lastID, err)
		}
		if cycle > 0 {
			resumes++
		}
		for len(got) < stopAt {
			e, err := es.Next()
			if err != nil {
				t.Fatalf("cycle %d: read at %d/%d bytes: %v", cycle, len(got), total, err)
			}
			switch e.Type {
			case contract.EventTerminalOutput:
				d := outputOf(t, e, uint64(len(got)))
				got = append(got, d.Bytes...)
				received.Store(int64(len(got)))
				frames++
			case contract.EventResync:
				t.Fatalf("cycle %d: resync at cursor %d — the resume point was still in the ring, so a resync here means bytes were thrown away", cycle, len(got))
			}
		}
		lastID = es.LastEventID()
		cancel()
		_ = es.Close()
	}
	if resumes != disconnects {
		t.Fatalf("resumed %d times, want %d", resumes, disconnects)
	}
	if len(got) != total {
		t.Fatalf("received %d bytes, want %d", len(got), total)
	}
	if sha256.Sum256(got) != sha256.Sum256(src) {
		t.Fatal("sha256 of the bytes received across 20 resumes != sha256 of the source")
	}
	t.Logf("row 2: %d bytes in %d frames across %d disconnect/resume cycles in %s — sha256 %x",
		total, frames, disconnects, time.Since(started).Round(time.Millisecond), sha256.Sum256(got))

	// The 21st resume: cursor 1 scrolled off long ago (5 MB through a 1 MiB ring).
	waitFor(t, "every follower to be released", 5*time.Second, func() bool { return f.reg.Followers(streamDesk) == 0 })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	es, err := f.client().TerminalStream(ctx, streamDesk, "1")
	if err != nil {
		t.Fatalf("21st resume: %v", err)
	}
	defer es.Close()
	first, err := es.Next()
	if err != nil {
		t.Fatalf("21st resume: first frame: %v", err)
	}
	if first.Type != contract.EventResync {
		t.Fatalf("21st resume (evicted cursor): first frame is %q, want resync — an evicted cursor must never be served 'best effort from the oldest byte' unannounced", first.Type)
	}
	base, err := strconv.ParseUint(first.ID, 10, 64)
	if err != nil || base <= 1 || base >= total {
		t.Fatalf("resync id = %q, want the ring's base cursor (1 < base < %d)", first.ID, total)
	}
	resyncs, have := 1, base
	for have < total {
		e, err := es.Next()
		if err != nil {
			t.Fatalf("after resync: %v", err)
		}
		switch e.Type {
		case contract.EventResync:
			resyncs++
		case contract.EventTerminalOutput:
			d := outputOf(t, e, have)
			if !bytes.Equal(d.Bytes, src[have:have+uint64(len(d.Bytes))]) {
				t.Fatalf("after resync the bytes at cursor %d are not the source's bytes at that offset", have)
			}
			have = d.Cursor
		}
	}
	if resyncs != 1 {
		t.Fatalf("got %d resync frames, want exactly 1", resyncs)
	}
}

// TestTerminalStreamUnhonourableCursorsResync — every Last-Event-ID the ring
// cannot honour exactly gets ONE resync whose id is the base, then the retained
// window: a cursor from the FUTURE (a restarted service's ring began again at
// zero) and a non-numeric id. A first attach with no header gets no resync —
// it holds nothing to invalidate — and starts at the base.
func TestTerminalStreamUnhonourableCursorsResync(t *testing.T) {
	f := newStreamFixture(t, nil)
	src := []byte("line one\nline two\nline three\n")
	f.feed(src)
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return f.cursor(t) == uint64(len(src)) })

	read := func(lastID string) (types []contract.EventType, ids []string, got []byte) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		es, err := f.client().TerminalStream(ctx, streamDesk, lastID)
		if err != nil {
			t.Fatalf("Last-Event-ID %q: %v", lastID, err)
		}
		defer es.Close()
		for {
			e, err := es.Next()
			if err != nil {
				t.Fatalf("Last-Event-ID %q: %v", lastID, err)
			}
			types, ids = append(types, e.Type), append(ids, e.ID)
			if e.Type == contract.EventTerminalOutput {
				var d contract.TerminalOutputData
				_ = json.Unmarshal(e.Data, &d)
				got = append(got, d.Bytes...)
			}
			if e.Type == contract.EventTerminalState { // the state frame closes the initial replay
				return types, ids, got
			}
		}
	}

	for _, id := range []string{"999999", "not-a-cursor", "-4"} {
		types, ids, got := read(id)
		if types[0] != contract.EventResync || ids[0] != "0" {
			t.Errorf("Last-Event-ID %q: first frame = %s id %s, want resync id 0 (the base)", id, types[0], ids[0])
		}
		if !bytes.Equal(got, src) {
			t.Errorf("Last-Event-ID %q: after resync got %q, want the whole retained window %q", id, got, src)
		}
	}

	// In-buffer resume: strictly AFTER the cursor, no resync, no replay.
	types, ids, got := read("9")
	if types[0] != contract.EventTerminalOutput || !bytes.Equal(got, src[9:]) {
		t.Errorf("resume from 9: first frame %s, bytes %q — want terminal.output carrying %q", types[0], got, src[9:])
	}
	if ids[0] != strconv.Itoa(len(src)) {
		t.Errorf("resume from 9: first id = %s, want %d (strictly greater than the cursor sent)", ids[0], len(src))
	}

	// Caught-up resume: nothing owed, so the first frame is the state frame and
	// its id is the cursor the client already holds (it moved no bytes).
	types, ids, got = read(strconv.Itoa(len(src)))
	if len(got) != 0 || types[0] != contract.EventTerminalState || ids[0] != strconv.Itoa(len(src)) {
		t.Errorf("caught-up resume: frames %v ids %v bytes %q — want one terminal.state with id %d and no bytes", types, ids, got, len(src))
	}

	// First attach: no header, no resync, whole window.
	types, _, got = read("")
	if types[0] == contract.EventResync || !bytes.Equal(got, src) {
		t.Errorf("first attach: frames %v bytes %q — want no resync and the whole window", types, got)
	}
}

// TestTerminalStreamRefusesUnauthorizedDesk is brief Verify row 8 (NEGATIVE
// PATH): a viewer without permission on the desk's cell gets 403 and ZERO
// frames — and never registers a follower on the desk.
func TestTerminalStreamRefusesUnauthorizedDesk(t *testing.T) {
	deny := newStreamFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	deny.feed([]byte("secret scrollback\n"))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return deny.cursor(t) > 0 })

	rctx, rcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rcancel()
	req, _ := http.NewRequestWithContext(rctx, http.MethodGet, deny.srv.URL+"/v1/desks/assay%2Fdispatch/terminal/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		peek := make([]byte, 256)
		n, _ := resp.Body.Read(peek)
		t.Fatalf("denied viewer answered %d, want 403; it was served: %q", resp.StatusCode, peek[:n])
	}
	body, _ := io.ReadAll(resp.Body) // returns: a refusal is a closed JSON body, not an open stream
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "event-stream") {
		t.Errorf("a refusal was served as %q — the gate must answer before the stream opens", ct)
	}
	for _, leak := range []string{"data:", "event:", "id:", "secret", "c2VjcmV0"} { // c2VjcmV0 = base64("secret")
		if bytes.Contains(body, []byte(leak)) {
			t.Errorf("the 403 body carries %q — want zero frames: %s", leak, body)
		}
	}
	if n := deny.reg.Followers(streamDesk); n != 0 {
		t.Errorf("a refused request left %d follower(s) registered on the desk", n)
	}
	if _, err := deny.client().TerminalStream(context.Background(), streamDesk, ""); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("contract.Client.TerminalStream on a denied desk: err = %v, want an HTTP 403 error before any frame", err)
	}

	// The gate is per CELL: a viewer denied a different cell's repo still streams
	// this one, and the refusal above was the gate, not a broken route.
	other := newStreamFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "tracker-restricted"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	es, err := other.client().TerminalStream(ctx, streamDesk, "")
	if err != nil {
		t.Fatalf("a viewer denied a DIFFERENT cell must still stream this desk: %v", err)
	}
	es.Close()
	// ...and is refused the cell they ARE denied.
	if _, err := other.client().TerminalStream(ctx, "tracker/dispatch", ""); err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("stream of a desk in the denied cell: err = %v, want HTTP 403", err)
	}
}

// TestTerminalStreamDisconnectLeavesDeskRunning — spec §2 "attach, never
// host": a client going away releases ITS follower and its goroutine, and
// nothing else. The session is not closed, written to or resized; the desk
// keeps producing; the next client resumes without a gap. Run 25 times over so
// a per-connection leak shows up as a goroutine count, not a guess.
func TestTerminalStreamDisconnectLeavesDeskRunning(t *testing.T) {
	f := newStreamFixture(t, nil)
	f.feed([]byte("before\n"))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return f.cursor(t) == 7 })

	hc := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	client := contract.NewClient(f.srv.URL, contract.WithHTTPClient(hc))
	connectAndDrop := func() string {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		es, err := client.TerminalStream(ctx, streamDesk, "")
		if err != nil {
			t.Fatal(err)
		}
		defer es.Close()
		for {
			e, err := es.Next()
			if err != nil {
				t.Fatal(err)
			}
			if e.Type == contract.EventTerminalState {
				return es.LastEventID()
			}
		}
	}

	connectAndDrop() // warm-up: lets the server's and transport's long-lived goroutines exist before the baseline
	waitFor(t, "the warm-up follower to be released", 5*time.Second, func() bool { return f.reg.Followers(streamDesk) == 0 })
	runtime.GC()
	baseline := runtime.NumGoroutine()

	var lastID string
	for i := 0; i < 25; i++ {
		lastID = connectAndDrop()
	}
	waitFor(t, "every follower to be released after 25 disconnects", 5*time.Second, func() bool { return f.reg.Followers(streamDesk) == 0 })
	waitFor(t, "the handler goroutines to exit", 5*time.Second, func() bool {
		runtime.GC()
		return runtime.NumGoroutine() <= baseline+2
	})

	if w, r, closed := f.sess.touched(); w != 0 || r != 0 || closed {
		t.Fatalf("the stream route reached the desk's session: writes=%d resizes=%d closed=%v — a read-only follower must do none of these", w, r, closed)
	}
	if !f.reg.Alive(streamDesk) {
		t.Fatal("the desk's session is no longer alive after its stream clients disconnected")
	}

	// The desk keeps running with nobody watching, and the next client resumes
	// from the last id the old one saw, with no gap.
	f.feed([]byte("after\n"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	es, err := client.TerminalStream(ctx, streamDesk, lastID)
	if err != nil {
		t.Fatal(err)
	}
	defer es.Close()
	for {
		e, err := es.Next()
		if err != nil {
			t.Fatal(err)
		}
		if e.Type == contract.EventTerminalOutput {
			if d := outputOf(t, e, 7); string(d.Bytes) != "after\n" {
				t.Fatalf("resumed client got %q, want %q", d.Bytes, "after\n")
			}
			return
		}
	}
}

// pinnedSocketBuffer is the kernel socket buffer the stalled-peer tests pin on
// both ends of the connection. Left alone, Linux autotunes a loopback
// connection's buffers up to tcp_rmem/tcp_wmem's maximums (32 MiB receive and
// 4 MiB send by default), so a peer that "reads nothing" still absorbs tens of
// MiB before a server Write ever blocks — and the per-frame write deadline the
// test is about only starts to bite once it does. That made reclaim time a
// function of the host's TCP tuning and load, not of the deadline
// (the authorization contract, r7 finding R7-3). Setting the size explicitly also turns
// autotuning off for that socket, on Linux and darwin alike.
const pinnedSocketBuffer = 32 << 10

// withPinnedSendBuffer pins the server side of every accepted connection.
func withPinnedSendBuffer(t *testing.T) func(*http.Server) {
	return func(srv *http.Server) {
		srv.ConnState = func(c net.Conn, s http.ConnState) {
			if s == http.StateNew {
				if err := pinSendBuffer(c); err != nil {
					t.Errorf("pin the server's send buffer: %v", err)
				}
			}
		}
	}
}

func pinSendBuffer(c net.Conn) error {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return fmt.Errorf("expected a TCP connection, got %T", c)
	}
	return tc.SetWriteBuffer(pinnedSocketBuffer)
}

func TestSendBufferPinErrors(t *testing.T) {
	t.Run("non-TCP", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()
		if err := pinSendBuffer(server); err == nil {
			t.Fatal("non-TCP connection silently skipped the send-buffer pin")
		}
	})
	t.Run("invalid-TCP", func(t *testing.T) {
		conn := new(net.TCPConn)
		if err := pinSendBuffer(conn); err == nil {
			t.Fatal("failed SetWriteBuffer silently skipped the send-buffer pin")
		}
	})
}

// rawStream opens the route on a bare TCP connection so the test controls
// exactly when — and whether — the client reads. The client's receive buffer is
// pinned ([pinnedSocketBuffer]) so "not reading" is not quietly 32 MiB of
// kernel buffering.
func rawStream(t *testing.T, f *streamFixture, lastID string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", strings.TrimPrefix(f.srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.(*net.TCPConn).SetReadBuffer(pinnedSocketBuffer); err != nil {
		t.Fatalf("pin the client's receive buffer: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	req := "GET /v1/desks/assay%2Fdispatch/terminal/stream HTTP/1.1\r\nHost: deskd\r\nAccept: text/event-stream\r\n"
	if lastID != "" {
		req += "Last-Event-ID: " + lastID + "\r\n"
	}
	if _, err := conn.Write([]byte(req + "\r\n")); err != nil {
		t.Fatal(err)
	}
	return conn, bufio.NewReaderSize(conn, 1<<20)
}

// TestTerminalStreamSlowClientNeverBlocksTheDesk — backpressure. A client that
// stops reading must not slow the desk's pump by one byte (there is no
// per-client queue to fill), and when it reads again after the ring has moved
// past it, it is TOLD (resync) rather than handed a stream with a hole in it.
func TestTerminalStreamSlowClientNeverBlocksTheDesk(t *testing.T) {
	f := newStreamFixture(t, nil)
	conn, br := rawStream(t, f, "")
	waitFor(t, "the stalled client's follower", 2*time.Second, func() bool { return f.reg.Followers(streamDesk) == 1 })

	// 12 MiB through a desk whose only client reads nothing. If a slow client
	// could block the pump, this feed (an unbuffered channel the pump drains)
	// would wedge and the wait below would time out.
	const total = 12 << 20
	src := sourceBytes(total)
	fed := make(chan struct{})
	go func() {
		defer close(fed)
		for off := 0; off < total; off += 64 << 10 {
			f.feed(src[off:min(off+64<<10, total)])
		}
	}()
	select {
	case <-fed:
	case <-time.After(20 * time.Second):
		t.Fatalf("the pump wedged behind a client that is not reading: ring cursor %d of %d", f.cursor(t), total)
	}
	waitFor(t, "the ring to hold the whole feed", 5*time.Second, func() bool { return f.cursor(t) == total })

	// Now the client reads. It must see a resync, and from the LAST resync on the
	// bytes must be exactly the source from that cursor to the end.
	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	es := contract.NewEventStream(skipHTTPHeader(t, br), nil)
	var have uint64
	var tail []byte
	resyncs := 0
	for have < total {
		e, err := es.Next()
		if err != nil {
			t.Fatalf("slow client read (cursor %d, %d resyncs): %v", have, resyncs, err)
		}
		switch e.Type {
		case contract.EventResync:
			resyncs++
			have, _ = strconv.ParseUint(e.ID, 10, 64)
			tail = tail[:0]
		case contract.EventTerminalOutput:
			d := outputOf(t, e, have)
			tail = append(tail, d.Bytes...)
			have = d.Cursor
		}
	}
	if resyncs == 0 {
		t.Fatal("12 MiB went through a 1 MiB ring while the client read nothing, and the client was never told — bytes were skipped silently")
	}
	if !bytes.Equal(tail, src[total-len(tail):]) {
		t.Fatal("after the last resync the client's bytes are not the source's tail")
	}
}

// TestTerminalStreamStalledClientIsReclaimed — a peer that stops reading
// WITHOUT closing must not hold a handler goroutine and a follower forever: the
// per-frame write deadline ends the stream server-side.
func TestTerminalStreamStalledClientIsReclaimed(t *testing.T) {
	const writeTimeout = 300 * time.Millisecond
	defer serve.SetTerminalStreamTimings(0, 0, writeTimeout)()
	f := newStreamFixture(t, nil, withPinnedSendBuffer(t))
	rawStream(t, f, "") // connected, never read, never closed by the client during the test
	waitFor(t, "the stalled client's follower", 2*time.Second, func() bool { return f.reg.Followers(streamDesk) == 1 })

	// Keep output flowing so the handler always has a frame to write — PACED,
	// at about 6 MiB/s. That outruns the pinned buffers within a frame or two;
	// an unpaced feed of back-to-back 1 MiB chunks instead holds the desk's lock
	// almost continuously, so the handler's own state read and the follower's
	// Close queue behind the pump and reclaim time measures lock contention and
	// GC, not the write deadline.
	src := sourceBytes(64 << 10)
	stop := make(chan struct{})
	defer close(stop)
	started := time.Now()
	go func() {
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
			}
			select {
			case <-stop:
				return
			case f.sess.ch <- src:
			}
		}
	}()
	waitFor(t, "the stalled client's follower to be reclaimed by the write deadline", 20*time.Second, func() bool {
		return f.reg.Followers(streamDesk) == 0
	})
	// With the buffers pinned and the feed paced, reclaim is one write deadline
	// plus the time to fill a few hundred KiB of buffer: 0.4–0.5 s on an idle
	// Linux host, at most 1.7 s with every core busy. The unpinned, unpaced
	// shape took 3–11 s idle and 7–14 s loaded, a few seconds short of the 20 s
	// wait above. The bound sits between the two, so a regression to that shape
	// is a red here rather than an occasional timeout.
	const reclaimBound = 5 * time.Second
	took := time.Since(started)
	t.Logf("stalled client reclaimed %s after output started (write deadline %s)", took.Round(time.Millisecond), writeTimeout)
	if took > reclaimBound {
		t.Errorf("reclaim took %s, over %s for a %s write deadline — something other than the deadline (kernel buffering, a saturated pump) is deciding when a stalled peer is dropped", took.Round(time.Millisecond), reclaimBound, writeTimeout)
	}
}

func skipHTTPHeader(t *testing.T, br *bufio.Reader) io.Reader {
	t.Helper()
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("read the response head: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream answered %d, want 200", resp.StatusCode)
	}
	return resp.Body // de-chunks the body
}

// TestTerminalStreamFramingHeartbeatAndState — the wire conventions: the four
// SSE headers /v1/events sets, an `id:`/`event:`/`data:` frame whose data is
// the whole contract.Event, a `: heartbeat` comment on an idle desk, and a
// terminal.state frame after the replay and again when the OSC title changes —
// its id being the cursor already delivered, so Last-Event-ID stays a cursor.
func TestTerminalStreamFramingHeartbeatAndState(t *testing.T) {
	defer serve.SetTerminalStreamTimings(40*time.Millisecond, 20*time.Millisecond, 0)()
	f := newStreamFixture(t, nil)
	f.feed([]byte("hello\n"))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return f.cursor(t) == 6 })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.srv.URL+"/v1/desks/assay%2Fdispatch/terminal/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	for k, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	title := "\x1b]0;assay-desk-app · working\x07"
	var lines []string
	sc := bufio.NewScanner(resp.Body)
	sawHeartbeat, fedTitle, sawTitle := false, false, false
	for sc.Scan() && !(sawHeartbeat && sawTitle) {
		line := sc.Text()
		lines = append(lines, line)
		if line == ": heartbeat" {
			sawHeartbeat = true
			if !fedTitle {
				fedTitle = true
				f.feed([]byte(title))
			}
		}
		if v, ok := strings.CutPrefix(line, "data: "); ok {
			var e contract.Event
			if err := json.Unmarshal([]byte(v), &e); err != nil {
				t.Fatalf("data line is not a contract.Event: %v", err)
			}
			if e.Type != contract.EventTerminalState {
				continue
			}
			var d contract.TerminalStateData
			if err := json.Unmarshal(e.Data, &d); err != nil || d.State == "" || d.At.IsZero() {
				t.Fatalf("terminal.state data = %s (err %v), want {state, title, at}", e.Data, err)
			}
			if d.Title == "assay-desk-app · working" {
				sawTitle = true
				if want := strconv.Itoa(6 + len(title)); e.ID != want {
					t.Errorf("state frame id = %s, want %s — the cursor delivered so far", e.ID, want)
				}
			}
		}
	}
	if !sawHeartbeat || !sawTitle {
		t.Fatalf("heartbeat=%v titleState=%v; stream so far:\n%s", sawHeartbeat, sawTitle, strings.Join(lines, "\n"))
	}
	want := []string{"id: 6", "event: terminal.output"}
	if len(lines) < 3 || lines[0] != want[0] || lines[1] != want[1] || !strings.HasPrefix(lines[2], `data: {"type":"terminal.output","id":"6"`) {
		t.Errorf("first frame = %q, want id:/event:/data: in the /v1/events framing", lines[:min(3, len(lines))])
	}
}

// TestTerminalStreamStateFrameFollowsItsBytes is the deterministic form of the
// race FramingHeartbeatAndState hit about one run in ten on Linux
// (the authorization contract, r7 finding R7-2). The bytes that set a title are in the
// ring, their wake has not been served, and the poll tick runs the state step
// first. The state frame's id is the Last-Event-ID a client resumes from, so it
// must cover those bytes: they go out first, then the frame, whose id is the
// cursor after them. Before the fix the step emitted the new title stamped with
// the stale cursor (6) and the bytes followed it.
func TestTerminalStreamStateFrameFollowsItsBytes(t *testing.T) {
	f := newStreamFixture(t, nil)
	title := "\x1b]0;assay-desk-app · working\x07"
	f.feed([]byte("hello\n"))
	f.feed([]byte(title))
	end := uint64(6 + len(title))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return f.cursor(t) == end })

	fl, ok := f.reg.Follow(streamDesk)
	if !ok {
		t.Fatal("the fixture desk is not configured")
	}
	defer fl.Close()
	// The client holds cursor 6 ("hello\n"); the title bytes are owed.
	es := contract.NewEventStream(bytes.NewReader(serve.TerminalStateStep(fl, 6)), nil)
	have := uint64(6)
	var stateSeen bool
	for {
		e, err := es.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read the state step's frames: %v", err)
		}
		switch e.Type {
		case contract.EventTerminalOutput:
			if stateSeen {
				t.Fatalf("a terminal.output frame (id %s) came AFTER the state frame — the state was sent before the bytes it describes", e.ID)
			}
			have = outputOf(t, e, have).Cursor
		case contract.EventTerminalState:
			stateSeen = true
			var d contract.TerminalStateData
			if err := json.Unmarshal(e.Data, &d); err != nil {
				t.Fatalf("terminal.state data did not decode: %v", err)
			}
			if d.Title != "assay-desk-app · working" {
				t.Fatalf("state frame title = %q, want the title the owed bytes set", d.Title)
			}
			if want := strconv.FormatUint(end, 10); e.ID != want {
				t.Fatalf("state frame id = %s, want %s — a client resuming from this id would never be sent the bytes that set the title", e.ID, want)
			}
		default:
			t.Fatalf("unexpected %s frame from the state step", e.Type)
		}
	}
	if !stateSeen || have != end {
		t.Fatalf("state frame seen=%v, bytes delivered up to %d — want the owed bytes (to %d) and then the state frame", stateSeen, have, end)
	}
}

// TestTerminalStreamIsReadOnlyAndHonest — the route accepts no write
// verb, ignores a request body, 404s an unknown desk, 503s (could-not-check)
// when no registry is wired, and streams a could-not-check state — not a 404 —
// for a configured desk with nothing attached (spec §5.2).
func TestTerminalStreamIsReadOnlyAndHonest(t *testing.T) {
	f := newStreamFixture(t, nil)
	const path = "/v1/desks/assay%2Fdispatch/terminal/stream"
	for _, verb := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, _ := http.NewRequest(verb, f.srv.URL+path, strings.NewReader("rm -rf /\n"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s %s answered %d, want 405", verb, path, resp.StatusCode)
		}
	}
	// A GET carrying a body: the body must never reach the session.
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.srv.URL+path, strings.NewReader("echo pwned\n"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	resp.Body.Close()
	if w, r, closed := f.sess.touched(); w != 0 || r != 0 || closed {
		t.Fatalf("a request to the stream route reached the session: writes=%d resizes=%d closed=%v", w, r, closed)
	}

	for _, unknown := range []string{"assay/nope", "nocell/dispatch", "noslash"} {
		if _, err := f.client().TerminalStream(context.Background(), unknown, ""); err == nil || !strings.Contains(err.Error(), "404") {
			t.Errorf("unknown desk %q: err = %v, want HTTP 404", unknown, err)
		}
	}

	unwired := httptest.NewServer(serve.NewV1(nil, desksFixtureCfg(), nil, nil, nil, nil, nil, nil))
	defer unwired.Close()
	if _, err := contract.NewClient(unwired.URL).TerminalStream(context.Background(), streamDesk, ""); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("no registry wired: err = %v, want HTTP 503 could-not-check", err)
	}

	// assay/review is configured but nothing is attached to it.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	es, err := f.client().TerminalStream(ctx2, "assay/review", "")
	if err != nil {
		t.Fatalf("a configured desk with no target must stream (could-not-check), not refuse: %v", err)
	}
	defer es.Close()
	e, err := es.Next()
	if err != nil {
		t.Fatal(err)
	}
	var d contract.TerminalStateData
	_ = json.Unmarshal(e.Data, &d)
	if e.Type != contract.EventTerminalState || d.State != string(terminal.StateCouldNotCheck) {
		t.Errorf("unattached desk: first frame %s state %q, want terminal.state could-not-check", e.Type, d.State)
	}
}

// TestTerminalHandleReadRefusesUnauthorizedCell — the sibling handle read,
// GET /v1/desks/{id}/terminal, carries the SAME per-cell gate as the stream.
// With a driver attached the handle is live data — OSC title, agent identity,
// derived state, cursor — of a desk whose cell may map to a private repo the
// viewer cannot read. A denied viewer gets 403 and none of it.
//
// No-enumeration: the gate is decided from the CELL named in the id, before the
// desk is looked up, so inside a denied cell a desk that exists and one that
// does not answer identically — 403-vs-404 cannot map a cell's desks.
func TestTerminalHandleReadRefusesUnauthorizedCell(t *testing.T) {
	const title = "\x1b]0;assay-desk-app · working on the private thing\x07"
	get := func(f *streamFixture, id string) (int, string) {
		t.Helper()
		resp, err := http.Get(f.srv.URL + "/v1/desks/" + strings.ReplaceAll(id, "/", "%2F") + "/terminal")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	deny := newStreamFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "console"})
	deny.feed([]byte(title + "secret scrollback\n"))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return deny.cursor(t) > 0 })

	code, body := get(deny, streamDesk)
	if code != http.StatusForbidden {
		t.Errorf("denied viewer read another cell's handle: answered %d, want 403; it was served: %s", code, body)
	}
	for _, leak := range []string{"assay-desk-app", "working on the private thing", `"cursor"`, `"state"`, `"target_kind"`} {
		if strings.Contains(body, leak) {
			t.Errorf("the refusal body carries %q — a denied viewer must learn nothing of the desk: %s", leak, body)
		}
	}
	// No enumeration: a real desk and a made-up one in the denied cell are
	// indistinguishable.
	codeReal, bodyReal := get(deny, "assay/review")
	codeFake, bodyFake := get(deny, "assay/no-such-desk")
	if codeReal != http.StatusForbidden || codeFake != http.StatusForbidden || bodyReal != bodyFake {
		t.Errorf("denied cell: existing desk → %d %q, non-existent desk → %d %q — want identical 403s", codeReal, bodyReal, codeFake, bodyFake)
	}

	// The gate is per cell, and an authorized viewer still gets the live handle.
	allow := newStreamFixture(t, denyRepoAuthz{denyOwner: "example-org", denyName: "tracker-restricted"})
	allow.feed([]byte(title))
	waitFor(t, "the ring to take the bytes", 2*time.Second, func() bool { return allow.cursor(t) > 0 })
	if code, body := get(allow, streamDesk); code != http.StatusOK || !strings.Contains(body, "assay-desk-app") {
		t.Errorf("authorized viewer: answered %d %s, want 200 with the live handle", code, body)
	}
	if code, _ := get(allow, "tracker/dispatch"); code != http.StatusForbidden {
		t.Errorf("the cell this viewer IS denied answered %d, want 403", code)
	}
	// Unknown desk in a readable cell, and an unknown cell, stay 404.
	for _, id := range []string{"assay/no-such-desk", "nocell/dispatch"} {
		if code, _ := get(allow, id); code != http.StatusNotFound {
			t.Errorf("unknown desk %q answered %d, want 404", id, code)
		}
	}
}
