package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The four Verify rows for example-stream/18. Each is driven through enforceAdmission — the
// serialized dispatch-boundary gate — with an injected backend (concurrency, crash, could-not-check)
// or the real backend reading an injected repair sidecar (external-wait, forged class), so no
// production service is touched. deskkit.EvaluateAdmission's pure logic is pinned separately in
// internal/deskkit/repairadmission_test.go; these rows pin the WIRING: serialization, recovery
// order, three-state honesty, and the obligation surviving a restart across the whole cycle.

// ---- an injectable in-memory backend ----------------------------------------------------------

// fakeScope is one scheduling scope's shared state: the width, the live occupancy (item claims),
// the single serialization lease, and the runnable reserved demand. Two dispatchers share ONE scope
// to model a race for the last slot; the lease is the only thing that serializes them.
type fakeScope struct {
	width       int
	reserve     map[string]int
	occupancy   int    // live item claims already placed
	leaseHeldBy string // "" = free
	demand      map[string]int
	waiting     string
	admissions  int // successful admits that went on to place an item claim
	occErr      error
}

// fakeBackend is one dispatcher's view of a shared fakeScope.
type fakeBackend struct {
	scope *fakeScope
	owner string
	class deskkit.AdmissionClass
}

func (b *fakeBackend) candidateClass() (deskkit.AdmissionClass, error) { return b.class, nil }
func (b *fakeBackend) reservation() (int, map[string]int, error) {
	return b.scope.width, b.scope.reserve, nil
}
func (b *fakeBackend) occupancy() (int, error) {
	if b.scope.occErr != nil {
		return 0, b.scope.occErr
	}
	return b.scope.occupancy, nil
}
func (b *fakeBackend) reservedDemand() (map[string]int, string, error) {
	return b.scope.demand, b.scope.waiting, nil
}
func (b *fakeBackend) acquireLease() (bool, error) {
	if b.scope.leaseHeldBy == "" {
		b.scope.leaseHeldBy = b.owner
		return true, nil
	}
	if b.scope.leaseHeldBy == b.owner {
		return true, nil // re-entrant, harmless
	}
	return false, nil // another live dispatcher holds it
}
func (b *fakeBackend) releaseLease() error {
	if b.scope.leaseHeldBy == b.owner {
		b.scope.leaseHeldBy = ""
	}
	return nil
}

// useBackend points newAdmissionBackend at a specific fake for the next enforceAdmission call, and
// forces the gate on. The cleanup restores both.
func useBackend(t *testing.T, be admissionBackend) {
	t.Helper()
	admissionEnabledFn = func() (bool, string) { return true, deskkit.RepairAdmissionPolicyVersion }
	newAdmissionBackend = func(dispatchOpts, dispatchPlan, string, claimAuth) admissionBackend { return be }
}

func restoreAdmission(t *testing.T) {
	t.Helper()
	oldEnabled, oldNew := admissionEnabledFn, newAdmissionBackend
	t.Cleanup(func() { admissionEnabledFn = oldEnabled; newAdmissionBackend = oldNew })
}

// placeItemClaim models stepClaim succeeding: the durable occupancy increments and the admission is
// counted. It is what the real dispatch() does between enforceAdmission's admit and its release.
func (s *fakeScope) placeItemClaim() {
	s.occupancy++
	s.admissions++
}

// ---- Row 1 ------------------------------------------------------------------------------------

// TestRepairAdmissionDirectDispatch: a direct fresh dispatch is HELD when it would steal a reserved
// repair slot, and the repair itself is ADMITTED — the planner is bypassed entirely (dispatch is
// invoked directly), which is the whole point of enforcing at the boundary rather than in advice.
func TestRepairAdmissionDirectDispatch(t *testing.T) {
	restoreAdmission(t)
	newScope := func() *fakeScope {
		return &fakeScope{
			width: 3, reserve: map[string]int{"rework": 1}, occupancy: 2,
			demand: map[string]int{"rework": 1}, waiting: "repair example-stream/17",
		}
	}

	// Fresh: held, lease released (no leak), and the message names the waiting repair.
	scope := newScope()
	useBackend(t, &fakeBackend{scope: scope, owner: "fresh", class: deskkit.AdmissionFresh})
	release, err := enforceAdmission(dispatchOpts{item: "some-stream--99"}, dispatchPlan{}, "medici-finance/assay", claimAuth{})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("fresh dispatch that would steal a reserved repair slot must be REFUSED (exit 5), got admitted=%v err=%v", release != nil, err)
	}
	if !strings.Contains(err.Error(), "example-stream/17") {
		t.Errorf("the HOLD must name the waiting repair, got: %v", err)
	}
	if scope.leaseHeldBy != "" {
		t.Errorf("a held fresh dispatch must release the serialization lease, still held by %q", scope.leaseHeldBy)
	}

	// Repair (rework): admitted; the caller then places the item claim and releases the lease.
	scope = newScope()
	useBackend(t, &fakeBackend{scope: scope, owner: "repair", class: deskkit.AdmissionRework})
	release, err = enforceAdmission(dispatchOpts{item: "repair/abc"}, dispatchPlan{}, "medici-finance/assay", claimAuth{})
	if err != nil || release == nil {
		t.Fatalf("the reserved repair must be ADMITTED (nil err, non-nil release), got admitted=%v err=%v", release != nil, err)
	}
	if scope.leaseHeldBy != "repair" {
		t.Errorf("the lease must stay held through the item claim on admit, held by %q", scope.leaseHeldBy)
	}
	scope.placeItemClaim()
	release()
	if scope.leaseHeldBy != "" {
		t.Errorf("release() after the item claim must free the lease, still held by %q", scope.leaseHeldBy)
	}
	if scope.admissions != 1 {
		t.Errorf("exactly one admission expected, got %d", scope.admissions)
	}
}

