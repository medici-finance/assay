package main

// gitlab_pipeline_boardread_test.go — the board's half of issue #1125.
//
// Its sibling gitlab_boardread_test.go pins the DEGRADED shape: a change whose head has no
// pipeline to read carries the deskkit.GitLabRollupUnmapped sentinel and must classify
// CI-UNKNOWN. That guard stays exactly as it was. What the backend no longer does is serve that
// sentinel for EVERY change: where GitLab ran a pipeline at the change's head SHA, the rollup
// now carries it as a real status context, and the board must read that as ordinary CI.
//
// Both halves live next to each other on purpose — the pair is what says "interpretable when
// the forge answered, could-not-check when it did not", rather than one blanket reading.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glMappedPipelineRow is the prBase a change with a green head pipeline produces, as
// testdata/forge_gitlab_golden/list_open_changes_head_pipeline_mapped pins the backend's
// output: one StatusContext rollup node named `pipeline` carrying the pipeline's state.
func glMappedPipelineRow(state string) prBase {
	return prBase{
		Number: 7, Title: "add the thing", HeadRefOid: "abc123",
		MergeStateStatus: "CLEAN",
		StatusCheckRollup: []check{{
			TypeName: "StatusContext", Context: deskkit.GitLabPipelineContext,
			State: state, CreatedAt: "2026-09-15T10:00:00Z",
		}},
	}
}

func TestGitLabMappedPipelineIsReadableCI(t *testing.T) {
	for _, tc := range []struct {
		state                        string
		pass, pending, fail, unknown int
	}{
		{"success", 1, 0, 0, 0},
		{"pending", 0, 1, 0, 0},
		{"failure", 0, 0, 1, 0},
	} {
		pass, pending, fail, unknown := ciState(glMappedPipelineRow(tc.state))
		if pass != tc.pass || pending != tc.pending || fail != tc.fail || unknown != tc.unknown {
			t.Errorf("ciState of a mapped %q pipeline = pass=%d pending=%d fail=%d unknown=%d, want %d/%d/%d/%d",
				tc.state, pass, pending, fail, unknown, tc.pass, tc.pending, tc.fail, tc.unknown)
		}
	}

	// The row the report turned on: reviewer approved at head, pipeline green, no conflict.
	// It must no longer land on CI-UNKNOWN — the board said "1 check-rollup entry could not be
	// interpreted" about an MR whose pipeline had succeeded.
	approved := reviewState{ever: true, atHead: true, approved: true}
	for _, ciRequired := range []bool{true, false} {
		action, note := classify(buildClassifyInput(glMappedPipelineRow("success"), approved, ciRequired, ""))
		if action == actCIUnknown || action == actCIUnverified {
			t.Errorf("approved GitLab change with a GREEN head pipeline (ciRequired=%v) → action=%s; "+
				"the CI verdict IS established (#1125). note: %s", ciRequired, action, note)
		}
	}

	// And the fail-closed direction is untouched: a RED pipeline is a red row, never a flip.
	action, _ := classify(buildClassifyInput(glMappedPipelineRow("failure"), approved, true, ""))
	if action == actFlip || action == actMergeNow {
		t.Errorf("approved GitLab change with a FAILED head pipeline reached %s", action)
	}
}
