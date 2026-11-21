package main

// declarationhold_sweep_test.go — #2690 through the REAL row path. A held PR goes through
// sweepActionsRepo → classifyPRFrom (the production wiring that computes the hold) and the
// resolved observation (the review-queue path, resolveObservation → BuildObservation's
// Classify). Both must read BLOCKED; the same PR without the hold must not, so the fixture
// provably reaches the approved arm the hold overrides.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

func dhSweep(t *testing.T, draft bool, body string) (actionRow, string) {
	t.Helper()
	const repo = "example-org/tracker"
	head := strings.Repeat("a", 40)
	p := greenRollupPR(70, head)
	p.MergeStateStatus = "CLEAN"
	p.Mergeable = "MERGEABLE"
	p.IsDraft = draft
	p.Author.Login = "shared-agent"
	// The fake forge's diff touches a security path, so the row is risk-classed: a security
	// pass at head is part of the approved shape the issue observed.
	reviewer := deskkit.Account{Login: "gl-reviewer", ID: 41987965}
	reviews := []deskkit.Review{
		{ID: 1, Author: reviewer, State: "APPROVED", CommitID: head, Body: body, SubmittedAt: "2026-09-14T16:00:00Z"},
		{ID: 2, Author: reviewer, State: "COMMENTED", CommitID: head, Body: "Security-Review: pass", SubmittedAt: "2026-09-14T16:01:00Z"},
	}
	js, err := json.Marshal([]prBase{p})
	if err != nil {
		t.Fatal(err)
	}
	installBoardRoster(t, classDegradeRoster)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON", string(js))
	stubForgeHooks(t, forgeHookSet{
		reviews:      func(string, int) ([]deskkit.Review, error) { return reviews, nil },
		queueReviews: func(string, int) ([]deskkit.Review, error) { return reviews, nil },
	})
	part, err := sweepActionsRepo(repo, nil, nil, nil, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("sweepActionsRepo: %v", err)
	}
	if len(part.rows) != 1 || len(part.resolved) != 1 {
		t.Fatalf("want one row and one resolved observation, got %d/%d", len(part.rows), len(part.resolved))
	}
	obs, _ := rf.Classify(part.resolved[0].facts.ClassifyInput(), reviewerBotDisplay())
	return part.rows[0], obs
}

// regression: #2690
func TestDeclarationHoldThroughSweep(t *testing.T) {
	for _, draft := range []bool{true, false} {
		row, obs := dhSweep(t, draft, "Verdict: approve\n"+dhUndeclared)
		if row.Action != actBlocked || !strings.Contains(row.Note, "declaration hold") {
			t.Errorf("draft=%v actions row = %q (%s), want BLOCKED naming the declaration hold", draft, row.Action, row.Note)
		}
		if obs != actBlocked {
			t.Errorf("draft=%v review-queue observation classifies %q, want BLOCKED on the same held PR", draft, obs)
		}
		ctlRow, ctlObs := dhSweep(t, draft, "Verdict: approve")
		if ctlRow.Action != actFlip && ctlRow.Action != actMergeNow {
			t.Errorf("draft=%v control row = %q (%s), want FLIP or MERGE-NOW — the fixture never reaches the arm the hold overrides", draft, ctlRow.Action, ctlRow.Note)
		}
		if ctlObs != ctlRow.Action {
			t.Errorf("draft=%v control: observation %q differs from actions row %q", draft, ctlObs, ctlRow.Action)
		}
	}
}
