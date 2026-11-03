package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

// DeskID returns the registry key for a cell's loop: "<cell>/<role>". It is the
// id the terminal read route carries in its path — a client names a DESK, never
// a target.
func DeskID(cell, role string) string { return cell + "/" + role }

// Handle is the registry's per-desk terminal state: the desk's [TargetSpec]
// (nil when it has no attachable target), its scrollback ring, the live
// session (nil until a driver attaches), and the two signals the agent state is
// derived from. It owns the ring and the state; a driver owns only bytes.
type Handle struct {
	mu     sync.Mutex
	deskID string
	spec   *TargetSpec // nil = no attachable target -> could-not-check row
	ring   *ring
	now    func() time.Time

	session       Session
	connected     bool
	title         string
	agentIdentity string
	titleSig      TitleSignal
	promptSig     PromptSignal
	lastAsks      bool
	lastOutput    time.Time
	idleWindow    time.Duration
	asOf          *time.Time

	// pendingResize is a resize requested before a driver attached — queued,
	// never dropped (the container driver facts: "a resize before attach is queued, not
	// dropped"), and applied the moment [Handle.attach] runs.
	pendingResize *Size

	// followers are the read-only live followers the terminal stream's SSE stream
	// registers (follow.go). They are poked, never fed: see [Follower].
	followers map[*Follower]struct{}

	// The attach lifecycle (the service contract). The supervisor, the attach
	// path and pump are the only writers; view() is the only reader.
	attachStatus string
	attachReason string
	attempts     int
	nextAttempt  *time.Time
	attachSince  time.Time
	epoch        time.Time
	capability   contract.AttachCapability
	// supervised is set when a [Supervisor] owns this handle's lifecycle.
	// The supervisor then writes every status transition (and logs it);
	// otherwise the attach path and pump record attached / exited themselves.
	supervised bool

	// boot is the owning registry's per-process nonce and epochN the count of
	// sessions this handle has seen; together they are the handle's EPOCH
	// (send.go). The epoch moves on every attach, detach and close, and the
	// nonce means a restarted deskd never re-issues an epoch a client holds.
	boot   string
	epochN uint64
	// send is the send path's per-handle state (send.go): the send lock,
	// the pending-wait count and the bounded, session-local receipts.
	send sendState
	// custom is set only on a custom session's handle (sessions.go); nil on
	// every configured desk, which is what makes "close only a custom row"
	// a property of the handle rather than of the id's spelling.
	custom *customSession
}

func newHandle(deskID string, spec *TargetSpec, now func() time.Time) *Handle {
	return newHandleAt(deskID, spec, now, now())
}

func newHandleAt(deskID string, spec *TargetSpec, now func() time.Time, epoch time.Time) *Handle {
	return &Handle{
		deskID:     deskID,
		spec:       spec,
		ring:       newRing(DefaultRingBytes, DefaultRingLines),
		now:        now,
		idleWindow: DefaultIdleWindow,
		boot:       processBoot,
		send:       newSendState(),
		// Until a supervisor claims the handle, nothing attaches it: that is
		// "off / disabled", which is exactly what --terminal-attach=off and
		// --once mean.
		attachStatus: AttachOff,
		attachReason: ReasonDisabled,
		attachSince:  epoch,
		epoch:        epoch,
		capability:   CapabilityFor(spec, Host{}),
	}
}

// Attach status values (closed enum, see [contract.AttachStatus]).
const (
	AttachAttached = "attached"
	AttachRetrying = "retrying"
	AttachFailed   = "failed"
	AttachOff      = "off"
)

// Attach reason values (closed enum). A driver's error text never becomes one
// of these. It goes to deskd's log only.
const (
	ReasonNone         = ""
	ReasonNoTarget     = "no-target"
	ReasonNoDriver     = "no-driver"
	ReasonDriverInit   = "driver-init"
	ReasonUnreachable  = "unreachable"
	ReasonUnauthorized = "unauthorized"
	ReasonUnsupported  = "unsupported"
	ReasonExited       = "exited"
	ReasonDisabled     = "disabled"
)

