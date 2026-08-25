package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// batch_test.go — the whole-scope scan is dispatched ONCE per pass.
//
// The no-op Exec used elsewhere in this package is the wrong instrument for this property: it
// returns success for everything, so a second scan dispatch colliding on the branch and the PR
// looks exactly like a first one succeeding. These tests run against a git/gh model that ENFORCES
// the two semantics the collision turned on:
//
//   - `git worktree add -b <branch>` FAILS when the branch already exists;
//   - `git worktree remove` removes the worktree and does NOT delete the branch — which is why a
//     collision survived into the next pass as well as within one.

// fakeGit is a minimal state machine over the commands the scan-carrier lane issues. It is
// deliberately strict: an unexpected shape is an error, not a silent success, because a lane whose
// arguments drift would otherwise keep passing this test.
type fakeGit struct {
	mu         sync.Mutex
	branches   map[string]bool
	worktrees  map[string]bool
	scans      int
	prsCreated int
	prsUpdated int
	prEdits    int
	commands   []string
}

func newFakeGit() *fakeGit {
	return &fakeGit{branches: map[string]bool{}, worktrees: map[string]bool{}}
}

func (g *fakeGit) exec(dir, name string, args ...string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.commands = append(g.commands, name+" "+strings.Join(args, " "))

	switch name {
	case "git":
		switch {
		case len(args) >= 5 && args[0] == "worktree" && args[1] == "add":
			// worktree add <path> -b <branch> <base>
			path, branch := args[2], args[4]
			if g.branches[branch] {
				return "", fmt.Errorf("fatal: a branch named '%s' already exists", branch)
			}
			if g.worktrees[path] {
				return "", fmt.Errorf("fatal: '%s' already exists", path)
			}
			g.branches[branch] = true
			g.worktrees[path] = true
		case len(args) >= 3 && args[0] == "worktree" && args[1] == "remove":
			// The load-bearing half: removing a worktree does NOT delete its branch.
			delete(g.worktrees, args[2])
		}
	case "statusgen":
		g.scans++
	case "deskscanbody":
		if args[0] == "emit" {
			return "intake scan: 3 created, 0 retired", nil
		}
	case "deskpr":
		switch args[0] {
		case "create":
			g.prsCreated++
		case "update":
			g.prsUpdated++
		}
	case "gh":
		if len(args) >= 2 && args[0] == "pr" && args[1] == "edit" {
			g.prEdits++
		}
	}
	return "", nil
}

func (g *fakeGit) ran(substr string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.commands {
		if strings.Contains(c, substr) {
			n++
		}
	}
	return n
}

// TestFakeGit_EnforcesTheCollisionSemantics is the control ON the control. A fixture that quietly
// accepts a colliding `worktree add -b` would make every test in this file vacuous — it would pass
// just as happily against the per-item dispatch it exists to rule out. So the model's two
// load-bearing semantics are asserted directly, against the exact argument shapes the lane emits.
func TestFakeGit_EnforcesTheCollisionSemantics(t *testing.T) {
	g := newFakeGit()
	if _, err := g.exec("/root", "git", "worktree", "add", "/wt", "-b", "chore/x", "origin/main"); err != nil {
		t.Fatalf("first worktree add errored: %v", err)
	}
	if _, err := g.exec("/root", "git", "worktree", "add", "/wt2", "-b", "chore/x", "origin/main"); err == nil {
		t.Fatal("the model accepted a SECOND worktree on an existing branch — every collision test here would be vacuous")
	}
	// Removing the worktree must NOT free the branch: that is precisely why a minute-granular name
	// collided across passes as well as within one.
	if _, err := g.exec("/root", "git", "worktree", "remove", "/wt"); err != nil {
		t.Fatalf("worktree remove errored: %v", err)
	}
	if _, err := g.exec("/root", "git", "worktree", "add", "/wt3", "-b", "chore/x", "origin/main"); err == nil {
		t.Fatal("the model let `worktree remove` delete a branch — real git does not, and the cross-pass collision depends on it")
	}
	// A second worktree on a DIFFERENT branch is fine, so the model is not simply refusing everything.
	if _, err := g.exec("/root", "git", "worktree", "add", "/wt4", "-b", "chore/y", "origin/main"); err != nil {
		t.Fatalf("a distinct branch was refused: %v", err)
	}
}

func batchLoop(t *testing.T, poll string, g *fakeGit) *ScanLoop {
	t.Helper()
	loop := passLoop(t, poll)
	loop.DryRun = false
	loop.Exec = g.exec
	loop.Write = func(string, string) error { return nil }
	return loop
}

