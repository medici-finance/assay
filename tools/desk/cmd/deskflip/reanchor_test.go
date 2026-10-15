package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #3093 — GitHub RE-ANCHORS an old review's commit_id to a later head. The reported case:
// CHANGES_REQUESTED at commit A, then an APPROVED submitted while A was still the head, which
// the forge now reports at commit B — a merge commit whose committer date is seven hours
// AFTER the review was submitted. Read on commit_id alone, that APPROVED is "a new verdict at
// a new head" and clears the standing block: the flip proceeds on a head no reviewer saw.
// The fix: an APPROVED (or security pass) whose submitted_at predates the head commit's
// committer date verifies nothing at that head — it is stale.
const (
	reanchorCommitA     = "a0a0a0a0b1b1b1b1c2c2c2c2d3d3d3d3e4e4e4e4"
	reanchorBlockAt     = "2026-10-14T00:22:30Z"
	reanchorApproveAt   = "2026-10-15T14:41:06Z" // BEFORE the head commit existed
	reanchorHeadDate    = "2026-10-15T21:35:48Z" // head B's committer date
	reanchorFreshApprov = "2026-10-15T22:05:00Z" // AFTER the head commit — a genuine re-review
)

func reanchorReviews(t *testing.T, approveAt string) []reviewInfo {
	t.Helper()
	bot := reviewerBot(t)
	blocked := reviewInfo{State: "CHANGES_REQUESTED", CommitID: reanchorCommitA, Body: "findings", SubmittedAt: reanchorBlockAt}
	blocked.User.Login = bot
	approved := reviewInfo{State: "APPROVED", CommitID: headSHA, Body: "looks correct", SubmittedAt: approveAt}
	approved.User.Login = bot
	return []reviewInfo{blocked, approved}
}

// The exact reported sequence: CR@A, then an APPROVED re-anchored to head B whose
// submitted_at predates B's committer date. Not approved; the block stands; no mutation.
func TestReanchoredApprovalDoesNotClearAStandingBlock(t *testing.T) {
	s := newStub()
	s.headCommittedAt = reanchorHeadDate
	s.install(t)
	s.reviews = reanchorReviews(t, reanchorApproveAt)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("re-anchored approval rc = %d, want %d (refused) — an APPROVED submitted before "+
			"the head commit existed is not a verdict at that head", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a re-anchored approval produced mutations: %v", m)
	}
}

// The same stale APPROVED with no prior block: still not an approval at head.
func TestReanchoredApprovalAloneIsNotApproval(t *testing.T) {
	s := newStub()
	s.headCommittedAt = reanchorHeadDate
	s.install(t)
	s.reviews = reanchorReviews(t, reanchorApproveAt)[1:]

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("re-anchored lone approval rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a re-anchored approval produced mutations: %v", m)
	}
}

// Positive control: the same sequence with the APPROVED submitted AFTER head B's committer
// date is a genuine re-verification at B — it clears the block at A and the flip proceeds.
func TestApprovalSubmittedAfterHeadCommitIsAccepted(t *testing.T) {
	s := newStub()
	s.headCommittedAt = reanchorHeadDate
	s.install(t)
	s.reviews = reanchorReviews(t, reanchorFreshApprov)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("fresh approval at head rc = %d, want %d", rc, deskkit.ExitOK)
	}
	if !s.flipped() {
		t.Errorf("the ready mutation never ran: %v", s.requests)
	}
}

// Fail closed: when the head commit's date cannot be read, the approval is not at head.
func TestHeadCommitDateUnreadableFailsClosed(t *testing.T) {
	s := newStub()
	s.commitErr = true
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc == deskkit.ExitOK {
		t.Fatalf("unreadable head commit date rc = %d — the approval must not count as at head", rc)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("an unverifiable head date still produced mutations: %v", m)
	}
}

// The filter only ever removes GRANTS: a CHANGES_REQUESTED, a security fail and a review at
// another commit all survive it, even when they predate the head commit — removing any of
// them would loosen a gate this fix exists to tighten.
func TestWithoutStaleGrantsRemovesOnlyStaleGrants(t *testing.T) {
	bot := reviewerBot(t)
	mk := func(state, commit, body, at string) reviewInfo {
		r := reviewInfo{State: state, CommitID: commit, Body: body, SubmittedAt: at}
		r.User.Login = bot
		return r
	}
	reviews := []reviewInfo{
		mk("CHANGES_REQUESTED", headSHA, "findings", reanchorApproveAt),
		mk("COMMENTED", headSHA, "Security-Review: fail\nreason", reanchorApproveAt),
		mk("APPROVED", reanchorCommitA, "ok", reanchorApproveAt),
		mk("APPROVED", headSHA, "stale approve", reanchorApproveAt),
		mk("COMMENTED", headSHA, "Security-Review: pass", reanchorApproveAt),
		mk("APPROVED", headSHA, "fresh approve", reanchorFreshApprov),
	}
	headAt := func() (time.Time, error) { return deskkit.ParseCommitDate(reanchorHeadDate), nil }
	got, err := withoutStaleGrants(reviews, bot, headSHA, condReviewerApproved, headAt)
	if err != nil {
		t.Fatal(err)
	}
	var bodies []string
	for _, r := range got {
		bodies = append(bodies, r.Body)
	}
	want := []string{"findings", "Security-Review: fail\nreason", "ok", "fresh approve"}
	if strings.Join(bodies, "|") != strings.Join(want, "|") {
		t.Fatalf("kept %q, want %q", bodies, want)
	}

	// No grant at head: the head date is never read.
	read := false
	_, err = withoutStaleGrants(reviews[:3], bot, headSHA, condReviewerApproved, func() (time.Time, error) {
		read = true
		return time.Time{}, errors.New("unreadable")
	})
	if err != nil || read {
		t.Fatalf("no grant at head: err=%v read=%v, want no read and no error", err, read)
	}
}