// setAttach records an attach-lifecycle transition and returns the previous
// status and reason (for the transition log line). Since moves only when the
// status or reason actually changes.
func (h *Handle) setAttach(status, reason string, attempts int, next *time.Time, at time.Time) (fromStatus, fromReason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fromStatus, fromReason = h.attachStatus, h.attachReason
	if status != fromStatus || reason != fromReason {
		h.attachSince = at
	}
	h.attachStatus, h.attachReason = status, reason
	h.attempts = attempts
	h.nextAttempt = next
	return fromStatus, fromReason
}

func (h *Handle) setCapability(c contract.AttachCapability) {
	h.mu.Lock()
	h.capability = c
	h.mu.Unlock()
}

// attachView is the wire [contract.AttachStatus]. Caller holds h.mu.
func (h *Handle) attachViewLocked() contract.AttachStatus {
	var next *time.Time
	if h.nextAttempt != nil {
		n := *h.nextAttempt
		next = &n
	}
	return contract.AttachStatus{
		Status:        h.attachStatus,
		Reason:        h.attachReason,
		Attempts:      h.attempts,
		NextAttemptAt: next,
		Since:         h.attachSince,
		Epoch:         h.epoch,
		Capability:    h.capability,
	}
}

// Ingest stamps a chunk of driver output into the scrollback, refreshes the
// signals, and marks the handle connected. It parses the OSC title (identity +
// status) and records that output just arrived (so the prompt signal reads as
// flowing until the idle window elapses). The registry's pump calls this for
// every chunk; a test calls it directly with a fakeTarget's bytes.
func (h *Handle) Ingest(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	h.ring.Write(b)
	h.connected = true
	h.lastOutput = now
	if title, ok := parseOSCTitle(b); ok {
		h.title = title
		h.titleSig = classifyTitle(title)
		h.agentIdentity = agentIdentityFromTitle(title)
	}
	h.refreshLocked(now)
	h.notifyFollowersLocked()
}

// Refresh recomputes the prompt/idle signal against the current time without
// new output — the tick that lets a settled prompt emerge once the idle window
// has elapsed since the last byte. The registry calls it on read; a test calls
// it with an advanced clock.
func (h *Handle) Refresh() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refreshLocked(h.now())
}

func (h *Handle) refreshLocked(now time.Time) {
	tail := h.ring.tail(4096)
	silence := now.Sub(h.lastOutput)
	h.promptSig = detectPrompt(tail, silence, h.idleWindow)
	h.lastAsks = lastLineAsks(tail)
	asOf := now
	h.asOf = &asOf
}

// attach records a live session on the handle (set by the registry once a
// driver's Attach succeeds). A resize queued before this session existed is
// replayed now, once, against the fresh session. It returns the session's
// epoch sequence number, which the pump uses to tear down only its own session.
func (h *Handle) attach(s Session) uint64 {
	h.mu.Lock()
	h.session = s
	h.connected = true
	h.bumpEpochLocked()
	n := h.epochN
	pending := h.pendingResize
	h.pendingResize = nil
	h.mu.Unlock()
	if pending != nil {
		_ = s.Resize(pending.Cols, pending.Rows)
	}
	return n
}

// write sends p to the handle's live session — the client-to-target byte path
// [Session.Write]'s doc names ("the send route gates the write route, but the
// transport is here"). Writing to a handle with no live session is an error,
// never a panic; there is nothing yet to route the write to.
func (h *Handle) write(p []byte) error {
	h.mu.Lock()
	sess := h.session
	h.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("terminal: desk %q has no live session to write to", h.deskID)
	}
	return sess.Write(p)
}

