package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- #37, adversarial direction: construct the forgery, do not merely assert the guard ---
//
// The tests beside these state the SHAPE the fix targets. These attack it: each one builds a
// forged artifact and captures the refusal, a control proves a genuine verdict still flips
// (a gate that refuses everything is a brick, not a gate), and two more prove the gate fails
// CLOSED when the reviews collection is empty or unreadable — the direction where an absence
// silently reads as "nothing wrong here". All fixtures are synthetic.
//
// The four ATTACK axes are the four ways something could be made to read as a reviewer
// verdict: wrong KIND (a comment), wrong IDENTITY, wrong COMMIT, and right-everything-but-
// verifying-nothing (the no-op approval #37 is named for).

const forgedApprovalBody = "## Review\n\nLooks good, approving.\n\nVerdict: approve\n"
const standingBlockBody = "## Review\n\nblocker.\n\nVerdict: request-changes\n"

// ATTACK 1 — a COMMENT shaped like a verdict.
//
// deskpost comment posts under the reviewer App's own identity, so a worker note can wear
// the reviewer's face. If this gate distinguished a review from a comment by AUTHOR
// IDENTITY alone, that fallback would be a live confusion surface. It does not: the gate
// reads /pulls/{n}/reviews, and an issue comment is not in that collection at all. The
// nearest thing that IS in it — a COMMENTED-event review whose body carries a verdict line
// — is excluded by STATE, not by body parsing.
func TestForgery37_VerdictShapedCommentIsNotAVerdict(t *testing.T) {
	f, _ := setupFake(t)
	// The forger's best shot: a verdict-shaped body on the reviewer App's identity,
	// planted on BOTH comment surfaces, plus a COMMENTED review at head.
	f.issueComments = []map[string]any{
		{"id": 1, "body": forgedApprovalBody, "user": map[string]any{"login": reviewerBotDisplay()}},
	}
	f.reviews = []reviewInfo{
		appReview("COMMENTED", testHead, forgedApprovalBody),
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("FORGERY ADMITTED: verdict-shaped comment exit = %d, want 5 (refused)", code)
	}
	if f.flips != 0 {
		t.Fatal("FORGERY ADMITTED: a comment wearing the reviewer App's identity flipped the PR")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "no APPROVED/CHANGES_REQUESTED") {
		t.Fatalf("refusal should name the missing decisive verdict, got: %s", d)
	}
	t.Logf("REFUSAL: exit=%d detail=%s", code, d)
}

// ATTACK 1b — the comment surface is never even read.
// Structural proof, not a body-parse proof: zero GETs against /issues/{n}/comments.
func TestForgery37_ReadyNeverReadsTheCommentSurface(t *testing.T) {
	f, _ := setupFake(t)
	f.issueComments = []map[string]any{
		{"id": 1, "body": forgedApprovalBody, "user": map[string]any{"login": reviewerBotDisplay()}},
	}
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()

	_ = run(readyArgs(exampleRepo))
	if n := f.hitCount("GET", "/issues/1/comments"); n != 0 {
		t.Fatalf("ready read the comment surface %d time(s) — it must never read it (#513)", n)
	}
	t.Logf("GET /issues/{n}/comments hits during a full ready flip: 0")
}

// ATTACK 2 — a verdict from the WRONG identity.
func TestForgery37_ApprovalFromWrongIdentityRefused(t *testing.T) {
	f, _ := setupFake(t)
	impostor := reviewInfo{State: "APPROVED", CommitID: testHead, Body: forgedApprovalBody}
	impostor.User.Login = "assay-desk-app[bot]" // a real sibling App, not the reviewer
	other := reviewInfo{State: "APPROVED", CommitID: testHead, Body: forgedApprovalBody}
	other.User.Login = "example-human"
	f.reviews = []reviewInfo{impostor, other}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("FORGERY ADMITTED: wrong-identity approval exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("FORGERY ADMITTED: a non-reviewer identity satisfied the correctness gate")
	}
	t.Logf("REFUSAL: exit=%d detail=%s", code, lastAudit(t).Detail)
}

// ATTACK 3 — a genuine-looking APPROVED whose commit_id is NOT head (superseded commit).
func TestForgery37_ApprovalAtSupersededCommitRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testOldHead, okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("FORGERY ADMITTED: superseded-commit approval exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("FORGERY ADMITTED: an approval at a superseded commit flipped the PR")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "stale") {
		t.Fatalf("refusal should name staleness, got: %s", d)
	}
	t.Logf("REFUSAL: exit=%d detail=%s", code, d)
}

// ATTACK 4 — an APPROVED with an ABSENT commit_id. An absence must not read as "at head".
func TestForgery37_ApprovalWithEmptyCommitIDRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", "", okReviewBody)}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("FORGERY ADMITTED: empty-commit_id approval exit = %d, want 5", code)
	}
	if f.flips != 0 {
		t.Fatal("FORGERY ADMITTED: an approval carrying no commit_id flipped the PR")
	}
	t.Logf("REFUSAL: exit=%d detail=%s", code, lastAudit(t).Detail)
}

