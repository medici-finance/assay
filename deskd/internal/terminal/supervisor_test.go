package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// Everything here is faked: the clock, the drivers, the sessions. No test in
// this file spawns a process, opens a socket, or reads a kubeconfig.

// ── a stepping clock: every After() is reported to the test, which fires it ──

type waitReq struct {
	d  time.Duration
	ch chan time.Time
}

type stepClock struct {
	mu   sync.Mutex
	t    time.Time
	reqs chan waitReq
}

func newStepClock() *stepClock {
	return &stepClock{t: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), reqs: make(chan waitReq, 256)}
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func (c *stepClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.reqs <- waitReq{d: d, ch: ch}
	return ch
}

// nextWait returns the next wait the supervisor asked for, failing the test if
// none arrives.
func (c *stepClock) nextWait(t *testing.T) waitReq {
	t.Helper()
	select {
	case r := <-c.reqs:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("the supervisor asked for no backoff wait within 5s")
	}
	return waitReq{}
}

// fire advances the clock by the wait and releases it.
func (c *stepClock) fire(r waitReq) {
	c.advance(r.d)
	r.ch <- c.Now()
}

// noWait asserts the supervisor asks for no further wait (no retry scheduled).
func (c *stepClock) noWait(t *testing.T, why string) {
	t.Helper()
	select {
	case r := <-c.reqs:
		t.Fatalf("%s: the supervisor scheduled a retry in %s", why, r.d)
	case <-time.After(200 * time.Millisecond):
	}
}

// ── a recording fake driver ──

type recTarget struct {
	mu          sync.Mutex
	specs       []TargetSpec
	inFlight    int
	maxInFlight int
	// results is consumed one per Attach: nil = success with a new session.
	// When exhausted, every further Attach succeeds.
	results  []error
	sessions []*fakeSession
	attached chan *fakeSession
	// gate, when non-nil, blocks every Attach until it is closed.
	gate chan struct{}
}

func newRecTarget(results ...error) *recTarget {
	return &recTarget{results: results, attached: make(chan *fakeSession, 64)}
}

func (f *recTarget) Attach(ctx context.Context, spec *TargetSpec, _ Size) (Session, error) {
	f.mu.Lock()
	f.specs = append(f.specs, *spec)
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	var res error
	if len(f.results) > 0 {
		res, f.results = f.results[0], f.results[1:]
	}
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	if res != nil {
		return nil, res
	}
	s := &fakeSession{ch: make(chan []byte, 16), alive: true}
	f.mu.Lock()
	f.sessions = append(f.sessions, s)
	f.mu.Unlock()
	f.attached <- s
	return s, nil
}

func (f *recTarget) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.specs)
}

// lookupOf builds a DriverLookup over per-kind fakes. A kind absent from the
// map is unregistered. podFactoryCalls counts every call of a pod factory: the
// constraint under test is that it stays 0.
func lookupOf(t *testing.T, byKind map[schema.TargetKind]Target, factoryCalls map[schema.TargetKind]*int) DriverLookup {
	var mu sync.Mutex
	return func(kind schema.TargetKind) (func() (Target, error), bool) {
		tgt, ok := byKind[kind]
		if !ok {
			return nil, false
		}
		return func() (Target, error) {
			mu.Lock()
			if n := factoryCalls[kind]; n != nil {
				*n++
			}
			mu.Unlock()
			return tgt, nil
		}, true
	}
}

func cfgWithLoops(loops ...schema.Loop) *schema.File {
	return &schema.File{Cells: []schema.Cell{{Name: "assay", Repos: []string{"example-org/console"}, Loops: loops}}}
}

func containerLoop(role schema.LoopRole, name string) schema.Loop {
	return schema.Loop{Role: role, Target: &TargetSpec{Kind: schema.TargetContainer, Container: name}}
}

func localLoop(role schema.LoopRole) schema.Loop {
	return schema.Loop{Role: role, Target: &TargetSpec{Kind: schema.TargetLocal, Command: []string{"/bin/sh"}}}
}

func podLoop(role schema.LoopRole) schema.Loop {
	return schema.Loop{Role: role, Target: &TargetSpec{Kind: schema.TargetPod, Namespace: "example-loops", Pod: "desk-review-0"}}
}

