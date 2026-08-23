package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// driveissues_test.go — methodology-metrics phase 2: the drive TRACKING ISSUE +
// aging operator-act issues + the exactly-one @operator ping with its
// re-ping-on-state-change-or-24h→72h dedup.

func liveDrive(slug string, items ...DriveItem) Drive {
	d := mkDrive(slug, items...)
	return d
}

// --- tracking issue -----------------------------------------------------------

func TestDriveTrackingIssueRolling(t *testing.T) {
	s := mkStream("hot", "active", "P1", mkBrief("01", "todo"), mkBrief("02", "done"))
	d := liveDrive("ship", DriveItem{Kind: "stream", Ref: "hot"})
	issues := driveIssues([]*Stream{s}, DriveSet{Active: []Drive{d}}, nil, driveTestNow, map[string]bool{}, "")
	if len(issues) != 1 {
		t.Fatalf("a rolling drive with no aged acts must emit exactly the tracking issue, got %d: %+v", len(issues), issues)
	}
	iss := issues[0]
	if iss.Kind != "tracking" || iss.State != driveStateRolling {
		t.Fatalf("want a ROLLING tracking issue, got kind=%q state=%q", iss.Kind, iss.State)
	}
	if strings.Contains(iss.Title, waitingOnYouTitleTag) {
		t.Errorf("a ROLLING drive title must NOT carry the [WAITING ON YOU] tag: %q", iss.Title)
	}
	if iss.Ping {
		t.Errorf("a ROLLING drive must not ping")
	}
	if !strings.Contains(iss.Body, driveTrackingMarker("ship")) {
		t.Errorf("tracking body must carry the idempotency marker")
	}
	if !strings.Contains(iss.Body, "Progress:** 1/2") {
		t.Errorf("tracking body must show progress 1/2: %s", iss.Body)
	}
	wantLabel := false
	for _, l := range iss.Labels {
		if l == driveLabelPrefix+"ship" {
			wantLabel = true
		}
	}
	if !wantLabel {
		t.Errorf("tracking issue must carry the drive:<slug> label, got %v", iss.Labels)
	}
}

// TestDriveTrackingIdempotent: a tracking marker already present suppresses
// re-emission of the create payload (the create is idempotent), exactly like
// --decision-issues. Here we assert the marker set is honored for the aging acts;
// the tracking issue itself is always RECOMPUTED (its body/state may have changed),
// and the actor create-if-absent's on the marker — mirroring decisionissues, whose
// consumer likewise skips create when the marker exists.
func TestDriveAgingIssueIdempotent(t *testing.T) {
	// An operator-act aged 3 days → its own aging issue.
	now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	d := liveDrive("ship",
		DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "grant workflows scope", Since: "2026-08-14"},
	)
	set := DriveSet{Active: []Drive{d}}

	fresh := driveIssues(nil, set, nil, now, map[string]bool{}, "")
	// tracking + one aging act.
	var aging *driveIssue
	for i := range fresh {
		if fresh[i].Kind == "operator-act" {
			aging = &fresh[i]
		}
	}
	if aging == nil {
		t.Fatalf("a 3-day operator-act must get its own aging issue: %+v", fresh)
	}
	if !strings.Contains(aging.Body, "grant workflows scope") {
		t.Errorf("aging issue must name what the act unblocks")
	}

	// With that act's marker already present, the aging issue is NOT re-emitted.
	existing := map[string]bool{aging.Marker: true}
	again := driveIssues(nil, set, nil, now, existing, "")
	for _, iss := range again {
		if iss.Kind == "operator-act" {
			t.Fatalf("an aging issue whose marker exists must not be re-emitted: %+v", iss)
		}
	}
}

// A brand-new operator-act (below the aging threshold) rides only the tracking
// issue's WAITING-ON-YOU table — no separate aging issue yet.
func TestDriveFreshActNoAgingIssue(t *testing.T) {
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC) // same day as starts → age 0
	d := liveDrive("ship", DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "x", Since: "2026-08-14"})
	issues := driveIssues(nil, DriveSet{Active: []Drive{d}}, nil, now, map[string]bool{}, "")
	for _, iss := range issues {
		if iss.Kind == "operator-act" {
			t.Fatalf("a same-day operator-act must not yet get an aging issue: %+v", iss)
		}
	}
	// but it IS listed in the tracking table.
	if len(issues) != 1 || !strings.Contains(issues[0].Body, "Waiting on you") {
		t.Fatalf("the fresh act must appear in the tracking WAITING-ON-YOU table: %+v", issues)
	}
}

// --- WAITING-ON-OPERATOR + @operator ping + dedup -------------------------------

// waitingOpDrive: a drive whose only covered brief is done, plus a pending
// operator-act — so the drive is WAITING-ON-OPERATOR and the operator is the
// bottleneck.
func waitingOpSetup(t *testing.T, since string, now time.Time) (DriveStatus, []*Stream) {
	t.Helper()
	s := mkStream("hot", "active", "P1", mkBrief("01", "done"))
	d := liveDrive("ship",
		DriveItem{Kind: "stream", Ref: "hot"},
		DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "grant scope", Since: since},
	)
	st := driveStatuses(DriveSet{Active: []Drive{d}}, []*Stream{s}, nil, now)[0]
	if st.State != driveStateWaitingOp {
		t.Fatalf("setup must be WAITING-ON-OPERATOR, got %q", st.State)
	}
	return st, []*Stream{s}
}

