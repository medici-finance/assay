package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

// send_test.go — the registry half (layer d) of the service contract's send path:
// atomic wait-then-send, the epoch binding, the lost-ack receipts, the bounds,
// and the custom session store. The HTTP half is internal/serve's
// v1_terminal_actions_test.go.

const sendDesk = "assay/review"

// sendFixture is a registry over testCfg() on a controllable clock with a fake
// session attached to assay/review.
type sendFixture struct {
	r   *Registry
	clk *fakeClock
	tgt *fakeTarget
}

func newSendFixture(t *testing.T) *sendFixture {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	r := newRegistryWithClock(testCfg(), clk.now)
	tgt := &fakeTarget{}
	if err := r.Attach(context.Background(), sendDesk, tgt, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	restore := sendPoll
	sendPoll = time.Millisecond
	t.Cleanup(func() { sendPoll = restore })
	return &sendFixture{r: r, clk: clk, tgt: tgt}
}

// atPrompt puts the desk at a settled shell prompt: a prompt line, then the
// idle window elapses on the fake clock.
func (f *sendFixture) atPrompt(t *testing.T) {
	t.Helper()
	f.r.handle(sendDesk).Ingest([]byte("user@box:~$ "))
	f.clk.advance(DefaultIdleWindow + 50*time.Millisecond)
}

// busy makes the desk look mid-thought: output just arrived and the clock
// does not move, so the idle window never elapses.
func (f *sendFixture) busy(t *testing.T) {
	t.Helper()
	f.r.handle(sendDesk).Ingest([]byte("compiling step 3 of 9...\n"))
}

func (f *sendFixture) epoch() string { return f.r.Epoch(sendDesk) }

func (f *sendFixture) writes() [][]byte {
	f.tgt.sess.mu.Lock()
	defer f.tgt.sess.mu.Unlock()
	return append([][]byte(nil), f.tgt.sess.writes...)
}

func bind(cell, epoch, rid string) SendBinding {
	return SendBinding{Cell: cell, Epoch: epoch, RequestID: rid}
}

func TestWaitThenSendWritesOnceAtPrompt(t *testing.T) {
	f := newSendFixture(t)
	if h, _ := f.r.Resolve(sendDesk); !h.Writable || h.Epoch == "" || h.Epoch != f.epoch() {
		t.Fatalf("an attached handle reads writable=%v epoch=%q, want writable with its epoch", h.Writable, h.Epoch)
	}
	f.atPrompt(t)
	rc, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("echo hi\n"), contract.SendWaitPrompt, time.Second, bind("assay", f.epoch(), "r1"))
	if err != nil {
		t.Fatalf("send at a settled prompt refused: %v", err)
	}
	if rc.Outcome != contract.SendSent {
		t.Fatalf("outcome = %q, want sent", rc.Outcome)
	}
	if w := f.writes(); len(w) != 1 || !bytes.Equal(w[0], []byte("echo hi\n")) {
		t.Fatalf("session writes = %q, want exactly one write of the input", w)
	}
	if rc.StateAfter != StateIdle {
		t.Errorf("state_after = %q, want idle", rc.StateAfter)
	}
}

// TestWaitThenSendTimeoutWritesNothing — the atomicity negative path at the
// registry: a desk that never reaches its prompt gets NOTHING typed into it.
func TestWaitThenSendTimeoutWritesNothing(t *testing.T) {
	f := newSendFixture(t)
	f.busy(t)
	rc, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("rm -rf build\n"), contract.SendWaitPrompt, 60*time.Millisecond, bind("assay", f.epoch(), ""))
	if !errors.Is(err, ErrSendTimeout) {
		t.Fatalf("err = %v, want ErrSendTimeout", err)
	}
	if rc.Outcome != contract.SendNotSent {
		t.Errorf("outcome = %q, want not-sent", rc.Outcome)
	}
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("a timed-out send wrote %q — a timeout must write nothing", w)
	}
	// wait_for: idle behaves the same on a desk that keeps talking.
	if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("x\n"), contract.SendWaitIdle, 40*time.Millisecond, bind("assay", f.epoch(), "")); !errors.Is(err, ErrSendTimeout) {
		t.Fatalf("idle wait on a flowing desk: err = %v, want ErrSendTimeout", err)
	}
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("idle-wait timeout wrote %q", w)
	}
}

