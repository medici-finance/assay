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

// removedBy is the `unlabeled` half of a re-stamp: the login that took the label OFF.
func removedBy(name, applier string) deskkit.LabelEvent {
	return deskkit.LabelEvent{Name: name, AppliedBy: applier, Removed: true}
}

// stamp puts a dispatch stamp on the fake PR: the timeline EVENTS that record who applied
// (or removed) each half, AND the labels the PR actually CARRIES afterwards. The floor reads
// BOTH — presence from the labels API, the applier from the events — so a fixture that set
// only one of them would describe a state GitHub cannot be in.
func (f *fakeGH) stamp(events ...deskkit.LabelEvent) {
	f.labelEvents = events
	live := map[string]bool{}
	var order []string
	for _, e := range events {
		if _, seen := live[e.Name]; !seen {
			order = append(order, e.Name)
		}
		live[e.Name] = !e.Removed
	}
	kept := f.prLabels[:0:0]
	for _, l := range f.prLabels {
		n := strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(n, deskkit.DispatchedModelPrefix) || strings.HasPrefix(n, deskkit.DispatchedTierPrefix) {
			continue
		}
		kept = append(kept, l)
	}
	for _, n := range order {
		if live[n] {
			kept = append(kept, n)
		}
	}
	f.prLabels = kept
}

// The model-capability floor's four named cases, wired through the review-verdict write.

// CASE strong: an attested strong-tier dispatch clears the floor and the verdict posts.
func TestModelFloorReviewStrongStampPosts(t *testing.T) {
	f, _ := setupFake(t)
	f.stamp(strongStampBy(deskDispatcherLogin(t))...)
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
	f.stamp(cheapStampBy(deskDispatcherLogin(t))...)
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
	f.stamp(cheapStampBy(deskDispatcherLogin(t))...)
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
	f.stamp(strongStampBy("shared-agent")...) // not the dispatcher
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

// The SECOND flip verb. `deskpost ready` is the App-identity ready-flip — the other live,
// sanctioned verb that performs the identical markPullRequestReadyForReview mutation. It
// MUST clear the same floor as deskflip, or a below-tier session simply flips through this
// verb instead. The greenStatus + APPROVED fixture is TestReadySuccess's, so the ONLY
// variable is the dispatch attestation.

// CASE strong: an attested strong-tier dispatch clears the floor and the PR flips.
func TestModelFloorReadyStrongStampFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.stamp(strongStampBy(deskDispatcherLogin(t))...)

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("strong-tier ready exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — a strong-tier dispatch must flip", f.flips)
	}
}

// CASE cheap (NEGATIVE PATH): an attested below-tier dispatch is REFUSED at the App-identity
// flip verb too, with remediation, and NOTHING is flipped.
func TestModelFloorReadyCheapStampRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.stamp(cheapStampBy(deskDispatcherLogin(t))...)

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("attested-cheap ready exit = %d, want %d (refused) — the flip floor is bypassable via deskpost ready", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatal("a below-tier session flipped a PR through deskpost ready")
	}
	if !strings.Contains(errBuf.String(), "strong-tier session") || !strings.Contains(errBuf.String(), "delegation downward") {
		t.Fatalf("ready refusal lacks the remediation:\n%s", errBuf.String())
	}
}

// CASE absent: an UNATTESTED PR is not bricked at the ready verb — it proceeds with a NOTICE.
func TestModelFloorReadyAbsentProceedsWithNotice(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	// labelEvents left nil.

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("unattested ready exit = %d, want 0 (absent is not a refusal)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — an unattested lane must not be bricked", f.flips)
	}
	if !strings.Contains(errBuf.String(), "NOTICE") {
		t.Fatalf("an unattested ready produced no NOTICE:\n%s", errBuf.String())
	}
}

// CASE override: the env override proceeds past the ready floor on the SAME cheap stamp that
// would otherwise refuse, and the bypass carries the loud grep-able marker.
func TestModelFloorReadyOverrideProceedsLoudly(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.stamp(cheapStampBy(deskDispatcherLogin(t))...)
	t.Setenv(deskkit.ModelFloorOverrideEnv, "1")

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("override ready exit = %d, want 0 (override proceeds)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — override must let the flip through", f.flips)
	}
	if !strings.Contains(errBuf.String(), deskkit.ModelFloorOverrideMarker) {
		t.Fatalf("ready override left no loud marker %q:\n%s", deskkit.ModelFloorOverrideMarker, errBuf.String())
	}
}

