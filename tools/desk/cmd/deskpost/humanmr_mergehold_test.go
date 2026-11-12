package main

// humanmr_mergehold_test.go — a request-changes verdict on a change that carries no merge-hold
// marker thread (#5898).
//
// Every change a desk verb opens gets its marker thread when it is created
// (deskkit.CreateHeldDraftChange). A merge request a human opens does not: it reaches review
// through a blessing, with no thread at all. Before the fix, a request-changes posted on such a
// change landed as a review, the hold step found nothing to re-arm, the verb exited 6, and the
// server-side gate (an unresolved thread blocks the merge) was never armed. The forge here is
// the GitLab hold fake from rerun_mergehold_test.go: a thread it opens reads unresolved, and a
// hold write on a change with no thread is refused, as the GitLab backend refuses it.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// humanMRFake is a GitLab change opened by a human outside the trusted set, admitted to review
// by the blessing authority's comment, with no merge-hold thread.
func humanMRFake() *glHoldFake {
	f := newGLHoldFake()
	f.prAuthor = deskkit.Account{Login: "contributor", ID: 7001}
	f.commitLogin = "contributor"
	f.trust = &deskkit.TrustPayload{Complete: true, Events: []deskkit.ContentEvent{
		{Author: "ada", AuthorID: 2001, CreatedAt: time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)},
	}}
	f.holdState = deskkit.MergeHoldAbsent
	return f
}

// regression: #5898
// TestHumanMRRequestChangesHolds: the request-changes leaves the change held, and a later
// approve at the current head releases that same hold.
func TestHumanMRRequestChangesHolds(t *testing.T) {
	f := humanMRFake()
	errBuf := setupGitLabForge(t, f)

	rc := writeBody(t, "rc.md", rcReviewBody)
	if code := run(reviewArgs(glReviewRepo, "1", "request-changes", testHead, rc)); code != deskkit.ExitOK {
		t.Fatalf("request-changes exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if len(f.postedReview) != 1 || f.postedReview[0].Event != "REQUEST_CHANGES" {
		t.Fatalf("posted %+v, want one REQUEST_CHANGES verdict", f.postedReview)
	}
	if f.opens != 1 {
		t.Fatalf("hold threads opened = %d, want 1", f.opens)
	}
	if f.holdState != deskkit.MergeHoldUnresolved {
		t.Fatalf("hold after the request-changes = %q, want %q: the merge is not held", f.holdState, deskkit.MergeHoldUnresolved)
	}
	if len(f.setHoldCalls) != 0 {
		t.Fatalf("hold writes = %+v, want none: a thread with no history is armed by opening it", f.setHoldCalls)
	}
	e := lastAudit(t)
	if e.Result != deskkit.ResultOK || !strings.Contains(e.Detail, "so one was opened") {
		t.Fatalf("audit = %+v, want ok naming the opened hold", e)
	}
	if !strings.Contains(errBuf.String(), "deskpost: NOTE: the PR carried no merge-hold thread") {
		t.Errorf("stderr does not report the opened hold:\n%s", errBuf.String())
	}

	errBuf.Reset()
	ok := writeBody(t, "ok.md", okReviewBody)
	if code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, ok)); code != deskkit.ExitOK {
		t.Fatalf("approve exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if f.opens != 1 {
		t.Fatalf("the approve opened a hold thread: %d opened in all, want 1", f.opens)
	}
	if len(f.setHoldCalls) != 1 || !f.setHoldCalls[0].Resolved || f.setHoldCalls[0].Head != testHead {
		t.Fatalf("hold writes = %+v, want one release at %s", f.setHoldCalls, testHead)
	}
	if f.holdState != deskkit.MergeHoldResolved || f.holdHead != testHead {
		t.Fatalf("hold after the approve = %q at %q, want resolved at %s", f.holdState, f.holdHead, testHead)
	}
}

// regression: #5898
// TestHumanMROpenFailIsLoud: when the thread cannot be opened, the verb says the verdict
// landed and the gate is NOT armed, and exits non-zero.
func TestHumanMROpenFailIsLoud(t *testing.T) {
	f := humanMRFake()
	f.openErr = errors.New("503 the instance is unavailable")
	errBuf := setupGitLabForge(t, f)

	rc := writeBody(t, "rc.md", rcReviewBody)
	if code := run(reviewArgs(glReviewRepo, "1", "request-changes", testHead, rc)); code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d\nstderr: %s", code, deskkit.ExitUnverifiable, errBuf.String())
	}
	if len(f.postedReview) != 1 || f.opens != 1 {
		t.Fatalf("posted %d, opened %d — want the verdict posted and one open attempted", len(f.postedReview), f.opens)
	}
	if f.holdState != deskkit.MergeHoldAbsent {
		t.Fatalf("fixture defect: hold after a failed open = %q", f.holdState)
	}
	if !strings.Contains(errBuf.String(), "NOT armed") {
		t.Errorf("stderr does not say the gate is not armed:\n%s", errBuf.String())
	}
}

