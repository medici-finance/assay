package main

import (
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// captureStderr runs fn with os.Stderr redirected to a temp file and returns what was
// written. deskflip prints its per-condition lines — including the floor's NOTICE and the
// loud override line — to os.Stderr, so an assertion on those needs the capture.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	f, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = f
	defer func() { os.Stderr = old }()
	fn()
	os.Stderr = old
	_ = f.Close()
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// foreignApplier is a login the fixture roster does NOT bind to the dispatcher role, so a
// dispatched-* stamp it applies is one the floor must not trust.
const foreignApplier = "not-the-dispatcher"

// The model-capability floor's four named cases, wired through the verb (Verify row 1 +
// rows 2/3/4). Each uses the fully-flippable greenPR so the ONLY variable is the dispatch
// attestation on the PR's label timeline.

// CASE strong: an attested strong-tier dispatch clears the floor and the PR flips.
func TestModelFloorStrongStampFlips(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = strongStamp(t)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("strong-tier flip rc = %d, want 0", rc)
	}
	if !s.flipped() {
		t.Error("a strong-tier dispatch did not reach the ready mutation")
	}
}

// CASE `any`: a dispatcher-applied stamp whose tier is `any` asserts NO strength, so it
// takes the floor's NOTICE path — the same outcome as an unstamped PR — and the NOTICE names
// the label so the operator can tell the two apart. Wired through the verb, because a floor
// decision that never reaches the verb's output is a decision nobody can act on.
//
// `any` is the brief schema's "no particular runner demanded"; reading it as an attested
// below-floor dispatch refused every write on a PR dispatched from an unremarkable brief,
// which is not the population the floor exists to catch. Uses --dry-run so the whole
// condition chain runs and the NOTICE is observable at exit 0.
func TestModelFloorTierAnyProceedsWithNotice(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = noClaimStamp(t)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo, "--dry-run"}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("tier-any dry-run rc = %d, want 0 (`any` is not a strength claim):\n%s", rc, out)
	}
	if !strings.Contains(out, condModelFloor) {
		t.Errorf("the floor decision does not name the %s condition:\n%s", condModelFloor, out)
	}
	if !strings.Contains(out, "NOTICE") {
		t.Errorf("a tier-any PR produced no NOTICE line:\n%s", out)
	}
	if !strings.Contains(out, deskkit.DispatchedTierPrefix+"any") {
		t.Errorf("the NOTICE does not name the label it read:\n%s", out)
	}
}

// CASE absent (Verify row 3): an UNATTESTED PR is not bricked — it proceeds with a NOTICE.
// Uses --dry-run so the whole condition chain runs and the NOTICE is observable at exit 0.
func TestModelFloorAbsentProceedsWithNotice(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	// labelEvents left nil: no dispatch attestation on the PR.

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo, "--dry-run"}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("unattested dry-run rc = %d, want 0 (absent is not a refusal)", rc)
	}
	if !strings.Contains(out, "NOTICE") {
		t.Errorf("an unattested PR produced no NOTICE line:\n%s", out)
	}
}

// CASE override (Verify row 4): the env override short-circuits the floor before the stamp
// state is examined at all, and the bypass is logged with the grep-able marker.
func TestModelFloorOverrideProceedsLoudly(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = noClaimStamp(t)
	t.Setenv(deskkit.ModelFloorOverrideEnv, "1")

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo, "--dry-run"}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("override dry-run rc = %d, want 0 (override proceeds)", rc)
	}
	if !strings.Contains(out, deskkit.ModelFloorOverrideMarker) {
		t.Errorf("the override left no loud grep-able marker %q:\n%s", deskkit.ModelFloorOverrideMarker, out)
	}
}

// A self-applied strong stamp — applied by a NON-dispatcher — must not clear the floor: the
// whole point of the attestation is that a stamp anyone can self-apply is worthless. This is
// the fail-closed core of the security surface.
func TestModelFloorSelfAppliedStampRefused(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = []deskkit.LabelEvent{
		{Name: deskkit.DispatchedModelPrefix + "opus-4.8", AppliedBy: "shared-agent"}, // not the dispatcher
		{Name: deskkit.DispatchedTierPrefix + "strong", AppliedBy: "shared-agent"},
	}

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("self-applied strong stamp rc = %d, want %d — attestation collapsed to self-report", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a self-applied stamp mutated the PR: %v", m)
	}
}

