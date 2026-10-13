package deskkit

import (
	"strings"
	"testing"
)

// #2760: each StampKind reads and repairs ONLY its own pair, and the dispatch kind is
// exactly the pre-existing dispatched-* behaviour the floor reads.

func TestStampKindsAreDisjoint(t *testing.T) {
	d, err := StampDispatch.Labels("example-model-1", "strong")
	if err != nil {
		t.Fatal(err)
	}
	a, err := StampAuthored.Labels("example-model-2", "any")
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := ModelStampLabels("example-model-1", "strong"); strings.Join(d, ",") != strings.Join(want, ",") {
		t.Fatalf("dispatch kind labels %v differ from ModelStampLabels %v", d, want)
	}
	for _, l := range a {
		if !strings.HasPrefix(l, "authored-") {
			t.Errorf("authored label %q outside the authored-* namespace", l)
		}
	}
	both := append(append([]string{}, d...), a...)
	if got, st := ModelStampOf(both); st != ModelStamped || got.Model != "example-model-1" {
		t.Errorf("the floor's content reader saw the authored pair: %+v %v", got, st)
	}
	if got, st := StampAuthored.StampOf(both); st != ModelStamped || got.Model != "example-model-2" || got.Tier != "any" {
		t.Errorf("the authored reader saw the dispatched pair: %+v %v", got, st)
	}
}

func TestAuthoredRestampSparesReview(t *testing.T) {
	trusted := func(string) bool { return true }
	tl := StampTimeline{
		Present: []string{"dispatched-model:review-model", "dispatched-tier:strong", "authored-model:old"},
		Events: []LabelEvent{
			{Name: "dispatched-model:review-model", AppliedBy: "r"},
			{Name: "dispatched-tier:strong", AppliedBy: "r"},
			{Name: "authored-model:old", AppliedBy: "d"},
		},
	}
	want, _ := StampAuthored.Labels("new-model", "strong")
	got := StampAuthored.ReStampRemovals(tl, want, trusted)
	if strings.Join(got, ",") != "authored-model:old" {
		t.Errorf("authored re-stamp removals = %v, want only the stale authored-model:old", got)
	}
	wantD, _ := StampDispatch.Labels("review-model-2", "strong")
	gotD := ReStampRemovals(tl, wantD, trusted)
	if strings.Join(gotD, ",") != "dispatched-model:review-model" {
		t.Errorf("review re-stamp removals = %v, want only the stale dispatched-model slug", gotD)
	}
	if _, st := StampAuthored.AttestedStampOf(tl, trusted); st != ModelIndeterminate {
		t.Errorf("a half authored stamp must read indeterminate, got %v", st)
	}
	if got, st := AttestedModelStampOf(tl, trusted); st != ModelStamped || got.Model != "review-model" {
		t.Errorf("the floor's reader was disturbed by an authored label: %+v %v", got, st)
	}
}

// regression: #2760
func TestAuthoredOnlyFloor(t *testing.T) {
	trusted := func(string) bool { return true }
	tl := StampTimeline{
		Present: []string{"authored-model:example-model", "authored-tier:strong"},
		Events: []LabelEvent{
			{Name: "authored-model:example-model", AppliedBy: "authority"},
			{Name: "authored-tier:strong", AppliedBy: "authority"},
		},
	}
	if got := ModelCapabilityFloor(tl, trusted, false, ClaimLivenessUnknown); got.State != ModelUnknown || got.Outcome != FloorNoticeAllow {
		t.Errorf("authored-only ordinary floor = %+v; want unstamped notice", got)
	}
	if got := ModelCapabilityFloorRiskAware(tl, trusted, false, ClaimLivenessUnknown, FloorRiskClassed); got.State != ModelUnknown || got.Outcome != FloorRefuse {
		t.Errorf("authored-only risk floor = %+v; want unstamped refusal", got)
	}
}
