package main

import "testing"

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