// waitStatus polls a desk's wire attach status until it reads want (status and
// reason), or fails.
func waitStatus(t *testing.T, r *Registry, id, status, reason string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h, ok := r.Resolve(id)
		if !ok {
			t.Fatalf("desk %s is not a row", id)
		}
		if h.Attach.Status == status && h.Attach.Reason == reason {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("desk %s attach = %s/%s, want %s/%s", id, h.Attach.Status, h.Attach.Reason, status, reason)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func identityJitter(d time.Duration) time.Duration { return d }

// TestSupervisorAttachesByRegistryKind is Verify row 3: each targeted desk is
// attached through the factory ITS `kind:` names; a desk with no target is
// failed / no-target; the row count is unchanged. The cluster desk is the
// binding constraint of DR-deskd-attach-start: its factory is never called (no
// kubeconfig is loaded) and it reads failed / unauthorized.
func TestSupervisorAttachesByRegistryKind(t *testing.T) {
	cfg := cfgWithLoops(
		containerLoop(schema.RoleReview, "c-review"),
		localLoop(schema.RoleVerify),
		podLoop(schema.RoleDispatch),
		schema.Loop{Role: schema.RoleMetrics},
	)
	clk := newStepClock()
	r := newRegistryWithClock(cfg, clk.Now)
	cont, loc, pod := newRecTarget(), newRecTarget(), newRecTarget()
	calls := map[schema.TargetKind]*int{schema.TargetPod: new(int), schema.TargetContainer: new(int), schema.TargetLocal: new(int)}
	s := NewSupervisor(r, clk, SupervisorOptions{
		Host:    Host{OS: "linux", Arch: "amd64"},
		Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: cont, schema.TargetLocal: loc, schema.TargetPod: pod}, calls),
		Jitter:  identityJitter,
	})
	s.Start(context.Background())
	defer s.Stop()

	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	waitStatus(t, r, "assay/verify", AttachAttached, ReasonNone)
	waitStatus(t, r, "assay/dispatch", AttachFailed, ReasonUnauthorized)
	waitStatus(t, r, "assay/metrics", AttachFailed, ReasonNoTarget)

	if cont.calls() != 1 || cont.specs[0].Container != "c-review" {
		t.Errorf("container fake saw %d attach(es) %+v, want exactly the container desk once", cont.calls(), cont.specs)
	}
	if loc.calls() != 1 || loc.specs[0].Kind != schema.TargetLocal {
		t.Errorf("local fake saw %d attach(es) %+v, want exactly the local desk once", loc.calls(), loc.specs)
	}
	if pod.calls() != 0 || *calls[schema.TargetPod] != 0 {
		t.Errorf("pod driver: %d factory call(s), %d attach(es) — deskd holds no cluster access grant, so the pod driver must never be built or called",
			*calls[schema.TargetPod], pod.calls())
	}
	if n := len(r.Handles()); n != 4 {
		t.Errorf("rows = %d, want 4 (one per configured desk)", n)
	}
	for _, id := range []string{"assay/dispatch", "assay/metrics"} {
		h, _ := r.Resolve(id)
		if h.State != string(StateCouldNotCheck) {
			t.Errorf("%s state = %s, want could-not-check", id, h.State)
		}
	}
	h, _ := r.Resolve("assay/dispatch")
	if h.Attach.Capability.Support != SupportCouldNotCheck || h.Attach.Capability.Reason != CapReasonNoClusterGrant {
		t.Errorf("pod capability = %+v, want could-not-check / no-cluster-grant", h.Attach.Capability)
	}
	clk.noWait(t, "the cluster desk must never be retried without a grant")
}

