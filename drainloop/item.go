// Package drainloop is a framework-agnostic drain engine: read a queue, claim an item, dispatch
// it and await its result, land it, release the claim, and move to the next — one item in flight
// at a time — idling when the queue empties. The scheduler lives here, in deterministic code, so
// it never has to live in an operator model's attention.
//
// It is importable and cloneable. The adapters shipped alongside it (an in-memory queue, a
// file-based claim, an echoing dispatcher) are stand-ins so the harness runs with nothing
// attached. Replace them with your own to drain real work in your own stack. Nothing in this
// package knows what an Item contains or where a worker runs, and nothing here couples to any
// particular infrastructure — the public core is deliberately deskkit-free (asserted by
// TestNoDeskkitImports).
//
// The contract is the six-method Loop interface in engine.go, plus a small Claimer interface
// for the dedupe chokepoint. Everything role-specific is confined to TierPolicy and Land; the
// pool arithmetic is the engine's, written once. A companion article one directory up
// (../drain-engine.md) explains why agent loops stall and how moving the scheduler out of the
// model fixes it.
package drainloop

// Item is one typed unit of drain work. Keep it typed: everything the engine reasons about
// (identity, who authored it, whatever your policy needs) should be a field, not free prose
// the scheduler has to interpret.
type Item struct {
	// ID is a stable, unique key for the item. It is the claim key, so two items with the
	// same ID are the same unit of work and must never be dispatched concurrently.
	ID string
	// Implementer names who authored or produced this item, when that matters. It powers the
	// opt-in author-not-runner structural guard (CheckAuthorRunner): an item must not be
	// dispatched to a runner that equals its own Implementer. Empty ⇒ the guard is inert for
	// this item.
	Implementer string
	// Payload carries whatever your Dispatch adapter needs to act on the item. The engine
	// never reads it.
	Payload map[string]string
	// Retry is the number of attempts already made against this item, an input to the opt-in
	// RetryPolicy. Zero means fresh. A queue that re-selects a failed item bumps it.
	Retry int
}

// Tier is a dispatch DESTINATION, not a worker name. An adapter maps a Tier onto a concrete
// runner class. TierHuman is not dispatchable at all: the engine routes a human-tiered item
// straight to Land (record it, do not run it) and the drain continues.
type Tier int

const (
	// TierLocal is the cheapest dispatchable destination (e.g. an in-process runner).
	TierLocal Tier = iota
	// TierCheap is a cheap external runner class.
	TierCheap
	// TierSession is a full session runner class.
	TierSession
	// TierHuman means "do not dispatch": route to Land as held and keep draining. Use it for
	// items that must go to a human or another system rather than a worker.
	TierHuman
)

// String renders a Tier for logs.
func (t Tier) String() string {
	switch t {
	case TierLocal:
		return "local"
	case TierCheap:
		return "cheap"
	case TierSession:
		return "session"
	case TierHuman:
		return "human"
	default:
		return "tier?"
	}
}

// Verdict is the structured outcome class of one landed item. It replaces a bare ok/not-ok
// boolean so the record distinguishes a clean pass, a clean failure, a held (never
// dispatched) item, and a dispatch that could not run at all.
type Verdict int

const (
	// VerdictUnknown is the zero value — an outcome that was never set. Treat it as
	// could-not-check, never as a pass.
	VerdictUnknown Verdict = iota
	// VerdictPass — the dispatched work succeeded.
	VerdictPass
	// VerdictFail — the dispatched work ran and failed cleanly.
	VerdictFail
	// VerdictHold — the item was routed to Land without dispatch (TierHuman, or a structural
	// guard).
	VerdictHold
	// VerdictError — the dispatch itself could not run (as opposed to running and failing).
	VerdictError
)

// String renders a Verdict for logs and evidence.
func (v Verdict) String() string {
	switch v {
	case VerdictPass:
		return "pass"
	case VerdictFail:
		return "fail"
	case VerdictHold:
		return "hold"
	case VerdictError:
		return "error"
	default:
		return "unknown"
	}
}

// EvidenceRow is one structured line of the record derived from a dispatched run — the
// command that ran, its exit code, a key output line. Prefer these structured fields over a
// prose blob: the point of the engine is that the record is derived from the run, not
// narrated afterward.
type EvidenceRow struct {
	// Name identifies what this row records (a check name, a phase). Free-form.
	Name string
	// Command is what was run, if anything.
	Command string
	// Exit is the command's exit code.
	Exit int
	// Output is the key output line(s), not a whole transcript.
	Output string
}

// Result is the structured outcome of one item, handed back to Land. The Item is folded in
// (Result.Item) so one structured value crosses the seam. Rows carry the per-check evidence,
// RunnerID records which runner produced it, and Artifact points at any durable artifact.
type Result struct {
	// Item is the item this result is for. The engine folds it in before Land is called, so
	// Land receives one value.
	Item Item
	// Verdict is the outcome class.
	Verdict Verdict
	// Rows is the evidence derived from the run, one structured row per check.
	Rows []EvidenceRow
	// RunnerID names the runner that produced this result, when known.
	RunnerID string
	// Artifact is an optional pointer (URL/path) to a durable artifact of the run.
	Artifact string
}

// failed reports whether a result is a run-level failure the retry taxonomy may act on. A
// held item is not a failure; an unknown verdict is not treated as a failure to retry.
func (r Result) failed() bool {
	return r.Verdict == VerdictFail || r.Verdict == VerdictError
}

// Handle is returned by Dispatch and awaited by the engine. It is an interface so the same
// engine loop drives synchronous dispatch (an already-resolved handle) and asynchronous
// dispatch (a handle backed by a real child process) without changing — that seam is the
// whole point. Done delivers exactly one Result; Item reports which item the handle is for.
type Handle interface {
	// Done returns a channel that delivers the single Result when the dispatched work
	// finishes. The engine reads it exactly once.
	Done() <-chan Result
	// Item reports the item this handle is dispatching.
	Item() Item
}

// resolvedHandle is a Handle whose Result is already known — the synchronous case. Its Done
// channel is pre-loaded, so the engine's <-h.Done() returns immediately.
type resolvedHandle struct {
	item Item
	ch   chan Result
}

func (h resolvedHandle) Done() <-chan Result { return h.ch }
func (h resolvedHandle) Item() Item          { return h.item }

// Resolved builds an already-complete Handle from an item and its Result. Adapters that
// dispatch synchronously use it; an async adapter would return a Handle backed by a live
// channel instead. Resolved folds the item into Result.Item so callers need not repeat it.
func Resolved(it Item, r Result) Handle {
	r.Item = it
	ch := make(chan Result, 1)
	ch <- r
	return resolvedHandle{item: it, ch: ch}
}
