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

// tlOf builds a StampTimeline from events alone, DERIVING the present label set from the
// events' standing state (a label is present when its last event is a `labeled`). It is a
// TEST convenience for rows whose subject is the applier or the content rule rather than
// the present-set read.
//
// A PRODUCTION caller must NOT do this: presence comes from the authoritative labels read,
// because a truncated timeline would otherwise make a standing stamp look ABSENT, and
// absent proceeds on the NOTICE path. TestStampActorIsLatestStandingLabeledEvent case (e)
// covers that direction with an explicit present set.
func tlOf(events ...LabelEvent) StampTimeline {
	live := map[string]bool{}
	seen := map[string]bool{}
	var order []string
	for _, e := range events {
		n := normLabel(e.Name)
		if !seen[n] {
			seen[n] = true
			order = append(order, n)
		}
		live[n] = !e.Removed
	}
	var present []string
	for _, n := range order {
		if live[n] {
			present = append(present, n)
		}
	}
	return StampTimeline{Present: present, Events: events}
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
		d := ModelCapabilityFloor(tlOf(strongStamp...), dispatcherIs(disp), false)
		if d.Outcome != FloorAllow || !d.Outcome.Proceeds() {
			t.Fatalf("outcome = %v, want FloorAllow", d.Outcome)
		}
		if d.State != ModelStamped || d.Stamp.Tier != "strong" {
			t.Fatalf("state/stamp = %v/%q, want stamped/strong", d.State, d.Stamp.Tier)
		}
	})

	t.Run("attested cheap refuses with remediation", func(t *testing.T) {
		d := ModelCapabilityFloor(tlOf(cheapStamp...), dispatcherIs(disp), false)
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
		d := ModelCapabilityFloor(StampTimeline{}, dispatcherIs(disp), false)
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
		d := ModelCapabilityFloor(tlOf(cheapStamp...), dispatcherIs(disp), true)
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
	d := ModelCapabilityFloor(tlOf(selfApplied...), dispatcherIs(disp), false)
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
	if d := ModelCapabilityFloor(tlOf(stamp...), nil, false); d.Outcome != FloorRefuse {
		t.Fatalf("nil predicate admitted a stamp (outcome %v) — an unconfigured floor must fail closed", d.Outcome)
	}
	// But a PR with NO stamp under a nil predicate is Unknown, not refused: absence is not a
	// forged stamp.
	if d := ModelCapabilityFloor(tlOf(), nil, false); d.Outcome != FloorNoticeAllow {
		t.Fatalf("no-stamp under nil predicate = %v, want FloorNoticeAllow (absent, not a broken stamp)", d.Outcome)
	}
}

func TestFloorOutcomeZeroValueRefuses(t *testing.T) {
	var z FloorOutcome
	if z != FloorRefuse || z.Proceeds() {
		t.Fatalf("the zero FloorOutcome (%v) does not refuse — a forgotten state must fail closed", z)
	}
}

// TestModelCapabilityFloorStampCases is the CASE TABLE for the whole stamp-to-decision
// contract, in one place, so the six answers are read side by side rather than inferred
// from five separate tests.
//
// WHY A TABLE, AND WHY NOW. The floor was reported in the field as refusing every STAMPED
// PR while an UNSTAMPED one proceeded — the stamp read as "applied by a non-dispatcher
// identity" no matter what it said. The cause was on the WRITER's side (the stamp was
// applied under the calling session's own credential rather than the dispatcher App's),
// but the table is what makes the reader's side unambiguous afterwards: each row states
// what the decision must be, so a future change that collapses two of these rows into one
// fails here rather than in a review lane.
func TestModelCapabilityFloorStampCases(t *testing.T) {
	const disp, other = "the-dispatcher", "someone-else"
	cases := []struct {
		why         string
		events      []LabelEvent
		wantOutcome FloorOutcome
		wantState   ModelState
		wantInMsg   []string
	}{
		{
			why:         "absent: no stamp at all proceeds with a NOTICE",
			events:      nil,
			wantOutcome: FloorNoticeAllow,
			wantState:   ModelUnknown,
			wantInMsg:   []string{"NOTICE"},
		},
		{
			why:         "strong by the dispatcher: CLEARED",
			events:      []LabelEvent{modelEvent("example-model-1", disp), tierEvent("strong", disp)},
			wantOutcome: FloorAllow,
			wantState:   ModelStamped,
			wantInMsg:   []string{"OK", "strong"},
		},
		{
			why:         "strong by another identity: UNREADABLE, refused",
			events:      []LabelEvent{modelEvent("example-model-1", other), tierEvent("strong", other)},
			wantOutcome: FloorRefuse,
			wantState:   ModelIndeterminate,
			wantInMsg:   []string{"UNREADABLE", "not attestation", other},
		},
		{
			why: "MIXED appliers: the dispatcher's tier plus someone else's model half is UNREADABLE",
			events: []LabelEvent{
				modelEvent("example-model-1", other),
				tierEvent("strong", disp),
			},
			wantOutcome: FloorRefuse,
			wantState:   ModelIndeterminate,
			wantInMsg:   []string{"UNREADABLE", other},
		},
		{
			why:         "any by the dispatcher: READABLE but below the floor — refuse NAMING the tier",
			events:      []LabelEvent{modelEvent("example-model-2", disp), tierEvent("any", disp)},
			wantOutcome: FloorRefuse,
			wantState:   ModelStamped,
			// A readable below-floor stamp must NOT be reported as unreadable: the operator's
			// next action differs (escalate the write vs re-stamp the PR), so the message must
			// name the tier it actually read and must not say UNREADABLE.
			wantInMsg: []string{`tier "any"`, "strong-tier session"},
		},
		{
			why: "conflicting tiers from the dispatcher: UNREADABLE",
			events: []LabelEvent{
				modelEvent("example-model-1", disp),
				tierEvent("strong", disp),
				tierEvent("any", disp),
			},
			wantOutcome: FloorRefuse,
			wantState:   ModelIndeterminate,
			wantInMsg:   []string{"UNREADABLE", "conflicting"},
		},
		{
			why:         "incomplete stamp (tier half only) from the dispatcher: UNREADABLE",
			events:      []LabelEvent{tierEvent("strong", disp)},
			wantOutcome: FloorRefuse,
			wantState:   ModelIndeterminate,
			wantInMsg:   []string{"UNREADABLE", "incomplete"},
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			d := ModelCapabilityFloor(tlOf(c.events...), dispatcherIs(disp), false)
			if d.Outcome != c.wantOutcome {
				t.Fatalf("outcome = %v, want %v\nmessage: %s", d.Outcome, c.wantOutcome, d.Message)
			}
			if d.State != c.wantState {
				t.Fatalf("state = %v, want %v\nmessage: %s", d.State, c.wantState, d.Message)
			}
			for _, want := range c.wantInMsg {
				if !strings.Contains(d.Message, want) {
					t.Errorf("message does not carry %q:\n%s", want, d.Message)
				}
			}
			// A readable stamp is never reported with the unreadable wording, and vice
			// versa: conflating them sends the operator to the wrong remedy.
			if c.wantState == ModelStamped && strings.Contains(d.Message, "UNREADABLE") {
				t.Errorf("a READABLE stamp was reported as UNREADABLE:\n%s", d.Message)
			}
		})
	}
}

