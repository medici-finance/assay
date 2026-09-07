package deskkit

// modelstampactor_test.go — WHICH label event names the applier of a standing stamp.
//
// THE DEFECT THESE PIN. The applier-aware reader treated ANY historical `labeled` event for
// a dispatched-* label as the applier of that label. A GitHub timeline is APPEND-ONLY, so a
// PR stamped once under a foreign login could never be repaired: the real dispatcher could
// remove the labels and re-apply them under its own identity, and the reader still found the
// original foreign `labeled` event and refused every authority-bearing write on that PR
// forever. Observed on a live public PR — the foreign stamp was removed and re-applied by
// the bound dispatcher App, and the floor still refused naming the ORIGINAL applier.
//
// THE RULE THEY FIX IT TO. For each dispatched-* label CURRENTLY on the PR, the applier is
// the actor of the LAST `labeled` event for that name that is not followed by an `unlabeled`
// of the same name. A genuine re-stamp is therefore readable; a foreign stamp that is still
// standing is NOT laundered by anything that happened before it.

import (
	"strings"
	"testing"
)

// labeledBy / unlabeledBy build the two timeline event kinds. They spell out the whole
// label name so a case's timeline reads in the order GitHub returns it.
func labeledBy(name, who string) LabelEvent {
	return LabelEvent{Name: name, AppliedBy: who}
}

func unlabeledBy(name, who string) LabelEvent {
	return LabelEvent{Name: name, AppliedBy: who, Removed: true}
}

// The actor-resolution table. Every row is a COMPLETE strong stamp by content — the only
// variable is who applied the STANDING application of each half — so a row's outcome is
// attributable to the actor rule and nothing else.
func TestStampActorIsLatestStandingLabeledEvent(t *testing.T) {
	const (
		disp    = "the-dispatcher"
		foreign = "some-other-login"
	)
	model := DispatchedModelPrefix + "example-model-1"
	tier := DispatchedTierPrefix + "strong"
	present := []string{model, tier}

	cases := []struct {
		name    string
		tl      StampTimeline
		want    ModelState
		wantWho string // a login the refusal must name, "" when none is expected
	}{
		{
			// (a) A foreign stamp that is STILL STANDING stays unreadable. The fix must not
			// launder history — only respect a genuine re-stamp.
			name: "foreign applied and never removed stays unreadable",
			tl: StampTimeline{Present: present, Events: []LabelEvent{
				labeledBy(model, foreign),
				labeledBy(tier, foreign),
			}},
			want:    ModelIndeterminate,
			wantWho: foreign,
		},
		{
			// (b) THE REPAIR. The dispatcher removed the foreign labels and re-applied them
			// under its own identity. The standing application is the dispatcher's, so the
			// stamp is readable and the floor CLEARS. Before the fix this row read the
			// original foreign `labeled` events and refused forever.
			name: "foreign removed then re-applied by the dispatcher clears",
			tl: StampTimeline{Present: present, Events: []LabelEvent{
				labeledBy(model, foreign),
				labeledBy(tier, foreign),
				unlabeledBy(model, disp),
				unlabeledBy(tier, disp),
				labeledBy(model, disp),
				labeledBy(tier, disp),
			}},
			want: ModelStamped,
		},
		{
			// (c) A re-stamp by a FOREIGN login is not a repair. The standing application is
			// still foreign.
			name: "removed then re-applied by a foreign login stays unreadable",
			tl: StampTimeline{Present: present, Events: []LabelEvent{
				labeledBy(model, foreign),
				labeledBy(tier, foreign),
				unlabeledBy(model, disp),
				unlabeledBy(tier, disp),
				labeledBy(model, foreign),
				labeledBy(tier, foreign),
			}},
			want:    ModelIndeterminate,
			wantWho: foreign,
		},
		{
			// (d) The inverse direction fails closed too: a dispatcher stamp that a foreign
			// login removed and re-applied is a FOREIGN standing application, and an earlier
			// dispatcher event does not vouch for it.
			name: "dispatcher stamp overwritten by a foreign login is unreadable",
			tl: StampTimeline{Present: present, Events: []LabelEvent{
				labeledBy(model, disp),
				labeledBy(tier, disp),
				unlabeledBy(model, foreign),
				unlabeledBy(tier, foreign),
				labeledBy(model, foreign),
				labeledBy(tier, foreign),
			}},
			want:    ModelIndeterminate,
			wantWho: foreign,
		},
		{
			// (e) A present label the events cannot attribute is could-not-check, NEVER
			// unstamped: an incomplete timeline read must not make a standing stamp look
			// absent, because absent proceeds on the NOTICE path.
			name: "present label with no labeled event is unreadable, not absent",
			tl: StampTimeline{Present: present, Events: []LabelEvent{
				labeledBy(model, disp),
			}},
			want: ModelIndeterminate,
		},
		{
			// The contradictory-content case is unchanged by the actor rule: two tier labels
			// standing, both from the dispatcher, still resolve to no single (model, tier).
			name: "two standing tier labels stay unreadable",
			tl: StampTimeline{
				Present: []string{model, tier, DispatchedTierPrefix + "any"},
				Events: []LabelEvent{
					labeledBy(model, disp),
					labeledBy(tier, disp),
					labeledBy(DispatchedTierPrefix+"any", disp),
				},
			},
			want: ModelIndeterminate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, state := AttestedModelStampOf(tc.tl, dispatcherIs(disp))
			if state != tc.want {
				t.Fatalf("state = %v, want %v", state, tc.want)
			}
			d := ModelCapabilityFloor(tc.tl, dispatcherIs(disp), false)
			if tc.want == ModelStamped {
				if d.Outcome != FloorAllow {
					t.Fatalf("floor outcome = %v (%s), want FloorAllow — a re-stamp by the "+
						"dispatcher is the only repair an append-only timeline allows", d.Outcome, d.Message)
				}
				return
			}
			if d.Outcome != FloorRefuse {
				t.Fatalf("floor outcome = %v, want FloorRefuse", d.Outcome)
			}
			if tc.wantWho != "" && !strings.Contains(d.Message, tc.wantWho) {
				t.Fatalf("refusal does not name the standing applier %q:\n%s", tc.wantWho, d.Message)
			}
		})
	}
}