// TestBatch_ManyNewIssuesYieldExactlyOneScanAndOnePR is the finding's direct control. Three inbound
// new issues in one pass used to produce three scan dispatches: the first opened a PR, the second
// and third failed `worktree add -b` on the first one's branch, each failure was counted as a
// dispatch error, and each item was then reported as having leaked out of the front door.
func TestBatch_ManyNewIssuesYieldExactlyOneScanAndOnePR(t *testing.T) {
	g := newFakeGit()
	loop := batchLoop(t,
		"INBOUND: medici-finance/assay#101 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#102 2026-08-24T11:51:00Z\n"+
			"INBOUND: example-org/tracker#7 2026-08-24T11:52:00Z\n", g)

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s\n%s", err, sb.String(), strings.Join(g.commands, "\n"))
	}
	if strings.Contains(sb.String(), "dispatch-error") {
		t.Fatalf("the pass produced a dispatch error:\n%s", sb.String())
	}
	if g.scans != 1 {
		t.Fatalf("the whole-scope scan ran %d times for one pass, want exactly 1", g.scans)
	}
	if g.prsCreated != 1 {
		t.Fatalf("%d scan PR(s) were created for one pass, want exactly 1", g.prsCreated)
	}
	if got := g.ran("worktree add"); got != 1 {
		t.Fatalf("%d worktrees were cut for one pass, want exactly 1", got)
	}

	// Every inbound item still leaves by its OWN exit — batching the dispatch does not batch the
	// ledger, because it is the inbound items that have to leave the front door.
	recs := loop.Ledger().Records()
	if len(recs) != 3 {
		t.Fatalf("ledger = %v, want one placeholder exit per inbound item", recs)
	}
	for _, r := range recs {
		if r.Exit != ExitPlaceholder {
			t.Fatalf("record %+v, want the placeholder exit", r)
		}
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("leak check = %v, want clean", leaked)
	}
	if !strings.Contains(sb.String(), "leak check: CLEAN") {
		// The report is checked too: the false "front door leaked" line is what an operator saw.
		renderPassReport(&sb, loop)
		if !strings.Contains(sb.String(), "leak check: CLEAN") {
			t.Fatalf("the pass report does not say the leak check was clean:\n%s", sb.String())
		}
	}
}

// TestBatch_TwoPassesAtTheSameInstantDoNotCollide — the cross-pass half of the same defect. The
// branch name used to be minute-granular, and `git worktree remove` does not delete a branch, so a
// second pass inside the same minute failed on the first pass's leftover branch.
func TestBatch_TwoPassesAtTheSameInstantDoNotCollide(t *testing.T) {
	g := newFakeGit()
	poll := "INBOUND: medici-finance/assay#111 2026-08-24T11:50:00Z\n"

	for pass := 1; pass <= 2; pass++ {
		loop := batchLoop(t, poll, g)
		loop.Now = func() time.Time { return coalesceNow } // the SAME frozen instant for both passes
		var sb strings.Builder
		if err := drainPass(passConfig(t), loop, &sb); err != nil {
			t.Fatalf("pass %d errored: %v\n%s", pass, err, sb.String())
		}
		if strings.Contains(sb.String(), "dispatch-error") {
			t.Fatalf("pass %d produced a dispatch error — the branch collided:\n%s", pass, sb.String())
		}
	}
	if g.scans != 2 || g.prsCreated != 2 {
		t.Fatalf("scans=%d prs=%d, want one of each per pass", g.scans, g.prsCreated)
	}
	if len(g.branches) != 2 {
		t.Fatalf("%d distinct branches for two passes at the same instant, want 2", len(g.branches))
	}
}

// TestBatch_CoalescedPassUpdatesTheOpenPRRatherThanOpeningASecond — with an open scan PR inside the
// window, one pass produces one follow-up push and one regeneration, and opens NO new PR.
func TestBatch_CoalescedPassUpdatesTheOpenPRRatherThanOpeningASecond(t *testing.T) {
	g := newFakeGit()
	loop := batchLoop(t,
		"INBOUND: medici-finance/assay#121 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#122 2026-08-24T11:51:00Z\n", g)
	open := openPR(3 * time.Minute)
	loop.OpenPR = func() (*OpenScanPR, error) { return open, nil }

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s\n%s", err, sb.String(), strings.Join(g.commands, "\n"))
	}
	if g.prsCreated != 0 {
		t.Fatalf("a coalescing pass opened %d new PR(s), want 0", g.prsCreated)
	}
	if g.prsUpdated != 1 || g.prEdits != 1 {
		t.Fatalf("pushes=%d body-regenerations=%d, want exactly one of each", g.prsUpdated, g.prEdits)
	}
	if len(loop.Ledger().Records()) != 2 {
		t.Fatalf("ledger = %v, want one exit per inbound item", loop.Ledger().Records())
	}
}

// TestBatch_UnwritableScanTargetProducesNoDispatchErrorAndNoLeak is the drain-level shape of the
// classification fix. The condition is STANDING — a repo does not become writable by being retried
// — so a lane-level refusal would have produced an error and a false leak flag on every pass
// forever. Classification routes it to the judgment lane instead, and the pass is clean.
func TestBatch_UnwritableScanTargetProducesNoDispatchErrorAndNoLeak(t *testing.T) {
	g := newFakeGit()
	loop := batchLoop(t, "INBOUND: medici-finance/assay#131 2026-08-24T11:50:00Z\n", g)
	loop.ScanTarget = readScopeOnlyRepo(t)

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s", err, sb.String())
	}
	if strings.Contains(sb.String(), "dispatch-error") {
		t.Fatalf("an unwritable scan target produced a dispatch error:\n%s", sb.String())
	}
	if g.scans != 0 || g.prsCreated != 0 {
		t.Fatalf("scans=%d prs=%d — nothing may be written when the scan target is outside the write boundary", g.scans, g.prsCreated)
	}
	if got := loop.Parked(); len(got) != 1 || got[0] != "medici-finance/assay#131" {
		t.Fatalf("parked = %v, want the item emitted for a model tier", got)
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("an item routed at classification time was flagged as a LEAK: %v", leaked)
	}
}

