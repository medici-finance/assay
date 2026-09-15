package main

// mergehold_test.go — the forge-gitlab merge-hold brief's Verify row 6: deskflip's
// reviewer-approved condition on a forge with a merge-hold (GitLab today) reads the marker
// thread's own state and refuses on hold-absent, hold-unresolved, resolved-by-a-non-reviewer
// and resolved-at-a-stale-head, re-arming the last of those as part of the refusal.
//
// checkMergeHoldApproved is exercised DIRECTLY, against a minimal fake deskkit.Forge that
// embeds the interface as nil (the same pattern envForge / glReviewFake use elsewhere in this
// tree): any call this test does not expect — including, load-bearingly, the project
// approval-configuration route (`GET /projects/:id/approvals`) that 403s on gitlab.com Free
// (#1091) — panics rather than silently succeeding. That is a STRONGER guarantee than
// asserting zero requests to one named route: the fake has no HTTP transport for that route
// to reach in the first place, so there is no path through this function that could touch it.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mergeHoldFake is a fake deskkit.Forge scripted for exactly the merge-hold write
// (SetMergeHold) checkMergeHoldApproved's stale-head branch performs. Every other method
// panics if reached — in particular there is no ReviewsAtHead, no approvals read, and no
// GetPullRequest: this function takes an already-read *deskkit.MergeHold and never re-derives
// it, so a correct implementation calls nothing else.
type mergeHoldFake struct {
	deskkit.Forge // nil — unimplemented ops panic if reached

	setCalls []deskkit.MergeHoldUpdate
	setErr   error
}

func (f *mergeHoldFake) SetMergeHold(_ deskkit.ForgeRepo, _ int, in deskkit.MergeHoldUpdate) error {
	f.setCalls = append(f.setCalls, in)
	return f.setErr
}

const (
	mhReviewerLogin = "assay-reviewer-app"
	mhHead          = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaface"
	mhStaleHead     = "000000000000000000000000000000000000dead"
	mhPR            = 7
)

var mhRepo = deskkit.ForgeRepo{Owner: "gl-group", Name: "gl-repo"}