// The unattributable case gets its OWN refusal wording. "Unreadable" without the cause sends
// the operator to re-stamp a PR whose stamp is fine and whose timeline read was short.
func TestUnattributedStampLabelIsNamedInTheRefusal(t *testing.T) {
	const disp = "the-dispatcher"
	tl := StampTimeline{
		Present: []string{DispatchedModelPrefix + "example-model-1", DispatchedTierPrefix + "strong"},
		Events:  []LabelEvent{labeledBy(DispatchedModelPrefix+"example-model-1", disp)},
	}
	got := UnattributedStampLabels(tl)
	if len(got) != 1 || got[0] != DispatchedTierPrefix+"strong" {
		t.Fatalf("UnattributedStampLabels = %v, want just the tier half", got)
	}
	d := ModelCapabilityFloor(tl, dispatcherIs(disp), false)
	if d.Outcome != FloorRefuse {
		t.Fatalf("outcome = %v, want FloorRefuse — an unattributable present stamp is could-not-check", d.Outcome)
	}
	if !strings.Contains(d.Message, DispatchedTierPrefix+"strong") {
		t.Fatalf("refusal does not name the unattributable label:\n%s", d.Message)
	}
}

// A REMOVED label contributes no content. Without this, a superseded
// `dispatched-tier:any` would keep conflicting with the standing `dispatched-tier:strong`
// and the PR would read unreadable forever — the same append-only trap, one axis over.
func TestRemovedLabelContributesNoStampContent(t *testing.T) {
	const disp = "the-dispatcher"
	tl := StampTimeline{
		Present: []string{DispatchedModelPrefix + "example-model-1", DispatchedTierPrefix + "strong"},
		Events: []LabelEvent{
			labeledBy(DispatchedTierPrefix+"any", disp),
			labeledBy(DispatchedModelPrefix+"example-model-1", disp),
			unlabeledBy(DispatchedTierPrefix+"any", disp),
			labeledBy(DispatchedTierPrefix+"strong", disp),
		},
	}
	stamp, state := AttestedModelStampOf(tl, dispatcherIs(disp))
	if state != ModelStamped || stamp.Tier != "strong" {
		t.Fatalf("state/tier = %v/%q, want stamped/strong — a removed label is not standing content",
			state, stamp.Tier)
	}
}

