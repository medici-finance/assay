package index_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// escFixture builds one open, attributed escalation for the fold fixtures.
func escFixture(id string, key int, filed time.Time, opts []domain.Option) (domain.Escalation, adapter.RawIssue) {
	e := domain.Escalation{
		ID:         domain.EscalationID(id),
		Cell:       "assay",
		Automation: domain.AutomationRef{Cell: "assay", Role: "review"},
		Kind:       domain.KindNeedsDecision,
		Provenance: domain.AgentRaised,
		Urgency:    domain.UrgencyUrgent,
		Title:      "escalation " + id,
		Situation:  "the same recurring decision",
		Options:    opts,
		Rev:        domain.OptionsRev(opts),
		FiledAt:    filed,
		Repeat:     1,
		Fields:     domain.FieldConfidence{},
	}
	ri := adapter.RawIssue{Number: key, URL: "https://example.invalid/" + id}
	return e, ri
}

func newIndex(t *testing.T) *index.DB {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "deskd-index.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestCollapseFoldFiveToOne is the attention layer Verify row 2: escalating the same
// (cell, automation, kind) 5× with an IDENTICAL option set folds to one row
// with repeat 5 and a non-null updated.
func TestCollapseFoldFiveToOne(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")
	opts := []domain.Option{{ID: "ship-partial", Label: "Ship the partial index", Consequence: "inbox live today"}}

	base := time.Now().UTC().Add(-time.Hour)
	var escs []domain.Escalation
	raw := map[domain.EscalationID]adapter.RawIssue{}
	for i := 0; i < 5; i++ {
		e, ri := escFixture(
			// lexicographically ascending ids so the earliest-filed is the survivor
			"01ESC0000000000000000000A"+string(rune('0'+i)),
			100+i,
			base.Add(time.Duration(i)*time.Minute),
			opts,
		)
		escs = append(escs, e)
		raw[e.ID] = ri
	}
	if err := db.UpsertEscalations(repo, escs, raw); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("5 same-key same-guard filings folded to %d rows, want 1", len(rows))
	}
	if rows[0].Repeat != 5 {
		t.Fatalf("folded repeat = %d, want 5", rows[0].Repeat)
	}
	if rows[0].Updated == nil {
		t.Fatalf("a folded row must carry a non-null updated (spec §1.1.3)")
	}
	// t_filed = min: a retrying loop must not reset its own clock (spec §3.1).
	if !rows[0].FiledAt.Equal(base) {
		t.Fatalf("survivor FiledAt = %v, want the earliest filing %v (t_filed = min)", rows[0].FiledAt, base)
	}
	// the surviving row still derives waiting (open escalation for its automation)
	if rows[0].Attention == nil || rows[0].Attention.State != domain.Waiting {
		t.Fatalf("folded open escalation should derive waiting, got %+v", rows[0].Attention)
	}
}

// TestCollapseChangedOptionsDoesNotMerge is the attention layer Verify row 3: filing #5
// carrying a CHANGED option set is a different decision (different OptionsRev)
// and never merges — the inbox shows 2 rows.
func TestCollapseChangedOptionsDoesNotMerge(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")
	optsA := []domain.Option{{ID: "ship-partial", Label: "Ship the partial index", Consequence: "inbox live today"}}
	optsB := []domain.Option{{ID: "wait-for-fix", Label: "Hold until the fix lands", Consequence: "nothing ships this week"}}

	base := time.Now().UTC().Add(-time.Hour)
	var escs []domain.Escalation
	raw := map[domain.EscalationID]adapter.RawIssue{}
	for i := 0; i < 5; i++ {
		opts := optsA
		if i == 4 {
			opts = optsB // the changed option set — a different decision
		}
		e, ri := escFixture(
			"01ESC0000000000000000000B"+string(rune('0'+i)),
			200+i,
			base.Add(time.Duration(i)*time.Minute),
			opts,
		)
		escs = append(escs, e)
		raw[e.ID] = ri
	}
	if err := db.UpsertEscalations(repo, escs, raw); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("4 same-guard + 1 changed-guard folded to %d rows, want 2", len(rows))
	}
	// the four identical filings fold to repeat 4; the changed one stands alone
	var foldedFour, alone bool
	for _, r := range rows {
		switch r.Repeat {
		case 4:
			foldedFour = true
		case 1:
			alone = true
		}
	}
	if !foldedFour || !alone {
		t.Fatalf("want one folded row (repeat 4) and one standalone (repeat 1); got repeats %d and %d",
			rows[0].Repeat, rows[1].Repeat)
	}
}

// TestUnattributableNeverFolds guards spec §1.1.3: unattributable filings (no
// collapse key) never merge, even with an identical body — the adapter cannot
// prove same-origin, so each keeps its own row.
func TestUnattributableNeverFolds(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")
	opts := []domain.Option{{ID: "a", Label: "A", Consequence: "c"}}
	base := time.Now().UTC().Add(-time.Hour)

	var escs []domain.Escalation
	raw := map[domain.EscalationID]adapter.RawIssue{}
	for i := 0; i < 3; i++ {
		e, ri := escFixture("01UNATTR000000000000000C"+string(rune('0'+i)), 300+i, base.Add(time.Duration(i)*time.Minute), opts)
		e.Automation = domain.AutomationRef{} // zero value ⇒ unattributable
		escs = append(escs, e)
		raw[e.ID] = ri
	}
	if err := db.UpsertEscalations(repo, escs, raw); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("unattributable filings folded to %d rows, want 3 (they must never merge)", len(rows))
	}
	for _, r := range rows {
		if r.Attention != nil {
			t.Fatalf("an unattributable row has no automation to derive a state for; want nil attention, got %+v", r.Attention)
		}
	}
}
