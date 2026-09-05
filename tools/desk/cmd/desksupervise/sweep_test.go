package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// noopArm is the arm seam for tests that do not assert on arming; a test that DOES (the
// arm-before-release order) injects its own recording arm instead.
func noopArm(claimRecord, string) error { return nil }

func mustParseTS(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test timestamp %q: %v", s, err)
	}
	return ts
}

func heartbeatDeadClaim() claimRecord {
	return claimRecord{
		Key: "x--01", Item: "x/01", Owner: "o", Repo: "o/r", Branch: "b",
		Tier: "cheap", State: "dispatched", DispatchedAt: "2026-09-02T10:00:00Z",
	}
}

func heartbeatDeadObs() observationSource {
	ts, _ := time.Parse(time.RFC3339, "2026-09-02T11:00:00Z") // fixed, known-valid literal
	obs := loopengine.Observation{At: ts}
	return func(claimRecord) (resolvedObservation, error) {
		return resolvedObservation{obs: obs, observed: true}, nil
	}
}

// TestSweep_DryRunNeverCallsActions is the fail-first guard: dry-run must never invoke the
// reclaim/file functions, whatever the classification. A mutation that dropped the dryRun
// check in runAction (actions.go) would trip this test — see the PR body's Fail-first
// section for the recorded red run against exactly that mutation.
func TestSweep_DryRunNeverCallsActions(t *testing.T) {
	reclaimCalled, fileCalled := false, false
	reclaim := func(claimRecord) error { reclaimCalled = true; return nil }
	fileBT := func(claimRecord) error { fileCalled = true; return nil }

	claims := []claimRecord{heartbeatDeadClaim()}
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	results, anyBlind, err := sweep(claims, heartbeatDeadObs(), loopengine.DefaultLivenessPolicy(), now, true, reclaim, fileBT, noopArm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if anyBlind {
		t.Fatal("this claim is not blind")
	}
	if len(results) != 1 || results[0].Disp != loopengine.ReclaimHeartbeat {
		t.Fatalf("expected a single HEARTBEAT-EXPIRED result, got %+v", results)
	}
	if reclaimCalled {
		t.Fatal("--dry-run must NEVER call reclaim")
	}
	if fileCalled {
		t.Fatal("--dry-run must NEVER call fileBlockedTimeout")
	}
	if !strings.Contains(out.String(), "HEARTBEAT-EXPIRED") || !strings.Contains(out.String(), "action=RECLAIM-ELIGIBLE") {
		t.Fatalf("output missing expected classification line: %q", out.String())
	}
}

// TestSweep_NonDryRunCallsReclaim proves the OTHER half: without --dry-run, a
// RECLAIM-ELIGIBLE classification DOES call reclaim exactly once, with the right claim.
func TestSweep_NonDryRunCallsReclaim(t *testing.T) {
	var gotClaim claimRecord
	calls := 0
	reclaim := func(c claimRecord) error { calls++; gotClaim = c; return nil }
	fileBT := func(claimRecord) error { t.Fatal("fileBlockedTimeout must not be called for a reclaim"); return nil }

	claims := []claimRecord{heartbeatDeadClaim()}
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	if _, _, err := sweep(claims, heartbeatDeadObs(), loopengine.DefaultLivenessPolicy(), now, false, reclaim, fileBT, noopArm, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 reclaim call, got %d", calls)
	}
	if gotClaim.Key != "x--01" {
		t.Fatalf("reclaim called with the wrong claim: %+v", gotClaim)
	}
}

// TestSweep_BlindNeverActs proves BLIND claims never reach either action, dry-run or not.
func TestSweep_BlindNeverActs(t *testing.T) {
	reclaimCalled, fileCalled := false, false
	reclaim := func(claimRecord) error { reclaimCalled = true; return nil }
	fileBT := func(claimRecord) error { fileCalled = true; return nil }
	blindSource := func(claimRecord) (resolvedObservation, error) {
		return resolvedObservation{observed: false}, nil
	}

	claims := []claimRecord{heartbeatDeadClaim()}
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	results, anyBlind, err := sweep(claims, blindSource, loopengine.DefaultLivenessPolicy(), now, false, reclaim, fileBT, noopArm, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !anyBlind {
		t.Fatal("expected anyBlind=true")
	}
	if len(results) != 1 || !results[0].Blind || results[0].Action != "BLIND" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if reclaimCalled || fileCalled {
		t.Fatal("a BLIND claim must never be acted on")
	}
	if !strings.Contains(out.String(), "COULD-NOT-CHECK") || !strings.Contains(out.String(), "action=BLIND") {
		t.Fatalf("output missing expected BLIND line: %q", out.String())
	}
	if strings.Contains(out.String(), "RECLAIM") {
		t.Fatalf("a BLIND claim's line must never mention RECLAIM: %q", out.String())
	}
}

// TestTickArmsStopBeforeRelease pins the ORDER property of the per-run stop: a
// non-dry-run reclaim (NEVER-STARTED / HEARTBEAT-EXPIRED) arms the per-run stop flag BEFORE
// it releases the claim, so a still-live wedged worker is already halted by Layer A when its
// claim is freed for re-dispatch. If arming moved after release — or dropped — this ordering
// (or presence) assertion fails.
func TestTickArmsStopBeforeRelease(t *testing.T) {
	var order []string
	arm := func(c claimRecord, reason string) error {
		if reason == "" {
			t.Error("arm was called with an empty reason")
		}
		order = append(order, "arm:"+c.Key)
		return nil
	}
	reclaim := func(c claimRecord) error { order = append(order, "release:"+c.Key); return nil }
	fileBT := func(claimRecord) error { t.Fatal("fileBlockedTimeout must not run for a reclaim"); return nil }

	claims := []claimRecord{heartbeatDeadClaim()} // HEARTBEAT-EXPIRED → RECLAIM-ELIGIBLE
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	if _, _, err := sweep(claims, heartbeatDeadObs(), loopengine.DefaultLivenessPolicy(), now, false, reclaim, fileBT, arm, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "arm:x--01" || order[1] != "release:x--01" {
		t.Fatalf("arm must precede release for a reclaimed run, got %v", order)
	}
}

// TestTickDryRunArmsNothing: under --dry-run the reclaim path arms NOTHING (and releases
// nothing), the same dry-run suppression the reclaim/file seams already have.
func TestTickDryRunArmsNothing(t *testing.T) {
	armed := false
	arm := func(claimRecord, string) error { armed = true; return nil }
	reclaim := func(claimRecord) error { t.Fatal("dry-run must not release"); return nil }
	fileBT := func(claimRecord) error { t.Fatal("dry-run must not file"); return nil }

	claims := []claimRecord{heartbeatDeadClaim()}
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	if _, _, err := sweep(claims, heartbeatDeadObs(), loopengine.DefaultLivenessPolicy(), now, true, reclaim, fileBT, arm, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if armed {
		t.Fatal("--dry-run must NEVER arm a per-run stop")
	}
}

// TestTickProductionArmWritesTheFlag exercises the PRODUCTION arm path (doArmRunStop →
// deskkit.ArmRunStop) hermetically by pointing HOME at a temp dir, and proves the flag it
// writes is the one deskkit.ListRunStops reads back.
func TestTickProductionArmWritesTheFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DESK_TOOLS_DISABLED", "")
	reclaim := func(claimRecord) error { return nil }
	fileBT := func(claimRecord) error { return nil }

	claims := []claimRecord{heartbeatDeadClaim()}
	now := mustParseTS(t, "2026-09-02T12:00:00Z")
	var out bytes.Buffer

	if _, _, err := sweep(claims, heartbeatDeadObs(), loopengine.DefaultLivenessPolicy(), now, false, reclaim, fileBT, doArmRunStop, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stops, err := deskkit.ListRunStops()
	if err != nil {
		t.Fatalf("ListRunStops: %v", err)
	}
	if len(stops) != 1 || stops[0].Key != "x--01" {
		t.Fatalf("production arm did not write the expected flag: %+v", stops)
	}
}

// TestSweep_MalformedTierAbortsWholeTick pins the fail-closed config-error path: a claim
// with an unrecognised tier aborts the tick as could-not-check rather than guessing a
// default tier (which would silently apply the wrong wall cap).
func TestSweep_MalformedTierAbortsWholeTick(t *testing.T) {
	claims := []claimRecord{{Key: "x--01", DispatchedAt: "2026-09-02T10:00:00Z", Tier: "bogus"}}
	always := func(claimRecord) (resolvedObservation, error) { return resolvedObservation{observed: true}, nil }
	var out bytes.Buffer

	_, _, err := sweep(claims, always, loopengine.DefaultLivenessPolicy(), time.Now(), true, nil, nil, noopArm, &out)
	if err == nil {
		t.Fatal("an unrecognised tier must abort the tick, not silently proceed")
	}
}

// TestSweep_AllFixtureScenarios re-runs the exact five fixture pairs the Verify table's
// rows 3-7 exercise, in-process (no built binary, no exec) — the same assertions, doubling
// as a fast regression net for `go test`.
func TestSweep_AllFixtureScenarios(t *testing.T) {
	noopReclaim := func(claimRecord) error { return nil }
	noopFile := func(claimRecord) error { return nil }

	cases := []struct {
		name           string
		claimsFile     string
		obsFile        string
		now            string
		wantContains   []string
		wantNotContain []string
		wantBlind      bool
	}{
		{
			name: "row3_dead_worker", claimsFile: "testdata/dead-worker.json", obsFile: "testdata/dead-worker-obs.json",
			now: "2026-09-02T12:00:00Z", wantContains: []string{"HEARTBEAT-EXPIRED", "action=RECLAIM-ELIGIBLE"},
		},
		{
			name: "row4_alive_worker", claimsFile: "testdata/alive-worker.json", obsFile: "testdata/alive-worker-obs.json",
			now: "2026-09-02T12:00:00Z", wantContains: []string{"ALIVE", "action=none"}, wantNotContain: []string{"RECLAIM"},
		},
		{
			name: "row5_blind", claimsFile: "testdata/dead-worker.json", obsFile: "testdata/blind-obs.json",
			now: "2026-09-02T12:00:00Z", wantContains: []string{"COULD-NOT-CHECK", "action=BLIND"},
			wantNotContain: []string{"RECLAIM"}, wantBlind: true,
		},
		{
			name: "row6_never_started", claimsFile: "testdata/never-started.json", obsFile: "testdata/none-obs.json",
			now: "2026-09-02T12:00:00Z", wantContains: []string{"NEVER-STARTED"},
		},
		{
			name: "row7_long_runner", claimsFile: "testdata/long-runner.json", obsFile: "testdata/long-runner-obs.json",
			now: "2026-09-02T14:00:00Z", wantContains: []string{"OVER-WALL-CAP", "action=BLOCKED-TIMEOUT"},
			wantNotContain: []string{"RECLAIM"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			claims, err := loadClaimsFixture(tc.claimsFile)
			if err != nil {
				t.Fatalf("loadClaimsFixture: %v", err)
			}
			obsByKey, err := loadObservationsFixture(tc.obsFile)
			if err != nil {
				t.Fatalf("loadObservationsFixture: %v", err)
			}
			now := mustParseTS(t, tc.now)
			var out bytes.Buffer

			// sweep() itself never errors on a blind claim — it only sets anyBlind=true;
			// mapping that to exit 6 is tick.go's job (cmdTick), not sweep's.
			_, anyBlind, serr := sweep(claims, fixtureObservationSource(obsByKey), loopengine.DefaultLivenessPolicy(), now, true, noopReclaim, noopFile, noopArm, &out)
			if serr != nil {
				t.Fatalf("unexpected error: %v", serr)
			}
			if anyBlind != tc.wantBlind {
				t.Fatalf("anyBlind = %v, want %v", anyBlind, tc.wantBlind)
			}
			got := out.String()
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
			for _, notWant := range tc.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("output must not contain %q:\n%s", notWant, got)
				}
			}
		})
	}
}
