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
