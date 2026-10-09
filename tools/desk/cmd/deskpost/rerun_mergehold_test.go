package main

// rerun_mergehold_test.go — what a verdict verb does about the merge-hold when it finds its
// verdict already on the change and so posts nothing.
//
// The forge here is the GitLab review fake, wrapped so that the reviews read serves back what
// was posted (as the backend's own read does) and so that a hold write that succeeds changes
// what the next hold read answers. Without the first, a re-run would post again and the path
// under test would never be reached; without the second, a test could not tell an armed hold
// from a released one.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const rcReviewBody = "## Review\n\nBlocking issue found.\n\nVerdict: request-changes\n"

// glHoldFake is glReviewFake plus a reviews read and a hold that keep state.
type glHoldFake struct {
	*glReviewFake

	// seeded is what the change's reviews already hold before this test posts, oldest
	// first. Reviews posted through PostReview are served after them.
	seeded     []deskkit.Review
	reviewsErr error
	// postErr, when set, is what PostReview answers AFTER it has recorded the review: the
	// forge accepted the post and the response was lost on the way back.
	postErr    error
	visibility string
	trust      *deskkit.TrustPayload
}

func newGLHoldFake() *glHoldFake {
	return &glHoldFake{glReviewFake: newGLReviewFake(), visibility: "private"}
}

func (g *glHoldFake) ReviewsAtHead(deskkit.ForgeRepo, int) ([]deskkit.Review, error) {
	if g.reviewsErr != nil {
		return nil, g.reviewsErr
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	out := append([]deskkit.Review(nil), g.seeded...)
	for i, in := range g.postedReview {
		out = append(out, deskkit.Review{
			ID:          int64(900 + i),
			Author:      deskkit.Account{Login: reviewerBotDisplay()},
			State:       deskkit.VerdictNoteState(in.Body),
			CommitID:    in.HeadSHA,
			Body:        in.Body,
			SubmittedAt: fmt.Sprintf("2026-03-01T10:%02d:00Z", i),
		})
	}
	return out, nil
}

// PostReview records the review, then answers postErr.
func (g *glHoldFake) PostReview(fr deskkit.ForgeRepo, n int, in deskkit.ReviewInput) error {
	if err := g.glReviewFake.PostReview(fr, n, in); err != nil {
		return err
	}
	return g.postErr
}

// SetMergeHold records the call and, when the write is not failed, applies it.
func (g *glHoldFake) SetMergeHold(_ deskkit.ForgeRepo, _ int, in deskkit.MergeHoldUpdate) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.setHoldCalls = append(g.setHoldCalls, in)
	if g.holdSetErr != nil {
		return g.holdSetErr
	}
	if in.Resolved {
		g.holdState, g.holdHead, g.holdResolvedBy = deskkit.MergeHoldResolved, in.Head, reviewerBotDisplay()
	} else {
		g.holdState, g.holdHead, g.holdResolvedBy = deskkit.MergeHoldUnresolved, "", ""
	}
	return nil
}

func (g *glHoldFake) RepoVisibility(deskkit.ForgeRepo) (string, error) { return g.visibility, nil }

func (g *glHoldFake) PRTrustEvents(deskkit.ForgeRepo, int) (*deskkit.TrustPayload, error) {
	if g.trust == nil {
		return nil, errors.New("the trust events were not expected to be read")
	}
	return g.trust, nil
}

// releases counts hold writes that RELEASE. No test in this file may see one on a re-run.
func (g *glHoldFake) releases() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.setHoldCalls {
		if c.Resolved {
			n++
		}
	}
	return n
}

// glRecorded is a verdict note already on the change, in the shape the writer posts.
func glRecorded(t *testing.T, id int64, login, body, head, at string) deskkit.Review {
	t.Helper()
	posted, err := deskkit.AppendOnBehalfOf([]byte(body), "", glReviewRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf: %v", err)
	}
	if string(posted) == body {
		t.Fatal("fixture defect: the writer appended no on-behalf-of line")
	}
	state := deskkit.VerdictNoteState(body)
	if state == "" {
		t.Fatalf("fixture defect: %q carries no verdict line", body)
	}
	return deskkit.Review{ID: id, Author: deskkit.Account{Login: login}, State: state, CommitID: head, Body: string(posted), SubmittedAt: at}
}

