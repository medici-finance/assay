package main

// deskclose_closeverify_test.go — the close is not trusted on the PATCH's return value.
//
// THE DEFECT THIS FILE PINS. `deskclose superseded` on an open PULL REQUEST posted its
// confirmation comment and the `Superseded-By:` back-reference, then reported SUCCESS — while
// the PR stayed open. The state-change call had "succeeded" at the HTTP layer without doing what
// was asked (a state_reason PATCH on a PR's issue number returns 422, swallowed after the comment
// posted), and nothing read the state back, so a comment-only outcome read as a completed close.
//
// The fix has two observable halves, both proved here on BOTH fake-forge shapes (the single
// number sequence GitHub uses, and the separate issue/change sequences GitLab uses):
//
//  1. the close is chosen by the item's KIND — a pull request is closed as a change, never with
//     an issue's state_reason PATCH (kind_test.go pins the routing; here we pin it for the exact
//     reported PR-subject superseded case);
//  2. after the close call the item is READ BACK, and a close that did not take — the call
//     refused, or the read shows the item still open — is reported as `partial: comment posted,
//     close refused: …` with exit 6, NEVER as success.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// changeRef spells a change reference the typed way (`!N`), the form the separate-sequence shape
// needs so a bare number is never resolved to the wrong kind.
func changeRef(n int) string { return fmt.Sprintf("!%d", n) }

// prSubjectKey is the fixture key of prWorld's PR subject (#90) on the single-sequence shape.
const prSubjectKey = testRepo + "#90"

// confirmPRWorld is prWorld with a standing worker proposal on the PR subject, so the reviewer's
// run is the CONFIRM half — the exact field invocation: `superseded -R <repo> <pr> --by <merged>`.
func confirmPRWorld(t *testing.T) (*stubRemote, string) {
	t.Helper()
	s, rul := prWorld(t)
	s.viewer = reviewerLogin
	s.plantProposal(testRepo, 90, workerLogin, testRepo+"#40")
	return s, rul
}

// TestSupersededPRCloseRoutesAsChange (fix part 1). Closing a PR-kind item via the superseded
// lane must use the pull-request close call, not the issues state_reason PATCH. The stub's
// CloseIssueTyped errors if a change close carries a state reason, so a clean close is itself the
// proof the reason was dropped; the typed-close trail proves the kind that was addressed.
func TestSupersededPRCloseRoutesAsChange(t *testing.T) {
	s, rul := confirmPRWorld(t)
	code, out := execCLI(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
	if code != deskkit.ExitOK {
		t.Fatalf("reviewer confirm of a PR should close (exit 0), got %d\n%s", code, out)
	}
	assertContains(t, "typed close", s.typedCloses, prSubjectKey+":"+string(deskkit.TargetChange))
	// And the close addressed the pull-request sequence in the recorded argv.
	closed := false
	for _, w := range s.writes() {
		if w[1] == "close" {
			if w[0] != "pr" {
				t.Fatalf("the PR was closed through the %q sequence, not the change sequence: %v", w[0], w)
			}
			if contains(w, "--reason") {
				t.Fatalf("a change close carried a state reason, which no forge records: %v", w)
			}
			closed = true
		}
	}
	if !closed {
		t.Fatalf("no close was issued: %v", s.writes())
	}
}

// TestSupersededCloseReadBackSilentSuccessIsPartial (fix part 2, GitHub single-sequence shape).
// THE reported bug, restated as a check: the close call returns without error but the item stays
// open. Before the fix this printed "closed" and exited 0; now the read-back catches it and the
// run is a partial, exit 6, and the PR is left visibly open rather than reported closed.
func TestSupersededCloseReadBackSilentSuccessIsPartial(t *testing.T) {
	s, rul := confirmPRWorld(t)
	s.closeNoReflect[prSubjectKey] = true // the close "succeeds" at the HTTP layer, state unchanged

	err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
	if err == nil {
		t.Fatal("a close that left the PR open was reported as success — this is the silent-success bug")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("want could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	for _, want := range []string{"partial", "comment posted", "close refused", "still reads state"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the partial report does not mention %q:\n%s", want, err.Error())
		}
	}
	// The confirmation comment DID post — the whole point is that a comment-only outcome is not a
	// close — and the subject PR is still open in the fixture.
	if !hasWrite(s, "comment") {
		t.Fatalf("the confirmation comment did not post: %v", s.writes())
	}
	if !strings.Contains(s.items[prSubjectKey], `"state":"open"`) {
		t.Fatalf("the read-back should have observed the PR still open: %s", s.items[prSubjectKey])
	}
}

