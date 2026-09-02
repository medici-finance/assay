package main

// pathclaim_test.go — the review kit's path-resolution clause, and the negative-path
// property behind it.
//
// THE DEFECT. A dispatched reviewer reported four files as non-existent because it checked
// path existence in the DISPATCHING desk's own checkout rather than in the repository the
// PR belongs to. All four were present in the PR's repository; three went out as findings.
// A file absent from the desk's tree and present in the PR's tree must never be reported
// as missing, and the only way an agent can get that right is to be told which tree the
// question is about and to be required to name it.
//
// WHAT IS MECHANICALLY CHECKABLE HERE. The reviewer is an agent, so this repo's artifact is
// the PROMPT it is dispatched with; nothing in this tree DECIDES whether a path exists (a
// search over the package confirms no helper does), so there is no decision function to
// unit-test. What these tests pin instead is everything the prompt must carry for the
// wrong answer to be unreachable: the clause itself, the requirement to name the tree, the
// could-not-check demotion, and — the negative path — that the tree the reviewer is pointed
// at is the PR's repository and never the dispatcher's checkout, even when the two differ.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reviewPrompt dispatches a review for `repo` from a checkout of some OTHER repo and
// returns the emitted prompt.
func reviewPrompt(t *testing.T, repo string) string {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/agent-home")

	promptFile := filepath.Join(t.TempDir(), "review.md")
	args := []string{"item-1", "--root", root, "--kit", "review", "--prompt-file", promptFile}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	if rc := run(args); rc != deskkit.ExitOK {
		t.Fatalf("review dispatch rc = %d, want 0", rc)
	}
	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	return string(body)
}

// The clause is present, and it carries each of the four things that make it actionable
// rather than a slogan.
func TestReviewKitRequiresPathClaimsResolvedInTheTargetRepo(t *testing.T) {
	kit, err := kitText("review")
	if err != nil {
		t.Fatalf("review kit: %v", err)
	}
	for _, want := range []string{
		"at the PR head",             // WHICH ref
		"contents API",               // HOW, without a checkout
		"Name the tree in the find",  // the finding must say where it looked
		"could-not-check, not a mis", // the demotion, not an absence
	} {
		if !strings.Contains(kit, want) {
			t.Errorf("the review kit does not carry %q — a reviewer cannot follow a rule it is not "+
				"given", want)
		}
	}
}

// Only the REVIEW class carries it: a clause emitted to every class is a clause nobody
// reads. (If it ever becomes right for the worker or verifier kit, that is a deliberate
// change and this test is the place it gets recorded.)
func TestThePathClaimClauseIsScopedToTheReviewKit(t *testing.T) {
	const marker = "Resolve every path claim"
	for _, name := range kitNames() {
		text, err := kitText(name)
		if err != nil {
			t.Fatalf("kit %q: %v", name, err)
		}
		if got := strings.Contains(text, marker); got != (name == "review") {
			t.Errorf("kit %q carries the path-claim clause = %v, want %v", name, got, name == "review")
		}
	}
}

// THE NEGATIVE PATH. A review dispatched for repo B, from a checkout of repo A, must point
// the reviewer at B. A file absent in A and present in B is then not a phantom, because
// the tree the reviewer was told to resolve against is B's.
//
// The dispatcher's checkout path appears in the prompt for exactly one purpose — it is the
// `git -C` source a worktree is cut FROM — and this asserts it is never offered as the tree
// a path claim is answered in.
func TestReviewPromptNamesTheTargetRepoNotTheCheckout(t *testing.T) {
	const target = "medici-finance/assay"
	prompt := reviewPrompt(t, target)

	if !strings.Contains(prompt, "**Target repo:** `"+target+"`") {
		t.Fatalf("the prompt does not name the PR's repo as the target:\n%s", prompt)
	}
	// Line wrapping in the kit is not the subject here, so the sentence-level assertions
	// below read a whitespace-collapsed copy.
	flat := strings.Join(strings.Fields(prompt), " ")
	// The clause tells the reviewer to resolve against the repo the PR belongs to, and the
	// assignment block above it is where that value is stated.
	if !strings.Contains(flat, "The assignment block above names the target repo") {
		t.Fatalf("the clause does not tie the tree back to the assignment's target repo:\n%s", prompt)
	}
	// And it says in as many words that the checkout the reviewer is running in is a
	// different tree — which is the sentence that stops a cwd-resolved absence.
	for _, want := range []string{
		"it is not the same value as the checkout you are running in",
		"agrees with the PR's repository only by coincidence",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the prompt does not separate the PR's tree from the reviewer's own: missing %q", want)
		}
	}
}

// A short diff is not a small tree. Reading the diff as the repository is how the invented
// absence gets started, so the kit says so and this pins it.
func TestReviewKitWarnsThatADiffIsNotTheTree(t *testing.T) {
	kit, err := kitText("review")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kit, "A short diff is not evidence that a tree is empty") {
		t.Error("the review kit does not warn that a PR's diff is not its repository")
	}
}
