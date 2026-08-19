package main

import (
	"strings"
	"testing"
	"time"
)

// drivefrontier_test.go — methodology-metrics phase 2 (runsheet 2.4): the drive
// LIFECYCLE + FRONTIER. These tests pin the frontier classification, the state
// derivation, and — above all — that phase 2 preserves phase 1's safety
// invariants: it is byte-neutral on the board (proven separately by the still-green
// TestDriveAbsentIsInert), fail-neutral, and deterministic.

// mkBrief is a small brief builder for the frontier tests.
func mkBrief(num, status string) Brief { return Brief{Num: num, Status: status} }

func mkBriefV1(num, status string, deps ...string) Brief {
	return Brief{Num: num, Status: status, Schema: "brief-v1", Depends: deps}
}

// mkDrive builds a live in-window drive with the given items (phase-2 tests set
// Items directly rather than through YAML — the loader is phase-1's concern).
func mkDrive(slug string, items ...DriveItem) Drive {
	return Drive{
		Slug: slug, DeclaredBy: "ian", Starts: "2026-08-14", Expires: "2026-08-20",
		Intensity: "push", State: "active", Why: "ship it", Items: items,
		streamSet: map[string]bool{}, briefSet: map[string]bool{},
	}
}

// --- frontier classification --------------------------------------------------

func TestDriveFrontierClassifiesEachStatus(t *testing.T) {
	s := mkStream("hot", "active", "P1",
		mkBrief("01", "done"),
		mkBrief("02", "verified"),
		mkBrief("03", "implemented"),
		mkBrief("04", "in-progress"),
		mkBrief("05", "todo"), // ready (legacy, wave 0)
		mkBrief("06", "blocked"),
	)
	streams := []*Stream{s}
	d := mkDrive("camp", DriveItem{Kind: "stream", Ref: "hot"})

	fr := driveFrontier(d, streams, nil, driveTestNow)
	want := map[string]string{
		"hot/01": fsDone, "hot/02": fsDone, "hot/03": fsBlockedReview,
		"hot/04": fsInFlight, "hot/05": fsReady, "hot/06": fsBlockedItem,
	}
	if len(fr) != len(want) {
		t.Fatalf("frontier has %d items, want %d: %+v", len(fr), len(want), fr)
	}
	for _, f := range fr {
		if want[f.Ref] != f.State {
			t.Errorf("%s: state %q, want %q", f.Ref, f.State, want[f.Ref])
		}
	}
}

func TestDriveFrontierBriefV1DepGating(t *testing.T) {
	// hot/02 (brief-v1) depends on hot/01; while hot/01 is todo, hot/02 is
	// blocked-on(item); once hot/01 is done, hot/02 is ready.
	blocked := mkStream("hot", "active", "P1",
		mkBriefV1("01", "todo"),
		mkBriefV1("02", "todo", "hot/01"),
	)
	d := mkDrive("camp", DriveItem{Kind: "brief", Ref: "hot/02"})
	fr := driveFrontier(d, []*Stream{blocked}, nil, driveTestNow)
	if len(fr) != 1 || fr[0].State != fsBlockedItem {
		t.Fatalf("hot/02 must be blocked-on(item) while hot/01 is open: %+v", fr)
	}

	done := mkStream("hot", "active", "P1",
		mkBriefV1("01", "done"),
		mkBriefV1("02", "todo", "hot/01"),
	)
	fr = driveFrontier(d, []*Stream{done}, nil, driveTestNow)
	if len(fr) != 1 || fr[0].State != fsReady {
		t.Fatalf("hot/02 must be ready once hot/01 is done: %+v", fr)
	}
}

func TestDriveFrontierClaimMakesInFlight(t *testing.T) {
	s := mkStream("hot", "active", "P1", mkBrief("01", "todo"))
	d := mkDrive("camp", DriveItem{Kind: "brief", Ref: "hot/01"})
	// No claim → ready.
	if fr := driveFrontier(d, []*Stream{s}, nil, driveTestNow); fr[0].State != fsReady {
		t.Fatalf("unclaimed todo must be ready: %+v", fr)
	}
	// Claimed (open origin branch/PR) → in-flight, not offered again.
	claimed := map[string]bool{"hot/01": true}
	if fr := driveFrontier(d, []*Stream{s}, claimed, driveTestNow); fr[0].State != fsInFlight {
		t.Fatalf("claimed todo must be in-flight: %+v", fr)
	}
}

func TestDriveFrontierIssueRefIsTracked(t *testing.T) {
	d := mkDrive("camp", DriveItem{Kind: "issue", Ref: "example-org/tracker#42"})
	fr := driveFrontier(d, nil, nil, driveTestNow)
	if len(fr) != 1 || fr[0].State != fsTracked || fr[0].Ref != "example-org/tracker#42" {
		t.Fatalf("a cross-repo issue ref must be tracked (could-not-check), never a stall: %+v", fr)
	}
}

// --- operator-act aging (the ONE sanctioned wall-clock, UTC-day) --------------

func TestDriveOperatorActAging(t *testing.T) {
	now := time.Date(2026, 8, 15, 23, 30, 0, 0, time.UTC)
	// since 2026-08-14 → 1 whole UTC day at 2026-08-15 (time-of-day truncated).
	d := mkDrive("camp", DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "grant scope", Since: "2026-08-14"})
	fr := driveFrontier(d, nil, nil, now)
	if len(fr) != 1 || fr[0].State != fsBlockedOperator {
		t.Fatalf("operator-act must be blocked-on(operator-act): %+v", fr)
	}
	if fr[0].AgeDays != 1 {
		t.Errorf("age = %d days, want 1 (UTC-day granularity, time-of-day ignored)", fr[0].AgeDays)
	}
	// Absent `since:` ages from the drive's `starts`.
	d2 := mkDrive("camp", DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "x"})
	fr2 := driveFrontier(d2, nil, nil, now)
	if fr2[0].Since != "2026-08-14" || fr2[0].AgeDays != 1 {
		t.Errorf("absent since must age from starts (2026-08-14 → 1 day): %+v", fr2[0])
	}
}

