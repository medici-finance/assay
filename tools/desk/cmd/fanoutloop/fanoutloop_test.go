package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// --- test harness -----------------------------------------------------------------------------

// setupDeskHome points $HOME at a fresh temp dir (clean desk-tools flag dir), neutralises the
// ambient kill switch, and pins DESK_LOOP to the REGISTERED loop name so the engine's per-loop
// stop guard resolves (an unregistered name is exit-6 could-not-check). Returns the desk-tools dir
// so a test can plant STOP flags.
func setupDeskHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("CLAUDE_SESSION_ID", "fanoutloop-test")
	t.Setenv("DESK_LOOP", "worker-desk")
	return filepath.Join(home, ".config", "assay")
}

func newLoopCfg(loop *FanoutLoop, pool int, claims string) loopengine.Config {
	return loopengine.Config{
		PoolSize:     pool,
		IdlePoll:     3 * time.Millisecond,
		ClaimsDir:    claims,
		StaleClaim:   time.Hour,
		RunnerID:     loop.RunnerID, // "" for batch — author!=runner disabled (per-worker authorship)
		WorkEvidence: loop.workEvidence,
	}
}

// runUntil runs the engine in a goroutine and plants a STOP flag once stopWhen() holds, so the
// never-self-exiting engine terminates deterministically.
func runUntil(t *testing.T, cfg loopengine.Config, loop loopengine.Loop, deskDir string, stopWhen func() bool) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- loopengine.Run(cfg, loop) }()
	deadline := time.After(5 * time.Second)
	planted := false
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-tick.C:
			if !planted && stopWhen() {
				planted = true
				if err := os.MkdirAll(deskDir, 0o700); err != nil {
					t.Fatalf("mkdir deskdir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("test stop\n"), 0o600); err != nil {
					t.Fatalf("write STOP: %v", err)
				}
			}
		case <-deadline:
			t.Fatal("engine did not stop within deadline (possible wedge / no-exit bug)")
		}
	}
}

func waitFor(t *testing.T, d time.Duration, cond func() bool, what string) {
	t.Helper()
	deadline := time.After(d)
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for %s", what)
		case <-tick.C:
			if cond() {
				return
			}
		}
	}
}

// dispatchRec is the interim-mode Feeder made deterministic: it records dispatch order and can
// BLOCK a chosen item in flight (via a per-ID gate) so a test can hold the pool in a known state.
type dispatchRec struct {
	mu    sync.Mutex
	order []string
	gates map[string]chan struct{}
}

func (d *dispatchRec) feeder(it loopengine.Item, tier loopengine.Tier, prompt string) (loopengine.Result, error) {
	d.mu.Lock()
	d.order = append(d.order, it.ID)
	g := d.gates[it.ID]
	d.mu.Unlock()
	if g != nil {
		<-g // block in flight until released
	}
	return loopengine.Result{Item: it, Verdict: loopengine.VerdictPass, RunnerID: "worker-app", Artifact: "pr://" + it.ID}, nil
}

func (d *dispatchRec) dispatched() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.order))
	copy(out, d.order)
	return out
}

func (d *dispatchRec) has(id string) bool {
	for _, x := range d.dispatched() {
		if x == id {
			return true
		}
	}
	return false
}

// recordingSink captures what the near-no-op Land does: RecordDispatch (the dispatch log) and
// ReleaseDispatchClaim (branch-as-claim takeover). There is deliberately NO evidence/flip method to
// capture — Land has none, which is the point.
type recordingSink struct {
	mu       sync.Mutex
	recorded []loopengine.Result
	released []string
}

func (s *recordingSink) RecordDispatch(r loopengine.Result) error {
	s.mu.Lock()
	s.recorded = append(s.recorded, r)
	s.mu.Unlock()
	return nil
}

func (s *recordingSink) ReleaseDispatchClaim(it loopengine.Item) error {
	s.mu.Lock()
	s.released = append(s.released, it.ID)
	s.mu.Unlock()
	return nil
}