// TestSupervisorReattachBackoffIsBounded is Verify row 4.
func TestSupervisorReattachBackoffIsBounded(t *testing.T) {
	cfg := cfgWithLoops(containerLoop(schema.RoleReview, "c-review"))
	clk := newStepClock()
	r := newRegistryWithClock(cfg, clk.Now)
	unreach := fmt.Errorf("fake: container c-review gone: %w", ErrUnreachable)
	// 1 success, 8 refusals, then success.
	tgt := newRecTarget(nil, unreach, unreach, unreach, unreach, unreach, unreach, unreach, unreach)
	s := NewSupervisor(r, clk, SupervisorOptions{
		Host:    Host{OS: "linux", Arch: "amd64"},
		Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil),
		Jitter:  identityJitter,
	})
	s.Start(context.Background())
	defer s.Stop()

	id := "assay/review"
	s1 := <-tgt.attached
	waitStatus(t, r, id, AttachAttached, ReasonNone)
	s1.ch <- []byte("before the drop\n")
	waitCursor(t, r, id, 1)
	before, _ := r.Resolve(id)

	s1.endStream()
	first := clk.nextWait(t)
	h, _ := r.Resolve(id)
	if h.Attach.Status != AttachRetrying || h.Attach.Reason != ReasonExited || h.State != string(StateCouldNotCheck) {
		t.Fatalf("after the session ended: %s/%s state %s, want retrying/exited could-not-check", h.Attach.Status, h.Attach.Reason, h.State)
	}
	if h.Attach.NextAttemptAt == nil || !h.Attach.NextAttemptAt.Equal(clk.Now().Add(first.d)) {
		t.Fatalf("next_attempt_at = %v, want now+%s", h.Attach.NextAttemptAt, first.d)
	}
	want := []time.Duration{1, 2, 4, 8, 16, 32, 60, 60, 60}
	got := []time.Duration{first.d}
	clk.fire(first)
	for i := 1; i < len(want); i++ {
		w := clk.nextWait(t)
		got = append(got, w.d)
		if w.d > BackoffCap {
			t.Fatalf("wait %d = %s, above the %s cap", i, w.d, BackoffCap)
		}
		clk.fire(w)
	}
	for i := range want {
		if got[i] != want[i]*time.Second {
			t.Fatalf("backoff ladder = %v, want %v seconds", got, want)
		}
	}
	<-tgt.attached // the 10th attempt succeeds
	waitStatus(t, r, id, AttachAttached, ReasonNone)
	if tgt.maxInFlight > 1 {
		t.Errorf("max attempts in flight for one desk = %d, want 1", tgt.maxInFlight)
	}
	after, _ := r.Resolve(id)
	if after.Cursor < before.Cursor {
		t.Fatalf("cursor went backwards across a re-attach: %d -> %d (the ring must survive inside one process)", before.Cursor, after.Cursor)
	}
	if after.Attach.Epoch != before.Attach.Epoch {
		t.Fatalf("epoch changed inside one process: %v -> %v", before.Attach.Epoch, after.Attach.Epoch)
	}

	// A session that survived StableAfter resets the ladder to 1 s.
	clk.advance(StableAfter + time.Second)
	tgt.mu.Lock()
	s2 := tgt.sessions[len(tgt.sessions)-1]
	tgt.mu.Unlock()
	s2.endStream()
	if w := clk.nextWait(t); w.d != BackoffBase {
		t.Fatalf("after a %s-stable session the next wait = %s, want the ladder reset to %s", StableAfter+time.Second, w.d, BackoffBase)
	}
}

