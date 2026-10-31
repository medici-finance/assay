package main

// securitybodycr_test.go — rule 2's THIRD exemption, the marked security fail cleared by a
// later same-head pass (#5092), driven end to end through the verb against the fake forge.
//
// The accepting case flips. Each refusal starts from it and breaks one clause of the ruling,
// and must refuse with NO mutation. The #5090 sequence without the marker still refuses, and
// neither lane's review ever clears the other lane's block.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	secFailAt    = "2026-01-01T00:00:00Z"
	secApproveAt = "2026-01-01T00:05:00Z"
	secPassAt    = "2026-01-01T00:20:00Z" // bodyEditedAt (00:10) falls between the fail and the pass
	otherHead    = "1111111122222222333333334444444455555555"
)

func secMarkedFailBody() string {
	return "## Security review\n\nThe body claims an allowlist the diff does not add.\n\nSecurity-Review: fail\n" +
		"Blocked-On-Body: " + bodyFindingID + " " + deskkit.PRBodyDigest(bodyBeforeEdit)
}

func secPassBody(live string) string {
	return "## Security review\n\nRe-read the live PR body via the API.\n\nSecurity-Review: pass\n" +
		"Body-Reread-Digest: " + deskkit.PRBodyDigest(live)
}

func secReview(t *testing.T, state, body, commit, at string) reviewInfo {
	t.Helper()
	r := reviewInfo{State: state, CommitID: commit, Body: body, SubmittedAt: at}
	r.User.Login = reviewerBot(t)
	return r
}

// secBodyStub is the accepting fixture: a marked security fail at head, a plain correctness
// APPROVE, the body edited, then a COMMENTED security pass re-reading the live body.
func secBodyStub(t *testing.T) *stub {
	t.Helper()
	s := newStub()
	s.pr.Body = bodyAfterEdit
	s.bodyEditedAt = bodyEditedAt
	s.reviews = []reviewInfo{
		secReview(t, "CHANGES_REQUESTED", secMarkedFailBody(), headSHA, secFailAt),
		citingApprove(t, "looks correct", secApproveAt),
		secReview(t, "COMMENTED", secPassBody(bodyAfterEdit), headSHA, secPassAt),
	}
	return s
}

// wantSecRefused runs the verb and wants a refusal with no mutation. When clause is set it
// also runs rule 2 on the same reviews and live body and wants the refusal to name that
// clause, so the refusal is pinned to the security route and not some other condition.
func wantSecRefused(t *testing.T, s *stub, why, clause string) {
	t.Helper()
	if clause != "" {
		live := liveBodyRead{Body: s.pr.Body, EditedAt: func() (string, error) { return s.bodyEditedAt, nil }}
		err := checkReviewerApproved(reviewerBot(t), s.reviews, headSHA, live, 7, nil)
		if err == nil || !strings.Contains(err.Error(), clause) {
			t.Fatalf("%s: rule 2 err = %v, want a refusal naming %q", why, err, clause)
		}
	}
	wantBodyEditRefused(t, s, why)
}

// regression: #5092
func TestSecFailMarkedClears(t *testing.T) {
	s := secBodyStub(t)
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("marked security fail cleared by a later same-head pass: rc = %d, want %d", rc, deskkit.ExitOK)
	}
	if !s.flipped() {
		t.Errorf("the clearance held but the ready mutation never ran: %v", s.requests)
	}
}

// The #5090 sequence, unmarked, in its real six-review shape at one head: a security fail,
// two correctness CRs, a second security fail, a documented body-edit APPROVE, and a
// COMMENTED pass carrying no digest line. An unmarked fail's basis may be code, so it still
// refuses.
//
// regression: #5092
func TestSecFail5090Replay(t *testing.T) {
	s := secBodyStub(t)
	unmarked := "## Security review\n\nSecurity-Review: fail"
	s.reviews = []reviewInfo{
		secReview(t, "CHANGES_REQUESTED", unmarked, headSHA, "2026-01-01T00:00:00Z"),
		secReview(t, "CHANGES_REQUESTED", bodyEditCRBody(), headSHA, "2026-01-01T00:01:00Z"),
		secReview(t, "CHANGES_REQUESTED", bodyEditCRBody(), headSHA, "2026-01-01T00:02:00Z"),
		secReview(t, "CHANGES_REQUESTED", unmarked, headSHA, "2026-01-01T00:03:00Z"),
		citingApprove(t, bodyEditApproveBody(), secApproveAt),
		secReview(t, "COMMENTED", "## Security review\n\nRe-read the PR.\n\nSecurity-Review: pass", headSHA, secPassAt),
	}
	wantSecRefused(t, s, "#5090 replay without the marker", "stands at the current head")
}

