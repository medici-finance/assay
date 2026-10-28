package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// ── a controllable clock so state derivation is time-testable without sleeps ──

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// ── fakeTarget / fakeSession: the driver stand-in every test in this brief uses ──
//
// It implements the exact [Target]/[Session] interfaces briefs 13/14/15 will,
// so the registry's attach/pump path is exercised without docker, kubectl, or a
// real PTY. Set unreachable to exercise the [ErrUnreachable] -> could-not-check
// path.
type fakeTarget struct {
	unreachable bool
	sess        *fakeSession
}

func (f *fakeTarget) Attach(_ context.Context, _ *TargetSpec, _ Size) (Session, error) {
	if f.unreachable {
		return nil, fmt.Errorf("fake target refused: %w", ErrUnreachable)
	}
	f.sess = &fakeSession{ch: make(chan []byte, 16), alive: true}
	return f.sess, nil
}

type fakeSession struct {
	ch       chan []byte
	mu       sync.Mutex
	writes   [][]byte
	alive    bool
	closed   bool  // Close() has run
	chClosed bool  // ch has been closed, by Close() or endStream()
	closeErr error // what Close returns (a failed detach), nil by default
}

func (s *fakeSession) Read() <-chan []byte { return s.ch }

func (s *fakeSession) Write(p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, append([]byte(nil), p...))
	return nil
}

func (s *fakeSession) Resize(int, int) error { return nil }

func (s *fakeSession) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// Close is idempotent (a second Close is a no-op, never a double-close panic
// on s.ch) — [Registry.pump] now calls Close itself once a session's Read()
// channel closes, so a test that also calls it directly (to end the fake
// session) must not race a double close with the pump's own call. It also
// tolerates ch already having been closed by [fakeSession.endStream] — the
// real drivers' shape, where an internal read goroutine (not Close) closes
// the channel first.
func (s *fakeSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.alive = false
	if !s.chClosed {
		s.chClosed = true
		close(s.ch)
	}
	return s.closeErr
}

// endStream closes the session's Read() channel from "underneath" — the
// shape a real driver's own internal read goroutine ends an attachment in
// (a stopped container, a hung-up local PTY), distinct from a caller
// invoking [fakeSession.Close] on the [Session] interface.
func (s *fakeSession) endStream() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.chClosed {
		return
	}
	s.chClosed = true
	close(s.ch)
}

// ── ring buffer: cursor resume + typed eviction (Verify row 2) ──

func TestRingBufferResumeFromCursor(t *testing.T) {
	t.Run("ResumeReturnsBytesAfterCursor", func(t *testing.T) {
		r := newRing(DefaultRingBytes, DefaultRingLines)
		r.Write([]byte("line-one\n"))
		mid := r.Write([]byte("line-two\n"))
		r.Write([]byte("line-three\n"))
		r.Write([]byte("line-four\n"))

		got, err := r.Since(mid)
		if err != nil {
			t.Fatalf("Since(mid) errored: %v", err)
		}
		want := []byte("line-three\nline-four\n")
		if !bytes.Equal(got, want) {
			t.Fatalf("resume from cursor %d = %q, want %q — the per-byte cursor must replay exactly the bytes after it", mid, got, want)
		}
		// A cursor at the end yields nothing new, not an error.
		if b, err := r.Since(r.Cursor()); err != nil || len(b) != 0 {
			t.Fatalf("Since(end) = (%q, %v), want (empty, nil)", b, err)
		}
	})

	t.Run("EvictedCursorIsTyped", func(t *testing.T) {
		// A tiny ring so a couple of writes scroll the first line off.
		r := newRing(8, 1000)
		early := r.Write([]byte("aaaaa\n")) // 6 bytes; early == 6
		r.Write([]byte("bbbbb\n"))          // total 12 > cap 8 -> "aaaaa\n" evicted
		_ = early

		if _, err := r.Since(0); !errors.Is(err, ErrCursorEvicted) {
			t.Fatalf("Since(0) after eviction = %v, want ErrCursorEvicted — a resume point below the base must be TYPED, never a silent truncation", err)
		}
	})

	t.Run("EvictedNotSilentlyTruncated", func(t *testing.T) {
		// The retained window still resumes exactly; eviction removes history,
		// it never hands back a byte-severed or wrong slice.
		r := newRing(8, 1000)
		r.Write([]byte("aaaaa\n"))
		base := r.Cursor() - uint64(len(r.tail(1<<20)))
		r.Write([]byte("bbbbb\n"))
		got, err := r.Since(base + 6) // the start of "bbbbb\n" after "aaaaa\n" evicted
		if err != nil {
			t.Fatalf("Since(retained) errored after eviction: %v", err)
		}
		if !bytes.Equal(got, []byte("bbbbb\n")) {
			t.Fatalf("retained resume = %q, want %q — eviction must not corrupt the retained window", got, "bbbbb\n")
		}
	})
}