// resize forwards to the live session, or queues for the next attach when
// there is none yet (facts: "a resize before attach is queued, not dropped").
func (h *Handle) resize(cols, rows int) error {
	h.mu.Lock()
	sess := h.session
	if sess == nil {
		h.pendingResize = &Size{Cols: cols, Rows: rows}
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()
	return sess.Resize(cols, rows)
}

// alive reports whether the handle's live session is still attached. False
// when there is no session at all (never attached, or the driver reported
// [ErrUnreachable]).
func (h *Handle) alive() bool {
	h.mu.Lock()
	sess := h.session
	h.mu.Unlock()
	if sess == nil {
		return false
	}
	return sess.Alive()
}

// since returns the handle's scrollback bytes written after cursor c — the
// resume primitive the terminal stream's SSE stream is built on ([ring.Since]'s
// [ErrCursorEvicted] on a stale cursor).
func (h *Handle) since(c uint64) ([]byte, error) {
	return h.ring.Since(c)
}

// markUnreachable records that the desk's target could not be attached — the
// [ErrUnreachable] path. The handle stays present (spec §5.2: a row, never a
// 404) and its state reads could-not-check.
func (h *Handle) markUnreachable() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.session = nil
	h.connected = false
	h.bumpEpochLocked()
	now := h.now()
	h.asOf = &now
}

// state derives the agent state. An unconnected handle (never attached, or
// unreachable) is could-not-check without consulting the signals; a connected
// one runs the two-signal machine.
func (h *Handle) state() AgentState {
	if !h.connected {
		return StateCouldNotCheck
	}
	return deriveState(h.titleSig, h.promptSig, h.lastAsks)
}

// AgentState is the handle's current derived state (exported for tests and the
// registry).
func (h *Handle) AgentState() AgentState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state()
}

// view projects the handle onto the wire type. Writable reports that a live
// session is attached — the precondition for a `send` (the service contract) —
// and grants nothing by itself: the send path re-checks the caller's identity,
// tier, cell and epoch on every request.
func (h *Handle) view() contract.TerminalHandle {
	h.mu.Lock()
	defer h.mu.Unlock()
	kind := ""
	if h.spec != nil {
		kind = string(h.spec.Kind)
	}
	return contract.TerminalHandle{
		DeskID:        h.deskID,
		TargetKind:    kind,
		AgentIdentity: h.agentIdentity,
		Title:         h.title,
		Connected:     h.connected,
		Writable:      h.session != nil,
		Epoch:         h.epochLocked(),
		State:         string(h.state()),
		Cursor:        h.ring.Cursor(),
		AsOf:          h.asOf,
		Attach:        h.attachViewLocked(),
	}
}

// agentIdentityFromTitle extracts the identity portion of a desk-skill OSC
// title. The convention is "<identity> · <status>" or "<identity>: <status>";
// with no separator the whole title is treated as the identity.
func agentIdentityFromTitle(title string) string {
	for _, sep := range []string{" · ", ": ", " - ", " — "} {
		if i := strings.Index(title, sep); i >= 0 {
			return strings.TrimSpace(title[:i])
		}
	}
	return strings.TrimSpace(title)
}

// Registry maps a desk id to its terminal [Handle]. It is built from the parsed
// cells.yaml and is the SINGLE authority on which target a desk id resolves to
// (spec §5.1): [Registry.Resolve] reads ONLY a configured handle's spec, so no
// request can name a target. One row per configured desk exists even when the
// desk has no target or is unreachable (spec §5.2).
type Registry struct {
	mu      sync.Mutex
	now     func() time.Time
	order   []string
	handles map[string]*Handle
	// epoch is when this registry (in practice, this deskd process) was built.
	// Every handle's attach.epoch carries it. It changes only on a restart,
	// which is the one event that resets every desk's scrollback cursor.
	epoch time.Time
	// boot is this registry's per-process nonce, stamped on every handle's
	// epoch (send.go) so a new process never re-issues an epoch.
	boot string
	// sessions lists the custom session ids in open order (sessions.go).
	// Their handles live in handles alongside the configured desks, flagged
	// by Handle.custom; they are never in order, which stays the configured
	// row set.
	sessions []string
	// attachMode and attachHost are what deskd's attach lifecycle runs under,
	// declared by [Supervisor.Start] (and set back to off by Stop). The
	// session store reads them: a session open is allowed only in mode
	// configured, for a kind [CapabilityFor] reports supported on that host
	// (obligations (vi) and (vii) of the combining merge). The zero value is
	// undeclared, which refuses every open exactly like off.
	attachMode AttachMode
	attachHost Host
	// detachLogf is the Supervisor's Logf, which [NewSupervisor] declares
	// here: a custom session's failed driver close is logged through it
	// (review F-cor-7). nil = discard.
	detachLogf func(format string, args ...any)
}

