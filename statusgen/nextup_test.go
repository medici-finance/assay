package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d) }

func TestNextUpScoringAndCaps(t *testing.T) {
	hot := mkStream("hot", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
		Brief{Num: "04", Wave: 0, Status: "todo"},
		Brief{Num: "05", Wave: 0, Status: "todo"}, // > perStreamCap so the cap actually caps
	)
	hot.LastTouch = day(10) // the max → staleness 0
	stale := mkStream("stale", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	stale.LastTouch = day(0) // 10 days stale → 2000 + 100
	paused := mkStream("paused", "paused", "P0", Brief{Num: "01", Wave: 0, Status: "todo"})
	paused.LastTouch = day(0)

	picks := nextUp([]*Stream{hot, stale, paused}, ClaimView{}, nil).Picks
	if len(picks) != perStreamCap+1 {
		t.Fatalf("got %d picks, want %d (%d hot capped + 1 stale, paused excluded)", len(picks), perStreamCap+1, perStreamCap)
	}
	if picks[0].Stream.Name != "hot" || picks[0].Score != 3000 {
		t.Errorf("pick 0: %+v", picks[0])
	}
	hotCount := 0
	for _, p := range picks {
		if p.Stream.Name == "hot" {
			hotCount++
		}
		if p.Stream.Name == "paused" {
			t.Error("paused stream must not be picked")
		}
	}
	if hotCount != perStreamCap {
		t.Errorf("per-stream cap violated: %d hot picks, want %d", hotCount, perStreamCap)
	}
	for _, p := range picks {
		if p.Stream.Name == "stale" && p.Score != 2100 {
			t.Errorf("stale score = %d, want 2100", p.Score)
		}
	}
}

func TestNextUpWaveGatingAndStale(t *testing.T) {
	s := mkStream("s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", StaleRef: "F-01"}, // stale → excluded
		Brief{Num: "02", Wave: 1, Status: "todo"},                   // wave 1 gated by 01 not done
		Brief{Num: "03", Wave: 0, Status: "in-progress"},            // in-progress always eligible
	)
	s.LastTouch = day(0)
	picks := nextUp([]*Stream{s}, ClaimView{}, nil).Picks
	if len(picks) != 1 || picks[0].Brief.Num != "03" {
		t.Fatalf("want only brief 03, got %+v", picks)
	}
}

// TestNextUpStreamFindingNotDispatchGate is the regression at
// the dispatch level: applyFindings + nextUp end to end. A stream-level finding
// (bare `affects: <stream>`) used to stamp StaleRef on every brief, and
// eligible() hard-excludes any StaleRef — so the whole issue-loop stream (its
// many todo placeholders) fell out of Next-up. The stream must stay dispatchable,
// while a brief-specific entry still removes exactly its own brief.
func TestNextUpStreamFindingNotDispatchGate(t *testing.T) {
	s := mkStream("issue-loop", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
		Brief{Num: "03", Wave: 0, Status: "todo", Schema: "placeholder-v1"},
	)
	s.LastTouch = day(0)
	findings := []Finding{
		{ID: "F-guardrails-dup", Affects: []string{"issue-loop"}},     // stream-level: gates nothing
		{ID: "F-bug-close", Affects: []string{"issue-loop/brief-03"}}, // brief-specific: gates 03
	}
	applyFindings([]*Stream{s}, findings)

	nu := nextUp([]*Stream{s}, ClaimView{}, nil)
	if nu.Eligible != 2 {
		t.Fatalf("want 2 eligible (01, 02 — only the brief-specific finding gates), got %d: %+v", nu.Eligible, nu.Picks)
	}
	for _, p := range nu.Picks {
		if p.Brief.Num == "03" {
			t.Errorf("brief-specific finding must still exclude brief 03: %+v", p)
		}
	}
}

func TestEligibilityDepPrecise(t *testing.T) {
	// Fixture modelled on the ledger-hardening/01 case: a wave-1 brief-v1
	// brief whose typed dep is verified, with an unrelated wave-0 todo in
	// the same stream. Under the old whole-wave rule, 02 is ineligible
	// because 01 (lower wave) is not done; under the new dep-precise rule,
	// 02 is eligible because dep/01 is verified.

	// --- subtest: dep verified → eligible ---
	depStream := mkStream("dep", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "verified"},
	)
	depStream.LastTouch = day(0)

	target := mkStream("target", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // unrelated — does not block 02
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"dep/01"}},
	)
	target.LastTouch = day(0)

	picks := nextUp([]*Stream{target, depStream}, ClaimView{}, nil).Picks
	found02 := false
	for _, p := range picks {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			found02 = true
		}
	}
	if !found02 {
		t.Fatalf("brief-v1 02 with verified dep should be eligible, got %+v", picks)
	}

	// --- subtest: dep merely implemented → ineligible ---
	depStream2 := mkStream("dep", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"},
	)
	depStream2.LastTouch = day(0)

	target2 := mkStream("target", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"dep/01"}},
	)
	target2.LastTouch = day(0)

	picks2 := nextUp([]*Stream{target2, depStream2}, ClaimView{}, nil).Picks
	for _, p := range picks2 {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			t.Fatalf("brief-v1 02 with implemented dep should be INELIGIBLE, got %+v", picks2)
		}
	}

	// --- subtest: empty depends → eligible now ---
	emptyDep := mkStream("empty", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // unrelated todo
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	emptyDep.LastTouch = day(0)

	picks3 := nextUp([]*Stream{emptyDep}, ClaimView{}, nil).Picks
	found02empty := false
	for _, p := range picks3 {
		if p.Stream.Name == "empty" && p.Brief.Num == "02" {
			found02empty = true
		}
	}
	if !found02empty {
		t.Fatalf("brief-v1 02 with empty depends should be eligible, got %+v", picks3)
	}

	// --- subtest: legacy stream — wave rule unchanged ---
	legacy := mkStream("legacy", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 1, Status: "todo"}, // no Schema → legacy
	)
	legacy.LastTouch = day(0)

	picks4 := nextUp([]*Stream{legacy}, ClaimView{}, nil).Picks
	for _, p := range picks4 {
		if p.Stream.Name == "legacy" && p.Brief.Num == "02" {
			t.Fatalf("legacy 02 should be wave-gated by 01, got %+v", picks4)
		}
	}
	// 01 should be eligible (wave 0, todo, no lower-wave blockers)
	found01 := false
	for _, p := range picks4 {
		if p.Stream.Name == "legacy" && p.Brief.Num == "01" {
			found01 = true
		}
	}
	if !found01 {
		t.Fatalf("legacy 01 (wave 0 todo) should be eligible, got %+v", picks4)
	}
}

