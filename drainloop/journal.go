package drainloop

import "time"

// Event is one scheduling event, for journalling and replay. It is deskkit-free: the public
// core emits Events; a house sink maps them onto its audit schema, and a replay reads them
// back to reconstruct which items were in flight after a crash. The Kind values the engine
// emits are CLAIM, DISPATCH, LAND, RELEASE (via LAND), SKIP, HOLD, GUARD, EVIDENCE, IDLE, EFFECT-INTENT, EFFECT-DONE and RECONCILE.
type Event struct {
	// Kind is the scheduling decision this event records.
	Kind string
	// Effect identity is structured, never reconstructed from Detail. Generation is
	// the coordination store's claim generation for transactional handoff events.
	RunID      string
	AttemptID  string
	Key        string
	Ref        string
	Generation string
	// ItemID is the item the event is about, empty for loop-level events (IDLE).
	ItemID string
	// Detail is a short free-form note (a verdict, a skip reason).
	Detail string
	// Time is when the event was recorded.
	Time time.Time
}

// Journal is the deskkit-free sink the engine writes scheduling Events to, when one is
// configured (Config.Journal). The default is nil (no journalling). A house implementation
// writes through its audit schema; a test implementation appends to a slice. Record should be
// durable before returning success for EFFECT-INTENT: failure aborts that effect.
// All other kinds remain best-effort; a lost EFFECT-DONE is repaired by authoritative
// reconciliation, never blind replay. TransactionalJournal replaces the intent write
// with an atomic intent/claim-generation commit. Ordinary scheduling stays best-effort.
type Journal interface {
	Record(Event) error
}

// Timeouts is the deskkit-free liveness taxonomy: the three timers a real drain uses to
// detect a stuck dispatch. The public core carries the type and the taxonomy; ENFORCEMENT
// (reclaiming a claim whose worker went silent, reading a runner's heartbeat) is house-side,
// because it reads live infrastructure. A zero Timeouts disables the corresponding timer.
type Timeouts struct {
	// ScheduleToStart bounds queued → started: how long an item may sit claimed before its
	// worker reports having begun. Exceeded ⇒ the dispatch never took; reclaim the item.
	ScheduleToStart time.Duration
	// StartToClose bounds started → landed: the maximum a single dispatched item may run.
	// Exceeded ⇒ the worker is stuck; reclaim and route to a human.
	StartToClose time.Duration
	// Heartbeat is the maximum gap between a running worker's heartbeats. Exceeded ⇒ the
	// worker is presumed dead even if StartToClose has not elapsed.
	Heartbeat time.Duration
}

// Exceeded reports whether elapsed time since a phase's start breaches the given timeout. A
// zero timeout is "no limit" and never breaches. It is a pure helper a house liveness layer
// uses when it checks a claim's age against ScheduleToStart/StartToClose/Heartbeat.
func Exceeded(timeout, elapsed time.Duration) bool {
	return timeout > 0 && elapsed > timeout
}