func rerunReviewer(t *testing.T) string {
	t.Helper()
	login := reviewerBotDisplay()
	if login == "" {
		t.Fatal("fixture defect: no reviewer identity is configured")
	}
	return login
}

// TestGitLabRerunRearmsTheMergeHold: a request-changes verdict lands on the change but the
// run that posted it does not complete the hold step, and exits 6. Two ways there: the hold
// write fails, or the forge accepts the post and its response is lost, so the hold step is
// never attempted. The same command run again finds the verdict on the change, posts
// nothing, and must leave the hold armed — it runs the hold step. Before this was fixed the
// second run exited 0 as a no-op and never looked at the hold.
func TestGitLabRerunRearmsTheMergeHold(t *testing.T) {
	for _, tc := range []struct {
		name           string
		holdState      string
		holdHead       string
		earlierApprove bool
		lostResponse   bool // the post lands and its response is lost; else the hold write fails
		forget         bool // the second run has none of the first run's audit rows
	}{
		{name: "the hold was open and its write failed"},
		{name: "the post landed and its response was lost, the hold released at this head",
			holdState: deskkit.MergeHoldResolved, holdHead: testHead, earlierApprove: true, lostResponse: true},
		{name: "the post landed and its response was lost, the hold resolved at an older head",
			holdState: deskkit.MergeHoldResolved, holdHead: testOldHead, lostResponse: true, forget: true},
		{name: "the same, re-run from another session", forget: true},
		{name: "an earlier approve had released the hold at this head",
			holdState: deskkit.MergeHoldResolved, holdHead: testHead, earlierApprove: true},
		{name: "the hold stands resolved at an older head",
			holdState: deskkit.MergeHoldResolved, holdHead: testOldHead, forget: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGLHoldFake()
			f.holdState, f.holdHead = tc.holdState, tc.holdHead
			errBuf := setupGitLabForge(t, f)
			if tc.earlierApprove {
				f.seeded = []deskkit.Review{glRecorded(t, 41, rerunReviewer(t), okReviewBody, testHead, "2026-03-01T09:00:00Z")}
			}
			bf := writeBody(t, "rev.md", rcReviewBody)
			args := reviewArgs(glReviewRepo, "1", "request-changes", testHead, bf)

			if tc.lostResponse {
				f.postErr = errors.New("502 bad gateway")
			} else {
				f.holdSetErr = errors.New("503 the instance is unavailable")
			}
			if code := run(args); code != deskkit.ExitUnverifiable {
				t.Fatalf("first run exit = %d, want %d\nstderr: %s", code, deskkit.ExitUnverifiable, errBuf.String())
			}
			if len(f.postedReview) != 1 {
				t.Fatalf("first run posted %d review(s), want 1", len(f.postedReview))
			}
			if f.holdState == deskkit.MergeHoldUnresolved && tc.holdState == deskkit.MergeHoldResolved {
				t.Fatal("fixture defect: the first run changed the hold")
			}
			firstCalls := len(f.setHoldCalls)
			if tc.lostResponse != (firstCalls == 0) {
				t.Fatalf("fixture defect: the first run sent %d hold write(s) (lost response: %v)", firstCalls, tc.lostResponse)
			}

			f.holdSetErr, f.postErr = nil, nil
			if tc.forget {
				forgetThisSession(t)
			}
			errBuf.Reset()
			code := run(args)
			if len(f.postedReview) != 1 {
				t.Fatalf("the re-run posted again: %d review(s), want 1", len(f.postedReview))
			}
			if code != deskkit.ExitOK {
				t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, errBuf.String())
			}
			wantWrites := 1
			if tc.holdHead == testOldHead {
				wantWrites = 2 // the stale resolution is re-armed first, as it is after a post
			}
			if got := len(f.setHoldCalls) - firstCalls; got != wantWrites {
				t.Fatalf("the re-run sent %d hold write(s), want %d — it must run the hold step, not skip it: %+v", got, wantWrites, f.setHoldCalls)
			}
			last := f.setHoldCalls[len(f.setHoldCalls)-1]
			if last.Resolved || last.Reason != "request-changes" {
				t.Fatalf("the re-run's hold write = %+v, want a re-arm with reason \"request-changes\"", last)
			}
			if f.holdState != deskkit.MergeHoldUnresolved {
				t.Fatalf("hold after the re-run = %q, want %q", f.holdState, deskkit.MergeHoldUnresolved)
			}
			e := lastAudit(t)
			if e.Result != deskkit.ResultOK {
				t.Fatalf("re-run audit result = %q, want ok (a hold write was sent): %+v", e.Result, e)
			}
			for _, want := range []string{"not posted again", "merge-hold re-armed"} {
				if !strings.Contains(e.Detail, want) {
					t.Errorf("re-run audit detail lacks %q: %q", want, e.Detail)
				}
			}
		})
	}
}

