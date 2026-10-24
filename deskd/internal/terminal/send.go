// send.go — [Registry.WaitThenSend], the WRITE half of the terminal seam
// (the service contract): one line or key sequence into one desk's live session,
// optionally after waiting for the desk to reach its prompt.
//
// # Atomic wait-then-send
//
// The wait and the write hold the handle's SEND LOCK (a one-slot semaphore, so
// a waiter can give up on its deadline instead of blocking forever), so no
// other send interleaves between "the desk is at its prompt" and "the line is
// written". The handle's state lock is NOT held across the wait — the pump must
// keep ingesting output, or the state the wait is watching could never change.
// A timeout writes NOTHING: the only write is after the condition held.
//
// # Layer (d) of the brief's single point of failure
//
// The registry refuses on its own signals, independently of the HTTP layer
// above it: a desk outside the caller's cell ([ErrWrongCell]), a handle with no
// live session ([ErrNotWritable]), and an epoch that is not the one the input
// was composed against ([ErrEpochChanged]). A send that reached a PTY passed
// all of them here, whatever the route did.
//
// # Report F-2: bound to one session, never replayed
//
//   - The send is bound to the handle's EPOCH at arrival AND re-checked after
//     the wait: an attach, detach or close moves the epoch and cancels the
//     queued input — it is never delivered to a replacement session. The epoch
//     carries a per-registry boot nonce, so a restarted deskd never re-issues
//     one a client still holds.
//   - The caller's authority is re-validated after the wait
//     ([SendBinding.Revalidate]); revoked access cancels the input.
//   - Input size and pending waits are bounded ([MaxSendBytes],
//     [MaxPendingSends]).
//   - Outcomes are three-state (contract.SendOutcome). A write that does not
//     report success is outcome-unknown, never "not sent", and is never
//     retried. A request id with a receipt returns that receipt and writes
//     nothing; receipts are bounded, session-local (dropped when the epoch
//     moves) and memory-only — after a restart the uncertainty stays explicit
//     as an epoch mismatch, never a silent re-send.
//   - Nothing here reads terminal text as an instruction. The bytes a desk
//     prints are scrollback; they cannot approve, ratify or mint anything, and
//     this package imports nothing that could (norest_test.go / the serve
//     authority test pin the import graph).
package terminal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/contract"
)

const (
	// MaxSendBytes bounds one send's input. A line or a key sequence is small;
	// a paste larger than this is refused rather than streamed into an agent.
	MaxSendBytes = 16 << 10
	// MaxPendingSends bounds the sends queued on one handle (waiting for the
	// send lock or the wait condition). The next one is refused, not queued.
	MaxPendingSends = 8
	// MaxReceipts bounds the session-local receipt list per handle.
	MaxReceipts = 64
	// DefaultSendTimeout is the wait bound when the request names none.
	DefaultSendTimeout = 10 * time.Second
	// MaxSendTimeout is the largest wait a request may ask for.
	MaxSendTimeout = 60 * time.Second
)

// sendPoll is how often a waiting send re-derives the desk's state. A
// variable only so a test can shorten it.
var sendPoll = 20 * time.Millisecond

// requestIDShape is the admissible request id: a short opaque token.
var requestIDShape = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// The refusals. Every one but [ErrOutcomeUnknown] means NOTHING was written.
var (
	ErrNoSuchDesk        = errors.New("no such desk")
	ErrWrongCell         = errors.New("the desk is not in the caller's cell")
	ErrNotWritable       = errors.New("not writable: the desk has no live session")
	ErrEpochRequired     = errors.New("epoch is required: a send is bound to the session it was composed against (read the handle's epoch first)")
	ErrEpochChanged      = errors.New("epoch changed: the desk's session is not the one this input was composed against")
	ErrSendTimeout       = errors.New("timeout")
	ErrSendCanceled      = errors.New("canceled: the caller went away before the wait finished")
	ErrRevoked           = errors.New("authority revoked during the wait")
	ErrInputEmpty        = errors.New("input is empty")
	ErrInputTooLarge     = fmt.Errorf("input exceeds %d bytes", MaxSendBytes)
	ErrBadWait           = errors.New("wait_for must be one of prompt | idle | none")
	ErrBadTimeout        = fmt.Errorf("timeout_ms must be between 0 and %d", MaxSendTimeout.Milliseconds())
	ErrBadRequestID      = errors.New("request_id must be 1-64 characters of [A-Za-z0-9._-]")
	ErrTooManyPending    = fmt.Errorf("too many pending sends on this desk (limit %d)", MaxPendingSends)
	ErrDuplicateInFlight = errors.New("a send with this request_id is already in flight")
	// ErrOutcomeUnknown: the write was attempted and did not report success.
	// Some or all bytes may have reached the desk; the receipt records it and
	// the send is never retried by the service.
	ErrOutcomeUnknown = errors.New("outcome unknown: the write did not report success")
)