func waitCursor(t *testing.T, r *Registry, id string, atLeast uint64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		h, _ := r.Resolve(id)
		if h.Cursor >= atLeast {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("cursor of %s never reached %d", id, atLeast)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestSupervisorLocalKindNeverRespawns is Verify row 5 (NEGATIVE PATH for the
// retry loop): a local desk is spawned exactly once; when it exits it reads
// failed / exited and no retry is ever scheduled. The same holds when the one
// spawn is refused.
func TestSupervisorLocalKindNeverRespawns(t *testing.T) {
	t.Run("exits", func(t *testing.T) {
		clk := newStepClock()
		r := newRegistryWithClock(cfgWithLoops(localLoop(schema.RoleVerify)), clk.Now)
		tgt := newRecTarget()
		s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetLocal: tgt}, nil)})
		s.Start(context.Background())
		defer s.Stop()
		sess := <-tgt.attached
		sess.endStream()
		waitStatus(t, r, "assay/verify", AttachFailed, ReasonExited)
		clk.advance(time.Hour)
		clk.noWait(t, "a local desk that exited must never be re-spawned")
		if n := tgt.calls(); n != 1 {
			t.Fatalf("local desk spawned %d times over a simulated hour, want exactly 1", n)
		}
	})
	t.Run("refused", func(t *testing.T) {
		clk := newStepClock()
		r := newRegistryWithClock(cfgWithLoops(localLoop(schema.RoleVerify)), clk.Now)
		tgt := newRecTarget(fmt.Errorf("fake: spawn refused: %w", ErrUnreachable))
		s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetLocal: tgt}, nil)})
		s.Start(context.Background())
		defer s.Stop()
		waitStatus(t, r, "assay/verify", AttachFailed, ReasonUnreachable)
		clk.noWait(t, "a local desk whose spawn was refused must never be re-spawned")
		if n := tgt.calls(); n != 1 {
			t.Fatalf("local desk spawned %d times, want exactly 1", n)
		}
	})
	// A local driver refusal wrapping ErrUnauthorized is still a local desk:
	// spawned once, never on the 5 min unauthorized timer (review advisory
	// A-cor-local-unauthorized-order).
	t.Run("refused-unauthorized", func(t *testing.T) {
		clk := newStepClock()
		r := newRegistryWithClock(cfgWithLoops(localLoop(schema.RoleVerify)), clk.Now)
		tgt := newRecTarget(fmt.Errorf("fake: spawn refused: %w", ErrUnauthorized))
		s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetLocal: tgt}, nil)})
		s.Start(context.Background())
		defer s.Stop()
		waitStatus(t, r, "assay/verify", AttachFailed, ReasonUnauthorized)
		clk.noWait(t, "a local desk refused as unauthorized must never be re-spawned")
		if n := tgt.calls(); n != 1 {
			t.Fatalf("local desk spawned %d times, want exactly 1", n)
		}
	})
}

// TestAttachFailureReasonsAreHonest is Verify row 6.
func TestAttachFailureReasonsAreHonest(t *testing.T) {
	const secretName = "c-name-that-must-not-leak"
	cases := []struct {
		name       string
		loop       schema.Loop
		host       Host
		drivers    map[schema.TargetKind]Target
		factoryErr bool
		status     string
		reason     string
		minNext    time.Duration // 0 = no next attempt expected
	}{
		{name: "unreachable", loop: containerLoop(schema.RoleReview, secretName),
			drivers: map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget(fmt.Errorf("docker: no such container %s: %w", secretName, ErrUnreachable))},
			status:  AttachRetrying, reason: ReasonUnreachable, minNext: BackoffBase},
		{name: "unauthorized", loop: containerLoop(schema.RoleReview, secretName),
			drivers: map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget(fmt.Errorf("permission denied on the socket for %s: %w", secretName, ErrUnauthorized))},
			status:  AttachRetrying, reason: ReasonUnauthorized, minNext: UnauthorizedRetry},
		{name: "no-driver", loop: containerLoop(schema.RoleReview, secretName),
			drivers: map[schema.TargetKind]Target{},
			status:  AttachFailed, reason: ReasonNoDriver},
		{name: "driver-init", loop: containerLoop(schema.RoleReview, secretName),
			drivers: map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget()}, factoryErr: true,
			status: AttachFailed, reason: ReasonDriverInit},
		{name: "unsupported-host", loop: containerLoop(schema.RoleReview, secretName), host: Host{OS: "windows", Arch: "amd64"},
			drivers: map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget()},
			status:  AttachFailed, reason: ReasonUnsupported},
		{name: "cluster-without-grant", loop: podLoop(schema.RoleReview),
			drivers: map[schema.TargetKind]Target{schema.TargetPod: newRecTarget()},
			status:  AttachFailed, reason: ReasonUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := newStepClock()
			r := newRegistryWithClock(cfgWithLoops(tc.loop), clk.Now)
			lookup := lookupOf(t, tc.drivers, nil)
			if tc.factoryErr {
				lookup = func(schema.TargetKind) (func() (Target, error), bool) {
					return func() (Target, error) { return nil, fmt.Errorf("init failed for %s", secretName) }, true
				}
			}
			var logMu sync.Mutex
			var logs []string
			host := tc.host
			if host == (Host{}) {
				host = Host{OS: "linux", Arch: "amd64"}
			}
			s := NewSupervisor(r, clk, SupervisorOptions{Host: host, Drivers: lookup, Jitter: identityJitter,
				Logf: func(f string, a ...any) { logMu.Lock(); logs = append(logs, fmt.Sprintf(f, a...)); logMu.Unlock() }})
			s.Start(context.Background())
			defer s.Stop()
			waitStatus(t, r, "assay/review", tc.status, tc.reason)

			h, ok := r.Resolve("assay/review")
			if !ok {
				t.Fatal("the desk vanished — a failed attach is a row, never a miss")
			}
			if h.State != string(StateCouldNotCheck) {
				t.Errorf("state = %s, want could-not-check (never idle)", h.State)
			}
			if tc.minNext > 0 {
				if h.Attach.NextAttemptAt == nil || h.Attach.NextAttemptAt.Sub(clk.Now()) < tc.minNext {
					t.Errorf("next_attempt_at = %v, want ≥ now+%s", h.Attach.NextAttemptAt, tc.minNext)
				}
			} else if h.Attach.NextAttemptAt != nil {
				t.Errorf("next_attempt_at = %v on a terminal failure, want nil", h.Attach.NextAttemptAt)
			}
			wire, _ := json.Marshal(h)
			if strings.Contains(string(wire), secretName) {
				t.Errorf("the wire handle leaks the driver's target name: %s", wire)
			}
			rows := r.DeskRows(&cfgWithLoops(tc.loop).Cells[0])
			rowWire, _ := json.Marshal(rows)
			if strings.Contains(string(rowWire), secretName) {
				t.Errorf("the desk row leaks the driver's target name: %s", rowWire)
			}
			if len(rows) != 1 || rows[0].Attach.Reason != tc.reason {
				t.Errorf("desk row attach = %+v, want reason %s", rows, tc.reason)
			}
		})
	}
}

