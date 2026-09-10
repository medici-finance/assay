package main

// queuelabel_test.go — #795 §4: a review dispatch applies the review-lane queue label
// `authorization-needed` to the picked-up change, forge-neutrally (the resolved forge's
// idempotent label ensure+apply under the reviewer role's own credential), and does so
// NON-FATALLY — a forge/credential failure is a loud warning, never a failed dispatch. The
// step is a documented no-op for a non-review dispatch and for a review dispatch with no --pr.
//
// The forge write is a SEAM (applyQueueLabelFn) so these drive the step with no real forge
// credential or network — the same discipline mintTokenFn already gives the model stamp.

import (
	"errors"
	"strings"
	"testing"
)

// queueLabelCall records one apply request the step made through the seam.
type queueLabelCall struct {
	repo string
	pr   int
}

// stubQueueLabel replaces the forge-write seam and records what it was asked to apply. added
// reports the labels the fake forge "created" (nil => already present); err simulates a
// forge/credential failure so the non-fatal contract can be asserted.
func stubQueueLabel(t *testing.T, err error, added ...string) *[]queueLabelCall {
	t.Helper()
	var calls []queueLabelCall
	old := applyQueueLabelFn
	applyQueueLabelFn = func(repo string, pr int) ([]string, string, error) {
		calls = append(calls, queueLabelCall{repo: repo, pr: pr})
		if err != nil {
			return nil, "github", err
		}
		return added, "github", nil
	}
	t.Cleanup(func() { applyQueueLabelFn = old })
	return &calls
}

func TestQueueLabel_SkippedForNonReviewDispatch(t *testing.T) {
	calls := stubQueueLabel(t, nil, queueLabelAuthorizationNeeded)
	for _, kit := range []string{"", "worker", "verifier"} {
		got := stepQueueLabelApply(dispatchOpts{kit: kit, pr: 77}, "medici-finance/assay")
		if !strings.HasPrefix(got, "SKIPPED:") {
			t.Errorf("kit %q: got %q, want a SKIPPED line (queue label is review-lane only)", kit, got)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("a non-review dispatch must not touch the forge; got %d apply call(s)", len(*calls))
	}
}

func TestQueueLabel_DeferredWhenNoPR(t *testing.T) {
	calls := stubQueueLabel(t, nil, queueLabelAuthorizationNeeded)
	got := stepQueueLabelApply(dispatchOpts{kit: "review", pr: 0}, "medici-finance/assay")
	if !strings.HasPrefix(got, "DEFERRED:") {
		t.Errorf("got %q, want a DEFERRED line (no --pr, nothing to label yet)", got)
	}
	if len(*calls) != 0 {
		t.Errorf("with no --pr there is no change to label; got %d apply call(s)", len(*calls))
	}
}

func TestQueueLabel_AppliedOnReviewPickup(t *testing.T) {
	calls := stubQueueLabel(t, nil, queueLabelAuthorizationNeeded)
	got := stepQueueLabelApply(dispatchOpts{kit: "review", pr: 77}, "medici-finance/assay")
	if !strings.HasPrefix(got, "OK: applied "+queueLabelAuthorizationNeeded) {
		t.Errorf("got %q, want an OK line naming the applied label", got)
	}
	if len(*calls) != 1 {
		t.Fatalf("want exactly one forge apply call, got %d", len(*calls))
	}
	if c := (*calls)[0]; c.repo != "medici-finance/assay" || c.pr != 77 {
		t.Errorf("apply call = %+v, want {medici-finance/assay 77}", c)
	}
}

// Idempotence: when the label is already present the forge adds nothing, and the step says so
// rather than reporting a fresh application — the create+apply is a reconcile, not an append.
func TestQueueLabel_AlreadyPresentIsIdempotentOK(t *testing.T) {
	stubQueueLabel(t, nil) // added == nil: the label already exists on the change
	got := stepQueueLabelApply(dispatchOpts{kit: "review", pr: 77}, "medici-finance/assay")
	if !strings.HasPrefix(got, "OK: "+queueLabelAuthorizationNeeded+" already present") {
		t.Errorf("got %q, want an idempotent already-present OK line", got)
	}
}

// The label is a legibility aid, not a correctness gate: a forge/credential failure is a loud
// WARNING and the dispatch continues. stepQueueLabelApply must NEVER surface this as an error
// (it returns a string), so a GitLab adopter with the label absent still gets its reviewer
// dispatched — the same contract as stepRoster and deskflip's ensureLabelSwap.
func TestQueueLabel_ForgeFailureIsNonFatalWarning(t *testing.T) {
	stubQueueLabel(t, errors.New("no gitlab-reviewer.token in the search path"))
	got := stepQueueLabelApply(dispatchOpts{kit: "review", pr: 77}, "medici-finance/assay")
	if !strings.HasPrefix(got, "WARNING:") {
		t.Errorf("got %q, want a WARNING line (a label failure never fails the dispatch)", got)
	}
	if !strings.Contains(got, "the review dispatch stands") {
		t.Errorf("the warning must make clear the dispatch is not failed; got %q", got)
	}
}
