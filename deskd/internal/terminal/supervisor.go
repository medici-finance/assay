// supervisor.go — deskd's attach lifecycle (the service contract, ruled option 1
// on the decision issue: attach at start, all three kinds, one start-up off
// switch, bounded-backoff re-attach, read-only, in-memory, no new credential;
// cluster desks read "unauthorized" until access is separately granted —
// DR-deskd-attach-start).
//
// The Supervisor is the ONLY production caller of the attach path. Its only
// input is the registry's configured handles (built from cells.yaml). It
// exposes Start and Stop and nothing a request handler could reach, so no
// request can cause, steer or repeat an exec (SPOF layer a). It holds the
// registry through [attachSurface], which has no write, resize or send method,
// and TestSupervisorHoldsNoWriteReference audits this file's source for any
// such selector (layer b).
//
// Per kind:
//
//   - container: attach at start. When the session ends, or an attempt is
//     refused, re-attach on the backoff ladder: 1 s doubling to a 60 s cap,
//     ±20 % jitter, no attempt limit. An attached session that survived 30 s
//     resets the ladder. ErrUnauthorized retries at a fixed 5 min instead.
//   - local: spawn ONCE. Never re-spawned in a loop (the service contract). When
//     it exits it reads failed / exited until deskd restarts.
//   - pod: NEVER attached (DR-deskd-attach-start). The supervisor reports the
//     desk failed / unauthorized before any driver lookup and never retries
//     it. This is one of two independent layers: deskd does not link the pod
//     driver at all (cmd/deskd/drivers.go), so DriverFor(pod) misses there
//     for every consumer. Granting deskd cluster access is a separate human
//     sign-off.
//
// At most one attempt is in flight per desk (one goroutine per desk) and at
// most [MaxAttachInFlight] across deskd. Each attempt has a 10 s timeout.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// The lifecycle numbers (brief facts). They are package constants, not flags:
// changing them is a reviewed code change.
const (
	BackoffBase       = 1 * time.Second
	BackoffCap        = 60 * time.Second
	BackoffJitter     = 0.20
	AttemptTimeout    = 10 * time.Second
	UnauthorizedRetry = 5 * time.Minute
	StableAfter       = 30 * time.Second
	MaxAttachInFlight = 4
)

// DefaultAttachSize is the window size deskd opens a session at. No viewer is
// attached when deskd opens it.
var DefaultAttachSize = Size{Cols: 80, Rows: 24}

// AttachMode is the --terminal-attach start-up switch.
type AttachMode string

const (
	// AttachModeConfigured attaches every desk cells.yaml names (the default).
	AttachModeConfigured AttachMode = "configured"
	// AttachModeOff attaches nothing: every desk reads off / disabled, which
	// is deskd's behaviour before the service contract.
	AttachModeOff AttachMode = "off"
)

// ParseAttachMode validates a --terminal-attach value.
func ParseAttachMode(s string) (AttachMode, error) {
	switch AttachMode(s) {
	case AttachModeConfigured, AttachModeOff:
		return AttachMode(s), nil
	}
	return "", fmt.Errorf("--terminal-attach=%q: want %q or %q", s, AttachModeConfigured, AttachModeOff)
}

// Clock is the supervisor's time source. Tests inject a fake one so the
// backoff ladder is observable without sleeping.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// RealClock is the wall clock.
type RealClock struct{}

func (RealClock) Now() time.Time                         { return time.Now().UTC() }
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// DriverLookup resolves a kind to its registered driver factory. The default
// is [DriverFor]. Tests pass recording fakes.
type DriverLookup func(kind schema.TargetKind) (factory func() (Target, error), ok bool)

// SupervisorOptions configures a [Supervisor]. The zero value is mode
// "configured" on the running host with the registered drivers.
type SupervisorOptions struct {
	Mode    AttachMode
	Host    Host
	Size    Size
	Drivers DriverLookup
	// Jitter maps a backoff delay to the delay actually waited. nil = ±20 %
	// uniform. Tests pin it.
	Jitter func(time.Duration) time.Duration
	// Logf receives one line per attach transition, and one per failed
	// detach, a custom session's failed close included. nil = discard. The
	// driver's error text appears here and never on the wire.
	Logf func(format string, args ...any)
}

// attachSurface is everything the supervisor may do to the registry. It has
// no Write, Resize or send method: lifecycle code cannot become an input
// path.
type attachSurface interface {
	ids() []string
	handle(id string) *Handle
	attachHandle(ctx context.Context, h *Handle, t Target, size Size) (<-chan struct{}, error)
	Detach(id string) error
	// declareAttachMode tells the session store which mode and host the
	// lifecycle runs under (obligations (vi) and (vii)).
	declareAttachMode(mode AttachMode, host Host)
	// closeCustomSessions closes the custom sessions on shutdown (pr192-F2 (iv)).
	closeCustomSessions()
}

