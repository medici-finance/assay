package deskkit

// forge_gitlab_verdictlane_test.go — the backend half of "a GitLab request-changes verdict
// is a standing rejection the read path can see" (issue #1124).
//
// forge_gitlab_reviewwrite_test.go already pins the CORRECTNESS round-trip (#798). These
// tests pin the two things that round-trip does not reach, each of which left a live
// rejection unreadable:
//
//   - the SECURITY lane, whose `fail` is submitted as REQUEST_CHANGES and whose body can
//     carry no correctness verdict line at all; and
//   - the ORDER of the returned slice, on which every downstream reduction of "the standing
//     verdict" depends.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestVerdictNoteState_1124 pins the reducer itself, including the two mappings that are
// deliberately NOT the obvious ones.
func TestVerdictNoteState_1124(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"correctness_approve", "## Review\n\nVerdict: approve\n", "APPROVED"},
		{"correctness_request_changes", "## Review\n\nVerdict: request-changes\n", "CHANGES_REQUESTED"},
		{
			// The #1124 case. `deskpost security-review --verdict fail` submits
			// REQUEST_CHANGES and its body may carry ONLY this line.
			"security_fail_is_a_rejection",
			"## Findings\n\n1. the thing\n\nSecurity-Review: fail\n",
			"CHANGES_REQUESTED",
		},
		{
			// A security PASS is submitted as COMMENT, never APPROVE, so that an all-clear
			// in one lane cannot erase a standing rejection in the other. The read must not
			// grant what the write withheld.
			"security_pass_is_not_an_approval",
			"## Findings\n\nnone\n\nSecurity-Review: pass\n",
			"",
		},
		{"no_verdict_line", "just a comment on the change\n", ""},
		{
			// Block-before-grant: a body carrying both reduces to the rejection.
			"rejection_outranks_grant",
			"Verdict: approve\n\nSecurity-Review: fail\n",
			"CHANGES_REQUESTED",
		},
		{
			// Emphasis is unwrapped on the read path (#232), in the retraction direction
			// above all.
			"emphasised_security_fail",
			"**Security-Review: fail**\n",
			"CHANGES_REQUESTED",
		},
		{
			// The documented quoting escape hatch: citing the format is not posting a
			// verdict in it.
			"quoted_line_is_a_citation",
			"the reviewer should have written\n\n> Security-Review: fail\n",
			"",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := VerdictNoteState(c.body); got != c.want {
				t.Errorf("VerdictNoteState(%q) = %q, want %q", c.body, got, c.want)
			}
		})
	}
}

// TestForgeGitlabSecurityFailIsReadableAtHead_1124 is the round-trip: the security verdict
// verb's REQUEST_CHANGES write, read back through the same backend, must show a
// CHANGES_REQUESTED at head. Read as an ordinary COMMENTED note it was invisible to every
// consumer, and the board reported the change as never reviewed.
func TestForgeGitlabSecurityFailIsReadableAtHead_1124(t *testing.T) {
	s := newGLStatefulServer(t)
	f := s.forge()

	const failBody = "## Findings\n\n1. the egress allowlist is widened\n\nSecurity-Review: fail\n"
	if err := f.PostReview(glRepo, 7, ReviewInput{HeadSHA: glHead, Event: "REQUEST_CHANGES", Body: failBody}); err != nil {
		t.Fatalf("PostReview REQUEST_CHANGES (security fail): %v", err)
	}
	reviews, err := f.ReviewsAtHead(glRepo, 7)
	if err != nil {
		t.Fatalf("ReviewsAtHead: %v", err)
	}
	if got := findReview(reviews, func(r Review) bool {
		return r.State == "CHANGES_REQUESTED" && r.CommitID == glHead
	}); got == nil {
		t.Fatalf("a security FAIL posted as REQUEST_CHANGES is not readable as CHANGES_REQUESTED at head %s "+
			"— the rejection is invisible to the board and the flip gate; reviews: %+v", glHead, reviews)
	}
}