// TestWaitThenSendHoldsTheLockAcrossWaitAndWrite — two sends race for one desk:
// the second cannot interleave between the first's wait and its write, so the
// session sees two whole writes in arrival order.
func TestWaitThenSendHoldsTheLockAcrossWaitAndWrite(t *testing.T) {
	f := newSendFixture(t)
	f.busy(t)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errs[0] = f.r.WaitThenSend(context.Background(), sendDesk, []byte("first\n"), contract.SendWaitPrompt, 2*time.Second, bind("assay", f.epoch(), "a"))
	}()
	// Let the first send take the lock and start waiting.
	waitUntil(t, func() bool { h := f.r.handle(sendDesk); h.mu.Lock(); defer h.mu.Unlock(); return h.send.pending == 1 })
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errs[1] = f.r.WaitThenSend(context.Background(), sendDesk, []byte("second\n"), contract.SendWaitNone, 2*time.Second, bind("assay", f.epoch(), "b"))
	}()
	waitUntil(t, func() bool { h := f.r.handle(sendDesk); h.mu.Lock(); defer h.mu.Unlock(); return h.send.pending == 2 })
	time.Sleep(20 * time.Millisecond)
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("a wait_for:none send interleaved while another send held the lock: writes %q", w)
	}
	f.atPrompt(t)
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("sends refused: %v / %v", errs[0], errs[1])
	}
	w := f.writes()
	if len(w) != 2 || string(w[0]) != "first\n" || string(w[1]) != "second\n" {
		t.Fatalf("writes = %q, want [first second] whole and in order", w)
	}
}

// TestWaitThenSendEpochChanged — Report F-2: a send is bound to the session it
// was composed against. A session that changes while it waits, a stale epoch
// at arrival, and a restarted registry all refuse — the input is never
// delivered to a replacement session.
func TestWaitThenSendEpochChanged(t *testing.T) {
	t.Run("session replaced during the wait", func(t *testing.T) {
		f := newSendFixture(t)
		f.busy(t)
		old := f.epoch()
		oldSess := f.tgt.sess
		done := make(chan error, 1)
		go func() {
			_, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("deploy\n"), contract.SendWaitPrompt, 2*time.Second, bind("assay", old, "q1"))
			done <- err
		}()
		waitUntil(t, func() bool { h := f.r.handle(sendDesk); h.mu.Lock(); defer h.mu.Unlock(); return h.send.pending == 1 })
		// A re-attach: a new session (and a new epoch) replaces the old one.
		// The desk stays busy, so the wait itself is never satisfied: the
		// waiting send must be CANCELLED by the epoch change at once, not
		// discovered at its deadline (and never delivered to the new session).
		// One session per desk (the service contract, ErrAlreadyAttached): a
		// replacement is the old session going away, then a new attach —
		// the only shape the attach lifecycle can produce.
		replacedAt := time.Now()
		f.r.handle(sendDesk).markUnreachable()
		if err := f.r.Attach(context.Background(), sendDesk, f.tgt, Size{Cols: 80, Rows: 24}); err != nil {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, ErrEpochChanged) {
			t.Fatalf("err = %v, want ErrEpochChanged", err)
		}
		if took := time.Since(replacedAt); took > time.Second {
			t.Fatalf("the epoch change cancelled the waiting send only after %s — it must cancel at once", took)
		}
		// Now the NEW session reaches its prompt: the cancelled send stays
		// cancelled.
		f.atPrompt(t)
		time.Sleep(20 * time.Millisecond)
		if f.epoch() == old {
			t.Fatal("a re-attach did not move the epoch")
		}
		if len(oldSess.writes) != 0 || len(f.writes()) != 0 {
			t.Fatalf("queued input reached a session: old %q new %q", oldSess.writes, f.writes())
		}
	})
	t.Run("stale epoch at arrival", func(t *testing.T) {
		f := newSendFixture(t)
		f.atPrompt(t)
		stale := f.epoch()
		f.r.handle(sendDesk).markUnreachable()
		if err := f.r.Attach(context.Background(), sendDesk, f.tgt, Size{}); err != nil {
			t.Fatal(err)
		}
		f.atPrompt(t)
		if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("ls\n"), contract.SendWaitNone, time.Second, bind("assay", stale, "")); !errors.Is(err, ErrEpochChanged) {
			t.Fatalf("err = %v, want ErrEpochChanged", err)
		}
		if len(f.writes()) != 0 {
			t.Fatalf("a stale-epoch send wrote %q", f.writes())
		}
	})
	t.Run("restart: a new registry never re-issues an epoch", func(t *testing.T) {
		f := newSendFixture(t)
		before := f.epoch()
		g := newSendFixture(t) // a restarted deskd: same config, same attach count
		if g.epoch() == before {
			t.Fatalf("two registries issued the same epoch %q — a restart could deliver old input to a new session", before)
		}
		g.atPrompt(t)
		if _, err := g.r.WaitThenSend(context.Background(), sendDesk, []byte("ls\n"), contract.SendWaitNone, time.Second, bind("assay", before, "")); !errors.Is(err, ErrEpochChanged) {
			t.Fatalf("pre-restart epoch after restart: err = %v, want ErrEpochChanged", err)
		}
		if len(g.writes()) != 0 {
			t.Fatalf("a pre-restart epoch wrote %q into the new session", g.writes())
		}
	})
	t.Run("missing epoch", func(t *testing.T) {
		f := newSendFixture(t)
		if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("ls\n"), contract.SendWaitNone, time.Second, bind("assay", "", "")); !errors.Is(err, ErrEpochRequired) {
			t.Fatalf("err = %v, want ErrEpochRequired", err)
		}
	})
}

