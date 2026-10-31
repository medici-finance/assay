package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #3093 — GitHub RE-ANCHORS an old review's commit_id to a later head. The reported case:
// CHANGES_REQUESTED at commit A, then an APPROVED submitted while A was still the head, which
// the forge now reports at head B — a merge commit whose committer date is seven hours AFTER
// the review was submitted. Read on commit_id alone, `ready` sees "the App's latest verdict is
// APPROVED at head" and flips a PR no reviewer saw at B. An APPROVED whose submitted_at
// predates the head commit's committer date is stale: not approval, not a re-verification.
const (
	reanchorBlockAt   = "2026-10-14T00:22:30Z"
	reanchorApproveAt = "2026-10-15T14:41:06Z" // BEFORE the head commit existed
	reanchorHeadDate  = "2026-10-15T21:35:48Z" // head B's committer date
	reanchorFreshAt   = "2026-10-15T22:05:00Z" // AFTER the head commit — a genuine re-review
)

func reanchorReviews(approveAt string) []reviewInfo {
	blocked := appReview("CHANGES_REQUESTED", testOldHead, "findings")
	blocked.SubmittedAt = reanchorBlockAt
	approved := appReview("APPROVED", testHead, okReviewBody)
	approved.SubmittedAt = approveAt
	return []reviewInfo{blocked, approved}
}

// The exact reported sequence: CR@A, then an APPROVED re-anchored to head B whose
// submitted_at predates B's committer date. Not approved, no flip.
func TestReadyReanchoredApprovalRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorReviews(reanchorApproveAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code == 0 {
		t.Fatalf("re-anchored approval exit = %d — an APPROVED submitted before the head commit "+
			"existed is not a verdict at that head", code)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d on a re-anchored approval, want 0", f.flips)
	}
}

// The same stale APPROVED with no prior block: still not an approval at head.
func TestReadyReanchoredLoneApprovalRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorReviews(reanchorApproveAt)[1:]
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code == 0 {
		t.Fatalf("re-anchored lone approval exit = %d, want a refusal", code)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d on a re-anchored approval, want 0", f.flips)
	}
}