// TestSupersededCloseReadBackSilentSuccessIsPartialGitLab is the same property on the SEPARATE
// issue/change sequence shape (GitLab's), so the read-back is proved on both fake forges. The
// change `!collideNum` closes without error but its state is not reflected.
func TestSupersededCloseReadBackSilentSuccessIsPartialGitLab(t *testing.T) {
	s, rul := collisionWorld(t)
	s.viewer = reviewerLogin
	s.plantProposalOn(changeKey(testRepo, collideNum), workerLogin, changeRef(supersedes))
	s.closeNoReflect[changeKey(testRepo, collideNum)] = true

	err := execErr(modeSuperseded, "-R", testRepo, changeRef(collideNum), "--by", changeRef(supersedes), "--rulings", rul)
	if err == nil {
		t.Fatal("a change close that did not take was reported as success on the separate-sequence shape")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("want could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "partial") || !strings.Contains(err.Error(), "close refused") {
		t.Fatalf("the partial report is missing on the separate-sequence shape:\n%s", err.Error())
	}
	// The ISSUE sharing the number is untouched, and the change is still open — nothing was
	// closed, and nothing on the wrong object either.
	if !strings.Contains(s.items[changeKey(testRepo, collideNum)], `"state":"open"`) {
		t.Fatalf("the change should still read open: %s", s.items[changeKey(testRepo, collideNum)])
	}
}

// TestSupersededCloseRefusedIsPartialNotSuccess (fix part 3). When the close CALL fails (a 422
// from the forge) after the comment posted, the tool reports `partial: comment posted, close
// refused: <forge body>` with exit 6 — and the forge's own words travel in the message. It never
// reports success even though the comment posted fine.
func TestSupersededCloseRefusedIsPartialNotSuccess(t *testing.T) {
	s, rul := confirmPRWorld(t)
	s.failClose[prSubjectKey] = true // CloseIssueTyped returns a 422

	err := execErr(modeSuperseded, "-R", testRepo, "90", "--by", testRepo+"#40", "--rulings", rul)
	if err == nil {
		t.Fatal("a refused close was reported as success")
	}
	if !deskkit.IsUnverifiable(err) {
		t.Fatalf("want could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	for _, want := range []string{"partial: comment posted, close refused:", "HTTP 422"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the partial report does not carry %q (the forge body must survive):\n%s", want, err.Error())
		}
	}
	if !hasWrite(s, "comment") {
		t.Fatalf("the confirmation comment should still have posted: %v", s.writes())
	}
	// The close never took: no typed close was recorded and the PR is still open.
	for _, c := range s.typedCloses {
		if strings.HasPrefix(c, prSubjectKey+":") {
			t.Fatalf("a refused close was recorded as a completed one: %v", s.typedCloses)
		}
	}
}

// TestSupersededArgumentOrderAndDashSpelling pins the argument-order fix. The documented order
// and the item-first order both parse, AND the single-dash long-flag spelling (`-by`, which Go's
// flag package accepts identically to `--by`) parses too. Before the fix the single-dash spelling
// tripped `flag needs an argument: -by`: the splitter did not recognise it as value-consuming,
// dropped the value that WAS present, and mis-read it as a second positional — a failure that
// looked "environmental" because it depended only on the dash spelling the caller happened to use.
func TestSupersededArgumentOrderAndDashSpelling(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"documented order", []string{"-R", testRepo, "55", "--by", mergedPRRef}},
		{"item first", []string{"55", "-R", testRepo, "--by", mergedPRRef}},
		{"item first single-dash by", []string{"55", "-R", testRepo, "-by", mergedPRRef}},
		{"flags first single-dash by", []string{"-R", testRepo, "-by", mergedPRRef, "55"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, rul := baseWorld(t) // subject issue #55, reviewer, a worker's standing proposal
			args := append([]string{modeSuperseded}, tc.args...)
			args = append(args, "--rulings", rul)
			code, out := execCLI(args...)
			if code != deskkit.ExitOK {
				t.Fatalf("the invocation must parse and confirm regardless of argument order and dash "+
					"spelling; got exit %d\n%s", code, out)
			}
		})
	}
}
