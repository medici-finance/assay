package main

// autoflipwiring_test.go pins that the model-autoflip job's own doc comment in
// the shipped CI workflow accurately describes the two-tier could-not-check
// exit policy reportAutoFlipModel implements ("Fix B" in autoflip.go). Prose
// can drift from code silently forever — this is the one test that reads the
// two together and fails the moment they disagree, the same wiring-test shape
// as corroboratewiring_test.go for `--corroborate`.
//
// It exists because the comment used to read "a candidate the tool could not
// check fails this job loudly (exit 1)" — true only for the MISCONFIGURATION
// kind of could-not-check, false for the STRUCTURALLY
// unresolvable kind (exit 0, a non-fatal NOTICE) that
// TestReportAutoFlipStructuralNonFatal pins directly against the binary. The
// CODE was already right — TestReportAutoFlipStructuralNonFatal,
// TestReportAutoFlipMisconfigFatal and TestReportAutoFlipMisconfigDominates
// (see below) all predate this file and pin it — the COMMENT overclaimed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModelAutoflipWorkflowCommentMatchesExitPolicy reads the model-autoflip
// job's header comment out of assay-statusgen.yml and checks it against the
// two-tier policy the binary actually implements: only a fixable
// MISCONFIGURATION could-not-check is fatal (exit 1); a structurally
// unresolvable one is a non-fatal NOTICE (exit 0).
func TestModelAutoflipWorkflowCommentMatchesExitPolicy(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "assay-statusgen.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("workflow not present in this tree: %v", err)
	}
	body := string(raw)

	idx := strings.Index(body, "\n  model-autoflip:")
	if idx == -1 {
		t.Skip("no model-autoflip job in this workflow — nothing to pin")
	}
	// The job's own doc comment is the contiguous run of `#`-prefixed lines
	// immediately above the job key.
	lines := strings.Split(body[:idx], "\n")
	start := len(lines)
	for start > 0 && strings.HasPrefix(strings.TrimSpace(lines[start-1]), "#") {
		start--
	}
	comment := strings.Join(lines[start:], "\n")
	lower := strings.ToLower(comment)

	// The comment must name the misconfiguration/structural split, or a future
	// edit can silently re-widen the claim to "every could-not-check is fatal"
	// with nothing catching it.
	if !strings.Contains(lower, "misconfig") {
		t.Errorf("the model-autoflip job comment no longer names the misconfiguration/structural "+
			"could-not-check split (autoflip.go's Fix B) — it must not claim every could-not-check "+
			"is fatal; comment:\n%s", comment)
	}
	if !strings.Contains(lower, "notice") || !strings.Contains(lower, "exits 0") {
		t.Errorf("the model-autoflip job comment must say the structurally-unresolvable kind is a "+
			"non-fatal NOTICE that exits 0 — see TestReportAutoFlipStructuralNonFatal; comment:\n%s", comment)
	}

	// The overclaim this test exists to catch: a blanket claim that ANY
	// could-not-check candidate fails the job, with no exception named.
	overclaims := []string{
		"a candidate the tool could not check fails this job loudly (exit 1)",
		"every could-not-check fails this job",
		"any could-not-check fails this job",
	}
	for _, bad := range overclaims {
		if strings.Contains(lower, bad) {
			t.Errorf("the model-autoflip job comment overclaims the exit policy (contains %q) — only a "+
				"fixable MISCONFIGURATION could-not-check is fatal; a structurally-unresolvable one is a "+
				"non-fatal NOTICE (reportAutoFlipModel / TestReportAutoFlipStructuralNonFatal)", bad)
		}
	}
}