// regression: #5092
func TestSecFailNoLaterPass(t *testing.T) {
	s := secBodyStub(t)
	s.reviews = s.reviews[:2]
	wantSecRefused(t, s, "no later security pass", "no `Security-Review: pass`")
}

// regression: #5092
func TestSecFailDigestMismatch(t *testing.T) {
	s := secBodyStub(t)
	s.reviews[2] = secReview(t, "COMMENTED", secPassBody(bodyBeforeEdit), headSHA, secPassAt)
	wantSecRefused(t, s, "pass re-read a body that is not the live one", "not the digest of the live PR body")
}

// The body moved between the gate's first read and the pre-mutation re-read.
//
// regression: #5092
func TestSecFailBodyMovedTOCTOU(t *testing.T) {
	s := secBodyStub(t)
	s.body2 = bodyAfterEdit + "\n\nA new, unreviewed claim."
	wantSecRefused(t, s, "body edited between reads", "")
}

// regression: #5092
func TestSecFailNoBodyEdit(t *testing.T) {
	t.Run("forge reports no edit", func(t *testing.T) {
		s := secBodyStub(t)
		s.bodyEditedAt = ""
		wantSecRefused(t, s, "never edited per the forge", "reports no edit")
	})
	t.Run("last edit before the fail", func(t *testing.T) {
		s := secBodyStub(t)
		s.bodyEditedAt = "2025-12-31T23:00:00Z"
		wantSecRefused(t, s, "last edited before the fail", "not after the security fail")
	})
}

// regression: #5092
func TestSecFailEditTimeUnreadable(t *testing.T) {
	s := secBodyStub(t)
	s.trustErr = true
	s.install(t)
	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable body edit time: rc = %d, want %d (could-not-check)", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("unreadable body edit time: produced mutations %v", m)
	}
}

// regression: #5092
func TestSecFailLaterUnmarkedFail(t *testing.T) {
	s := secBodyStub(t)
	s.reviews = []reviewInfo{
		s.reviews[0], s.reviews[1],
		secReview(t, "CHANGES_REQUESTED", "## Security review\n\nSecurity-Review: fail", headSHA, "2026-01-01T00:06:00Z"),
		s.reviews[2],
	}
	wantSecRefused(t, s, "later unmarked fail before the pass", "later unmarked security fail blocks")
}

// regression: #5092
func TestSecFailAfterThePass(t *testing.T) {
	s := secBodyStub(t)
	s.reviews = append(s.reviews,
		secReview(t, "COMMENTED", "## Security review\n\nSecurity-Review: fail", headSHA, "2026-01-01T00:30:00Z"))
	wantSecRefused(t, s, "a fail after the pass", "later unmarked security fail blocks")
}

// regression: #5092
func TestSecFailPassNotAtHead(t *testing.T) {
	s := secBodyStub(t)
	s.reviews[2] = secReview(t, "COMMENTED", secPassBody(bodyAfterEdit), otherHead, secPassAt)
	wantSecRefused(t, s, "the pass is at another commit", "no `Security-Review: pass`")
}

// SAME REVIEWER. The only pass after the fail comes from another account: the clearance has
// no pass to read, so the refusal names the missing pass. It must come from the clearance's
// own actor filter, not the security-verdict reduction behind it, so the reason is pinned.
//
// regression: #5092
func TestSecFailOtherActorPass(t *testing.T) {
	s := secBodyStub(t)
	s.reviews[2].User.Login = "someone-else"
	wantSecRefused(t, s, "a pass from another account", "no `Security-Review: pass`")
}

// LANES. A fully documented correctness APPROVE never clears a marked security fail. Before
// #5092 rule 2 sent this CR to the correctness body-edit decision, which cleared it.
//
// regression: #5092
func TestSecFailNotClearedByApprove(t *testing.T) {
	cr := secReview(t, "CHANGES_REQUESTED", secMarkedFailBody(), headSHA, secFailAt)
	approve := citingApprove(t, bodyEditApproveBody(), secPassAt)
	err := checkReviewerApproved(reviewerBot(t), []reviewInfo{cr, approve}, headSHA,
		liveBodyRead{Body: bodyAfterEdit, EditedAt: func() (string, error) { return bodyEditedAt, nil }}, 7, nil)
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a correctness APPROVE cleared a security fail: err = %v", err)
	}
}