// ── agent state: two-signal disagreement is could-not-check (Verify row 3) ──

func TestStateDisagreementIsCouldNotCheck(t *testing.T) {
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	// The disagreement: the OSC title says "working", but once output has been
	// silent past the idle window the PTY shows a settled shell prompt. Two
	// instruments, two stories -> could-not-check, never the optimistic busy/idle.
	clk := &fakeClock{t: base}
	h := newHandle("assay/review", &TargetSpec{Kind: schema.TargetLocal, Command: []string{"bash"}}, clk.now)
	h.Ingest([]byte("\x1b]0;assay-desk · working\x07\nrunning the thing\n$ "))
	clk.advance(300 * time.Millisecond) // past the 200ms idle window
	h.Refresh()
	if got := h.AgentState(); got != StateCouldNotCheck {
		t.Fatalf("title=working + settled prompt -> state %q, want could-not-check (the signals disagree; never round up to busy/idle)", got)
	}

	// Positive control: with an AGREEING title the same prompt is a plain idle,
	// so the machine is not simply always could-not-check.
	clk2 := &fakeClock{t: base}
	h2 := newHandle("assay/review", &TargetSpec{Kind: schema.TargetLocal, Command: []string{"bash"}}, clk2.now)
	h2.Ingest([]byte("\x1b]0;assay-desk · idle\x07\nall done\n$ "))
	clk2.advance(300 * time.Millisecond)
	h2.Refresh()
	if got := h2.AgentState(); got != StateIdle {
		t.Fatalf("title=idle + settled prompt -> state %q, want idle (positive control: the machine discriminates)", got)
	}
}

// TestDeriveStateTable pins the pure two-signal machine directly.
func TestDeriveStateTable(t *testing.T) {
	cases := []struct {
		title  TitleSignal
		prompt PromptSignal
		asks   bool
		want   AgentState
	}{
		{TitleWorking, PromptDetected, false, StateCouldNotCheck}, // disagreement
		{TitleIdle, OutputFlowing, false, StateCouldNotCheck},     // disagreement
		{TitleWorking, OutputFlowing, false, StateBusy},
		{TitleUnknown, OutputFlowing, false, StateLive},
		{TitleAsking, PromptDetected, false, StateWaiting},
		{TitleUnknown, PromptDetected, true, StateWaiting},
		{TitleUnknown, PromptDetected, false, StateIdle},
		{TitleWorking, PromptUnknown, false, StateBusy},
		{TitleUnknown, PromptUnknown, false, StateCouldNotCheck},
	}
	for _, c := range cases {
		if got := deriveState(c.title, c.prompt, c.asks); got != c.want {
			t.Errorf("deriveState(%v,%v,%v)=%q want %q", c.title, c.prompt, c.asks, got, c.want)
		}
	}
}

// ── registry: one row per configured desk; target absence & ErrUnreachable are could-not-check ──

func testCfg() *schema.File {
	return &schema.File{Cells: []schema.Cell{{
		Name: "assay",
		Loops: []schema.Loop{
			{Role: schema.RoleReview, Target: &schema.TargetSpec{Kind: schema.TargetContainer, Container: "assay-review"}},
			{Role: schema.RoleVerify}, // no target -> could-not-check row, never a 404
		},
	}}}
}