// ForeignStampLabels is the WRITER's half of the same resolution: exactly the labels a
// re-stamp must REMOVE before applying its own, because adding over a present label is a
// no-op. Reader and writer project it from one function so they cannot disagree about which
// application is standing.
func TestForeignStampLabels(t *testing.T) {
	const (
		disp    = "the-dispatcher"
		foreign = "some-other-login"
	)
	model := DispatchedModelPrefix + "example-model-1"
	tier := DispatchedTierPrefix + "strong"

	cases := []struct {
		why  string
		tl   StampTimeline
		want []string
	}{
		{
			why:  "a standing foreign application must be removed",
			tl:   StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{labeledBy(model, foreign), labeledBy(tier, disp)}},
			want: []string{model},
		},
		{
			why: "the dispatcher's own standing stamp is left alone (no churn per re-dispatch)",
			tl: StampTimeline{Present: []string{model, tier},
				Events: []LabelEvent{labeledBy(model, disp), labeledBy(tier, disp)}},
			want: nil,
		},
		{
			why: "an ALREADY REPAIRED stamp is not stripped again",
			tl: StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
				labeledBy(model, foreign), labeledBy(tier, foreign),
				unlabeledBy(model, disp), unlabeledBy(tier, disp),
				labeledBy(model, disp), labeledBy(tier, disp),
			}},
			want: nil,
		},
		{
			why:  "an unattributable present label is re-stamped: re-applying it is safe, leaving it is refused",
			tl:   StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{labeledBy(model, disp)}},
			want: []string{tier},
		},
		{
			why:  "a REMOVED label is not removed again",
			tl:   StampTimeline{Present: nil, Events: []LabelEvent{labeledBy(model, foreign), unlabeledBy(model, disp)}},
			want: nil,
		},
		{
			why:  "non-stamp labels are never touched",
			tl:   StampTimeline{Present: []string{"size:S", "approval-needed"}, Events: []LabelEvent{labeledBy("size:S", foreign)}},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			got := ForeignStampLabels(c.tl, dispatcherIs(disp))
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Fatalf("ForeignStampLabels = %v, want %v", got, c.want)
			}
		})
	}

	// A nil predicate vouches for nobody, so every present stamp label is returned — the
	// same fail-closed direction the reader takes.
	all := ForeignStampLabels(StampTimeline{Present: []string{model, tier},
		Events: []LabelEvent{labeledBy(model, disp), labeledBy(tier, disp)}}, nil)
	if len(all) != 2 {
		t.Fatalf("nil predicate returned %v, want both stamp labels", all)
	}
}