// TestHumanMRApproveOpensNoHold: an approve on a change with no thread is unchanged. It opens
// nothing and still fails on the release it cannot make.
func TestHumanMRApproveOpensNoHold(t *testing.T) {
	f := humanMRFake()
	errBuf := setupGitLabForge(t, f)

	ok := writeBody(t, "ok.md", okReviewBody)
	if code := run(reviewArgs(glReviewRepo, "1", "approve", testHead, ok)); code == deskkit.ExitOK {
		t.Fatalf("approve on a change with no hold thread exited 0, want a failure\nstderr: %s", errBuf.String())
	}
	if f.opens != 0 {
		t.Fatalf("the approve opened %d hold thread(s), want 0", f.opens)
	}
	if len(f.postedReview) != 1 {
		t.Fatalf("posted %d review(s), want 1", len(f.postedReview))
	}
}

// TestRoleAppMRHoldUnchanged: a change a role App opened carries its thread from creation, and
// on it every verdict's hold step is what it was: re-arm or release, never an open.
func TestRoleAppMRHoldUnchanged(t *testing.T) {
	const worker = "assay-worker-app[bot]"
	for _, tc := range []struct {
		name      string
		verdict   string
		holdState string
		holdHead  string
		want      []deskkit.MergeHoldUpdate
	}{
		{name: "request-changes, hold up", verdict: "request-changes", holdState: deskkit.MergeHoldUnresolved,
			want: []deskkit.MergeHoldUpdate{{Reason: "request-changes"}}},
		{name: "request-changes, hold released at an older head", verdict: "request-changes",
			holdState: deskkit.MergeHoldResolved, holdHead: testOldHead,
			want: []deskkit.MergeHoldUpdate{{Reason: "new head " + testHead}, {Reason: "request-changes"}}},
		{name: "approve, hold up", verdict: "approve", holdState: deskkit.MergeHoldUnresolved,
			want: []deskkit.MergeHoldUpdate{{Resolved: true, Head: testHead}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newGLHoldFake()
			f.prAuthor = deskkit.Account{Login: worker, ID: 300000006}
			f.commitLogin = worker
			f.holdState, f.holdHead = tc.holdState, tc.holdHead
			if tc.holdState == deskkit.MergeHoldResolved {
				f.holdResolvedBy = rerunReviewer(t)
			}
			errBuf := setupGitLabForge(t, f)
			body := okReviewBody
			if tc.verdict == "request-changes" {
				body = rcReviewBody
			}
			bf := writeBody(t, "rev.md", body)
			if code := run(reviewArgs(glReviewRepo, "1", tc.verdict, testHead, bf)); code != deskkit.ExitOK {
				t.Fatalf("exit = %d, want 0\nstderr: %s", code, errBuf.String())
			}
			if f.opens != 0 {
				t.Fatalf("opened %d hold thread(s) on a change that has one", f.opens)
			}
			if len(f.setHoldCalls) != len(tc.want) {
				t.Fatalf("hold writes = %+v, want %+v", f.setHoldCalls, tc.want)
			}
			for i, w := range tc.want {
				if f.setHoldCalls[i] != w {
					t.Errorf("hold write %d = %+v, want %+v", i, f.setHoldCalls[i], w)
				}
			}
			if strings.Contains(errBuf.String(), "no merge-hold thread") {
				t.Errorf("stderr reports an opened hold on a change that had one:\n%s", errBuf.String())
			}
		})
	}
}

// regression: #5898
// TestHumanMRFinishHolds: `finish review` end to end, the path the defect was seen on. Before
// the fix it stopped at step 1 with the claim held and the gate released. Now the
// request-changes holds the change before the claim is released.
func TestHumanMRFinishHolds(t *testing.T) {
	f := humanMRFake()
	errBuf := setupGitLabForge(t, f)

	var out strings.Builder
	savedOut := stdout
	stdout = &out
	t.Cleanup(func() { stdout = savedOut })

	const key = "gl-repo--pr-1"
	holdAtRelease := ""
	savedRelease := releaseReviewClaimFn
	releaseReviewClaimFn = func(_, _, _ string) (string, error) {
		holdAtRelease = f.holdState
		return finishClaimReleased, nil
	}
	t.Cleanup(func() { releaseReviewClaimFn = savedRelease })

	bf := writeBody(t, "rev.md", rcReviewBody)
	args := []string{"finish", "review", glReviewRepo, "1", "--verdict", "request-changes", "--head", testHead, "--body-file", bf, "--claim", key}
	if code := run(args); code != deskkit.ExitOK {
		t.Fatalf("finish exit = %d, want 0\nstderr: %s", code, errBuf.String())
	}
	if holdAtRelease != deskkit.MergeHoldUnresolved {
		t.Fatalf("the claim was released with the hold %q, want it held first", holdAtRelease)
	}
	if got := out.String(); !strings.Contains(got, "state=CHANGES_REQUESTED") || !strings.Contains(got, "claim=released") {
		t.Fatalf("result line = %q", got)
	}
}