// declareAttachMode records the attach mode and host deskd's lifecycle runs
// under. Only the [Supervisor] calls it: the --terminal-attach switch reaches
// the session store through the one object that owns it.
func (r *Registry) declareAttachMode(mode AttachMode, host Host) {
	r.mu.Lock()
	r.attachMode, r.attachHost = mode, host
	r.mu.Unlock()
}

// declareDetachLog records where a custom session's failed driver close is
// logged: the Supervisor's Logf. Only [NewSupervisor] calls it.
func (r *Registry) declareDetachLog(logf func(format string, args ...any)) {
	r.mu.Lock()
	r.detachLogf = logf
	r.mu.Unlock()
}

func (r *Registry) attachPolicy() (AttachMode, Host) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.attachMode, r.attachHost
}

// NewRegistry builds the registry from cfg: one handle per cell×loop, keyed
// "<cell>/<role>", carrying that loop's optional parsed target. A nil cfg
// yields an empty registry (every read is then a 404 for an unknown desk).
func NewRegistry(cfg *schema.File) *Registry {
	return newRegistryWithClock(cfg, func() time.Time { return time.Now().UTC() })
}

func newRegistryWithClock(cfg *schema.File, now func() time.Time) *Registry {
	r := &Registry{now: now, handles: map[string]*Handle{}, epoch: now(), boot: newBootNonce()}
	if cfg == nil {
		return r
	}
	for _, c := range cfg.Cells {
		for _, l := range c.Loops {
			id := DeskID(c.Name, string(l.Role))
			h := newHandleAt(id, l.Target, now, r.epoch)
			h.boot = r.boot
			r.handles[id] = h
			r.order = append(r.order, id)
		}
	}
	return r
}

// Len is the number of configured desks (rows the registry resolves).
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.order)
}

// WithTarget is the count of configured desks that carry an attachable target.
func (r *Registry) WithTarget() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, id := range r.order {
		if h := r.handles[id]; h != nil && h.spec != nil {
			n++
		}
	}
	return n
}

// Resolve returns the wire handle for a desk id and whether the desk is
// CONFIGURED. A configured desk with no target — or an unreachable one — is a
// found handle whose state is could-not-check, never a miss (spec §5.2). Only a
// desk id that names no configured desk is a miss (the handler 404s that).
func (r *Registry) Resolve(id string) (contract.TerminalHandle, bool) {
	r.mu.Lock()
	h := r.handles[id]
	r.mu.Unlock()
	if h == nil {
		return contract.TerminalHandle{}, false
	}
	h.Refresh()
	return h.view(), true
}

// Handles returns the wire view of every configured desk, in config order — the
// one-row-per-desk listing the desk__desks MCP tool and the startup banner
// read. It never omits a desk for being unreachable.
func (r *Registry) Handles() []contract.TerminalHandle {
	r.mu.Lock()
	ids := append([]string(nil), r.order...)
	handles := make([]*Handle, len(ids))
	for i, id := range ids {
		handles[i] = r.handles[id]
	}
	r.mu.Unlock()
	out := make([]contract.TerminalHandle, 0, len(ids))
	for _, h := range handles {
		if h == nil {
			continue
		}
		h.Refresh()
		out = append(out, h.view())
	}
	return out
}

// handle returns the internal handle for a desk id (registry-internal + tests).
func (r *Registry) handle(id string) *Handle {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handles[id]
}

// ErrAlreadyAttached refuses a second attach while a desk's session is live:
// one session per desk (the service contract). The attempt is refused, not
// queued.
var ErrAlreadyAttached = errors.New("terminal: desk already has a live session")

