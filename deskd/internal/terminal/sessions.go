// sessions.go — the CUSTOM terminal session store (the service contract,
// "+ open a session"): an extra session on a configured desk's own target.
//
// The client names a CONFIGURED desk (`on`), never a target: the session opens
// on that desk's parsed cells.yaml `target:` through the same driver a
// configured desk uses (the service contract's single point of failure — the
// registry, not the request, chooses where bytes go).
//
// Memory only. The store is this registry's handle map and an id list; nothing
// here is written under the data dir (norest_test.go walks this file with the
// rest of the package). A deskd restart forgets every custom session — its
// process dies with the service — and the rows vanish honestly rather than
// being resurrected from a file.
//
// Close is for custom sessions only: a configured desk is refused
// ([ErrConfiguredDesk]) on the handle's own flag, not on the spelling of its id.
package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/contract"
)

// MaxCustomSessionsPerCell bounds the custom sessions open on one cell.
const MaxCustomSessionsPerCell = 8

var (
	ErrConfiguredDesk    = errors.New("configured desk: only a custom session can be closed")
	ErrNoSuchSession     = errors.New("no such custom session")
	ErrNotConfiguredDesk = errors.New("a session opens on a CONFIGURED desk, never on another custom session")
	ErrNoTarget          = errors.New("the desk has no attachable target configured (could-not-check)")
	ErrNoDriver          = errors.New("no driver for the desk's target kind is available on this build (could-not-check)")
	ErrTooManySessions   = fmt.Errorf("too many custom sessions on this cell (limit %d)", MaxCustomSessionsPerCell)
	// ErrClusterUnauthorized refuses a custom session on a pod desk: deskd
	// holds no cluster access grant (the authorization contract ruling — cluster desks
	// read "unauthorized" until access is separately granted), so the pod
	// driver is never built for one and no API server is contacted.
	ErrClusterUnauthorized = errors.New("pod desk: deskd holds no cluster access grant, so a session cannot open (unauthorized)")
	// ErrAttachOff refuses every session open while deskd's attach lifecycle
	// is off (--terminal-attach=off), has not started, or has stopped: the
	// start-up off switch covers the request-caused exec path too
	// (obligation (vi); DR-deskd-attach-start's fail-closed reading).
	ErrAttachOff = errors.New("terminal attach is off on this deskd (--terminal-attach=off, or the attach lifecycle is not running), so no session opens")
	// ErrKindUnsupported refuses a session open for a kind this host cannot
	// attach, decided by [CapabilityFor] itself, the same function the
	// supervisor asks (obligation (vii)).
	ErrKindUnsupported = errors.New("this deskd host cannot attach the desk's kind")
)

// customSession is the metadata a custom session's handle carries.
type customSession struct {
	on       string
	title    string
	openedBy string
	openedAt time.Time
}

// CustomSessionID is the desk id a custom session is addressed by:
// "<cell>/custom/<random>".
func CustomSessionID(cell, random string) string { return cell + "/custom/" + random }

