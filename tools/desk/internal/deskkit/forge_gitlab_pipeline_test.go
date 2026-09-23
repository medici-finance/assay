package deskkit

// forge_gitlab_pipeline_test.go — the cross-instrument property issue #1125 reported broken:
// the flip gate and the board read ONE GitLab pipeline ONE way.
//
// The report had two halves on a single merge request whose `merge_request_event` pipeline
// was green at the head: the flip gate refused because the project required a status check
// named `pipeline` that "did not report on this head at all", while the board classified the
// same change CI-UNKNOWN because its rollup carried one entry it could not interpret. Both
// halves are the same defect — a required verdict, and a board rollup, that no backend read
// ever populated from the pipeline the forge actually ran.
//
// These tests assert against the BACKEND, not against a hand-built shape, so they fail on the
// unfixed reader rather than on a fixture nobody serves.

import (
	"sort"
	"strings"
	"testing"
)

// glPipelineGated is the project fixture for the adopter's configuration: GitLab gates the
// merge on the pipeline (`only_allow_merge_if_pipeline_succeeds`), which is the setting that
// makes RequiredStatusChecks answer non-empty.
func glPipelineGated(s *glServer) {
	s.project = map[string]any{"only_allow_merge_if_pipeline_succeeds": true}
}

// rollupLabels flattens a ChecksAtHead into the labels the flip gate matches required contexts
// against — a status context's Context, a check run's Name. It mirrors the gate's own
// comparison (trimmed, case-insensitive) so this test asks the question the gate asks.
func rollupLabels(c *ChecksAtHead) map[string]bool {
	out := map[string]bool{}
	for _, s := range c.Statuses {
		out[strings.ToLower(strings.TrimSpace(s.Context))] = true
	}
	for _, r := range c.CheckRuns {
		out[strings.ToLower(strings.TrimSpace(r.Name))] = true
	}
	return out
}

// TestGitLabGreenMRPipelineReachesBothInstruments is the #1125 regression, stated as the
// property both instruments must share: every context the project REQUIRES is a context the
// rollup CARRIES, and the board's bulk read of the same head yields an interpretable green
// entry rather than the could-not-check sentinel.
func TestGitLabGreenMRPipelineReachesBothInstruments(t *testing.T) {
	s := newGLServer(t)
	glPipelineGated(s)
	s.commit = map[string]any{"id": "abc123", "status": "success",
		"last_pipeline": map[string]any{"id": 77, "sha": "abc123", "status": "success",
			"source": "merge_request_event", "created_at": "2026-09-15T10:00:00Z"}}
	s.jobs = []map[string]any{
		{"id": 9101, "name": "statusgen-lint", "status": "success",
			"finished_at": "2026-09-15T10:04:00Z"},
	}
	s.mrList = []map[string]any{glMR(map[string]any{"iid": 7, "created_at": "2026-09-01T09:00:00Z"})}
	s.pipelines = []map[string]any{
		{"id": 77, "sha": "abc123", "status": "success", "source": "merge_request_event",
			"created_at": "2026-09-15T10:00:00Z"},
	}
	f := s.forge()

	// --- the flip gate's half -------------------------------------------------------------
	required, err := f.RequiredStatusChecks(glRepo, "main")
	if err != nil {
		t.Fatalf("RequiredStatusChecks: %v", err)
	}
	if len(required) == 0 {
		t.Fatalf("a pipeline-gated project reported NO required checks — the absent-rollup arm " +
			"would then read as 'nothing gates the merge'")
	}
	checks, err := f.ChecksAtHead(glRepo, "abc123")
	if err != nil {
		t.Fatalf("ChecksAtHead: %v", err)
	}
	have := rollupLabels(checks)
	for _, r := range required {
		if !have[strings.ToLower(strings.TrimSpace(r))] {
			t.Errorf("the project requires %q but the rollup at the head carries no entry of that "+
				"name (%v) — a required verdict the backend never publishes is could-not-check "+
				"forever, which is exactly the flip refusal #1125 reported", r, labelList(have))
		}
	}
	// The pipeline entry is not merely present: it carries the pipeline's real verdict.
	var pipeState string
	for _, sc := range checks.Statuses {
		if strings.EqualFold(sc.Context, GitLabPipelineContext) {
			pipeState = sc.State
		}
	}
	if pipeState != "success" {
		t.Errorf("the head pipeline succeeded but its rollup entry reads state %q, want \"success\"", pipeState)
	}
	// The short-read reconcile the gate runs before judging must still be exact: an asserted
	// total lower than the entries served would be read as a rollup nobody read in full.
	if checks.StatusTotalCount != len(checks.Statuses) {
		t.Errorf("status total %d vs %d entries served — the mapped pipeline entry must be counted "+
			"in the asserted total, or the caller's short-read reconcile misreads it",
			checks.StatusTotalCount, len(checks.Statuses))
	}

	// --- the board's half -----------------------------------------------------------------
	oc, err := f.ListOpenChanges(glRepo)
	if err != nil {
		t.Fatalf("ListOpenChanges: %v", err)
	}
	if len(oc.Changes) != 1 {
		t.Fatalf("open changes = %d, want 1", len(oc.Changes))
	}
	rollup := oc.Changes[0].Rollup
	if len(rollup) != 1 {
		t.Fatalf("board rollup = %d entries, want 1", len(rollup))
	}
	if rollup[0].Typename == GitLabRollupUnmapped {
		t.Fatalf("the board rollup for a head with a GREEN pipeline is still the could-not-check "+
			"sentinel %q — the board reads CI-UNKNOWN on an MR whose pipeline succeeded (#1125)",
			GitLabRollupUnmapped)
	}
	if got := strings.ToUpper(rollup[0].State); got != "SUCCESS" {
		t.Errorf("board rollup entry state = %q, want SUCCESS", rollup[0].State)
	}
	if !strings.EqualFold(rollup[0].Context, GitLabPipelineContext) {
		t.Errorf("board rollup entry context = %q, want %q — the board and the gate must name the "+
			"same pipeline the same way", rollup[0].Context, GitLabPipelineContext)
	}
}

