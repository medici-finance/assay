package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitlabreviewer_test.go — the board half of the forge-aware reviewer identity.
//
// The board's matcher is a DIFFERENT comparison from the flip gate's: isReviewerBot tests
// `login == want` for exact equality, where deskflip runs deskkit.SameActor. Both were
// wrong in the same way and for the same reason — the expected login carried GitHub's
// `[bot]` suffix on every forge — so a GitLab approval that really existed reduced to
// "no verdict", and classify's `!in.ever` arm reported the PR as NEEDS-REVIEW, which is
// what feeds the UNREVIEWED alarm. Fixing only deskflip would have left the queue still
// insisting the PR had never been looked at, so this test pins the board arm separately
// rather than trusting that one fix covers both.

const boardRosterGitLabReviewer = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=gitlab:gl-reviewer:41987965
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
`

// mkReview builds a board review with an author, since review.User is an anonymous struct.
func mkReview(login, state, commit, body string) review {
	var r review
	r.User.Login = login
	r.State = state
	r.CommitID = commit
	r.Body = body
	return r
}

// TestBoardAcceptsGitLabReviewerUsername is the matcher itself: the bare username is the
// reviewer, and the GitHub decoration is NOT a rendering a GitLab account answers to.
func TestBoardAcceptsGitLabReviewerUsername(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)
	if !deskkit.EffectiveConfig().Configured() {
		t.Fatal("precondition: the roster must load as CONFIGURED")
	}

	if !isReviewerBot("gl-reviewer") {
		t.Error("isReviewerBot(\"gl-reviewer\") = false — the rostered GitLab service account " +
			"was not recognised as the reviewer, so its approvals are invisible to the queue")
	}
	if isReviewerBot("gl-reviewer[bot]") {
		t.Error("isReviewerBot(\"gl-reviewer[bot]\") = true — the GitHub [bot] decoration is not " +
			"minted for a GitLab account, so a login wearing it is not this identity")
	}
	if isReviewerBot("") {
		t.Error("isReviewerBot(\"\") = true — an author-less review must never match a role")
	}
}

// TestBoardReducesGitLabApprovalAtHead runs the real reduction the queue runs, then the
// real classification, and asserts the PR is not reported as never-reviewed.
func TestBoardReducesGitLabApprovalAtHead(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)

	const head = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"
	reviews := []review{
		mkReview("gl-reviewer", "APPROVED", head, "Verdict: approve\nSecurity-Review: pass"),
	}

	st := reduceReviews(reviews, head)
	if !st.ever {
		t.Fatal("reduceReviews: ever = false over an at-head APPROVED by the rostered GitLab " +
			"reviewer — this is the exact reduction that made classify report NEEDS-REVIEW and " +
			"kept the UNREVIEWED alarm firing on a PR that had been approved")
	}
	if !st.atHead || !st.approved {
		t.Fatalf("reduceReviews: atHead = %v, approved = %v, want true/true", st.atHead, st.approved)
	}
	if !st.securityPass {
		t.Error("reduceReviews: securityPass = false — the security marker on the GitLab " +
			"reviewer's at-head review was dropped with the rest of its identity")
	}

	// The classification arm the alarm is keyed on. ciRequired=false makes CI vacuously
	// green so this asserts the REVIEW axis and nothing else.
	var p prBase
	p.HeadRefOid = head
	action, note := classify(buildClassifyInput(p, st, false, ""))
	if action == actNeedsReview {
		t.Fatalf("classify = %s (%s) — the board still reports a GitLab-approved PR as needing "+
			"review, which is what drives the UNREVIEWED banner", action, note)
	}
}

// TestBoardStillRefusesLookalikeGitLabLogin is the no-widening guard on the board arm: only
// the rostered username is the reviewer. A neighbouring account whose name merely contains
// or extends it is a different actor.
func TestBoardStillRefusesLookalikeGitLabLogin(t *testing.T) {
	installBoardRoster(t, boardRosterGitLabReviewer)

	for _, login := range []string{"gl-reviewer2", "gl-review", "not-gl-reviewer", "app/gl-reviewer"} {
		if isReviewerBot(login) {
			t.Errorf("isReviewerBot(%q) = true — only the rostered username is the reviewer; "+
				"accepting a look-alike would let another account's approval flip a merge request", login)
		}
	}
}