// TestSupervisorOffModeDisablesAll: --terminal-attach=off marks every desk
// off / disabled (including one with no target) and looks up no driver.
func TestSupervisorOffModeDisablesAll(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c"), localLoop(schema.RoleVerify), podLoop(schema.RoleDispatch), schema.Loop{Role: schema.RoleMetrics}), clk.Now)
	looked := 0
	s := NewSupervisor(r, clk, SupervisorOptions{Mode: AttachModeOff, Drivers: func(schema.TargetKind) (func() (Target, error), bool) {
		looked++
		return nil, false
	}})
	s.Start(context.Background())
	defer s.Stop()
	for _, h := range r.Handles() {
		if h.Attach.Status != AttachOff || h.Attach.Reason != ReasonDisabled {
			t.Errorf("%s attach = %s/%s, want off/disabled", h.DeskID, h.Attach.Status, h.Attach.Reason)
		}
	}
	if looked != 0 {
		t.Errorf("mode off looked up %d driver(s), want 0", looked)
	}
}

// TestSupervisorGlobalInFlightCap: at most MaxAttachInFlight attempts run at
// once across deskd, however many desks are configured.
func TestSupervisorGlobalInFlightCap(t *testing.T) {
	var loops []schema.Loop
	for _, role := range []schema.LoopRole{schema.RoleReview, schema.RoleVerify, schema.RoleDispatch, schema.RoleMetrics} {
		loops = append(loops, containerLoop(role, "c-"+string(role)))
	}
	cfg := &schema.File{Cells: []schema.Cell{
		{Name: "a", Loops: loops},
		{Name: "b", Loops: loops},
	}}
	clk := newStepClock()
	r := newRegistryWithClock(cfg, clk.Now)
	tgt := newRecTarget()
	tgt.gate = make(chan struct{})
	s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil)})
	s.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	tgt.mu.Lock()
	inFlight := tgt.inFlight
	tgt.mu.Unlock()
	close(tgt.gate)
	for i := 0; i < 8; i++ {
		<-tgt.attached
	}
	s.Stop()
	if inFlight != MaxAttachInFlight || tgt.maxInFlight > MaxAttachInFlight {
		t.Fatalf("in flight = %d (max seen %d) with 8 desks, want exactly %d", inFlight, tgt.maxInFlight, MaxAttachInFlight)
	}
}

// TestSupervisorStopDetaches: Stop closes deskd's own session and returns;
// the desk row stays.
func TestSupervisorStopDetaches(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	tgt := newRecTarget()
	s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil)})
	s.Start(context.Background())
	sess := <-tgt.attached
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return within 5s")
	}
	sess.mu.Lock()
	closed := sess.closed
	sess.mu.Unlock()
	if !closed {
		t.Fatal("Stop left deskd's session open")
	}
	if _, ok := r.Resolve("assay/review"); !ok {
		t.Fatal("the desk row vanished on Stop")
	}
}