// Supervisor owns deskd's attach lifecycle. Build it with [NewSupervisor],
// call Start once after the HTTP surface is listening, and Stop on shutdown.
type Supervisor struct {
	reg   attachSurface
	clock Clock
	opts  SupervisorOptions
	sem   chan struct{}

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewSupervisor builds a supervisor over reg. clock nil = [RealClock].
func NewSupervisor(reg *Registry, clock Clock, opts SupervisorOptions) *Supervisor {
	if clock == nil {
		clock = RealClock{}
	}
	if opts.Mode == "" {
		opts.Mode = AttachModeConfigured
	}
	if opts.Size == (Size{}) {
		opts.Size = DefaultAttachSize
	}
	if opts.Drivers == nil {
		opts.Drivers = DriverFor
	}
	if opts.Jitter == nil {
		opts.Jitter = defaultJitter
	}
	if reg != nil {
		// A custom session's failed close is logged where a configured
		// desk's is (review F-cor-7).
		reg.declareDetachLog(opts.Logf)
	}
	return &Supervisor{reg: reg, clock: clock, opts: opts, sem: make(chan struct{}, MaxAttachInFlight)}
}

func defaultJitter(d time.Duration) time.Duration {
	f := 1 + BackoffJitter*(2*rand.Float64()-1)
	return time.Duration(float64(d) * f)
}

// Start begins the lifecycle: one goroutine per configured desk with a
// target. In mode off it marks every desk off / disabled and starts nothing.
// A second Start is a no-op.
func (s *Supervisor) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()
	// The session store reads the same switch: in mode off no session opens
	// either (obligation (vi)).
	s.reg.declareAttachMode(s.opts.Mode, s.opts.Host)

	for _, id := range s.reg.ids() {
		h := s.reg.handle(id)
		if h == nil {
			continue
		}
		h.setCapability(CapabilityFor(h.spec, s.opts.Host))
		h.mu.Lock()
		h.supervised = true
		h.mu.Unlock()
		switch {
		case s.opts.Mode == AttachModeOff:
			s.transition(h, AttachOff, ReasonDisabled, 0, nil, nil)
		case h.spec == nil:
			s.transition(h, AttachFailed, ReasonNoTarget, 0, nil, nil)
		case h.spec.Kind == schema.TargetPod:
			// No cluster access grant (DR-deskd-attach-start): the pod driver
			// is never built and no API server is contacted. Not retried.
			s.transition(h, AttachFailed, ReasonUnauthorized, 0, nil, nil)
		default:
			// Decided before any attempt (report F-2): a host that cannot
			// attach this kind never retries, spawns or falls back.
			switch CapabilityFor(h.spec, s.opts.Host).Support {
			case SupportSupported:
				s.wg.Add(1)
				go s.run(ctx, h)
			case SupportUnsupported:
				s.transition(h, AttachFailed, ReasonUnsupported, 0, nil, nil)
			default:
				s.transition(h, AttachFailed, ReasonNoDriver, 0, nil, nil)
			}
		}
	}
}

// Stop ends the lifecycle: it cancels every desk's goroutine, refuses any
// further session open, detaches every session deskd holds — the configured
// desks' and the custom sessions' (the desks themselves keep running) — and
// waits for the goroutines to exit. The detaches run in parallel: ending a container desk's
// shell is one bounded `docker exec` (container/session.go), and one desk's
// detach must not queue behind another's inside the shutdown window.
func (s *Supervisor) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	// No session opens once shutdown has begun.
	s.reg.declareAttachMode(AttachModeOff, s.opts.Host)
	var detaches sync.WaitGroup
	for _, id := range s.reg.ids() {
		detaches.Add(1)
		go func(id string) {
			defer detaches.Done()
			s.detach(id)
		}(id)
	}
	// Custom sessions are not in the configured order ids() walks (review
	// pr192-F2 (iv)); they close alongside, inside the same window.
	detaches.Add(1)
	go func() {
		defer detaches.Done()
		s.reg.closeCustomSessions()
	}()
	detaches.Wait()
	s.wg.Wait()
}

// detach ends deskd's own session on desk id. A failed detach is logged, not
// swallowed: for a container desk it means the signal to deskd's shell inside
// the container was not confirmed sent, so that shell may still be running
// there (review advisory A-cor-unset-import-abort). The driver's text goes to
// the log only, like every other driver error.
func (s *Supervisor) detach(id string) {
	if err := s.reg.Detach(id); err != nil && s.opts.Logf != nil {
		s.opts.Logf("deskd: terminal attach: desk=%s detach failed: %v", id, err)
	}
}

