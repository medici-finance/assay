package main

// repair_test.go — the durable REPAIR OBLIGATION acceptance tests (example-stream/17).
//
// These are the brief's Verify rows: they exercise the PRODUCTION repair source (repair.go), the
// deskkit obligation lifecycle it drives, and the dispatch prompt that carries an obligation onto a
// worker. Each uses injected forge/clock state — no production service, no network — matching the
// offline reference build's posture.

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// at is a fixed clock for deterministic lease evaluation.
func at(t *testing.T, rfc3339 string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t.Fatalf("bad test time %q: %v", rfc3339, err)
	}
	return tm
}

// writeObligationLog appends obligations to a root's docs/streams/repair-obligations.jsonl, one
// JSONL line each, exactly as the landing sink would.
func writeObligationLog(t *testing.T, root string, obs ...deskkit.RepairObligation) {
	t.Helper()
	var b strings.Builder
	for _, o := range obs {
		line, err := o.MarshalLine()
		if err != nil {
			t.Fatalf("marshal obligation: %v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	writeFile(t, root, "docs/streams/"+repairSidecarName, b.String())
}

// emptyBoard wires a FanoutLoop whose Next-up / rework / orphan sources are all empty, so a
// SelectQueue asserts ONLY the repair lane.
func repairOnlyLoop(t *testing.T, obligs []deskkit.RepairObligation, now time.Time) *FanoutLoop {
	t.Helper()
	return &FanoutLoop{
		Root:      t.TempDir(),
		TargetSHA: "deadbeef",
		Board:     func() ([]BoardRow, error) { return nil, nil },
		Rework:    func() ([]BoardRow, error) { return nil, nil },
		Orphans:   func() ([]OrphanPR, error) { return nil, nil },
		RepairObligations: func() ([]deskkit.RepairObligation, error) {
			return deskkit.ReconcileObligations(obligs), nil
		},
		Now: func() time.Time { return now },
	}
}

// --- Row 1 ---------------------------------------------------------------------------------------

// TestRepairObligationFailToWorker: one verifier failure creates exactly one rework item carrying
// its reproduction and target repository.
func TestRepairObligationFailToWorker(t *testing.T) {
	setupDeskHome(t)
	now := at(t, "2026-09-20T12:00:00Z")

	o := deskkit.NewRepairObligation(
		"medici-finance/assay", "example-stream/17", "receipt/abc",
		[]int{2, 1}, deskkit.BlockerImplementation,
		"`go test ./cmd/fanoutloop/ -run ^TestX$` exited 1: assertion failed",
		"`go test ./cmd/fanoutloop/ -run ^TestX$` exits 0", "",
		now.Format(time.RFC3339))

	// A duplicate delivery of the SAME failure must still yield exactly one item.
	f := repairOnlyLoop(t, []deskkit.RepairObligation{o, o}, now)

	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	repairItems := itemsOfKind(items, kindRework)
	if len(repairItems) != 1 {
		t.Fatalf("want exactly one repair rework item, got %d (%+v)", len(repairItems), repairItems)
	}
	it := repairItems[0]
	if it.ID != o.ID {
		t.Errorf("item ID = %q, want the immutable obligation id %q", it.ID, o.ID)
	}
	if got := it.Payload["repo"]; got != "medici-finance/assay" {
		t.Errorf("target repo = %q, want medici-finance/assay", got)
	}
	if repro := it.Payload[payloadReproduction]; !strings.Contains(repro, "assertion failed") {
		t.Errorf("reproduction not carried on the item: %q", repro)
	}
	if it.Payload[payloadRows] != "1,2" {
		t.Errorf("failing rows = %q, want the sorted set 1,2", it.Payload[payloadRows])
	}
	// The prompt a dispatch would emit names the failure, not a fresh brief.
	prompt := renderDispatchPrompt(it, loopengine.TierSession)
	if !strings.Contains(prompt, "REPAIR OBLIGATION") || !strings.Contains(prompt, "Reproduction:") {
		t.Errorf("dispatch prompt missing repair framing:\n%s", prompt)
	}
}

// --- Row 2 ---------------------------------------------------------------------------------------

// TestRepairObligationRestartAndDuplicate: duplicate delivery and a lost response reconcile to ONE
// obligation, and a replacement worker can resume after a DEAD claim (an expired lease returns the
// SAME obligation to the queue, never a second).
func TestRepairObligationRestartAndDuplicate(t *testing.T) {
	setupDeskHome(t)

	base := deskkit.NewRepairObligation(
		"medici-finance/assay", "example-stream/17", "receipt/xyz",
		[]int{1}, deskkit.BlockerImplementation, "repro", "expected", "",
		at(t, "2026-09-20T09:00:00Z").Format(time.RFC3339))

	// Worker A claims it, stamping a lease at 09:05.
	claimed := base.Claim("session-A", at(t, "2026-09-20T09:05:00Z"))
	// The append-only log holds: initial post, the claim, and a LOST-ACK re-post of the initial
	// (idempotent duplicate). Reconciliation must fold all three to ONE obligation at the
	// furthest-progressed state (repairing), never three.
	log := []deskkit.RepairObligation{base, claimed, base}

	reconciled := deskkit.ReconcileObligations(log)
	if len(reconciled) != 1 {
		t.Fatalf("reconcile of duplicate+lost-ack log = %d obligations, want exactly 1", len(reconciled))
	}
	if reconciled[0].State != deskkit.RepairRepairing {
		t.Fatalf("reconciled state = %q, want repairing (furthest progressed)", reconciled[0].State)
	}

	// A LIVE lease (evaluated 20 min after the claim, well under the TTL): NOT assignable — the
	// live worker is not stolen from.
	live := repairOnlyLoop(t, log, at(t, "2026-09-20T09:25:00Z"))
	if got := itemsOfKind(mustSelect(t, live), kindRework); len(got) != 0 {
		t.Fatalf("a live-lease obligation must not be assignable; got %d item(s)", len(got))
	}

	// A DEAD lease (evaluated well beyond the TTL): the SAME obligation returns to the queue for a
	// replacement worker — exactly one item, same immutable id.
	dead := repairOnlyLoop(t, log, at(t, "2026-09-20T11:00:00Z"))
	got := itemsOfKind(mustSelect(t, dead), kindRework)
	if len(got) != 1 {
		t.Fatalf("a dead-lease obligation must be re-assignable exactly once; got %d item(s)", len(got))
	}
	if got[0].ID != base.ID {
		t.Errorf("replacement item id = %q, want the SAME obligation id %q (no duplicate)", got[0].ID, base.ID)
	}
}

// --- Row 3 ---------------------------------------------------------------------------------------

// TestRepairObligationMergeIsNotResolved: a repair merge WAKES independent verification but does not
// resolve; a same-actor or wrong-revision pass cannot resolve; only a valid independent pass at the
// repaired revision resolves.
func TestRepairObligationMergeIsNotResolved(t *testing.T) {
	o := deskkit.NewRepairObligation(
		"medici-finance/assay", "example-stream/17", "receipt/r3",
		[]int{1}, deskkit.BlockerImplementation, "repro", "expected", "",
		at(t, "2026-09-20T09:00:00Z").Format(time.RFC3339))
	o = o.Claim("session-A", at(t, "2026-09-20T09:05:00Z"))

	// A worker assertion (repair PR opened, then approved) never reaches resolved.
	o = o.WithRepairPR("medici-finance/assay#5", "", "assay-worker-app[bot]")
	if o.IsResolved() {
		t.Fatal("opening a repair PR must not resolve the obligation")
	}
	o = o.MarkAwaitingMerge()
	if o.IsResolved() {
		t.Fatal("approval must not resolve the obligation")
	}

	// The merge WAKES reverification — awaiting-reverification, NOT resolved.
	merged := applyMerge(t, o, "cafef00d", "assay-worker-app[bot]")
	if merged.State != deskkit.RepairAwaitingReverification {
		t.Fatalf("post-merge state = %q, want awaiting-reverification", merged.State)
	}
	if merged.IsResolved() {
		t.Fatal("a merge ALONE must not resolve an implementation obligation")
	}

	// Same-actor pass cannot resolve (self-certification).
	if _, ok, reason := merged.Resolve("assay-worker-app[bot]", "cafef00d"); ok {
		t.Fatalf("same-actor pass resolved the obligation, must not: %s", reason)
	}
	// Wrong-revision pass cannot resolve (did not observe the fix).
	if _, ok, reason := merged.Resolve("assay-verifier-app[bot]", "0ldstale"); ok {
		t.Fatalf("wrong-revision pass resolved the obligation, must not: %s", reason)
	}
	// Valid INDEPENDENT pass at the repaired revision resolves.
	resolved, ok, reason := merged.Resolve("assay-verifier-app[bot]", "cafef00d")
	if !ok {
		t.Fatalf("a valid independent pass at the repaired revision must resolve; got: %s", reason)
	}
	if resolved.State != deskkit.RepairResolved || resolved.ResolvedByVerifier != "assay-verifier-app[bot]" {
		t.Fatalf("resolved obligation not stamped correctly: %+v", resolved)
	}

	// A resolved obligation is no longer assignable — it never re-enters the queue.
	f := repairOnlyLoop(t, []deskkit.RepairObligation{resolved}, at(t, "2026-09-25T00:00:00Z"))
	if got := itemsOfKind(mustSelect(t, f), kindRework); len(got) != 0 {
		t.Fatalf("a resolved obligation must not be dispatchable; got %d item(s)", len(got))
	}
}

// --- Row 4 ---------------------------------------------------------------------------------------

// TestRepairObligationCrossRepoFollowUp: failed work delivered in a sibling with a MERGED original
// PR produces the correct deliverable repo, a fresh follow-up branch (never the merged branch), and
// the linked repair — with no duplicate of the original work.
func TestRepairObligationCrossRepoFollowUp(t *testing.T) {
	setupDeskHome(t)
	now := at(t, "2026-09-20T12:00:00Z")

	// The tracking root (where the projection lives) is one repo; the deliverable is a SIBLING.
	const trackingRepo = "example-org/tracker"
	const deliverableRepo = "example-org/sibling"
	trackingRoot := t.TempDir()

	o := deskkit.NewRepairObligation(
		deliverableRepo, // deliverable SIBLING repo, distinct from the tracking repo
		"example-stream/17", "receipt/x-repo", []int{1},
		deskkit.BlockerImplementation, "repro", "expected", "",
		now.Format(time.RFC3339))
	o.OriginalPR = deliverableRepo + "#42"
	o.OriginalMerged = true
	writeObligationLog(t, trackingRoot, o, o) // duplicate delivery — must fold to one

	// Read ACROSS the configured roots — the cross-root collector folds the duplicate to one.
	roots := []deskkit.RootConfig{{Repo: trackingRepo, Path: trackingRoot}}
	obligs, err := collectRepairObligations(roots)
	if err != nil {
		t.Fatalf("collectRepairObligations: %v", err)
	}
	if len(obligs) != 1 {
		t.Fatalf("cross-root collect folded to %d obligations, want 1 (no duplicate original work)", len(obligs))
	}

	items := assignableRepairItems(obligs, roots, "sha", now, repairLeaseTTL)
	if len(items) != 1 {
		t.Fatalf("want exactly one repair item, got %d", len(items))
	}
	it := items[0]

	// Correct DELIVERABLE repo, resolved from the obligation (not the tracking repo).
	if got := it.Payload["repo"]; got != deliverableRepo {
		t.Errorf("deliverable repo = %q, want the sibling %q", got, deliverableRepo)
	}
	// A merged original demands a FRESH follow-up branch, never the merged branch.
	fb := it.Payload[payloadFollowUpBranch]
	if fb == "" || !strings.Contains(fb, "followup") {
		t.Errorf("follow-up branch = %q, want a fresh repair/...-followup-N branch", fb)
	}
	if it.Payload[payloadOriginalMerged] != "true" {
		t.Errorf("original_merged = %q, want true", it.Payload[payloadOriginalMerged])
	}
	// The emitted prompt forbids resuming the merged branch and names the follow-up.
	prompt := renderDispatchPrompt(it, loopengine.TierSession)
	if !strings.Contains(prompt, "Do NOT resume or push that branch") || !strings.Contains(prompt, fb) {
		t.Errorf("prompt does not forbid resuming the merged branch / name the follow-up:\n%s", prompt)
	}
	if !strings.Contains(prompt, deliverableRepo) {
		t.Errorf("prompt does not name the deliverable repo:\n%s", prompt)
	}
}

// --- helpers -------------------------------------------------------------------------------------

func itemsOfKind(items []loopengine.Item, kind string) []loopengine.Item {
	var out []loopengine.Item
	for _, it := range items {
		if it.Payload["kind"] == kind {
			out = append(out, it)
		}
	}
	return out
}

func mustSelect(t *testing.T, f *FanoutLoop) []loopengine.Item {
	t.Helper()
	items, err := f.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	return items
}

// applyMerge drives the forge-transition reconciliation for a merge observation (task 4) and
// asserts it changed state.
func applyMerge(t *testing.T, o deskkit.RepairObligation, mergedSHA, repairedBy string) deskkit.RepairObligation {
	t.Helper()
	out, changed := applyForgeTransition(o, RepairForgeObservation{
		Merged: true, MergedSHA: mergedSHA, RepairedBy: repairedBy, RepairPR: o.RepairPR,
	})
	if !changed {
		t.Fatalf("merge observation did not advance obligation state (was %q)", o.State)
	}
	return out
}
