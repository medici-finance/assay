package improve

import (
	"errors"
	"fmt"
	"strings"
)

// THE GOLD GATE
// -------------
// §6.2, applied to process change: an agent may PROPOSE; only the human
// ADOPTS. The pane can show that three reports cluster into one systemic
// issue, and a draft proposal can be attached to that cluster — but a retro
// action exists only when the human commits it, one per cadence.
//
// That rule is carried here by three properties, none of which is a comment:
//
//  1. [Route] has exactly ONE legal value. There is no "write the register
//     directly" route to select, so making the pane write requires adding a
//     route constant, wiring it, and deleting a test.
//  2. [AdoptAction] is a routing VALUE. It holds no register path, no file
//     handle and no command; there is nothing in this package that could
//     execute one. The append-only registers keep their existing lint because
//     nothing here goes near them.
//  3. The glyph is mandatory. §7.6 forbids colour-only encoding of a human
//     gate, so an adopt element that does not carry the glyph is refused at
//     validation rather than rendered in a colour somebody may not see.

// AdoptGlyph accompanies every human-gate element (§7.6). It is required, not
// decorative: colour alone is not an encoding, and a gate a colour-blind or
// monochrome reader cannot see is a gate that gets walked through.
const AdoptGlyph = "⟡"

// Route is where an adopt action sends the operator. It is a closed type with
// one legal value.
type Route string

// RouteHumanCommit is the ONLY declared route: the human commit path. The
// pane hands the operator there and stops. It does not append to a register,
// it does not open a PR, and it does not record an adoption of its own.
const RouteHumanCommit Route = "human-commit-path"

var declaredRoutes = map[Route]bool{RouteHumanCommit: true}

// AdoptAction is the human-gate element on a proposal row.
type AdoptAction struct {
	// Glyph must be [AdoptGlyph].
	Glyph string
	// Route must be [RouteHumanCommit].
	Route Route
	// Label is the operator-facing text beside the glyph. It is required for
	// the same reason the glyph is: a control identified only by an icon is a
	// control identified only by shape.
	Label string
	// Adoptable reports whether this cadence's single adoption is still
	// available. It is set by [ApplyCadenceGate], never by hand at the call
	// site — see that function for why.
	Adoptable bool
	// NotAdoptableWhy names the adoption that consumed the cadence. Required
	// whenever Adoptable is false, so that a greyed-out control says who took
	// the slot rather than merely being greyed out.
	NotAdoptableWhy string
}

// HumanCommitAdopt builds the only shape of adopt action this package permits.
// It is the constructor a caller should use; hand-building the struct is
// possible but must still pass [AdoptAction.Validate].
func HumanCommitAdopt(label string) AdoptAction {
	return AdoptAction{Glyph: AdoptGlyph, Route: RouteHumanCommit, Label: label, Adoptable: true}
}

// Validate refuses an adopt element that cannot honestly render.
func (a AdoptAction) Validate(owner string) error {
	if a.Glyph != AdoptGlyph {
		return fmt.Errorf("adopt element carries glyph %q, not the human-gate glyph — every human-gate element carries it and no gate is encoded by colour alone, so an element without it is refused rather than rendered", a.Glyph)
	}
	if !declaredRoutes[a.Route] {
		return fmt.Errorf("adopt element routes to %q, which is not a declared route — the only declared route is the human commit path, because a retro action exists only when the human commits it", a.Route)
	}
	if strings.TrimSpace(a.Label) == "" {
		return errors.New("adopt element has no label — a control identified only by a glyph is a control identified only by shape")
	}
	if !a.Adoptable && strings.TrimSpace(a.NotAdoptableWhy) == "" {
		return errors.New("adopt element is not adoptable but does not say why — a disabled gate with no stated reason reads as a broken control rather than as a rule being applied")
	}
	if a.Adoptable && strings.TrimSpace(a.NotAdoptableWhy) != "" {
		return errors.New("adopt element is adoptable but carries a not-adoptable reason")
	}
	_ = owner
	return nil
}

// Render is the adopt element as a line. The glyph leads, so the gate is the
// first thing on the row in any medium, including a plain-text one.
func (a AdoptAction) Render() string {
	state := "adoptable"
	if !a.Adoptable {
		state = "queued-not-adoptable (" + a.NotAdoptableWhy + ")"
	}
	return fmt.Sprintf("%s %s → %s [%s]", a.Glyph, a.Label, a.Route, state)
}

// AdoptionsPerCadence is the hard rule from §5.3: ONE adopted process change
// per cadence. It is a constant rather than configuration because the rule is
// what keeps the system from growing controls faster than it can judge them,
// and a configurable limit is a limit that gets raised on a busy week.
const AdoptionsPerCadence = 1

// ApplyCadenceGate marks each proposal adoptable or not according to how many
// adoptions the cadence has already taken. It is the single place the rule is
// applied, so a caller cannot set Adoptable=true on a row by hand and have it
// survive.
//
// The rule caps adoptions TAKEN, not controls SHOWN. While the cadence's slot
// is free every proposal is offered, because choosing which one to adopt is
// the human's half of the gate and a pane that pre-selected would be taking
// it. Once the slot is spent, every remaining proposal renders as
// queued-not-adoptable naming the adoption that took it.
//
// alreadyAdopted is the count of retro actions already committed in THIS
// cadence, and takenBy names them — a greyed-out control that says which
// change took the slot is a rule being applied; one that says nothing is a
// bug as far as the operator can tell.
func ApplyCadenceGate(proposals []ProposalRow, alreadyAdopted int, takenBy string) []ProposalRow {
	out := make([]ProposalRow, 0, len(proposals))
	slotFree := alreadyAdopted < AdoptionsPerCadence
	for _, p := range proposals {
		if slotFree {
			p.Adopt.Adoptable = true
			p.Adopt.NotAdoptableWhy = ""
		} else {
			p.Adopt.Adoptable = false
			p.Adopt.NotAdoptableWhy = fmt.Sprintf(
				"this cadence's %d adoption(s) are already taken by %s — one process change per cadence (§5.3), so this proposal stays queued rather than being adoptable-and-ignored",
				AdoptionsPerCadence, fallback(takenBy, "an adoption whose name was not supplied, which is itself a gap"))
		}
		out = append(out, p)
	}
	return out
}

// AdoptableCount reports how many rows in a gated set are actually adoptable.
// It exists so a test can assert the rule holds over a whole strip rather than
// row by row.
func AdoptableCount(proposals []ProposalRow) int {
	var n int
	for _, p := range proposals {
		if p.Adopt.Adoptable {
			n++
		}
	}
	return n
}
