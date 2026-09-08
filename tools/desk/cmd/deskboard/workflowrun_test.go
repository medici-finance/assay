package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestFetchOpenPRs_TwoShapeRollupSurvives is the deskboard half of the actions:read regression
// guard. The bulk open-PR read must not depend on the `checkSuite.workflowRun` link (it needs
// `actions:read`, which the reviewer App does not hold) — that constraint now lives in the
// forge backend's query (ghOpenChangesQuery, guarded in
// internal/deskkit/openchangesquery_test.go, which asserts the query never requests
// checkSuite/workflowRun). This test guards the deskboard side of the same contract: the
// rollup the typed ListOpenChanges op returns — BOTH node shapes, from checks:read alone —
// maps through fetchOpenPRs into prBase and classifies as green, so the CI verdict the board
// rests on survives the migration off the CLI.
func TestFetchOpenPRs_TwoShapeRollupSurvives(t *testing.T) {
	stubForgeList(t, func(repo string) (*deskkit.OpenChanges, error) {
		return &deskkit.OpenChanges{Cap: prListLimit, Changes: []deskkit.OpenChange{{
			Number: 7, Title: "t", State: "OPEN", Draft: true,
			Author:           deskkit.Account{Login: "assay-worker-app[bot]"},
			CreatedAt:        "2026-01-01T00:00:00Z",
			HeadSHA:          "abc123",
			HeadRef:          "feat/x",
			BaseRef:          "main",
			MergeStateStatus: "BLOCKED",
			Rollup: []deskkit.RollupNode{
				{Typename: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: "SUCCESS"},
				{Typename: "StatusContext", Context: "legacy", State: "SUCCESS"},
			},
		}}}, nil
	})

	prs, _, err := fetchOpenPRs("example-org/tracker")
	if err != nil {
		t.Fatalf("fetchOpenPRs must not fail; the board is blinded otherwise. got exit %d: %v",
			deskkit.ExitCodeOf(err), err)
	}
	if len(prs) != 1 || prs[0].Number != 7 {
		t.Fatalf("expected the one open PR to be read; got %+v", prs)
	}
	// The rollup the board classifies on must survive — both node shapes, from checks:read.
	if got := len(prs[0].StatusCheckRollup); got != 2 {
		t.Fatalf("expected 2 rollup contexts (CheckRun + StatusContext); got %d", got)
	}
	pass, pending, fail, unknown := ciState(prs[0])
	if pass != 2 || pending != 0 || fail != 0 || unknown != 0 {
		t.Errorf("rollup must classify as 2 pass; got pass=%d pending=%d fail=%d unknown=%d",
			pass, pending, fail, unknown)
	}
}
