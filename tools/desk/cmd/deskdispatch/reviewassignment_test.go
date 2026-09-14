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
// The target repo's origin is stubbed to github.com so a review dispatch resolves its forge
// (the fixture roster configures no ASSAY_REPO_FORGES entry, so resolution falls to the
// origin host) — a worker dispatch never reads it, so this is inert for the worker path.
func dispatchPrompt(t *testing.T, kit, item string, extra ...string) string {
	t.Helper()
	return dispatchPromptOrigin(t, "git@github.com:medici-finance/assay.git", kit, item, extra...)
}

// dispatchPromptOrigin is dispatchPrompt with the origin-remote URL the stubbed target
// checkout reports, so a test can steer the forge the review dispatch resolves (github.com
// → github, gitlab.com → gitlab). An empty origin exercises the unresolvable-forge path.
func dispatchPromptOrigin(t *testing.T, origin, kit, item string, extra ...string) string {
	t.Helper()
	body, rc := dispatchPromptOriginRC(t, origin, kit, item, extra...)
	if rc != deskkit.ExitOK {
		t.Fatalf("%s dispatch rc = %d, want 0", kit, rc)
	}
	return body
}

// dispatchPromptOriginRC runs the dispatch and returns the emitted prompt (empty when none
// was written) AND the exit code, so a test can assert a REFUSAL as well as a prompt.
func dispatchPromptOriginRC(t *testing.T, origin, kit, item string, extra ...string) (string, int) {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	if strings.TrimSpace(origin) != "" {
		s.replies = append(s.replies, reply{match: "remote get-url origin", stdout: origin})
	}

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	args := []string{item, "--root", root, "--repo", allowedRepo, "--kit", kit,
		"--dry-run", "--prompt-file", promptFile}
	args = append(args, extra...)
	rc := run(args)
	body, err := os.ReadFile(promptFile)
	if err != nil {
		// No prompt file is the expected outcome of a refusal; return the code so the caller
		// asserts on it rather than the read error masking the verdict under test.
		return "", rc
	}
	return string(body), rc
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

// On a GitLab-served repo the review head-fetch refspec is the GITLAB coordinate
// (merge-requests/<iid>/head), never the GitHub one (pull/<N>/head). This is #773: a
// reviewer that follows a GitHub-shaped prompt verbatim on a GitLab MR fetches a ref that
// does not exist and cannot check out the head it was dispatched to verdict.
//
// FAIL-FIRST: against the unfixed code writeReviewAssignment emitted `pull/%d/head`
// unconditionally, so this test's merge-requests assertion fails and its no-`pull/` assertion
// fails on the exact same line, for BOTH the numbered and numberless forms. It passes only
// once the refspec follows the resolved forge.
func TestReviewKitHeadFetchIsGitLabShapedOnAGitLabRepo(t *testing.T) {
	// Numbered: the resolved GitLab forge (origin gitlab.com, no roster forge entry) yields
	// the merge-requests refspec by iid.
	asg := assignmentSection(t, dispatchPromptOrigin(t,
		"git@gitlab.com:medici-finance/assay.git", "review", "medici-finance/assay/mr/1", "--pr", "1"))
	flat := strings.Join(strings.Fields(asg), " ")
	if !strings.Contains(flat, "merge-requests/1/head") {
		t.Errorf("the GitLab review Assignment must fetch the MR head at merge-requests/1/head:\n%s", asg)
	}
	if strings.Contains(flat, "pull/1/head") || strings.Contains(flat, "pull/<N>/head") {
		t.Errorf("the GitLab review Assignment leaked the GitHub-shaped pull/<N>/head fetch (#773):\n%s", asg)
	}

	// Numberless: still GitLab-shaped, with the <N> placeholder rather than an invented iid.
	asg = assignmentSection(t, dispatchPromptOrigin(t,
		"https://gitlab.com/medici-finance/assay.git", "review", "review-item-9"))
	flat = strings.Join(strings.Fields(asg), " ")
	if !strings.Contains(flat, "merge-requests/<N>/head") {
		t.Errorf("the numberless GitLab review Assignment must point at merge-requests/<N>/head:\n%s", asg)
	}
	if strings.Contains(flat, "pull/") {
		t.Errorf("the numberless GitLab review Assignment leaked a GitHub-shaped pull/ fetch (#773):\n%s", asg)
	}
}

// The complement: a GitHub-served repo still fetches pull/<N>/head. The fix must resolve the
// forge, not swap one hard-coded shape for another.
func TestReviewKitHeadFetchIsGitHubShapedOnAGitHubRepo(t *testing.T) {
	asg := assignmentSection(t, dispatchPromptOrigin(t,
		"git@github.com:medici-finance/assay.git", "review", "medici-finance/assay/pr/1", "--pr", "1"))
	flat := strings.Join(strings.Fields(asg), " ")
	if !strings.Contains(flat, "pull/1/head") {
		t.Errorf("the GitHub review Assignment must fetch the PR head at pull/1/head:\n%s", asg)
	}
	if strings.Contains(flat, "merge-requests/") {
		t.Errorf("the GitHub review Assignment leaked the GitLab-shaped merge-requests fetch:\n%s", asg)
	}
}

// A review dispatch whose target repo's forge cannot be resolved — no ASSAY_REPO_FORGES entry
// and an unreadable/unmappable origin — REFUSES rather than guess a forge and emit a
// coordinate the reviewer cannot check out. The refusal is pre-claim, so nothing durable is
// touched, and it carries deskkit's could-not-check exit (never ExitOK, never a prompt).
func TestReviewKitRefusesWhenForgeUnresolvable(t *testing.T) {
	body, rc := dispatchPromptOriginRC(t, "", "review", "medici-finance/assay/pr/1", "--pr", "1")
	if rc == deskkit.ExitOK {
		t.Fatalf("an unresolvable forge must refuse the review dispatch, got rc=%d and a prompt:\n%s", rc, body)
	}
	if body != "" {
		t.Errorf("a refused review dispatch must emit no prompt, got:\n%s", body)
	}
}

// The WORKER path is forge-agnostic: it emits no forge-shaped ref, so it neither reads the
// origin remote for a forge decision nor refuses when the forge is unresolvable. A worker
// dispatch against the same unresolvable-forge repo still succeeds and still carries its
// implementer scaffold — the review-only resolution must not leak into it.
func TestWorkerKitUnaffectedByForgeResolution(t *testing.T) {
	body, rc := dispatchPromptOriginRC(t, "", "worker", "example-stream--07")
	if rc != deskkit.ExitOK {
		t.Fatalf("the worker dispatch must not depend on forge resolution, got rc=%d", rc)
	}
	asg := assignmentSection(t, body)
	if !strings.Contains(asg, "deskpr create") {
		t.Errorf("the worker Assignment lost its implementer scaffold:\n%s", asg)
	}
}