// LANES. A check-only security fail is not cleared by a correctness APPROVE citing a green run.
//
// regression: #5092
func TestSecFailCheckOnlyNotCleared(t *testing.T) {
	cr := secReview(t, "CHANGES_REQUESTED", "## Security review\n\nSecurity-Review: fail\nBlocked-On-Check: changelog",
		headSHA, secFailAt)
	approve := citingApprove(t, "Cleared-Check-Run: 41234567890", secPassAt)
	runs := func() ([]deskkit.CheckRun, error) {
		return []deskkit.CheckRun{{ID: "41234567890", Name: "changelog", Status: "completed", Conclusion: "success",
			CompletedAt: "2026-01-01T00:15:00Z"}}, nil
	}
	err := checkReviewerApproved(reviewerBot(t), []reviewInfo{cr, approve}, headSHA, liveBodyRead{Body: bodyAfterEdit}, 7, runs)
	if err == nil || !strings.Contains(err.Error(), "never clears a security fail") {
		t.Fatalf("a correctness APPROVE cleared a check-only security fail: err = %v", err)
	}
}

// LANES. A security pass documenting the body-edit lines never clears a correctness CR.
//
// regression: #5092
func TestSecPassNotClearingCR(t *testing.T) {
	s := bodyEditStub(t)
	s.reviews[1] = secReview(t, "COMMENTED", bodyEditApproveBody()+"\nSecurity-Review: pass", headSHA, bodyApproveAt)
	wantSecRefused(t, s, "security pass over a correctness body-edit CR", "no correctness APPROVE")
}

// HEAD MOVE. A clearance binds to its head: after a content-preserving head move the fail
// stands again in condition security-verdict, and the old-head pass clears nothing.
//
// regression: #5092
func TestSecClearanceHeadMove(t *testing.T) {
	login := reviewerBot(t)
	reviews := []reviewInfo{
		secReview(t, "CHANGES_REQUESTED", secMarkedFailBody(), otherHead, secFailAt),
		secReview(t, "COMMENTED", secPassBody(bodyAfterEdit), otherHead, secPassAt),
	}
	quiet := []fileInfo{{Path: "docs/streams/example/note.md"}}
	pr := prInfo{AuthorLogin: "ada", Body: bodyAfterEdit, ChangedFiles: len(quiet)}
	err := checkSecurityVerdict(flipOpts{pr: 7}, privateCIRepo, pr, quiet, reviews, login, headSHA)
	if err == nil || !strings.Contains(err.Error(), "standing `Security-Review: fail`") {
		t.Fatalf("a clearance recorded at an old head survived the head move: err = %v", err)
	}
	if err := checkSecurityVerdict(flipOpts{pr: 7}, privateCIRepo, pr, quiet, reviews, login, otherHead); err != nil {
		t.Fatalf("at the head it was recorded at, the clearance stands in the security lane: %v", err)
	}
}

// REVIEW STATE. The clearing pass counts only in a state that carries a standing pass,
// APPROVED or COMMENTED, the list `deskpost ready` reads for the same grant. Every other
// state the forge reports refuses with the clearance's own reason, which names the state, so
// the refusal is not the security-verdict reduction's (that one reads no state). The two
// accepted states flip.
//
// regression: #5092
func TestSecFailPassStates(t *testing.T) {
	for _, st := range []string{"APPROVED", "COMMENTED", "CHANGES_REQUESTED", "DISMISSED", "PENDING", ""} {
		t.Run("state="+st, func(t *testing.T) {
			s := secBodyStub(t)
			s.reviews[2].State = st
			if !deskkit.SecurityPassStateStands(st) {
				wantSecRefused(t, s, "a pass in review state "+st, "review state \""+st+"\" does not stand")
				return
			}
			s.install(t)
			if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
				t.Fatalf("a pass in review state %s: rc = %d, want %d", st, rc, deskkit.ExitOK)
			}
			if !s.flipped() {
				t.Errorf("a pass in review state %s: the ready mutation never ran", st)
			}
		})
	}
}
