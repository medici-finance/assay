package main

// declarationhold_test.go — the board must not recommend a flip or a merge for a PR whose
// ready gate will refuse on an open desk-decided declaration hold (#2690). The hold is
// deskkit.DeskDecidedRefusal, the same reduction deskflip and `deskpost ready` run.

import (
	"strings"
	"testing"
)

const dhUndeclared = "Undeclared-desk-decision: picked a default without a ruling"

func dhPR(labels ...string) prBase {
	var p prBase
	p.Number = 2687
	p.HeadRefOid = beBoardHead
	p.Body = "plain body"
	for _, l := range labels {
		p.Labels = append(p.Labels, struct {
			Name string `json:"name"`
		}{Name: l})
	}
	return p
}

func dhCorrectness(state, body string) review {
	return beBoardReview(state, body, "2026-10-12T02:00:00Z")
}

func dhSecurity(body string) review {
	return beBoardReview("COMMENTED", body+"\nSecurity-Review: pass", "2026-10-12T02:01:00Z")
}

// dhClassify drives the real reduction into the pure classifier for an approved, green,
// risk-classed-and-passed row, the shape the issue observed.
func dhClassify(p prBase, reviews []review, draft bool) (string, string) {
	in := classifyInput{
		ever: true, atHead: true, approvedAtHead: true, draft: draft,
		pass: 3, ciGreen: true, riskClassed: true, securityPass: true,
		declarationHold: declarationHold(p, reviews),
	}
	return classify(in)
}

// regression: #2690
func TestDeclarationHoldBlocksRecommendation(t *testing.T) {
	cases := map[string]struct {
		p       prBase
		reviews []review
	}{
		"both lanes carry the line (the observed shape)": {dhPR(), []review{
			dhCorrectness("APPROVED", "Correctness ok.\n"+dhUndeclared), dhSecurity(dhUndeclared)}},
		"correctness lane only": {dhPR(), []review{
			dhCorrectness("APPROVED", "Correctness ok.\n"+dhUndeclared), dhSecurity("clean")}},
		"security lane only": {dhPR(), []review{
			dhCorrectness("APPROVED", "Correctness ok."), dhSecurity(dhUndeclared)}},
		"label without a body block": {dhPR("desk-decided"), []review{
			dhCorrectness("APPROVED", "Correctness ok."), dhSecurity("clean")}},
		"body block without the label": {func() prBase {
			p := dhPR()
			p.Body = "## Desk-decided\n\nno marker, no items\n"
			return p
		}(), []review{dhCorrectness("APPROVED", "Correctness ok."), dhSecurity("clean")}},
		"commented correctness note does not clear": {dhPR(), []review{
			dhCorrectness("APPROVED", "Correctness ok.\n"+dhUndeclared),
			dhCorrectness("COMMENTED", "just a note"), dhSecurity("clean")}},
		"security pass does not clear the correctness lane": {dhPR(), []review{
			dhCorrectness("APPROVED", "Correctness ok.\n"+dhUndeclared), dhSecurity("clean")}},
	}
	for name, c := range cases {
		for _, draft := range []bool{true, false} {
			t.Run(name, func(t *testing.T) {
				got, note := dhClassify(c.p, c.reviews, draft)
				if got == actMergeNow || got == actFlip {
					t.Fatalf("draft=%v classify() = %q on an open declaration hold (note: %s)", draft, got, note)
				}
				if got != actBlocked || !strings.Contains(note, "declaration hold") {
					t.Fatalf("draft=%v classify() = %q (%s), want BLOCKED naming the declaration hold", draft, got, note)
				}
			})
		}
	}
}

// regression: #2690
func TestDeclarationHoldClearedByDecisiveVerdict(t *testing.T) {
	cases := map[string][]review{
		"later APPROVED omits the line": {
			dhCorrectness("APPROVED", "Correctness ok.\n"+dhUndeclared),
			{User: dhCorrectness("", "").User, State: "APPROVED", CommitID: beBoardHead, Body: "Re-read; declared.",
				SubmittedAt: "2026-10-12T03:00:00Z"},
			dhSecurity("clean")},
		"later security pass omits the line": {
			dhCorrectness("APPROVED", "Correctness ok."), dhSecurity(dhUndeclared),
			{User: dhCorrectness("", "").User, State: "COMMENTED", CommitID: beBoardHead,
				Body: "Security-Review: pass", SubmittedAt: "2026-10-12T03:00:00Z"}},
		"no line anywhere (control)": {
			dhCorrectness("APPROVED", "Correctness ok."), dhSecurity("clean")},
	}
	for name, reviews := range cases {
		t.Run(name, func(t *testing.T) {
			got, note := dhClassify(dhPR(), reviews, true)
			if got != actMergeNow {
				t.Fatalf("classify() = %q (%s), want MERGE-NOW once the hold is cleared", got, note)
			}
		})
	}
}
