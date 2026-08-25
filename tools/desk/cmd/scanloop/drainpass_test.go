package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// drainpass_test.go — the pass end to end, against fixtures. No network, no checkout, no remote.

func passConfig(t *testing.T) loopengine.Config {
	t.Helper()
	return loopengine.Config{
		PoolSize:   1,
		IdlePoll:   time.Millisecond,
		ClaimsDir:  filepath.Join(t.TempDir(), "claims"),
		StaleClaim: deskkit.DefaultStaleClaim,
		Progress:   io_Discard{},
	}
}

func passLoop(t *testing.T, poll string) *ScanLoop {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "target")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return &ScanLoop{
		Root:         root,
		ScanTarget:   "medici-finance/assay",
		WorktreeBase: filepath.Join(base, "worktrees"),
		Scope:        deskkit.ScanRepos(),
		Policy:       CoalescePolicy{},
		Monitor:      func() (*MonitorReport, error) { return ParseMonitorOutput(poll), nil },
		Probe:        probeReturning("ada", time.Time{}, nil, true, nil),
		Emit:         io_Discard{},
		DryRun:       true,
		Now:          func() time.Time { return coalesceNow },
	}
}

// TestDrainPass_MechanicalItemLandsThePlaceholderExit is the end-to-end positive control: an
// inbound issue from a trusted author with no local placeholder drains through the scan-carrier
// lane and leaves by the placeholder exit.
func TestDrainPass_MechanicalItemLandsThePlaceholderExit(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#77 2026-08-24T11:50:00Z\n")
	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s", err, sb.String())
	}
	recs := loop.Ledger().Records()
	if len(recs) != 1 {
		t.Fatalf("ledger = %v", recs)
	}
	if recs[0].Exit != ExitPlaceholder || recs[0].Lane != LaneScanCarrierPR {
		t.Fatalf("record = %+v, want the placeholder exit off the scan-carrier lane", recs[0])
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("leak check = %v, want clean", leaked)
	}
}

// TestDrainPass_JudgmentItemIsEmittedAndParked_NotLandedAndNotLeaked — an item whose disposition is
// a judgment call is emitted for a model tier. It is neither an exit nor a leak, and the pass says
// so rather than folding it into either count.
func TestDrainPass_JudgmentItemIsEmittedAndParked_NotLandedAndNotLeaked(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#77 2026-08-24T11:50:00Z\n")
	// Plant a placeholder so the item classifies as an UPDATE — the judgment lane.
	dir := filepath.Join(loop.Root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-77.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("an emitted judgment item failed the pass: %v", err)
	}
	if got := loop.Parked(); len(got) != 1 || got[0] != "medici-finance/assay#77" {
		t.Fatalf("parked = %v, want the emitted item named", got)
	}
	if len(loop.Ledger().Records()) != 0 {
		t.Fatal("an emitted item was recorded as having left — emitting is not an exit")
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("an awaiting-routing item was reported as a LEAK (%v) — the leak check would cry wolf", leaked)
	}

	renderPassReport(&sb, loop)
	out := sb.String()
	if !strings.Contains(out, "EMITTED — awaiting a routing decision") {
		t.Fatalf("the report does not name the emitted item:\n%s", out)
	}
	if !strings.Contains(out, "NOT recorded as exited") {
		t.Fatalf("the report does not say an emission is not an exit:\n%s", out)
	}
}