// TestEligibilityV2EscapesWholeWaveGate is the review-requested regression
// (PR #1251, correctness lane, finding 1) for the arm of this change that
// actually moves the live board: before this PR a brief-v2 brief fell
// through to the legacy whole-wave rule (the schema check named only
// "brief-v1"), so an unfinished earlier-wave sibling held it regardless of
// its own `depends:`. After this PR a brief-v2 todo brief is gated by the
// evaluator's verdict on `depends:`/`gates:` alone — a wave-0 sibling that
// is merely `implemented` (not done/verified) no longer holds it. Modelled
// on the real board delta this PR produced (apps-installer/02,
// desk-supervision/08): a wave-1 brief-v2 brief with a satisfied `depends:`
// on a brief in ANOTHER stream, next to a same-stream wave-0 sibling that is
// not done.
func TestEligibilityV2EscapesWholeWaveGate(t *testing.T) {
	// --- subtest: depends satisfied → eligible despite the unfinished wave-0 sibling ---
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "done"},
	)
	blocker.LastTouch = day(0)

	target := mkStream("target", "active", "P1",
		Brief{Num: "08", Wave: 0, Status: "implemented"}, // unfinished wave-0 sibling
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v2", Depends: []string{"blocker/01"}},
	)
	target.LastTouch = day(0)

	picks := nextUp([]*Stream{target, blocker}, ClaimView{}, nil).Picks
	found02 := false
	for _, p := range picks {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			found02 = true
		}
	}
	if !found02 {
		t.Fatalf("brief-v2 02 with satisfied depends should be eligible despite the unfinished wave-0 sibling (legacy whole-wave rule must not apply to v2), got %+v", picks)
	}

	// --- subtest (converse): depends unsatisfied → held ---
	// The target is `implemented` with no gate and no Evidence. Under the
	// depends: rule pinned by TestDependsHumanGatePass below, `implemented`
	// satisfies a depends: edge ONLY for a gate:human brief with a recorded
	// pass — this one is neither, so it still holds.
	blocker2 := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented"}, // not done/verified, not gate:human-with-PASS
	)
	blocker2.LastTouch = day(0)

	target2 := mkStream("target", "active", "P1",
		Brief{Num: "08", Wave: 0, Status: "done"}, // wave-0 sibling done — would pass the legacy rule
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v2", Depends: []string{"blocker/01"}},
	)
	target2.LastTouch = day(0)

	picks2 := nextUp([]*Stream{target2, blocker2}, ClaimView{}, nil).Picks
	for _, p := range picks2 {
		if p.Stream.Name == "target" && p.Brief.Num == "02" {
			t.Fatalf("brief-v2 02 with unsatisfied depends should be HELD even though the wave-0 sibling is done, got %+v", picks2)
		}
	}

	// --- subtest (rule change): the SAME implemented target, but gate:human
	// with a recorded strict PASS → the depends: edge is satisfied → offered.
	// Before this rule an `implemented` target held every dependent whatever
	// its gate and verdict; this subtest is the deliberate flip of that.
	blocker3 := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Gate: "human", Evidence: evPass},
	)
	blocker3.LastTouch = day(0)
	target3 := mkStream("target", "active", "P1",
		Brief{Num: "08", Wave: 0, Status: "done"},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v2", Depends: []string{"blocker/01"}},
	)
	target3.LastTouch = day(0)
	if !pickedIn(nextUp([]*Stream{target3, blocker3}, ClaimView{}, nil).Picks, "target", "02") {
		t.Fatalf("brief-v2 02 whose depends: target is gate:human at implemented with a recorded PASS should be eligible")
	}
}

// Evidence bodies for the depends: rule tests below.
const (
	evPass      = "| Date | Runner |\n|---|---|\n| 2026-01-02 | verifier |\n\n**VERIFY: PASS**\n"
	evFail      = "**VERIFY: FAIL** — row 2 red\n"
	evPassFail  = "**VERIFY: PASS**\n\nre-run:\n\n**VERIFY: FAIL** — regression\n"
	evFailPass  = "**VERIFY: FAIL** — row 2 red\n\nre-run after fix:\n\n**VERIFY: PASS**\n"
	evLoosePass = "Non-implementer verifier run — VERIFY: PASS (no strict marker)\n"
	// A FAIL that answers a strict PASS, followed by a prose mention of a
	// future pass: the last verdict token reads PASS, so only the
	// FAIL-after-strict-PASS check holds it.
	evPassFailProsePass = "**VERIFY: PASS**\n\nre-run:\n\n**VERIFY: FAIL** — regression\n\nWill record VERIFY: PASS once green.\n"
	// A strict PASS that appears only struck, fenced or quoted: the marker is
	// present but no live verdict exists, so only the last-verdict check holds it.
	evStruckPass = "~~**VERIFY: PASS**~~ withdrawn\n"
	evFencedPass = "Expected output:\n\n```\n**VERIFY: PASS**\n```\n"
	evQuotedPass = "> **VERIFY: PASS**\n"
)

func pickedIn(picks []Pick, stream, num string) bool {
	for _, p := range picks {
		if p.Stream.Name == stream && p.Brief.Num == num {
			return true
		}
	}
	return false
}

// TestDependsHumanGatePass pins the depends: satisfaction rule: a target at
// done/verified satisfies a depends: edge; so does a gate:human target at
// `implemented` whose last recorded verdict is a strict PASS. Every other
// shape — a FAIL, a PASS later answered by a FAIL (even when prose mentions a
// pass after it), no verdict, a loose-form PASS only, a strict PASS that is only
// struck, fenced or quoted, a non-human gate, a status other than implemented —
// stays held.
// Each case is checked through all three readers: depIsSatisfied, the
// eligibility evaluator, and Next-up.
func TestDependsHumanGatePass(t *testing.T) {
	cases := []struct {
		name   string
		dep    Brief
		wantOK bool
	}{
		{"human implemented PASS", Brief{Status: "implemented", Gate: "human", Evidence: evPass}, true},
		{"human implemented FAIL then PASS", Brief{Status: "implemented", Gate: "human", Evidence: evFailPass}, true},
		{"human implemented FAIL", Brief{Status: "implemented", Gate: "human", Evidence: evFail}, false},
		{"human implemented PASS then FAIL", Brief{Status: "implemented", Gate: "human", Evidence: evPassFail}, false},
		{"human implemented no verdict", Brief{Status: "implemented", Gate: "human"}, false},
		{"human implemented loose PASS only", Brief{Status: "implemented", Gate: "human", Evidence: evLoosePass}, false},
		{"human implemented PASS then FAIL then prose PASS", Brief{Status: "implemented", Gate: "human", Evidence: evPassFailProsePass}, false},
		{"human implemented struck PASS only", Brief{Status: "implemented", Gate: "human", Evidence: evStruckPass}, false},
		{"human implemented fenced PASS only", Brief{Status: "implemented", Gate: "human", Evidence: evFencedPass}, false},
		{"human implemented quoted PASS only", Brief{Status: "implemented", Gate: "human", Evidence: evQuotedPass}, false},
		{"model implemented PASS", Brief{Status: "implemented", Gate: "model", Evidence: evPass}, false},
		{"legacy implemented PASS", Brief{Status: "implemented", Evidence: evPass}, false},
		{"human in-progress PASS", Brief{Status: "in-progress", Gate: "human", Evidence: evPass}, false},
		{"human todo PASS", Brief{Status: "todo", Gate: "human", Evidence: evPass}, false},
		{"model verified", Brief{Status: "verified", Gate: "model"}, true},
		{"human done", Brief{Status: "done", Gate: "human"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dep := tc.dep
			dep.Num, dep.Wave = "01", 0
			blocker := mkStream("blocker", "active", "P1", dep)
			blocker.LastTouch = day(0)
			target := mkStream("target", "active", "P1",
				Brief{Num: "02", Wave: 0, Status: "todo", Schema: "brief-v2", Depends: []string{"blocker/01"}},
			)
			target.LastTouch = day(0)
			streams := []*Stream{target, blocker}

			if got := depIsSatisfied(streams, "blocker/01"); got != tc.wantOK {
				t.Errorf("depIsSatisfied = %v, want %v", got, tc.wantOK)
			}
			ev := evaluateEligibility(streams, nil, "")["target/02"]
			if gotOK := ev.Verdict == VerdictEligible; gotOK != tc.wantOK {
				t.Errorf("evaluator verdict = %s (holds %+v), want eligible=%v", ev.Verdict, ev.Holds, tc.wantOK)
			}
			if got := pickedIn(nextUp(streams, ClaimView{}, nil).Picks, "target", "02"); got != tc.wantOK {
				t.Errorf("Next-up offers target/02 = %v, want %v", got, tc.wantOK)
			}
		})
	}
}