// TestBatch_ReadScopeOnlyIssueDrainsThroughTheScanTarget — the positive counterpart of the finding:
// an issue on a repo the desk may READ but not WRITE is ordinary work, because its placeholder lands
// in the scan target, not in its own repo.
func TestBatch_ReadScopeOnlyIssueDrainsThroughTheScanTarget(t *testing.T) {
	readOnly := readScopeOnlyRepo(t)
	g := newFakeGit()
	loop := batchLoop(t, "INBOUND: "+readOnly+"#141 2026-08-24T11:50:00Z\n", g)

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("pass errored: %v\n%s", err, sb.String())
	}
	recs := loop.Ledger().Records()
	if len(recs) != 1 || recs[0].Exit != ExitPlaceholder || recs[0].ItemID != readOnly+"#141" {
		t.Fatalf("ledger = %v, want a placeholder exit for the read-scope-only item", recs)
	}
	if g.prsCreated != 1 {
		t.Fatalf("prs=%d, want the delta carried to the scan target", g.prsCreated)
	}
}

// TestBatch_MixedPassBatchesTheMechanicalHalfAndEmitsTheRest — the two halves of a realistic pass
// do not interfere: one scan dispatch for the new issues, one emission per update.
func TestBatch_MixedPassBatchesTheMechanicalHalfAndEmitsTheRest(t *testing.T) {
	g := newFakeGit()
	loop := batchLoop(t,
		"INBOUND: medici-finance/assay#151 2026-08-24T11:50:00Z\n"+
			"INBOUND: medici-finance/assay#152 2026-08-24T11:51:00Z\n"+
			"INBOUND: medici-finance/assay#153 2026-08-24T11:52:00Z\n", g)
	dir := filepath.Join(loop.Root, deskkit.ScanDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issue-153.md"), []byte("---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loop.Feeder = func(it loopengine.Item, _ loopengine.Tier, _ LaneOutcome) (loopengine.Result, error) {
		it.Payload["exit"] = string(ExitNeedsDecision)
		return loopengine.Result{Item: it, Verdict: loopengine.VerdictPass}, nil
	}

	if err := drainPass(passConfig(t), loop, io_Discard{}); err != nil {
		t.Fatalf("pass errored: %v", err)
	}
	if g.scans != 1 || g.prsCreated != 1 {
		t.Fatalf("scans=%d prs=%d, want exactly one of each for the mechanical half", g.scans, g.prsCreated)
	}
	counts := loop.Ledger().CountByExit()
	if counts[ExitPlaceholder] != 2 || counts[ExitNeedsDecision] != 1 {
		t.Fatalf("exits = %v, want 2 placeholder + 1 needs-decision", counts)
	}
}

// TestRun_TrustBlindnessReachesTheExitCode — an item whose trust could not be READ is neither
// admitted nor quarantined. Exiting 0 on such a pass tells anything scripting this drain that the
// inbound surface was clean when in fact it was never evaluated.
func TestRun_TrustBlindnessReachesTheExitCode(t *testing.T) {
	err := cmdRun([]string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", armedStateDir(t),
		"--offline", // no trust probe is wired at all
		"--inbound", writeTemp(t, "poll.txt", "INBOUND: medici-finance/assay#161 2026-08-24T11:50:00Z\n"),
		"--now", "2026-08-24T12:00:00Z",
	}, &strings.Builder{})
	if err == nil {
		t.Fatal("a pass whose trust gate was never evaluated exited 0 — that reads as a clean surface")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
	if !strings.Contains(err.Error(), "trust gate could not be evaluated") {
		t.Fatalf("the refusal does not name the blindness: %v", err)
	}
}

// TestCouldNotCheck_NamesOnlyTheUnreadItems — the counterpart control: a fully-read pass reports no
// blindness, so the check above cannot be passing for the wrong reason.
func TestCouldNotCheck_NamesOnlyTheUnreadItems(t *testing.T) {
	adm := []Admission{
		{Item: inbound("example-org/tracker", 1), State: AdmissionAdmitted},
		{Item: inbound("example-org/tracker", 2), State: AdmissionQuarantined},
		{Item: inbound("example-org/tracker", 3), State: AdmissionCouldNotCheck},
	}
	got := couldNotCheck(adm)
	if len(got) != 1 || got[0] != "example-org/tracker#3" {
		t.Fatalf("couldNotCheck = %v, want only the unread item", got)
	}
	if n := couldNotCheck(adm[:2]); len(n) != 0 {
		t.Fatalf("a fully-read pass reported blindness: %v", n)
	}
}