// TestSupervisorLogsAFailedDetach (review advisory A-cor-unset-import-abort):
// when deskd's own session will not close cleanly on Stop (for a container
// desk: the detach exec failed, so deskd's shell may still be running in the
// container), the supervisor logs it with the desk and the driver's text
// instead of dropping the error.
func TestSupervisorLogsAFailedDetach(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	tgt := newRecTarget()
	var mu sync.Mutex
	var lines []string
	logf := func(f string, a ...any) {
		mu.Lock()
		lines = append(lines, fmt.Sprintf(f, a...))
		mu.Unlock()
	}
	s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil), Logf: logf})
	s.Start(context.Background())
	sess := <-tgt.attached
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	sess.mu.Lock()
	sess.closeErr = errors.New("container c: the detach exec for deskd's shell (pid 42) failed, so that shell may still be running: exit status 2")
	sess.mu.Unlock()
	s.Stop()
	mu.Lock()
	defer mu.Unlock()
	for _, l := range lines {
		if strings.Contains(l, "desk=assay/review detach failed:") && strings.Contains(l, "may still be running") {
			return
		}
	}
	t.Fatalf("Stop dropped a failed detach: no `desk=assay/review detach failed:` line naming the driver's text in the log:\n%s", strings.Join(lines, "\n"))
}

// TestSupervisorStopClosesCustomSessions — review pr192-F2 (iv): Stop tears
// down every session deskd holds, not only the configured desks' ones. A
// custom session (the service contract's `session` action) is not in the
// registry's configured order, so Stop must close it explicitly: its driver
// session is closed and its row is gone.
func TestSupervisorStopClosesCustomSessions(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	tgt := newRecTarget()
	s := NewSupervisor(r, clk, SupervisorOptions{Host: Host{OS: "linux", Arch: "amd64"}, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil)})
	s.Start(context.Background())
	<-tgt.attached
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	custom := &specTarget{}
	row, err := r.OpenSession(context.Background(), "assay/review", "assay", "operator", "", func(schema.TargetKind) (Target, error) { return custom, nil }, Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("could-not-check: open a custom session under a running lifecycle: %v", err)
	}
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return within 5s")
	}
	custom.sess.mu.Lock()
	closed := custom.sess.closed
	custom.sess.mu.Unlock()
	if !closed {
		t.Fatalf("Stop left the custom session %s's driver session open", row.ID)
	}
	if got := r.Sessions("assay"); len(got) != 0 {
		t.Fatalf("Stop left custom session rows behind: %v", got)
	}
}

// failingCloseTarget opens custom sessions whose Close returns closeErr: a
// container session whose detach exec failed (review F-cor-7).
type failingCloseTarget struct {
	closeErr error
	mu       sync.Mutex
	sess     []*fakeSession
}

func (f *failingCloseTarget) Attach(context.Context, *TargetSpec, Size) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &fakeSession{ch: make(chan []byte, 16), alive: true, closeErr: f.closeErr}
	f.sess = append(f.sess, s)
	return s, nil
}

// detachLog is a Logf that records its lines.
type detachLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *detachLog) logf(f string, a ...any) {
	l.mu.Lock()
	l.lines = append(l.lines, fmt.Sprintf(f, a...))
	l.mu.Unlock()
}

func (l *detachLog) has(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.lines {
		if strings.Contains(s, "desk="+id+" detach failed:") && strings.Contains(s, "may still be running") {
			return true
		}
	}
	return false
}

