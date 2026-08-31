package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// queuetruth_part2_test.go — FAIL-FIRST coverage for the three remaining `verifyloop plan`
// queue-truthfulness gaps left after the first pass:
//
//  1. a longitudinal brief whose Verify exit criteria are an observation/accrual window, when it
//     carries NO explicit `blocked-until:` marker, must still be DEFERRED, not counted dispatchable;
//  2. a cluster / online-lane brief whose Verify rows run against a live cluster, when it carries
//     NO explicit `verify-lane:` marker, must be bucketed AWAITING-ONLINE-LANE, not dispatchable;
//  3. a `gate: human` brief must land in AWAITING-HUMAN even when it is ALSO `irreversible: yes`
//     (which TierPolicy routes to TierLocal for evidence) — the direct gate:human signal on the
//     brief itself, not only the tier the risk-router computed, decides the human bucket.
//
// Each case starts from a marker-less brief the way a real board carries it, so before the fix the
// item lands in DISPATCH (the over-report the plan summary hid) and after the fix it is bucketed.

// TestGap3_GateHumanIrreversibleIsAwaitingHuman is the security-adjacent fail-open fix: a
// gate:human + irreversible:yes brief was dispatched because irreversible:yes routes to TierLocal
// (dispatched-for-evidence) BEFORE the risk-router's gate:human → TierHuman branch runs, so the
// awaiting-human bucket — keyed only on the computed tier — never saw it. The bucketer must read
// the brief's own gate directly.
func TestGap3_GateHumanIrreversibleIsAwaitingHuman(t *testing.T) {
	v := &VerifyLoop{}
	it := loopengine.Item{
		ID:   "example-stream/01",
		Gate: "human",
		Risk: loopengine.RiskFlags{Irreversible: true},
	}
	tier, err := v.TierPolicy(it)
	if err != nil {
		t.Fatalf("TierPolicy: %v", err)
	}
	// Precondition anchor: irreversible routes to TierLocal, which is exactly why the
	// tier-only awaiting-human check missed it. If this ever stops being TierLocal the test's
	// premise changed and it should be revisited.
	if tier != loopengine.TierLocal {
		t.Fatalf("premise: gate:human+irreversible tier = %v; want TierLocal (irreversible dispatched for evidence)", tier)
	}
	disp, _ := classifyItem(it, tier)
	if disp != dispAwaitingHuman {
		t.Fatalf("gate:human+irreversible classified %v; want awaiting-human (a model may not flip a gate:human brief)", disp)
	}
}

// TestGap3_PlainIrreversibleStaysDispatched guards the boundary: an irreversible-but-NOT-gate:human
// brief must STAY dispatched (TierLocal, dispatched-for-evidence; Land writes evidence with no flip
// + a human checkpoint). The gate:human fix must not fold plain-irreversible out of the queue and
// lose its mechanical verification.
func TestGap3_PlainIrreversibleStaysDispatched(t *testing.T) {
	v := &VerifyLoop{}
	it := loopengine.Item{
		ID:   "example-stream/02",
		Gate: "model",
		Risk: loopengine.RiskFlags{Irreversible: true},
	}
	tier, _ := v.TierPolicy(it)
	disp, _ := classifyItem(it, tier)
	if disp != dispDispatch {
		t.Fatalf("plain irreversible (gate:model) classified %v; want dispatch (dispatched-for-evidence, human flip)", disp)
	}
}