// OpenSession opens a custom session on the configured desk `on`, attaching
// the driver for that desk's OWN target kind to that desk's OWN target spec —
// driver is asked for a kind the registry read from cells.yaml, never one the
// request named. cell is the caller's authorized cell (it must own `on`);
// openedBy is the authenticated login, recorded for the row.
func (r *Registry) OpenSession(ctx context.Context, on, cell, openedBy, title string, driver func(schema.TargetKind) (Target, error), size Size) (contract.SessionRow, error) {
	if r == nil {
		return contract.SessionRow{}, ErrNoSuchDesk
	}
	// Obligation (vi): with the attach lifecycle off (or not running) every
	// open is refused, before the desk is even looked up and long before any
	// driver factory runs.
	mode, host := r.attachPolicy()
	if mode != AttachModeConfigured {
		return contract.SessionRow{}, ErrAttachOff
	}
	base := r.handle(on)
	if base == nil {
		return contract.SessionRow{}, ErrNoSuchDesk
	}
	if base.custom != nil {
		return contract.SessionRow{}, ErrNotConfiguredDesk
	}
	if cell == "" || deskCell(on) != cell {
		return contract.SessionRow{}, ErrWrongCell
	}
	if base.spec == nil {
		return contract.SessionRow{}, ErrNoTarget
	}
	// Decided on the configured kind BEFORE any driver factory runs: a pod
	// driver's factory loads a kubeconfig / in-cluster credential, and deskd
	// holds no cluster access grant. Refused here, never attempted and failed.
	if base.spec.Kind == schema.TargetPod {
		return contract.SessionRow{}, ErrClusterUnauthorized
	}
	// Obligation (vii): the host must be able to attach this kind, decided
	// by CapabilityFor itself (never a copy of its table), before any driver
	// factory runs.
	if c := CapabilityFor(base.spec, host); c.Support != SupportSupported {
		return contract.SessionRow{}, fmt.Errorf("%w (%s: %s %s on %s/%s)", ErrKindUnsupported, c.Support, base.spec.Kind, c.Reason, c.HostOS, c.HostArch)
	}
	if driver == nil {
		return contract.SessionRow{}, ErrNoDriver
	}
	t, err := driver(base.spec.Kind)
	if err != nil || t == nil {
		return contract.SessionRow{}, ErrNoDriver
	}

	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return contract.SessionRow{}, fmt.Errorf("could not mint a session id: %w", err)
	}
	id := CustomSessionID(cell, hex.EncodeToString(nonce[:]))
	// attach.epoch is per deskd process (the service contract): a custom
	// session's handle carries the registry's, like every configured desk.
	h := newHandleAt(id, base.spec, r.now, r.epoch)
	h.boot = r.boot
	h.custom = &customSession{on: on, title: title, openedBy: openedBy, openedAt: r.now()}

	// Reserve the slot under the registry lock so two concurrent opens cannot
	// both pass the bound; release it if the attach fails.
	r.mu.Lock()
	// Obligation (vi) again, at insertion (security review A-sec-stop-open-race):
	// an open that passed the mode check above before Stop began must not
	// insert after closeCustomSessions has taken its snapshot. Stop declares
	// off under r.mu BEFORE that snapshot (also under r.mu), so this re-read
	// sees off for every insertion the snapshot would miss; an insertion ahead
	// of the snapshot is closed by it, through the closed-while-attaching
	// check below.
	if r.attachMode != AttachModeConfigured {
		r.mu.Unlock()
		return contract.SessionRow{}, ErrAttachOff
	}
	if r.customCountLocked(cell) >= MaxCustomSessionsPerCell {
		r.mu.Unlock()
		return contract.SessionRow{}, ErrTooManySessions
	}
	r.handles[id] = h
	r.sessions = append(r.sessions, id)
	r.mu.Unlock()

	sess, err := t.Attach(ctx, base.spec, size)
	if err != nil {
		r.forget(id)
		return contract.SessionRow{}, err
	}
	n := h.attach(sess)
	if r.handle(id) != h {
		// Closed while the driver was attaching: the row is gone, so the
		// session it just opened must not outlive it.
		r.logFailedDetach(id, sess.Close())
		return contract.SessionRow{}, ErrNoSuchSession
	}
	// Unsupervised: the open itself records "attached", and pump records
	// failed / exited when this session ends (the service contract).
	h.setAttach(AttachAttached, ReasonNone, 0, nil, r.now())
	go r.pump(h, sess, n, nil)
	return h.sessionRow(), nil
}