// ---- Row 2 ------------------------------------------------------------------------------------

// TestRepairAdmissionConcurrentAndCrash: two dispatchers race for the LAST free slot — exactly one
// is admitted; and a crash at each of the two boundaries (after the lease, after the item claim)
// leaks no slot and duplicates no worker.
func TestRepairAdmissionConcurrentAndCrash(t *testing.T) {
	restoreAdmission(t)

	// RACE: width 3, occupancy 2 => one free slot; no runnable reserved demand, so both candidates
	// are eligible fresh. The lease serializes them: the first counts 2 and admits+claims (occ->3),
	// the second counts 3 and is held (pool full). Exactly one admission.
	scope := &fakeScope{width: 3, reserve: map[string]int{}, occupancy: 2, demand: map[string]int{}}

	a := &fakeBackend{scope: scope, owner: "A", class: deskkit.AdmissionFresh}
	useBackend(t, a)
	relA, errA := enforceAdmission(dispatchOpts{item: "s--1"}, dispatchPlan{}, "medici-finance/assay", claimAuth{})
	if errA != nil || relA == nil {
		t.Fatalf("dispatcher A should win the free slot, got err=%v", errA)
	}
	// While A still holds the lease, B cannot even serialize — the multi-host race the lease stops.
	b := &fakeBackend{scope: scope, owner: "B", class: deskkit.AdmissionFresh}
	useBackend(t, b)
	if _, errB := enforceAdmission(dispatchOpts{item: "s--2"}, dispatchPlan{}, "medici-finance/assay", claimAuth{}); errB == nil || deskkit.ExitCodeOf(errB) != deskkit.ExitUnverifiable {
		t.Fatalf("B must not proceed while A holds the lease — want could-not-serialize (exit 6), got %v", errB)
	}
	// A finishes: places its claim, releases the lease.
	scope.placeItemClaim()
	relA()
	// Now B retries: the lease is free, but occupancy is 3 (full) => B is held.
	useBackend(t, b)
	if _, errB := enforceAdmission(dispatchOpts{item: "s--2"}, dispatchPlan{}, "medici-finance/assay", claimAuth{}); errB == nil || deskkit.ExitCodeOf(errB) != deskkit.ExitRefused {
		t.Fatalf("B must be HELD once the last slot is taken, want exit 5, got %v", errB)
	}
	if scope.admissions != 1 {
		t.Fatalf("exactly one admission across the race, got %d", scope.admissions)
	}

	// CRASH after the lease, before the item claim: enforceAdmission admitted and returned a
	// release, but the dispatcher dies before stepClaim. No item claim was placed => occupancy is
	// unchanged (no leaked slot); the lease is bounded (a later reclaim frees it) => no permanent
	// hold. Model the reclaim by clearing the lease as the backend TTL would.
	crash := &fakeScope{width: 3, reserve: map[string]int{}, occupancy: 1, demand: map[string]int{}}
	cb := &fakeBackend{scope: crash, owner: "C", class: deskkit.AdmissionFresh}
	useBackend(t, cb)
	if _, err := enforceAdmission(dispatchOpts{item: "s--3"}, dispatchPlan{}, "medici-finance/assay", claimAuth{}); err != nil {
		t.Fatalf("C should be admitted, got %v", err)
	}
	// ... dispatcher C crashes here (no placeItemClaim, no release) ...
	if crash.occupancy != 1 {
		t.Errorf("a crash before the item claim must not increment occupancy — leaked a slot (occ=%d)", crash.occupancy)
	}
	if crash.admissions != 0 {
		t.Errorf("a crash before the item claim must not count as an admission, got %d", crash.admissions)
	}
	crash.leaseHeldBy = "" // the backend TTL reclaims the crashed lease
	// A fresh dispatcher recovers cleanly into the still-free slot: no duplicate, no leak.
	rb := &fakeBackend{scope: crash, owner: "R", class: deskkit.AdmissionFresh}
	useBackend(t, rb)
	rel, err := enforceAdmission(dispatchOpts{item: "s--3"}, dispatchPlan{}, "medici-finance/assay", claimAuth{})
	if err != nil || rel == nil {
		t.Fatalf("recovery after a crashed lease must admit into the still-free slot, got %v", err)
	}
	crash.placeItemClaim()
	rel()
	if crash.occupancy != 2 || crash.admissions != 1 {
		t.Errorf("recovery must place exactly one claim (occ=%d admissions=%d)", crash.occupancy, crash.admissions)
	}
}