// TestGap2_ClusterVerifyRowsBucketWithoutMarker: a brief whose Verify commands run against a live
// cluster (kubectl / live cluster), carrying NO `verify-lane:` frontmatter marker, must bucket as
// awaiting-online-lane after the board read. Before the fix, scanAwaiting left verify_lane empty
// and the item was dispatchable.
func TestGap2_ClusterVerifyRowsBucketWithoutMarker(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | cluster-lane | 0 | S | implemented | — | — |\n"
	brief := "---\nbrief: cluster-lane\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n" +
		"# Brief\n\n## Verify\n\n| # | Command | Expect |\n" +
		"| 1 | `kubectl -n prod get pods` against the live cluster | pod Running |\n\n" +
		"## Evidence\n<!-- appended at verification time -->\n"
	writeFixtureStream(t, root, "example-stream", table, map[string]string{"01": brief})

	it := selectOne(t, root, "example-stream/01")
	// The board read derives the online lane from the Verify content, so classifyItem sees it.
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	disp, reason := classifyItem(it, tier)
	if disp != dispAwaitingOnlineLane {
		t.Fatalf("cluster Verify rows (no marker) classified %v; want awaiting-online-lane", disp)
	}
	if reason == "" {
		t.Fatalf("awaiting-online-lane member should carry a lane reason")
	}
}

// TestGap1_LongitudinalWindowDefersWithoutMarker: a brief whose Verify exit criteria are an
// observation/accrual window, carrying NO `blocked-until:` marker, must be DEFERRED after the board
// read. Before the fix, blocked_until was empty and the item was dispatchable, re-verifying to the
// identical "window not accrued" non-verdict every run.
func TestGap1_LongitudinalWindowDefersWithoutMarker(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | longitudinal | 0 | S | implemented | — | — |\n"
	brief := "---\nbrief: longitudinal\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n---\n\n" +
		"# Brief\n\n## Verify\n\n| # | Command | Expect |\n" +
		"| 1 | observe the shadow window and confirm it has accrued | shadow window elapsed |\n\n" +
		"## Evidence\n<!-- appended at verification time -->\n"
	writeFixtureStream(t, root, "example-stream", table, map[string]string{"01": brief})

	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	disp, reason := classifyItem(it, tier)
	if disp != dispDeferred {
		t.Fatalf("longitudinal window brief (no marker) classified %v; want deferred", disp)
	}
	if reason == "" {
		t.Fatalf("deferred member should carry a why-it-waits reason")
	}
}

// TestGap12_ExplicitMarkersTakePrecedenceOverDerivation: an author's explicit marker still wins —
// its exact reason string reaches the plan, not a derived one — so the content derivation is an
// additive safety net, never an override of the authored intent.
func TestGap12_ExplicitMarkersTakePrecedenceOverDerivation(t *testing.T) {
	root := t.TempDir()
	table := "| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | authored | 0 | S | implemented | — | — |\n"
	// Carries BOTH an explicit blocked-until marker AND cluster Verify content; the explicit
	// marker's reason must be the one surfaced (deferred beats online-lane by precedence, and the
	// reason is the authored condition).
	brief := "---\nbrief: authored\ngate: model\n" +
		"risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\neffort: S\n" +
		"blocked-until: 2026-12-01 (authored condition)\n---\n\n" +
		"# Brief\n\n## Verify\n\n| # | Command | Expect |\n" +
		"| 1 | `kubectl get pods` against the live cluster | ok |\n\n" +
		"## Evidence\n<!-- appended at verification time -->\n"
	writeFixtureStream(t, root, "example-stream", table, map[string]string{"01": brief})

	it := selectOne(t, root, "example-stream/01")
	tier, _ := (&VerifyLoop{}).TierPolicy(it)
	disp, reason := classifyItem(it, tier)
	if disp != dispDeferred {
		t.Fatalf("explicit blocked-until classified %v; want deferred", disp)
	}
	if !strings.Contains(reason, "authored condition") {
		t.Fatalf("explicit marker reason should win, got %q", reason)
	}
}

// selectOne runs the board read and returns the one item with the given ID.
func selectOne(t *testing.T, root, id string) loopengine.Item {
	t.Helper()
	items, err := (&VerifyLoop{Root: root, TargetSHA: "abc"}).SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("item %q not found in queue (%d items)", id, len(items))
	return loopengine.Item{}
}