// A self-applied strong stamp must NOT clear the ready floor either — the fail-closed core
// applies identically to both flip verbs.
func TestModelFloorReadySelfAppliedStampRefused(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.stamp(strongStampBy("shared-agent")...) // not the dispatcher

	if code := run(readyArgs(exampleRepo)); code != deskkit.ExitRefused {
		t.Fatalf("self-applied strong stamp ready exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatal("a self-applied stamp flipped a PR through deskpost ready")
	}
}

// THE REPAIR PATH, end to end through the App-identity verbs. A GitHub timeline is
// APPEND-ONLY, so the `labeled` event recording a foreign stamp never goes away; the only
// repair available is for the dispatcher to REMOVE the labels and re-apply them as itself.
// A floor that read any historical `labeled` event made that repair invisible and refused
// the PR forever — observed live, where the re-stamp by the bound dispatcher App changed
// nothing and the refusal still named the original applier.
func TestModelFloorReviewRestampedByDispatcherPosts(t *testing.T) {
	f, errBuf := setupFake(t)
	d := deskDispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	f.stamp(
		deskkit.LabelEvent{Name: model, AppliedBy: "jojig-dao"}, // the foreign stamp
		deskkit.LabelEvent{Name: tier, AppliedBy: "jojig-dao"},
		removedBy(model, d), // the dispatcher un-stamps
		removedBy(tier, d),
		deskkit.LabelEvent{Name: model, AppliedBy: d}, // and re-stamps as itself
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
	)
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != 0 {
		t.Fatalf("re-stamped verdict exit = %d, want 0 — an append-only timeline leaves no other "+
			"repair:\n%s", code, errBuf.String())
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1 — a dispatcher re-stamp must clear the floor", f.postedReview)
	}
}

// The same repair at the ready-flip verb: both authority-bearing writes read one floor, so
// both must honour the same repair.
func TestModelFloorReadyRestampedByDispatcherFlips(t *testing.T) {
	f, errBuf := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	d := deskDispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	f.stamp(
		deskkit.LabelEvent{Name: model, AppliedBy: "jojig-dao"},
		deskkit.LabelEvent{Name: tier, AppliedBy: "jojig-dao"},
		removedBy(model, d),
		removedBy(tier, d),
		deskkit.LabelEvent{Name: model, AppliedBy: d},
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
	)

	if code := run(readyArgs(exampleRepo)); code != 0 {
		t.Fatalf("re-stamped ready exit = %d, want 0:\n%s", code, errBuf.String())
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — a dispatcher re-stamp must clear the ready floor", f.flips)
	}
}

// A re-stamp by a FOREIGN login is not a repair, and neither is an earlier dispatcher event:
// what counts is who holds the STANDING application. Both directions fail closed.
func TestModelFloorReviewStandingApplierDecides(t *testing.T) {
	d := ""
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	cases := []struct {
		why    string
		events func(disp string) []deskkit.LabelEvent
	}{
		{
			why: "removed then re-applied by a foreign login",
			events: func(disp string) []deskkit.LabelEvent {
				return []deskkit.LabelEvent{
					{Name: model, AppliedBy: "jojig-dao"},
					{Name: tier, AppliedBy: "jojig-dao"},
					removedBy(model, disp), removedBy(tier, disp),
					{Name: model, AppliedBy: "jojig-dao"},
					{Name: tier, AppliedBy: "jojig-dao"},
				}
			},
		},
		{
			why: "dispatcher stamp overwritten by a foreign login",
			events: func(disp string) []deskkit.LabelEvent {
				return []deskkit.LabelEvent{
					{Name: model, AppliedBy: disp},
					{Name: tier, AppliedBy: disp},
					removedBy(model, "jojig-dao"), removedBy(tier, "jojig-dao"),
					{Name: model, AppliedBy: "jojig-dao"},
					{Name: tier, AppliedBy: "jojig-dao"},
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			f, errBuf := setupFake(t)
			d = deskDispatcherLogin(t)
			f.stamp(c.events(d)...)
			bf := writeBody(t, "rev.md", okReviewBody)

			if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != deskkit.ExitRefused {
				t.Fatalf("exit = %d, want %d — the STANDING application is foreign", code, deskkit.ExitRefused)
			}
			if f.postedReview != 0 {
				t.Fatal("a foreign standing stamp posted a verdict")
			}
			if !strings.Contains(errBuf.String(), "jojig-dao") {
				t.Errorf("the refusal does not name the standing applier:\n%s", errBuf.String())
			}
		})
	}
}

// A label the PR CARRIES that the timeline read cannot attribute is could-not-check, NEVER
// unstamped. Absent is the one state that proceeds, so a short timeline read must not be
// able to reach it — that direction is the difference between a fail-closed floor and a
// floor a truncated read waves past.
func TestModelFloorReviewPresentStampWithNoEventRefuses(t *testing.T) {
	f, errBuf := setupFake(t)
	f.prLabels = []string{
		deskkit.DispatchedModelPrefix + "opus-4.8",
		deskkit.DispatchedTierPrefix + "strong",
	}
	// labelEvents deliberately left nil: the labels are there, the events are not.
	bf := writeBody(t, "rev.md", okReviewBody)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, bf)); code != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d — a present-but-unattributable stamp is could-not-check, not absent",
			code, deskkit.ExitRefused)
	}
	if f.postedReview != 0 {
		t.Fatal("an unattributable stamp posted a verdict")
	}
	if strings.Contains(errBuf.String(), "NOTICE") {
		t.Errorf("a present stamp was reported as ABSENT:\n%s", errBuf.String())
	}
}

// The repair usually lands LATE on a busy PR — beyond the first timeline page. The client
// walks 100 events per page, so a stamp on page 2 is only seen by a reader that keeps
// walking; one that stopped at page 1 would find the labels unattributable and refuse.
func TestModelFloorReviewReadsTimelineBeyondTheFirstPage(t *testing.T) {
	f, errBuf := setupFake(t)
	d := deskDispatcherLogin(t)
	var events []deskkit.LabelEvent
	for i := 0; i < 100; i++ { // a full first page of unrelated label churn
		events = append(events, deskkit.LabelEvent{Name: "size:S", AppliedBy: d})
	}
	events = append(events, strongStampBy(d)...) // the stamp itself is on page 2
	f.stamp(events...)

	if code := run(reviewArgs(exampleRepo, "1", "approve", testHead, stdReviewBody(t))); code != 0 {
		t.Fatalf("paged timeline exit = %d, want 0 — page 2 of the timeline was not read:\n%s",
			code, errBuf.String())
	}
	if f.postedReview != 1 {
		t.Fatalf("postedReview = %d, want 1", f.postedReview)
	}
}

// stdReviewBody writes the standard review body used by the paging test.
func stdReviewBody(t *testing.T) string {
	t.Helper()
	return writeBody(t, "rev.md", okReviewBody)
}