// ReStampRemovals is the writer's set for a re-dispatch that means to REPLACE a corrupt stamp,
// not merely repair a foreign applier. The rows a plain ForeignStampLabels MISSES are the
// point: a conflicting or stale dispatched-* label the DISPATCHER itself left is dropped by
// ForeignStampLabels (the applier is trusted) yet keeps the stamp unreadable, so re-dispatch
// reported OK while the floor kept refusing. This is that present-but-unreadable deadlock.
func TestReStampRemovals(t *testing.T) {
	const (
		disp    = "the-dispatcher"
		foreign = "some-other-login"
	)
	model := DispatchedModelPrefix + "example-model-1"
	tier := DispatchedTierPrefix + "strong"
	want := []string{model, tier} // the pair the re-dispatch is applying

	staleModel := DispatchedModelPrefix + "example-model-2"
	staleTier := DispatchedTierPrefix + "any"
	badTier := DispatchedTierPrefix + "medium" // out of vocabulary

	cases := []struct {
		why     string
		tl      StampTimeline
		predNil bool
		remove  []string
	}{
		{
			// THE DEADLOCK. An earlier run of the dispatcher itself left a DIFFERENT model
			// slug; both halves the re-dispatch wants are already standing under the dispatcher.
			// ForeignStampLabels removes nothing here (every applier is trusted), so the
			// conflicting slug survives and the floor refuses forever. ReStampRemovals drops it.
			why: "a conflicting model slug the dispatcher itself applied is removed",
			tl: StampTimeline{Present: []string{model, staleModel, tier}, Events: []LabelEvent{
				labeledBy(model, disp), labeledBy(staleModel, disp), labeledBy(tier, disp),
			}},
			remove: []string{staleModel},
		},
		{
			// A stale SECOND tier the dispatcher left on an earlier run — same deadlock, tier axis.
			why: "a stale second tier the dispatcher applied is removed",
			tl: StampTimeline{Present: []string{model, tier, staleTier}, Events: []LabelEvent{
				labeledBy(model, disp), labeledBy(tier, disp), labeledBy(staleTier, disp),
			}},
			remove: []string{staleTier},
		},
		{
			// A malformed (out-of-vocabulary) half the dispatcher left is cleared too.
			why: "an out-of-vocabulary tier the dispatcher applied is removed",
			tl: StampTimeline{Present: []string{model, tier, badTier}, Events: []LabelEvent{
				labeledBy(model, disp), labeledBy(tier, disp), labeledBy(badTier, disp),
			}},
			remove: []string{badTier},
		},
		{
			// The correct pair already standing under the dispatcher: nothing to churn.
			why: "the exact wanted pair standing under the dispatcher is left alone",
			tl: StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
				labeledBy(model, disp), labeledBy(tier, disp),
			}},
			remove: nil,
		},
		{
			// The ForeignStampLabels case still holds: a foreign-applied WANT label is removed.
			why: "a foreign-applied half of the wanted pair is still removed",
			tl: StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
				labeledBy(model, foreign), labeledBy(tier, disp),
			}},
			remove: []string{model},
		},
		{
			// A present want label the timeline read cannot attribute is re-stamped.
			why: "an unattributable half of the wanted pair is removed",
			tl: StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
				labeledBy(model, disp),
			}},
			remove: []string{tier},
		},
		{
			// Non-stamp labels are never touched, whoever applied them.
			why: "non-stamp labels are never removed",
			tl: StampTimeline{Present: []string{"size:S", model, tier}, Events: []LabelEvent{
				labeledBy("size:S", foreign), labeledBy(model, disp), labeledBy(tier, disp),
			}},
			remove: nil,
		},
		{
			// A nil predicate vouches for nobody: every present stamp label goes, including the
			// wanted pair, so the re-apply lands them all under the dispatcher.
			why: "a nil predicate removes every present stamp label",
			tl: StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
				labeledBy(model, disp), labeledBy(tier, disp),
			}},
			predNil: true,
			remove:  []string{model, tier},
		},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			pred := dispatcherIs(disp)
			if c.predNil {
				pred = nil
			}
			got := ReStampRemovals(c.tl, want, pred)
			if strings.Join(got, ",") != strings.Join(c.remove, ",") {
				t.Fatalf("ReStampRemovals = %v, want %v", got, c.remove)
			}
		})
	}
}

