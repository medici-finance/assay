package deskkit

import (
	"strings"
	"testing"
)

// FloorRiskOf reads the SAME classifier the security-review gate reads: a private repo is
// classed only when a changed path is a security trigger, a public repo is classed on every
// PR, and the classifier's own fail-closed answers (empty list, unknown repo) carry through.
// It never returns FloorRiskUnknown — the caller resolves an unreadable diff before reaching
// here — so the two values it CAN return are the whole of its contract.
func TestFloorRiskOfReusesTheSecurityGateSignal(t *testing.T) {
	cases := []struct {
		name  string
		repo  string
		files []string
		want  FloorRisk
	}{
		{"private repo, clean diff", trackerRepo, []string{"docs/adopting-assay.md"}, FloorNotRiskClassed},
		{"private repo, security path", trackerRepo, []string{"secrets/api.key"}, FloorRiskClassed},
		{"private repo, risk among clean", trackerRepo, []string{"README.md", "secrets/db.pw"}, FloorRiskClassed},
		{"public repo, README only", exampleK8sRepo, []string{"README.md"}, FloorRiskClassed},
		{"empty diff fails closed", trackerRepo, nil, FloorRiskClassed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FloorRiskOf(c.repo, c.files); got != c.want {
				t.Fatalf("FloorRiskOf(%q, %v) = %v, want %v — the floor's risk must agree with "+
					"RiskPathTriggered(%v)", c.repo, c.files, got, c.want, RiskPathTriggered(c.repo, c.files))
			}
		})
	}
}

// RefusesUnstamped is the fail-closed line: only a POSITIVELY not-risk-classed PR keeps the
// permissive NOTICE branch. The zero value (Unknown) and a risk-classed PR both refuse, so a
// caller that forgot to resolve risk cannot slip an unstamped write through.
func TestFloorRiskRefusesUnstampedFailsClosed(t *testing.T) {
	cases := []struct {
		r    FloorRisk
		want bool
	}{
		{FloorRiskUnknown, true},     // zero value — fail closed
		{FloorRiskClassed, true},     // risk-classed — refuse
		{FloorNotRiskClassed, false}, // the ONE lenient value
	}
	for _, c := range cases {
		if got := c.r.RefusesUnstamped(); got != c.want {
			t.Errorf("(%v).RefusesUnstamped() = %v, want %v", c.r, got, c.want)
		}
	}
}

// THE FAIL-FIRST EVIDENCE, in one test: on the IDENTICAL unstamped timeline, the base floor
// PROCEEDS (FloorNoticeAllow — today's blanket permissive behavior) and the risk-aware floor
// REFUSES once the PR is risk-classed. The two assertions on one input are the before/after:
// the base decision is what shipped, the wrapper is ruling 3.
func TestRiskAwareFloorRefusesUnstampedRiskClassed(t *testing.T) {
	const disp = "the-dispatcher"

	// (a) NO stamp at all.
	noStamp := StampTimeline{}
	// (b) an `any`-tier stamp — dispatcher-applied, readable, claims no strength.
	anyStamp := tlOf(modelEvent("haiku-3", disp), tierEvent("any", disp))

	for _, c := range []struct {
		name  string
		tl    StampTimeline
		claim ClaimLiveness
	}{
		{"no stamp", noStamp, ClaimLivenessUnknown},
		{"any-tier stamp", anyStamp, ClaimLivenessUnknown},
	} {
		t.Run(c.name, func(t *testing.T) {
			// BEFORE (base floor, unchanged): the unstamped PR PROCEEDS with a NOTICE.
			base := ModelCapabilityFloor(c.tl, dispatcherIs(disp), false, c.claim)
			if base.Outcome != FloorNoticeAllow {
				t.Fatalf("base floor outcome = %v, want FloorNoticeAllow (the behavior ruling 3 tightens)", base.Outcome)
			}

			// AFTER, non-risk: still proceeds — the regression guard. An unstamped non-risk
			// PR must be UNAFFECTED.
			if d := ModelCapabilityFloorRiskAware(c.tl, dispatcherIs(disp), false, c.claim, FloorNotRiskClassed); d.Outcome != FloorNoticeAllow {
				t.Fatalf("non-risk outcome = %v, want FloorNoticeAllow — an unstamped non-risk PR must "+
					"still proceed with a NOTICE", d.Outcome)
			}

			// AFTER, risk-classed: REFUSES. This is the hole ruling 3 closes.
			d := ModelCapabilityFloorRiskAware(c.tl, dispatcherIs(disp), false, c.claim, FloorRiskClassed)
			if d.Outcome != FloorRefuse || d.Outcome.Proceeds() {
				t.Fatalf("risk-classed outcome = %v, want FloorRefuse — an unstamped risk-classed write "+
					"must not proceed", d.Outcome)
			}
			// The refusal must say WHY and WHAT INSTEAD (talk-33): risk-classed, and the
			// review-lane remedy plus the loud override.
			for _, want := range []string{"RISK-CLASSED", "deskdispatch --kit review", ModelFloorOverrideEnv} {
				if !strings.Contains(d.Message, want) {
					t.Errorf("refusal message missing %q; got: %s", want, d.Message)
				}
			}

			// AFTER, unresolved risk (Unknown): also refuses — fail closed.
			if u := ModelCapabilityFloorRiskAware(c.tl, dispatcherIs(disp), false, c.claim, FloorRiskUnknown); u.Outcome != FloorRefuse {
				t.Fatalf("unknown-risk outcome = %v, want FloorRefuse — an undetermined risk fails closed", u.Outcome)
			}
		})
	}
}