// lossySession is a session whose Write hands the bytes over and then fails —
// the lost-acknowledgment shape: the desk may well have the input, the service
// cannot know.
type lossySession struct{ *fakeSession }

func (s lossySession) Write(p []byte) error {
	_ = s.fakeSession.Write(p)
	return errors.New("broken pipe after write")
}

type lossyTarget struct{ sess *fakeSession }

func (l *lossyTarget) Attach(context.Context, *TargetSpec, Size) (Session, error) {
	l.sess = &fakeSession{ch: make(chan []byte, 16), alive: true}
	return lossySession{l.sess}, nil
}

// TestTerminalSendLostAck — Report F-2: a write that does not report success
// is outcome-unknown (never "not sent"), is recorded, and is never replayed —
// a retry with the same request id returns the receipt and writes nothing; a
// lost HTTP ack on a successful send is reconciled the same way; and after a
// restart the uncertainty stays explicit (the old epoch refuses) rather than
// the input being re-sent into the new session.
func TestTerminalSendLostAck(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)}
	r := newRegistryWithClock(testCfg(), clk.now)
	lt := &lossyTarget{}
	if err := r.Attach(context.Background(), sendDesk, lt, Size{}); err != nil {
		t.Fatal(err)
	}
	ep := r.Epoch(sendDesk)
	rc, err := r.WaitThenSend(context.Background(), sendDesk, []byte("git push\n"), contract.SendWaitNone, time.Second, bind("assay", ep, "lost-1"))
	if !errors.Is(err, ErrOutcomeUnknown) || rc.Outcome != contract.SendOutcomeUnknown {
		t.Fatalf("lossy write: err=%v outcome=%q, want ErrOutcomeUnknown / outcome-unknown", err, rc.Outcome)
	}
	retry, err := r.WaitThenSend(context.Background(), sendDesk, []byte("git push\n"), contract.SendWaitNone, time.Second, bind("assay", ep, "lost-1"))
	if err != nil || !retry.Duplicate || retry.Outcome != contract.SendOutcomeUnknown {
		t.Fatalf("retry with the same request id: err=%v dup=%v outcome=%q — want the recorded outcome-unknown receipt, nothing re-sent", err, retry.Duplicate, retry.Outcome)
	}
	if n := len(lt.sess.writes); n != 1 {
		t.Fatalf("the desk received %d writes, want exactly 1 — an unknown outcome must never be replayed", n)
	}

	// A successful send whose HTTP acknowledgment was lost: the retry is
	// reconciled against the receipt.
	f := newSendFixture(t)
	f.atPrompt(t)
	if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("make\n"), contract.SendWaitNone, time.Second, bind("assay", f.epoch(), "ok-1")); err != nil {
		t.Fatal(err)
	}
	again, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("make\n"), contract.SendWaitNone, time.Second, bind("assay", f.epoch(), "ok-1"))
	if err != nil || !again.Duplicate || again.Outcome != contract.SendSent {
		t.Fatalf("duplicate request: err=%v dup=%v outcome=%q, want the sent receipt", err, again.Duplicate, again.Outcome)
	}
	if n := len(f.writes()); n != 1 {
		t.Fatalf("duplicate request wrote %d times, want 1", n)
	}

	// Restart after the unknown outcome: the receipts died with the process;
	// the retry's old epoch is refused, so the uncertainty stays explicit.
	restarted := newRegistryWithClock(testCfg(), clk.now)
	lt2 := &lossyTarget{}
	if err := restarted.Attach(context.Background(), sendDesk, lt2, Size{}); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.WaitThenSend(context.Background(), sendDesk, []byte("git push\n"), contract.SendWaitNone, time.Second, bind("assay", ep, "lost-1")); !errors.Is(err, ErrEpochChanged) {
		t.Fatalf("retry after restart: err = %v, want ErrEpochChanged (not-sent, never replayed)", err)
	}
	if n := len(lt2.sess.writes); n != 0 {
		t.Fatalf("a post-restart retry wrote %d times into the new session", n)
	}
}