// TestGitLabRerunHoldStepOnlyArmsOrConfirms walks the already-recorded path over the states
// it can meet. Three rules hold in every row: nothing is posted, no hold write releases, and
// the run exits 0 only when the hold agrees with the verdict it found.
func TestGitLabRerunHoldStepOnlyArmsOrConfirms(t *testing.T) {
	const (
		tRecorded = "2026-03-01T09:00:00Z"
		tLater    = "2026-03-01T09:30:00Z"
	)
	for _, tc := range []struct {
		name      string
		verdict   string // the verdict this run is asked to post; it is already recorded
		later     func(t *testing.T) []deskkit.Review
		holdState string
		holdHead  string
		holdBy    string // who resolved the hold; the reviewer identity when empty
		readErr   error

		wantExit   int
		wantWrites int    // hold writes, every one a re-arm
		wantResult string // audit result
		wantText   string // in stderr or the audit detail
	}{
		{name: "approve recorded, hold released at this head",
			verdict: "approve", holdState: deskkit.MergeHoldResolved, holdHead: testHead,
			wantExit: deskkit.ExitOK, wantResult: deskkit.ResultNoop, wantText: "released at this head"},
		{name: "approve recorded, hold still open: reported, never released here",
			verdict: "approve", holdState: deskkit.MergeHoldUnresolved,
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "does not release"},
		{name: "approve recorded, hold resolved at an older head: re-armed and reported",
			verdict: "approve", holdState: deskkit.MergeHoldResolved, holdHead: testOldHead,
			wantExit: deskkit.ExitUnverifiable, wantWrites: 1, wantResult: deskkit.ResultUnverifiable, wantText: "does not release"},
		{name: "approve recorded, hold resolved at this head by another account",
			verdict: "approve", holdState: deskkit.MergeHoldResolved, holdHead: testHead, holdBy: "someone-else",
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "not by the reviewer identity"},
		{name: "approve recorded, hold resolved with no head named",
			verdict: "approve", holdState: deskkit.MergeHoldResolved,
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "names no head"},
		{name: "request-changes recorded, hold open",
			verdict: "request-changes", holdState: deskkit.MergeHoldUnresolved,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{glRecorded(t, 52, rerunReviewer(t), okReviewBody, testHead, tLater)}
			},
			wantExit: deskkit.ExitOK, wantResult: deskkit.ResultNoop, wantText: "merge-hold is armed"},
		{name: "request-changes recorded, a later approve by the reviewer released the hold",
			verdict: "request-changes", holdState: deskkit.MergeHoldResolved, holdHead: testHead,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{glRecorded(t, 52, rerunReviewer(t), okReviewBody, testHead, tLater)}
			},
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "review id 52"},
		{name: "request-changes recorded, a later review by the reviewer has no readable kind",
			verdict: "request-changes", holdState: deskkit.MergeHoldResolved, holdHead: testHead,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{{ID: 53, Author: deskkit.Account{Login: rerunReviewer(t)}, State: "APPROVED", CommitID: testHead, SubmittedAt: tLater}}
			},
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "review id 53"},
		{name: "request-changes recorded, a later security fail is the other lane's",
			verdict: "request-changes", holdState: deskkit.MergeHoldResolved, holdHead: testHead,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{glRecorded(t, 54, rerunReviewer(t), secFailBody, testHead, tLater)}
			},
			wantExit: deskkit.ExitOK, wantWrites: 1, wantResult: deskkit.ResultOK, wantText: "merge-hold re-armed"},
		{name: "request-changes recorded, a later approve is another account's",
			verdict: "request-changes", holdState: deskkit.MergeHoldResolved, holdHead: testHead,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{glRecorded(t, 55, "someone-else", okReviewBody, testHead, tLater)}
			},
			wantExit: deskkit.ExitOK, wantWrites: 1, wantResult: deskkit.ResultOK, wantText: "merge-hold re-armed"},
		{name: "request-changes recorded but superseded, hold resolved at an older head: re-armed",
			verdict: "request-changes", holdState: deskkit.MergeHoldResolved, holdHead: testOldHead,
			later: func(t *testing.T) []deskkit.Review {
				return []deskkit.Review{glRecorded(t, 52, rerunReviewer(t), okReviewBody, testHead, tLater)}
			},
			wantExit: deskkit.ExitOK, wantWrites: 1, wantResult: deskkit.ResultOK, wantText: "merge-hold re-armed"},
		{name: "request-changes recorded, the change has no hold thread",
			verdict: "request-changes", holdState: deskkit.MergeHoldAbsent,
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "no merge-hold"},
		{name: "request-changes recorded, the hold cannot be read",
			verdict: "request-changes", readErr: errors.New("502 bad gateway"),
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "could not be read"},
		{name: "approve recorded, the hold cannot be read",
			verdict: "approve", readErr: errors.New("502 bad gateway"),
			wantExit: deskkit.ExitUnverifiable, wantResult: deskkit.ResultUnwritten, wantText: "could not be read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGLHoldFake()
			f.holdState, f.holdHead, f.holdReadErr = tc.holdState, tc.holdHead, tc.readErr
			errBuf := setupGitLabForge(t, f)
			body := okReviewBody
			if tc.verdict == "request-changes" {
				body = rcReviewBody
			}
			f.seeded = []deskkit.Review{glRecorded(t, 51, rerunReviewer(t), body, testHead, tRecorded)}
			if tc.later != nil {
				f.seeded = append(f.seeded, tc.later(t)...)
			}
			if f.holdResolvedBy = tc.holdBy; tc.holdBy == "" && tc.holdState == deskkit.MergeHoldResolved {
				f.holdResolvedBy = rerunReviewer(t)
			}
			bf := writeBody(t, "rev.md", body)

			code := run(reviewArgs(glReviewRepo, "1", tc.verdict, testHead, bf))
			if len(f.postedReview) != 0 {
				t.Fatalf("posted %d review(s): the verdict was already recorded", len(f.postedReview))
			}
			if n := f.releases(); n != 0 {
				t.Fatalf("%d hold write(s) RELEASED on the already-recorded path: %+v", n, f.setHoldCalls)
			}
			if code != tc.wantExit {
				t.Fatalf("exit = %d, want %d\nstderr: %s", code, tc.wantExit, errBuf.String())
			}
			if len(f.setHoldCalls) != tc.wantWrites {
				t.Fatalf("hold writes = %d, want %d: %+v", len(f.setHoldCalls), tc.wantWrites, f.setHoldCalls)
			}
			e := lastAudit(t)
			if e.Result != tc.wantResult {
				t.Fatalf("audit result = %q, want %q: %+v", e.Result, tc.wantResult, e)
			}
			if all := errBuf.String() + "\n" + e.Detail; !strings.Contains(all, tc.wantText) {
				t.Errorf("neither stderr nor the audit detail holds %q:\n%s", tc.wantText, all)
			}
			if tc.wantExit == deskkit.ExitOK && tc.verdict == "request-changes" && f.holdState != deskkit.MergeHoldUnresolved {
				t.Fatalf("exit 0 with the hold %q under a recorded request-changes", f.holdState)
			}
		})
	}
}