// TestGatesNeverHumanPass pins the other half of the rule: the gate:human +
// implemented + PASS target that satisfies a depends: edge NEVER satisfies a
// gates: edge (still held) or a feathers: edge (still a notice). A gates:
// edge keeps waiting for verified/done.
func TestGatesNeverHumanPass(t *testing.T) {
	dep := Brief{Num: "01", Wave: 0, Status: "implemented", Gate: "human", Evidence: evPass}
	blocker := mkStream("blocker", "active", "P1", dep)
	blocker.LastTouch = day(0)

	gated := mkStream("target", "active", "P1",
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "brief-v2",
			Gates: []GraphEdge{{Ref: "blocker/01", Type: "ordering-gate", Reason: "must be in force first"}}},
		Brief{Num: "03", Wave: 0, Status: "todo", Schema: "brief-v2",
			Feathers: []GraphEdge{{Ref: "blocker/01", Type: "build-dep"}}},
	)
	gated.LastTouch = day(0)
	streams := []*Stream{gated, blocker}

	if state, why := resolveInRepoBriefRef("blocker/01", streams); state != StateUnsatisfied || why == "" {
		t.Fatalf("gates:/feathers: resolution of a gate:human implemented PASS target: want unsatisfied with a reason, got %s %q", state, why)
	}
	elig := evaluateEligibility(streams, nil, "")
	if ev := elig["target/02"]; ev.Verdict != VerdictHeld || len(ev.Holds) != 1 || ev.Holds[0].State != StateUnsatisfied {
		t.Fatalf("gates: edge on a gate:human implemented PASS target must HOLD; got %+v", ev)
	}
	if ev := elig["target/03"]; ev.Verdict != VerdictEligibleWithNotice || len(ev.Notices) != 1 {
		t.Fatalf("feathers: edge on a gate:human implemented PASS target must stay a notice; got %+v", ev)
	}
	if pickedIn(nextUp(streams, ClaimView{}, nil).Picks, "target", "02") {
		t.Fatalf("Next-up must not offer a brief whose gates: target is only gate:human implemented PASS")
	}
	// Same target satisfies a depends: edge — the asymmetry is the rule.
	if !depIsSatisfied(streams, "blocker/01") {
		t.Fatalf("control: the same target must satisfy a depends: edge")
	}
}

func TestNextUpClaimAware(t *testing.T) {
	s := mkStream("hot", "active", "P0",
		Brief{Num: "06", Wave: 0, Status: "todo"},
		Brief{Num: "07", Wave: 0, Status: "todo"},
	)
	s.LastTouch = day(0)

	claimed := map[string]bool{"hot/06": true}
	picks := nextUp([]*Stream{s}, KnownClaims(claimed), nil).Picks
	if len(picks) != 1 || picks[0].Brief.Num != "07" {
		t.Fatalf("want only brief 07 (06 claimed), got %+v", picks)
	}

	// Claim lifts (PR merged/closed) → the brief reappears.
	picks = nextUp([]*Stream{s}, KnownClaims(map[string]bool{}), nil).Picks
	if len(picks) != 2 {
		t.Fatalf("want both briefs once the claim lifts, got %+v", picks)
	}
}

// withSpan sets the span-of-control cap + overflow threshold for the duration of
// a test and restores them afterward.
func withSpan(t *testing.T, cap, threshold int) {
	t.Helper()
	oldSpan, oldThresh := spanOfControl, overflowThreshold
	spanOfControl, overflowThreshold = cap, threshold
	t.Cleanup(func() { spanOfControl, overflowThreshold = oldSpan, oldThresh })
}

// eligibleStreams builds `n` eligible (active, P2, wave-0 todo) briefs spread
// across streams of at most `perStream` briefs each, so the per-stream cap
// doesn't collapse the count. All share the same LastTouch → equal scores.
func eligibleStreams(n, perStream int) []*Stream {
	var streams []*Stream
	made := 0
	for i := 0; made < n; i++ {
		k := perStream
		if made+k > n {
			k = n - made
		}
		var briefs []Brief
		for j := 0; j < k; j++ {
			briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", j+1), Wave: 0, Status: "todo"})
		}
		s := mkStream(fmt.Sprintf("s%02d", i), "active", "P2", briefs...)
		s.LastTouch = day(0)
		streams = append(streams, s)
		made += k
	}
	return streams
}

// --- Verify row 6 (phase 3): the anti-starvation floor ------------------------

