package main

import (
	"strings"
	"testing"
)

// sampleDriveStatus builds a DriveStatus with frontier rows covering every
// state, in a deliberately scrambled order, so order assertions prove the
// renderer orders, not the fixture.
func sampleDriveStatus() DriveStatus {
	d := Drive{
		Slug:       "board-liveness",
		DeclaredBy: "ian",
		Starts:     "2026-08-20",
		Expires:    "2026-08-30",
		Intensity:  "push",
		Why:        "prove the fleet notices a frozen board",
		State:      "active",
	}
	fr := []FrontierItem{
		{Kind: "brief", Ref: "desk-hardening/01", State: fsBlockedReview},
		{Kind: "brief", Ref: "agentic-metrics/04", State: fsInFlight},
		{Kind: "operator-act", State: fsBlockedOperator, Unblocks: "sign the release", Since: "2026-08-20", AgeDays: 2},
		{Kind: "brief", Ref: "education/11", State: fsReady},
		{Kind: "operator-act", State: fsBlockedOperator, Unblocks: "grant the token", Since: "2026-08-22", AgeDays: 0},
		{Kind: "brief", Ref: "methodology/48", State: fsDone},
		{Kind: "issue", Ref: "example-org/reconciler#27", State: fsTracked},
		{Kind: "plan-gap", State: fsNeedsBrief},
	}
	return DriveStatus{Drive: d, Frontier: fr, State: driveState(fr)}
}

// TestDriveDashboardSectionOrder is Verify row D: the `## Drive:` section
// renders operator-slice-FIRST (state banner + heartbeat, then the
// WAITING-ON-YOU `act · unblocks · age · issue` table), then progress
// done/total, in-flight, blocked-on-review, frontier-next.
func TestDriveDashboardSectionOrder(t *testing.T) {
	const heartbeat = "1b4e4a4 2026-08-22T10:00:00+00:00"
	sec := driveDashboardSection(sampleDriveStatus(), heartbeat)

	order := []string{
		"## Drive: `board-liveness`",
		"**State:** `ROLLING`",             // state banner
		"_last regen: " + heartbeat + "_",  // heartbeat line
		"| Act | Unblocks | Age | Issue |", // WAITING-ON-YOU table
		"**Progress:**",
		"**In-flight:**",
		"**Blocked on review:**",
		"**Frontier next:**",
	}
	last := -1
	for _, want := range order {
		idx := strings.Index(sec, want)
		if idx < 0 {
			t.Fatalf("section lacks %q:\n%s", want, sec)
		}
		if idx <= last {
			t.Errorf("%q appears before the previous element (idx %d <= %d):\n%s", want, idx, last, sec)
		}
		last = idx
	}

	// The WAITING-ON-YOU table carries both operator acts, oldest first, with
	// act · unblocks · age · issue columns.
	if !strings.Contains(sec, "| act 1 | sign the release | 2 days |") {
		t.Errorf("oldest act row missing from the table:\n%s", sec)
	}
	if !strings.Contains(sec, "| act 2 | grant the token | today | tracking issue |") {
		t.Errorf("younger act row missing from the table:\n%s", sec)
	}
	if !strings.Contains(sec, "aging issue (act 1)") {
		t.Errorf("aged act must reference its aging issue:\n%s", sec)
	}
	if !strings.Contains(sec, "tracking issue") {
		t.Errorf("fresh act must reference the tracking issue:\n%s", sec)
	}

	// Absent ⇒ inert: no active drives renders nothing (the safety bar).
	if got := driveSections(nil, heartbeat); got != "" {
		t.Errorf("driveSections with no drives = %q, want empty", got)
	}
	if got := driveSections([]DriveStatus{}, heartbeat); got != "" {
		t.Errorf("driveSections with empty statuses = %q, want empty", got)
	}
}

// TestDriveTrackingIssueMirrorsBanner is Verify row M: the phase-2 tracking
// issue body mirrors the dashboard's banner + WAITING-ON-YOU slice; same
// `drive:<slug>` label + idempotency marker (no second issue), and the phase-2
// ping timing is unchanged.
func TestDriveTrackingIssueMirrorsBanner(t *testing.T) {
	const heartbeat = "1b4e4a4 2026-08-22T10:00:00+00:00"
	st := sampleDriveStatus()

	track := trackingIssue(st, map[string]bool{}, heartbeat)

	if track.Kind != "tracking" {
		t.Errorf("kind = %q, want tracking", track.Kind)
	}
	if track.Marker != driveTrackingMarker("board-liveness") {
		t.Errorf("marker = %q, want %q", track.Marker, driveTrackingMarker("board-liveness"))
	}
	if len(track.Labels) != 2 || track.Labels[0] != driveTrackingLabel ||
		track.Labels[1] != driveLabelPrefix+"board-liveness" {
		t.Errorf("labels = %v, want [%s %sboard-liveness]", track.Labels, driveTrackingLabel, driveLabelPrefix)
	}
	// The body mirrors the dashboard's operator slice — same state banner,
	// same heartbeat, same 4-column table.
	if !strings.Contains(track.Body, "**State:** `ROLLING`") {
		t.Errorf("tracking body lacks the mirrored state banner")
	}
	if !strings.Contains(track.Body, "_last regen: "+heartbeat+"_") {
		t.Errorf("tracking body lacks the mirrored heartbeat line")
	}
	if !strings.Contains(track.Body, "| Act | Unblocks | Age | Issue |") {
		t.Errorf("tracking body lacks the mirrored WAITING-ON-YOU table")
	}
	if !strings.Contains(track.Body, "| act 1 | sign the release | 2 days |") {
		t.Errorf("tracking body lacks the mirrored oldest act row")
	}

	// Ping timing unchanged: ROLLING never pings, whatever markers exist.
	if track.Ping {
		t.Errorf("ROLLING drive pinged — phase-4 must not change ping timing")
	}
	if ping, _ := drivePingDecision("board-liveness", driveStateRolling, 3, map[string]bool{}); ping {
		t.Errorf("drivePingDecision pings on ROLLING — phase-4 must not change ping timing")
	}
}