// Attach opens the driver session for a configured desk and starts pumping its
// output into the handle's ring. An [ErrUnreachable] or [ErrUnauthorized] from
// the driver is rendered as could-not-check (the handle stays present); any
// other error is returned. In production the [Supervisor] is the only caller
// of the attach path (the service contract); this entry point stays for the
// drivers' own tests.
func (r *Registry) Attach(ctx context.Context, id string, t Target, size Size) error {
	h := r.handle(id)
	if h == nil {
		return fmt.Errorf("terminal: no configured desk %q to attach", id)
	}
	_, err := r.attachHandle(ctx, h, t, size)
	if errors.Is(err, ErrUnreachable) || errors.Is(err, ErrUnauthorized) {
		return nil
	}
	return err
}

// attachHandle is the one attach path. On success it returns a channel that
// closes when the session ends (pump has drained and closed it). The driver's
// error is returned unwrapped so the supervisor can classify it. A live
// session refuses a second attach.
func (r *Registry) attachHandle(ctx context.Context, h *Handle, t Target, size Size) (<-chan struct{}, error) {
	h.mu.Lock()
	live := h.session != nil
	h.mu.Unlock()
	if live {
		return nil, fmt.Errorf("desk %q: %w", h.deskID, ErrAlreadyAttached)
	}
	sess, err := t.Attach(ctx, h.spec, size)
	if err != nil {
		if errors.Is(err, ErrUnreachable) || errors.Is(err, ErrUnauthorized) {
			h.markUnreachable()
		}
		return nil, err
	}
	// attach() returns this session's epoch sequence; pump tears down only
	// the session it drained (review pr192-F2 (i)/(ii)).
	n := h.attach(sess)
	if !h.isSupervised() {
		h.setAttach(AttachAttached, ReasonNone, h.attemptCount(), nil, r.now())
	}
	done := make(chan struct{})
	go r.pump(h, sess, n, done)
	return done, nil
}

func (h *Handle) attemptCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.attempts
}

func (h *Handle) isSupervised() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.supervised
}

// pump drains a session's output into the handle until the channel closes,
// then closes the session. On a normal exit the driver closes its Read()
// channel from underneath (the container exits, the local PTY hangs up) —
// nothing else ever observes that and calls Close(), so without this a
// session's resources (a pty master fd, a docker-exec child's host-side pty)
// leak on every attach/detach cycle, for whichever driver is attached. Fixed
// once here, at the one place every driver's Session is consumed, rather than
// per driver (review finding on #137, same root cause as #135).
//
// When the session ends, done is closed so the supervisor learns the session
// is over and records the transition. An unsupervised handle (a driver test's
// direct Attach) reads failed / exited (the service contract). The ring and its cursor are untouched: a
// re-attach inside one deskd process appends to the same scrollback.
func (r *Registry) pump(h *Handle, sess Session, epochN uint64, done chan<- struct{}) {
	for b := range sess.Read() {
		h.Ingest(b)
	}
	_ = sess.Close()
	h.mu.Lock()
	// Only the session this pump drained is torn down: a close or re-attach
	// may already have moved the epoch on, and that newer state is not ours.
	ours := h.epochN == epochN
	if ours {
		h.connected = false
		h.session = nil
		h.bumpEpochLocked()
	}
	attempts, supervised := h.attempts, h.supervised
	h.mu.Unlock()
	if ours && !supervised {
		h.setAttach(AttachFailed, ReasonExited, attempts, nil, r.now())
	}
	if done != nil {
		close(done)
	}
}

// Detach closes a configured desk's live session, if it has one. It closes
// deskd's OWN attachment only: the container, pod or tmux pane keeps running.
// A local desk is deskd's child and ends with its session. Detach is how the
// supervisor tears sessions down on shutdown. It opens no input path.
func (r *Registry) Detach(id string) error {
	h := r.handle(id)
	if h == nil {
		return fmt.Errorf("terminal: no configured desk %q", id)
	}
	h.mu.Lock()
	sess := h.session
	h.mu.Unlock()
	if sess == nil {
		return nil
	}
	return sess.Close()
}

// ids returns the configured desk ids in config order.
func (r *Registry) ids() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

