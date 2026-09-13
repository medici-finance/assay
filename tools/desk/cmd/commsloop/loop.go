package main

import (
	"context"
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
// NO DETERMINISTIC ROUTING (#1767 ruling 3). EVERY accepted message that
// clears the routing-boundary ACL re-check (routing.go) is routed by the
// contained prose consult (decide.go) — there is no deterministic routing
// table and no fast path (a report-verb mechanical shortcut lived here until
// the router landed; it is retired). The consult returns one action from a
// closed set; assign.go's compiled table then resolves (action, class, risk)
// to a dispatch Tier. An invalid/timed-out/budget-exhausted/valve-disabled
// consult resolves to the default action (quarantine), so this loop is
// fail-closed by construction whether or not a decider is even configured.
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
	// Router is the inbound prose router (decide.go) consulted for the
	// routing decision on every message that clears the ACL re-check. A nil
	// Router is a valid, fail-closed state: TierPolicy falls back to an
	// ephemeral default (the package Question + shared Budget, no Advisor),
	// which reads as the valve being off — every message quarantines.
	// Production wiring (main.go) always supplies a real one via NewRouter.
	Router *Router
	// Now is a test seam; nil means time.Now (UTC).
	Now func() time.Time

	mu      sync.Mutex
	reasons map[string]string // item ID -> routing rationale, set by TierPolicy, consumed by Land.
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

// router returns l.Router, or an ephemeral fail-closed default when none is
// wired: the package Question and the shared package Budget, no Advisor.
// That default reads exactly like the valve being off (no advisor -> every
// consult resolves to the default action, quarantine) — a Loop built without
// an explicit Router (e.g. an older test fixture) stays fail-closed rather
// than panicking or silently allow-listing.
func (l *Loop) router() *Router {
	if l.Router != nil {
		return l.Router
	}
	return &Router{Question: routerQuestion, Budget: routerBudget}
}

// normalizeClass defaults an absent envelope class to "routine" — the
// least-severe of the two known classes. ParseEnvelope already refuses any
// OTHER unrecognised class at parse time (comms.KnownClass), so an accepted
// message's Class is either "", "routine", or "sensitive"; only the empty
// case needs a default here so Assign() always receives a class it accepts.
func normalizeClass(class string) string {
	if class == "" {
		return "routine"
	}
	return class
}

// TierPolicy implements loopengine.Loop. EVERY accepted message that clears
// the routing-boundary ACL re-check is routed by the contained prose consult
// (decide.go) — #1767 ruling 3, no deterministic routing table, no fast path.
// The consult's action, plus the envelope's class, resolves through
// assign.go's compiled table to a dispatch Tier; a routing rationale is
// recorded either way (setReason) so Land can explain a quarantine or narrate
// a landed message without re-deriving the decision.
func (l *Loop) TierPolicy(item loopengine.Item) (loopengine.Tier, error) {
	env, err := envelopeFromItem(item)
	if err != nil {
		l.setReason(item.ID, err.Error())
		return loopengine.TierHuman, nil
	}

	// DEFENSE IN DEPTH — see routing.go. A message that somehow bypassed
	// commsgw's own ACL stage is still caught HERE, independently, BEFORE it
	// ever reaches the prose consult.
	if err := checkLaneAtRoutingBoundary(env, l.ACL); err != nil {
		l.setReason(item.ID, fmt.Sprintf("ACL bypass caught at the routing boundary (defense in depth, independent of commsgw's own check): %v", err))
		return loopengine.TierHuman, nil
	}

	action := l.router().Route(context.Background(), env)
	class := normalizeClass(env.Class)
	// risk: no per-message mechanical risk signal is carried on the envelope
	// yet (assign.yaml documents risk as "carried on the envelope as an INPUT
	// to model resolution", a follow-up wiring gap distinct from this
	// brief's contract — NEEDS_CONTEXT if that gap needs closing here).
	// Passing false is the conservative, in-scope default: it never grants
	// MORE autonomy than a risk-flagged message would (assign.yaml's own
	// risk:yes-forces-human rule only ever narrows), and the router's own
	// action choice is expected to carry anything risk-shaped to
	// escalate-human-issue/quarantine until a mechanical risk field lands.
	const risk = false
	tier, err := Assign(action, class, risk)
	if err != nil {
		l.setReason(item.ID, fmt.Sprintf("router chose action %q (class=%s) but assignment refused: %v — quarantined (fail closed)", action, class, err))
		return loopengine.TierHuman, nil
	}
	l.setReason(item.ID, fmt.Sprintf("router action=%s class=%s tier=%s", action, class, tierName(tier)))
	return tier, nil
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

// Dispatch implements loopengine.Loop. TierHuman is never dispatched by the
// engine itself (its contract routes straight to Land via
// VerdictRouteHuman); every OTHER tier the router's action can resolve to
// (TierLocal/TierCheap/TierSession) reaches here. Firing a real worker
// session for a dispatch-class action (route-work-dispatch/route-review/
// route-verify) is the executor dispatch leg's job (a separate, inert-until-
// cutover brief — #1767 ruling 2), so Dispatch stays INTERIM here (emit +
// await) for every tier: it synthesizes the PASS result directly rather than
// spawning anything. "No session fired" is therefore true for every message
// that lands this way today, not only report-shaped ones.
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
		reason := l.takeReason(result.Item.ID)
		if reason == "" {
			reason = "no routing rationale recorded — see TierPolicy"
		}
		if err := commsqueue.AppendJournal(l.Root,
			fmt.Sprintf("%s landed id=%s from=%s/%s to=%s/%s verb=%s (%s, no session fired)",
				now.Format(time.RFC3339), env.ID, env.From.Cell, env.From.Role, env.To.Cell, env.To.Role, env.Verb, reason)); err != nil {
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
