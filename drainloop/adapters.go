package drainloop

import "fmt"

// MemoryQueue is a stand-in Loop: a fixed slice of items and a record of what has landed, so
// SelectQueue stops returning an item once it has been landed. Replace it with a read against
// your real backlog (a tracker, a table, a board). Its Dispatch is the EchoDispatcher
// stand-in — it "runs" the item by echoing it and reports success.
type MemoryQueue struct {
	name    string
	pending []Item
	landed  map[string]Result
}

// NewMemoryQueue seeds a queue with items to drain.
func NewMemoryQueue(name string, items []Item) *MemoryQueue {
	return &MemoryQueue{name: name, pending: items, landed: map[string]Result{}}
}

// Name identifies the loop.
func (q *MemoryQueue) Name() string { return q.name }

// SelectQueue returns the items not yet landed. When it returns empty, the drain idles.
func (q *MemoryQueue) SelectQueue() ([]Item, error) {
	var out []Item
	for _, it := range q.pending {
		if _, done := q.landed[it.ID]; !done {
			out = append(out, it)
		}
	}
	return out, nil
}

// TierPolicy sends everything to the cheapest dispatchable destination. Override to route
// some items to a human with TierHuman, or to different runner classes. This is a policy hook
// by design. The error return lets a real board read report could-not-check.
func (q *MemoryQueue) TierPolicy(Item) (Tier, error) { return TierLocal, nil }

// Dispatch is the EchoDispatcher stand-in: it "runs" the item by echoing it and reports a
// pass. Swap this one method for a call to your real worker (a subprocess, an HTTP call, an
// agent spawn). The engine loop does not change when you do. The resolved Tier is available
// for runner-class routing.
func (q *MemoryQueue) Dispatch(it Item, tier Tier) (Handle, error) {
	cmd := fmt.Sprintf("echo drain %s", it.ID)
	return Resolved(it, Result{
		Verdict:  VerdictPass,
		RunnerID: "echo",
		Rows: []EvidenceRow{{
			Name:    "drain",
			Command: cmd,
			Exit:    0,
			Output:  fmt.Sprintf("drained %s payload=%v tier=%s", it.ID, it.Payload, tier),
		}},
	}), nil
}

// Land records the result so the item stops being selected. A real Land would write the
// evidence somewhere durable and derive it from the Result's structured Rows.
func (q *MemoryQueue) Land(r Result) error {
	q.landed[r.Item.ID] = r
	return nil
}

// OnIdle refreshes nothing in this stand-in; the drain's stop policy is set by
// Config.StopWhenIdle (a batch demo sets it true to stop when the queue empties). A
// long-running desk would refresh its board here and let the engine poll again.
func (q *MemoryQueue) OnIdle() error { return nil }

// Landed reports the recorded result for an item, for tests and demos.
func (q *MemoryQueue) Landed(id string) (Result, bool) {
	r, ok := q.landed[id]
	return r, ok
}