func (l *detachLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

const failedDetachText = "container c-probe: the detach exec for deskd's shell (pid 4242) failed, so that shell may still be running: exit status 2"

// TestCloseSessionLogsAFailedDetach — review F-cor-7 (the DELETE half): a
// custom session's failed driver close is logged with its id and never
// returned. CloseSession has already forgotten the row when the driver's
// Close runs, so the close has happened (a retry finds no row), and the
// driver's text, which names the container and pid, must not reach the
// caller that writes the wire answer.
func TestCloseSessionLogsAFailedDetach(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	log := &detachLog{}
	s := NewSupervisor(r, clk, SupervisorOptions{Host: linuxHost, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget()}, nil), Logf: log.logf})
	s.Start(context.Background())
	t.Cleanup(s.Stop)
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	custom := &failingCloseTarget{closeErr: errors.New(failedDetachText)}
	row, err := r.OpenSession(context.Background(), "assay/review", "assay", "operator", "", func(schema.TargetKind) (Target, error) { return custom, nil }, Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("could-not-check: open a custom session under a running lifecycle: %v", err)
	}
	if err := r.CloseSession(row.ID, "assay"); err != nil {
		t.Fatalf("CloseSession returned the driver's close error to its caller (the row is already gone, and the text names the container): %v", err)
	}
	if !custom.sess[0].closed {
		t.Fatal("CloseSession did not close the driver session")
	}
	if !log.has(row.ID) {
		t.Fatalf("CloseSession dropped a failed detach: no `desk=%s detach failed:` line naming the driver's text in the log:\n%s", row.ID, log)
	}
	if err := r.CloseSession(row.ID, "assay"); !errors.Is(err, ErrNoSuchSession) {
		t.Fatalf("second close: %v, want ErrNoSuchSession", err)
	}
}

// TestSupervisorStopLogsAFailedCustomSessionDetach — review F-cor-7 (the
// shutdown half): Stop closes custom sessions too (pr192-F2 (iv)), and one
// whose detach fails is logged like a configured desk's
// (TestSupervisorLogsAFailedDetach), never dropped.
func TestSupervisorStopLogsAFailedCustomSessionDetach(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	log := &detachLog{}
	s := NewSupervisor(r, clk, SupervisorOptions{Host: linuxHost, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: newRecTarget()}, nil), Logf: log.logf})
	s.Start(context.Background())
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)
	custom := &failingCloseTarget{closeErr: errors.New(failedDetachText)}
	row, err := r.OpenSession(context.Background(), "assay/review", "assay", "operator", "", func(schema.TargetKind) (Target, error) { return custom, nil }, Size{Cols: 80, Rows: 24})
	if err != nil {
		t.Fatalf("could-not-check: open a custom session under a running lifecycle: %v", err)
	}
	s.Stop()
	if !custom.sess[0].closed {
		t.Fatalf("Stop left the custom session %s's driver session open", row.ID)
	}
	if !log.has(row.ID) {
		t.Fatalf("Stop dropped a custom session's failed detach: no `desk=%s detach failed:` line naming the driver's text in the log:\n%s", row.ID, log)
	}
}

// TestSupervisorStopRefusesAnInFlightOpen — security review
// A-sec-stop-open-race: an open that passed OpenSession's mode check before
// Stop ran must not insert and attach afterwards. The driver resolver is held
// across the whole of Stop; once released, the open is refused with
// ErrAttachOff, no row is left and the driver's Attach is never called.
func TestSupervisorStopRefusesAnInFlightOpen(t *testing.T) {
	clk := newStepClock()
	r := newRegistryWithClock(cfgWithLoops(containerLoop(schema.RoleReview, "c")), clk.Now)
	tgt := newRecTarget()
	s := NewSupervisor(r, clk, SupervisorOptions{Host: linuxHost, Drivers: lookupOf(t, map[schema.TargetKind]Target{schema.TargetContainer: tgt}, nil)})
	s.Start(context.Background())
	<-tgt.attached
	waitStatus(t, r, "assay/review", AttachAttached, ReasonNone)

	custom := &specTarget{}
	entered, release := make(chan struct{}), make(chan struct{})
	resolver := func(schema.TargetKind) (Target, error) {
		close(entered)
		<-release
		return custom, nil
	}
	opened := make(chan error, 1)
	go func() {
		_, err := r.OpenSession(context.Background(), "assay/review", "assay", "operator", "", resolver, Size{Cols: 80, Rows: 24})
		opened <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("could-not-check: the open never reached the driver resolver (it did not pass the mode check)")
	}
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return within 5s")
	}
	close(release)
	var err error
	select {
	case err = <-opened:
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight open did not return within 5s of release")
	}
	if !errors.Is(err, ErrAttachOff) {
		t.Errorf("an open in flight across Stop: err = %v, want ErrAttachOff", err)
	}
	if got := r.Sessions("assay"); len(got) != 0 {
		t.Errorf("an open in flight across Stop left custom session rows after Stop: %v", got)
	}
	if custom.sess != nil {
		t.Errorf("an open in flight across Stop attached its driver session after Stop (alive=%v)", custom.sess.alive)
	}
}