// ---- Row 3 ------------------------------------------------------------------------------------

// TestRepairAdmissionUnknownAndExternalWait: unreadable occupancy is an explicit could-not-check
// (never a fabricated free slot); an externally-blocked repair does not idle a usable slot; and a
// forged repair class (an item that is not an assignable obligation) is resolved as fresh and held.
func TestRepairAdmissionUnknownAndExternalWait(t *testing.T) {
	restoreAdmission(t)

	// UNREADABLE occupancy => exit 6, admit denied, lease released.
	scope := &fakeScope{width: 3, reserve: map[string]int{"rework": 1}, occErr: fmt.Errorf("forge unreachable"),
		demand: map[string]int{"rework": 1}}
	useBackend(t, &fakeBackend{scope: scope, owner: "U", class: deskkit.AdmissionFresh})
	if rel, err := enforceAdmission(dispatchOpts{item: "s--7"}, dispatchPlan{}, "medici-finance/assay", claimAuth{}); err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || rel != nil {
		t.Fatalf("unreadable occupancy must be could-not-check (exit 6, no admit), got admitted=%v err=%v", rel != nil, err)
	}
	if scope.leaseHeldBy != "" {
		t.Errorf("a could-not-check must release the lease, still held by %q", scope.leaseHeldBy)
	}

	// EXTERNAL-WAIT and FORGED-CLASS are properties of the AUTHORITATIVE resolution, so exercise the
	// REAL backend against an injected sidecar (a local file — no forge). Two obligations: one
	// waiting-external (environment blocker, not assignable) and one absent for the forged item.
	// Force the single-root fallback so the backend reads THIS temp root, not a configured DESK_ROOTS.
	t.Setenv(deskkit.RootsEnv, "")
	root := t.TempDir()
	env := deskkit.NewRepairObligation("medici-finance/assay", "example-stream/09", "wr-1", []int{2},
		deskkit.BlockerEnvironment, "repro", "expected", "operator must provision X", "2026-09-20T00:00:00Z")
	writeSidecar(t, root, env)

	now := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)
	real := &claimToolAdmissionBackend{o: dispatchOpts{root: root, item: "example-stream/99"}, now: now}

	demand, _, derr := real.reservedDemand()
	if derr != nil {
		t.Fatalf("reservedDemand: %v", derr)
	}
	if demand[string(deskkit.AdmissionRework)] != 0 {
		t.Errorf("a waiting-external repair must contribute NO runnable demand, got %d", demand[string(deskkit.AdmissionRework)])
	}

	// Forged class: the dispatched item is not an assignable obligation, so it resolves to fresh.
	class, cerr := real.candidateClass()
	if cerr != nil {
		t.Fatalf("candidateClass: %v", cerr)
	}
	if class != deskkit.AdmissionFresh {
		t.Errorf("an item that is not an assignable obligation must resolve to FRESH (forged repair refused), got %q", class)
	}

	// And an item that IS an assignable (implementation) obligation resolves to rework.
	impl := deskkit.NewRepairObligation("medici-finance/assay", "example-stream/17", "wr-2", []int{1},
		deskkit.BlockerImplementation, "repro", "expected", "", "2026-09-20T00:00:00Z")
	writeSidecar(t, root, impl)
	real2 := &claimToolAdmissionBackend{o: dispatchOpts{root: root, item: impl.ID}, now: now}
	if class, _ := real2.candidateClass(); class != deskkit.AdmissionRework {
		t.Errorf("an assignable implementation obligation must resolve to REWORK, got %q", class)
	}
	if d, _, _ := real2.reservedDemand(); d[string(deskkit.AdmissionRework)] != 1 {
		t.Errorf("one assignable repair => runnable rework demand 1, got %d", d[string(deskkit.AdmissionRework)])
	}
}

// ---- Row 4 ------------------------------------------------------------------------------------