// TestGitLabPipelineByShaFallbackReachesGate is the #1411 regression, one step past #1125:
// the SAME green MR pipeline must reach the flip gate even when the commit document carries NO
// `last_pipeline` at all — the ordinary shape GitLab leaves for a `merge_request_event` head,
// where the pipeline is reachable ONLY through the by-SHA read (`pipelines?sha=`) ListOpenChanges
// already uses. #1125 / PR #1134 published the pipeline only from `commit.last_pipeline`; when
// that field is empty the required context `pipeline` was never appended, so `deskflip` refused
// checks-green on a real success. missingRequiredChecks must come back EMPTY here.
func TestGitLabPipelineByShaFallbackReachesGate(t *testing.T) {
	s := newGLServer(t)
	glPipelineGated(s)
	// The commit document has NO last_pipeline — the field GitLab leaves empty for a
	// merge_request_event head. The pipeline is reachable ONLY via pipelines?sha=.
	s.commit = map[string]any{"id": "abc123", "status": "success"}
	s.pipelines = []map[string]any{
		{"id": 77, "sha": "abc123", "status": "success", "source": "merge_request_event",
			"created_at": "2026-09-15T10:00:00Z"},
	}
	s.jobs = []map[string]any{
		{"id": 9101, "name": "statusgen-lint", "status": "success",
			"finished_at": "2026-09-15T10:04:00Z"},
	}
	f := s.forge()

	required, err := f.RequiredStatusChecks(glRepo, "main")
	if err != nil {
		t.Fatalf("RequiredStatusChecks: %v", err)
	}
	if len(required) == 0 {
		t.Fatalf("a pipeline-gated project reported NO required checks")
	}
	checks, err := f.ChecksAtHead(glRepo, "abc123")
	if err != nil {
		t.Fatalf("ChecksAtHead: %v", err)
	}
	have := rollupLabels(checks)
	var missing []string
	for _, r := range required {
		if !have[strings.ToLower(strings.TrimSpace(r))] {
			missing = append(missing, r)
		}
	}
	if len(missing) != 0 {
		t.Errorf("commit.last_pipeline is empty but a GREEN pipeline exists at the head SHA via "+
			"pipelines?sha=; the flip gate still reports required checks %v as missing (rollup carries "+
			"%v) — #1411: ChecksAtHead did not fall back to the by-SHA read", missing, labelList(have))
	}
	// The pipeline entry is not merely present: it carries the pipeline's real verdict, mapped
	// from the by-SHA read rather than invented.
	var pipeState string
	for _, sc := range checks.Statuses {
		if strings.EqualFold(sc.Context, GitLabPipelineContext) {
			pipeState = sc.State
		}
	}
	if pipeState != "success" {
		t.Errorf("the head pipeline succeeded but its rollup entry reads state %q, want \"success\"", pipeState)
	}
	// The short-read reconcile stays exact: the fallback-mapped entry must be counted in the
	// asserted total, or the caller reads the appended entry as an over-serve.
	if checks.StatusTotalCount != len(checks.Statuses) {
		t.Errorf("status total %d vs %d entries served — the fallback pipeline entry must be counted "+
			"in the asserted total", checks.StatusTotalCount, len(checks.Statuses))
	}
}