func TestDriveWaitingOpPingsOperatorOnce(t *testing.T) {
	now := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	st, streams := waitingOpSetup(t, "2026-08-14", now)
	d := st.Drive

	// First run, no existing markers → exactly one ping, mentioning @operator, with
	// the [WAITING ON YOU] title tag.
	issues := driveIssues(streams, DriveSet{Active: []Drive{d}}, nil, now, map[string]bool{}, "")
	track := findTracking(t, issues)
	if !strings.Contains(track.Title, waitingOnYouTitleTag) {
		t.Errorf("WAITING-ON-OPERATOR title must carry the [WAITING ON YOU] tag: %q", track.Title)
	}
	if !track.Ping {
		t.Fatal("entering WAITING-ON-OPERATOR must ping")
	}
	if !strings.Contains(track.PingComment, operatorHandle) {
		t.Errorf("the ping comment must @-mention %s: %q", operatorHandle, track.PingComment)
	}
	if track.PingMarker == "" || !strings.Contains(track.PingComment, track.PingMarker) {
		t.Errorf("the ping comment must embed its dedup marker")
	}
	if !strings.Contains(track.Body, operatorHandle) {
		t.Errorf("the WAITING-ON-YOU body must @-mention %s", operatorHandle)
	}

	// Second run WITH the ping marker recorded → DEDUP: no re-ping (same state, same
	// age bucket).
	existing := map[string]bool{track.PingMarker: true, track.Marker: true}
	issues2 := driveIssues(streams, DriveSet{Active: []Drive{d}}, nil, now, existing, "")
	track2 := findTracking(t, issues2)
	if track2.Ping {
		t.Fatalf("a steady WAITING-ON-OPERATOR at the same age bucket must NOT re-ping (dedup): marker=%q", track2.PingMarker)
	}
}

func TestDriveRePingsOnAgeEscalation(t *testing.T) {
	// Day 1 (bucket 1: ≥24h) then day 3 (bucket 2: ≥72h) — the escalation re-pings
	// once, with a DIFFERENT marker.
	d1now := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC) // 1 day
	st1, streams := waitingOpSetup(t, "2026-08-14", d1now)
	first := findTracking(t, driveIssues(streams, DriveSet{Active: []Drive{st1.Drive}}, nil, d1now, map[string]bool{}, ""))
	if !first.Ping {
		t.Fatal("first WAITING-ON-OPERATOR ping (bucket 1) must fire")
	}

	// Now 3 days old, with the bucket-1 marker already recorded → re-ping at bucket 2.
	d3now := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	st3, _ := waitingOpSetup(t, "2026-08-14", d3now)
	existing := map[string]bool{first.PingMarker: true, first.Marker: true}
	esc := findTracking(t, driveIssues(streams, DriveSet{Active: []Drive{st3.Drive}}, nil, d3now, existing, ""))
	if !esc.Ping {
		t.Fatal("crossing the 72h escalation must re-ping once")
	}
	if esc.PingMarker == first.PingMarker {
		t.Errorf("the escalation ping must use a NEW dedup marker, got the same: %q", esc.PingMarker)
	}
}

// A non-WAITING-ON-OPERATOR drive never pings @operator, regardless of age.
func TestDriveNonBottleneckNeverPings(t *testing.T) {
	now := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	// Ready work present alongside an old operator-act → ROLLING, no ping.
	s := mkStream("hot", "active", "P1", mkBrief("01", "todo"))
	d := liveDrive("ship",
		DriveItem{Kind: "stream", Ref: "hot"},
		DriveItem{Kind: "operator-act", Owner: "operator", Unblocks: "x", Since: "2026-08-14"},
	)
	track := findTracking(t, driveIssues([]*Stream{s}, DriveSet{Active: []Drive{d}}, nil, now, map[string]bool{}, ""))
	if track.State != driveStateRolling {
		t.Fatalf("want ROLLING, got %q", track.State)
	}
	if track.Ping {
		t.Fatal("a ROLLING drive must never ping @operator even with an old operator-act")
	}
}

// --- fail-neutral: no active drive, malformed manifest ------------------------

func TestDriveIssuesEmptyWhenNoActiveDrive(t *testing.T) {
	issues := driveIssues(nil, DriveSet{}, nil, driveTestNow, map[string]bool{}, "")
	if len(issues) != 0 {
		t.Fatalf("no active drive → no issues, got %+v", issues)
	}
	// A NotApplied (fail-neutral) set also emits nothing — a rejected manifest never
	// opens an issue, exactly as it never freezes the board.
	issues = driveIssues(nil, DriveSet{NotApplied: true, Reason: "expired"}, nil, driveTestNow, map[string]bool{}, "")
	if len(issues) != 0 {
		t.Fatalf("a rejected (NotApplied) manifest must emit no issues, got %+v", issues)
	}
}

// --- markers file loading -----------------------------------------------------

func TestLoadDriveMarkers(t *testing.T) {
	// Empty path → empty set.
	if m, err := loadDriveMarkers(""); err != nil || len(m) != 0 {
		t.Fatalf("empty path must yield an empty set: %v %v", m, err)
	}
	// Extracts every marker flavour from a raw blob (tracking / act / ping).
	dir := t.TempDir()
	blob := "noise\n" + driveTrackingMarker("ship") + "\nmore\n" +
		driveActMarker("ship", 2) + " trailing\n" + drivePingMarker("ship", driveStateWaitingOp, 1) + "\n"
	path := dir + "/markers.txt"
	if err := os.WriteFile(path, []byte(blob), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadDriveMarkers(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{driveTrackingMarker("ship"), driveActMarker("ship", 2), drivePingMarker("ship", driveStateWaitingOp, 1)} {
		if !m[want] {
			t.Errorf("marker %q not extracted from the blob", want)
		}
	}
}

// --- helpers ------------------------------------------------------------------

func findTracking(t *testing.T, issues []driveIssue) driveIssue {
	t.Helper()
	for _, iss := range issues {
		if iss.Kind == "tracking" {
			return iss
		}
	}
	t.Fatalf("no tracking issue in %+v", issues)
	return driveIssue{}
}
