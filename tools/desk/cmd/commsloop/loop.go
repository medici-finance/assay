package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// loop.go — commsloop's loopengine.Loop adapter: the DRAIN half of the cell
// gateway. commsgw accepts and queues; this consumer reads the SAME
// accepted-queue (a separate process/binary, agreeing on disk — see
// internal/commsqueue) and lands every item exactly once.
//
// FROZEN CONTRACT (arch doc §8): Name/SelectQueue/TierPolicy/Dispatch/Land/
// OnIdle, implemented exactly — see the compile-time assertion below.
//
// NO DETERMINISTIC ROUTING (#1767 ruling 3). commsloop answers exactly one
// question per message — is this a pure REPORT it can land immediately with
// no session fired, or does it need the not-yet-built prose router's
// judgment — and never invents a routing decision the router owns. Until the
// router lands, every non-report message quarantines: inert either way,
// fail-closed by construction.
type Loop struct {
	// Root is the gateway's queue directory (ASSAY_COMMS_QUEUE_DIR).
	Root string
	// Mon is the accepted-queue read surface. Required; see monitor.go.
	Mon Monitor
	// ACL is commsloop's OWN compiled lane ACL — independent of commsgw's copy
	// (routing.go's file doc explains why it is a second instance, not a
	// shared call).
	ACL *comms.ACL
	// GuardFn is the kill-switch check. nil defaults to deskkit.Guard; a test
	// substitutes a fake to prove "STOP flag halts mid-drain" without the real
	// ~/.config/assay state directory.
	GuardFn func() error
	// Filer raises the quarantine issue. nil means "held, no issue filed" —
	// still never a drop, just missing the second half of the silent-desk
	// rule (a real Loop always supplies one; see main.go's construction).
	Filer commsqueue.IssueFiler
	// Now is a test seam; nil means time.Now (UTC).
	Now func() time.Time

	mu      sync.Mutex
	reasons map[string]string // item ID -> quarantine reason, set by TierPolicy, consumed by Land.
}

// var _ loopengine.Loop = (*Loop)(nil) pins the frozen contract at compile
// time — Verify row 5.
var _ loopengine.Loop = (*Loop)(nil)

func (l *Loop) clock() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now().UTC()
}

func (l *Loop) guard() error {
	if l.GuardFn != nil {
		return l.GuardFn()
	}
	return deskkit.Guard()
}

// Name implements loopengine.Loop.
func (l *Loop) Name() string { return "commsloop" }

// envelopeItemKey is the map key under Item.Payload the parsed envelope's JSON
// is stored at (SelectQueue writes it; TierPolicy/Land read it back — this
// package's ONE encoding of "an accepted message, carried as a loopengine
// Item").
const envelopeItemKey = "envelope"

// SelectQueue implements loopengine.Loop: a deterministic read of the
// accepted-queue, gated by the kill switch FIRST (so an armed STOP halts the
// drain before a single item is even read this cycle — Verify row 6).
func (l *Loop) SelectQueue() ([]loopengine.Item, error) {
	if err := l.guard(); err != nil {
		return nil, fmt.Errorf("commsloop: %w", err)
	}
	items, err := readMonitor(l.Mon)
	if err != nil {
		return nil, err
	}
	out := make([]loopengine.Item, 0, len(items))
	for _, it := range items {
		li, err := toLoopItem(it)
		if err != nil {
			return nil, fmt.Errorf("commsloop: cannot carry accepted message %s as a drain item: %w", it.Envelope.ID, err)
		}
		out = append(out, li)
	}
	return out, nil
}

func toLoopItem(it commsqueue.AcceptedItem) (loopengine.Item, error) {
	raw, err := json.Marshal(it.Envelope)
	if err != nil {
		return loopengine.Item{}, err
	}
	return loopengine.Item{
		ID:     "commsmsg/" + it.Envelope.ID,
		Gate:   "model",
		Effort: "S",
		Payload: map[string]string{
			envelopeItemKey: string(raw),
			"from":          it.Envelope.From.Cell + "/" + it.Envelope.From.Role,
			"to":            it.Envelope.To.Cell + "/" + it.Envelope.To.Role,
			"verb":          it.Envelope.Verb,
		},
	}, nil
}

func envelopeFromItem(item loopengine.Item) (*comms.Envelope, error) {
	raw, ok := item.Payload[envelopeItemKey]
	if !ok || raw == "" {
		return nil, fmt.Errorf("commsloop: item %s carries no envelope payload", item.ID)
	}
	var env comms.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, fmt.Errorf("commsloop: item %s envelope payload does not parse: %w", item.ID, err)
	}
	return &env, nil
}

func (l *Loop) setReason(itemID, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.reasons == nil {
		l.reasons = make(map[string]string)
	}
	l.reasons[itemID] = reason
}

