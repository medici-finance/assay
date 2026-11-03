package index

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestDraftRoundTrip covers save / load / overwrite / clear.
func TestDraftRoundTrip(t *testing.T) {
	db := openTestDB(t)
	id := domain.EscalationID("01ESCALATION")

	if _, ok, err := db.LoadDraft(id); ok || err != nil {
		t.Fatalf("empty store: ok=%v err=%v", ok, err)
	}

	if err := db.SaveDraft(DraftRecord{EscalationID: id, Option: "opt-a", Text: "first"}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.LoadDraft(id)
	if err != nil || !ok {
		t.Fatalf("load after save: ok=%v err=%v", ok, err)
	}
	if got.Text != "first" || got.Option != "opt-a" {
		t.Fatalf("unexpected draft: %+v", got)
	}

	// overwrite
	if err := db.SaveDraft(DraftRecord{EscalationID: id, Option: "opt-b", Text: "second", SavedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	got, _, _ = db.LoadDraft(id)
	if got.Text != "second" || got.Option != "opt-b" {
		t.Fatalf("overwrite failed: %+v", got)
	}

	if err := db.ClearDraft(id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := db.LoadDraft(id); ok {
		t.Fatal("draft present after clear")
	}
	// clearing a missing draft is not an error
	if err := db.ClearDraft(id); err != nil {
		t.Fatalf("clear of missing draft errored: %v", err)
	}
}

// TestDraftSurvivesRebuild is the storage side of Verify 9: a draft is LOCAL
// (not derived), so dropping the derived cache leaves it — kill deskd mid-answer,
// restart with --rebuild-derived, and the draft is still there.
func TestDraftSurvivesRebuild(t *testing.T) {
	db := openTestDB(t)
	id := domain.EscalationID("01KEEPME")
	if err := db.SaveDraft(DraftRecord{EscalationID: id, Text: "half-typed answer"}); err != nil {
		t.Fatal(err)
	}
	if err := db.DropDerived(); err != nil {
		t.Fatalf("DropDerived: %v", err)
	}
	got, ok, err := db.LoadDraft(id)
	if err != nil || !ok {
		t.Fatalf("draft did not survive a derived rebuild: ok=%v err=%v", ok, err)
	}
	if got.Text != "half-typed answer" {
		t.Fatalf("draft text lost across rebuild: %q", got.Text)
	}
}