// run is one desk's lifecycle, for a desk whose kind this host supports.
// Exactly one run exists per desk, so at most one attempt is ever in flight
// for it.
func (s *Supervisor) run(ctx context.Context, h *Handle) {
	defer s.wg.Done()
	kind := h.spec.Kind
	if kind == schema.TargetPod {
		// Unreachable by construction (Start never runs a pod desk). Kept as
		// a second, independent refusal: a cluster desk never gets here.
		s.transition(h, AttachFailed, ReasonUnauthorized, 0, nil, nil)
		return
	}
	factory, ok := s.opts.Drivers(kind)
	if !ok {
		s.transition(h, AttachFailed, ReasonNoDriver, 0, nil, nil)
		return
	}
	target, err := factory()
	if err != nil {
		s.transition(h, AttachFailed, ReasonDriverInit, 0, nil, err)
		return
	}

	delay := BackoffBase
	attempts := 0
	for {
		attempts++
		done, err := s.attempt(ctx, h, target, attempts)
		if ctx.Err() != nil {
			if done != nil {
				s.detach(h.deskID)
			}
			return
		}

		var wait time.Duration
		var reason string
		switch {
		case err == nil:
			s.transition(h, AttachAttached, ReasonNone, attempts, nil, nil)
			attachedAt := s.clock.Now()
			select {
			case <-done:
			case <-ctx.Done():
				s.detach(h.deskID)
				return
			}
			if kind == schema.TargetLocal {
				// Spawned once; never re-spawned in a loop.
				s.transition(h, AttachFailed, ReasonExited, attempts, nil, nil)
				return
			}
			if s.clock.Now().Sub(attachedAt) >= StableAfter {
				delay = BackoffBase
			}
			wait, delay = s.backoff(delay)
			reason = ReasonExited
		case kind == schema.TargetLocal:
			// A refused spawn is never retried either, whatever
			// the refusal: this case comes before the unauthorized timer.
			reason := ReasonUnreachable
			if errors.Is(err, ErrUnauthorized) {
				reason = ReasonUnauthorized
			}
			s.transition(h, AttachFailed, reason, attempts, nil, err)
			return
		case errors.Is(err, ErrUnauthorized):
			// A socket-permission refusal does not heal in a second, and a
			// tight loop is noise in the platform's audit log.
			wait, reason = UnauthorizedRetry, ReasonUnauthorized
		default:
			wait, delay = s.backoff(delay)
			reason = ReasonUnreachable
		}

		next := s.clock.Now().Add(wait)
		s.transition(h, AttachRetrying, reason, attempts, &next, err)
		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(wait):
		}
	}
}

// backoff returns the jittered wait for delay (never above the cap) and the
// next rung of the ladder.
func (s *Supervisor) backoff(delay time.Duration) (wait, nextDelay time.Duration) {
	wait = s.opts.Jitter(delay)
	if wait > BackoffCap {
		wait = BackoffCap
	}
	nextDelay = delay * 2
	if nextDelay > BackoffCap {
		nextDelay = BackoffCap
	}
	return wait, nextDelay
}

// attempt makes one bounded attach attempt. It waits for a deskd-wide slot,
// gives the driver [AttemptTimeout] to return, and on success returns the
// channel that closes when the session ends. The session itself outlives the
// attempt timeout.
func (s *Supervisor) attempt(ctx context.Context, h *Handle, target Target, n int) (<-chan struct{}, error) {
	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	h.mu.Lock()
	h.attempts = n
	h.mu.Unlock()

	actx, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(AttemptTimeout, cancel)
	done, err := s.reg.attachHandle(actx, h, target, s.opts.Size)
	timer.Stop()
	if err != nil {
		cancel()
		return nil, err
	}
	go func() {
		<-done
		cancel()
	}()
	return done, nil
}

// transition records the new status on the handle and, when it changed, logs
// one line. The driver's error text (err) goes to the log only.
func (s *Supervisor) transition(h *Handle, status, reason string, attempts int, next *time.Time, err error) {
	now := s.clock.Now()
	from, fromReason := h.setAttach(status, reason, attempts, next, now)
	if from == status && fromReason == reason {
		return
	}
	if s.opts.Logf == nil {
		return
	}
	kind := ""
	if h.spec != nil {
		kind = string(h.spec.Kind)
	}
	nextStr := "-"
	if next != nil {
		nextStr = next.Sub(now).Round(time.Millisecond).String()
	}
	s.opts.Logf("deskd: terminal attach: desk=%s kind=%s %s -> %s reason=%s attempt=%d next=%s",
		h.deskID, kind, from, status, reason, attempts, nextStr)
	if err != nil {
		s.opts.Logf("deskd: terminal attach: desk=%s detail: %v", h.deskID, err)
	}
}
