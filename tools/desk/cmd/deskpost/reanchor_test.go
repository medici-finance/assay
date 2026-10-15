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
func reanchorSecurityReviews(passAt string) []reviewInfo {
	approved := appReview("APPROVED", testHead, okReviewBody)
	approved.SubmittedAt = reanchorFreshAt
	pass := appReview("APPROVED", testHead, "## Security review\n\nNo auth regressions.\n\nSecurity-Review: pass\n")
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