// TestWaitThenSendScopeAndBounds — layer (d) and the Report F-2 bounds, each
// with the upper layers bypassed (the registry called directly).
func TestWaitThenSendScopeAndBounds(t *testing.T) {
	f := newSendFixture(t)
	f.atPrompt(t)
	ep := f.epoch()
	cases := []struct {
		name  string
		id    string
		in    []byte
		b     SendBinding
		wait  contract.SendWait
		want  error
		touch bool
	}{
		{"desk outside the caller's cell", sendDesk, []byte("ls\n"), bind("tracker", ep, ""), contract.SendWaitNone, ErrWrongCell, false},
		{"no cell at all", sendDesk, []byte("ls\n"), bind("", ep, ""), contract.SendWaitNone, ErrWrongCell, false},
		{"unknown desk", "assay/nope", []byte("ls\n"), bind("assay", ep, ""), contract.SendWaitNone, ErrNoSuchDesk, false},
		{"desk with no live session", "assay/verify", []byte("ls\n"), bind("assay", "x.1", ""), contract.SendWaitNone, ErrNotWritable, false},
		{"input too large", sendDesk, bytes.Repeat([]byte("a"), MaxSendBytes+1), bind("assay", ep, ""), contract.SendWaitNone, ErrInputTooLarge, false},
		{"empty input", sendDesk, nil, bind("assay", ep, ""), contract.SendWaitNone, ErrInputEmpty, false},
		{"bad wait", sendDesk, []byte("ls\n"), bind("assay", ep, ""), contract.SendWait("soon"), ErrBadWait, false},
		{"bad request id", sendDesk, []byte("ls\n"), bind("assay", ep, "has space"), contract.SendWaitNone, ErrBadRequestID, false},
	}
	for _, c := range cases {
		_, err := f.r.WaitThenSend(context.Background(), c.id, c.in, c.wait, time.Second, c.b)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("a refused send reached the session: %q", w)
	}

	// Revoked authority after the wait: nothing is written.
	revoked := bind("assay", ep, "")
	revoked.Revalidate = func() error { return errors.New("session ended") }
	if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("ls\n"), contract.SendWaitNone, time.Second, revoked); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked: err = %v, want ErrRevoked", err)
	}
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("a revoked send wrote %q", w)
	}

	// The pending bound: MaxPendingSends waits queue, the next is refused, and
	// a request id already in flight is refused rather than doubled.
	f.busy(t)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < MaxPendingSends; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = f.r.WaitThenSend(ctx, sendDesk, []byte("q\n"), contract.SendWaitPrompt, 5*time.Second, bind("assay", ep, fmt.Sprintf("p%d", i)))
		}(i)
	}
	waitUntil(t, func() bool {
		h := f.r.handle(sendDesk)
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.send.pending == MaxPendingSends
	})
	if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("q\n"), contract.SendWaitNone, time.Second, bind("assay", ep, "")); !errors.Is(err, ErrTooManyPending) {
		t.Errorf("send past the pending bound: err = %v, want ErrTooManyPending", err)
	}
	cancel()
	wg.Wait()
	if w := f.writes(); len(w) != 0 {
		t.Fatalf("canceled waits wrote %q", w)
	}
}

