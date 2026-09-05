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

// CASE cheap (NEGATIVE PATH, Verify row 2): an attested below-tier dispatch is REFUSED, the
// refusal names the floor and carries the remediation, and NOTHING is mutated.
func TestModelFloorCheapStampRefusedWithRemediation(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = cheapStamp(t)

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

// CASE override (Verify row 4): the env override proceeds past the floor on the SAME cheap
// stamp that would otherwise refuse, and the bypass is logged with the grep-able marker.
func TestModelFloorOverrideProceedsLoudly(t *testing.T) {
	s := newStub()
	s.reviews = nil
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	s.labelEvents = cheapStamp(t)
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