func (s *recordingSink) landedIDs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.recorded))
	for _, r := range s.recorded {
		out = append(out, r.Item.ID)
	}
	return out
}

func briefRow(stream, num, effort, execTier, gate string, oo bool) BoardRow {
	return BoardRow{
		Stream: stream, Num: num, Title: "t", Effort: effort, ExecTier: execTier, Gate: gate,
		OutOfRepo: oo, BriefPath: "docs/streams/" + stream + "/brief-" + num + "-x.md",
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func indexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}

// --- Verify row 2: standing pool + refill + orphan-resume priority ----------------------------

// TestPool exercises the whole standing-pool contract batch-fanout needs from the engine: fills to
// the standing N, REFILLS on completion, ORPHAN-RESUME PRIORITY preempts fresh dispatch,
// intake/`issue-<NN>` placeholders ARE dispatched (this loop's own work, Procedure 2), and a
// different loop's `review-request` dispatch token is excluded. Subtest names carry the words `refill` and
// `resume-priority` so the brief's Verify row 2 grep (`-run 'Pool' … grep -cE refill resume-priority`)
// observes both were exercised.
func TestPool(t *testing.T) {
	t.Run("fills-to-standing-pool-of-8", func(t *testing.T) {
		deskDir := setupDeskHome(t)
		gate := make(chan struct{})
		dr := &dispatchRec{gates: map[string]chan struct{}{}}
		sink := &recordingSink{}
		var rows []BoardRow
		for i := 0; i < 12; i++ {
			id := fmt.Sprintf("%02d", i)
			rows = append(rows, briefRow("pool", id, "M", "", "model", false))
			dr.gates["pool/"+id] = gate // every item blocks until released
		}
		loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, DispatchSink: sink, Emit: io.Discard}
		cfg := newLoopCfg(loop, 8, t.TempDir())

		errc := make(chan error, 1)
		go func() { errc <- loopengine.Run(cfg, loop) }()

		waitFor(t, 5*time.Second, func() bool { return len(dr.dispatched()) >= 8 }, "pool to fill to 8")
		time.Sleep(25 * time.Millisecond) // no completion can arrive (gate closed) → pool cannot exceed 8
		if n := len(dr.dispatched()); n != 8 {
			t.Fatalf("standing pool exceeded its cap: %d in flight, want exactly 8", n)
		}
		t.Logf("standing-pool: held exactly 8 workers in flight under 12 eligible")

		close(gate) // release all → they land, freed slots refill with the remaining 4
		waitFor(t, 5*time.Second, func() bool { return len(sink.landedIDs()) >= 12 }, "all 12 to drain via refill")

		if err := os.MkdirAll(deskDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("stop\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("engine did not exit after STOP")
		}
		if n := len(dr.dispatched()); n != 12 {
			t.Fatalf("pool of 8 must refill all 12 through: dispatched %d", n)
		}
	})

	t.Run("refill-on-completion", func(t *testing.T) {
		deskDir := setupDeskHome(t)
		dr := &dispatchRec{gates: map[string]chan struct{}{}}
		var rows []BoardRow
		for i := 0; i < 5; i++ {
			rows = append(rows, briefRow("s", fmt.Sprintf("0%d", i), "M", "", "model", false))
		}
		loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, Emit: io.Discard}
		cfg := newLoopCfg(loop, 2, t.TempDir())

		err := runUntil(t, cfg, loop, deskDir, func() bool { return len(dr.dispatched()) >= 5 })
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if n := len(dr.dispatched()); n != 5 {
			t.Fatalf("a pool of 2 must refill 5 items through on completion; dispatched %d", n)
		}
		t.Logf("refill: a standing pool of 2 drained 5 items by refill-on-completion")
	})

	t.Run("resume-priority-preempts-fresh", func(t *testing.T) {
		deskDir := setupDeskHome(t)
		dr := &dispatchRec{gates: map[string]chan struct{}{}}
		fresh := briefRow("fresh", "01", "S", "", "model", false)
		orphan := OrphanPR{Repo: "medici-finance/assay", Number: 1234, ID: "resume:pr-1234", Branch: "feat/x", Findings: "address review"}
		loop := &FanoutLoop{
			Board:   func() ([]BoardRow, error) { return []BoardRow{fresh}, nil },
			Orphans: func() ([]OrphanPR, error) { return []OrphanPR{orphan}, nil },
			Feeder:  dr.feeder, Emit: io.Discard,
		}
		cfg := newLoopCfg(loop, 1, t.TempDir()) // ONE slot: whoever goes first is the priority winner

		err := runUntil(t, cfg, loop, deskDir, func() bool { return dr.has("resume:pr-1234") && dr.has("fresh/01") })
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		order := dr.dispatched()
		if len(order) == 0 || order[0] != "resume:pr-1234" {
			t.Fatalf("resume-priority: orphan resume did not preempt fresh dispatch (order=%v)", order)
		}
		t.Logf("resume-priority: the orphan resume preempted the fresh brief (order=%v)", order)
	})

	t.Run("intake-placeholder-dispatched", func(t *testing.T) {
		// Per the worker-desk dispatch spec (Procedure 2 — "INCLUDE issue-placeholders —
		// `issue-<NN>` rows ARE yours to dispatch"), an unclaimed `todo` issue placeholder on the
		// board IS this loop's work and MUST be dispatched. issue-loop is the single largest work
		// class; dropping it silently ships the loop pre-blinded to it at cutover.
		deskDir := setupDeskHome(t)
		dr := &dispatchRec{gates: map[string]chan struct{}{}}
		rows := []BoardRow{
			{Stream: "intake", Num: "issue-42", Title: "placeholder", BriefPath: "docs/streams/intake/x.md"},
			briefRow("real", "01", "M", "", "model", false),
		}
		loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, Emit: io.Discard}
		cfg := newLoopCfg(loop, 4, t.TempDir())

		err := runUntil(t, cfg, loop, deskDir, func() bool { return dr.has("intake/issue-42") && dr.has("real/01") })
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !dr.has("intake/issue-42") {
			t.Fatal("an intake/issue-* placeholder was NOT dispatched — per worker-desk Procedure 2, `issue-<NN>` rows ARE this loop's to dispatch")
		}
	})

	t.Run("review-request-token-excluded", func(t *testing.T) {
		// A `review-request` issue is a dispatch token for the review loop (pr-review-desk), NOT
		// worker-desk's work — the issue-loop work-scanner excludes it by its `review-request`
		// label. It must NOT be dispatched here even though it is issue-shaped by number.
		deskDir := setupDeskHome(t)
		dr := &dispatchRec{gates: map[string]chan struct{}{}}
		rows := []BoardRow{
			{Stream: "intake", Num: "issue-77", Title: "review-request: PR #123 — code + security", BriefPath: "docs/streams/intake/rr.md"},
			briefRow("real", "02", "M", "", "model", false),
		}
		loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, Emit: io.Discard}
		cfg := newLoopCfg(loop, 4, t.TempDir())

		err := runUntil(t, cfg, loop, deskDir, func() bool { return dr.has("real/02") })
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if dr.has("intake/issue-77") {
			t.Fatal("a review-request dispatch token was dispatched — those belong to the review loop, not worker-desk")
		}
	})
}