// TestDriveAntiStarvationFloor (brief-44 Verify row 6): drive work is capped at 15
// of 20 board slots via a 2-pass fill (held-back to HeldByDriveCap), and
// effectiveCap = min(driveStreamCap, maxConcurrent); spanOfControl 20 and
// perStreamCap 4 are unchanged.
func TestDriveAntiStarvationFloor(t *testing.T) {
	// The floors are numeric invariants: pin them so a silent retune reddens.
	if spanOfControl != 20 {
		t.Fatalf("spanOfControl must remain 20, got %d", spanOfControl)
	}
	if perStreamCap != 4 {
		t.Fatalf("perStreamCap must remain 4, got %d", perStreamCap)
	}
	if driveSlotCap != 15 {
		t.Fatalf("driveSlotCap must be 15, got %d", driveSlotCap)
	}
	if driveWorkerCap != 6 {
		t.Fatalf("driveWorkerCap must be 6, got %d", driveWorkerCap)
	}

	// --- the 6-of-8 WORKER floor, bound on the dispatch queue (--next-up JSON) ---
	//
	// statusgen does not dispatch workers; it emits the queue a dispatcher starts
	// from. The floor binds THERE: with an active drive, drive picks offered for
	// dispatch never exceed driveWorkerCap minus the drive work already in flight
	// (claimed items the drive covers). Each case reads the JSON the dispatcher
	// consumes, not an internal field.
	for _, tc := range []struct {
		name        string
		inFlight    int  // claimed items in a drive-covered stream
		unknown     bool // claim filtering did not run
		wantDrive   int  // drive rows the queue may offer
		wantHeld    int  // drive rows withheld by the worker floor
		wantUnknown bool
	}{
		{name: "nothing-in-flight-offers-at-most-6", inFlight: 0, wantDrive: 6, wantHeld: 2},
		{name: "4-in-flight-offers-2", inFlight: 4, wantDrive: 2, wantHeld: 6},
		{name: "6-in-flight-offers-none", inFlight: 6, wantDrive: 0, wantHeld: 8},
		{name: "claims-unknown-is-could-not-check-and-offers-none", unknown: true, wantDrive: 0, wantHeld: 8, wantUnknown: true},
	} {
		t.Run("worker-floor-"+tc.name, func(t *testing.T) {
			view := workerFloorQueue(t, true, tc.inFlight, tc.unknown)
			if got := intField(t, view, "driveWorkerCap"); got != driveWorkerCap {
				t.Fatalf("the dispatch JSON must carry driveWorkerCap=%d while a drive is active, got %d", driveWorkerCap, got)
			}
			driveRows, nonDrive := 0, 0
			for _, r := range view["rows"].([]any) {
				if r.(map[string]any)["driveSlug"] != nil {
					driveRows++
				} else {
					nonDrive++
				}
			}
			if driveRows != tc.wantDrive {
				t.Fatalf("drive rows offered for dispatch: got %d, want %d (driveWorkerCap %d, in flight %d)", driveRows, tc.wantDrive, driveWorkerCap, tc.inFlight)
			}
			if nonDrive != 8 {
				t.Fatalf("the worker floor must never withhold NON-drive work: got %d non-drive rows, want 8", nonDrive)
			}
			if got := intField(t, view, "heldByDriveWorkerCap"); got != tc.wantHeld {
				t.Fatalf("heldByDriveWorkerCap: got %d, want %d", got, tc.wantHeld)
			}
			if !tc.unknown {
				if got := intField(t, view, "driveInFlight"); got != tc.inFlight {
					t.Fatalf("driveInFlight: got %d, want %d", got, tc.inFlight)
				}
			}
			reason, _ := view["driveWorkerUnknown"].(string)
			if tc.wantUnknown != (reason != "") {
				t.Fatalf("driveWorkerUnknown must be set iff claims are unread: got %q", reason)
			}
			// Nothing is dropped silently: shown + every held bucket == eligible.
			sum := intField(t, view, "shown") + intField(t, view, "heldByStreamCap") + intField(t, view, "heldBySpan") +
				intField(t, view, "heldByDriveCap") + intField(t, view, "heldByDriveWorkerCap")
			if sum != intField(t, view, "eligible") {
				t.Fatalf("held-back decomposition must account for every eligible pick: %d != eligible %d (%v)", sum, intField(t, view, "eligible"), view)
			}
		})
	}

	t.Run("worker-floor-critical-pick-takes-headroom-first", func(t *testing.T) {
		// One slot of headroom (5 of 6 in flight). The drive covers 4 routine P0
		// briefs and a P3 fire/01 that three reciprocated dependents lift into the
		// critical tier; the P0 picks outscore fire/01 even with its unblocks term.
		// The one slot must go to the critical pick, never to a higher-scored
		// routine drive pick: the floor grants headroom in the (criticalTier,
		// score) order, the same order the board ranks by.
		fire := mkStream("fire", "active", "P3",
			Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1", Unblocks: fireDependents})
		fire.LastTouch = day(0)
		blocked := mkStream("blocked", "active", "P2",
			Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
			Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
			Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"fire/01"}},
		)
		blocked.LastTouch = day(0)
		busy := driveBriefStream("busy", 5)
		drv0 := driveBriefStream("drv0", 4)
		drv0.Priority = "P0"
		streams := []*Stream{drv0, fire, blocked, busy}
		claimed := map[string]bool{}
		for _, b := range busy.Briefs {
			claimed["busy/"+b.Num] = true
		}
		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-critical", "declared-by: operator\n"+liveWindow+
			"intensity: surge\nstate: active\nitems:\n  - stream: drv0\n  - stream: fire\n  - stream: busy\n")
		ds := loadDrives(root, streams, driveTestNow)
		if !ds.applied() {
			t.Fatalf("the surge drive must apply: %+v", ds)
		}
		withDrives(t, ds)
		withFindings(t, nil)
		cv := KnownClaims(claimed)
		nu := nextUp(streams, cv, nil)
		view := buildDispatchView(nu, streams, "", cv.Source)
		var drive []dispatchRow
		for _, r := range view.Rows {
			if r.DriveSlug != "" {
				drive = append(drive, r)
			}
		}
		if len(drive) != 1 || drive[0].Brief != "fire/01" || drive[0].CriticalArm == "" {
			t.Fatalf("the one slot of headroom must go to the critical pick fire/01, got %+v (held %d)", drive, view.HeldByDriveWorkerCap)
		}
		if view.HeldByDriveWorkerCap != 4 {
			t.Fatalf("the 4 routine drive picks must be withheld, got held=%d", view.HeldByDriveWorkerCap)
		}
	})

	t.Run("worker-floor-inert-without-a-drive", func(t *testing.T) {
		view := workerFloorQueue(t, false, 4, false)
		for _, k := range []string{"driveWorkerCap", "driveInFlight", "heldByDriveWorkerCap", "driveWorkerUnknown"} {
			if _, ok := view[k]; ok {
				t.Fatalf("with no active drive the dispatch JSON must not carry %q (payload unchanged): %v", k, view)
			}
		}
		if n := len(view["rows"].([]any)); n != 16 {
			t.Fatalf("with no drive every eligible pick is offered: got %d rows, want 16", n)
		}
	})

	t.Run("15-of-20-via-2-pass-fill", func(t *testing.T) {
		// 5 driven streams × 4 briefs = 20 drive-eligible picks; 2 free streams × 4 = 8
		// non-drive picks. A surge drive covers the 5 driven streams. Pass 1 places 15
		// drive picks (the floor), reserving ≥5 slots that the higher-ranked drive picks
		// cannot consume; those 5 slots go to non-drive work in pass 1. Pass 2 finds the
		// span full → the 5 held-back drive picks attribute to HeldByDriveCap.
		var streams []*Stream
		var items string
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("drv%d", i)
			s := driveBriefStream(name, 4)
			streams = append(streams, s)
			items += "  - stream: " + name + "\n"
		}
		for i := 0; i < 2; i++ {
			streams = append(streams, driveBriefStream(fmt.Sprintf("free%d", i), 4))
		}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-wide", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n"+items)
		ds := loadDrives(root, streams, driveTestNow)
		if !ds.applied() {
			t.Fatalf("the surge drive must apply: %+v", ds)
		}
		withDrives(t, ds)
		withFindings(t, nil)

		nu := nextUp(streams, KnownClaims(map[string]bool{}), nil)
		if len(nu.Picks) != spanOfControl {
			t.Fatalf("board must fill the span (%d), got %d picks", spanOfControl, len(nu.Picks))
		}
		driveShown, nonDrive := 0, 0
		for _, p := range nu.Picks {
			if p.DriveTerm > 0 {
				driveShown++
			} else {
				nonDrive++
			}
		}
		if driveShown != driveSlotCap {
			t.Fatalf("drive-boosted picks must be floored at %d of %d, got %d shown", driveSlotCap, spanOfControl, driveShown)
		}
		if nonDrive != spanOfControl-driveSlotCap {
			t.Fatalf("the reserved %d non-drive slots must be filled by non-drive work, got %d", spanOfControl-driveSlotCap, nonDrive)
		}
		// The 5 drive picks past the floor are held OFF the board and attributed.
		if nu.HeldByDriveCap != 20-driveSlotCap {
			t.Fatalf("held-back drive picks must attribute to HeldByDriveCap: got %d, want %d", nu.HeldByDriveCap, 20-driveSlotCap)
		}
	})

	t.Run("backfill-when-no-non-drive-work", func(t *testing.T) {
		// With NO non-drive work, the board must NOT be left short: pass 2 backfills the
		// held-back drive picks up to the span. The floor reserves slots for non-drive
		// work WHEN IT EXISTS; it never idles the board.
		var streams []*Stream
		var items string
		for i := 0; i < 6; i++ { // 6 × 4 = 24 drive picks, span 20
			name := fmt.Sprintf("only%d", i)
			streams = append(streams, driveBriefStream(name, 4))
			items += "  - stream: " + name + "\n"
		}
		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-all", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n"+items)
		ds := loadDrives(root, streams, driveTestNow)
		withDrives(t, ds)
		withFindings(t, nil)

		nu := nextUp(streams, KnownClaims(map[string]bool{}), nil)
		if len(nu.Picks) != spanOfControl {
			t.Fatalf("with only drive work the board must still fill the span (%d), got %d", spanOfControl, len(nu.Picks))
		}
		driveShown := 0
		for _, p := range nu.Picks {
			if p.DriveTerm > 0 {
				driveShown++
			}
		}
		if driveShown != spanOfControl {
			t.Fatalf("with no non-drive work pass 2 must backfill the full span with drive picks, got %d", driveShown)
		}
	})

	t.Run("effectiveCap-min-driveStreamCap-maxConcurrent", func(t *testing.T) {
		// A driven stream declaring max-concurrent 2, with the drive setting
		// drive-stream-cap 1: effectiveCap = min(2,1) = 1 → at most one pick from it.
		capped := driveBriefStream("capped", 4)
		mc := 2
		capped.MaxConcurrent = &mc
		filler := driveBriefStream("filler", 4)
		streams := []*Stream{capped, filler}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-capped", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: capped\n    drive-stream-cap: 1\n")
		ds := loadDrives(root, streams, driveTestNow)
		if !ds.applied() {
			t.Fatalf("drive must apply: %+v", ds)
		}
		withDrives(t, ds)
		withFindings(t, nil)

		nu := nextUp(streams, KnownClaims(map[string]bool{}), nil)
		cappedCount := 0
		for _, p := range nu.Picks {
			if p.Stream.Name == "capped" {
				cappedCount++
			}
		}
		if cappedCount != 1 {
			t.Fatalf("effectiveCap = min(driveStreamCap 1, maxConcurrent 2) = 1 → exactly one capped pick, got %d", cappedCount)
		}
	})

	t.Run("maxConcurrent-always-wins", func(t *testing.T) {
		// max-concurrent 1, drive-stream-cap 3: the stream's declaration wins →
		// effectiveCap = min(1,3) = 1. A drive can never widen a stream above its
		// declared serialization.
		capped := driveBriefStream("serial", 4)
		mc := 1
		capped.MaxConcurrent = &mc
		streams := []*Stream{capped, driveBriefStream("filler", 4)}

		root := t.TempDir()
		makeStreamsDir(t, root)
		writeDrive(t, root, "surge-serial", "declared-by: ian\n"+liveWindow+"intensity: surge\nstate: active\nitems:\n  - stream: serial\n    drive-stream-cap: 3\n")
		ds := loadDrives(root, streams, driveTestNow)
		withDrives(t, ds)
		withFindings(t, nil)

		nu := nextUp(streams, KnownClaims(map[string]bool{}), nil)
		serialCount := 0
		for _, p := range nu.Picks {
			if p.Stream.Name == "serial" {
				serialCount++
			}
		}
		if serialCount != 1 {
			t.Fatalf("a stream's declared max-concurrent (1) must ALWAYS win over drive-stream-cap (3), got %d picks", serialCount)
		}
	})
}

