package reviewflow

import (
	"strings"
	"testing"
)

func TestBlockedPolicyRouting(t *testing.T) {
	for _, tc := range []struct {
		name, state, mergeable string
		draft                  bool
		want                   string
	}{
		{"draft policy hold", "BLOCKED", "MERGEABLE", true, ActFlip},
		{"ready policy hold", "BLOCKED", "MERGEABLE", false, ActCheck},
		{"conflict", "BLOCKED", "CONFLICTING", true, ActConflict},
		{"dirty wins", "DIRTY", "MERGEABLE", true, ActConflict},
		{"clean but conflicting", "CLEAN", "CONFLICTING", true, ActConflict},
		{"unknown conflict state", "BLOCKED", "UNKNOWN", true, ActMergeStateUnknown},
		{"absent conflict state", "BLOCKED", "", true, ActMergeStateUnknown},
		{"unrecognized conflict state", "BLOCKED", "FUTURE", true, ActMergeStateUnknown},
		{"clean unchanged", "CLEAN", "MERGEABLE", true, ActMergeNow},
		{"behind unchanged", "BEHIND", "MERGEABLE", false, ActMergeBehind},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := ChangeFact{MergeStateStatus: tc.state, Mergeable: tc.mergeable, IsDraft: tc.draft,
				Checks: []CheckFact{{Status: "COMPLETED", Conclusion: "SUCCESS"}}}
			in := BuildClassifyInput(p, ReviewState{Ever: true, AtHead: true, Approved: true}, true, "")
			got, note := Classify(in, "reviewer")
			if got != tc.want {
				t.Fatalf("action=%s want=%s: %s", got, tc.want, note)
			}
			if tc.state == "BLOCKED" && tc.mergeable == "MERGEABLE" && !strings.Contains(note, "MERGE-NOW is WITHHELD") {
				t.Fatalf("policy hold disappeared: %s", note)
			}
		})
	}
}

func TestBlockedPolicyRetainsReviewCISecurityAndHumanGates(t *testing.T) {
	p := ChangeFact{MergeStateStatus: "BLOCKED", Mergeable: "MERGEABLE", IsDraft: true,
		Checks: []CheckFact{{Status: "COMPLETED", Conclusion: "SUCCESS"}}}
	base := BuildClassifyInput(p, ReviewState{Ever: true, AtHead: true, Approved: true}, true, "")
	for _, tc := range []struct {
		name, want string
		change     func(*ClassifyInput)
	}{
		{"blocking review", ActBlocked, func(in *ClassifyInput) { in.Blocking = true }},
		{"missing review", ActNeedsReview, func(in *ClassifyInput) { in.Ever = false }},
		{"CI failed", ActCIRed, func(in *ClassifyInput) { in.CiGreen = false; in.Fail = 1 }},
		{"CI pending", ActWaitCI, func(in *ClassifyInput) { in.CiGreen = false; in.Pending = 1 }},
		{"CI unknown", ActCIUnknown, func(in *ClassifyInput) { in.CiUnknown = 1 }},
		{"CI absent", ActCIUnverified, func(in *ClassifyInput) { in.CiGreen = false; in.Pass = 0 }},
		{"security required", ActSecReview, func(in *ClassifyInput) { in.RiskClassed = true }},
		{"human gate", ActHumanGate, func(in *ClassifyInput) { in.HumanGate = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.change(&in)
			got, note := Classify(in, "reviewer")
			if got != tc.want {
				t.Fatalf("got %s want %s: %s", got, tc.want, note)
			}
		})
	}
}