// --- state derivation ---------------------------------------------------------

func TestDriveStateDerivation(t *testing.T) {
	cases := []struct {
		name  string
		items []FrontierItem
		want  string
	}{
		{"ready-is-rolling", []FrontierItem{{State: fsReady}}, driveStateRolling},
		{"in-flight-is-rolling", []FrontierItem{{State: fsInFlight}}, driveStateRolling},
		{"needs-brief-is-rolling", []FrontierItem{{State: fsNeedsBrief}}, driveStateRolling},
		{"ready-beats-operator", []FrontierItem{{State: fsReady}, {State: fsBlockedOperator}}, driveStateRolling},
		{"operator-only", []FrontierItem{{State: fsBlockedOperator}, {State: fsDone}}, driveStateWaitingOp},
		{"operator-beats-review", []FrontierItem{{State: fsBlockedOperator}, {State: fsBlockedReview}}, driveStateWaitingOp},
		{"review-only", []FrontierItem{{State: fsBlockedReview}, {State: fsDone}}, driveStateWaitingRev},
		{"blocked-item-only-is-stuck", []FrontierItem{{State: fsBlockedItem}, {State: fsDone}}, driveStateStuck},
		{"all-done-is-rolling", []FrontierItem{{State: fsDone}, {State: fsDone}}, driveStateRolling},
		{"tracked-only-is-rolling", []FrontierItem{{State: fsTracked}}, driveStateRolling},
		{"empty-is-rolling", nil, driveStateRolling},
	}
	for _, tc := range cases {
		if got := driveState(tc.items); got != tc.want {
			t.Errorf("%s: state = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestDriveWaitingOnOperatorOnlyWhenBottleneck pins the load-bearing rule: the
// operator is named the bottleneck ONLY when nothing else can progress — an
// operator-act alongside ready work stays ROLLING (the worker takes other work).
func TestDriveWaitingOnOperatorOnlyWhenBottleneck(t *testing.T) {
	s := mkStream("hot", "active", "P1", mkBrief("01", "todo"), mkBrief("02", "done"))
	d := mkDrive("camp",
		DriveItem{Kind: "stream", Ref: "hot"},
		DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "grant scope", Since: "2026-08-14"},
	)
	st := driveStatuses(DriveSet{Active: []Drive{d}}, []*Stream{s}, nil, driveTestNow)[0]
	if st.State != driveStateRolling {
		t.Fatalf("an operator-act alongside ready work must stay ROLLING, got %q", st.State)
	}
	// Now the only open brief is claimed (in-flight) — still ROLLING.
	claimed := map[string]bool{"hot/01": true}
	st = driveStatuses(DriveSet{Active: []Drive{d}}, []*Stream{s}, claimed, driveTestNow)[0]
	if st.State != driveStateRolling {
		t.Fatalf("in-flight work keeps a drive ROLLING even with a pending operator-act, got %q", st.State)
	}
	// Everything the drive covers is done — now the operator IS the bottleneck.
	done := mkStream("hot", "active", "P1", mkBrief("01", "done"), mkBrief("02", "done"))
	st = driveStatuses(DriveSet{Active: []Drive{d}}, []*Stream{done}, nil, driveTestNow)[0]
	if st.State != driveStateWaitingOp {
		t.Fatalf("with no dispatchable work left, a pending operator-act must be WAITING-ON-OPERATOR, got %q", st.State)
	}
}

// TestDriveProgress pins the done/total brief count (external/operator rows excluded).
func TestDriveProgress(t *testing.T) {
	s := mkStream("hot", "active", "P1", mkBrief("01", "done"), mkBrief("02", "todo"), mkBrief("03", "verified"))
	d := mkDrive("camp",
		DriveItem{Kind: "stream", Ref: "hot"},
		DriveItem{Kind: "issue", Ref: "example-org/tracker#1"},
		DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "x"},
	)
	st := DriveStatus{Drive: d, Frontier: driveFrontier(d, []*Stream{s}, nil, driveTestNow)}
	st.State = driveState(st.Frontier)
	done, total := st.progress()
	if done != 2 || total != 3 {
		t.Fatalf("progress = %d/%d, want 2/3 (only brief items count)", done, total)
	}
}

// TestDriveSinceValidationFailNeutral pins that a malformed operator-act `since:`
// is fail-neutral at the loader (never freezes the board) — the phase-1 safety bar
// extended to the new field.
func TestDriveSinceValidationFailNeutral(t *testing.T) {
	item, err := classifyDriveItem(driveItemYAML{Owner: "operator", Unblocks: "x", Since: "not-a-date"})
	if err == "" {
		t.Fatal("a malformed operator-act since must be rejected (fail-neutral), got no error")
	}
	if !strings.Contains(err, "since") {
		t.Errorf("error must name the since field: %q", err)
	}
	_ = item
	// A well-formed since parses through.
	if _, e := classifyDriveItem(driveItemYAML{Owner: "operator", Unblocks: "x", Since: "2026-08-14"}); e != "" {
		t.Errorf("a valid since must classify cleanly: %q", e)
	}
	// Absent since is fine.
	if it, e := classifyDriveItem(driveItemYAML{Owner: "operator", Unblocks: "x"}); e != "" || it.Since != "" {
		t.Errorf("absent since must classify cleanly with empty Since: %q %+v", e, it)
	}
}