// workerFloorQueue builds the worker-floor fixture and returns the `--next-up`
// dispatch JSON as a generic map (what a consumer parses). Two driven streams × 4
// briefs = 8 drive picks, two free streams × 4 = 8 non-drive picks; a third driven
// stream "busy" holds inFlight briefs, every one claimed (in flight, not eligible).
// drive=false skips the manifest; unknown=true runs with claims unread.
func workerFloorQueue(t *testing.T, drive bool, inFlight int, unknown bool) map[string]any {
	t.Helper()
	streams := []*Stream{driveBriefStream("drv0", 4), driveBriefStream("drv1", 4),
		driveBriefStream("free0", 4), driveBriefStream("free1", 4)}
	claimed := map[string]bool{}
	if inFlight > 0 {
		busy := driveBriefStream("busy", inFlight)
		streams = append(streams, busy)
		for _, b := range busy.Briefs {
			claimed["busy/"+b.Num] = true
		}
	}
	ds := DriveSet{}
	if drive {
		root := t.TempDir()
		makeStreamsDir(t, root)
		items := "  - stream: drv0\n  - stream: drv1\n"
		if inFlight > 0 {
			items += "  - stream: busy\n"
		}
		writeDrive(t, root, "surge-workers", "declared-by: operator\n"+liveWindow+
			"intensity: surge\nstate: active\nitems:\n"+items)
		ds = loadDrives(root, streams, driveTestNow)
		if !ds.applied() {
			t.Fatalf("the surge drive must apply: %+v", ds)
		}
	}
	withDrives(t, ds)
	withFindings(t, nil)
	cv := KnownClaims(claimed)
	if unknown {
		cv = ClaimView{Claimed: claimed}
	}
	nu := nextUp(streams, cv, nil)
	raw, err := json.Marshal(buildDispatchView(nu, streams, "", cv.Source))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

// intField reads an integer JSON field; an absent key reads as 0 (omitempty).
func intField(t *testing.T, view map[string]any, key string) int {
	t.Helper()
	v, ok := view[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("field %q is not a number: %v", key, v)
	}
	return int(f)
}

// TestNextUpSpanCapOverflow — 23 eligible briefs, span cap 7: exactly 7 shown,
// overflow flagged, 16 held back, and the STATUS view renders the "7 of 23
// eligible" overflow line (Verify item 2).
func TestNextUpSpanCapOverflow(t *testing.T) {
	withSpan(t, 7, 7)
	streams := eligibleStreams(23, 2) // 12 streams, per-stream cap keeps 7 spread-able
	nu := nextUp(streams, ClaimView{}, nil)
	if nu.Eligible != 23 {
		t.Fatalf("eligible = %d, want 23", nu.Eligible)
	}
	if len(nu.Picks) != 7 {
		t.Fatalf("shown = %d, want 7 (span cap)", len(nu.Picks))
	}
	if !nu.Overflow() {
		t.Fatal("want Overflow() = true when 23 eligible > cap 7")
	}
	if nu.HeldBack() != 16 {
		t.Fatalf("held back = %d, want 16", nu.HeldBack())
	}
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if !strings.Contains(out, "7 of 23 eligible") {
		t.Errorf("STATUS Next-up missing explicit overflow line:\n%s", out)
	}
	if !strings.Contains(out, "held back") {
		t.Error("overflow line must state how many are held back")
	}
}

// TestNextUpNoOverflow — 5 eligible briefs, span cap 7: all 5 shown, no overflow
// indicator anywhere in the STATUS view (Verify item 3).
func TestNextUpNoOverflow(t *testing.T) {
	withSpan(t, 7, 7)
	streams := eligibleStreams(5, 1) // 5 streams × 1 brief → all 5 fit under the cap
	nu := nextUp(streams, ClaimView{}, nil)
	if nu.Eligible != 5 {
		t.Fatalf("eligible = %d, want 5", nu.Eligible)
	}
	if len(nu.Picks) != 5 {
		t.Fatalf("shown = %d, want 5", len(nu.Picks))
	}
	if nu.Overflow() {
		t.Fatal("want Overflow() = false when 5 eligible <= cap 7")
	}
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	if strings.Contains(out, "held back") || strings.Contains(out, "eligible —") {
		t.Errorf("no overflow indicator expected with 5 ≤ cap 7:\n%s", out)
	}
}

// nextUpAllScores returns every eligible brief's raw score keyed by "stream/NN",
// bypassing the span/per-stream caps so a test can assert the score directly. It
// mirrors nextUp's per-brief formula (stream-touch clock; briefTouch not needed
// here).
func nextUpAllScores(streams []*Stream) map[string]int {
	var now time.Time
	if len(streams) > 0 {
		now = streams[0].LastTouch
	}
	for _, s := range streams {
		if s.LastTouch.After(now) {
			now = s.LastTouch
		}
	}
	rev, status := buildRevDeps(streams)
	out := map[string]int{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			if !eligible(streams, s, b, nil, wiredQueues(streams), eligibilityForStreams(streams)) {
				continue
			}
			days := int(now.Sub(s.LastTouch).Hours() / 24)
			if days < 0 {
				days = 0
			}
			if days > stalenessCapDays {
				days = stalenessCapDays
			}
			out[s.Name+"/"+b.Num] = priorityWeight(s.Priority) + days*stalenessPerDay +
				valueWeight(b.Value) + unblocksWeight*blockedCount(rev, status, s.Name+"/"+b.Num)
		}
	}
	return out
}