// TestGitLabRerunHoldWritePassesTheWriteGates: the re-arm on the already-recorded path is an
// outward write, so it is sent only where the post step itself could have written — a
// trusted change, a repository this identity may write to — and never on a dry run.
func TestGitLabRerunHoldWritePassesTheWriteGates(t *testing.T) {
	for _, tc := range []struct {
		name       string
		arrange    func(f *glHoldFake)
		extra      []string
		wantExit   int
		wantResult string
	}{
		{name: "the repository reads public and is not listed as one",
			arrange:  func(f *glHoldFake) { f.visibility = "public" },
			wantExit: deskkit.ExitRefused, wantResult: deskkit.ResultRefused},
		{name: "the change's author is not trusted and not blessed",
			arrange: func(f *glHoldFake) {
				f.prAuthor = deskkit.Account{Login: "stranger", ID: 9}
				f.trust = &deskkit.TrustPayload{Complete: true}
			},
			wantExit: deskkit.ExitRefused, wantResult: deskkit.ResultRefused},
		{name: "a dry run", extra: []string{"--dry-run"},
			wantExit: deskkit.ExitOK, wantResult: deskkit.ResultDryRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGLHoldFake()
			f.holdState, f.holdHead = deskkit.MergeHoldResolved, testHead
			if tc.arrange != nil {
				tc.arrange(f)
			}
			errBuf := setupGitLabForge(t, f)
			f.seeded = []deskkit.Review{glRecorded(t, 51, rerunReviewer(t), rcReviewBody, testHead, "2026-03-01T09:00:00Z")}
			bf := writeBody(t, "rev.md", rcReviewBody)

			code := run(append(reviewArgs(glReviewRepo, "1", "request-changes", testHead, bf), tc.extra...))
			if len(f.setHoldCalls) != 0 {
				t.Fatalf("a hold write was sent: %+v", f.setHoldCalls)
			}
			if len(f.postedReview) != 0 {
				t.Fatalf("posted %d review(s)", len(f.postedReview))
			}
			if code != tc.wantExit {
				t.Fatalf("exit = %d, want %d\nstderr: %s", code, tc.wantExit, errBuf.String())
			}
			if e := lastAudit(t); e.Result != tc.wantResult {
				t.Fatalf("audit result = %q, want %q: %+v", e.Result, tc.wantResult, e)
			}
		})
	}
}