// TestSessionsOpenOnConfiguredTargetOnly — the session store: a session opens
// on the configured desk's own target with the driver for the configured kind,
// never on another custom session or an unknown/untargeted desk; it shows as a
// custom row; close refuses a configured desk and forgets a custom one.
func TestSessionsOpenOnConfiguredTargetOnly(t *testing.T) {
	r := NewRegistry(testCfg())
	r.declareAttachMode(AttachModeConfigured, linuxHost) // a running lifecycle (obligation (vi))
	var askedKind schema.TargetKind
	var gotSpec *TargetSpec
	tgt := &specTarget{}
	driver := func(k schema.TargetKind) (Target, error) { askedKind = k; return tgt, nil }

	if _, err := r.OpenSession(context.Background(), "assay/nope", "assay", "operator", "", driver, Size{}); !errors.Is(err, ErrNoSuchDesk) {
		t.Errorf("unknown on: err = %v, want ErrNoSuchDesk", err)
	}
	if _, err := r.OpenSession(context.Background(), "assay/verify", "assay", "operator", "", driver, Size{}); !errors.Is(err, ErrNoTarget) {
		t.Errorf("desk with no target: err = %v, want ErrNoTarget", err)
	}
	if _, err := r.OpenSession(context.Background(), sendDesk, "tracker", "operator", "", driver, Size{}); !errors.Is(err, ErrWrongCell) {
		t.Errorf("wrong cell: err = %v, want ErrWrongCell", err)
	}
	row, err := r.OpenSession(context.Background(), sendDesk, "assay", "operator", "scratch", driver, Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("open on a configured, targeted desk: %v", err)
	}
	gotSpec = tgt.spec
	if askedKind != schema.TargetContainer || gotSpec == nil || gotSpec.Container != "assay-review" {
		t.Fatalf("driver asked for kind %q and attached to %+v — want the CONFIGURED desk's own kind and spec", askedKind, gotSpec)
	}
	if !strings.HasPrefix(row.ID, "assay/custom/") || row.On != sendDesk || row.OpenedBy != "operator" || row.TargetKind != "container" {
		t.Fatalf("row = %+v", row)
	}
	if _, err := r.OpenSession(context.Background(), row.ID, "assay", "operator", "", driver, Size{}); !errors.Is(err, ErrNotConfiguredDesk) {
		t.Errorf("session on a custom session: err = %v, want ErrNotConfiguredDesk", err)
	}
	var customRows int
	for _, d := range r.DeskRows(&testCfg().Cells[0]) {
		if d.Group == "custom" && d.ID == row.ID {
			customRows++
		}
	}
	if customRows != 1 {
		t.Fatalf("the open session is not a custom row in the desk registry")
	}
	// Close: a configured desk is refused on the handle's own flag.
	if err := r.CloseSession(sendDesk, "assay"); !errors.Is(err, ErrConfiguredDesk) {
		t.Fatalf("close configured desk: err = %v, want ErrConfiguredDesk", err)
	}
	if err := r.CloseSession(row.ID, "tracker"); !errors.Is(err, ErrWrongCell) {
		t.Fatalf("close from another cell: err = %v, want ErrWrongCell", err)
	}
	if err := r.CloseSession(row.ID, "assay"); err != nil {
		t.Fatalf("close custom: %v", err)
	}
	if !tgt.sess.closed {
		t.Error("closing the custom session did not close its driver session")
	}
	if _, ok := r.Resolve(row.ID); ok || len(r.Sessions("assay")) != 0 {
		t.Fatal("a closed session still resolves")
	}
	if err := r.CloseSession(row.ID, "assay"); !errors.Is(err, ErrNoSuchSession) {
		t.Fatalf("second close: err = %v, want ErrNoSuchSession", err)
	}
	// No driver linked for the kind: could-not-check, nothing opened.
	if _, err := r.OpenSession(context.Background(), sendDesk, "assay", "operator", "", func(schema.TargetKind) (Target, error) { return nil, ErrNoDriver }, Size{}); !errors.Is(err, ErrNoDriver) {
		t.Fatalf("no driver: err = %v, want ErrNoDriver", err)
	}
	if len(r.Sessions("assay")) != 0 {
		t.Fatal("a failed open left a row behind")
	}
}