// Write sends p to a configured desk's live session — the client-to-target
// half of the byte pipe [Target]/[Session] describe (target.go: "Write sends
// input; the send route gates the write route, but the transport is here"). This is
// Go-level plumbing only: no wire route reaches it. The send route's `send` named
// action writes through [Registry.WaitThenSend], which binds the write to the
// handle epoch, never through Write, so nothing outside this process can call
// Write. It exists because the container driver's own Verify
// must prove bytes a caller writes reach the container AND the container's
// reply reaches the ring — through the registry, not by holding the driver's
// [Session] directly — and that round trip needs a write path to exist.
func (r *Registry) Write(id string, p []byte) error {
	h := r.handle(id)
	if h == nil {
		return fmt.Errorf("terminal: no configured desk %q", id)
	}
	return h.write(p)
}

// Resize forwards a window resize to a configured desk's live session, or
// queues it (once) for the next attach if none is live yet.
func (r *Registry) Resize(id string, cols, rows int) error {
	h := r.handle(id)
	if h == nil {
		return fmt.Errorf("terminal: no configured desk %q", id)
	}
	return h.resize(cols, rows)
}

// Alive reports whether a configured desk's live session is still attached
// (false with no session at all — never attached, or unreachable).
func (r *Registry) Alive(id string) bool {
	h := r.handle(id)
	if h == nil {
		return false
	}
	return h.alive()
}

// Since returns a configured desk's scrollback bytes written after cursor c.
// It is the same primitive the terminal stream's SSE stream will resume clients from
// ([ErrCursorEvicted] on a cursor older than the ring's retained window);
// exposed here so this brief's Verify can confirm ring content through the
// registry seam rather than by re-deriving it from a held [Session].
func (r *Registry) Since(id string, cursor uint64) ([]byte, error) {
	h := r.handle(id)
	if h == nil {
		return nil, fmt.Errorf("terminal: no configured desk %q", id)
	}
	return h.since(cursor)
}

// ── driver registration (the service contract, the service contract, the service contract) ──
//
// A driver constructor is registered here, by kind, rather than imported by
// name: the container driver (13), the pod driver (14) and the local-PTY
// driver (15) each pull in their own construction dependency (the pod
// driver's is k8s.io/client-go — a dependency this package must not force on
// a build that never attaches to a cluster), so this package cannot import
// any of them directly without forcing every driver's dependency graph onto
// every consumer of the registry. A driver package instead calls
// [RegisterDriver] from its own init(); the wiring that actually calls
// [DriverFor] and attaches — main.go's per-desk loop — lands with whichever
// driver brief lands it (co-ordination point across 13/14/15, not decided by
// this file alone).
//
// The registered value is a FACTORY (func() (Target, error)), not a bare
// Target: the pod driver's construction can fail (no kubeconfig, no
// in-cluster config) and that failure needs to surface to whatever calls
// DriverFor, not get swallowed at package-init time. A driver whose
// constructor never fails (the local-PTY driver's New()) just wraps itself
// in a factory that always returns a nil error.
var (
	driverMu sync.Mutex
	drivers  = map[schema.TargetKind]func() (Target, error){}
)

// RegisterDriver registers factory as the constructor for kind. Call it from
// a driver package's init() (see internal/terminal/pod's doc.go for the
// kind: pod registration, and internal/terminal/local's driver.go for
// kind: local). A second registration for the same kind is a programming
// error — a build linking two drivers for one kind — and panics rather than
// silently picking one.
func RegisterDriver(kind schema.TargetKind, factory func() (Target, error)) {
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, exists := drivers[kind]; exists {
		panic(fmt.Sprintf("terminal: a driver for kind %q is already registered", kind))
	}
	drivers[kind] = factory
}

// DriverFor returns the registered constructor for kind, and whether one has
// registered at all — false means the running binary never imported that
// kind's driver package (e.g. a build that excludes the pod driver), not that
// the kind is invalid (schema validation is the authority on that).
func DriverFor(kind schema.TargetKind) (factory func() (Target, error), ok bool) {
	driverMu.Lock()
	defer driverMu.Unlock()
	factory, ok = drivers[kind]
	return factory, ok
}
