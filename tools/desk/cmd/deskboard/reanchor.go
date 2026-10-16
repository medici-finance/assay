package main

import (
	"fmt"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reanchor.go — the board's stale-grant filter (#3093). See deskkit/reanchor.go for the
// defect: the forge can RE-ANCHOR an old review's commit_id to a later head, so
// "commit_id == head" alone does not show a review verified the head. A grant at head whose
// submitted_at predates the head commit's committer date verified nothing there.

// isGrantAtHead reports whether r is a GRANTING reviewer-App review pinned at head: an
// APPROVED, or a `Security-Review: pass` marker (the board's security reduction admits the
// marker from any state). A body whose security reading is FAIL is never a grant — dropping
// it would launder a retraction.
func isGrantAtHead(r review, head string) bool {
	if !isReviewerBot(r.User.Login) || !sameHead(r.CommitID, head) {
		return false
	}
	sec := classifySecurityBody(r.Body)
	if sec == secFail {
		return false
	}
	return r.State == "APPROVED" || sec == secPass
}

// withoutStaleGrants returns reviews with every STALE grant at the PR's head removed — a
// grant (isGrantAtHead) submitted BEFORE the head commit's committer date — and how many it
// removed. It only ever removes grants: a CHANGES_REQUESTED, a security fail, and every
// review not pinned at head pass through, so reduceReviews and applyBodyEditReverification
// downstream can only read the row as LESS approved, never more.
//
// The head date comes from the list read (p.HeadCommittedDate, bound to headRefOid) and,
// when that is absent or unreadable (GitLab, or a backend that does not carry it), from a
// per-commit read — taken ONLY when a grant at head exists, so the common row pays nothing.
// When neither yields a date, the error is returned WITH every grant at head removed: the
// caller degrades the row, and a grant that could not be shown to postdate the head is not
// approval at head (fail closed).
func withoutStaleGrants(repo string, p prBase, reviews []review) ([]review, int, error) {
	head := p.HeadRefOid
	grants := 0
	for _, r := range reviews {
		if isGrantAtHead(r, head) {
			grants++
		}
	}
	if grants == 0 {
		return reviews, 0, nil
	}
	at := deskkit.ParseCommitDate(p.HeadCommittedDate)
	var readErr error
	if at.IsZero() {
		var when time.Time
		when, _, readErr = fetchHeadCommit(repo, head)
		if readErr == nil {
			at = when
		}
	}
	out := make([]review, 0, len(reviews))
	dropped := 0
	for _, r := range reviews {
		if isGrantAtHead(r, head) && (readErr != nil || deskkit.ReviewPredatesHead(r.SubmittedAt, at)) {
			dropped++
			continue
		}
		out = append(out, r)
	}
	if readErr != nil {
		return out, dropped, fmt.Errorf("could not read head commit %s's committer date (%v) — %d App "+
			"grant(s) pinned at head cannot be shown to postdate it, so none counts as at head (#3093)",
			short(head), readErr, dropped)
	}
	return out, dropped, nil
}
