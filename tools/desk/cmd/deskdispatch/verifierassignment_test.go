package main

// verifierassignment_test.go — the verifier kit's ASSIGNMENT is verifier-shaped, never the
// implementer scaffold (#1029).
//
// THE DEFECT. `deskdispatch --kit verifier` emitted a prompt whose top Assignment section was
// the WORKER scaffold — "Open the draft PR", `deskpr create`, `export DESK_LOOP=worker-desk`,
// and the release-on-push line — directly contradicting the verifier-prompt kit body that
// follows (a verifier is READ-ONLY against MERGED main: it posts a written verdict, opens no
// PR, pushes no branch, and never adopts the worker App's identity). `assemblePrompt` computed
// `review := reviewKit(o.kit)` and branched ONLY `writeReviewAssignment` vs. `writeWorkerAssignment`
// — `reviewKit` recognizes exactly the literal "review", so "verifier" fell into the worker
// `else` branch and inherited its scaffold wholesale. A verifier agent following the prompt
// literally would open a spurious draft PR under the WORKER App's identity for a plain verify
// pass, and would release its dispatch claim on the wrong signal (branch push, not
// verdict-landed).
//
// WHAT THESE TESTS PIN. The verifier is an agent, so this repo's artifact is the PROMPT. These
// assert the Assignment section (everything before the standing-clauses boundary) is
// verifier-shaped under --kit verifier — no worker scaffold, no DESK_LOOP=worker-desk, no
// "PR opens THERE" framing — and that the worker/review paths are byte-for-byte unchanged.

import (
	"strings"
	"testing"
)

// verifierOnlyScaffold is the WORKER-identity line that must never reach a verifier: adopting
// it would mint the wrong App on any write the verifier makes. workerScaffold (defined in
// reviewassignment_test.go, same package) already covers the PR-opening lines this shares with
// the review test; this adds the one line specific to the worker's identity export that a
// review dispatch never emits in the first place, so it was never asserted there.
const workerLoopIdentityLine = "export DESK_LOOP=worker-desk"

// Under --kit verifier the Assignment must NOT carry the implementer scaffold (nor the
// worker's own loop-identity export), and MUST carry the verifier-shaped instructions:
// read-only against merged main, no PR, release-on-verdict-landed.
//
// FAIL-FIRST: against the unfixed code the Assignment WAS the worker scaffold, so every
// workerScaffold assertion below fires, workerLoopIdentityLine fires, and the verifier-shaped
// assertions are absent. It passes only once the verifier kit's Assignment is made
// verifier-shaped.
func TestVerifierKitAssignmentIsVerifierShapedNotWorkerScaffold(t *testing.T) {
	prompt := dispatchPrompt(t, "verifier", "verify-item-9")
	asg := assignmentSection(t, prompt)

	for _, bad := range workerScaffold {
		if strings.Contains(asg, bad) {
			t.Errorf("the verifier Assignment carries the implementer scaffold %q — a verifier handed it can "+
				"open a spurious PR:\n%s", bad, asg)
		}
	}
	if strings.Contains(asg, workerLoopIdentityLine) {
		t.Errorf("the verifier Assignment adopts the worker's own loop identity %q — a verifier's write "+
			"would mint the WRONG App identity:\n%s", workerLoopIdentityLine, asg)
	}
	if strings.Contains(asg, "the PR opens THERE") {
		t.Errorf("the verifier Assignment carries the PR-opens-here framing, which invites a spurious "+
			"draft PR:\n%s", asg)
	}

	flat := strings.Join(strings.Fields(asg), " ")
	for _, want := range []string{
		"READ-ONLY",                    // the verifier produces no change
		"Do not create a pull request", // it opens no PR
		"MERGED main",                  // it runs the Verify table against merged main
		"VERIFY: PASS",                 // its output is a written verdict
		"never flips a PR ready",       // it never advances the item past verify itself
		"once your verdict has LANDED", // claim release keys on verdict landing, not a push
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the verifier Assignment is missing the verifier-shaped instruction %q:\n%s", want, asg)
		}
	}
}

// The Branch bullet is the implementer's output surface; a verifier's worktree is temporary
// and cut off origin/main at the merged head, so naming a branch here would invite a verifier
// to push one. Mirrors the review kit's omission of the same bullet.
func TestVerifierKitAssignmentOmitsBranchBullet(t *testing.T) {
	asg := assignmentSection(t, dispatchPrompt(t, "verifier", "verify-item-9"))
	if strings.Contains(asg, "**Branch:**") {
		t.Errorf("the verifier Assignment names a Branch — a verifier does not produce one:\n%s", asg)
	}
}

// The worker and review paths are UNCHANGED: only the verifier kit's Assignment was wrong.
func TestWorkerAndReviewKitsUnaffectedByVerifierSplit(t *testing.T) {
	workerAsg := assignmentSection(t, dispatchPrompt(t, "worker", "example-stream--07"))
	if !strings.Contains(workerAsg, "export DESK_LOOP=worker-desk") ||
		!strings.Contains(workerAsg, "## Open the draft PR in that repo") {
		t.Errorf("the worker Assignment lost its implementer scaffold:\n%s", workerAsg)
	}

	reviewAsg := assignmentSection(t, dispatchPrompt(t, "review", "review-item-9"))
	if !strings.Contains(reviewAsg, "READ-ONLY") || strings.Contains(reviewAsg, "MERGED main") {
		t.Errorf("the review Assignment was perturbed by the verifier split:\n%s", reviewAsg)
	}
}