// TestDrainPass_JudgmentItemWithAFeederLands — with a model tier's routing fed back, the same item
// leaves by the exit that tier chose.
func TestDrainPass_JudgmentItemWithAFeederLands(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#78 2026-08-24T11:50:00Z\n")
	dir := filepath.Join(loop.Root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-78.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loop.Feeder = func(it loopengine.Item, _ loopengine.Tier, _ LaneOutcome) (loopengine.Result, error) {
		it.Payload["exit"] = string(ExitNeedsDecision)
		return loopengine.Result{Item: it, Verdict: loopengine.VerdictPass, Artifact: "an issue in the decision queue"}, nil
	}
	if err := drainPass(passConfig(t), loop, io_Discard{}); err != nil {
		t.Fatalf("pass errored: %v", err)
	}
	recs := loop.Ledger().Records()
	if len(recs) != 1 || recs[0].Exit != ExitNeedsDecision {
		t.Fatalf("ledger = %v, want the exit the model tier chose", recs)
	}
}

// TestDrainPass_QuarantinedItemIsNeverDispatched — the trust gate at the queueing boundary means an
// untrusted item never reaches a lane at all.
func TestDrainPass_QuarantinedItemIsNeverDispatched(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#79 2026-08-24T11:50:00Z\n")
	loop.Probe = probeReturning("outsider", time.Time{}, nil, true, nil)
	var dispatched int
	loop.Exec = func(string, string, ...string) (string, error) { dispatched++; return "", nil }

	if err := drainPass(passConfig(t), loop, io_Discard{}); err != nil {
		t.Fatalf("pass errored: %v", err)
	}
	if dispatched != 0 {
		t.Fatalf("a quarantined item reached a lane (%d command(s) run)", dispatched)
	}
	if len(loop.Queued()) != 0 {
		t.Fatalf("queued = %v, want nothing queued", loop.Queued())
	}
	if got := len(loop.Admissions()); got != 1 {
		t.Fatalf("admissions = %d — the quarantined item must stay VISIBLE", got)
	}
}

// TestDrainPass_OutOfScopeEventIsNeverQueued — a leftover baseline in the poller's state dir can
// outlive the roster entry that created it. A narrowing must not be silently undone by old state.
func TestDrainPass_OutOfScopeEventIsNeverQueued(t *testing.T) {
	loop := passLoop(t, "INBOUND: someone-else/private-thing#1 2026-08-24T11:50:00Z\n")
	if err := drainPass(passConfig(t), loop, io_Discard{}); err != nil {
		t.Fatalf("pass errored: %v", err)
	}
	if len(loop.Queued()) != 0 {
		t.Fatalf("an out-of-scope event was queued: %v", loop.Queued())
	}
	adm := loop.Admissions()
	if len(adm) != 1 || adm[0].Admitted() {
		t.Fatalf("admissions = %+v, want the event visible and not admitted", adm)
	}
	if !strings.Contains(adm[0].Why, "scan scope") {
		t.Fatalf("the reason does not name the scope: %s", adm[0].Why)
	}
}

// TestDrainPass_EmptyScopeAdmitsNothing — the fail-closed direction. An unset scope is never "all
// repos".
func TestDrainPass_EmptyScopeAdmitsNothing(t *testing.T) {
	in, out := splitByScope([]Inbound{inbound("medici-finance/assay", 1)}, nil)
	if len(in) != 0 || len(out) != 1 {
		t.Fatalf("an empty scope admitted %v", in)
	}
}

// TestDrainPass_ClaimHeldElsewhereIsNotDispatchedAndIsNotALeak — dedupe-at-start is the claim gate's
// guarantee, and an item another dispatcher holds is not this pass's exit to owe.
func TestDrainPass_ClaimHeldElsewhereIsNotDispatchedAndIsNotALeak(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#80 2026-08-24T11:50:00Z\n")
	var dispatched int
	loop.Exec = func(string, string, ...string) (string, error) { dispatched++; return "", nil }

	cfg := passConfig(t)
	if err := os.MkdirAll(cfg.ClaimsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// A live claim held by someone else, taken through the same entry point this pass uses. The key
	// is the SCAN's, not the issue's: what must not run twice at once is a whole-scope scan against
	// one target, and that is exactly what the batch dispatch claims.
	acquired, err := deskkit.Acquire(deskkit.ClaimConfig{ClaimsDir: cfg.ClaimsDir},
		deskkit.Claim{Kind: deskkit.KindRoute, Item: "scan:medici-finance/assay", Owner: "another-session"})
	if err != nil || !acquired {
		t.Fatalf("could not plant the competing claim: %v %v", acquired, err)
	}

	var sb strings.Builder
	if err := drainPass(cfg, loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s", err, sb.String())
	}
	if dispatched != 0 {
		t.Fatalf("an item under a live claim was dispatched (%d command(s))", dispatched)
	}
	if !strings.Contains(sb.String(), "dedup:") {
		t.Fatalf("the pass did not say what it deduplicated against:\n%s", sb.String())
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("an item claimed elsewhere was reported as a leak: %v", leaked)
	}
}

// TestDrainPass_StopFlagHaltsBeforeTheNextItem — a human halting the drain must not wait for the
// pass to finish, and the halt must land on an item boundary rather than mid-action.
func TestDrainPass_StopFlagHaltsBeforeTheNextItem(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#81 2026-08-24T11:50:00Z\n")
	var dispatched int
	loop.Exec = func(string, string, ...string) (string, error) { dispatched++; return "", nil }

	dir, err := deskkit.StateDir()
	if err != nil {
		t.Skipf("no desk state dir in this environment: %v", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	flag := filepath.Join(dir, "STOP")
	if err := os.WriteFile(flag, []byte("halted by the test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(flag) })

	err = drainPass(passConfig(t), loop, io_Discard{})
	if err == nil {
		t.Fatal("the pass ran to completion with a stop flag armed")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitDisabled {
		t.Fatalf("exit = %d, want %d (disabled)", deskkit.ExitCodeOf(err), deskkit.ExitDisabled)
	}
	if dispatched != 0 {
		t.Fatalf("the drain dispatched %d item(s) with a stop flag armed", dispatched)
	}
}

// TestDrainPass_OneFailingDispatchDoesNotStrandTheRestOfTheQueue — one dispatch failing must not
// wedge the drain behind it, and the pass's exit code must still carry the failure.
//
// The two dispatches here are deliberately of DIFFERENT kinds: the mechanical scan batch (made to
// fail at its first git step) and a judgment item routed by a feeder. A pass that abandoned the
// queue on the first failure would land nothing at all.
func TestDrainPass_OneFailingDispatchDoesNotStrandTheRestOfTheQueue(t *testing.T) {
	loop := passLoop(t,
		"INBOUND: medici-finance/assay#90 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#91 2026-08-24T11:51:00Z\n")
	// #91 already has local state, so it classifies as a JUDGMENT item; #90 is the scan batch.
	dir := filepath.Join(loop.Root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-91.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loop.Exec = func(_, name string, args ...string) (string, error) {
		if name == "git" && len(args) > 0 && args[0] == "fetch" {
			return "", os.ErrPermission
		}
		return "", nil
	}
	loop.DryRun = false
	loop.Write = func(string, string) error { return nil }
	loop.Feeder = func(it loopengine.Item, _ loopengine.Tier, _ LaneOutcome) (loopengine.Result, error) {
		it.Payload["exit"] = string(ExitNeedsDecision)
		return loopengine.Result{Item: it, Verdict: loopengine.VerdictPass}, nil
	}

	var sb strings.Builder
	err := drainPass(passConfig(t), loop, &sb)
	if err == nil {
		t.Fatal("a failed lane produced a clean pass")
	}
	if !strings.Contains(sb.String(), "dispatch-error") {
		t.Fatalf("the pass did not name the failing dispatch:\n%s", sb.String())
	}
	recs := loop.Ledger().Records()
	if len(recs) != 1 || recs[0].ItemID != "medici-finance/assay#91" {
		t.Fatalf("the judgment item did not drain behind the failing scan: %v", recs)
	}
}
