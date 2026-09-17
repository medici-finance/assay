package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eligibility_test.go — the eligibility evaluator's own tests
// (graph-execution/01). Top-level functions are named TestEligibility* so
// Verify row 1's `-run 'TestEligibility'` selector counts them all.

// eligibilityFixtureRoot copies the shared milestone fixture tree
// (testdata/eligibility) into a fresh temp root and loads its hydrated
// streams — the same checkBriefFiles path (Gates/Feathers wiring included)
// that nextUp()/eligibilityForStreams read in production.
func eligibilityFixtureRoot(t *testing.T) (root string, streams []*Stream) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/eligibility")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, streams
}

// mutateBrief02 copies the fixture into a fresh root, replaces `old` with
// `new` (exactly once) in example-a's brief 02 file, and returns the
// reloaded hydrated streams. It fails the test outright if `old` is not
// found, so a fixture edit that silently stops matching cannot pass by
// accident.
func mutateBrief02(t *testing.T, old, new string) []*Stream {
	t.Helper()
	root, _ := eligibilityFixtureRoot(t)
	p := filepath.Join(root, "docs", "streams", "example-a", "brief-02-b.md")
	orig, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(orig), old, new, 1)
	if mutated == string(orig) {
		t.Fatalf("mutation %q -> %q did not match any text in %s — the fixture has drifted from this test's expectation", old, new, p)
	}
	if err := os.WriteFile(p, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadHydratedStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

// TestEligibilityDeclarationChangesDispatch is brief-01's milestone test
// (Task item 5, Verify row 2): editing ONE gates: on: line — nothing else —
// flips example-a/02 from held to eligible, and Next-up agrees. The same
// evaluator (evaluateEligibility) and the same nextUp() run against both
// trees; no code path here is keyed on the stream name "example-a".
func TestEligibilityDeclarationChangesDispatch(t *testing.T) {
	// Run 1: the fixture as authored — 02's gates: targets example-a/01,
	// which is todo. HELD.
	_, streamsHeld := eligibilityFixtureRoot(t)
	eligHeld := eligibilityForStreams(streamsHeld)
	ev, ok := eligHeld["example-a/02"]
	if !ok {
		t.Fatalf("example-a/02 missing from the evaluator's verdict map: %+v", eligHeld)
	}
	if ev.Verdict != VerdictHeld {
		t.Fatalf("run 1 (gates: on: example-a/01, todo): want held, got %s (%+v)", ev.Verdict, ev)
	}
	if len(ev.Holds) != 1 || ev.Holds[0].Ref != "example-a/01" || ev.Holds[0].State != StateUnsatisfied {
		t.Fatalf("run 1 hold detail: want exactly one unsatisfied hold on example-a/01; got %+v", ev.Holds)
	}
	nuHeld := nextUp(streamsHeld, ClaimView{}, nil)
	for _, p := range nuHeld.Picks {
		if p.Stream.Name == "example-a" && p.Brief.Num == "02" {
			t.Fatalf("run 1: Next-up must not offer example-a/02 while it is held: %+v", nuHeld.Picks)
		}
	}
	t.Logf("run 1 (unmutated fixture): example-a/02 verdict=%s holds=%+v", ev.Verdict, ev.Holds)

	// Run 2: a FRESH copy of the fixture, with ONLY brief 02's gates: on:
	// retargeted from example-a/01 (todo) to example-a/03 (done) — one line,
	// nothing else changed.
	streamsEligible := mutateBrief02(t, `on: "example-a/01"`, `on: "example-a/03"`)
	eligEligible := eligibilityForStreams(streamsEligible)
	ev2, ok := eligEligible["example-a/02"]
	if !ok {
		t.Fatalf("example-a/02 missing from the evaluator's verdict map after retarget: %+v", eligEligible)
	}
	if ev2.Verdict != VerdictEligible {
		t.Fatalf("run 2 (gates: on: example-a/03, done): want eligible, got %s (%+v)", ev2.Verdict, ev2)
	}
	if len(ev2.Holds) != 0 {
		t.Fatalf("run 2: want zero holds; got %+v", ev2.Holds)
	}
	nuEligible := nextUp(streamsEligible, ClaimView{}, nil)
	found := false
	for _, p := range nuEligible.Picks {
		if p.Stream.Name == "example-a" && p.Brief.Num == "02" {
			found = true
		}
	}
	if !found {
		t.Fatalf("run 2: Next-up must offer example-a/02 once its gate is satisfied: %+v", nuEligible.Picks)
	}
	t.Logf("run 2 (gates: on: retargeted to example-a/03): example-a/02 verdict=%s holds=%+v", ev2.Verdict, ev2.Holds)

	// The two verdicts must disagree — the whole point of the milestone: the
	// declaration alone changed dispatch.
	if ev.Verdict == ev2.Verdict {
		t.Fatalf("the two runs must disagree: run 1=%s run 2=%s (same gates: target class, no declaration change actually took effect)", ev.Verdict, ev2.Verdict)
	}
}

// TestEligibilityCouldNotCheckHolds is brief-01's Verify row 3: a gates:
// entry on an `unpublished: true` alias yields state could-not-check and
// verdict held; the SAME ref under feathers: yields eligible-with-notice
// (never held — a feather is advisory by construction).
func TestEligibilityCouldNotCheckHolds(t *testing.T) {
	const unpubRef = "unpub:example-a/01"

	// gates: on an unpublished alias -> could-not-check -> held.
	gatedStreams := mutateBrief02(t, `on: "example-a/01"`, `on: "`+unpubRef+`"`)
	gatedElig := eligibilityForStreams(gatedStreams)
	gev, ok := gatedElig["example-a/02"]
	if !ok {
		t.Fatalf("example-a/02 missing from the evaluator's verdict map: %+v", gatedElig)
	}
	if gev.Verdict != VerdictHeld {
		t.Fatalf("gates: on an unpublished alias: want held, got %s (%+v)", gev.Verdict, gev)
	}
	if len(gev.Holds) != 1 || gev.Holds[0].Ref != unpubRef || gev.Holds[0].State != StateCouldNotCheck {
		t.Fatalf("gates: on an unpublished alias: want exactly one could-not-check hold on %s; got %+v", unpubRef, gev.Holds)
	}
	if gev.Holds[0].Why == "" {
		t.Errorf("a could-not-check hold must name WHY (the registry/checkout gap), not just THAT — Why was empty")
	}

	// The SAME ref, but declared feathers: instead of gates:, on a brief that
	// carries no gates: at all — could-not-check, but a NOTICE only, never a
	// hold.
	featheredStreams := mutateBrief02(t,
		"gates:\n  - on: \"example-a/01\"\n    type: ordering-gate\n    reason: \"example-a/01 must be in force before example-a/02 may start\"\n",
		"feathers:\n  - \""+unpubRef+"\"\n",
	)
	featheredElig := eligibilityForStreams(featheredStreams)
	fev, ok := featheredElig["example-a/02"]
	if !ok {
		t.Fatalf("example-a/02 missing from the evaluator's verdict map: %+v", featheredElig)
	}
	if fev.Verdict != VerdictEligibleWithNotice {
		t.Fatalf("feathers: on an unpublished alias: want eligible-with-notice, got %s (%+v)", fev.Verdict, fev)
	}
	if len(fev.Holds) != 0 {
		t.Fatalf("feathers: must never hold; got holds %+v", fev.Holds)
	}
	if len(fev.Notices) != 1 || fev.Notices[0].Ref != unpubRef || fev.Notices[0].State != StateCouldNotCheck {
		t.Fatalf("feathers: on an unpublished alias: want exactly one could-not-check notice on %s; got %+v", unpubRef, fev.Notices)
	}

	// Next-up must OFFER the eligible-with-notice brief (a feather never
	// excludes), mirroring the eligible-with-notice contract (Task item 1).
	nu := nextUp(featheredStreams, ClaimView{}, nil)
	found := false
	for _, p := range nu.Picks {
		if p.Stream.Name == "example-a" && p.Brief.Num == "02" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Next-up must offer an eligible-with-notice brief: %+v", nu.Picks)
	}
}
