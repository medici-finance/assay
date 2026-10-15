package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// #2760. The authoring stamp (`--stamp-only`, worker kits) and the review stamp (`--kit
// review`) used to share the dispatched-* pair, so whichever ran later replaced the other.
// The authoring stamp now writes authored-* and never touches dispatched-*; the review stamp
// writes dispatched-* and never touches authored-*. These tests spell the label names as
// literals so they also compile, and fail, against the pre-fix code.

const (
	nsReviewModel = "dispatched-model:review-model-1"
	nsReviewTier  = "dispatched-tier:strong"
	nsAuthorModel = "authored-model:gpt-6.1-sol"
	nsAuthorTier  = "authored-tier:strong"
)

func roleLogin(t *testing.T, role string) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin(role)
	if !ok {
		t.Fatalf("the fixture roster binds no App to role %q", role)
	}
	return login
}

// reviewAttested reads the dispatched-* pair the way the capability floor does.
func reviewAttested(gh *ghStampServer) (deskkit.ModelStamp, deskkit.ModelState) {
	return deskkit.AttestedModelStampOf(deskkit.StampTimeline{Present: gh.labels, Events: ghEvents(gh)},
		deskkit.IsStampAuthorityLogin)
}

// plantReviewPair puts a reviewer-applied, floor-readable dispatched-* pair on the PR.
func plantReviewPair(t *testing.T, gh *ghStampServer) {
	t.Helper()
	r := roleLogin(t, deskkit.ReviewDispatcherRole)
	gh.labels = append(gh.labels, nsReviewModel, nsReviewTier)
	gh.timeline = append(gh.timeline, labeled(nsReviewModel, r), labeled(nsReviewTier, r))
}

func assertReviewPairStands(t *testing.T, gh *ghStampServer) {
	t.Helper()
	for _, l := range []string{nsReviewModel, nsReviewTier} {
		if gh.removed(l) {
			t.Errorf("the authoring stamp removed the review session's %s: %+v", l, gh.requests)
		}
	}
	got, state := reviewAttested(gh)
	if state != deskkit.ModelStamped || got.Model != "review-model-1" || got.Tier != "strong" {
		t.Errorf("the floor no longer reads the review stamp: %+v %v (labels %v)", got, state, gh.labels)
	}
}

func assertAuthorPairApplied(t *testing.T, gh *ghStampServer) {
	t.Helper()
	if !labelsPresent(gh.labels, []string{nsAuthorModel, nsAuthorTier}) {
		t.Errorf("the authoring pair is not on the PR: %v", gh.labels)
	}
}

// regression: #2760
func TestWorkerStampKeepsReviewPair(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "the-desk")
	gh := installGHStamp(t)
	plantReviewPair(t, gh)

	if rc := run(onlyArgs("gpt-6.1-sol")); rc != deskkit.ExitOK {
		t.Fatalf("stamp-only rc=%d, want 0", rc)
	}
	assertReviewPairStands(t, gh)
	assertAuthorPairApplied(t, gh)
	if gh.applied("dispatched-") {
		t.Errorf("the authoring stamp wrote a dispatched-* label: %+v", gh.requests)
	}
}

// The same defect through the second entry: a full worker dispatch with --pr known runs the
// same stamp step and must also leave the review pair alone.
//
// regression: #2760
func TestDispatchStampKeepsReview(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(t.TempDir())
	gh := installGHStamp(t)
	plantReviewPair(t, gh)

	if rc := run(stampArgs(t, root)); rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc=%d, want 0", rc)
	}
	assertReviewPairStands(t, gh)
	if !labelsPresent(gh.labels, []string{"authored-model:example-model-1", "authored-tier:strong"}) {
		t.Errorf("the worker dispatch did not apply the authoring pair: %v", gh.labels)
	}
}

// The review stamp leaves a standing authoring pair alone.
func TestReviewStampKeepsAuthorPair(t *testing.T) {
	s := &stub{}
	s.install(t)
	t.Setenv("DESK_LOOP", "pr-review-desk")
	gh := installGHStamp(t)
	d := roleLogin(t, deskkit.DispatcherRole)
	gh.labels = []string{nsAuthorModel, nsAuthorTier}
	gh.timeline = []ghStampEvt{labeled(nsAuthorModel, d), labeled(nsAuthorTier, d)}

	if rc := run(append(onlyArgs("review-model-1"), "--kit", "review")); rc != deskkit.ExitOK {
		t.Fatalf("review stamp-only rc=%d, want 0", rc)
	}
	for _, l := range []string{nsAuthorModel, nsAuthorTier} {
		if gh.removed(l) {
			t.Errorf("the review stamp removed the authoring %s: %+v", l, gh.requests)
		}
	}
	if !labelsPresent(gh.labels, []string{nsAuthorModel, nsAuthorTier, nsReviewModel, nsReviewTier}) {
		t.Errorf("want both pairs on the PR, got %v", gh.labels)
	}
}

// The issue's sequence in both orders: author, review, then a LATE authoring re-stamp. Both
// pairs stand at the end, and the floor still reads the review session's model.
//
// regression: #2760
func TestBothStampsCoexist(t *testing.T) {
	s := &stub{}
	s.install(t)
	gh := installGHStamp(t)
	steps := []struct {
		loop string
		args []string
	}{
		{"the-desk", onlyArgs("gpt-6.1-sol")},
		{"pr-review-desk", append(onlyArgs("review-model-1"), "--kit", "review")},
		{"the-desk", onlyArgs("gpt-6.1-sol")},
	}
	for i, st := range steps {
		t.Setenv("DESK_LOOP", st.loop)
		if rc := run(st.args); rc != deskkit.ExitOK {
			t.Fatalf("step %d (%s) rc=%d, want 0", i, st.loop, rc)
		}
	}
	if !labelsPresent(gh.labels, []string{nsAuthorModel, nsAuthorTier}) {
		t.Errorf("the authoring pair did not survive the review stamp: %v", gh.labels)
	}
	got, state := reviewAttested(gh)
	if state != deskkit.ModelStamped || got.Model != "review-model-1" {
		t.Errorf("the late authoring stamp replaced the review attestation: %+v %v (labels %v)",
			got, state, gh.labels)
	}
	for _, l := range gh.labels {
		if strings.HasPrefix(l, "dispatched-model:") && l != nsReviewModel {
			t.Errorf("a second dispatched-model slug is on the PR: %v", gh.labels)
		}
	}
}

// regression: #2760
func TestStampKitKinds(t *testing.T) {
	for _, tc := range []struct {
		kit  string
		want deskkit.StampKind
	}{
		{"worker", deskkit.StampAuthored}, {"worker-objective", deskkit.StampAuthored},
		{"review", deskkit.StampDispatch}, {"verifier", deskkit.StampDispatch},
		{" ReView ", deskkit.StampDispatch}, {" Verifier ", deskkit.StampDispatch},
	} {
		if got := stampKindForKit(tc.kit); got != tc.want {
			t.Errorf("kit %q: got %v, want %v", tc.kit, got, tc.want)
		}
	}
}