// TestGitLabAbsentPipelineStaysCouldNotCheck pins the fail-closed direction on BOTH instruments:
// a head with no pipeline is could-not-check, never a pass. Nothing in the fix may turn the
// absence of a verdict into one.
func TestGitLabAbsentPipelineStaysCouldNotCheck(t *testing.T) {
	s := newGLServer(t)
	glPipelineGated(s)
	// No last_pipeline on the commit and no pipeline at the head for the board read.
	s.commit = map[string]any{"id": "abc123", "status": "success"}
	s.mrList = []map[string]any{glMR(map[string]any{"iid": 7, "created_at": "2026-09-01T09:00:00Z"})}
	f := s.forge()

	required, err := f.RequiredStatusChecks(glRepo, "main")
	if err != nil {
		t.Fatalf("RequiredStatusChecks: %v", err)
	}
	checks, err := f.ChecksAtHead(glRepo, "abc123")
	if err != nil {
		t.Fatalf("ChecksAtHead: %v", err)
	}
	have := rollupLabels(checks)
	missing := 0
	for _, r := range required {
		if !have[strings.ToLower(strings.TrimSpace(r))] {
			missing++
		}
	}
	if missing == 0 {
		t.Errorf("a head with NO pipeline reported every required check as present (%v) — an absent "+
			"pipeline must stay could-not-check, never a pass", labelList(have))
	}

	oc, err := f.ListOpenChanges(glRepo)
	if err != nil {
		t.Fatalf("ListOpenChanges: %v", err)
	}
	if len(oc.Changes) != 1 {
		t.Fatalf("open changes = %d, want 1", len(oc.Changes))
	}
	if got := oc.Changes[0].Rollup; len(got) != 1 || got[0].Typename != GitLabRollupUnmapped {
		t.Errorf("board rollup for a head with no pipeline = %+v, want the single %q could-not-check "+
			"entry — an EMPTY rollup reads as vacuously green on a CI-less repo", got, GitLabRollupUnmapped)
	}
}

// TestGitLabAllowedFailureJobDoesNotRedden pins the per-job half of the checks-green rule: a
// failed job declared `allow_failure: true` does not block, so it must not arrive as a failing
// conclusion while the pipeline it belongs to reports success. A job entry that contradicts the
// pipeline entry inside one rollup is the same disagreement, moved one level down.
func TestGitLabAllowedFailureJobDoesNotRedden(t *testing.T) {
	s := newGLServer(t)
	s.commit = map[string]any{"id": "abc123", "status": "success",
		"last_pipeline": map[string]any{"id": 78, "sha": "abc123", "status": "success"}}
	s.jobs = []map[string]any{
		{"id": 9201, "name": "statusgen-lint", "status": "success"},
		{"id": 9202, "name": "flaky-probe", "status": "failed", "allow_failure": true},
		{"id": 9203, "name": "go-test", "status": "failed"},
	}
	checks, err := s.forge().ChecksAtHead(glRepo, "abc123")
	if err != nil {
		t.Fatalf("ChecksAtHead: %v", err)
	}
	got := map[string]string{}
	for _, r := range checks.CheckRuns {
		got[r.Name] = r.Conclusion
	}
	if got["flaky-probe"] != "neutral" {
		t.Errorf("an allow_failure job that failed maps to conclusion %q, want \"neutral\" — a job "+
			"the forge does not gate on must not redden a head whose pipeline is green", got["flaky-probe"])
	}
	// The guard on the guard: a REQUIRED job that failed still reddens. A fold that swallowed
	// both would satisfy the line above while making the gate unable to see any failure at all.
	if got["go-test"] != "failure" {
		t.Errorf("a blocking job that failed maps to conclusion %q, want \"failure\"", got["go-test"])
	}
}

// labelList renders a label set for a failure message, sorted for a stable read.
func labelList(have map[string]bool) []string {
	out := make([]string, 0, len(have))
	for k := range have {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