// A stamp that AGED OUT (ruling 2: its dispatch claim was released) reads unstamped, so on a
// risk-classed PR it must refuse exactly like a naked absent stamp — the age-out's own file
// comment anticipated this: "it stops being free the moment an unstamped branch is made to
// refuse for some class of PR." Non-risk, it still proceeds.
func TestRiskAwareFloorRefusesAgedOutStampWhenRiskClassed(t *testing.T) {
	const disp = "the-dispatcher"
	// A dispatcher-applied STRONG stamp that would clear the floor while LIVE, but whose
	// dispatch claim has been released.
	strong := tlOf(modelEvent("opus-4.8", disp), tierEvent("strong", disp))

	// Sanity: released + base floor ages it out to a NOTICE (today's behavior).
	if base := ModelCapabilityFloor(strong, dispatcherIs(disp), false, ClaimReleased); base.Outcome != FloorNoticeAllow {
		t.Fatalf("aged-out base outcome = %v, want FloorNoticeAllow", base.Outcome)
	}
	// Non-risk: still a NOTICE.
	if d := ModelCapabilityFloorRiskAware(strong, dispatcherIs(disp), false, ClaimReleased, FloorNotRiskClassed); d.Outcome != FloorNoticeAllow {
		t.Fatalf("aged-out non-risk outcome = %v, want FloorNoticeAllow", d.Outcome)
	}
	// Risk-classed: refuses.
	if d := ModelCapabilityFloorRiskAware(strong, dispatcherIs(disp), false, ClaimReleased, FloorRiskClassed); d.Outcome != FloorRefuse {
		t.Fatalf("aged-out risk-classed outcome = %v, want FloorRefuse", d.Outcome)
	}
}

// The overlay touches ONLY the NOTICE outcome. A stamped-strong PR proceeds, a present-but-
// unreadable stamp refuses, and the loud override bypasses — all identically whether the PR is
// risk-classed or not. This is the proof rulings 1/2/4's settled cases are not re-decided by
// the risk value.
func TestRiskAwareFloorLeavesEveryOtherOutcomeUntouched(t *testing.T) {
	const disp = "the-dispatcher"
	strong := tlOf(modelEvent("opus-4.8", disp), tierEvent("strong", disp))
	selfApplied := tlOf(modelEvent("opus-4.8", "not-the-dispatcher"), tierEvent("strong", "not-the-dispatcher"))
	anyStamp := tlOf(modelEvent("haiku-3", disp), tierEvent("any", disp))

	for _, risk := range []FloorRisk{FloorNotRiskClassed, FloorRiskClassed, FloorRiskUnknown} {
		// Attested strong: always allow — risk is irrelevant to a cleared floor.
		if d := ModelCapabilityFloorRiskAware(strong, dispatcherIs(disp), false, ClaimLivenessUnknown, risk); d.Outcome != FloorAllow {
			t.Errorf("strong stamp, risk=%v: outcome = %v, want FloorAllow", risk, d.Outcome)
		}
		// Present-but-unreadable (self-applied): always refuse — but as UNREADABLE, not the
		// risk refusal, so the message is the unreadable one regardless of risk.
		if d := ModelCapabilityFloorRiskAware(selfApplied, dispatcherIs(disp), false, ClaimLivenessUnknown, risk); d.Outcome != FloorRefuse {
			t.Errorf("self-applied stamp, risk=%v: outcome = %v, want FloorRefuse", risk, d.Outcome)
		} else if !strings.Contains(d.Message, "UNREADABLE") {
			t.Errorf("self-applied stamp, risk=%v: message should be the UNREADABLE refusal, got: %s", risk, d.Message)
		}
		// Override: always bypasses, even a risk-classed unstamped PR — incident recovery
		// must not be defeated by the new refusal.
		if d := ModelCapabilityFloorRiskAware(anyStamp, dispatcherIs(disp), true, ClaimLivenessUnknown, risk); d.Outcome != FloorOverrideAllow {
			t.Errorf("override, risk=%v: outcome = %v, want FloorOverrideAllow", risk, d.Outcome)
		}
	}
}