// TestSessionOpenRefusesPodDeskBeforeDriver — review pr192-F1(a): deskd holds
// no cluster access grant, so a custom session on a pod desk is
// ErrClusterUnauthorized and the driver resolver is NEVER called (the pod
// driver's factory would load a kubeconfig / in-cluster credential and contact
// an API server). Nothing is reserved in the session store.
func TestSessionOpenRefusesPodDeskBeforeDriver(t *testing.T) {
	cfg := testCfg()
	cfg.Cells[0].Loops = append(cfg.Cells[0].Loops, schema.Loop{
		Role:   schema.RoleDispatch,
		Target: &schema.TargetSpec{Kind: schema.TargetPod, Namespace: "loops", Pod: "dispatch-0"},
	})
	r := NewRegistry(cfg)
	r.declareAttachMode(AttachModeConfigured, linuxHost) // a running lifecycle: the pod layer is what refuses
	called := 0
	driver := func(schema.TargetKind) (Target, error) { called++; return &specTarget{}, nil }
	_, err := r.OpenSession(context.Background(), "assay/dispatch", "assay", "operator", "", driver, Size{})
	if !errors.Is(err, ErrClusterUnauthorized) {
		t.Errorf("pod desk: err = %v, want ErrClusterUnauthorized", err)
	}
	if called != 0 {
		t.Fatalf("the driver resolver ran %d time(s) for a pod desk — the pod driver must never be built", called)
	}
	if s := r.Sessions("assay"); len(s) != 0 {
		t.Fatalf("a refused pod-desk open left a session row: %v", s)
	}
}

// specTarget records the spec it was attached to.
type specTarget struct {
	spec *TargetSpec
	sess *fakeSession
}

func (s *specTarget) Attach(_ context.Context, spec *TargetSpec, _ Size) (Session, error) {
	s.spec = spec
	s.sess = &fakeSession{ch: make(chan []byte, 16), alive: true}
	return s.sess, nil
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for a condition")
		}
		time.Sleep(time.Millisecond)
	}
}

var linuxHost = Host{OS: "linux", Arch: "amd64"}

// TestSessionOpenRefusedWhileAttachOff — obligation (vi) at the session
// store (review pr192-F1(b); #191 security 5302965225): while deskd's attach
// lifecycle is off, has not started, or has stopped, OpenSession refuses
// EVERY open with ErrAttachOff and the driver resolver is never called. The
// mode reaches the store only through the Supervisor (declareAttachMode).
func TestSessionOpenRefusedWhileAttachOff(t *testing.T) {
	cfg := cfgWithLoops(containerLoop(schema.RoleReview, "c"), localLoop(schema.RoleVerify))
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, r *Registry)
	}{
		{"never started (undeclared fails closed)", func(*testing.T, *Registry) {}},
		{"started with --terminal-attach=off", func(t *testing.T, r *Registry) {
			s := NewSupervisor(r, newStepClock(), SupervisorOptions{Mode: AttachModeOff, Host: linuxHost, Drivers: lookupOf(t, nil, nil)})
			s.Start(context.Background())
			t.Cleanup(s.Stop)
		}},
		{"stopped", func(t *testing.T, r *Registry) {
			s := NewSupervisor(r, newStepClock(), SupervisorOptions{Host: linuxHost, Drivers: lookupOf(t, nil, nil)})
			s.Start(context.Background())
			s.Stop()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRegistry(cfg)
			tc.setup(t, r)
			called := 0
			driver := func(schema.TargetKind) (Target, error) { called++; return &specTarget{}, nil }
			for _, on := range []string{"assay/review", "assay/verify", "assay/nope"} {
				if _, err := r.OpenSession(context.Background(), on, "assay", "operator", "", driver, Size{}); !errors.Is(err, ErrAttachOff) {
					t.Errorf("open on %s: err = %v, want ErrAttachOff", on, err)
				}
			}
			if called != 0 || len(r.Sessions("assay")) != 0 {
				t.Fatalf("with attach off the driver resolver ran %d time(s) and %d session(s) exist, want 0 and 0", called, len(r.Sessions("assay")))
			}
		})
	}
	// Control: the same registry under a running lifecycle in mode configured opens.
	r := NewRegistry(cfg)
	s := NewSupervisor(r, newStepClock(), SupervisorOptions{Host: linuxHost, Drivers: lookupOf(t, nil, nil)})
	s.Start(context.Background())
	t.Cleanup(s.Stop)
	if _, err := r.OpenSession(context.Background(), "assay/review", "assay", "operator", "", func(schema.TargetKind) (Target, error) { return &specTarget{}, nil }, Size{}); err != nil {
		t.Fatalf("could-not-check: mode configured on linux refused an open: %v", err)
	}
}

