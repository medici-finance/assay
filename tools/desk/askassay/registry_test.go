package askassay

import (
	"strings"
	"testing"
)

// TestEveryDeclaredQuestionIsServable — the registry is the pane's whole
// vocabulary. A malformed row is a figure that renders with UNDECLARED in it.
func TestEveryDeclaredQuestionIsServable(t *testing.T) {
	if len(Questions()) == 0 {
		t.Fatal("the registry is empty — a pane with no declared questions answers nothing, which is not the same as answering safely")
	}
	for _, q := range Questions() {
		if err := q.Validate(); err != nil {
			t.Errorf("%s: %v", q.ID, err)
		}
		if q.ID != strings.TrimSpace(q.ID) || strings.ContainsAny(q.ID, " \t") {
			t.Errorf("%q: question IDs are rendered tokens and must not contain whitespace", q.ID)
		}
	}
}

// TestEveryQuestionCarriesItsClassCaveats — the caveats are the measured ways
// each class of number lies, and they are attached BY CLASS rather than by
// hand, so a new question cannot be authored without them. This test holds the
// attachment true all the way to the rendered line: a caveat that exists in a
// map but never reaches the operator is not a caveat.
func TestEveryQuestionCarriesItsClassCaveats(t *testing.T) {
	for _, q := range Questions() {
		rendered := Computed(q, 1, testStamp()).Render()
		for _, want := range requiredCaveats[q.Class] {
			if !strings.Contains(rendered, want) {
				t.Errorf("%s (class %s): a mandatory caveat for its class does not reach the rendered answer", q.ID, q.Class)
			}
		}
		if len(q.allCaveats()) == 0 {
			t.Errorf("%s: renders with no caveats at all", q.ID)
		}
	}
}

// TestEveryClassDeclaresItsCaveats — the regression this guards is a NEW class
// added with an empty caveat list, which would silently produce bare figures
// for a whole family of questions.
func TestEveryClassDeclaresItsCaveats(t *testing.T) {
	all := []Class{ClassInventory, ClassBoard, ClassCompletion, ClassFlow, ClassPRState}
	for _, c := range all {
		got, ok := requiredCaveats[c]
		if !ok {
			t.Errorf("class %q has no entry in requiredCaveats", c)
			continue
		}
		if len(got) == 0 {
			t.Errorf("class %q declares an empty caveat list — every class here exists because its numbers lie in a specific way, and the way has to be written down", c)
		}
	}
	if len(requiredCaveats) != len(all) {
		t.Errorf("requiredCaveats holds %d classes but %d are declared as constants — a class in one and not the other is a question family with no floor", len(requiredCaveats), len(all))
	}
	// Positive control: an undeclared class is refused, and produces no
	// caveats at all — which is exactly why it must not be servable.
	bogus := Question{ID: "control", Text: "control", Class: Class("made-up"),
		Source: Source{Cmd: "c", Probe: "p", Window: "w", Limit: "l — none"}}
	if err := bogus.Validate(); err == nil {
		t.Fatal("POSITIVE CONTROL FAILED: a question in an undeclared class validated clean")
	}
	if len(bogus.allCaveats()) != 0 {
		t.Fatal("POSITIVE CONTROL FAILED: an undeclared class produced caveats from somewhere")
	}
}

// TestEverySourceStatesItsLimit — no silent caps, at the registry level.
func TestEverySourceStatesItsLimit(t *testing.T) {
	for _, q := range Questions() {
		if err := q.Source.Validate(); err != nil {
			t.Errorf("%s: %v", q.ID, err)
			continue
		}
		lim := strings.ToLower(q.Source.Limit)
		// A limit of "none" is legitimate, but it has to be argued: a bare
		// "none" is a shrug wearing the shape of a declaration.
		if strings.HasPrefix(lim, "none") && !strings.Contains(lim, "—") && !strings.Contains(lim, "-") {
			t.Errorf("%s: declares limit %q — an uncapped source must say WHY it cannot truncate", q.ID, q.Source.Limit)
		}
	}
	// Positive control.
	for _, bad := range []Source{
		{Cmd: "c", Probe: "p", Window: "w"},
		{Cmd: "c", Probe: "p", Limit: "l"},
		{Cmd: "c", Window: "w", Limit: "l"},
		{Probe: "p", Window: "w", Limit: "l"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("POSITIVE CONTROL FAILED: an incomplete source validated clean: %+v", bad)
		}
	}
}

// TestOneNumberOneSource — derive-or-diff. Two questions that answer the same
// thing from two commands is how two panes disagree about one fact.
func TestOneNumberOneSource(t *testing.T) {
	type fact struct{ probe, cmd string }
	byProbe := map[fact][]string{}
	for _, q := range Questions() {
		f := fact{probe: q.Source.Probe, cmd: q.Source.Cmd}
		byProbe[f] = append(byProbe[f], q.ID)
	}
	for f, ids := range byProbe {
		if len(ids) > 1 {
			t.Errorf("questions %v all derive %q from %q — one number, one source", ids, f.probe, f.cmd)
		}
	}
}

