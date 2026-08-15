package askassay

import (
	"strings"
	"testing"
)

func prStateQuestion(t *testing.T) Question {
	t.Helper()
	q, ok := Lookup("pr-action-count")
	if !ok {
		t.Fatal("pr-action-count is not declared")
	}
	return q
}

// TestJoinLosesNoRows — the join can lose information; it must not lose rows.
// Dropping the unjoinable ones is what silently shrinks a total.
func TestJoinLosesNoRows(t *testing.T) {
	actions := []ActionRow{
		{Repo: "example-org/loans", Number: 1, Action: "review"},
		{Repo: "example-org/loans", Number: 2, Action: "merge"},
		{Repo: "example-org/ledger", Number: 9, Action: "review"},
	}
	prs := []PRRow{
		{Repo: "example-org/loans", Number: 1, HeadSHA: "aaa1111"},
		{Repo: "example-org/loans", Number: 2, HeadSHA: ""}, // present, but no key
		{Repo: "example-org/loans", Number: 3, HeadSHA: "ccc3333"},
	}
	// The union of the two key sets is 4: loans#1 and loans#2 appear on both
	// sides, loans#3 only in the inventory, ledger#9 only in the actions.
	rows := Join(actions, prs)
	if got, want := len(rows), 4; got != want {
		t.Fatalf("join produced %d rows, want %d (the union of both key sets)", got, want)
	}

	byNum := map[int]JoinedRow{}
	for _, r := range rows {
		byNum[r.Number] = r
	}
	if got := byNum[1]; got.Resolution != Resolved || got.HeadSHA != "aaa1111" || got.Action != "review" {
		t.Errorf("row 1 = %+v, want a resolved row carrying both halves", got)
	}
	if got := byNum[2]; got.Resolution != UnresolvedNoHeadSHA {
		t.Errorf("row 2 = %+v, want %q", got, UnresolvedNoHeadSHA)
	}
	if got := byNum[3]; got.Resolution != UnresolvedNoActionRow {
		t.Errorf("row 3 = %+v, want %q", got, UnresolvedNoActionRow)
	}
	if got := byNum[9]; got.Resolution != UnresolvedNoPRRow {
		t.Errorf("row 9 = %+v, want %q", got, UnresolvedNoPRRow)
	}

	resolved, unresolved := Tally(rows)
	if resolved != 1 || unresolved != 3 {
		t.Errorf("Tally = (%d resolved, %d unresolved), want (1, 3)", resolved, unresolved)
	}
}

// TestJoinThatResolvesNothingIsCouldNotCheck — a count over an empty join is
// unmeasured, not zero.
func TestJoinThatResolvesNothingIsCouldNotCheck(t *testing.T) {
	q := prStateQuestion(t)
	rows := Join(
		[]ActionRow{{Repo: "example-org/loans", Number: 1, Action: "review"}},
		[]PRRow{{Repo: "example-org/ledger", Number: 7, HeadSHA: "ddd"}},
	)
	a := CountJoined(q, rows, func(r JoinedRow) bool { return r.Action == "review" }, testStamp())
	if a.State() != CouldNotCheck {
		t.Fatalf("state = %q, want %q", a.State(), CouldNotCheck)
	}
	if got := figureOf(t, a); got != FigureField {
		t.Fatalf("figure = %q, want %q — a join that resolved nothing measured nothing", got, FigureField)
	}
	if !strings.Contains(a.Reason(), "resolved 0 of") {
		t.Errorf("reason does not state the join failure: %q", a.Reason())
	}
}

// TestPartialJoinReportsItsRemainderAsABound — a partial answer is answerable,
// but only with the bound stated. The figure alone is a lower bound and is
// read as a total unless the answer says otherwise.
func TestPartialJoinReportsItsRemainderAsABound(t *testing.T) {
	q := prStateQuestion(t)
	rows := Join(
		[]ActionRow{
			{Repo: "example-org/loans", Number: 1, Action: "review"},
			{Repo: "example-org/loans", Number: 2, Action: "review"},
			{Repo: "example-org/loans", Number: 3, Action: "review"},
		},
		[]PRRow{{Repo: "example-org/loans", Number: 1, HeadSHA: "aaa"}},
	)
	a := CountJoined(q, rows, func(r JoinedRow) bool { return r.Action == "review" }, testStamp())
	if a.State() != Checked {
		t.Fatalf("state = %q, want %q — one row DID resolve", a.State(), Checked)
	}
	v, _ := a.Value()
	if v != 1 {
		t.Fatalf("figure = %d, want 1", v)
	}
	out := a.Render()
	if !strings.Contains(out, "PARTIAL JOIN") {
		t.Fatalf("a partial join rendered without naming itself:\n%s", out)
	}
	if !strings.Contains(out, "at least 1 and at most 3") {
		t.Errorf("the partial-join caveat does not bound the true figure:\n%s", out)
	}
}

// TestFullyResolvedJoinCarriesNoPartialCaveat is the positive control on the
// caveat: if it appears unconditionally it means nothing.
func TestFullyResolvedJoinCarriesNoPartialCaveat(t *testing.T) {
	q := prStateQuestion(t)
	rows := Join(
		[]ActionRow{{Repo: "example-org/loans", Number: 1, Action: "review"}},
		[]PRRow{{Repo: "example-org/loans", Number: 1, HeadSHA: "aaa"}},
	)
	a := CountJoined(q, rows, func(r JoinedRow) bool { return r.Action == "review" }, testStamp())
	if strings.Contains(a.Render(), "PARTIAL JOIN") {
		t.Fatalf("POSITIVE CONTROL FAILED: a fully resolved join claimed to be partial:\n%s", a.Render())
	}
}

// TestEmptyInputJoinIsAMeasuredZeroOnlyIfBothSidesWereRead — with no rows at
// all there is nothing to resolve, and the count is whatever the caller's
// classification of the probe said. This test pins the boundary so that the
// no-rows case is not accidentally routed through the resolved==0 refusal.
func TestEmptyInputJoinIsAMeasuredZeroOnlyIfBothSidesWereRead(t *testing.T) {
	q := prStateQuestion(t)
	a := CountJoined(q, nil, func(JoinedRow) bool { return true }, testStamp())
	if a.State() != Checked {
		t.Fatalf("state = %q, want %q — no rows on either side is a measured empty join, and the blindness question is settled upstream by Classify", a.State(), Checked)
	}
	if v, _ := a.Value(); v != 0 {
		t.Errorf("figure = %d, want 0", v)
	}
}

// TestNeitherVerbAloneFormsTheKey documents, executably, why the join exists:
// the action row type has no head SHA and the PR row type has no review state,
// mirroring the two board verbs.
func TestNeitherVerbAloneFormsTheKey(t *testing.T) {
	a := ActionRow{Repo: "r", Number: 1, Action: "review"}
	p := PRRow{Repo: "r", Number: 1, HeadSHA: "abc"}
	// Compile-time shape check: if a head SHA is ever added to ActionRow, or a
	// review state to PRRow, this test's premise changes and the join's
	// justification has to be rewritten rather than silently kept.
	if a.Repo == "" || p.HeadSHA == "" {
		t.Fatal("unreachable")
	}
	rows := Join([]ActionRow{a}, []PRRow{p})
	if len(rows) != 1 || !rows[0].OK() {
		t.Fatalf("a matched pair did not resolve: %+v", rows)
	}
	if rows[0].HeadSHA != "abc" || rows[0].Action != "review" {
		t.Errorf("the resolved row does not carry both halves of the key: %+v", rows[0])
	}
}
