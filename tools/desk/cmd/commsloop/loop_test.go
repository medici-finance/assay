package main

import (
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/commsqueue"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// fakeFiler records every quarantine filing without shelling out to the real
// `deskfile` CLI.
type fakeFiler struct {
	calls   int
	reasons []string
}

func (f *fakeFiler) File(env comms.Envelope, reason string) error {
	f.calls++
	f.reasons = append(f.reasons, reason)
	return nil
}

func newTestLoop(t *testing.T) (*Loop, string, *fakeFiler) {
	t.Helper()
	root := t.TempDir()
	acl := comms.Compiled()
	filer := &fakeFiler{}
	loop := &Loop{
		Root:    root,
		Mon:     DirMonitor{Root: root},
		ACL:     &acl,
		Filer:   filer,
		GuardFn: func() error { return nil },
		Now:     func() time.Time { return time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC) },
	}
	return loop, root, filer
}

func plantAccepted(t *testing.T, root string, env comms.Envelope) {
	t.Helper()
	if err := commsqueue.WriteAccepted(root, env, time.Now().UTC()); err != nil {
		t.Fatalf("plant accepted %s: %v", env.ID, err)
	}
}

func drainOnce(t *testing.T, loop *Loop) {
	t.Helper()
	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	for _, item := range items {
		tier, err := loop.TierPolicy(item)
		if err != nil {
			t.Fatalf("TierPolicy(%s): %v", item.ID, err)
		}
		var result loopengine.Result
		if tier == loopengine.TierHuman {
			// The engine itself never calls Dispatch for TierHuman — it
			// synthesizes VerdictRouteHuman and lands directly. Mirror that
			// here rather than driving the real (shared, already-tested)
			// engine.Run loop.
			result = loopengine.Result{Item: item, Verdict: loopengine.VerdictRouteHuman}
		} else {
			handle, err := loop.Dispatch(item, tier)
			if err != nil {
				t.Fatalf("Dispatch(%s): %v", item.ID, err)
			}
			result = <-handle.Done()
		}
		if err := loop.Land(result); err != nil {
			t.Fatalf("Land(%s): %v", item.ID, err)
		}
	}
}

// --- Verify row 3: Bypass ----------------------------------------------------

func TestBypass(t *testing.T) {
	loop, root, filer := newTestLoop(t)

	// Plant a message DIRECTLY into the accepted-queue — skipping commsgw's
	// own precheck pipeline entirely (a real bypass: commsgw's checkLane
	// would ALSO have refused this, had it ever seen it). The verb ("status")
	// is deliberately chosen to be a REPORT-CLASS verb by isReportClass's own
	// mechanical rule (routing.go) — a message misrouted this way would be
	// mistakenly landed done (no session, no ACL re-check) rather than held,
	// if the routing-boundary ACL check were ever removed or bypassed. This
	// is the meaningful case: verb+lane legality, checked INDEPENDENTLY of
	// (and BEFORE, in TierPolicy) the report-class mechanical rule, is what
	// this test actually pins — a bypassed message that also happened to be
	// non-report-class would quarantine anyway via the "awaiting the prose
	// router" fallback, proving nothing about THIS check.
	bad := comms.Envelope{
		Schema: comms.Schema, ID: "bypass-1", Cell: "cell-a",
		From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "worker-desk"},
		Verb: "status", // NOT a within_cell verb (laneacl.yaml) — cross-cell only
	}
	plantAccepted(t, root, bad)

	drainOnce(t, loop)

	// It must NOT be in the accepted-queue any more (landed, one way or the
	// other) and must be HELD (quarantined) — never silently routed.
	accepted, err := commsqueue.ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(accepted) != 0 {
		t.Fatalf("bypassed message must not remain in the accepted-queue: %v", accepted)
	}
	held, err := commsqueue.ListHeld(root)
	if err != nil {
		t.Fatalf("ListHeld: %v", err)
	}
	if len(held) != 1 || held[0].Envelope.ID != "bypass-1" {
		t.Fatalf("bypassed message must be held (routing-boundary refusal caught it), held=%v", held)
	}
	if filer.calls != 1 {
		t.Fatalf("quarantine issue filing calls = %d, want 1", filer.calls)
	}
}

