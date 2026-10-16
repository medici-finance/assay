package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reanchor.go — `ready`'s stale-grant filter (#3093). See deskkit/reanchor.go for the defect:
// the forge can RE-ANCHOR an old review's commit_id to a later head, so "commit_id == head"
// alone does not show a review verified the head. A grant at head whose submitted_at predates
// the head commit's committer date verified nothing there.

// isGrantAtHead reports whether r is a GRANTING reviewer-App review pinned at head: a
// correctness APPROVED, or a security pass (securityGrantOf). A body whose security reading
// is FAIL is never a grant, whatever its state — dropping it would launder a retraction.
func isGrantAtHead(r reviewInfo, head string) bool {
	if head == "" || r.CommitID != head || !isReviewerBot(r.User.Login) {
		return false
	}
	if classifySecurityBody(r.Body) == secFail {
		return false
	}
	return r.State == "APPROVED" || securityGrantOf(r) == secPass
}

// withoutStaleGrants returns reviews with every STALE grant at head removed — a grant (see
// isGrantAtHead) submitted BEFORE the head commit's committer date. It only ever removes
// grants: a CHANGES_REQUESTED, a security fail, and every review not pinned at head pass
// through, so each reduction downstream (latestAppVerdict, securityVerdictStanding, the
// external-prerequisite exemption's citing APPROVEs) can only refuse more, never less.
//
// The head commit's date is read only when a grant at head exists. A read error, or a commit
// that reports no parseable date, is returned as an error: whether the grant postdates the
// head is could-not-check, and the caller refuses (fail closed). deskDecidedRefusal does NOT
// read this filtered list — it refuses on findings, and a finding on a stale review stands.
func withoutStaleGrants(client postBackend, reviews []reviewInfo, head string, stderr io.Writer) ([]reviewInfo, error) {
	grants := 0
	for _, r := range reviews {
		if isGrantAtHead(r, head) {
			grants++
		}
	}
	if grants == 0 {
		return reviews, nil
	}
	raw, err := client.headCommittedAt(head)
	if err != nil {
		return nil, fmt.Errorf("an App grant is pinned at head %s, but the head commit's committer date could "+
			"not be read (%w) — whether that review postdates the head commit cannot be established, and a "+
			"forge can re-anchor an older review onto a newer head (#3093)", short(head), err)
	}
	at := deskkit.ParseCommitDate(raw)
	if at.IsZero() {
		return nil, deskkit.Unverifiable(fmt.Sprintf("an App grant is pinned at head %s, but the head commit "+
			"reports no readable committer date (%q) — whether that review postdates the head commit cannot be "+
			"established (#3093)", short(head), raw), errors.New("head commit date unreadable"))
	}
	out := make([]reviewInfo, 0, len(reviews))
	for _, r := range reviews {
		if isGrantAtHead(r, head) && deskkit.ReviewPredatesHead(r.SubmittedAt, at) {
			fmt.Fprintf(stderr, "deskpost: NOTE: ignoring the App's %s review reported at head %s — it was "+
				"submitted %s, BEFORE the head commit's committer date %s, so the forge re-anchored it from an "+
				"older head and it verifies nothing at this one (#3093)\n",
				r.State, short(head), r.SubmittedAt, at.UTC().Format(time.RFC3339))
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