func (l *Loop) takeReason(itemID string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.reasons[itemID]
	delete(l.reasons, itemID)
	return r
}

// TierPolicy implements loopengine.Loop. It answers exactly one mechanical
// question (see the file doc); everything JUDGMENT-shaped is never computed
// here — see routing.go's isReportClass doc for the exact boundary this
// brief draws and why.
func (l *Loop) TierPolicy(item loopengine.Item) (loopengine.Tier, error) {
	env, err := envelopeFromItem(item)
	if err != nil {
		l.setReason(item.ID, err.Error())
		return loopengine.TierHuman, nil
	}

	// DEFENSE IN DEPTH — see routing.go. A message that somehow bypassed
	// commsgw's own ACL stage is still caught HERE, independently.
	if err := checkLaneAtRoutingBoundary(env, l.ACL); err != nil {
		l.setReason(item.ID, fmt.Sprintf("ACL bypass caught at the routing boundary (defense in depth, independent of commsgw's own check): %v", err))
		return loopengine.TierHuman, nil
	}

	if isReportClass(env) {
		return loopengine.TierLocal, nil
	}

	l.setReason(item.ID, "no deterministic routing (#1767 ruling 3) — awaiting the prose router (not yet landed); quarantined until it lands")
	return loopengine.TierHuman, nil
}

// staticHandle is an already-resolved loopengine.Handle: Dispatch never fires
// a real session here ("Dispatch stays INTERIM here (emit + await) —
// inert-by-default"; the native dispatch leg is a follow-up deliverable). Its
// Done channel is pre-loaded, so the engine's await returns immediately.
type staticHandle struct {
	item loopengine.Item
	done chan loopengine.Result
}

func (h staticHandle) Done() <-chan loopengine.Result { return h.done }
func (h staticHandle) Item() loopengine.Item          { return h.item }

// Dispatch implements loopengine.Loop. Only TierLocal (report-class) items
// reach here — TierHuman is never dispatched by the engine itself (its
// contract routes straight to Land via VerdictRouteHuman). A report-class
// message needs no agent at all: it is answered by construction (the class of
// message IS the answer), so Dispatch synthesizes the PASS result directly —
// "report-class messages land with NO session fired" is true here in the
// most literal sense: nothing is ever spawned.
func (l *Loop) Dispatch(item loopengine.Item, _ loopengine.Tier) (loopengine.Handle, error) {
	ch := make(chan loopengine.Result, 1)
	ch <- loopengine.Result{Item: item, Verdict: loopengine.VerdictPass, RunnerID: "commsloop"}
	close(ch)
	return staticHandle{item: item, done: ch}, nil
}

// Land implements loopengine.Loop: exactly ONE tracked exit per accepted
// message — landed (done + journal line) or quarantined (held + filed issue)
// — and the accepted-queue entry is retired either way, so a message is never
// left both "accepted" and "done"/"held" at once.
func (l *Loop) Land(result loopengine.Result) error {
	env, err := envelopeFromItem(result.Item)
	if err != nil {
		return err
	}
	now := l.clock()

	switch result.Verdict {
	case loopengine.VerdictRouteHuman:
		reason := l.takeReason(result.Item.ID)
		if reason == "" {
			reason = "quarantined (no reason recorded — see TierPolicy)"
		}
		if err := commsqueue.Quarantine(l.Root, *env, reason, now, l.Filer); err != nil &&
			!isExpectedQuarantineErr(err) {
			return err
		}
		return commsqueue.RemoveAccepted(l.Root, env.ID)

	case loopengine.VerdictPass:
		if err := commsqueue.AppendJournal(l.Root,
			fmt.Sprintf("%s landed id=%s from=%s/%s to=%s/%s verb=%s (report-class, no session fired)",
				now.Format(time.RFC3339), env.ID, env.From.Cell, env.From.Role, env.To.Cell, env.To.Role, env.Verb)); err != nil {
			return err
		}
		return commsqueue.RemoveAccepted(l.Root, env.ID)

	default:
		return fmt.Errorf("commsloop: unexpected verdict %q for message %s", result.Verdict, env.ID)
	}
}

// isExpectedQuarantineErr reports whether err is commsqueue.Quarantine's
// EXPECTED informational return (ErrQuarantined — the message IS held, this
// is not a failure to propagate as a Land error) as opposed to a genuine
// failure (the held-mailbox write itself failed, which commsqueue.Quarantine
// does NOT wrap in ErrQuarantined — see its doc).
func isExpectedQuarantineErr(err error) bool {
	return errors.Is(err, commsqueue.ErrQuarantined)
}

// OnIdle implements loopengine.Loop. There is no separate state to refresh
// between cycles — the accepted-queue itself is re-read on the next
// SelectQueue call, and the kill switch is re-checked there too.
func (l *Loop) OnIdle() error { return nil }