// TestNextUpValueOrdering — at equal priority and staleness, the explicit value
// field re-orders briefs: high > med > low, and an absent value scores exactly
// as med (the neutral zero point).
func TestNextUpValueOrdering(t *testing.T) {
	s := mkStream("v", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Value: "low"},
		Brief{Num: "02", Wave: 0, Status: "todo", Value: "high"},
		Brief{Num: "03", Wave: 0, Status: "todo"}, // absent → med
	)
	s.LastTouch = day(0)

	withSpan(t, 7, 7)
	picks := nextUp([]*Stream{s}, ClaimView{}, nil).Picks
	if len(picks) != 3 {
		t.Fatalf("all 3 fit under per-stream cap %d → 3 picks, got %d", perStreamCap, len(picks))
	}
	// value re-orders within the stream: high (02) tops; med/absent (03) second;
	// low (01) third. (All three fit — the per-stream cap is >= 3.)
	if picks[0].Brief.Num != "02" {
		t.Fatalf("high-value 02 should rank first, got %q", picks[0].Brief.Num)
	}
	if picks[1].Brief.Num != "03" {
		t.Fatalf("med (absent-value) 03 should rank second, got %q", picks[1].Brief.Num)
	}
	if picks[2].Brief.Num != "01" {
		t.Fatalf("low-value 01 should rank third, got %q", picks[2].Brief.Num)
	}

	scores := nextUpAllScores([]*Stream{s})
	if scores["v/03"] != 2000 {
		t.Errorf("absent value must score as med (2000), got %d", scores["v/03"])
	}
	if scores["v/02"] != 2000+valueWeightHigh {
		t.Errorf("high value score = %d, want %d", scores["v/02"], 2000+valueWeightHigh)
	}
	if scores["v/01"] != 2000+valueWeightLow {
		t.Errorf("low value score = %d, want %d", scores["v/01"], 2000+valueWeightLow)
	}
}

// TestNextUpBlockedCountPromotes — a brief that transitively holds up others
// (blockedCount ≥ 1) outranks a same-priority sibling that blocks nothing, even
// with identical priority, staleness and value. Chain: blocker/01 ← b/02 ← b/03.
func TestNextUpBlockedCountPromotes(t *testing.T) {
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
		Brief{Num: "09", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}}, // blocks nothing
	)
	blocker.LastTouch = day(0)
	chained := mkStream("b", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"b/02"}},
	)
	chained.LastTouch = day(0)

	scores := nextUpAllScores([]*Stream{blocker, chained})
	if got := scores["blocker/01"]; got != 2000+2*unblocksWeight {
		t.Fatalf("blocker/01 score = %d, want %d (blockedCount 2)", got, 2000+2*unblocksWeight)
	}
	if got := scores["blocker/09"]; got != 2000 {
		t.Fatalf("blocker/09 blocks nothing, score = %d, want 2000", got)
	}
	if scores["blocker/01"] <= scores["blocker/09"] {
		t.Fatalf("a blocker must outrank a same-priority non-blocker: %d vs %d",
			scores["blocker/01"], scores["blocker/09"])
	}
}

// TestNextUpStalenessFromBriefHistory — the F-09 clock fix. A brief ages from its
// OWN last transition (briefTouch, the historian) rather than the stream's git
// LastTouch, so a recent touch of the stream (sibling-brief activity) does not
// reset an unrelated brief's aging.
func TestNextUpStalenessFromBriefHistory(t *testing.T) {
	s := mkStream("h", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "todo"}, // last transitioned day(0)
		Brief{Num: "02", Wave: 0, Status: "todo"}, // last transitioned day(10)
	)
	// Stream touched recently (day 10) — under the OLD clock both briefs read
	// staleness 0. Under the fix, 01 ages from its own day(0).
	s.LastTouch = day(10)
	briefTouch := map[string]time.Time{
		"h/01": day(0),  // 10 days stale
		"h/02": day(10), // fresh
	}

	withSpan(t, 7, 7)
	picks := nextUp([]*Stream{s}, ClaimView{}, briefTouch).Picks
	scores := map[string]int{}
	for _, p := range picks {
		scores[p.Brief.Num] = p.Score
	}
	if scores["01"] != 2100 {
		t.Errorf("brief 01 should age from its own day(0) transition → 2100, got %d", scores["01"])
	}
	if scores["02"] != 2000 {
		t.Errorf("brief 02 transitioned at the touch day → 2000, got %d", scores["02"])
	}
	if picks[0].Brief.Num != "01" {
		t.Errorf("the brief stale by its own history should rank first, got %q", picks[0].Brief.Num)
	}
}

// TestNextUpStalenessFallsBackToStreamTouch — a brief with no historian row
// (absent from briefTouch) falls back to the stream's LastTouch, preserving the
// pre-14 behaviour.
func TestNextUpStalenessFallsBackToStreamTouch(t *testing.T) {
	s := mkStream("f", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	s.LastTouch = day(0)
	fresh := mkStream("g", "active", "P1", Brief{Num: "01", Wave: 0, Status: "todo"})
	fresh.LastTouch = day(10)

	// briefTouch knows nothing about f/01 → falls back to stream LastTouch day(0),
	// 10 days behind g's day(10): 2100 vs 2000.
	scores := map[string]int{}
	for _, p := range nextUp([]*Stream{s, fresh}, ClaimView{}, map[string]time.Time{}).Picks {
		scores[p.Stream.Name] = p.Score
	}
	if scores["f"] != 2100 {
		t.Errorf("f/01 with no history row should fall back to stream touch → 2100, got %d", scores["f"])
	}
	if scores["g"] != 2000 {
		t.Errorf("g/01 fresh → 2000, got %d", scores["g"])
	}
}

// TestNextUpBlockedCountOutranksInRender — Verify item 3: in the rendered
// Next-up table, a brief with blockedCount ≥3 sorts ABOVE a same-priority
// sibling that blocks nothing. ablock/00 is depended on (transitively) by three
// not-done briefs; zfree/00 blocks none. Both P1, equal staleness.
func TestNextUpBlockedCountOutranksInRender(t *testing.T) {
	blocker := mkStream("ablock", "active", "P1",
		Brief{Num: "00", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	// Three briefs gated on ablock/00 (still todo → not done): blockedCount = 3.
	deps := mkStream("deps", "active", "P1",
		Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"ablock/00"}},
	)
	deps.LastTouch = day(0)
	free := mkStream("zfree", "active", "P1",
		Brief{Num: "00", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{}},
	)
	free.LastTouch = day(0)

	withSpan(t, 7, 7)
	streams := []*Stream{blocker, deps, free}
	nu := nextUp(streams, ClaimView{}, nil)
	if got := blockedCountFor(streams, "ablock/00"); got != 3 {
		t.Fatalf("ablock/00 blockedCount = %d, want 3", got)
	}
	// The dep briefs are ineligible (their dep is still todo) so only the two
	// blockedCount-comparison briefs show.
	var order []string
	for _, p := range nu.Picks {
		order = append(order, p.Stream.Name+"/"+p.Brief.Num)
	}
	if len(order) < 2 || order[0] != "ablock/00" {
		t.Fatalf("blocker (blockedCount 3) must rank first; order = %v", order)
	}
	out := emit(streams, nil, nu, nil, nil, IntakeAlarmResult{}, nil, "")
	iBlock := strings.Index(out, "ablock | 00")
	iFree := strings.Index(out, "zfree | 00")
	if iBlock < 0 || iFree < 0 || iBlock > iFree {
		t.Fatalf("rendered Next-up must place the blocker above the non-blocker:\n%s", out)
	}
	t.Logf("Verify-3 observed Next-up ordering: %v", order)
}

// blockedCountFor is a test helper wrapping buildRevDeps + blockedCount.
func blockedCountFor(streams []*Stream, target string) int {
	rev, status := buildRevDeps(streams)
	return blockedCount(rev, status, target)
}

// --- Gate-score tests ---

// TestGateScoresChain verifies blockedCount walks a transitive dependency chain:
// blocker/01 ← chained/02 ← chained/03 (all not-done). blocker/01 has blockedCount 2.
func TestGateScoresChain(t *testing.T) {
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	chained := mkStream("chained", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"chained/02"}},
	)
	chained.LastTouch = day(0)

	gates := gateScores([]*Stream{blocker, chained}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 2 {
		t.Errorf("blocker/01 blockedCount = %d, want 2", gates[0].BlockedCount)
	}
	if gates[0].Score != 2000+2*unblocksWeight {
		t.Errorf("blocker/01 score = %d, want %d", gates[0].Score, 2000+2*unblocksWeight)
	}
}