// Positive control: the APPROVED submitted AFTER head B's committer date is a genuine
// verdict at B — it clears the block at A and the flip proceeds.
func TestReadyApprovalAfterHeadCommitAccepted(t *testing.T) {
	f, _ := setupFake(t)
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorReviews(reanchorFreshAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("fresh approval at head exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// Fail closed: when the head commit's date cannot be read, the approval is not at head.
func TestReadyHeadCommitDateUnreadableFailsClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.headCommitDateErr = true
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code == 0 {
		t.Fatalf("unreadable head commit date exit = %d — the approval must not count as at head", code)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d with an unreadable head date, want 0", f.flips)
	}
}

// reanchorSecurityReviews is a risk-classed PR's two App verdicts at head B: a genuine
// correctness APPROVED submitted after B existed, and a `Security-Review: pass` submitted at
// passAt. The security pass is the second, independent grant on a risk-classed PR, and the
// forge re-anchors it exactly as it does an APPROVED (#3093).
//
// The pass is in the legacy APPROVED shape; reanchorCommentedSecurityReviews is the COMMENTED
// shape `deskpost security-review --verdict pass` actually posts.
func reanchorSecurityReviews(passAt string) []reviewInfo {
	return reanchorSecurityReviewsIn("APPROVED", passAt)
}

// reanchorCommentedSecurityReviews is reanchorSecurityReviews with the security pass carried on
// a COMMENTED review — the shape `deskpost security-review --verdict pass` posts (the COMMENT
// event; see securityPassStates). An APPROVED pass is dropped by the filter's APPROVED arm on
// its own, so only this shape pins the filter's security-pass arm.
func reanchorCommentedSecurityReviews(passAt string) []reviewInfo {
	return reanchorSecurityReviewsIn("COMMENTED", passAt)
}

func reanchorSecurityReviewsIn(passState, passAt string) []reviewInfo {
	approved := appReview("APPROVED", testHead, okReviewBody)
	approved.SubmittedAt = reanchorFreshAt
	pass := appReview(passState, testHead, "## Security review\n\nNo auth regressions.\n\nSecurity-Review: pass\n")
	pass.SubmittedAt = passAt
	return []reviewInfo{approved, pass}
}

// A security pass re-anchored onto head B — submitted before B's committer date — is not a
// security verdict at B: gate (e) refuses the risk-classed flip.
func TestReadyReanchoredSecurityPassRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/service/jwt.key"} // risk-classed
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorSecurityReviews(reanchorApproveAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("re-anchored security pass exit = %d, want %d — a Security-Review: pass submitted "+
			"before the head commit existed is not a security verdict at that head", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d on a re-anchored security pass, want 0", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "Security-Review") {
		t.Errorf("the refusal must be gate (e)'s missing security verdict; audit detail: %s", d)
	}
}

// Positive control: the same security pass submitted AFTER head B's committer date grants.
func TestReadySecurityPassAfterHeadCommitAccepted(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/service/jwt.key"}
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorSecurityReviews(reanchorFreshAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("fresh security pass at head exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// The same re-anchored security pass in the shape the desk posts: a COMMENTED review carrying
// `Security-Review: pass`, submitted before head B's committer date. Gate (e) refuses.
func TestReadyReanchoredCommentedSecurityPassRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/service/jwt.key"} // risk-classed
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorCommentedSecurityReviews(reanchorApproveAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("re-anchored COMMENTED security pass exit = %d, want %d — a Security-Review: pass "+
			"submitted before the head commit existed is not a security verdict at that head", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d on a re-anchored COMMENTED security pass, want 0", f.flips)
	}
	if d := lastAudit(t).Detail; !strings.Contains(d, "Security-Review") {
		t.Errorf("the refusal must be gate (e)'s missing security verdict; audit detail: %s", d)
	}
}

// Positive control: the COMMENTED security pass submitted AFTER head B's committer date grants.
func TestReadyCommentedSecurityPassAfterHeadCommitAccepted(t *testing.T) {
	f, _ := setupFake(t)
	f.files = []string{"secrets/service/jwt.key"}
	f.headCommittedAt = reanchorHeadDate
	f.reviews = reanchorCommentedSecurityReviews(reanchorFreshAt)
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("fresh COMMENTED security pass at head exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// The external-prerequisite exemption decides on the filtered reviews too. The CR at head
// declares an external prerequisite; the APPROVE that cites the clearance was submitted
// BEFORE head B's committer date (re-anchored onto B), and the only APPROVE submitted after B
// cites nothing. The stale citation verified nothing at B, so it cannot clear the block.
func TestReadyReanchoredPrereqCitationDoesNotClear(t *testing.T) {
	const (
		headAt  = "2026-09-21T11:30:00Z" // after the citing APPROVE at epqApproveAt
		freshAt = "2026-09-21T12:00:00Z"
	)
	f, _ := setupFake(t)
	withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
	f.headCommittedAt = headAt

	cr := epqCRBody("waiting on "+epqPrereqObject+" to merge", epqExternalFinding("epq-1", epqPrereqObject))
	citing := epqApproveBody("epq-1 " + epqPrereqObject + " " + epqMergeSHA)
	f.reviews = []reviewInfo{
		epqReview("CHANGES_REQUESTED", testHead, epqCRAt, cr),
		epqReview("APPROVED", testHead, epqApproveAt, citing),
		epqReview("APPROVED", testHead, freshAt, epqApproveBody()),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code == 0 {
		t.Fatal("ready exit = 0 — a prerequisite clearance cited only on an APPROVE submitted before the " +
			"head commit existed is not a clearance at that head")
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// Positive control: the same CR and the same citation, with the citing APPROVE submitted
// after head B's committer date. The exemption clears and the flip proceeds.
func TestReadyPrereqCitationAfterHeadCommitClears(t *testing.T) {
	const headAt = "2026-09-21T10:45:00Z" // before the citing APPROVE at epqApproveAt
	f, _ := setupFake(t)
	withObserver(t, func(deskkit.PrereqCondition) deskkit.PrereqObservation { return epqSatisfied() })
	f.headCommittedAt = headAt

	cr := epqCRBody("waiting on "+epqPrereqObject+" to merge", epqExternalFinding("epq-1", epqPrereqObject))
	citing := epqApproveBody("epq-1 " + epqPrereqObject + " " + epqMergeSHA)
	f.reviews = []reviewInfo{
		epqReview("CHANGES_REQUESTED", testHead, epqCRAt, cr),
		epqReview("APPROVED", testHead, epqApproveAt, citing),
	}
	f.status = greenStatus()

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("ready exit = %d, want 0 (the citation after the head commit should clear)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}
