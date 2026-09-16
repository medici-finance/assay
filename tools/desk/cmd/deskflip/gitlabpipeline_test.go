package main

// gitlabpipeline_test.go — the flip gate's half of issue #1125, pinned from the gate's side.
//
// The GitLab backend now publishes the head PIPELINE into the rollup as a status context named
// deskkit.GitLabPipelineContext — the same name RequiredStatusChecks returns on a project that
// gates the merge on the pipeline. This test runs the gate's OWN reduction over the rollup that
// backend produces (the shape pinned by the checks_at_head_green_mr_pipeline golden) and asserts
// the two conclusions the reported refusal turned on: the rollup is green, and no required
// context is missing from it.
//
// The gate code itself is unchanged by the fix. That is the point: the required set and the
// rollup are now spelled from ONE constant, so the gate's existing name comparison answers
// correctly instead of demanding a verdict nothing ever published.

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// glGreenMRChecks is the ChecksAtHead a green single-job `merge_request_event` pipeline
// produces, field for field as testdata/forge_gitlab_golden/checks_at_head_green_mr_pipeline
// pins it: the pipeline as a status context, the one job as a completed green check run.
func glGreenMRChecks() *deskkit.ChecksAtHead {
	return &deskkit.ChecksAtHead{
		CombinedState:    "success",
		StatusTotalCount: 1,
		Statuses: []deskkit.StatusContext{
			{State: "success", Context: deskkit.GitLabPipelineContext, CreatedAt: "2026-09-15T10:00:00Z"},
		},
		CheckRunsTotalCount: 1,
		CheckRuns: []deskkit.CheckRun{
			{ID: "9101", Name: "statusgen-lint", Status: "completed", Conclusion: "success",
				CompletedAt: "2026-09-15T10:04:00Z"},
		},
	}
}

func TestGitLabGreenPipelineSatisfiesChecksGreen(t *testing.T) {
	checks := glGreenMRChecks()
	entries, err := readChecks(flipOpts{}, func() (*deskkit.ChecksAtHead, error) { return checks, nil }, "abc123")
	if err != nil {
		t.Fatalf("readChecks over a complete GitLab rollup: %v", err)
	}
	reduced := latestPerRollupName(entries)
	if got := evalRollup(reduced); got != ciGreen {
		t.Errorf("evalRollup of a green MR pipeline = %v, want ciGreen (failed: %v)", got, failedChecks(reduced))
	}
	// The condition that refused in the field: a rollup that is green but missing a required
	// verdict. With the pipeline published under the name the gate requires, nothing is missing.
	required := []string{deskkit.GitLabPipelineContext}
	if missing := missingRequiredChecks(reduced, required); len(missing) != 0 {
		t.Errorf("required checks missing from a green MR pipeline's rollup = %v, want none — this is "+
			"the exit-6 'an absent required verdict is could-not-check' refusal #1125 reported", missing)
	}
}

func TestGitLabAbsentPipelineRefusesChecksGreen(t *testing.T) {
	// Same project, same gate — but nothing ran at this head, so the rollup carries only an
	// unrelated externally-posted commit status. The required pipeline verdict is ABSENT, and
	// absence is could-not-check: the gate must still refuse.
	checks := &deskkit.ChecksAtHead{
		CombinedState:    "success",
		StatusTotalCount: 1,
		Statuses: []deskkit.StatusContext{
			{State: "success", Context: "external/policy", CreatedAt: "2026-09-15T09:00:00Z"},
		},
	}
	entries, err := readChecks(flipOpts{}, func() (*deskkit.ChecksAtHead, error) { return checks, nil }, "abc123")
	if err != nil {
		t.Fatalf("readChecks: %v", err)
	}
	reduced := latestPerRollupName(entries)
	missing := missingRequiredChecks(reduced, []string{deskkit.GitLabPipelineContext})
	if len(missing) != 1 || missing[0] != deskkit.GitLabPipelineContext {
		t.Errorf("missing required checks at a head with no pipeline = %v, want [%s] — a head whose "+
			"pipeline never ran must not flip on an unrelated green status",
			missing, deskkit.GitLabPipelineContext)
	}
}

// TestGitHubRequiredContextsUnchanged is the regression guard on the OTHER backend: named
// GitHub required contexts are still matched by name, a missing one still refuses, and nothing
// about the GitLab mapping loosened that. The out-of-band leak-sweep disclosure gate is the
// live case — its verdict is a commit status that may simply never arrive at a head.
func TestGitHubRequiredContextsUnchanged(t *testing.T) {
	entries := []rollupEntry{
		{Name: "go-test", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{Context: "changelog", State: "SUCCESS"},
	}
	if got := evalRollup(entries); got != ciGreen {
		t.Fatalf("evalRollup of a green GitHub rollup = %v, want ciGreen", got)
	}
	if missing := missingRequiredChecks(entries, []string{"go-test", "changelog"}); len(missing) != 0 {
		t.Errorf("required contexts present in the rollup reported missing: %v", missing)
	}
	missing := missingRequiredChecks(entries, []string{"go-test", "leak-sweep"})
	if len(missing) != 1 || missing[0] != "leak-sweep" {
		t.Errorf("a required GitHub context that did not report = %v, want [leak-sweep] — an absent "+
			"required verdict stays could-not-check on GitHub exactly as before", missing)
	}
}