// --- out-of-repo serialization -----------------------------------------------------------------

// TestSerial proves the out-of-repo serialization: at most ONE brief declaring
// `out-of-repo files:` may be in flight across all streams. It is enforced on the engine's existing
// WorkEvidence config seam (a typed check), so the frozen Loop contract is untouched.
func TestSerial(t *testing.T) {
	deskDir := setupDeskHome(t)
	gateA := make(chan struct{})
	dr := &dispatchRec{gates: map[string]chan struct{}{"oo-a/01": gateA}}
	sink := &recordingSink{}
	rows := []BoardRow{
		briefRow("oo-a", "01", "M", "", "model", true),   // out-of-repo A (blocks in flight)
		briefRow("oo-b", "01", "M", "", "model", true),   // out-of-repo B (must be refused while A holds the slot)
		briefRow("plain", "01", "M", "", "model", false), // normal C (dispatches freely)
	}
	loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, DispatchSink: sink, Emit: io.Discard}
	cfg := newLoopCfg(loop, 8, t.TempDir())

	errc := make(chan error, 1)
	go func() { errc <- loopengine.Run(cfg, loop) }()

	// A is in flight and C landed; B must NOT have been dispatched (single out-of-repo slot held by A).
	waitFor(t, 5*time.Second, func() bool { return dr.has("oo-a/01") && contains(sink.landedIDs(), "plain/01") }, "A in flight and C landed")
	time.Sleep(30 * time.Millisecond) // give a wrongful second out-of-repo dispatch a chance to happen
	if dr.has("oo-b/01") {
		t.Fatal("a SECOND out-of-repo brief dispatched while the first was in flight — out-of-repo serialization broken")
	}

	close(gateA) // A lands → the single out-of-repo slot frees → B becomes dispatchable
	waitFor(t, 5*time.Second, func() bool { return dr.has("oo-b/01") }, "B to dispatch after A landed")

	order := dr.dispatched()
	if indexOf(order, "oo-a/01") > indexOf(order, "oo-b/01") {
		t.Fatalf("B dispatched before A — serialization must release only after A lands (order=%v)", order)
	}

	if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not exit after STOP")
	}
}