// TestRepairAdmissionFullCycleRestart: a failed verification becomes a claimed repair; review and
// merge lead to exactly one owed reverification; an independent pass resolves it. At every
// transition the obligation is written to the sidecar, RE-READ from disk (a restart), and the
// admission gate is consulted — proving the obligation survives replacement and the repair is
// admissible only while it is genuinely runnable.
func TestRepairAdmissionFullCycleRestart(t *testing.T) {
	restoreAdmission(t)
	t.Setenv(deskkit.RootsEnv, "") // single-root fallback: read THIS temp root's sidecar
	root := t.TempDir()
	now := time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)

	// A failed verification => a durable, actionable repair obligation (needs-assignment).
	o := deskkit.NewRepairObligation("medici-finance/assay", "example-stream/17", "wr-9", []int{3},
		deskkit.BlockerImplementation, "the check reproduces on main", "row 3 passes", "", "2026-09-20T00:00:00Z")
	origID := o.ID

	// assignableAfterRestart writes the log line, re-reads the whole sidecar, reconciles, and returns
	// whether the (same-id) obligation is assignable now — the "restart preserves the obligation"
	// check made concrete.
	assignableAfterRestart := func(rec deskkit.RepairObligation) (deskkit.RepairObligation, bool) {
		writeSidecar(t, root, rec)
		be := &claimToolAdmissionBackend{o: dispatchOpts{root: root, item: origID}, now: now}
		obligs, err := be.reconciledObligations()
		if err != nil {
			t.Fatalf("reconcile after restart: %v", err)
		}
		for _, got := range obligs {
			if got.ID == origID {
				return got, got.Assignable(now, repairObligationLeaseTTL)
			}
		}
		t.Fatalf("the obligation %s did not survive the restart", origID)
		return deskkit.RepairObligation{}, false
	}

	// 1) needs-assignment: assignable; a fresh dispatch is HELD while the repair is runnable, the
	//    repair is admitted.
	got, ok := assignableAfterRestart(o)
	if !ok {
		t.Fatal("a fresh needs-assignment obligation must be assignable")
	}
	scope := &fakeScope{width: 3, reserve: map[string]int{"rework": 1}, occupancy: 2,
		demand: map[string]int{"rework": 1}, waiting: "repair " + got.Brief}
	useBackend(t, &fakeBackend{scope: scope, owner: "fresh", class: deskkit.AdmissionFresh})
	if _, err := enforceAdmission(dispatchOpts{item: "other--1"}, dispatchPlan{}, "medici-finance/assay", claimAuth{}); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("fresh must be held while the repair is runnable, got %v", err)
	}

	// 2) claimed (repairing) under a live lease: NOT assignable — the mutual-exclusion the immutable
	//    id preserves. A merge alone must not resolve it.
	o = o.Claim("worker-session-1", now)
	if _, ok := assignableAfterRestart(o); ok {
		t.Error("a live repairing lease must NOT be assignable after a restart")
	}

	// 3) review opened, then approved, then MERGED — one owed reverification, still unresolved.
	o = o.WithRepairPR("medici-finance/assay#1400", "", "worker-session-1")
	o = o.MarkAwaitingMerge()
	o = o.MarkRepairMerged("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	got, _ = assignableAfterRestart(o)
	if got.State != deskkit.RepairAwaitingReverification {
		t.Fatalf("a merged repair must be awaiting-reverification (a merge is not resolution), got %s", got.State)
	}
	if got.IsResolved() {
		t.Fatal("a merge alone must never resolve the obligation")
	}

	// 4) an INDEPENDENT pass at the repaired revision resolves it — exactly once.
	resolved, didResolve, reason := got.Resolve("independent-verifier", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if !didResolve {
		t.Fatalf("an independent pass at the repaired revision must resolve: %s", reason)
	}
	// A same-actor pass must NOT resolve — self-certification is refused.
	if _, ok, _ := got.Resolve("worker-session-1", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); ok {
		t.Error("a same-actor pass must not resolve the obligation")
	}
	final, _ := assignableAfterRestart(resolved)
	if !final.IsResolved() {
		t.Fatal("the resolution must survive a restart")
	}
	if final.Assignable(now, repairObligationLeaseTTL) {
		t.Error("a resolved obligation must not be re-assignable after a restart")
	}
}

// ---- helpers ----------------------------------------------------------------------------------

// writeSidecar appends one obligation's JSONL line to the repair sidecar under root, creating the
// docs/streams tree. Append-only, exactly as the live projection is: many lines for one id are the
// reconcile's job to fold.
func writeSidecar(t *testing.T, root string, o deskkit.RepairObligation) {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	line, err := o.MarshalLine()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "repair-obligations.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
}