// TestGateScoresDiamond verifies blockedCount handles a diamond dependency:
// A ← B, A ← C (B and C both depend on A). A has blockedCount 2.
func TestGateScoresDiamond(t *testing.T) {
	root := mkStream("root", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	root.LastTouch = day(0)
	leaves := mkStream("leaves", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"root/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"root/01"}},
	)
	leaves.LastTouch = day(0)

	gates := gateScores([]*Stream{root, leaves}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 2 {
		t.Errorf("root/01 blockedCount = %d, want 2 (diamond: two deps)", gates[0].BlockedCount)
	}
}

// TestGateScoresCrossStream verifies blockedCount walks across stream boundaries:
// stream-a/01 ← stream-b/02 (cross-stream dep).
func TestGateScoresCrossStream(t *testing.T) {
	a := mkStream("stream-a", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	a.LastTouch = day(0)
	b := mkStream("stream-b", "active", "P1",
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"stream-a/01"}},
	)
	b.LastTouch = day(0)

	gates := gateScores([]*Stream{a, b}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 1 {
		t.Errorf("stream-a/01 blockedCount = %d, want 1 (cross-stream dep)", gates[0].BlockedCount)
	}
}

// TestGateScoresCycleSafe verifies blockedCount is safe against dependency cycles.
// A depends on B, B depends on A — blockedCount must not loop forever.
func TestGateScoresCycleSafe(t *testing.T) {
	a := mkStream("cycle", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{"cycle/02"}},
		Brief{Num: "02", Wave: 0, Status: "todo", Schema: "brief-v1", Depends: []string{"cycle/01"}},
	)
	a.LastTouch = day(0)

	gates := gateScores([]*Stream{a}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief (cycle/01), got %d", len(gates))
	}
	// In a cycle, each node's blockedCount should terminate and count the other node
	// at most once. 01 → 02 (but 02 already seen) = 1.
	if gates[0].BlockedCount != 1 {
		t.Errorf("cycle/01 blockedCount = %d, want 1 (cycle: 02 counted once)", gates[0].BlockedCount)
	}
}

// TestGateScoresUnknownDep verifies an unknown/unresolvable dep contributes 0
// blockedCount and never panics.
func TestGateScoresUnknownDep(t *testing.T) {
	s := mkStream("s", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{"nonexistent/99"}},
	)
	s.LastTouch = day(0)

	gates := gateScores([]*Stream{s}, nil)
	if len(gates) != 1 {
		t.Fatalf("expected 1 awaiting brief, got %d", len(gates))
	}
	if gates[0].BlockedCount != 0 {
		t.Errorf("unknown dep blockedCount = %d, want 0", gates[0].BlockedCount)
	}
}

// TestGateScoresOrdering verifies gate scores are sorted descending by score,
// with stream name then brief num as tie-breakers.
func TestGateScoresOrdering(t *testing.T) {
	// P0 stream — any brief here outranks P1.
	p0 := mkStream("p0s", "active", "P0",
		Brief{Num: "10", Wave: 0, Status: "implemented", Value: "high"},
		Brief{Num: "01", Wave: 0, Status: "implemented"},
	)
	p0.LastTouch = day(0)
	// P1 stream with a blocker.
	blocker := mkStream("blocker", "active", "P1",
		Brief{Num: "01", Wave: 0, Status: "implemented", Schema: "brief-v1", Depends: []string{}},
	)
	blocker.LastTouch = day(0)
	deps := mkStream("deps", "active", "P1",
		Brief{Num: "01", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "02", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
		Brief{Num: "03", Wave: 1, Status: "todo", Schema: "brief-v1", Depends: []string{"blocker/01"}},
	)
	deps.LastTouch = day(0)

	gates := gateScores([]*Stream{p0, blocker, deps}, nil)
	if len(gates) != 3 {
		t.Fatalf("expected 3 awaiting briefs, got %d", len(gates))
	}
	// p0s/10 (P0 + high value = 3200) > p0s/01 (P0 = 3000) > blocker/01 (P1 + 3*500 = 3500)
	// Wait: 3500 > 3200, so blocker should rank second.
	// p0s/10: 3000 + 0 + 200 + 0 = 3200
	// p0s/01: 3000 + 0 + 0 + 0 = 3000
	// blocker/01: 2000 + 0 + 0 + 3*500 = 3500
	// So order: blocker/01 (3500), p0s/10 (3200), p0s/01 (3000)
	if gates[0].Stream.Name != "blocker" || gates[0].Brief.Num != "01" {
		t.Errorf("first should be blocker/01 (score 3500), got %s/%s (score %d)",
			gates[0].Stream.Name, gates[0].Brief.Num, gates[0].Score)
	}
	if gates[1].Stream.Name != "p0s" || gates[1].Brief.Num != "10" {
		t.Errorf("second should be p0s/10 (score 3200), got %s/%s (score %d)",
			gates[1].Stream.Name, gates[1].Brief.Num, gates[1].Score)
	}
	if gates[2].Stream.Name != "p0s" || gates[2].Brief.Num != "01" {
		t.Errorf("third should be p0s/01 (score 3000), got %s/%s (score %d)",
			gates[2].Stream.Name, gates[2].Brief.Num, gates[2].Score)
	}
}

// TestGateScoresIgnoresNonAwaiting verifies only implemented/verified briefs are scored.
func TestGateScoresIgnoresNonAwaiting(t *testing.T) {
	s := mkStream("s", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "in-progress"},
		Brief{Num: "03", Wave: 0, Status: "implemented"},
		Brief{Num: "04", Wave: 0, Status: "verified"},
		Brief{Num: "05", Wave: 0, Status: "done"},
	)
	s.LastTouch = day(0)

	gates := gateScores([]*Stream{s}, nil)
	if len(gates) != 2 {
		t.Fatalf("expected 2 awaiting briefs (implemented + verified), got %d", len(gates))
	}
	nums := map[string]bool{}
	for _, g := range gates {
		nums[g.Brief.Num] = true
	}
	if !nums["03"] || !nums["04"] {
		t.Errorf("want 03 and 04, got %v", nums)
	}
}

