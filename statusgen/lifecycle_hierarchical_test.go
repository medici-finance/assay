package main

import "testing"

// TestLifecycleCellMatchesHierarchicalBriefID pins the real-world edge derived-
// board/07 rollout depends on: under brief-v2 the brief file's `brief:` id is the
// hierarchical <cell>:<repo>:<stream>:<NN> form (briefv2.go's parseBriefV2ID; the
// migration in migrations/0001-...-derived-board.md rewrites every id to this
// shape), but the `Brief:` trailer a PR body carries is the SHORT <stream>/<NN>
// form (deskpr/trailer.go's grammar; every real PR observed on
// medici-finance/assay writes it this way, e.g. "Brief: derived-board/07").
// DeriveLifecycle joined PRs to briefs on a bare map key with no reduction, so a
// brief-v2 tree's PR-derived cell could never leave `todo`, no matter how many
// merged, trailer-carrying PRs implemented it — the whole point of the derived
// board.
func TestLifecycleCellMatchesHierarchicalBriefID(t *testing.T) {
	got := deriveSingle(t, BriefIdent{ID: "assay:assay:derived-board:07", Gate: "model", Version: 1},
		LifecycleInput{LookedAt: true, PRs: []PRRecord{
			{BriefRef: "derived-board/07", Number: 736, State: prMerged, MergeSHA: "0123456789abcdef"},
		}})
	if got.Cell != "implemented" {
		t.Fatalf("want implemented (hierarchical brief id joined against short trailer), got %q (reason %q)", got.Cell, got.Reason)
	}
}