// TestGitLabFinishRerunRearmsTheHoldBeforeItReleasesTheClaim: `finish review` end to end on
// the forge with a merge-hold. The first run posts, fails the hold write, and stops at step 1
// with the claim held. The second run posts nothing, re-arms the hold, confirms and only
// then releases the claim.
func TestGitLabFinishRerunRearmsTheHoldBeforeItReleasesTheClaim(t *testing.T) {
	f := newGLHoldFake()
	f.holdState, f.holdHead = deskkit.MergeHoldResolved, testHead
	errBuf := setupGitLabForge(t, f)
	f.seeded = []deskkit.Review{glRecorded(t, 41, rerunReviewer(t), okReviewBody, testHead, "2026-03-01T09:00:00Z")}

	var out strings.Builder
	savedOut := stdout
	stdout = &out
	t.Cleanup(func() { stdout = savedOut })

	const key = "gl-repo--pr-1"
	released := 0
	holdAtRelease := ""
	savedRelease := releaseReviewClaimFn
	releaseReviewClaimFn = func(_, _, k string) (string, error) {
		released++
		holdAtRelease = f.holdState
		if k != key {
			t.Errorf("released %q, want %q", k, key)
		}
		return finishClaimReleased, nil
	}
	t.Cleanup(func() { releaseReviewClaimFn = savedRelease })

	bf := writeBody(t, "rev.md", rcReviewBody)
	args := []string{"finish", "review", glReviewRepo, "1", "--verdict", "request-changes", "--head", testHead, "--body-file", bf, "--claim", key}

	f.holdSetErr = errors.New("503 the instance is unavailable")
	if code := run(args); code != deskkit.ExitUnverifiable {
		t.Fatalf("first run exit = %d, want %d\nstderr: %s", code, deskkit.ExitUnverifiable, errBuf.String())
	}
	if len(f.postedReview) != 1 || released != 0 {
		t.Fatalf("first run: posted %d, released %d — want 1 posted and the claim held", len(f.postedReview), released)
	}
	if !strings.Contains(errBuf.String(), "STOPPED at step 1 of 3 (post)") {
		t.Fatalf("first run did not stop at the post step:\n%s", errBuf.String())
	}
	if f.holdState != deskkit.MergeHoldResolved {
		t.Fatalf("fixture defect: hold after the failed write = %q", f.holdState)
	}

	f.holdSetErr = nil
	errBuf.Reset()
	if code := run(args); code != deskkit.ExitOK {
		t.Fatalf("re-run exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if len(f.postedReview) != 1 {
		t.Fatalf("the re-run posted again: %d review(s)", len(f.postedReview))
	}
	if released != 1 {
		t.Fatalf("the re-run released the claim %d time(s), want 1", released)
	}
	if holdAtRelease != deskkit.MergeHoldUnresolved {
		t.Fatalf("the claim was released with the hold %q — the hold must be armed first", holdAtRelease)
	}
	if got := out.String(); !strings.Contains(got, "state=CHANGES_REQUESTED") || !strings.Contains(got, "claim=released") {
		t.Fatalf("result line = %q", got)
	}
}