// --- negative test: claim collision via the shared claims dir ---------------------------------

// TestClaimCollisionSharedClaimsDir proves a concurrent claimant holding an item via the SHARED
// claims dir (e.g. an intake dispatcher) blocks this loop from double-dispatching that item —
// the structural no-double-dispatch guarantee, which is engine-owned and inherited unchanged.
func TestClaimCollision_SharedClaimsDir(t *testing.T) {
	deskDir := setupDeskHome(t)
	claims := t.TempDir()
	ok, err := deskkit.Acquire(
		deskkit.ClaimConfig{ClaimsDir: claims, StaleClaim: time.Hour},
		deskkit.Claim{Kind: deskkit.KindDispatch, Item: "hold/01", Owner: "intake-desk"})
	if err != nil || !ok {
		t.Fatalf("preplant concurrent claim: ok=%v err=%v", ok, err)
	}

	dr := &dispatchRec{gates: map[string]chan struct{}{}}
	rows := []BoardRow{
		briefRow("free", "01", "M", "", "model", false),
		briefRow("hold", "01", "M", "", "model", false), // held elsewhere via the shared claims dir
		briefRow("free", "02", "M", "", "model", false),
	}
	loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Feeder: dr.feeder, Emit: io.Discard}
	cfg := newLoopCfg(loop, 3, claims)

	err = runUntil(t, cfg, loop, deskDir, func() bool { return dr.has("free/01") && dr.has("free/02") })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if dr.has("hold/01") {
		t.Fatal("an item with a live claim held elsewhere via the shared claims dir was dispatched (double-dispatch)")
	}
}

// --- negative test: cap-starved pool fills remaining slots with orphan resumes ----------------

