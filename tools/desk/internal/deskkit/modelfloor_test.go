package deskkit

import (
	"strings"
	"testing"
)

// dispatcherIs returns a predicate that vouches for exactly one login — the stand-in for
// IsDispatcherLogin the applier-aware reader needs, without depending on a live roster.
func dispatcherIs(login string) func(string) bool {
	return func(applier string) bool { return applier == login }
}

func modelEvent(slug, applier string) LabelEvent {
	return LabelEvent{Name: DispatchedModelPrefix + slug, AppliedBy: applier}
}

func tierEvent(tier, applier string) LabelEvent {
	return LabelEvent{Name: DispatchedTierPrefix + tier, AppliedBy: applier}
}

// The four cases the brief's Verify row 1 names — strong proceeds, cheap refuses with the
// remediation, absent proceeds with the NOTICE, override proceeds with the loud marker —
// plus the fail-closed Indeterminate case, exercised against the PURE decision so the
// wiring tests in each verb package need only prove the read + transport.
func TestModelCapabilityFloorFourCases(t *testing.T) {
	const disp = "the-dispatcher"
	strongStamp := []LabelEvent{modelEvent("opus-4.8", disp), tierEvent("strong", disp)}
	cheapStamp := []LabelEvent{modelEvent("haiku-3", disp), tierEvent("any", disp)}

	t.Run("attested strong proceeds", func(t *testing.T) {
		d := ModelCapabilityFloor(strongStamp, dispatcherIs(disp), false)
		if d.Outcome != FloorAllow || !d.Outcome.Proceeds() {
			t.Fatalf("outcome = %v, want FloorAllow", d.Outcome)
		}
		if d.State != ModelStamped || d.Stamp.Tier != "strong" {
			t.Fatalf("state/stamp = %v/%q, want stamped/strong", d.State, d.Stamp.Tier)
		}
	})

	t.Run("attested cheap refuses with remediation", func(t *testing.T) {
		d := ModelCapabilityFloor(cheapStamp, dispatcherIs(disp), false)
		if d.Outcome != FloorRefuse || d.Outcome.Proceeds() {
			t.Fatalf("outcome = %v, want FloorRefuse", d.Outcome)
		}
		// Remediation must say WHAT INSTEAD, not just refuse (pre-mortem row: a dead-end
		// refusal is a finding).
		if !strings.Contains(d.Message, "strong-tier session") {
			t.Fatalf("remediation does not name the escalation target:\n%s", d.Message)
		}
		if !strings.Contains(d.Message, "delegation downward") {
			t.Fatalf("remediation does not state the asymmetry (delegate down, not up):\n%s", d.Message)
		}
	})

	t.Run("absent proceeds with NOTICE", func(t *testing.T) {
		d := ModelCapabilityFloor(nil, dispatcherIs(disp), false)
		if d.Outcome != FloorNoticeAllow || !d.Outcome.Proceeds() {
			t.Fatalf("outcome = %v, want FloorNoticeAllow", d.Outcome)
		}
		if d.State != ModelUnknown {
			t.Fatalf("state = %v, want ModelUnknown (absent, not indeterminate)", d.State)
		}
		if !strings.Contains(d.Message, "NOTICE") {
			t.Fatalf("absent message is not a NOTICE:\n%s", d.Message)
		}
	})

	t.Run("override proceeds with the loud marker", func(t *testing.T) {
		// Override on the CHEAP stamp: it must proceed anyway, and loudly.
		d := ModelCapabilityFloor(cheapStamp, dispatcherIs(disp), true)
		if d.Outcome != FloorOverrideAllow || !d.Outcome.Proceeds() {
			t.Fatalf("outcome = %v, want FloorOverrideAllow", d.Outcome)
		}
		if !strings.Contains(d.Message, ModelFloorOverrideMarker) {
			t.Fatalf("override message carries no grep-able marker %q:\n%s", ModelFloorOverrideMarker, d.Message)
		}
	})
}

// A self-applied stamp is worthless by design: a dispatched-tier:strong label the WORKER
// applied to itself must NOT clear the floor. This is the core security property — without
// it the floor is the honor-system with extra steps.
func TestModelCapabilityFloorRefusesSelfAppliedStamp(t *testing.T) {
	const disp, worker = "the-dispatcher", "the-worker"
	selfApplied := []LabelEvent{
		modelEvent("opus-4.8", worker), // worker stamped itself strong
		tierEvent("strong", worker),
	}
	d := ModelCapabilityFloor(selfApplied, dispatcherIs(disp), false)
	if d.Outcome != FloorRefuse {
		t.Fatalf("a self-applied strong stamp cleared the floor (outcome %v) — attestation collapsed to self-report", d.Outcome)
	}
	if d.State != ModelIndeterminate {
		t.Fatalf("state = %v, want ModelIndeterminate for a non-dispatcher applier", d.State)
	}
	if !strings.Contains(d.Message, "not attestation") {
		t.Fatalf("indeterminate remediation does not explain why a self-applied stamp fails:\n%s", d.Message)
	}
}

// A nil dispatcher predicate vouches for no one, so any stamp is unprovable and the floor
// refuses — an unconfigured deployment fails CLOSED, never open.
func TestModelCapabilityFloorNilPredicateFailsClosed(t *testing.T) {
	stamp := []LabelEvent{modelEvent("opus-4.8", "anyone"), tierEvent("strong", "anyone")}
	if d := ModelCapabilityFloor(stamp, nil, false); d.Outcome != FloorRefuse {
		t.Fatalf("nil predicate admitted a stamp (outcome %v) — an unconfigured floor must fail closed", d.Outcome)
	}
	// But a PR with NO stamp under a nil predicate is Unknown, not refused: absence is not a
	// forged stamp.
	if d := ModelCapabilityFloor(nil, nil, false); d.Outcome != FloorNoticeAllow {
		t.Fatalf("no-stamp under nil predicate = %v, want FloorNoticeAllow (absent, not a broken stamp)", d.Outcome)
	}
}

func TestFloorOutcomeZeroValueRefuses(t *testing.T) {
	var z FloorOutcome
	if z != FloorRefuse || z.Proceeds() {
		t.Fatalf("the zero FloorOutcome (%v) does not refuse — a forgotten state must fail closed", z)
	}
}

func TestModelFloorTierIsTopOfVocabulary(t *testing.T) {
	// The floor must be the STRONGEST tier the vocabulary offers, or it is not a floor. Pin
	// it against DispatchTiers so a vocabulary change that adds a stronger tier is caught.
	for _, tier := range DispatchTiers() {
		if tierRank(tier) > tierRank(ModelFloorTier) {
			t.Fatalf("tier %q outranks the floor %q — the floor is no longer the top rung", tier, ModelFloorTier)
		}
	}
	if !tierMeetsFloor(ModelFloorTier) {
		t.Fatal("the floor tier does not meet its own floor")
	}
	if tierMeetsFloor("any") {
		t.Fatal("tier 'any' cleared the strong floor")
	}
}