// TestRegistryRefusesSecondAttach: one session per desk — a second attach
// while one is live is refused, not queued.
func TestRegistryRefusesSecondAttach(t *testing.T) {
	r := NewRegistry(cfgWithLoops(containerLoop(schema.RoleReview, "c")))
	if err := r.Attach(context.Background(), "assay/review", &fakeTarget{}, Size{Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	err := r.Attach(context.Background(), "assay/review", &fakeTarget{}, Size{Cols: 80, Rows: 24})
	if !errors.Is(err, ErrAlreadyAttached) {
		t.Fatalf("second attach err = %v, want ErrAlreadyAttached", err)
	}
}

// writeNames are the identifiers that would make lifecycle code an input
// path: the registry's and session's write/resize methods and the send
// primitive the service contract adds.
var writeNames = map[string]bool{"Write": true, "Resize": true, "WaitThenSend": true, "write": true, "resize": true}

// TestSupervisorHoldsNoWriteReference is Verify row 8's AST audit: neither the
// supervisor nor the file that links deskd's drivers names a write, resize or
// send method — not as a call, not as an interface method, not as a field.
func TestSupervisorHoldsNoWriteReference(t *testing.T) {
	for _, path := range []string{"supervisor.go", "capability.go", "../../cmd/deskd/drivers.go"} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && writeNames[id.Name] {
				t.Errorf("%s: %s names %q — attach lifecycle code must hold no write/resize/send reference (it would become an input path into a desk)",
					path, fset.Position(id.Pos()), id.Name)
			}
			return true
		})
	}
}

// TestTerminalPlatformProfiles is Verify row 101 (report F-2): what each host
// can do for each kind, decided before any attempt. Container and local need
// a host pty, which has no Windows backend; the cluster kind is could-not-check
// everywhere because deskd holds no cluster grant.
func TestTerminalPlatformProfiles(t *testing.T) {
	container := &TargetSpec{Kind: schema.TargetContainer, Container: "c"}
	local := &TargetSpec{Kind: schema.TargetLocal}
	pod := &TargetSpec{Kind: schema.TargetPod, Namespace: "n", Pod: "p"}
	cases := []struct {
		spec                     *TargetSpec
		host                     Host
		backend, profile, sup, r string
	}{
		{container, Host{"linux", "amd64"}, "docker-exec", ProfileTrustedHost, SupportSupported, ""},
		{container, Host{"darwin", "arm64"}, "docker-exec", ProfileTrustedHost, SupportSupported, ""},
		{container, Host{"windows", "amd64"}, "docker-exec", ProfileTrustedHost, SupportUnsupported, CapReasonNoPTYBackend},
		{local, Host{"linux", "arm64"}, "host-pty", ProfileTrustedHost, SupportSupported, ""},
		{local, Host{"windows", "arm64"}, "host-pty", ProfileTrustedHost, SupportUnsupported, CapReasonNoPTYBackend},
		{pod, Host{"linux", "amd64"}, "kube-exec", ProfileClusterRBAC, SupportCouldNotCheck, CapReasonNoClusterGrant},
		{pod, Host{"windows", "amd64"}, "kube-exec", ProfileClusterRBAC, SupportCouldNotCheck, CapReasonNoClusterGrant},
		{nil, Host{"linux", "amd64"}, "", "", SupportCouldNotCheck, CapReasonNoTarget},
		{&TargetSpec{Kind: "vm"}, Host{"linux", "amd64"}, "", "", SupportCouldNotCheck, CapReasonUnknownKind},
	}
	for _, tc := range cases {
		c := CapabilityFor(tc.spec, tc.host)
		if c.HostOS != tc.host.OS || c.HostArch != tc.host.Arch || c.Backend != tc.backend || c.Profile != tc.profile || c.Support != tc.sup || c.Reason != tc.r {
			t.Errorf("CapabilityFor(%+v, %+v) = %+v, want backend=%s profile=%s support=%s reason=%s",
				tc.spec, tc.host, c, tc.backend, tc.profile, tc.sup, tc.r)
		}
	}
}
