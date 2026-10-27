package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// desk__claim — TTL leases (design §6.6). A claim requires renewal; expiry is
// visible on the Portfolio pane, so a dead loop's claims release rather than
// squat. The lease is deskd-local state: claiming a work item coordinates the
// fleet, it does not write GitHub, so this tool is NON-MUTATING on the
// platform and authors nothing there — the authority model reads it as a read.

// Lease is a held claim on a work item.
type Lease struct {
	Cell      string    `json:"cell"`
	Item      string    `json:"item"`
	Holder    string    `json:"holder"`
	ClaimedAt time.Time `json:"claimed_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Expired reports whether the lease has lapsed as of now.
func (l Lease) Expired(now time.Time) bool { return !now.Before(l.ExpiresAt) }

// LeaseStore holds work-item leases. It is an interface so the in-memory store
// can be swapped for a persisted one later without touching the tool.
type LeaseStore interface {
	// Claim grants (cell,item) to holder for ttl. It succeeds if the item is
	// free or already held by the SAME holder (a renewal); it returns
	// ok=false with the current holder when someone else holds a live lease.
	Claim(cell, item, holder string, ttl time.Duration) (Lease, bool)
	// Release drops a lease held by holder. Releasing an item held by someone
	// else is a no-op (ok=false) — a loop cannot release another's claim.
	Release(cell, item, holder string) bool
	// Active lists the live (unexpired) leases, for the Portfolio pane.
	Active() []Lease
}

// MemoryLeaseStore is the in-process lease store. Leases live only as long as
// deskd; a restart releases every claim, which is the correct failure mode —
// a claim nobody is renewing must not outlive the process holding it.
type MemoryLeaseStore struct {
	mu     sync.Mutex
	now    func() time.Time
	leases map[string]Lease // key: cell\x1fitem
}

// NewMemoryLeaseStore builds an in-memory store.
func NewMemoryLeaseStore(now func() time.Time) *MemoryLeaseStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &MemoryLeaseStore{now: now, leases: map[string]Lease{}}
}

func leaseKey(cell, item string) string { return cell + "\x1f" + item }

// Claim implements LeaseStore.
func (m *MemoryLeaseStore) Claim(cell, item, holder string, ttl time.Duration) (Lease, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	k := leaseKey(cell, item)
	if cur, ok := m.leases[k]; ok && !cur.Expired(now) && cur.Holder != holder {
		return cur, false // someone else holds a live lease
	}
	l := Lease{Cell: cell, Item: item, Holder: holder, ClaimedAt: now, ExpiresAt: now.Add(ttl)}
	// A renewal keeps the original ClaimedAt so lease age stays honest.
	if cur, ok := m.leases[k]; ok && cur.Holder == holder && !cur.Expired(now) {
		l.ClaimedAt = cur.ClaimedAt
	}
	m.leases[k] = l
	return l, true
}

// Release implements LeaseStore.
func (m *MemoryLeaseStore) Release(cell, item, holder string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := leaseKey(cell, item)
	if cur, ok := m.leases[k]; ok && cur.Holder == holder {
		delete(m.leases, k)
		return true
	}
	return false
}

// Active implements LeaseStore.
func (m *MemoryLeaseStore) Active() []Lease {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	var out []Lease
	for _, l := range m.leases {
		if !l.Expired(now) {
			out = append(out, l)
		}
	}
	return out
}

// ClaimArgs is the desk__claim argument shape.
type ClaimArgs struct {
	Cell    string `json:"cell"`
	Item    string `json:"item"`    // the work item, e.g. "the service contract"
	Release bool   `json:"release"` // when true, release the lease instead of claiming
	// TTLSeconds is the lease duration; a claim MUST be renewed before it
	// expires or it releases on its own (§6.6). Defaults to defaultLeaseTTL.
	TTLSeconds int `json:"ttl_seconds"`
}

// ClaimResult is what desk__claim returns.
type ClaimResult struct {
	Granted bool   `json:"granted"`
	Lease   *Lease `json:"lease,omitempty"`
	Holder  string `json:"current_holder,omitempty"`
	Note    string `json:"note,omitempty"`
}

const defaultLeaseTTL = 15 * time.Minute

func handleClaim(_ context.Context, s *Server, call Call) (any, error) {
	var a ClaimArgs
	if err := json.Unmarshal(nonNull(call.Args), &a); err != nil {
		return nil, fmt.Errorf("desk__claim arguments did not decode: %w", err)
	}
	a.Cell = strings.TrimSpace(a.Cell)
	a.Item = strings.TrimSpace(a.Item)
	if a.Cell == "" || a.Item == "" {
		return nil, fmt.Errorf("desk__claim: cell and item are both required")
	}
	holder := call.CallerLogin
	if holder == "" {
		holder = "anonymous"
	}

	if a.Release {
		ok := s.Leases.Release(a.Cell, a.Item, holder)
		note := "released"
		if !ok {
			note = "no live lease held by you to release"
		}
		return ClaimResult{Granted: ok, Note: note}, nil
	}

	ttl := defaultLeaseTTL
	if a.TTLSeconds > 0 {
		ttl = time.Duration(a.TTLSeconds) * time.Second
	}
	l, ok := s.Leases.Claim(a.Cell, a.Item, holder, ttl)
	if !ok {
		return ClaimResult{Granted: false, Holder: l.Holder,
			Note: "held by another loop until its lease expires — renew-or-release keeps a dead loop from squatting (§6.6)"}, nil
	}
	lease := l
	return ClaimResult{Granted: true, Lease: &lease,
		Note: "TTL lease — renew before expiry or it releases on its own"}, nil
}