// TestFlipGitLabMergeHold is the brief's Verify row 6 fixture set.
func TestFlipGitLabMergeHold(t *testing.T) {
	t.Run("hold_absent_refuses", func(t *testing.T) {
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{State: deskkit.MergeHoldAbsent}
		err := checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead)
		if err == nil {
			t.Fatal("expected a refusal on an absent merge-hold, got nil")
		}
		if !strings.Contains(err.Error(), condReviewerApproved) {
			t.Errorf("refusal does not name condition %s: %v", condReviewerApproved, err)
		}
		if len(f.setCalls) != 0 {
			t.Errorf("an absent hold must not be written to, got %d SetMergeHold call(s)", len(f.setCalls))
		}
	})

	t.Run("hold_unresolved_refuses", func(t *testing.T) {
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{State: deskkit.MergeHoldUnresolved, ID: "disc-1"}
		err := checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead)
		if err == nil {
			t.Fatal("expected a refusal on an unresolved merge-hold, got nil")
		}
		if !strings.Contains(err.Error(), condReviewerApproved) {
			t.Errorf("refusal does not name condition %s: %v", condReviewerApproved, err)
		}
		if len(f.setCalls) != 0 {
			t.Errorf("an unresolved hold must not be written to, got %d SetMergeHold call(s)", len(f.setCalls))
		}
	})

	t.Run("resolved_by_non_reviewer_refuses", func(t *testing.T) {
		// GitLab lets any Developer resolve any thread — the exact hazard this fixture
		// guards: a hand-resolve must never satisfy the reviewer-approved gate, whatever
		// head it names.
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{
			State: deskkit.MergeHoldResolved, ID: "disc-1",
			ResolvedBy: "some-developer", Head: mhHead,
		}
		err := checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead)
		if err == nil {
			t.Fatal("expected a refusal on a hold resolved by a non-reviewer, got nil")
		}
		if !strings.Contains(err.Error(), condReviewerApproved) {
			t.Errorf("refusal does not name condition %s: %v", condReviewerApproved, err)
		}
		if !strings.Contains(err.Error(), "some-developer") {
			t.Errorf("refusal does not name who actually resolved it: %v", err)
		}
		if len(f.setCalls) != 0 {
			t.Errorf("a hand-resolved-at-head hold must not be re-armed (the head is current — only WHO "+
				"resolved it is wrong), got %d SetMergeHold call(s)", len(f.setCalls))
		}
	})

	t.Run("resolved_at_stale_head_refuses_and_rearms", func(t *testing.T) {
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{
			State: deskkit.MergeHoldResolved, ID: "disc-1",
			ResolvedBy: mhReviewerLogin, Head: mhStaleHead,
		}
		err := checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead)
		if err == nil {
			t.Fatal("expected a refusal on a hold resolved at a stale head, got nil")
		}
		if !strings.Contains(err.Error(), condReviewerApproved) {
			t.Errorf("refusal does not name condition %s: %v", condReviewerApproved, err)
		}
		if len(f.setCalls) != 1 {
			t.Fatalf("expected exactly 1 re-arm SetMergeHold call, got %d: %+v", len(f.setCalls), f.setCalls)
		}
		if h := f.setCalls[0]; h.Resolved || !strings.Contains(h.Reason, "new head") {
			t.Errorf("SetMergeHold call = %+v, want a re-arm naming the new head", h)
		}
	})

	t.Run("resolved_at_current_head_by_reviewer_passes", func(t *testing.T) {
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{
			State: deskkit.MergeHoldResolved, ID: "disc-1",
			ResolvedBy: mhReviewerLogin, Head: mhHead,
		}
		if err := checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead); err != nil {
			t.Fatalf("expected the happy path to pass, got: %v", err)
		}
		if len(f.setCalls) != 0 {
			t.Errorf("a hold already resolved at the current head must not be written to again, got %d call(s)",
				len(f.setCalls))
		}
	})

	// The pre-mortem's other named failure mode: "the flip's mergeable condition refuses
	// draft_status on every GitLab draft, so the gate is correct and nothing can ever flip" —
	// caught by proving mergeable does NOT block on the raw statuses the merge-hold is about
	// to release, while everything else it has always refused keeps refusing.
	t.Run("mergeable_condition_allows_draft_status_and_discussions_not_resolved", func(t *testing.T) {
		o := flipOpts{pr: mhPR, quiet: true}
		for _, raw := range []string{"draft_status", "discussions_not_resolved", "DRAFT_STATUS"} {
			if err := checkMergeableCondition(o, mhPR, "UNKNOWN", raw); err != nil {
				t.Errorf("checkMergeableCondition(UNKNOWN, %q) = %v, want nil (non-blocking)", raw, err)
			}
		}
	})

	t.Run("mergeable_condition_still_refuses_every_other_unknown_reason", func(t *testing.T) {
		o := flipOpts{pr: mhPR, quiet: true}
		for _, raw := range []string{"checking", "unchecked", "", "not_approved", "blocked_status"} {
			if err := checkMergeableCondition(o, mhPR, "UNKNOWN", raw); err == nil {
				t.Errorf("checkMergeableCondition(UNKNOWN, %q) = nil, want a refusal", raw)
			}
		}
	})

	t.Run("mergeable_condition_still_refuses_conflicting", func(t *testing.T) {
		o := flipOpts{pr: mhPR, quiet: true}
		// Even a raw status this leniency would otherwise excuse must not rescue a change the
		// tri-state verdict has ALREADY decided is CONFLICTING — that decision outranks the raw
		// string entirely.
		if err := checkMergeableCondition(o, mhPR, "CONFLICTING", "draft_status"); err == nil {
			t.Error("checkMergeableCondition(CONFLICTING, draft_status) = nil, want a refusal")
		}
	})

	t.Run("mergeable_condition_unaffected_on_github", func(t *testing.T) {
		// GitHub never populates GitLabMergeStatus, so the raw argument is always empty — the
		// leniency path is structurally unreachable there, and MERGEABLE/CONFLICTING behave
		// exactly as they always have (Verify row 8).
		o := flipOpts{pr: mhPR, quiet: true}
		if err := checkMergeableCondition(o, mhPR, "MERGEABLE", ""); err != nil {
			t.Errorf("checkMergeableCondition(MERGEABLE, \"\") = %v, want nil", err)
		}
		if err := checkMergeableCondition(o, mhPR, "UNKNOWN", ""); err == nil {
			t.Error("checkMergeableCondition(UNKNOWN, \"\") = nil, want a refusal (GitHub's genuine not-yet-computed case)")
		}
	})

	// The property row 6 names explicitly: no request in this path ever reaches the project
	// approval-configuration route. mergeHoldFake embeds a NIL deskkit.Forge and implements
	// only SetMergeHold — every case above ran to completion (or a controlled refusal)
	// without panicking, which is only possible because checkMergeHoldApproved never calls
	// ReviewsAtHead or any other method that route's degrade logic lives behind.
	t.Run("touches_no_other_forge_op", func(t *testing.T) {
		f := &mergeHoldFake{}
		hold := &deskkit.MergeHold{State: deskkit.MergeHoldUnresolved, ID: "disc-1"}
		_ = checkMergeHoldApproved(hold, f, mhRepo, mhReviewerLogin, mhPR, mhHead)
		// Reaching here without a panic (a call to an unstubbed, nil-embedded method) IS the
		// assertion — see the file header.
	})
}