// SendBinding is what a send is bound to besides the desk id: the caller's
// authorized cell, the epoch the input was composed against, an optional
// idempotency key, and the post-wait authority re-check.
type SendBinding struct {
	// Cell is the cell the caller was authorized on. The desk id must belong to
	// it ([ErrWrongCell]).
	Cell string
	// Epoch is the handle epoch the caller composed the input against.
	// Required ([ErrEpochRequired]).
	Epoch string
	// RequestID is the caller's idempotency key ("" = none).
	RequestID string
	// Revalidate re-checks the caller's authority AFTER the wait, immediately
	// before the write. A non-nil error cancels the send ([ErrRevoked]). nil
	// means the caller supplied no re-check (tests of this package only).
	Revalidate func() error
}

// SendReceipt is what the registry knows about one send.
type SendReceipt struct {
	DeskID      string
	RequestID   string
	Epoch       string
	Outcome     contract.SendOutcome
	Waited      time.Duration
	CursorAfter uint64
	StateAfter  AgentState
	// Duplicate: this is an earlier send's receipt, returned for a retry with
	// the same request id — this call wrote nothing.
	Duplicate bool
}

// Result projects the receipt onto the wire type.
func (rc SendReceipt) Result() contract.SendResult {
	return contract.SendResult{
		DeskID:      rc.DeskID,
		RequestID:   rc.RequestID,
		Epoch:       rc.Epoch,
		Outcome:     rc.Outcome,
		WaitedMS:    rc.Waited.Milliseconds(),
		CursorAfter: rc.CursorAfter,
		StateAfter:  string(rc.StateAfter),
		Duplicate:   rc.Duplicate,
	}
}

// sendState is one handle's send-path state. sem is the send lock; the rest
// is guarded by the handle's mu.
type sendState struct {
	sem      chan struct{}
	pending  int
	inFlight map[string]bool
	receipts []SendReceipt
}

func newSendState() sendState {
	return sendState{sem: make(chan struct{}, 1), inFlight: map[string]bool{}}
}

func (s *sendState) receipt(requestID, epoch string) (SendReceipt, bool) {
	for i := len(s.receipts) - 1; i >= 0; i-- {
		if rc := s.receipts[i]; rc.RequestID == requestID && rc.Epoch == epoch {
			return rc, true
		}
	}
	return SendReceipt{}, false
}

func (s *sendState) record(rc SendReceipt) {
	s.receipts = append(s.receipts, rc)
	if over := len(s.receipts) - MaxReceipts; over > 0 {
		s.receipts = append([]SendReceipt(nil), s.receipts[over:]...)
	}
}

// processBoot is the fallback nonce for a handle built outside a registry.
var processBoot = newBootNonce()

// newBootNonce returns a fresh random nonce for epoch values. A failure of the
// system random source is not survivable for an id meant never to repeat, so
// it falls back to the wall clock rather than to a constant.
func newBootNonce() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// bumpEpochLocked moves the handle to a new epoch and drops the receipts of
// the old one (they are session-local). Caller holds h.mu.
func (h *Handle) bumpEpochLocked() {
	h.epochN++
	h.send.receipts = nil
}

// epochLocked renders the current epoch, "" before the first attach. Caller
// holds h.mu.
func (h *Handle) epochLocked() string {
	if h.epochN == 0 {
		return ""
	}
	return fmt.Sprintf("%s.%d", h.boot, h.epochN)
}

// Epoch returns a configured desk's current epoch ("" if never attached or
// unknown).
func (r *Registry) Epoch(id string) string {
	h := r.handle(id)
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.epochLocked()
}

// deskCell is the cell a desk id belongs to: everything before the first
// slash ("<cell>/<role>" or "<cell>/custom/<id>").
func deskCell(id string) string {
	cell, _, ok := strings.Cut(id, "/")
	if !ok {
		return ""
	}
	return cell
}

// ValidateSend checks a send's shape without touching any handle: the wait
// condition, the input bounds, the request id and the timeout. The HTTP layer
// calls it before authority is re-derived for the write; WaitThenSend calls it
// again so the registry never relies on its caller having done so.
func ValidateSend(input []byte, wait contract.SendWait, timeout time.Duration, requestID string) error {
	switch wait {
	case contract.SendWaitPrompt, contract.SendWaitIdle, contract.SendWaitNone:
	default:
		return ErrBadWait
	}
	if len(input) == 0 {
		return ErrInputEmpty
	}
	if len(input) > MaxSendBytes {
		return ErrInputTooLarge
	}
	if timeout < 0 || timeout > MaxSendTimeout {
		return ErrBadTimeout
	}
	if requestID != "" && !requestIDShape.MatchString(requestID) {
		return ErrBadRequestID
	}
	return nil
}