// TestEmptyIsBlindByDefault — the default must be the safe one. A question
// that treats an empty payload as a zero has to say why, in writing.
func TestEmptyIsBlindByDefault(t *testing.T) {
	for _, q := range Questions() {
		if q.EmptyMeansZero && strings.TrimSpace(q.EmptyRationale) == "" {
			t.Errorf("%s: declares an empty payload to be a real zero with no rationale", q.ID)
		}
	}
	// The zero value of Question must be blind, not zero-meaning.
	var zero Question
	if zero.EmptyMeansZero {
		t.Fatal("the zero value of Question treats an empty result as a real zero — the unsafe default")
	}
	// Positive control.
	q := wellFormed()
	q.EmptyMeansZero = true
	if err := q.Validate(); err == nil {
		t.Fatal("POSITIVE CONTROL FAILED: EmptyMeansZero with no rationale validated clean")
	}
}

// TestCompletionAnswersCarryTheUnbackedCaveat — "verified does not mean
// verified" is the caveat most likely to be dropped, because it is the one
// that makes the good news worse.
func TestCompletionAnswersCarryTheUnbackedCaveat(t *testing.T) {
	q, ok := Lookup("completion-count")
	if !ok {
		t.Fatal("completion-count is not declared")
	}
	a := Computed(q, 141, testStamp())
	out := a.Render()
	if !strings.Contains(out, "SELF-ATTRIBUTED") {
		t.Errorf("a completion figure renders without the self-attribution caveat:\n%s", out)
	}
	if !strings.Contains(out, "statusgen --corroborate") {
		t.Errorf("the self-attribution caveat does not name the corroboration probe:\n%s", out)
	}
}

// TestBoardCountsCarryTheBoardLiesCaveat — a count of todo rows is not a count
// of unstarted work, and this is the figure most often quoted as if it were.
func TestBoardCountsCarryTheBoardLiesCaveat(t *testing.T) {
	q, ok := Lookup("brief-status-count")
	if !ok {
		t.Fatal("brief-status-count is not declared")
	}
	out := Computed(q, 41, testStamp()).Render()
	if !strings.Contains(out, "not work state") {
		t.Errorf("a board count renders without the board-lies caveat:\n%s", out)
	}
}

// TestPRAnswersNameTheMissingJoinKey — neither board verb alone forms
// (repo, pr, head, verb), and the answer has to say which half it is missing.
func TestPRAnswersNameTheMissingJoinKey(t *testing.T) {
	act, ok := Lookup("pr-action-count")
	if !ok {
		t.Fatal("pr-action-count is not declared")
	}
	if !strings.Contains(Computed(act, 5, testStamp()).Render(), "NO head SHA") {
		t.Error("the action-count answer does not state that its rows carry no head SHA")
	}
	inv, ok := Lookup("pr-inventory-count")
	if !ok {
		t.Fatal("pr-inventory-count is not declared")
	}
	if !strings.Contains(Computed(inv, 5, testStamp()).Render(), "NO review state") {
		t.Error("the PR-inventory answer does not state that its rows carry no review state")
	}
}

// TestUnknownQuestionIsRefusedNotGuessed — the pane's answer to a question it
// has no source for is a stamped could-not-check, never a number.
func TestUnknownQuestionIsRefusedNotGuessed(t *testing.T) {
	if _, ok := Lookup("how-many-angels"); ok {
		t.Fatal("an undeclared question resolved")
	}
	a := Unanswerable("how-many-angels", "how many angels fit on a pin?", testStamp())
	if a.State() != CouldNotCheck {
		t.Fatalf("state = %q, want %q", a.State(), CouldNotCheck)
	}
	if !strings.Contains(a.Reason(), "no declared source") {
		t.Errorf("refusal does not say the source is missing: %q", a.Reason())
	}
	if got := figureOf(t, a); got != FigureField {
		t.Errorf("figure = %q, want %q", got, FigureField)
	}
}

// TestEveryRegistrySourceIsAReadOnlyProbe — the registry cannot declare a
// source the read-only guard would refuse. This is the seam where a write
// would enter: a new question naming a mutating command.
func TestEveryRegistrySourceIsAReadOnlyProbe(t *testing.T) {
	for _, q := range Questions() {
		argv := naiveArgv(q.Source.Cmd)
		if err := GuardReadOnly(argv); err != nil {
			t.Errorf("%s: declared source is not a permitted read-only probe: %v\n  cmd: %s", q.ID, err, q.Source.Cmd)
		}
	}
}

// naiveArgv splits a declared command on whitespace. It is deliberately crude:
// it is used only to feed the guard the binary and verb positions, which is
// all the guard keys on.
func naiveArgv(cmd string) []string {
	return strings.Fields(cmd)
}