func TestRegistryResolvesDesks(t *testing.T) {
	r := NewRegistry(testCfg())
	if r.Len() != 2 {
		t.Fatalf("Len()=%d want 2 (one row per configured loop)", r.Len())
	}
	if r.WithTarget() != 1 {
		t.Fatalf("WithTarget()=%d want 1", r.WithTarget())
	}

	// A configured desk with a target but nothing attached: found, could-not-check.
	h, ok := r.Resolve(DeskID("assay", "review"))
	if !ok {
		t.Fatal("configured desk assay/review did not resolve")
	}
	if h.TargetKind != "container" {
		t.Errorf("target_kind=%q want container", h.TargetKind)
	}
	if h.State != string(StateCouldNotCheck) || h.Connected {
		t.Errorf("unattached desk state=%q connected=%v, want could-not-check / false", h.State, h.Connected)
	}
	if h.Writable {
		t.Error("writable must be false while nothing is attached — a send needs a live session (the service contract)")
	}

	// A configured desk with NO target: still a row (could-not-check), never a miss.
	hv, ok := r.Resolve(DeskID("assay", "verify"))
	if !ok {
		t.Fatal("desk with no target must resolve to a could-not-check ROW, never a 404 (spec §5.2)")
	}
	if hv.TargetKind != "" || hv.State != string(StateCouldNotCheck) {
		t.Errorf("no-target desk = kind %q state %q, want empty kind / could-not-check", hv.TargetKind, hv.State)
	}

	// An UNCONFIGURED desk id is the only miss.
	if _, ok := r.Resolve("nope/nobody"); ok {
		t.Error("an unconfigured desk id must be a miss (the handler 404s it)")
	}
}

func TestUnreachableIsCouldNotCheck(t *testing.T) {
	r := NewRegistry(testCfg())
	if err := r.Attach(context.Background(), DeskID("assay", "review"), &fakeTarget{unreachable: true}, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach with an unreachable target returned a hard error, want nil + could-not-check: %v", err)
	}
	h, _ := r.Resolve(DeskID("assay", "review"))
	if h.State != string(StateCouldNotCheck) || h.Connected {
		t.Fatalf("unreachable target -> state %q connected %v, want could-not-check / false (ErrUnreachable is a row, not a crash)", h.State, h.Connected)
	}
}

func TestAttachPumpsOutput(t *testing.T) {
	r := NewRegistry(testCfg())
	ft := &fakeTarget{}
	if err := r.Attach(context.Background(), DeskID("assay", "review"), ft, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	ft.sess.ch <- []byte("\x1b]0;assay-desk · working\x07\nhello\n")
	// Wait for the pump to ingest.
	deadline := time.Now().Add(2 * time.Second)
	for {
		h, _ := r.Resolve(DeskID("assay", "review"))
		if h.Connected && h.Cursor > 0 {
			if h.AgentIdentity != "assay-desk" {
				t.Fatalf("agent_identity=%q want assay-desk (parsed from the OSC title)", h.AgentIdentity)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pump did not ingest the session output within the deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
	_ = ft.sess.Close()
}

// TestPumpClosesSessionOnNormalExit is the Verify row for the fd/handle-leak
// fix (review finding on #137, same root cause as #135): a session that ends
// because its own Read() channel closes — the shape every real driver uses
// from inside its own read loop (a stopped container, a hung-up local PTY),
// never a caller calling Close() first — must still have Close() called on
// it by the registry's pump. Before the fix, pump dropped the session on the
// floor once the loop exited and nothing ever reached Close().
func TestPumpClosesSessionOnNormalExit(t *testing.T) {
	r := NewRegistry(testCfg())
	ft := &fakeTarget{}
	if err := r.Attach(context.Background(), DeskID("assay", "review"), ft, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	// Simulate the driver ending the attachment from underneath, the way the
	// real container/local drivers' own internal read goroutines close their
	// channel — not a caller-initiated Close().
	ft.sess.endStream()

	deadline := time.Now().Add(2 * time.Second)
	for {
		ft.sess.mu.Lock()
		closed := ft.sess.closed
		ft.sess.mu.Unlock()
		if closed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("Registry.pump did not call sess.Close() after the session's Read() channel closed on normal exit")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