// A timeline that cannot be READ is could-not-check, never a cleared floor: the flip refuses
// UNVERIFIABLE rather than proceeding blind.
func TestModelFloorTimelineUnreadableIsUnverifiable(t *testing.T) {
	s := newStub()
	s.timelineErr = true
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("unreadable timeline rc = %d, want %d (could-not-check is never green)", rc, deskkit.ExitUnverifiable)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("mutation on an unverifiable tier read: %v", m)
	}
}

// THE REPAIR PATH, end to end through the verb. A GitHub timeline is APPEND-ONLY: the
// `labeled` event that recorded a foreign stamp is never removed. So the ONLY repair
// available is for the dispatcher to REMOVE the labels and re-apply them under its own
// identity. The applier-aware floor reads the STANDING applier of each label — its last
// `labeled` event — so a genuine re-stamp lands as the standing applier and the PR flips,
// while an earlier foreign `labeled` event no longer bricks it forever. (The resolver drops
// the `unlabeled` events; the re-stamp's own `labeled` events are what carry the repair.)
func TestModelFloorRestampedByDispatcherFlips(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.labelEvents = []deskkit.LabelEvent{
		{Name: model, AppliedBy: foreignApplier}, // the foreign stamp
		{Name: tier, AppliedBy: foreignApplier},
		{Name: model, AppliedBy: d, Removed: true}, // the dispatcher un-stamps
		{Name: tier, AppliedBy: d, Removed: true},
		{Name: model, AppliedBy: d}, // and re-stamps as itself
		{Name: tier, AppliedBy: d},
	}

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("re-stamped PR rc = %d, want 0 — an append-only timeline leaves no other repair:\n%s", rc, out)
	}
	if !s.flipped() {
		t.Errorf("a dispatcher re-stamp did not reach the ready mutation:\n%s", out)
	}
}

// A foreign stamp that is STILL STANDING is not laundered by the repair: only a genuine
// re-stamp is honoured. Here the dispatcher stamped first, then a foreign login re-applied —
// so the STANDING applier is foreign, and the flip must refuse naming it.
func TestModelFloorForeignStampStillStandingRefused(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.labelEvents = []deskkit.LabelEvent{
		{Name: model, AppliedBy: d}, // the dispatcher stamped first...
		{Name: tier, AppliedBy: d},
		{Name: model, AppliedBy: d, Removed: true},
		{Name: tier, AppliedBy: d, Removed: true},
		{Name: model, AppliedBy: foreignApplier}, // ...but a foreign login holds it now
		{Name: tier, AppliedBy: foreignApplier},
	}

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("standing foreign stamp rc = %d, want %d — an earlier dispatcher event must not vouch "+
			"for a later foreign one:\n%s", rc, deskkit.ExitRefused, out)
	}
	if !strings.Contains(out, foreignApplier) {
		t.Errorf("the refusal does not name the STANDING applier:\n%s", out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a foreign-stamped PR was mutated: %v", m)
	}
}

// The repair usually lands LATE in a busy PR's timeline — i.e. on a later page. The resolver
// walks the timeline page by page, so a reader that stopped after page one would see only the
// foreign stamp and never the re-stamp that fixed it. Page one is filled to the page boundary
// with quiet unrelated label events (the foreign stamp among them); the dispatcher re-stamp
// lives entirely on page two, so a flip proves the second page was read.
func TestModelFloorReadsTimelineBeyondTheFirstPage(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"

	// Page one (exactly timelinePageSize events, so the resolver must fetch a second page):
	// the standing foreign stamp plus quiet filler label events that the floor ignores.
	events := []deskkit.LabelEvent{
		{Name: model, AppliedBy: foreignApplier},
		{Name: tier, AppliedBy: foreignApplier},
	}
	for i := len(events); i < timelinePageSize; i++ {
		events = append(events, deskkit.LabelEvent{Name: "queue:example", AppliedBy: "someone"})
	}
	// Page two: the dispatcher un-stamps the foreign labels and re-applies them as itself.
	// Only these events turn the refusal into a flip, and they exist only past page one.
	events = append(events,
		deskkit.LabelEvent{Name: model, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: tier, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: model, AppliedBy: d},
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
	)
	s.labelEvents = events

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("paged timeline rc = %d, want 0 — page 2 of the timeline was not read:\n%s", rc, out)
	}
	if !s.flipped() {
		t.Errorf("the re-stamp on page 2 did not reach the ready mutation:\n%s", out)
	}
}
