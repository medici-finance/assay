package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// reanchor.go — the stale-grant filter (#3093). See deskkit/reanchor.go for the defect: the
// forge can RE-ANCHOR an old review's commit_id to a later head, so "commit_id == head" alone
// does not show a review verified the head. A grant at head submitted BEFORE the head
// commit's committer date verified nothing there.

// headCommittedAtReader returns a memoized reader for the head commit's committer date. It
// costs nothing until a grant at head actually needs it, and is read at most once per flip
// (the head is fixed for the whole flip; a moved head refuses at head-stable). An unreadable
// commit, or a commit reporting no parseable date, is an ERROR — could-not-check — never a
// zero date silently compared against.
func headCommittedAtReader(fg deskkit.Forge, fr deskkit.ForgeRepo, head string) func() (time.Time, error) {
	var (
		done bool
		at   time.Time
		err  error
	)
	return func() (time.Time, error) {
		if done {
			return at, err
		}
		done = true
		c, rerr := fg.GetCommit(fr, head)
		switch {
		case rerr != nil:
			err = rerr
		case c == nil:
			err = errors.New("the forge returned no commit")
		default:
			at = deskkit.ParseCommitDate(c.CommittedDate)
			if at.IsZero() {
				err = fmt.Errorf("the head commit reports no readable committer date (%q)", c.CommittedDate)
			}
		}
		return at, err
	}
}

// isGrantAtHead reports whether r is a GRANTING review by the reviewer pinned at head: an
// APPROVED (the correctness grant), or a `Security-Review: pass` (the security grant). A body
// carrying a `Security-Review: fail` is never a grant, whatever its state — the security
// reduction reads it as a standing retraction, and dropping it would loosen that lane.
func isGrantAtHead(r reviewInfo, reviewerLogin, head string) bool {
	if head == "" || r.CommitID != head || !deskkit.SameActor(r.User.Login, reviewerLogin) {
		return false
	}
	if deskkit.HasSecurityReviewFail(r.Body) {
		return false
	}
	return r.State == "APPROVED" || deskkit.HasSecurityReviewPass(r.Body)
}

// withoutStaleGrants returns reviews with every STALE grant at head removed: a grant (see
// isGrantAtHead) whose submitted_at predates the head commit's committer date. It ONLY ever
// removes grants — a CHANGES_REQUESTED, a failing security marker, a COMMENTED finding, and
// any review not pinned at head pass through untouched — so every reduction downstream can
// only refuse more, never less.
//
// The head date is read only when at least one grant at head exists (the common no-approval
// path pays nothing). When it cannot be read, the grants cannot be shown to postdate the head,
// and the flip is refused unverifiable: fail closed.
//
// The returned slice is what checkReviewerApproved and checkSecurityVerdict decide on. It is
// NOT what checkDeskDecided reads: that condition refuses on FINDINGS, and a finding carried on
// a stale APPROVED body is still a finding.
func withoutStaleGrants(reviews []reviewInfo, reviewerLogin, head, cond string,
	headAt func() (time.Time, error)) ([]reviewInfo, error) {
	grants := 0
	for _, r := range reviews {
		if isGrantAtHead(r, reviewerLogin, head) {
			grants++
		}
	}
	if grants == 0 {
		return reviews, nil
	}
	at, err := headAt()
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"condition %s: %s has a grant (APPROVED or Security-Review pass) pinned at head %s, but the head "+
				"commit's committer date could not be read (%s) — whether that review was submitted AFTER the "+
				"head commit existed cannot be established, and a forge can re-anchor an older review onto a "+
				"newer head (#3093). could-not-check is never an approval.",
			cond, reviewerLogin, short(head), firstLine(err.Error())), err)
	}
	out := make([]reviewInfo, 0, len(reviews))
	for _, r := range reviews {
		if isGrantAtHead(r, reviewerLogin, head) && deskkit.ReviewPredatesHead(r.SubmittedAt, at) {
			fmt.Fprintf(os.Stderr, "deskflip: NOTE: ignoring %s's %s review reported at head %s — it was "+
				"submitted %s, BEFORE the head commit's committer date %s, so the forge re-anchored it from an "+
				"older head and it verifies nothing at this one (#3093).\n",
				reviewerLogin, r.State, short(head), r.SubmittedAt, at.UTC().Format(time.RFC3339))
			continue
		}
		out = append(out, r)
	}
	return out, nil
}