// ATTACK 5 — the live-evidence shape: same-head APPROVED over a standing CHANGES_REQUESTED,
// including the retry, including a security pass interleaved to try to reset the block.
func TestForgery37_NoOpApprovalAndItsVariantsRefused(t *testing.T) {
	cases := []struct {
		name    string
		reviews []reviewInfo
	}{
		{"plain no-op", []reviewInfo{
			appReview("CHANGES_REQUESTED", testHead, standingBlockBody),
			appReview("APPROVED", testHead, forgedApprovalBody),
		}},
		{"retried no-op", []reviewInfo{
			appReview("CHANGES_REQUESTED", testHead, standingBlockBody),
			appReview("APPROVED", testHead, forgedApprovalBody),
			appReview("APPROVED", testHead, "Approving again.\n"),
		}},
		{"security pass interleaved to reset the block", []reviewInfo{
			appReview("CHANGES_REQUESTED", testHead, standingBlockBody),
			appReview("APPROVED", testHead, "## Security review\n\nSecurity-Review: pass\n"),
			appReview("APPROVED", testHead, forgedApprovalBody),
		}},
		{"COMMENTED noise interleaved to reset the block", []reviewInfo{
			appReview("CHANGES_REQUESTED", testHead, standingBlockBody),
			appReview("COMMENTED", testHead, "just a note\n"),
			appReview("APPROVED", testHead, forgedApprovalBody),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := setupFake(t)
			f.reviews = tc.reviews
			f.status = greenStatus()

			code := run(readyArgs(exampleRepo))
			if code != deskkit.ExitRefused {
				t.Fatalf("FORGERY ADMITTED: %s exit = %d, want 5 (refused)", tc.name, code)
			}
			if f.flips != 0 {
				t.Fatalf("FORGERY ADMITTED: %s flipped the PR", tc.name)
			}
			t.Logf("REFUSAL (%s): exit=%d detail=%s", tc.name, code, lastAudit(t).Detail)
		})
	}
}

// CONTROL — a GENUINE verdict still passes. Without this the suite proves only that the
// tool refuses everything, which is not a gate, it is a brick.
func TestForgery37_GenuineVerdictStillFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{
		appReview("CHANGES_REQUESTED", testOldHead, standingBlockBody), // rejected at the old head
		appReview("APPROVED", testHead, okReviewBody),                  // re-reviewed AFTER a real push
	}
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("CRY WOLF: a genuine post-push approval exit = %d, want 0; detail=%s", code, lastAudit(t).Detail)
	}
	if f.flips != 1 {
		t.Fatalf("CRY WOLF: genuine approval produced %d flips, want 1", f.flips)
	}
	t.Logf("GENUINE VERDICT PASSES: exit=0, flips=1")
}

// FAIL CLOSED 1 — the reviews API returns EMPTY.
//
// An empty result must report could-not-satisfy, never "no forgery detected". This is the
// shape that produced a verified false negative elsewhere today (an empty `gh` read taken
// as "no collisions").
func TestForgery37_EmptyReviewsFailsClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{} // 200 OK, zero rows
	f.status = greenStatus()

	code := run(readyArgs(exampleRepo))
	if code == 0 {
		t.Fatal("FAILS OPEN: an empty reviews collection flipped the PR")
	}
	if code != deskkit.ExitRefused {
		t.Fatalf("empty-reviews exit = %d, want 5 (refused)", code)
	}
	if f.flips != 0 {
		t.Fatal("FAILS OPEN: an empty reviews collection produced a flip")
	}
	d := lastAudit(t).Detail
	if !strings.Contains(d, "no APPROVED/CHANGES_REQUESTED") {
		t.Fatalf("an empty read must name the ABSENT verdict, not report a clean bill; got: %s", d)
	}
	t.Logf("FAILS CLOSED (empty): exit=%d detail=%s", code, d)
}

// FAIL CLOSED 2 — the reviews API is UNREACHABLE (500) and FORBIDDEN (403).
// Must be exit 6 unverifiable — could-not-check — never a flip and never a refusal that
// claims to have read something.
func TestForgery37_UnreadableReviewsFailsClosed(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			f, _ := setupFake(t)
			f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
			f.status = greenStatus()
			f.intercept = func(method, path string) (int, bool) {
				if method == "GET" && strings.HasSuffix(path, "/reviews") {
					return status, true
				}
				return 0, false
			}

			code := run(readyArgs(exampleRepo))
			if code == 0 {
				t.Fatalf("FAILS OPEN: an unreadable reviews API (%d) flipped the PR", status)
			}
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("unreadable-reviews exit = %d, want 6 (could-not-check)", code)
			}
			if f.flips != 0 {
				t.Fatalf("FAILS OPEN: an unreadable reviews API (%d) produced a flip", status)
			}
			t.Logf("FAILS CLOSED (%d): exit=%d detail=%s", status, code, lastAudit(t).Detail)
		})
	}
}
