package main

// workerassignment_test.go — a dispatched worker's Assignment names its OWN loop identity
// and, for ISSUE-ONLY work, the sanctioned verb to comment on the issue it was dispatched
// from plus the exact `deskpr create` trailer.
//
// THE DEFECT (assay#700). A dispatched worker had no sanctioned verb to comment on the
// ISSUE it was dispatched from: `deskreply` is PR-only, and `deskfile attach` refused with
// `$DESK_LOOP is unset` because the emitted kit never told the worker its own loop is
// `worker-desk`. Issue-only workers also met a second refusal at the PR ceremony — the
// emitted key never said the `deskpr create` body must carry `Issue: #<N>` rather than a
// `Brief:` line. Both pushed workers toward hand-rolled `gh` writes the desk verbs exist to
// replace.
//
// WHAT THESE TESTS PIN. The worker's artifact is the PROMPT. These assert the worker
// Assignment now names `export DESK_LOOP=worker-desk` (for every worker) and, for an
// issue-only item key, the `Issue: #<N>` trailer and the `deskfile attach ... --to <N>`
// comment verb — and that a brief-based item gets the loop line but NOT the issue-shaped
// lines.

import (
	"strings"
	"testing"
)

// TestWorkerAssignmentNamesOwnLoopIdentity — every worker Assignment, brief or issue,
// tells the worker to export its own DESK_LOOP so the desk write verbs stop refusing.
//
// FAIL-FIRST: against the unfixed code the Assignment never mentioned DESK_LOOP, so this
// fired. It passes only once the loop line is emitted.
func TestWorkerAssignmentNamesOwnLoopIdentity(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "worker", "example-stream--07"))
	for _, want := range []string{
		"export DESK_LOOP=worker-desk",
		"do NOT inherit the dispatching desk's",
	} {
		if !strings.Contains(asg, want) {
			t.Errorf("the worker Assignment is missing the loop-identity instruction %q:\n%s", want, asg)
		}
	}
}

// TestIssueOnlyWorkerAssignmentNamesTrailerAndCommentVerb — an issue-only item key emits
// the exact `Issue: #<N>` trailer and the `deskfile attach` comment verb keyed on that
// issue number.
//
// FAIL-FIRST: against the unfixed code neither line was emitted for an issue key, so both
// assertions fired.
func TestIssueOnlyWorkerAssignmentNamesTrailerAndCommentVerb(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "worker", "issue-42"))
	flat := strings.Join(strings.Fields(asg), " ")
	for _, want := range []string{
		"Issue: #42", // the exact deskpr create trailer
		"deskfile attach -R medici-finance/assay --to 42", // the sanctioned issue-comment verb
		"`deskreply` is for your OWN open PR only",        // and what it is NOT
		"export DESK_LOOP=worker-desk",                    // the loop is still named
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the issue-only worker Assignment is missing %q:\n%s", want, asg)
		}
	}
}

// TestBriefWorkerAssignmentOmitsIssueShapedLines — a brief-based item carries the loop line
// but NOT the issue-only trailer or comment verb, so a brief worker is never told to add an
// `Issue:` trailer its body must not carry.
func TestBriefWorkerAssignmentOmitsIssueShapedLines(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "worker", "example-stream--07"))
	for _, bad := range []string{
		"Issue: #",
		"deskfile attach",
		"This is ISSUE-ONLY work",
	} {
		if strings.Contains(asg, bad) {
			t.Errorf("the brief-based worker Assignment leaked issue-only instruction %q:\n%s", bad, asg)
		}
	}
}

// TestIssueNumFromItem pins the key-form reduction directly: every issue-only spelling
// deskdispatch is handed reduces to the number, and a brief-based key does not.
func TestIssueNumFromItem(t *testing.T) {
	for _, tc := range []struct {
		item string
		want int
		ok   bool
	}{
		{"issue-42", 42, true},
		{"medici-finance/assay:issue-700", 700, true},
		{"assay--issue-321", 321, true},
		{"example-stream--07", 0, false},
		{"medici-finance/assay:example-stream/07", 0, false},
		{"issue-", 0, false},
		{"issue-0", 0, false},
		{"issue-abc", 0, false},
	} {
		got, ok := issueNumFromItem(tc.item)
		if got != tc.want || ok != tc.ok {
			t.Errorf("issueNumFromItem(%q) = (%d, %v), want (%d, %v)", tc.item, got, ok, tc.want, tc.ok)
		}
	}
}
