package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskboardBoardGo is the ONE declared source of the ACTION vocabulary. This file's table
// is a derivation of it, and this test is the diff (derive-or-diff). It lives in the same
// Go module, so the tools/** path filter already triggers it — no cross-module registry
// row is needed.
const deskboardBoardGo = "../deskboard/board.go"

var actionConstRe = regexp.MustCompile(`(?m)^\s*act[A-Za-z]+\s*=\s*"([A-Z][A-Z0-9-]*)"`)

// deskboardActions parses the act* constant block out of deskboard's source. Parsing the
// SOURCE rather than restating the list is the point: a hand-kept second list is exactly
// the thing that let the pr-review-desk skill's nine-action description drift nine actions
// behind the board.
func deskboardActions(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Clean(deskboardBoardGo))
	if err != nil {
		t.Fatalf("cannot read %s: %v — this coupling test must not silently skip", deskboardBoardGo, err)
	}
	var out []string
	for _, m := range actionConstRe.FindAllStringSubmatch(string(b), -1) {
		out = append(out, m[1])
	}
	if len(out) < 9 {
		t.Fatalf("parsed only %d ACTION constants from %s — the parse broke, which would make this test vacuous", len(out), deskboardBoardGo)
	}
	sort.Strings(out)
	return out
}

// TestActionTableIsExhaustiveOverDeskboard is the anti-silent-cap gate. Every ACTION
// deskboard can emit must have a disposition here; an action added to the board without a
// decision here reddens rather than becoming an invisible blind spot.
func TestActionTableIsExhaustiveOverDeskboard(t *testing.T) {
	var missing []string
	for _, a := range deskboardActions(t) {
		if _, ok := actionTable[a]; !ok {
			missing = append(missing, a)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("deskboard emits ACTION(s) the reactor's table does not know: %s\n"+
			"Decide, do not silence: give each a Disposition in actionTable. An unhandled ACTION is a board "+
			"surface that disappears from the reactor without anything saying so.", strings.Join(missing, ", "))
	}
}

// TestActionTableHasNoPhantomActions is the other direction. A table entry for an action
// the board cannot emit is dead code that reads as coverage.
func TestActionTableHasNoPhantomActions(t *testing.T) {
	real := map[string]bool{}
	for _, a := range deskboardActions(t) {
		real[a] = true
	}
	var phantom []string
	for a := range actionTable {
		if !real[a] {
			phantom = append(phantom, a)
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Fatalf("actionTable carries ACTION(s) deskboard never emits: %s — dead entries read as coverage", strings.Join(phantom, ", "))
	}
}

// TestActionTableCoversMoreThanTheSkillsNine is the measured record of the drift this
// brief found: .claude/skills/pr-review-desk/SKILL.md boot step 2 describes NINE actions;
// the board computes more. A reactor written from the skill would have dropped the rest.
// If the board ever shrinks back to nine, this assertion is the thing that says so.
func TestActionTableCoversMoreThanTheSkillsNine(t *testing.T) {
	skillNine := []string{"NEEDS-REVIEW", "RE-REVIEW", "BLOCKED", "CHECK", "WAIT-CI", "CI-RED", "MERGE-CURR", "FLIP", "READY"}
	for _, a := range skillNine {
		if _, ok := actionTable[a]; !ok {
			t.Fatalf("the skill's documented ACTION %q is missing from the table", a)
		}
	}
	if len(actionTable) <= len(skillNine) {
		t.Fatalf("actionTable has %d entries, not more than the skill's %d — either the board shrank or the table stopped tracking it",
			len(actionTable), len(skillNine))
	}
}

// TestLookupUnknownActionIsUnverifiable — the fail-closed arm. An ACTION the table does
// not know is exit 6, never a silent no-op.
func TestLookupUnknownActionIsUnverifiable(t *testing.T) {
	_, err := LookupAction("SOME-NEW-STATE-DESKBOARD-GREW")
	if err == nil {
		t.Fatal("an unknown ACTION returned no error — the reactor would treat a new board state as nothing to do")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
}

// TestOnlyDispatchIsActionable pins the idle gate's input predicate: FLIP, SURFACE, WAIT
// and NO-OP rows are real work states but they do not consume a reviewer slot, and folding
// any of them into "actionable" would change what an idle claim means.
func TestOnlyDispatchIsActionable(t *testing.T) {
	for _, d := range []Disposition{DispositionFlip, DispositionSurface, DispositionWait, DispositionNoOp} {
		if d.Actionable() {
			t.Fatalf("%s reports Actionable() — only DISPATCH consumes a reviewer slot", d)
		}
	}
	if !DispositionDispatch.Actionable() {
		t.Fatal("DISPATCH does not report Actionable()")
	}
}

// TestWaitingStatesAreVisibleNotDropped — the brief's review point (d). Every non-verb
// disposition must still render a row.
func TestWaitingStatesAreVisibleNotDropped(t *testing.T) {
	b := healthyBoard(t, row(t, "BLOCKED", 1), row(t, "WAIT-CI", 2), row(t, "READY", 3), row(t, "MERGE-CURR", 4), row(t, "CI-RED", 5))
	var sb strings.Builder
	b.RenderRows(&sb)
	for _, want := range []string{"BLOCKED", "WAIT-CI", "READY", "MERGE-CURR", "CI-RED"} {
		if !strings.Contains(sb.String(), want) {
			t.Fatalf("row %s was dropped from the reactor's output — a waiting state must stay visible", want)
		}
	}
}

// TestSuspectApprovalSurfacesAndNeverFlips (#37) pins the one disposition in this table
// that a plausible-looking edit would get WRONG in the dangerous direction.
//
// SUSPECT-APPROVAL is BLOCKED's near-twin — same standing CHANGES_REQUESTED, same unhappy
// row — so the obvious filing is WAIT, next to BLOCKED. It must not be. BLOCKED is routine:
// the reviewer asked for changes and the worker owes a push. SUSPECT-APPROVAL is that row
// with an APPROVED laid on top at an UNCHANGED head, which is the observable signature of a
// forged verdict (deskboard suppresses the approval and ranks this action ABOVE BLOCKED
// precisely so it cannot hide inside it). Filing it as WAIT would undo that ranking one
// layer down, in the reactor, and the forgery would go unremarked. Nor may it ever carry a
// verb: the row means "an approval here cannot be trusted", so emitting the ready-flip off
// it is the exact bug #37 closes.
func TestSuspectApprovalSurfacesAndNeverFlips(t *testing.T) {
	r, err := LookupAction("SUSPECT-APPROVAL")
	if err != nil {
		t.Fatalf("SUSPECT-APPROVAL unknown to the reactor: %v", err)
	}
	if r.Disposition != DispositionSurface {
		t.Fatalf("SUSPECT-APPROVAL disposition = %s, want SURFACE — a suspected forged verdict "+
			"must reach a human, not be filed as a routine wait alongside BLOCKED (#37)", r.Disposition)
	}
	if r.Verb != "" {
		t.Fatalf("SUSPECT-APPROVAL carries verb %q — an approval this row exists to distrust "+
			"must never drive an outward verb, least of all the ready-flip (#37)", r.Verb)
	}
	if r.Disposition.Actionable() {
		t.Fatal("SUSPECT-APPROVAL must not consume a reviewer slot — it is a human's to read")
	}

	// And it must survive rendering: a row nobody sees is the same defect as no row.
	b := healthyBoard(t, row(t, "SUSPECT-APPROVAL", 1))
	var sb strings.Builder
	b.RenderRows(&sb)
	if !strings.Contains(sb.String(), "SUSPECT-APPROVAL") {
		t.Fatal("SUSPECT-APPROVAL was dropped from the reactor's output — the one row that must not vanish")
	}
}

// TestFlipIsTheOnlyOutwardVerbAndItIsNotAMerge — the desk does not merge, and MERGE-NOW
// must never acquire a verb here.
func TestFlipIsTheOnlyOutwardVerbAndItIsNotAMerge(t *testing.T) {
	verbs := map[string]bool{}
	for a, r := range actionTable {
		if r.Verb == "" {
			continue
		}
		verbs[r.Verb] = true
		if strings.Contains(strings.ToLower(r.Verb), "merge") {
			t.Fatalf("ACTION %q carries verb %q — the review desk never merges", a, r.Verb)
		}
	}
	if r := actionTable["MERGE-NOW"]; r.Verb != "" {
		t.Fatalf("MERGE-NOW carries verb %q — the merge is always the human's", r.Verb)
	}
	delete(verbs, "review")
	delete(verbs, "ready")
	if len(verbs) != 0 {
		t.Fatalf("unexpected outward verbs in the action table: %v (only review dispatch and the ready-flip are in scope)", verbs)
	}
}
