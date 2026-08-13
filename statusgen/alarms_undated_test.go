package main

import (
	"strings"
	"testing"
)

// Undated-finding guard for the FINDINGS alarms (docs/three-state-instrument-rule.md,
// sub-rule 1). A finding whose date heading does not parse used to be dropped from every
// alarm KPI with no output at all — a date typo silently removed it from the flood count
// and the standing-age list, and the report still described itself affirmatively.

// TestUndatedFindingIsUncountedButReported is the POSITIVE CONTROL: it pins the
// underlying drop that makes the guard necessary. The undated finding must still be absent
// from OpenedTotal / ActiveCount / Standing — the fix does NOT invent a date for it, which
// would be a different and worse lie. What changes is that the omission is now REPORTED.
// Without this control, the guard's own green test would prove only that a field is set.
func TestUndatedFindingIsUncountedButReported(t *testing.T) {
	now := mustTime(t, "2026-08-13")
	findings := []Finding{
		finding("F-dated", "2026-08-01", "a finding the instrument can date", false),
		finding("F-typo", "2026-8-1", "a finding whose date heading does not parse", false),
	}
	rep := computeAlarms(findings, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now)

	if rep.OpenedTotal != 1 {
		t.Errorf("OpenedTotal = %d, want 1 (the undated finding is genuinely uncounted)", rep.OpenedTotal)
	}
	if rep.ActiveCount != 1 {
		t.Errorf("ActiveCount = %d, want 1", rep.ActiveCount)
	}
	for _, a := range rep.Standing {
		if a.ID == "F-typo" {
			t.Fatal("F-typo appeared in Standing — the fix must report the gap, not fabricate a date")
		}
	}
	// The counts being a floor is exactly why it must be declared.
	if len(rep.Undated) != 1 || rep.Undated[0] != "F-typo" {
		t.Fatalf("Undated = %v, want [F-typo] — the drop is unreported, which is the defect", rep.Undated)
	}
}

// TestStandingAlarmNoticesReportUndated: the --lint NOTICE path must name the gap and call
// the counts a floor.
func TestStandingAlarmNoticesReportUndated(t *testing.T) {
	now := mustTime(t, "2026-08-13")
	findings := []Finding{finding("F-typo", "not-a-date", "undated", false)}

	notices := standingAlarmNotices(findings, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now)
	joined := strings.Join(notices, "\n")
	for _, want := range []string{"COULD-NOT-CHECK", "F-typo", "FLOOR"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notices missing %q; got:\n%s", want, joined)
		}
	}
}

// TestRenderAlarmsNeverClaimsCleanOverUndated is the sharp end: with the ONLY finding
// undated, Standing is empty and the renderer used to print the affirmative
// "_None — no unresolved findings._" — a clean read of a register it could not read.
func TestRenderAlarmsNeverClaimsCleanOverUndated(t *testing.T) {
	now := mustTime(t, "2026-08-13")
	findings := []Finding{finding("F-typo", "", "no date at all", false)}

	out := renderAlarms(computeAlarms(findings, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now))
	if strings.Contains(out, "None — no unresolved findings") {
		t.Errorf("renderAlarms claimed a clean register while a finding was undated:\n%s", out)
	}
	for _, want := range []string{"Could-not-check", "F-typo", "floor, not a total"} {
		if !strings.Contains(out, want) {
			t.Errorf("render missing %q; got:\n%s", want, out)
		}
	}
}

// TestRenderAlarmsCleanRegisterStillReadsClean is the guard against over-firing: a register
// whose findings all parse must keep its original affirmative wording.
func TestRenderAlarmsCleanRegisterStillReadsClean(t *testing.T) {
	now := mustTime(t, "2026-08-13")
	out := renderAlarms(computeAlarms(nil, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now))
	if !strings.Contains(out, "None — no unresolved findings") {
		t.Errorf("a genuinely empty register lost its clean line:\n%s", out)
	}
	if strings.Contains(out, "Could-not-check") {
		t.Errorf("could-not-check section rendered with nothing undated:\n%s", out)
	}
}