// --- Verify row 4: Quarantine ------------------------------------------------

func TestQuarantine(t *testing.T) {
	loop, root, filer := newTestLoop(t)

	// An ACL-legal but NOT-YET-ROUTABLE message: a within-cell "ask" (needs
	// the prose router, which has not landed) must quarantine, not be
	// dropped.
	env := comms.Envelope{
		Schema: comms.Schema, ID: "unroutable-1", Cell: "cell-a",
		From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "worker-desk"},
		Verb: "ask",
	}
	plantAccepted(t, root, env)

	drainOnce(t, loop)

	accepted, err := commsqueue.ListAccepted(root)
	if err != nil {
		t.Fatalf("ListAccepted: %v", err)
	}
	if len(accepted) != 0 {
		t.Fatalf("landed message must leave the accepted-queue: %v", accepted)
	}
	held, err := commsqueue.ListHeld(root)
	if err != nil {
		t.Fatalf("ListHeld: %v", err)
	}
	if len(held) != 1 || held[0].Envelope.ID != "unroutable-1" {
		t.Fatalf("unroutable message must be held, never dropped: held=%v", held)
	}
	if filer.calls != 1 {
		t.Fatalf("quarantine issue filing calls = %d, want 1", filer.calls)
	}

	// A REPORT-class message, by contrast, lands done — not held.
	report := comms.Envelope{
		Schema: comms.Schema, ID: "report-1", Cell: "cell-a",
		From: comms.SenderID{Cell: "cell-b", Role: "the-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "the-desk"},
		Verb: "status",
	}
	plantAccepted(t, root, report)
	drainOnce(t, loop)
	held2, err := commsqueue.ListHeld(root)
	if err != nil {
		t.Fatalf("ListHeld: %v", err)
	}
	for _, h := range held2 {
		if h.Envelope.ID == "report-1" {
			t.Fatalf("a report-class message must land done, not quarantine")
		}
	}
}

// --- Verify row 6: Stop ------------------------------------------------------

func TestStop(t *testing.T) {
	loop, root, _ := newTestLoop(t)
	plantAccepted(t, root, comms.Envelope{
		Schema: comms.Schema, ID: "stop-1", Cell: "cell-a",
		From: comms.SenderID{Cell: "cell-a", Role: "the-desk"},
		To:   comms.Lane{Cell: "cell-a", Role: "worker-desk"},
		Verb: "handoff",
	})

	// First cycle: no stop armed, the item is selected.
	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue (no stop): %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item before STOP, got %d", len(items))
	}

	// Arm STOP mid-drain (simulating deskkit.Guard tripping between cycles —
	// a real armed STOP file, or DISABLED, or a per-run stop; the fake
	// reproduces exactly the refusal Guard would return without touching
	// ~/.config/assay).
	stopErr := errors.New("STOP flag armed (simulated)")
	loop.GuardFn = func() error { return stopErr }

	_, err = loop.SelectQueue()
	if err == nil {
		t.Fatalf("SelectQueue must refuse once the kill switch is armed — the drain must halt mid-drain, not silently continue")
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("SelectQueue's refusal must carry the guard's own error, got %v", err)
	}

	// The queued-but-not-yet-landed item must still be sitting in the
	// accepted-queue — halting mid-drain must never lose or half-process it.
	accepted, lerr := commsqueue.ListAccepted(root)
	if lerr != nil {
		t.Fatalf("ListAccepted: %v", lerr)
	}
	if len(accepted) != 1 || accepted[0].Envelope.ID != "stop-1" {
		t.Fatalf("halted item must remain in the accepted-queue untouched: %v", accepted)
	}
}