// CloseSession closes a custom session and forgets it. A configured desk is
// refused ([ErrConfiguredDesk]); cell is the caller's authorized cell.
//
// It returns only refusals. The row is forgotten before the driver's Close
// runs, so by then the close has happened and a retry finds no row. A failed
// driver close (for a container session, the detach exec for deskd's shell)
// is therefore logged with the session id, as the [Supervisor] logs a
// configured desk's, and not returned: the driver's text names the container
// and the shell's pid, and the caller writes its error onto the wire (review
// F-cor-7).
func (r *Registry) CloseSession(id, cell string) error {
	h := r.handle(id)
	if h == nil {
		return ErrNoSuchSession
	}
	if cell == "" || deskCell(id) != cell {
		return ErrWrongCell
	}
	if h.custom == nil {
		return ErrConfiguredDesk
	}
	r.forget(id)
	h.mu.Lock()
	sess := h.session
	h.session = nil
	h.connected = false
	h.bumpEpochLocked() // cancels any send still waiting on this session
	h.mu.Unlock()
	if sess != nil {
		r.logFailedDetach(id, sess.Close())
	}
	return nil
}

// logFailedDetach logs err, a custom session's failed driver close, under the
// session id, in the line the [Supervisor] writes for a configured desk. It
// goes to the Supervisor's Logf (nil = discard, as there); a custom session
// only opens under a running Supervisor, so one is always declared.
func (r *Registry) logFailedDetach(id string, err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	logf := r.detachLogf
	r.mu.Unlock()
	if logf != nil {
		logf("deskd: terminal attach: desk=%s detach failed: %v", id, err)
	}
}

// closeCustomSessions closes every custom session, in parallel: shutdown
// tears down every session deskd holds, and a custom session is not in the
// configured order [Supervisor.Stop] detaches (review pr192-F2 (iv)). Each
// close is the driver's own bounded Close. A failed one is logged inside
// CloseSession (review F-cor-7); the only error left to drop here is a
// refusal, a DELETE that closed the session first.
func (r *Registry) closeCustomSessions() {
	r.mu.Lock()
	ids := append([]string(nil), r.sessions...)
	r.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = r.CloseSession(id, deskCell(id))
		}(id)
	}
	wg.Wait()
}

// Sessions lists a cell's custom sessions in open order.
func (r *Registry) Sessions(cell string) []contract.SessionRow {
	hs := r.customHandles(cell)
	out := make([]contract.SessionRow, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.sessionRow())
	}
	return out
}

// customHandles returns a cell's custom-session handles in open order.
func (r *Registry) customHandles(cell string) []*Handle {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var hs []*Handle
	for _, id := range r.sessions {
		if h := r.handles[id]; h != nil && deskCell(id) == cell {
			hs = append(hs, h)
		}
	}
	return hs
}

func (r *Registry) customCountLocked(cell string) int {
	n := 0
	for _, id := range r.sessions {
		if deskCell(id) == cell {
			n++
		}
	}
	return n
}

// forget removes a custom session from the registry.
func (r *Registry) forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handles, id)
	for i, s := range r.sessions {
		if s == id {
			r.sessions = append(r.sessions[:i:i], r.sessions[i+1:]...)
			break
		}
	}
}

// sessionRow projects a custom session's handle onto the wire row.
func (h *Handle) sessionRow() contract.SessionRow {
	h.mu.Lock()
	defer h.mu.Unlock()
	row := contract.SessionRow{ID: h.deskID}
	if h.spec != nil {
		row.TargetKind = string(h.spec.Kind)
	}
	if h.custom != nil {
		row.On = h.custom.on
		row.Title = h.custom.title
		row.OpenedBy = h.custom.openedBy
		row.OpenedAt = h.custom.openedAt
	}
	return row
}

// RegisteredDriver resolves kind through the driver registration table
// ([DriverFor]) — the resolver deskd hands [Registry.OpenSession]. A kind no
// linked driver package registered is [ErrNoDriver] (could-not-check), never a
// fallback to another driver.
func RegisteredDriver(kind schema.TargetKind) (Target, error) {
	factory, ok := DriverFor(kind)
	if !ok {
		return nil, ErrNoDriver
	}
	return factory()
}
