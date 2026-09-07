package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
)

// monitor.go — the accepted-queue's read surface ("copy scanloop's adapter
// shape... Monitor as queue source (here: the gateway's accepted-queue);
// nil/unreachable Monitor = refuse, never 'empty queue'"). commsgw (a
// SEPARATE process/binary) is the writer; this consumer only ever reads.

// Monitor is the accepted-queue's read surface. A nil Monitor, or a Monitor
// whose Read fails, is UNREACHABLE and must be REFUSED by the caller — never
// silently treated as "the queue is empty". An empty (but successfully read)
// queue and an unreachable one are different conditions with different
// operator responses, and collapsing them is exactly the blindness this rule
// exists to prevent (cf. cmd/scanloop's monitor.go, which wraps rather than
// re-derives the same distinction for its own, unrelated inbound surface).
type Monitor interface {
	Read() ([]commsqueue.AcceptedItem, error)
}

// DirMonitor reads the accepted-queue directly off the gateway's on-disk
// queue root.
type DirMonitor struct {
	Root string
}

// Read implements Monitor.
func (m DirMonitor) Read() ([]commsqueue.AcceptedItem, error) {
	if strings.TrimSpace(m.Root) == "" {
		return nil, fmt.Errorf("commsloop: monitor has no queue root configured — unreachable, not empty")
	}
	items, err := commsqueue.ListAccepted(m.Root)
	if err != nil {
		return nil, fmt.Errorf("commsloop: accepted-queue at %s is unreachable: %w", m.Root, err)
	}
	return items, nil
}

// readMonitor is the ONE call site every SelectQueue path uses, so "nil
// Monitor = refuse" cannot be forgotten at a second call site later.
func readMonitor(m Monitor) ([]commsqueue.AcceptedItem, error) {
	if m == nil {
		return nil, fmt.Errorf("commsloop: nil monitor — refused, never treated as an empty queue")
	}
	return m.Read()
}