// The end-to-end invariant of the recovery: a PR whose stamp is present-but-unreadable because
// the dispatcher left a conflicting label REFUSES before a re-stamp, and after ReStampRemovals
// clears the conflict and the intended pair stands alone under the dispatcher, the floor CLEARS
// — while a genuinely below-floor tier still refuses and a foreign applier still refuses. This
// is the whole point: the recovery breaks the deadlock without widening what the floor trusts.
//
// `dispatched-tier:any` is NOT the below-floor example. Per the floor's contract (see
// modelfloor.go, "WHY `any` IS ABSENT, NOT BELOW") `any` is the brief schema's "no particular
// runner demanded" — a stamp that records no strength claim — so a re-stamp to `any` takes the
// same outcome as an unstamped PR: proceed with a NOTICE that names the label. The below-floor
// refusal is kept live by the rank order, and this test drives it the way the floor's own
// rank test does (`-run TestFloorTierRank` in modelfloor_test.go, which pins the below-floor
// branch live): a synthetic rung the vocabulary does not carry neither meets the floor nor
// claims no strength, so it lands on the refuse side of both questions, and a stamp naming it
// refuses end to end.
func TestReStampRecoveryKeepsTheFloor(t *testing.T) {
	const disp = "the-dispatcher"
	model := DispatchedModelPrefix + "example-model-1"
	tier := DispatchedTierPrefix + "strong"
	staleModel := DispatchedModelPrefix + "example-model-2"

	// Before: dispatcher-applied conflicting model slug. Floor refuses, and it is NOT a
	// below-tier or foreign refusal — it is the content-unreadable branch.
	before := StampTimeline{Present: []string{model, staleModel, tier}, Events: []LabelEvent{
		labeledBy(model, disp), labeledBy(staleModel, disp), labeledBy(tier, disp),
	}}
	if d := ModelCapabilityFloor(before, dispatcherIs(disp), false); d.Outcome != FloorRefuse {
		t.Fatalf("pre-recovery outcome = %v, want FloorRefuse", d.Outcome)
	}

	// Apply the recovery in the model: remove what ReStampRemovals names, then the intended
	// pair stands alone under the dispatcher.
	remove := ReStampRemovals(before, []string{model, tier}, dispatcherIs(disp))
	if len(remove) != 1 || remove[0] != staleModel {
		t.Fatalf("recovery removed %v, want just the conflicting slug %q", remove, staleModel)
	}
	after := StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
		labeledBy(model, disp), labeledBy(staleModel, disp), labeledBy(tier, disp),
		unlabeledBy(staleModel, disp),
	}}
	if d := ModelCapabilityFloor(after, dispatcherIs(disp), false); d.Outcome != FloorAllow {
		t.Fatalf("post-recovery outcome = %v (%s), want FloorAllow", d.Outcome, d.Message)
	}

	// A re-stamp to `any` is a readable, dispatcher-applied stamp that claims no strength:
	// NOTICE, not refusal, and the NOTICE names the label it read so the operator can tell it
	// from the unstamped case.
	lowTier := DispatchedTierPrefix + NoStrengthClaimTier
	low := StampTimeline{Present: []string{model, lowTier}, Events: []LabelEvent{
		labeledBy(model, disp), labeledBy(lowTier, disp),
	}}
	if d := ModelCapabilityFloor(low, dispatcherIs(disp), false); d.Outcome != FloorNoticeAllow {
		t.Fatalf("tier-any outcome = %v (%s), want FloorNoticeAllow — `any` is no strength claim, not a below-floor tier", d.Outcome, d.Message)
	} else if !strings.Contains(d.Message, "NOTICE") || !strings.Contains(d.Message, lowTier) {
		t.Fatalf("the tier-any NOTICE does not name the %s label it read:\n%s", lowTier, d.Message)
	}

	// The floor is NOT weakened. A re-stamp to a genuinely below-floor tier still refuses —
	// the recovery must not admit a weak tier. The vocabulary as it stands has no such rung,
	// so this is driven exactly as the floor's rank test drives it: a synthetic rung that
	// neither meets the floor nor claims no strength is on the refuse side of both questions
	// the floor asks of a readable stamp, in that order.
	const weakTier = "some-tier-nobody-emits"
	if tierMeetsFloor(weakTier) {
		t.Fatalf("synthetic rung %q meets the %s floor — the recovery would admit a weak tier", weakTier, ModelFloorTier)
	}
	if tierClaimsNoStrength(weakTier) {
		t.Fatalf("synthetic rung %q reads as claiming no strength — it would take the NOTICE, not the refusal", weakTier)
	}
	// And end to end, a stamp naming that rung, applied by the dispatcher after the same
	// recovery, refuses. (The reader holds the vocabulary to {any, strong}, so today the rung
	// arrives as present-but-unreadable rather than as a readable below-floor tier; either
	// way the floor fails closed, which is the invariant this block guards.)
	weak := StampTimeline{Present: []string{model, DispatchedTierPrefix + weakTier}, Events: []LabelEvent{
		labeledBy(model, disp), labeledBy(DispatchedTierPrefix+weakTier, disp),
	}}
	if d := ModelCapabilityFloor(weak, dispatcherIs(disp), false); d.Outcome != FloorRefuse {
		t.Fatalf("below-floor tier outcome = %v (%s), want FloorRefuse — the recovery must not admit a weak tier", d.Outcome, d.Message)
	}

	// And a clean pair applied by a NON-dispatcher still refuses (self-report is worthless).
	self := StampTimeline{Present: []string{model, tier}, Events: []LabelEvent{
		labeledBy(model, "the-worker-itself"), labeledBy(tier, "the-worker-itself"),
	}}
	if d := ModelCapabilityFloor(self, dispatcherIs(disp), false); d.Outcome != FloorRefuse {
		t.Fatalf("self-applied stamp outcome = %v, want FloorRefuse — the recovery must not admit a self-stamp", d.Outcome)
	}
}