// TestCapStarvedFillsWithOrphans proves that when the fresh Next-up queue is cap-starved (fewer
// eligible rows than free slots), the remaining slots are filled with orphan resumes BEFORE a slot
// idles — the SKILL's "must never idle a slot" rule, realised through SelectQueue's ordering.
func TestCapStarvedFillsWithOrphans(t *testing.T) {
	deskDir := setupDeskHome(t)
	gate := make(chan struct{})
	dr := &dispatchRec{gates: map[string]chan struct{}{}}
	for _, id := range []string{"capped/01", "resume:1", "resume:2"} {
		dr.gates[id] = gate // hold all three so occupancy is observable
	}
	fresh := briefRow("capped", "01", "M", "", "model", false)
	orphans := []OrphanPR{
		{Repo: "r", Number: 1, ID: "resume:1", Branch: "b1"},
		{Repo: "r", Number: 2, ID: "resume:2", Branch: "b2"},
	}
	loop := &FanoutLoop{
		Board:   func() ([]BoardRow, error) { return []BoardRow{fresh}, nil },
		Orphans: func() ([]OrphanPR, error) { return orphans, nil },
		Feeder:  dr.feeder, Emit: io.Discard,
	}
	cfg := newLoopCfg(loop, 3, t.TempDir()) // 3 slots, only 1 fresh brief → 2 must be orphan resumes

	errc := make(chan error, 1)
	go func() { errc <- loopengine.Run(cfg, loop) }()

	waitFor(t, 5*time.Second, func() bool { return len(dr.dispatched()) >= 3 }, "3 slots to fill (1 fresh + 2 orphan resumes)")
	time.Sleep(20 * time.Millisecond)
	d := dr.dispatched()
	if len(d) != 3 {
		t.Fatalf("want exactly 3 slots filled, got %d: %v", len(d), d)
	}
	for _, want := range []string{"capped/01", "resume:1", "resume:2"} {
		if !dr.has(want) {
			t.Fatalf("slot not filled by %s — orphans must fill before a slot idles (dispatched=%v)", want, d)
		}
	}

	close(gate)
	if err := os.WriteFile(filepath.Join(deskDir, "STOP"), []byte("stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errc:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("engine did not exit after STOP")
	}
}

// --- TierPolicy: effort × exec-tier, gate:human dispatches normally ---------------------------

func TestTierPolicyEffortAndExecTier(t *testing.T) {
	f := &FanoutLoop{}
	brief := func(effort, execTier, gate string) loopengine.Item {
		return loopengine.Item{Effort: effort, ExecTier: execTier, Gate: gate, Payload: map[string]string{"kind": kindBrief}}
	}
	cases := []struct {
		name string
		it   loopengine.Item
		want loopengine.Tier
	}{
		{"S -> session", brief("S", "", "model"), loopengine.TierSession},
		{"M -> cheap", brief("M", "", "model"), loopengine.TierCheap},
		{"L -> cheap", brief("L", "", "model"), loopengine.TierCheap},
		{"unspecified -> cheap", brief("", "", "model"), loopengine.TierCheap},
		{"exec:strong overrides L -> session", brief("L", "strong", "model"), loopengine.TierSession},
		{"exec:strong overrides S -> session", brief("S", "strong", "model"), loopengine.TierSession},
		{"gate:human dispatches normally (M -> cheap, NOT human)", brief("M", "", "human"), loopengine.TierCheap},
		{"orphan resume -> cheap", loopengine.Item{Payload: map[string]string{"kind": kindOrphan}}, loopengine.TierCheap},
	}
	for _, c := range cases {
		got, err := f.TierPolicy(c.it)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
		if got == loopengine.TierHuman {
			t.Fatalf("%s: batch-fanout must NEVER emit TierHuman (gate:human dispatches normally)", c.name)
		}
	}

	// Reachable set is disjoint from verify-desk's on TierLocal — the contract-generalization point.
	reach := f.reachableTiers()
	if len(reach) != 2 || reach[0] != loopengine.TierCheap || reach[1] != loopengine.TierSession {
		t.Fatalf("reachableTiers = %v; want [cheap session]", reach)
	}
	for _, tr := range reach {
		if tr == loopengine.TierLocal || tr == loopengine.TierHuman {
			t.Fatalf("reachable set must exclude TierLocal and TierHuman, got %v", tr)
		}
	}
}

// --- Land is a near-no-op: record + release, no Evidence, no flip -----------------------------

func TestLand_NearNoOp_RecordsAndReleases(t *testing.T) {
	sink := &recordingSink{}
	loop := &FanoutLoop{DispatchSink: sink}
	r := loopengine.Result{
		Item:     loopengine.Item{ID: "fixture/02", Payload: map[string]string{"repo": "medici-finance/assay"}},
		Verdict:  loopengine.VerdictPass,
		RunnerID: "worker-app",
		Artifact: "https://github.com/medici-finance/assay/pull/1",
	}
	if err := loop.Land(r); err != nil {
		t.Fatalf("Land: %v", err)
	}
	if len(sink.recorded) != 1 || sink.recorded[0].Item.ID != "fixture/02" {
		t.Fatalf("Land did not record the handle in the dispatch log: %+v", sink.recorded)
	}
	if len(sink.released) != 1 || sink.released[0] != "fixture/02" {
		t.Fatalf("Land did not release the dispatch claim (branch-as-claim takeover): %v", sink.released)
	}
	// Land marks the item handled so the board's claim-filtering is modelled and the queue drains.
	if !loop.isHandled("fixture/02") {
		t.Fatal("Land did not mark the item handled")
	}
}

// --- the real claim-release command shape (unit, no network) ----------------------------------

func TestReleaseDispatchClaim_GhCommandShape(t *testing.T) {
	var got []string
	s := &ghDispatchSink{out: io.Discard, run: func(args ...string) (string, error) { got = args; return "", nil }}
	it := loopengine.Item{ID: "fixture/02", Payload: map[string]string{"repo": "medici-finance/assay"}}
	if err := s.ReleaseDispatchClaim(it); err != nil {
		t.Fatalf("ReleaseDispatchClaim: %v", err)
	}
	want := []string{"gh", "api", "-X", "DELETE", "repos/medici-finance/assay/git/refs/dispatch/fixture--02"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("release command shape:\n got %v\nwant %v", got, want)
	}
	// A missing ref (already released) is a no-op, not an error.
	s.run = func(args ...string) (string, error) { return "Not Found", errors.New("exit status 1") }
	if err := s.ReleaseDispatchClaim(it); err != nil {
		t.Fatalf("missing ref must be a no-op: %v", err)
	}
}

// --- dispatch prompt carries the skill essentials verbatim, no shared-checkout path -----------

func TestDispatchPrompt_CarriesSkillEssentials(t *testing.T) {
	it := loopengine.Item{
		ID: "fixture/02", BriefPath: "docs/streams/fixture/brief-02-x.md", TargetSHA: "deadbeef",
		Gate: "human", ExecTier: "strong", Payload: map[string]string{"repo": "medici-finance/assay", "kind": kindBrief},
	}
	p := renderDispatchPrompt(it, loopengine.TierSession)
	for _, must := range []string{
		"deadbeef",
		"refs/remotes/origin/main --detach",                  // isolation / branch-from-main (F-35)
		"git rev-parse --show-toplevel",                      // home-worktree abort guard
		"KUBECONFIG=/dev/null",                               // offline envelope
		"BLOCKED-ON-HUMAN — security-gate removal",           // security-gate clause
		"A task completed via substitution is a failed task", // no-evasion clause
		"merge, NEVER rebase",                                // merge-not-rebase
		"One brief = one branch = one PR",                    // one-brief-one-branch-one-PR
		"Stop at `implemented`",                              // stop-at-implemented
		"fast/cheap-tier model, STOP",                        // exec:strong pickup-STOP
		"gate:human",                                         // gate:human handling present
		"BLOCKED-ON-IAN",                                     // gate:human cutover stop-point
	} {
		if !strings.Contains(p, must) {
			t.Fatalf("dispatch prompt missing essential %q:\n%s", must, p)
		}
	}
	if err := assertNoSharedCheckout(p); err != nil {
		t.Fatalf("clean prompt flagged: %v", err)
	}
	leaky := p + "\ncd /Users/x/tracker/.claude/worktrees/thedesk && go test"
	if err := assertNoSharedCheckout(leaky); err == nil {
		t.Fatal("a shared-checkout path was not refused")
	}
}

func TestDispatchPrompt_OrphanResumeShape(t *testing.T) {
	it := loopengine.Item{
		ID:      "resume:pr-1234",
		Payload: map[string]string{"repo": "medici-finance/assay", "kind": kindOrphan, "pr": "1234", "branch": "feat/x", "findings": "address the reviewer's finding"},
	}
	p := renderDispatchPrompt(it, loopengine.TierCheap)
	for _, must := range []string{"RESUME PR", "1234", "feat/x", "address the reviewer's finding", "KUBECONFIG=/dev/null"} {
		if !strings.Contains(p, must) {
			t.Fatalf("orphan resume prompt missing %q:\n%s", must, p)
		}
	}
}

// --- the default STATUS.md board reader (intake skip + out-of-repo detection) -------------

func TestReadNextUp_ParsesRowsAndDetectsOutOfRepo(t *testing.T) {
	root := t.TempDir()
	status := "# STATUS\n\n## Next up\n\n" +
		"_Next-up: 2 of 2 eligible._\n\n" +
		"| Stream | Brief | Wave | Score |\n" +
		"|---|---|---|---|\n" +
		"| fixture | 02 — Generalize batch-fanout [exec:strong] | 1 | 2010 |\n" +
		"| intake | issue-42 — placeholder row | 0 | 1000 |\n" +
		"\n## Intake queue\n\n| x |\n"
	if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	briefDir := filepath.Join(root, "docs", "streams", "fixture")
	if err := os.MkdirAll(briefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "---\nbrief: fixture/02\ngate: human\neffort: M\nexec-tier: strong\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\n" +
		"## Context\nfiles: cmd/fanoutloop/\nout-of-repo files: `~/.claude/skills/batch-fanout/SKILL.md`\n\n## Task\ndo it\n"
	if err := os.WriteFile(filepath.Join(briefDir, "brief-02-generalize.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := readNextUp(root, "abc123")
	if err != nil {
		t.Fatalf("readNextUp: %v", err)
	}
	var le, il *BoardRow
	for i := range rows {
		switch rows[i].Stream {
		case "fixture":
			le = &rows[i]
		case "intake":
			il = &rows[i]
		}
	}
	if le == nil {
		t.Fatalf("fixture/02 not parsed from Next up: %+v", rows)
	}
	if le.Num != "02" || le.Effort != "M" || le.Gate != "human" || le.ExecTier != "strong" {
		t.Fatalf("frontmatter not resolved: %+v", *le)
	}
	if !le.OutOfRepo {
		t.Fatal("out-of-repo declaration not detected from the brief Context")
	}
	if le.BriefPath != filepath.Join("docs", "streams", "fixture", "brief-02-generalize.md") {
		t.Fatalf("brief path not resolved relative to root: %q", le.BriefPath)
	}
	if il == nil || !isIssuePlaceholder(*il) {
		t.Fatalf("intake/issue-42 must be recognised as an issue placeholder (this loop's own work): %+v", il)
	}
	if isIssuePlaceholder(*le) {
		t.Fatal("a real brief row was misclassified as an intake placeholder")
	}
}

// TestSelectQueue_IncludesPlaceholdersExcludesForeignTokens is the direct, engine-free proof of the
// dispatch-selection fix: an unclaimed `todo` `issue-<NN>` work placeholder SURVIVES SelectQueue (it
// is this loop's work — worker-desk dispatch spec, Procedure 2), a normal brief row survives, and a
// `review-request` dispatch token — which belongs to the review loop, not worker-desk — is dropped.
func TestSelectQueue_IncludesPlaceholdersExcludesForeignTokens(t *testing.T) {
	rows := []BoardRow{
		briefRow("real", "01", "M", "", "model", false),
		{Stream: "intake", Num: "issue-42", Title: "some real work item", BriefPath: "docs/streams/intake/x.md"},
		{Stream: "intake", Num: "issue-77", Title: "review-request: PR #123 — code + security", BriefPath: "docs/streams/intake/rr.md"},
	}
	loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, TargetSHA: "sha"}

	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if !contains(ids, "intake/issue-42") {
		t.Errorf("issue-<NN> placeholder was dropped from the queue; per Procedure 2 it IS dispatchable: %v", ids)
	}
	if !contains(ids, "real/01") {
		t.Errorf("real brief row missing from the queue: %v", ids)
	}
	if contains(ids, "intake/issue-77") {
		t.Errorf("review-request dispatch token was included; it belongs to the review loop: %v", ids)
	}
}

// TestDispatchTokenDiscriminators pins the two independent predicates the fix separates: an
// `issue-<NN>` placeholder is recognised as this loop's work and NOT as a foreign token, while a
// `review-request` token (issue-shaped by number but the review loop's) is a foreign token.
func TestDispatchTokenDiscriminators(t *testing.T) {
	placeholder := BoardRow{Stream: "intake", Num: "issue-42", Title: "some real work item"}
	reviewToken := BoardRow{Stream: "intake", Num: "issue-77", Title: "review-request: PR #123 — code + security"}
	realBrief := briefRow("real", "01", "M", "", "model", false)

	if !isIssuePlaceholder(placeholder) {
		t.Error("issue-42 not recognised as an issue placeholder")
	}
	if isForeignDispatchToken(placeholder) {
		t.Error("a plain issue placeholder was misclassified as a foreign dispatch token — it IS this loop's work")
	}
	if !isForeignDispatchToken(reviewToken) {
		t.Error("a review-request token was not recognised as a foreign dispatch token")
	}
	if isForeignDispatchToken(realBrief) {
		t.Error("a real brief row was misclassified as a foreign dispatch token")
	}
}

// TestForeignDispatchTokenDiscriminatesOnLabel pins the assay#101 hardening: the authoritative
// discriminator is the `review-request` LABEL, so a token whose TITLE deviated from the canonical
// prefix is still caught when the board source resolved the issue's real labels, while the title
// convention remains a fallback for label-less rows (the default STATUS.md reader has no labels).
func TestForeignDispatchTokenDiscriminatesOnLabel(t *testing.T) {
	// Authoritative catch: canonical title missing, but the review-request label is present.
	labelledDeviantTitle := BoardRow{
		Stream: "intake", Num: "issue-88",
		Title:  "please review PR #200", // NOT the canonical prefix
		Labels: []string{"review-request"},
	}
	if !isForeignDispatchToken(labelledDeviantTitle) {
		t.Error("a review-request-LABELLED token with a non-canonical title slipped the filter — the label is authoritative (assay#101)")
	}

	// Case-insensitive label match.
	if !isForeignDispatchToken(BoardRow{Num: "issue-89", Labels: []string{"Review-Request"}}) {
		t.Error("label match must be case-insensitive")
	}

	// Title fallback still catches a label-less token (STATUS.md source has no label column).
	labelless := BoardRow{Stream: "intake", Num: "issue-90", Title: "review-request: PR #300 — code"}
	if !isForeignDispatchToken(labelless) {
		t.Error("title-convention fallback must still catch a label-less review-request token")
	}

	// A work placeholder that carries only ordinary labels is NOT a foreign token.
	workWithLabels := BoardRow{
		Stream: "intake", Num: "issue-91",
		Title:  "some real work item",
		Labels: []string{"enhancement", "priority-2"},
	}
	if isForeignDispatchToken(workWithLabels) {
		t.Error("a work placeholder with ordinary labels was misclassified as a foreign dispatch token")
	}
}
