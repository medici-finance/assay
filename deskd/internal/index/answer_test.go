package index_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// TestUpsertEscalationsPersistsAnswer is the service contract increment 3 task 6's
// storage half: a derived Answer survives the round trip through
// UpsertEscalations and Inbox(includeClosed=true), and the row is marked
// Closed so a caller can tell it apart from a live open escalation.
func TestUpsertEscalationsPersistsAnswer(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")

	at := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	opt := domain.OptionID("opt-a")
	e, ri := escFixture("01ESCANSWERED0000000000001", 700, at.Add(-time.Hour), nil)
	e.Answer = &domain.Answer{
		Option:     &opt,
		OptionsRev: "deadbeef",
		By:         domain.Actor{Name: "operator"},
		At:         at,
	}
	ri.Closed = true

	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	rows, err := db.Inbox("", true)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	got := rows[0]
	if !got.Closed {
		t.Fatalf("row.Closed = false, want true")
	}
	if got.Answer == nil {
		t.Fatalf("row.Answer = nil, want the derived answer")
	}
	if got.Answer.By.Name != "operator" {
		t.Errorf("Answer.By.Name = %q, want operator", got.Answer.By.Name)
	}
	if got.Answer.Option == nil || *got.Answer.Option != opt {
		t.Errorf("Answer.Option = %v, want %v", got.Answer.Option, opt)
	}
	if got.Answer.OptionsRev != "deadbeef" {
		t.Errorf("Answer.OptionsRev = %q, want deadbeef", got.Answer.OptionsRev)
	}
	if !got.Answer.At.Equal(at) {
		t.Errorf("Answer.At = %v, want %v", got.Answer.At, at)
	}

	// The default (includeClosed=false) read must still exclude it — Task 6
	// changes what a caller CAN see, never the default open-inbox behaviour.
	open, err := db.Inbox("", false)
	if err != nil {
		t.Fatalf("inbox (open only): %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("open-only inbox returned %d rows, want 0 (the escalation is closed)", len(open))
	}
}

// TestUpsertEscalationsNoAnswerIsNil pins the NULL-flag rule: an escalation
// with no derived Answer must round-trip as row.Answer == nil, never a
// zero-value Answer that would misread as "answered at the epoch".
func TestUpsertEscalationsNoAnswerIsNil(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")
	e, ri := escFixture("01ESCNOANSWER000000000001", 701, time.Now().UTC(), nil)

	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got := mustInboxOne(t, db)
	if got.Answer != nil {
		t.Fatalf("row.Answer = %v, want nil", got.Answer)
	}
	if got.Closed {
		t.Fatalf("row.Closed = true, want false")
	}
}

// TestUpsertOneDoesNotWipeSiblings is the refresh nudge's storage contract
// (the service contract increment 3, task 2): re-deriving ONE issue must leave
// every other escalation of its repo untouched, unlike UpsertEscalations
// (which replaces the whole repo slice every poll).
func TestUpsertOneDoesNotWipeSiblings(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")

	e1, ri1 := escFixture("01ESCSIBLING0000000000001", 801, time.Now().UTC(), nil)
	e2, ri2 := escFixture("01ESCSIBLING0000000000002", 802, time.Now().UTC(), nil)
	// escFixture builds foldable rows by default (same cell/automation/kind/
	// situation, no options) — give e2 a different automation so the two
	// stay SEPARATE rows; this test is about UpsertOne's storage isolation,
	// not the collapse fold.
	e2.Automation = domain.AutomationRef{Cell: "assay", Role: "verify"}
	raw := map[domain.EscalationID]adapter.RawIssue{e1.ID: ri1, e2.ID: ri2}
	if err := db.UpsertEscalations(repo, []domain.Escalation{e1, e2}, raw); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}

	// Refresh e1 alone, with an updated title — simulating a re-derive that
	// found the title changed.
	e1.Title = "retitled by refresh"
	if err := db.UpsertOne(repo, e1, ri1); err != nil {
		t.Fatalf("UpsertOne: %v", err)
	}

	rows, err := db.Inbox("", true)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows after UpsertOne, want 2 (siblings must survive)", len(rows))
	}
	byID := map[domain.EscalationID]index.Row{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if byID[e1.ID].Title != "retitled by refresh" {
		t.Errorf("e1.Title = %q, want the refreshed title", byID[e1.ID].Title)
	}
	if byID[e2.ID].Title == "retitled by refresh" {
		t.Errorf("e2 (the sibling) was overwritten by the refresh of e1")
	}
}

