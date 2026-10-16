package deskkit

import (
	"reflect"
	"strings"
	"testing"
)

// TestPerfLaunchRosterShape pins the launch roster to the names the cellctl launch records,
// keeps it disjoint from the boot roster deskboot is pinned to, and orders the boot report's
// steps after the boot steps.
// regression: #3099
func TestPerfLaunchRosterShape(t *testing.T) {
	if got := PerfLaunchSteps(); !reflect.DeepEqual(got, launchStepNames) {
		t.Fatalf("PerfLaunchSteps = %v, want %v", got, launchStepNames)
	}
	for _, s := range PerfLaunchSteps() {
		if !strings.HasPrefix(s, "launch-") {
			t.Errorf("%q lacks the launch- prefix", s)
		}
		for _, b := range PerfBootSteps() {
			if b == s {
				t.Errorf("launch step %q collides with a boot step", s)
			}
		}
	}
	recs := []PerfRecord{
		{Verb: "step:launch-worktree", Role: "none", WallMs: 3},
		{Verb: "step:zz-other", Role: "none", WallMs: 1},
		{Verb: "step:launch-storage-admission", Role: "none", WallMs: 9},
		{Verb: "step:token-mint", Role: "none", WallMs: 2},
	}
	sum := SummariseBootSteps(recs)
	var order []string
	for _, st := range sum.Roles[0].Steps {
		order = append(order, st.Step)
	}
	want := []string{"token-mint", "launch-storage-admission", "launch-worktree", "zz-other"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("step order = %v, want %v", order, want)
	}
}