// WaitThenSend writes input to desk id's live session once the wait condition
// holds, in ONE write, holding the handle's send lock from before the wait
// until after the write. See the file doc for every refusal. A zero timeout is
// [DefaultSendTimeout].
func (r *Registry) WaitThenSend(ctx context.Context, id string, input []byte, wait contract.SendWait, timeout time.Duration, b SendBinding) (SendReceipt, error) {
	rc := SendReceipt{DeskID: id, RequestID: b.RequestID, Epoch: b.Epoch, Outcome: contract.SendNotSent, StateAfter: StateCouldNotCheck}
	if err := ValidateSend(input, wait, timeout, b.RequestID); err != nil {
		return rc, err
	}
	if timeout == 0 {
		timeout = DefaultSendTimeout
	}
	var h *Handle
	if r != nil {
		h = r.handle(id)
	}
	if h == nil {
		return rc, ErrNoSuchDesk
	}
	// Layer (d): the caller's authorized cell must own this desk. Decided from
	// the id the registry keys on, never from anything else in the request.
	if b.Cell == "" || deskCell(id) != b.Cell {
		return rc, ErrWrongCell
	}
	if b.Epoch == "" {
		return rc, ErrEpochRequired
	}

	start := time.Now()
	finish := func(err error) (SendReceipt, error) {
		rc.Waited = time.Since(start)
		rc.StateAfter = h.AgentState()
		return rc, err
	}

	// Admission: a duplicate returns its receipt and writes nothing; the
	// pending bound refuses rather than queues.
	h.mu.Lock()
	if b.RequestID != "" {
		if prior, ok := h.send.receipt(b.RequestID, b.Epoch); ok {
			h.mu.Unlock()
			prior.Duplicate = true
			return prior, nil
		}
		if h.send.inFlight[b.RequestID] {
			h.mu.Unlock()
			return finish(ErrDuplicateInFlight)
		}
	}
	if h.send.pending >= MaxPendingSends {
		h.mu.Unlock()
		return finish(ErrTooManyPending)
	}
	h.send.pending++
	if b.RequestID != "" {
		h.send.inFlight[b.RequestID] = true
	}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		h.send.pending--
		if b.RequestID != "" {
			delete(h.send.inFlight, b.RequestID)
		}
		h.mu.Unlock()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// The send lock. Waiting for it counts against the same deadline.
	select {
	case h.send.sem <- struct{}{}:
	case <-timer.C:
		return finish(ErrSendTimeout)
	case <-ctx.Done():
		return finish(ErrSendCanceled)
	}
	defer func() { <-h.send.sem }()

	// The wait. The binding is re-checked on every tick, so a session that
	// changes under a waiting send cancels it at once rather than at the
	// deadline.
	for {
		if err := h.boundTo(b.Epoch); err != nil {
			return finish(err)
		}
		if h.waitSatisfied(wait) {
			break
		}
		select {
		case <-timer.C:
			return finish(ErrSendTimeout)
		case <-ctx.Done():
			return finish(ErrSendCanceled)
		case <-time.After(sendPoll):
		}
	}

	// Authority after the wait: revoked access, an expired session or a
	// changed answerer cancels the input.
	if b.Revalidate != nil {
		if err := b.Revalidate(); err != nil {
			return finish(fmt.Errorf("%w: %v", ErrRevoked, err))
		}
	}

	// The final bind and the session capture happen under the state lock; the
	// write itself does not hold it (a PTY write may block on output the pump
	// must keep draining). The captured session is the bound one: a later
	// replacement is never written to.
	h.mu.Lock()
	sess := h.session
	if sess == nil {
		h.mu.Unlock()
		return finish(ErrNotWritable)
	}
	if h.epochLocked() != b.Epoch {
		h.mu.Unlock()
		return finish(ErrEpochChanged)
	}
	h.mu.Unlock()

	werr := sess.Write(input)

	h.mu.Lock()
	rc.Waited = time.Since(start)
	rc.CursorAfter = h.ring.Cursor()
	h.refreshLocked(h.now())
	rc.StateAfter = h.state()
	rc.Outcome = contract.SendSent
	if werr != nil {
		rc.Outcome = contract.SendOutcomeUnknown
	}
	// Record the receipt only for a write that was attempted, and only while
	// the epoch is still the bound one (a moved epoch dropped the receipts).
	if b.RequestID != "" && h.epochLocked() == b.Epoch {
		h.send.record(rc)
	}
	h.mu.Unlock()
	if werr != nil {
		return rc, fmt.Errorf("%w: %v", ErrOutcomeUnknown, werr)
	}
	return rc, nil
}

// boundTo reports whether the handle still has a live session at epoch.
func (h *Handle) boundTo(epoch string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.session == nil {
		return ErrNotWritable
	}
	if h.epochLocked() != epoch {
		return ErrEpochChanged
	}
	return nil
}

// waitSatisfied re-derives the handle's signals and reports whether the wait
// condition holds now.
func (h *Handle) waitSatisfied(wait contract.SendWait) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	h.refreshLocked(now)
	switch wait {
	case contract.SendWaitNone:
		return true
	case contract.SendWaitPrompt:
		// A settled prompt, including a desk asking a human (spec §5.1).
		s := h.state()
		return s == StateIdle || s == StateWaiting
	case contract.SendWaitIdle:
		return h.connected && now.Sub(h.lastOutput) >= h.idleWindow
	}
	return false
}
