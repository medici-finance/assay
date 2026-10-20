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

// TestRepairAdmissionConcurrentAndCrash: dispatchers are separate OS processes sharing one claim
// store (see repairadmission_proc_test.go). Two race for the LAST free slot and exactly one is
// admitted; a dispatcher killed at each boundary (holding the lease; after the item claim) leaks
// no unbounded slot and never yields a second worker for an item.
func TestRepairAdmissionConcurrentAndCrash(t *testing.T) {
	// RACE: width 3, two live claims => one free slot. Both contenders start on one signal and
	// hold the lease across a widened count -> claim window. Several rounds, fresh store each.
	contention := 0
	for round := 0; round < 3; round++ {
		f := newAdmitFixture(t, 3, "")
		f.place("assay--busy--1")
		f.place("assay--busy--2")
		a, b := fmt.Sprintf("race-%d--a", round), fmt.Sprintf("race-%d--b", round)
		codes, said, n := f.race(a, b)
		contention += n
		wins := 0
		for i, c := range codes {
			switch c {
			case 0:
				wins++
			case deskkit.ExitRefused:
				if !strings.Contains(said[i], "pool full") {
					t.Errorf("round %d: the loser must be HELD on a full pool, got: %s", round, said[i])
				}
			default:
				t.Errorf("round %d: unexpected dispatcher exit %d: %s", round, c, said[i])
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: exactly one admission for the last slot, got %d (exits %v)", round, wins, codes)
		}
		if got := len(f.items()); got != 3 {
			t.Fatalf("round %d: the store must hold width=3 item claims, got %d: %v", round, got, f.items())
		}
		if f.held(admitLeaseKey) {
			t.Fatalf("round %d: the admission lease must be released after the race", round)
		}
	}
	if contention == 0 {
		t.Fatal("no round observed a dispatcher blocked on the held lease — the race never overlapped")
	}

	// CRASH holding the lease, before the item claim.
	f := newAdmitFixture(t, 3, "")
	f.place("assay--busy--1")
	f.crash("crash--c", crashLease)
	if !f.held(admitLeaseKey) || f.held("crash--c") {
		t.Fatalf("killed after the lease: want lease held and no item claim, items=%v", f.items())
	}
	if code, said := f.run("crash--d"); code != deskkit.ExitUnverifiable || !strings.Contains(said, "holds the serialization lease") {
		t.Fatalf("while the dead lease is inside its TTL another dispatcher must wait (exit 6), got %d: %s", code, said)
	}
	if f.held("crash--d") {
		t.Fatal("a dispatcher refused at the lease must not have claimed its item")
	}
	f.age(admitLeaseKey) // the dead dispatcher's lease passes the claim tool's TTL
	if code, said := f.run("crash--d"); code != 0 {
		t.Fatalf("the claim tool must reclaim the expired lease and admit, got %d: %s", code, said)
	}
	if code, said := f.run("crash--c"); code != 0 {
		t.Fatalf("the crashed dispatcher's item must still be dispatchable (no slot leaked), got %d: %s", code, said)
	}
	if got := len(f.items()); got != 3 || f.held(admitLeaseKey) {
		t.Fatalf("after recovery: want 3 item claims and no lease, got %v lease=%v", f.items(), f.held(admitLeaseKey))
	}

	// CRASH after the item claim, before the lease release. Width 2, empty pool.
	f = newAdmitFixture(t, 2, "")
	f.crash("crash--e", crashClaim)
	if !f.held(admitLeaseKey) || !f.held("crash--e") {
		t.Fatalf("killed after the item claim: want lease and item claim held, items=%v", f.items())
	}
	f.age(admitLeaseKey)
	// Resuming the SAME item: admitted (a slot is free) but the live item claim refuses it, and the
	// resumed dispatcher releases the lease it took — never a second worker for crash--e.
	code, said := f.run("crash--e")
	if code != deskkit.ExitRefused || !strings.Contains(said, "LIVE holder") {
		t.Fatalf("resuming a crashed item whose claim is live must be refused (exit 5, LIVE holder), got %d: %s", code, said)
	}
	if f.held(admitLeaseKey) {
		t.Fatal("a refused item claim must release the admission lease")
	}
	// The dead claim counts as occupancy while inside its TTL: one more fills the pool.
	if code, said := f.run("crash--f"); code != 0 {
		t.Fatalf("the second slot must admit, got %d: %s", code, said)
	}
	if code, said := f.run("crash--g"); code != deskkit.ExitRefused || !strings.Contains(said, "pool full") {
		t.Fatalf("with the dead claim inside its TTL the pool is full, got %d: %s", code, said)
	}
	// Past its TTL the dead claim stops counting and exactly one resume reclaims it in place.
	f.age("crash--e")
	if code, said := f.run("crash--e"); code != 0 {
		t.Fatalf("resume must reclaim the expired item claim, got %d: %s", code, said)
	}
	if code, said := f.run("crash--e"); code != deskkit.ExitRefused {
		t.Fatalf("a second resume of the same item must be refused, got %d: %s", code, said)
	}
	if code, said := f.run("crash--g"); code != deskkit.ExitRefused {
		t.Fatalf("the reclaimed slot is occupied again; a new item must be held, got %d: %s", code, said)
	}
	if got := f.items(); len(got) != 2 {
		t.Fatalf("the pool must hold exactly width=2 item claims, got %v", got)
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

// repairForge is the repair PR as a forge read reports it. observe renders it as the observation
// the production observation step (deskkit.ApplyForgeObservation, shared with fanoutloop) consumes.
type repairForge struct {
	pr, author, state, mergedSHA string // state: "", open, approved, merged
}

func (g *repairForge) observe() deskkit.RepairForgeObservation {
	return deskkit.RepairForgeObservation{
		ReviewOpened: g.state == "open",
		Approved:     g.state == "approved",
		Merged:       g.state == "merged",
		MergedSHA:    g.mergedSHA,
		RepairPR:     g.pr,
		RepairedBy:   g.author,
	}
}

// TestRepairAdmissionFullCycleRestart: a failed verification becomes a claimed repair; forge
// observations of review and merge lead to exactly one owed reverification; an independent pass
// resolves it. Dispatchers are real processes on a shared claim store (repairadmission_proc_test.go)
// and the repair sidecar is the only obligation memory. At EVERY state boundary a dispatcher is
// killed mid-admission and the process state is rebuilt from disk: the obligation survives with
// one id, replaying the last forge observation changes nothing, and resuming never yields a second
// worker for the repair.
func TestRepairAdmissionFullCycleRestart(t *testing.T) {
	// Width 4, rework reserve 2, two other live workers: a free slot exists, but only for the repair.
	f := newAdmitFixture(t, 4, "rework=2")
	f.place("assay--busy--1")
	f.place("assay--busy--2")
	forge := &repairForge{pr: "medici-finance/assay#1400", author: "worker-session-1"}
	const mergedSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// A failed verification => a durable, actionable repair obligation (needs-assignment).
	o := deskkit.NewRepairObligation(admitRepo, "example-stream/17", "wr-9", []int{3},
		deskkit.BlockerImplementation, "the check reproduces on main", "row 3 passes", "",
		time.Now().UTC().Format(time.RFC3339))
	id := o.ID
	repairKey := claimKeyFor(id, admitRepo)
	writeSidecar(t, f.root, o)

	// boundary kills a dispatcher of the repair item while it holds the admission lease, restarts
	// from disk, replays the last forge observation, and (resume) re-runs the dispatcher once the
	// dead lease expires. It returns the restarted obligation.
	boundary := func(stage, wantState string, resume bool) deskkit.RepairObligation {
		t.Helper()
		f.crash(id, crashLease)
		got := sidecarObligations(t, f.root, id)
		if got.State != wantState {
			t.Fatalf("%s: after a dispatcher kill and restart the obligation is %s, want %s", stage, got.State, wantState)
		}
		if replayed, changed := deskkit.ApplyForgeObservation(got, forge.observe()); changed {
			t.Fatalf("%s: replaying the forge observation after restart moved %s -> %s", stage, got.State, replayed.State)
		}
		f.age(admitLeaseKey)
		if !resume {
			return got
		}
		code, said := f.run(id)
		if code != deskkit.ExitRefused || !strings.Contains(said, "LIVE holder") {
			t.Fatalf("%s: resuming while the repair worker holds the item must be refused (LIVE holder), got %d: %s",
				stage, code, said)
		}
		if f.held(admitLeaseKey) {
			t.Fatalf("%s: the resumed dispatcher left the admission lease held", stage)
		}
		return got
	}

	// advance applies the current forge observation through the production observation step and records
	// the result in the append-only sidecar.
	advance := func(stage, wantState string) {
		t.Helper()
		next, changed := deskkit.ApplyForgeObservation(sidecarObligations(t, f.root, id), forge.observe())
		if !changed || next.State != wantState {
			t.Fatalf("%s: forge observation gave %s (changed=%v), want %s", stage, next.State, changed, wantState)
		}
		writeSidecar(t, f.root, next)
	}

	// 1) needs-assignment. A fresh dispatch is HELD: the last free slots are reserved for the
	//    runnable repair, which the hold names.
	if code, said := f.run("assay--fresh--1"); code != deskkit.ExitRefused || !strings.Contains(said, "example-stream/17") {
		t.Fatalf("fresh must be held while the repair is runnable, got %d: %s", code, said)
	}
	// Kill a repair dispatcher holding the lease: nothing durable changed, the obligation is intact.
	f.crash(id, crashLease)
	if f.held(repairKey) {
		t.Fatal("a dispatcher killed before its item claim must leave no claim")
	}
	if got := sidecarObligations(t, f.root, id); got.State != deskkit.RepairNeedsAssignment {
		t.Fatalf("restart after a lease kill: %s, want needs-assignment", got.State)
	}
	f.age(admitLeaseKey)
	// Kill the next one after its item claim, before the lease release.
	f.crash(id, crashClaim)
	if !f.held(repairKey) {
		t.Fatal("the dispatcher killed after its item claim must have placed the claim")
	}
	f.age(admitLeaseKey)
	if code, said := f.run(id); code != deskkit.ExitRefused || !strings.Contains(said, "LIVE holder") {
		t.Fatalf("resuming while the dead dispatcher's item claim is live must be refused, got %d: %s", code, said)
	}
	// Past its TTL the dead claim is reclaimed by exactly one resume: the repair worker.
	f.age(repairKey)
	if code, said := f.run(id); code != 0 {
		t.Fatalf("resume must reclaim the expired repair claim and admit the repair, got %d: %s", code, said)
	}
	if code, _ := f.run(id); code == 0 {
		t.Fatal("a second dispatch of the claimed repair must not be admitted")
	}

	// 2) repairing: the worker takes the obligation lease.
	writeSidecar(t, f.root, sidecarObligations(t, f.root, id).Claim("worker-session-1", time.Now()))
	got := boundary("repairing", deskkit.RepairRepairing, true)
	be := &claimToolAdmissionBackend{o: dispatchOpts{root: f.root, item: id}, now: time.Now()}
	if d, _, _ := be.reservedDemand(); d[string(deskkit.AdmissionRework)] != 0 {
		t.Errorf("a repair under a live lease is not unmet demand, got %d", d[string(deskkit.AdmissionRework)])
	}
	if got.Assignable(time.Now(), repairObligationLeaseTTL) {
		t.Fatal("a live repairing lease must not be assignable after a restart")
	}

	// 3) the repair PR opens: awaiting-review.
	forge.state = "open"
	advance("review opened", deskkit.RepairAwaitingReview)
	boundary("awaiting-review", deskkit.RepairAwaitingReview, true)

	// 4) approved: awaiting-merge.
	forge.state = "approved"
	advance("approved", deskkit.RepairAwaitingMerge)
	boundary("awaiting-merge", deskkit.RepairAwaitingMerge, true)

	// 5) merged: one owed reverification — never a resolution.
	forge.state, forge.mergedSHA = "merged", mergedSHA
	advance("merged", deskkit.RepairAwaitingReverification)
	got = boundary("awaiting-reverification", deskkit.RepairAwaitingReverification, true)
	if got.IsResolved() || got.RepairSHA != mergedSHA {
		t.Fatalf("a merge must wake reverification at %s without resolving: %+v", mergedSHA, got)
	}
	// The repair worker exits; its claim frees the slot. The obligation is not runnable again.
	if err := deskkit.Release(f.cfg(), repairKey); err != nil {
		t.Fatal(err)
	}
	be = &claimToolAdmissionBackend{o: dispatchOpts{root: f.root, item: id}, now: time.Now()}
	if class, _ := be.candidateClass(); class != deskkit.AdmissionFresh {
		t.Errorf("an obligation awaiting reverification is not repair work, resolved class %q", class)
	}

	// 6) an INDEPENDENT pass at the repaired revision resolves it; the repairing worker cannot.
	if _, ok, _ := got.Resolve("worker-session-1", mergedSHA); ok {
		t.Fatal("a same-actor pass must not resolve the obligation")
	}
	resolved, ok, reason := got.Resolve("independent-verifier", mergedSHA)
	if !ok {
		t.Fatalf("an independent pass at the repaired revision must resolve: %s", reason)
	}
	writeSidecar(t, f.root, resolved)
	// No resume here: the repair worker is gone and the obligation is closed, so a dispatch of the
	// item would be fresh work under the ordinary floor, not the repair.
	final := boundary("resolved", deskkit.RepairResolved, false)
	if final.Assignable(time.Now(), repairObligationLeaseTTL) {
		t.Error("a resolved obligation must not be re-assignable after a restart")
	}
	// Across the whole cycle the sidecar recorded one line per transition for ONE obligation id.
	if n := sidecarLines(t, f.root); n != 6 {
		t.Errorf("want 6 sidecar lines (one per transition), got %d", n)
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

// regression: #2163
// TestCountLiveSuffixItem: occupancy skips exactly this scope's admission lease. Another claim
// whose id merely ends in "--admission" (another loop's lease, or an item named that way) is a
// held claim and is counted — under-counting occupancy would admit past a full pool.
func TestCountLiveSuffixItem(t *testing.T) {
	lease := "assay--" + admissionLoop + "--admission"
	out := strings.Join([]string{
		"dispatch-claim: HELD " + lease + " — dispatch-claim owner=a state=claimed at=2026-09-20T00:00:00Z age=1m",
		"dispatch-claim: HELD assay--other-loop--admission — dispatch-claim owner=b state=claimed at=2026-09-20T00:00:00Z age=1m",
		"dispatch-claim: HELD assay--s--1 — dispatch-claim owner=c state=claimed at=2026-09-20T00:00:00Z age=1m",
	}, "\n")
	if n := countLiveClaims(out, lease); n != 2 {
		t.Fatalf("countLiveClaims = %d, want 2 (only the scope's own lease is excluded)", n)
	}
}
