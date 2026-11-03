package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func snapshotVisibility(v string) *string { return &v }

func policyFromSweep(t *testing.T, res sweepResult) policyDriftAlarm {
	t.Helper()
	boardRows(t, res)
	var report struct {
		PolicyDrift *policyDriftAlarm `json:"policyDrift"`
	}
	if err := json.Unmarshal(res.stdout, &report); err != nil || report.PolicyDrift == nil {
		t.Fatalf("policy-drift alarm absent: %v\n%s", err, res.stdout)
	}
	return *report.PolicyDrift
}

// regression: #4885
// The exact same live queue snapshot already identifies the repository and can
// carry its visibility. A complete actions sweep must consume that observation
// instead of issuing an otherwise redundant metadata request.
func TestActionsUsesLiveVisibility(t *testing.T) {
	ff := newEtagForge(t, 3)
	ff.snapshotVisibility = snapshotVisibility("PRIVATE")
	for _, name := range []string{"cold", "unchanged"} {
		res := ff.sweep(t)
		alarm := policyFromSweep(t, res)
		if len(alarm.Drift) != 0 || len(alarm.Scope) != 1 || alarm.Scope[0] != fakeRepo {
			t.Fatalf("%s: snapshot policy observation changed: %+v", name, alarm)
		}
		if contains(res.all, "GET /repos/"+fakeRepo) {
			t.Fatalf("%s: actions re-read repository metadata despite live snapshot visibility: %v", name, res.all)
		}
		if !contains(res.all, "POST /graphql") {
			t.Fatalf("%s: no fresh queue read: %v", name, res.all)
		}
	}
}

// regression: #4885
// Cross-run reuse is not licensed: the next snapshot can report a visibility
// change while the PR's head, check rollup and per-PR fact key stay unchanged.
func TestActionsVisibilityStaysFresh(t *testing.T) {
	ff := newEtagForge(t, 2)
	ff.snapshotVisibility = snapshotVisibility("PRIVATE")
	if alarm := policyFromSweep(t, ff.sweep(t)); len(alarm.Drift) != 0 {
		t.Fatalf("private fixture should match configured policy: %+v", alarm)
	}
	ff.mu.Lock()
	ff.snapshotVisibility = snapshotVisibility("PUBLIC")
	ff.mu.Unlock()
	res := ff.sweep(t)
	alarm := policyFromSweep(t, res)
	if len(alarm.Drift) != 1 || !strings.Contains(alarm.Drift[0], fakeRepo) {
		t.Fatalf("fresh visibility change was hidden: %+v", alarm)
	}
	if contains(res.all, "GET /repos/"+fakeRepo) {
		t.Fatalf("fresh snapshot unexpectedly used the older metadata fixture: %v", res.all)
	}
}

func TestActionsVisibilityFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		visibility *string
	}{
		{"older-backend", nil},
		{"empty", snapshotVisibility("")},
		{"unknown-enum", snapshotVisibility("FUTURE_VISIBILITY")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ff := newEtagForge(t, 1)
			ff.snapshotVisibility = tc.visibility
			res := ff.sweep(t)
			alarm := policyFromSweep(t, res)
			if len(alarm.Drift) != 0 || !contains(res.all, "GET /repos/"+fakeRepo) {
				t.Fatalf("unavailable snapshot must retain the explicit live metadata probe: alarm=%+v calls=%v", alarm, res.all)
			}
		})
	}
}

func TestActionsVisibilityFailureLoud(t *testing.T) {
	ff := newEtagForge(t, 1)
	ff.snapshotVisibility = snapshotVisibility("FUTURE_VISIBILITY")
	ff.metadataFailure = true
	res := ff.sweep(t)
	alarm := policyFromSweep(t, res)
	if len(alarm.Drift) != 1 || !strings.Contains(alarm.Drift[0], "NOT OBSERVED") || !strings.Contains(alarm.Drift[0], fakeRepo) {
		t.Fatalf("unreadable visibility became a clean policy result: %+v", alarm)
	}
	if !contains(res.all, "GET /repos/"+fakeRepo) {
		t.Fatalf("no fallback was attempted: %v", res.all)
	}
}