// TestSessionOpenRefusedForUnsupportedKind — obligation (vii) at the session
// store (review pr192-F1(c)): an open for a kind CapabilityFor does not
// report supported on the lifecycle's host is ErrKindUnsupported, decided
// before the driver resolver runs. A Windows host has no pty backend for
// either container or local desks.
func TestSessionOpenRefusedForUnsupportedKind(t *testing.T) {
	r := NewRegistry(cfgWithLoops(containerLoop(schema.RoleReview, "c"), localLoop(schema.RoleVerify)))
	s := NewSupervisor(r, newStepClock(), SupervisorOptions{Host: Host{OS: "windows", Arch: "amd64"}, Drivers: lookupOf(t, nil, nil)})
	s.Start(context.Background())
	t.Cleanup(s.Stop)
	called := 0
	driver := func(schema.TargetKind) (Target, error) { called++; return &specTarget{}, nil }
	for _, on := range []string{"assay/review", "assay/verify"} {
		if _, err := r.OpenSession(context.Background(), on, "assay", "operator", "", driver, Size{}); !errors.Is(err, ErrKindUnsupported) {
			t.Errorf("windows host, open on %s: err = %v, want ErrKindUnsupported", on, err)
		}
	}
	if called != 0 || len(r.Sessions("assay")) != 0 {
		t.Fatalf("on an unsupported host the driver resolver ran %d time(s) and %d session(s) exist, want 0 and 0", called, len(r.Sessions("assay")))
	}
}

// TestPumpTearsDownOnlyItsOwnSession — review pr192-F2 (i)/(ii) at the
// combining merge: pump keeps the service contract's `h.epochN == epochN` guard
// beside the service contract's done channel, and attachHandle hands pump the
// epoch sequence attach() returned. A session that ends AFTER a replacement
// attached must not tear the replacement down: the new session stays
// attached and writable, its epoch does not move, and a send bound to it is
// still delivered to it.
func TestPumpTearsDownOnlyItsOwnSession(t *testing.T) {
	f := newSendFixture(t)
	oldSess := f.tgt.sess
	// The old session is dropped without being closed (the unreachable path),
	// then a replacement attaches.
	f.r.handle(sendDesk).markUnreachable()
	if err := f.r.Attach(context.Background(), sendDesk, f.tgt, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	newSess := f.tgt.sess
	epoch := f.epoch()
	// Now the OLD session's stream ends: its pump runs its teardown.
	oldSess.endStream()
	waitUntil(t, func() bool { oldSess.mu.Lock(); defer oldSess.mu.Unlock(); return oldSess.closed })
	time.Sleep(20 * time.Millisecond)
	h, _ := f.r.Resolve(sendDesk)
	if !h.Writable || !h.Connected || h.Epoch != epoch {
		t.Fatalf("the old session's pump tore down its replacement: writable=%v connected=%v epoch %q -> %q", h.Writable, h.Connected, epoch, h.Epoch)
	}
	f.atPrompt(t)
	if _, err := f.r.WaitThenSend(context.Background(), sendDesk, []byte("ok\n"), contract.SendWaitNone, time.Second, bind("assay", epoch, "p1")); err != nil {
		t.Fatalf("a send bound to the replacement was refused: %v", err)
	}
	if w := newSess.writes; len(w) != 1 {
		t.Fatalf("the replacement received %d write(s), want 1", len(w))
	}
	// And the replacement's OWN end does tear it down: attachHandle handed
	// its pump the epoch sequence attach() returned for it (pr192-F2 (ii)).
	newSess.endStream()
	waitUntil(t, func() bool { h, _ := f.r.Resolve(sendDesk); return !h.Writable })
	if h, _ := f.r.Resolve(sendDesk); h.Connected || h.Epoch == epoch {
		t.Fatalf("the replacement ended but the handle reads connected=%v epoch=%q (was %q)", h.Connected, h.Epoch, epoch)
	}
}
