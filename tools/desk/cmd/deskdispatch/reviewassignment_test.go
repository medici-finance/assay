package main

// reviewassignment_test.go — the review kit's ASSIGNMENT is review-shaped, never the
// implementer scaffold.
//
// THE DEFECT. `deskdispatch --kit review` emitted a prompt whose top Assignment section was
// the WORKER scaffold — "Open the draft PR", `deskpr create`, "Stop at `implemented`", a
// `feat/…-pr-…` branch off main, and the release-on-push line — directly contradicting the
// review-kit body that follows (a reviewer is READ-ONLY: it posts a verdict, opens no PR,
// pushes no branch). A reviewer handed both could open a spurious draft PR for a PR that is
// already open, or review the wrong tree (the fresh branch off main, not the PR's head).
//
// WHAT THESE TESTS PIN. The reviewer is an agent, so this repo's artifact is the PROMPT.
// These assert the Assignment section (everything before the standing clauses) is
// review-shaped under --kit review — no worker scaffold, and a fetch of the PR's head — and
// that the worker/implement path is byte-for-byte unchanged.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// assignmentSection returns the Assignment half of an emitted prompt — everything before the
// standing-clauses boundary. The review-kit body itself legitimately mentions lifecycle
// tokens like `implemented`, so the worker-scaffold assertions must read the ASSIGNMENT, not
// the whole prompt.
func assignmentSection(t *testing.T, prompt string) string {
	t.Helper()
	i := strings.Index(prompt, "# Standing clauses")
	if i < 0 {
		t.Fatalf("prompt has no standing-clauses boundary:\n%s", prompt)
	}
	return prompt[:i]
}

// dispatchPrompt emits the prompt for a kit via --dry-run (no steps run) and returns it.
func dispatchPrompt(t *testing.T, kit, item string, extra ...string) string {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	args := []string{item, "--root", root, "--repo", allowedRepo, "--kit", kit,
		"--dry-run", "--prompt-file", promptFile}
	args = append(args, extra...)
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("%s dispatch rc = %d, want 0", kit, rc)
	}
	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	return string(body)
}

// workerScaffold is the set of strings that belong ONLY to the implementer's assignment. A
// reviewer handed any of them can act as an implementer — the exact defect.
var workerScaffold = []string{
	"Open the draft PR",
	"deskpr create",
	"Stop at `implemented`",
	"feat/", // the auto-cut implementer branch, e.g. feat/…-pr-…
	"once your branch is pushed",
	"branch-as-claim takes over",
}

// Under --kit review the Assignment must NOT carry the implementer scaffold, and MUST carry
// the review-shaped instructions: read-only, a fetch of the PR's head, a verdict via deskpost.
//
// FAIL-FIRST: against the unfixed code the Assignment WAS the worker scaffold, so every
// workerScaffold assertion below fires and the review-shaped ones are absent. It passes only
// once the review kit's Assignment is made review-shaped.
func TestReviewKitAssignmentIsReviewShapedNotWorkerScaffold(t *testing.T) {
	prompt := dispatchPrompt(t, "review", "medici-finance/assay/pr/547", "--pr", "547")
	asg := assignmentSection(t, prompt)

	for _, bad := range workerScaffold {
		if strings.Contains(asg, bad) {
			t.Errorf("the review Assignment carries the implementer scaffold %q — a reviewer handed it can "+
				"open a spurious PR or review the wrong tree:\n%s", bad, asg)
		}
	}

	flat := strings.Join(strings.Fields(asg), " ")
	for _, want := range []string{
		"READ-ONLY",                           // the reviewer produces no change
		"Do not create a pull request",        // it opens no PR
		"do not advance the item past review", // it is not an implementation
		"posts its verdict via `deskpost`",    // its output is a verdict
		"pull/547/head",                       // it reviews the PR's HEAD, by number
		"PR #547",                             // the PR under review is named
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the review Assignment is missing the review-shaped instruction %q:\n%s", want, asg)
		}
	}
}

// Without --pr the review Assignment is still review-shaped and points at pull/<N>/head as a
// placeholder rather than inventing a number.
func TestReviewKitAssignmentWithoutPRNumber(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "review", "review-item-9"))
	flat := strings.Join(strings.Fields(asg), " ")
	for _, want := range []string{"READ-ONLY", "pull/<N>/head", "the PR named in your dispatch"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the numberless review Assignment is missing %q:\n%s", want, asg)
		}
	}
	for _, bad := range workerScaffold {
		if strings.Contains(asg, bad) {
			t.Errorf("the numberless review Assignment carries the implementer scaffold %q:\n%s", bad, asg)
		}
	}
}

// The worker/implement path is UNCHANGED: its Assignment still carries the full implementer
// scaffold. Only the review kit's Assignment was wrong, and the fix must not touch this one.
func TestWorkerKitAssignmentKeepsTheImplementerScaffold(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "worker", "example-stream--07"))
	for _, want := range []string{
		"## Open the draft PR in that repo",
		"deskpr create",
		"Stop at `implemented`",
		"once your branch is pushed",
		"branch-as-claim takes over",
	} {
		if !strings.Contains(asg, want) {
			t.Errorf("the worker Assignment lost the implementer scaffold %q — the fix must leave the "+
				"worker path byte-for-byte unchanged:\n%s", want, asg)
		}
	}
	// And a worker never receives the review read-only framing.
	if strings.Contains(asg, "READ-ONLY") || strings.Contains(asg, "pull/") {
		t.Errorf("the worker Assignment leaked review-shaped instructions:\n%s", asg)
	}
}
