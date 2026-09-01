package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskDispatcherLogin is the roster's desk-App login — the ONLY applier whose dispatched-*
// stamp the floor trusts. Read from the fixture roster, never a literal.
func deskDispatcherLogin(t *testing.T) string {
	t.Helper()
	login, ok := deskkit.RoleAppLogin("desk")
	if !ok {
		t.Fatal("the fixture roster does not bind the desk (dispatcher) role")
	}
	return login
}

func strongStampBy(applier string) []deskkit.LabelEvent {
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "opus-4.8", AppliedBy: applier},
		{Name: deskkit.DispatchedTierPrefix + "strong", AppliedBy: applier},
	}
}

func cheapStampBy(applier string) []deskkit.LabelEvent {
	return []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "haiku-3", AppliedBy: applier},
		{Name: deskkit.DispatchedTierPrefix + "any", AppliedBy: applier},
	}
}

// The model-capability floor's four named cases, wired through the review-verdict write.

// CASE strong: an attested strong-tier dispatch clears the floor and the verdict posts.
func TestModelFloorReviewStrongStampPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.labelEvents = strongStampBy(deskDispatcherLogin(t))
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("strong-tier verdict exit = %d, want 0", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a strong-tier verdict must post", f.postedReview)
	}
}

// CASE cheap (NEGATIVE PATH): an attested below-tier dispatch is REFUSED with remediation,
// and NOTHING is posted.
func TestModelFloorReviewCheapStampRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	f.labelEvents = cheapStampBy(deskDispatcherLogin(t))
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("attested-cheap verdict exit = %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatal("a below-tier session posted a verdict")
	}
	if !strings.Contains(errBuf.String(), "strong-tier session") || !strings.Contains(errBuf.String(), "delegation downward") {
		t.Fatalf("refusal lacks the remediation:\n%s", errBuf.String())
	}
}

// CASE absent: an UNATTESTED PR is not bricked — it proceeds with a NOTICE.
func TestModelFloorReviewAbsentProceedsWithNotice(t *testing.T) {
	f, errBuf := setupFake(t)
	// labelEvents left nil: no attestation.
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("unattested verdict exit = %d, want 0 (absent is not a refusal)", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — an unattested lane must not be bricked", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), "NOTICE") {
		t.Fatalf("an unattested PR produced no NOTICE:\n%s", errBuf.String())
	}
}

// CASE override: the env override proceeds past the floor on the SAME cheap stamp that would
// otherwise refuse, and the bypass carries the loud grep-able marker.
func TestModelFloorReviewOverrideProceedsLoudly(t *testing.T) {
	f, errBuf := setupFake(t)
	f.labelEvents = cheapStampBy(deskDispatcherLogin(t))
	t.Setenv(deskkit.ModelFloorOverrideEnv, "1")
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("override verdict exit = %d, want 0 (override proceeds)", code)
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — override must let the verdict through", f.postedReview)
	}
	if !strings.Contains(errBuf.String(), deskkit.ModelFloorOverrideMarker) {
		t.Fatalf("override left no loud marker %q:\n%s", deskkit.ModelFloorOverrideMarker, errBuf.String())
	}
}

// A self-applied strong stamp — applied by a NON-dispatcher — must NOT clear the floor. This
// is the fail-closed core: a stamp anyone can self-apply is worthless.
func TestModelFloorReviewSelfAppliedStampRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.labelEvents = strongStampBy("shared-agent") // not the dispatcher
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("self-applied strong stamp exit = %d, want %d — attestation collapsed to self-report", code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatal("a self-applied stamp posted a verdict")
	}
}

// A timeline that cannot be READ is could-not-check, never a cleared floor: the verdict
// refuses rather than posting blind.
func TestModelFloorReviewUnreadableTimelineRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.timelineErr = true
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code == 0 {
		t.Fatal("an unreadable timeline proceeded — could-not-check must never post a verdict")
	}
	if f.postedReview != 0 {
		t.Fatal("a verdict posted on an unverifiable tier read")
	}
}