// TestUpsertOneClosesAndCarriesAnswer: a refresh that finds the issue now
// closed, with an answer marker in the newly-polled comments, persists both —
// this is the storage path Verify row 11 (the outcome derived FROM GITHUB by
// polling) depends on for a targeted refresh, not just a full cycle.
func TestUpsertOneClosesAndCarriesAnswer(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")

	e, ri := escFixture("01ESCREFRESHCLOSE00000001", 900, time.Now().UTC(), nil)
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}

	// Simulate the refresh's re-derive: the issue is now closed and answered.
	ri.Closed = true
	ri.Comments = []adapter.RawComment{
		{Body: "<!-- assay:answer v1 login=operator option=opt-a options_rev=rev1 -->", Login: "operator", CreatedAt: time.Now().UTC()},
	}
	ans, ok := adapter.DeriveAnswer(ri, adapter.AnswerVerifier{})
	if !ok {
		t.Fatalf("DeriveAnswer did not find the marker in the fixture")
	}
	e.Answer = ans

	if err := db.UpsertOne(repo, e, ri); err != nil {
		t.Fatalf("UpsertOne: %v", err)
	}

	rows, err := db.Inbox("", true)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly one row, got %d", len(rows))
	}
	got := rows[0]
	if !got.Closed {
		t.Fatalf("row.Closed = false after refresh, want true")
	}
	if got.Answer == nil || got.Answer.OptionsRev != "rev1" {
		t.Fatalf("row.Answer = %v, want a derived answer with rev1", got.Answer)
	}
}

// TestUnverifiedAnswerPersists: an answered-unverified derivation (gap-3 item
// (5)) survives the store as unverified — it never comes back out of the
// cache looking like a verified human answer.
func TestUnverifiedAnswerPersists(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	e, ri := escFixture("01ESCUNVERIFIED00000000001", 710, at.Add(-time.Hour), nil)
	e.Answer = &domain.Answer{By: domain.Actor{Name: "gateway-app[bot]", IsBot: true}, At: at, Unverified: true}
	ri.Closed = true
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	rows, err := db.Inbox("", true)
	if err != nil || len(rows) != 1 {
		t.Fatalf("inbox = %d rows, %v", len(rows), err)
	}
	got := rows[0].Answer
	if got == nil || !got.Unverified || got.Option != nil || got.By.Name != "gateway-app[bot]" || !got.By.IsBot {
		t.Fatalf("stored answer = %+v, want answered-unverified by the bot", got)
	}

	// The refresh path (UpsertOne) carries the flag too — and clears it when
	// the re-derivation verifies.
	e.Answer = &domain.Answer{By: domain.Actor{Name: "operator"}, At: at, OptionsRev: "rev1"}
	if err := db.UpsertOne(repo, e, ri); err != nil {
		t.Fatalf("UpsertOne: %v", err)
	}
	rows, _ = db.Inbox("", true)
	if got := rows[0].Answer; got == nil || got.Unverified {
		t.Fatalf("after a verified re-derive, answer = %+v", got)
	}
	e.Answer = &domain.Answer{By: domain.Actor{Name: "gateway-app[bot]", IsBot: true}, At: at, Unverified: true}
	if err := db.UpsertOne(repo, e, ri); err != nil {
		t.Fatalf("UpsertOne: %v", err)
	}
	rows, _ = db.Inbox("", true)
	if got := rows[0].Answer; got == nil || !got.Unverified {
		t.Fatalf("UpsertOne dropped the unverified flag: %+v", got)
	}
}

// TestStaleDerivedCacheDropped: an index written by a build that predates
// answered-unverified (no answer_unverified column) holds answers derived
// WITHOUT poll-side verification. Opening it drops the derived cache — to be
// re-derived, verified, on the next poll — and leaves local state alone.
func TestStaleDerivedCacheDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deskd-index.db")
	db, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	repo, _ := adapter.ParseRepo("owner/alpha")
	opt := domain.OptionID("opt-a")
	e, ri := escFixture("01ESCSTALECACHE00000000001", 720, time.Now().UTC(), nil)
	e.Answer = &domain.Answer{Option: &opt, OptionsRev: "rev1", By: domain.Actor{Name: "operator", IsBot: true}, At: time.Now().UTC()}
	ri.Closed = true
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Rewind the schema to the pre-#222 shape and leave a local row behind.
	if err := db.ExecForTest(`ALTER TABLE derived_escalations DROP COLUMN answer_unverified`); err != nil {
		t.Fatalf("rewind schema: %v", err)
	}
	if err := db.ExecForTest(`INSERT INTO local_attention (escalation_id, actor) VALUES (?, 'operator')`, string(e.ID)); err != nil {
		t.Fatalf("seed local row: %v", err)
	}
	db.Close()

	db, err = index.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if n, err := db.QueryIntForTest(`SELECT COUNT(*) FROM derived_escalations`); err != nil || n != 0 {
		t.Fatalf("stale derived rows after reopen = %d (%v), want 0", n, err)
	}
	if n, err := db.QueryIntForTest(`SELECT COUNT(*) FROM pragma_table_info('derived_escalations') WHERE name='answer_unverified'`); err != nil || n != 1 {
		t.Fatalf("answer_unverified column present = %d (%v), want 1", n, err)
	}
	if n, err := db.QueryIntForTest(`SELECT COUNT(*) FROM local_attention`); err != nil || n != 1 {
		t.Fatalf("local rows after reopen = %d (%v), want 1 (local state survives)", n, err)
	}

	// A current index is NOT dropped on reopen.
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	db.Close()
	db2, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	if n, _ := db2.QueryIntForTest(`SELECT COUNT(*) FROM derived_escalations`); n != 1 {
		t.Fatalf("current-schema rows after reopen = %d, want 1", n)
	}
}