// TestNextUpMaxConcurrent tests the per-stream max-concurrent cap. A stream
// with max-concurrent: 1 and two eligible todo
// briefs should offer at most one pick; when one is already claimed (in-flight),
// the offer budget is zero.
func TestNextUpMaxConcurrent(t *testing.T) {
	withSpan(t, 7, 7)

	// --- subtest: max-concurrent: 1 → one pick from two eligible briefs ---
	one := 1
	s := mkStream("serial", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	s.LastTouch = day(0)
	s.MaxConcurrent = &one

	picks := nextUp([]*Stream{s}, KnownClaims(nil), nil).Picks
	if len(picks) != 1 {
		t.Fatalf("max-concurrent: 1 with 2 eligible: want 1 pick, got %d", len(picks))
	}

	// --- subtest: one claimed → zero picks (budget exhausted) ---
	claimed := map[string]bool{"serial/01": true}
	picks = nextUp([]*Stream{s}, KnownClaims(claimed), nil).Picks
	if len(picks) != 0 {
		t.Fatalf("max-concurrent: 1 with 1 claimed: want 0 picks, got %d", len(picks))
	}

	// --- subtest: read the remote, found no claims → one pick ---
	// The empty map here is a REAL answer, not a missing one, so the declared
	// budget is offerable in full.
	picks = nextUp([]*Stream{s}, KnownClaims(map[string]bool{}), nil).Picks
	if len(picks) != 1 {
		t.Fatalf("max-concurrent: 1 with a known-empty claim set: want 1 pick, got %d", len(picks))
	}

	// --- subtest: streams without the knob unchanged ---
	hot := mkStream("hot", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
	)
	hot.LastTouch = day(0)

	picks = nextUp([]*Stream{hot}, ClaimView{}, nil).Picks
	// Without max-concurrent, perStreamCap (4) controls; all 3 briefs should appear.
	if len(picks) != 3 {
		t.Fatalf("stream without max-concurrent: want 3 picks (under perStreamCap=4), got %d", len(picks))
	}

	// --- subtest: mixed — serial stream capped at 1, unconstrained stream at perStreamCap ---
	mixedSerial := mkStream("serial", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	mixedSerial.LastTouch = day(0)
	mixedSerial.MaxConcurrent = &one

	unconstrained := mkStream("free", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
		Brief{Num: "04", Wave: 0, Status: "todo"},
		Brief{Num: "05", Wave: 0, Status: "todo"},
	)
	unconstrained.LastTouch = day(0)

	nu := nextUp([]*Stream{mixedSerial, unconstrained}, KnownClaims(nil), nil)
	serialCount := 0
	for _, p := range nu.Picks {
		if p.Stream.Name == "serial" {
			serialCount++
		}
	}
	if serialCount != 1 {
		t.Fatalf("mixed: serial stream should have exactly 1 pick, got %d (picks=%v)", serialCount, nu.Picks)
	}
	// The unconstrained stream can fill the rest of the 7-slot span.
	freeCount := 0
	for _, p := range nu.Picks {
		if p.Stream.Name == "free" {
			freeCount++
		}
	}
	if freeCount < 3 {
		t.Fatalf("mixed: unconstrained stream should have at least 3 picks, got %d (picks=%v)", freeCount, nu.Picks)
	}
	if len(nu.Picks) < 4 {
		t.Fatalf("mixed: total picks should be at least 4 (1 serial + 3+ free), got %d", len(nu.Picks))
	}
}

// TestNextUpMaxConcurrentClaimsUnknown is the three-state leg: a stream that
// DECLARED max-concurrent asked for its briefs to serialize, and honouring that
// needs the in-flight signal. When claim filtering did not run, in-flight is
// unknowable — so the stream is held back to zero and NAMED, rather than
// offered its nominal budget as if nothing were in flight.
//
// This is the case the previous behaviour got wrong: it treated "could not
// read the remote" as "nothing is claimed" and offered a pick, which is exactly
// the parallel dispatch the declaration exists to forbid — on a stream that had
// said, in writing, do not do this.
func TestNextUpMaxConcurrentClaimsUnknown(t *testing.T) {
	withSpan(t, 7, 7)
	one := 1

	serial := mkStream("serial", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
	)
	serial.LastTouch = day(0)
	serial.MaxConcurrent = &one

	// A stream that declared NOTHING, in the same run: the default must not
	// move. Under unknown claims it keeps offering exactly what it offered
	// before — up to perStreamCap.
	plain := mkStream("plain", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
	)
	plain.LastTouch = day(0)

	unknown := ClaimView{Claimed: map[string]bool{}, Source: ClaimSource{Reason: "ls-remote timed out"}}
	nu := nextUp([]*Stream{serial, plain}, unknown, nil)

	serialCount, plainCount := 0, 0
	for _, p := range nu.Picks {
		switch p.Stream.Name {
		case "serial":
			serialCount++
		case "plain":
			plainCount++
		}
	}
	if serialCount != 0 {
		t.Fatalf("claims unknown + max-concurrent declared: want 0 picks from the serialized stream, got %d", serialCount)
	}
	if plainCount != 3 {
		t.Fatalf("claims unknown + no declaration: want 3 picks (unchanged default), got %d", plainCount)
	}
	if len(nu.SerializedUnknown) != 1 || nu.SerializedUnknown[0] != "serial" {
		t.Fatalf("could-not-check must be REPORTED, not silent: SerializedUnknown = %v, want [serial]", nu.SerializedUnknown)
	}
	// The view must also carry the degradation through, so emit renders the
	// banner without the caller re-assigning it.
	if nu.Claims.Known {
		t.Error("nextUp must carry the ClaimView's source into NextUp.Claims")
	}

	// And the same board with a successful (empty) claim read offers the
	// declared budget and reports nothing — proving the zero-pick above is the
	// could-not-check state, not a cap that binds unconditionally.
	nuKnown := nextUp([]*Stream{serial, plain}, KnownClaims(map[string]bool{}), nil)
	serialCount = 0
	for _, p := range nuKnown.Picks {
		if p.Stream.Name == "serial" {
			serialCount++
		}
	}
	if serialCount != 1 {
		t.Fatalf("claims known-empty: want 1 pick from the serialized stream, got %d", serialCount)
	}
	if len(nuKnown.SerializedUnknown) != 0 {
		t.Errorf("known claims must report no could-not-check, got %v", nuKnown.SerializedUnknown)
	}
}

// TestNextUpHeldBackAttribution: the held-back count must name the cap that
// actually fired. It used to blame the span-of-control cap for every held-back
// brief, which is the wrong knob whenever a per-stream cap is what bound —
// and with perStreamCap 4 against a span of 20, that is the common case.
func TestNextUpHeldBackAttribution(t *testing.T) {
	withSpan(t, 20, 20)

	// 6 eligible in one stream, perStreamCap 4 → 2 held back, span never reached.
	s := mkStream("wide", "active", "P0",
		Brief{Num: "01", Wave: 0, Status: "todo"},
		Brief{Num: "02", Wave: 0, Status: "todo"},
		Brief{Num: "03", Wave: 0, Status: "todo"},
		Brief{Num: "04", Wave: 0, Status: "todo"},
		Brief{Num: "05", Wave: 0, Status: "todo"},
		Brief{Num: "06", Wave: 0, Status: "todo"},
	)
	s.LastTouch = day(0)

	nu := nextUp([]*Stream{s}, KnownClaims(nil), nil)
	if len(nu.Picks) != perStreamCap {
		t.Fatalf("want %d picks (perStreamCap), got %d", perStreamCap, len(nu.Picks))
	}
	if nu.HeldByStreamCap != 2 {
		t.Fatalf("HeldByStreamCap = %d, want 2", nu.HeldByStreamCap)
	}
	if nu.HeldBySpan() != 0 {
		t.Fatalf("HeldBySpan = %d, want 0 — the span cap (20) was never reached", nu.HeldBySpan())
	}
	if got := heldBackReason(nu); !strings.Contains(got, "per-stream") {
		t.Errorf("held-back reason blames the wrong knob: %q", got)
	}

	// Nothing held back by a per-stream cap → the span cap is the honest answer.
	plain := mkStream("plain", "active", "P0", Brief{Num: "01", Wave: 0, Status: "todo"})
	plain.LastTouch = day(0)
	nuSpan := nextUp([]*Stream{plain}, KnownClaims(nil), nil)
	if got := heldBackReason(nuSpan); !strings.Contains(got, "span-of-control") || strings.Contains(got, "per-stream") {
		t.Errorf("with no per-stream holdback the reason should be the span cap alone; got %q", got)
	}
}