// A refusal that says only "applied by a non-dispatcher identity" cannot be acted on: the
// operator cannot tell WHICH identity applied the stamp, nor which one the floor would
// have accepted. This is the diagnosis the field report had to reconstruct by hand from
// the timeline API, so the message must carry both logins.
func TestFloorRefusalNamesBothLogins(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster) // binds desk=example-desk-app
	events := []LabelEvent{
		{Name: DispatchedModelPrefix + "example-model-1", AppliedBy: "example-worker-app[bot]"},
		{Name: DispatchedTierPrefix + "strong", AppliedBy: "example-worker-app[bot]"},
	}
	d := ModelCapabilityFloor(tlOf(events...), IsDispatcherLogin, false)
	if d.Outcome != FloorRefuse {
		t.Fatalf("outcome = %v, want FloorRefuse", d.Outcome)
	}
	if !strings.Contains(d.Message, "example-worker-app[bot]") {
		t.Errorf("the refusal does not name the identity that APPLIED the stamp:\n%s", d.Message)
	}
	if !strings.Contains(d.Message, "example-desk-app[bot]") {
		t.Errorf("the refusal does not name the dispatcher identity it would have ACCEPTED:\n%s", d.Message)
	}
}

// NonDispatcherStampAppliers is the diagnosis the message above is built from: it names
// every STANDING applier of a dispatched-* label the predicate will not vouch for,
// de-duplicated and ordered, and nothing else. An empty answer means "no untrusted standing
// applier", never "not checked" — a nil predicate vouches for nobody, so every standing
// applier is listed.
//
// STANDING is the load-bearing word. A superseded application (its label later removed and
// re-applied) must NOT be listed: naming a login that no longer holds the stamp is exactly
// the report that made a repaired PR look unrepairable.
func TestNonDispatcherStampAppliers(t *testing.T) {
	const disp = "the-dispatcher"
	tl := tlOf(
		LabelEvent{Name: "size:S", AppliedBy: "someone-else"}, // not a stamp label: never listed
		modelEvent("example-model-1", "b-applier"),
		tierEvent("strong", "a-applier"),
		modelEvent("example-model-1", "b-applier"), // same standing applier collapses
	)
	got := NonDispatcherStampAppliers(tl, dispatcherIs(disp))
	if strings.Join(got, ",") != "a-applier,b-applier" {
		t.Fatalf("appliers = %v, want [a-applier b-applier] (sorted, de-duplicated, stamp labels only)", got)
	}
	if got := NonDispatcherStampAppliers(tl, nil); len(got) != 2 {
		t.Fatalf("nil predicate vouched for a standing applier (%v) — it must vouch for nobody", got)
	}
	if got := NonDispatcherStampAppliers(StampTimeline{}, dispatcherIs(disp)); len(got) != 0 {
		t.Fatalf("no events yielded appliers %v", got)
	}

	// A SUPERSEDED foreign application is not reported: the dispatcher removed it and
	// re-applied the label, so the standing applier is the dispatcher.
	repaired := tlOf(
		modelEvent("example-model-1", "b-applier"),
		tierEvent("strong", "b-applier"),
		LabelEvent{Name: DispatchedModelPrefix + "example-model-1", AppliedBy: disp, Removed: true},
		LabelEvent{Name: DispatchedTierPrefix + "strong", AppliedBy: disp, Removed: true},
		modelEvent("example-model-1", disp),
		tierEvent("strong", disp),
	)
	if got := NonDispatcherStampAppliers(repaired, dispatcherIs(disp)); len(got) != 0 {
		t.Fatalf("a re-stamped PR still names %v — a superseded application is not the standing one", got)
	}
}

// The dispatcher role is ONE declared value shared by the reader (IsDispatcherLogin) and
// the writer (the dispatch verb's stamp step). The field defect was exactly this pair
// naming different identities, so the constant is pinned here: the reader must resolve the
// dispatcher login from DispatcherRole and nothing else.
func TestDispatcherRoleIsOneDeclaration(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster)
	login, ok := RoleAppLogin(DispatcherRole)
	if !ok {
		t.Fatalf("the roster binds no App to the dispatcher role %q", DispatcherRole)
	}
	if !IsDispatcherLogin(login) {
		t.Fatalf("IsDispatcherLogin rejected %q, the App bound to the dispatcher role %q — reader and "+
			"role declaration have drifted apart", login, DispatcherRole)
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
