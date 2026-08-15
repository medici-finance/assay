package improve

import (
	"strings"
	"testing"
)

func proposals(n int) []ProposalRow {
	out := make([]ProposalRow, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ProposalRow{
			ID: "P-" + string(rune('1'+i)), Title: "t", IntakeRef: "I-0" + string(rune('1'+i)),
			MotivatingEvidence: []string{"e"}, TargetMetric: "lead time",
			Adopt: HumanCommitAdopt("adopt this change"),
		})
	}
	return out
}

// TestImprovePaneAdoptRoutesToTheHumanAndCarriesTheGlyph — §6.2 and §7.6
// together: an agent may propose, only the human adopts, and the gate is never
// encoded by colour alone.
func TestImprovePaneAdoptRoutesToTheHumanAndCarriesTheGlyph(t *testing.T) {
	a := HumanCommitAdopt("adopt this change")
	if err := a.Validate("P-1"); err != nil {
		t.Fatalf("the constructed adopt action should validate, got %v", err)
	}
	if a.Route != RouteHumanCommit {
		t.Fatalf("route = %q, want the human commit path", a.Route)
	}
	if a.Glyph != AdoptGlyph {
		t.Fatalf("glyph = %q, want the human-gate glyph", a.Glyph)
	}
	out := a.Render()
	if !strings.HasPrefix(out, AdoptGlyph) {
		t.Fatalf("the glyph must lead the rendered element so the gate is first in any medium:\n%s", out)
	}
	if !strings.Contains(out, string(RouteHumanCommit)) {
		t.Fatalf("the rendered element must name where it routes:\n%s", out)
	}

	// There is exactly ONE declared route. If a second appears, adding it was
	// a decision somebody made in a reviewable diff — this assertion is what
	// makes that true.
	if len(declaredRoutes) != 1 {
		t.Fatalf("this package declares %d routes; the gold gate depends on there being exactly one, and the only one being the human commit path", len(declaredRoutes))
	}

	bad := []struct {
		name string
		a    AdoptAction
		want string
	}{
		{"no glyph", AdoptAction{Route: RouteHumanCommit, Label: "l", Adoptable: true}, "human-gate glyph"},
		{"wrong glyph", AdoptAction{Glyph: "*", Route: RouteHumanCommit, Label: "l", Adoptable: true}, "human-gate glyph"},
		{"undeclared route", AdoptAction{Glyph: AdoptGlyph, Route: Route("write-register"), Label: "l", Adoptable: true}, "not a declared route"},
		{"empty route", AdoptAction{Glyph: AdoptGlyph, Label: "l", Adoptable: true}, "not a declared route"},
		{"no label", AdoptAction{Glyph: AdoptGlyph, Route: RouteHumanCommit, Adoptable: true}, "no label"},
		{"disabled with no reason", AdoptAction{Glyph: AdoptGlyph, Route: RouteHumanCommit, Label: "l"}, "does not say why"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.a.Validate("P-1")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not name %q", err, tc.want)
			}
		})
	}
}

// TestImprovePaneOneAdoptionPerCadence — the hard rule from §5.3, applied over
// a whole strip rather than row by row, and with the disabled control saying
// WHICH change took the slot.
func TestImprovePaneOneAdoptionPerCadence(t *testing.T) {
	ps := proposals(3)

	// While the cadence's single slot is free, every proposal is OFFERED —
	// the rule caps adoptions TAKEN, not controls shown, and hiding the other
	// two would be the pane choosing for the human.
	fresh := ApplyCadenceGate(ps, 0, "")
	if got := AdoptableCount(fresh); got != len(ps) {
		t.Fatalf("with the cadence's slot free, %d of %d proposals are adoptable; the human picks which one, so all are offered", got, len(ps))
	}

	spent := ApplyCadenceGate(ps, 1, "A-7 (shorten the review queue)")
	if got := AdoptableCount(spent); got != 0 {
		t.Fatalf("with the cadence's adoption taken, %d proposals are still adoptable; want 0", got)
	}
	for _, p := range spent {
		if err := p.Validate(); err != nil {
			t.Fatalf("a queued proposal must still validate and render, got %v", err)
		}
		out := p.Adopt.Render()
		if !strings.Contains(out, "queued-not-adoptable") {
			t.Fatalf("a queued proposal must render as queued, not as adoptable-and-ignored:\n%s", out)
		}
		if !strings.Contains(out, "A-7") {
			t.Fatalf("the disabled control must name the adoption that consumed the cadence:\n%s", out)
		}
	}

	// A cadence overspent past the rule stays closed rather than wrapping.
	over := ApplyCadenceGate(ps, 5, "several")
	if got := AdoptableCount(over); got != 0 {
		t.Fatalf("an overspent cadence left %d adoptable; want 0", got)
	}

	// The rule is a constant, not configuration. A limit that can be raised on
	// a busy week is not a limit.
	if AdoptionsPerCadence != 1 {
		t.Fatalf("AdoptionsPerCadence = %d; §5.3's rule is ONE per cadence and changing it is a methodology decision, not a tuning knob", AdoptionsPerCadence)
	}
}

// TestImprovePaneCadenceGateIsTheOnlyWayToBeAdoptable — a caller cannot set
// Adoptable by hand and have it survive the gate.
func TestImprovePaneCadenceGateIsTheOnlyWayToBeAdoptable(t *testing.T) {
	ps := proposals(2)
	for i := range ps {
		ps[i].Adopt.Adoptable = true
		ps[i].Adopt.NotAdoptableWhy = ""
	}
	gated := ApplyCadenceGate(ps, 1, "A-7")
	if got := AdoptableCount(gated); got != 0 {
		t.Fatalf("hand-set Adoptable survived the cadence gate on %d rows", got)
	}
}