// TestForgeGitlabReviewsAreAscending_1124 pins the interface's ordering contract at the
// backend that has to work for it. The fake serves notes NEWEST-FIRST, which is what
// GitLab's notes endpoint does, and plants an approval whose system note dates it BETWEEN
// two of them — so a fix that merely reversed the notes slice, leaving approvals bolted on
// at one end, would still fail here.
func TestForgeGitlabReviewsAreAscending_1124(t *testing.T) {
	const (
		head     = "abc123"
		tVersion = "2026-09-10T09:00:00Z"
		tOldest  = "2026-09-10T10:00:00Z"
		tApprove = "2026-09-10T11:00:00Z"
		tNewest  = "2026-09-10T12:00:00Z"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		path := r.URL.EscapedPath()
		author := map[string]any{"id": 42, "username": "reviewer-bot"}
		switch {
		case lProjApproval.MatchString(path):
			enc(map[string]any{"reset_approvals_on_push": true})
		case lMRVersions.MatchString(path):
			enc([]map[string]any{{"id": 3, "head_commit_sha": head, "created_at": tVersion}})
		case lMRApprovals.MatchString(path):
			enc(map[string]any{"approved_by": []map[string]any{{"user": author}}})
		case lMRNotes.MatchString(path):
			// Newest first, as the endpoint returns them.
			enc([]map[string]any{
				{"id": 3, "body": "Verdict: request-changes", "system": false, "created_at": tNewest, "author": author},
				{"id": 2, "body": "approved this merge request", "system": true, "created_at": tApprove, "author": author},
				{"id": 1, "body": "Verdict: approve", "system": false, "created_at": tOldest, "author": author},
			})
		case lMR.MatchString(path):
			enc(glMR(nil))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
	reviews, err := f.ReviewsAtHead(glRepo, 7)
	if err != nil {
		t.Fatalf("ReviewsAtHead: %v", err)
	}
	want := []string{tOldest, tApprove, tNewest}
	if len(reviews) != len(want) {
		t.Fatalf("got %d reviews, want %d: %+v", len(reviews), len(want), reviews)
	}
	for i, r := range reviews {
		if r.SubmittedAt != want[i] {
			t.Fatalf("review[%d].SubmittedAt = %q, want %q — the slice must be ascending by submitted "+
				"time (approvals interleaved with notes), because every consumer reduces it as "+
				"'the last decisive verdict governs'; got %+v", i, r.SubmittedAt, want[i], reviews)
		}
	}
	// The newest verdict is the rejection, so that is what a last-wins reduction reports.
	if last := reviews[len(reviews)-1]; last.State != "CHANGES_REQUESTED" {
		t.Fatalf("the LAST review is %s, want CHANGES_REQUESTED — the newest verdict must be the one a "+
			"last-wins reduction lands on", last.State)
	}
}

// TestSortReviewsAscendingPutsUndatableFirst_1124 pins the fail-closed tie-break in
// isolation. A verdict whose submitted time could not be established must never be able to
// supersede one that carries a time — on GitLab an approval carries no timestamp of its own
// unless the system-note timeline supplies one, so this is a live shape, not a hypothetical.
func TestSortReviewsAscendingPutsUndatableFirst_1124(t *testing.T) {
	rs := []Review{
		{ID: 1, State: "CHANGES_REQUESTED", SubmittedAt: "2026-09-10T12:00:00Z"},
		{ID: 2, State: "APPROVED", SubmittedAt: ""},
		{ID: 3, State: "APPROVED", SubmittedAt: "2026-09-10T10:00:00Z"},
	}
	sortReviewsAscending(rs)
	wantIDs := []int64{2, 3, 1}
	for i, want := range wantIDs {
		if rs[i].ID != want {
			t.Fatalf("order = [%d %d %d], want [2 3 1] — an undatable verdict sorts FIRST so it can be "+
				"superseded but can never supersede", rs[0].ID, rs[1].ID, rs[2].ID)
		}
	}
}
