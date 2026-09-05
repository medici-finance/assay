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

// The model-capability floor's four named cases, wired through the verb (Verify row 1 +
// rows 2/3/4). Each uses the fully-flippable greenPR so the ONLY variable is the dispatch
// attestation on the PR's label timeline.

// CASE strong: an attested strong-tier dispatch clears the floor and the PR flips.
func TestModelFloorStrongStampFlips(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.stamp(strongStamp(t)...)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("strong-tier flip rc = %d, want 0", rc)
	}
	if !s.ran("pr ready") {
		t.Error("a strong-tier dispatch did not reach the ready mutation")
	}
}

// CASE cheap (NEGATIVE PATH, Verify row 2): an attested below-tier dispatch is REFUSED, the
// refusal names the floor and carries the remediation, and NOTHING is mutated.
func TestModelFloorCheapStampRefusedWithRemediation(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.stamp(cheapStamp(t)...)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("attested-cheap rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a below-tier session mutated the PR: %v", m)
	}
	if !strings.Contains(out, condModelFloor) {
		t.Errorf("refusal does not name the %s condition:\n%s", condModelFloor, out)
	}
	if !strings.Contains(out, "strong-tier session") || !strings.Contains(out, "delegation downward") {
		t.Errorf("refusal lacks the remediation (escalate + delegate-down):\n%s", out)
	}
}

// CASE absent (Verify row 3): an UNATTESTED PR is not bricked — it proceeds with a NOTICE.
// Uses --dry-run so the whole condition chain runs and the NOTICE is observable at exit 0.
func TestModelFloorAbsentProceedsWithNotice(t *testing.T) {
	s := &stub{pr: greenPR()}
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

// CASE override (Verify row 4): the env override proceeds past the floor on the SAME cheap
// stamp that would otherwise refuse, and the bypass is logged with the grep-able marker.
func TestModelFloorOverrideProceedsLoudly(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.stamp(cheapStamp(t)...)
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
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.stamp(
		deskkit.LabelEvent{Name: deskkit.DispatchedModelPrefix + "opus-4.8", AppliedBy: "shared-agent"}, // not the dispatcher
		deskkit.LabelEvent{Name: deskkit.DispatchedTierPrefix + "strong", AppliedBy: "shared-agent"},
	)

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
	s := &stub{pr: greenPR(), timelineErr: true}
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
// identity — and a floor that read any historical `labeled` event made even that repair
// invisible, refusing the PR forever with no move left to anyone. Observed on a live public
// PR: the foreign stamp was removed and re-applied by the bound dispatcher App, and the flip
// still refused naming the ORIGINAL applier.
func TestModelFloorRestampedByDispatcherFlips(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.stamp(
		deskkit.LabelEvent{Name: model, AppliedBy: "jojig-dao"}, // the foreign stamp
		deskkit.LabelEvent{Name: tier, AppliedBy: "jojig-dao"},
		deskkit.LabelEvent{Name: model, AppliedBy: d, Removed: true}, // the dispatcher un-stamps
		deskkit.LabelEvent{Name: tier, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: model, AppliedBy: d}, // and re-stamps as itself
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
	)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("re-stamped PR rc = %d, want 0 — an append-only timeline leaves no other repair:\n%s", rc, out)
	}
	if !s.ran("pr ready") {
		t.Errorf("a dispatcher re-stamp did not reach the ready mutation:\n%s", out)
	}
}

// A foreign stamp that is STILL STANDING is not laundered by the fix: only a genuine
// re-stamp is honoured, and this row is what tells the two apart.
func TestModelFloorForeignStampStillStandingRefused(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.stamp(
		deskkit.LabelEvent{Name: model, AppliedBy: d}, // the dispatcher stamped first...
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
		deskkit.LabelEvent{Name: model, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: tier, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: model, AppliedBy: "jojig-dao"}, // ...but a foreign login holds it now
		deskkit.LabelEvent{Name: tier, AppliedBy: "jojig-dao"},
	)

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("standing foreign stamp rc = %d, want %d — an earlier dispatcher event must not vouch "+
			"for a later foreign one", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(out, "jojig-dao") {
		t.Errorf("the refusal does not name the STANDING applier:\n%s", out)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a foreign-stamped PR was mutated: %v", m)
	}
}

// The repair usually lands LATE in a busy PR's timeline — i.e. on a later page. `gh api
// --paginate` emits each page as its own top-level JSON array, so a reader that parsed only
// the first array would see the foreign stamp and never the re-stamp that fixed it.
func TestModelFloorReadsTimelineBeyondTheFirstPage(t *testing.T) {
	s := &stub{pr: greenPR()}
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	d := dispatcherLogin(t)
	model := deskkit.DispatchedModelPrefix + "opus-4.8"
	tier := deskkit.DispatchedTierPrefix + "strong"
	s.stamp(
		deskkit.LabelEvent{Name: model, AppliedBy: "jojig-dao"},
		deskkit.LabelEvent{Name: tier, AppliedBy: "jojig-dao"},
		deskkit.LabelEvent{Name: model, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: tier, AppliedBy: d, Removed: true},
		deskkit.LabelEvent{Name: model, AppliedBy: d},
		deskkit.LabelEvent{Name: tier, AppliedBy: d},
	)
	s.timelineSplit = 2 // the repair lives entirely in the SECOND page

	var rc int
	out := captureStderr(t, func() { rc = run([]string{"7", "--repo", privateCIRepo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("paged timeline rc = %d, want 0 — page 2 of the timeline was not read:\n%s", rc, out)
	}
}
