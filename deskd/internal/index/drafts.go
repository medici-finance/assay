package index

import (
	"database/sql"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// This file is the answer-DRAFT store (the answer flow, §7.6 "draft survives failure").
// A draft is the human's own in-progress answer text — NOT derived from GitHub —
// so it lives in the local half of the store beside read/ack (ruling D7), under
// the local_ prefix, and is untouched by DropDerived. That is what makes
// Verify 9 hold: kill deskd mid-answer, restart (even with --rebuild-derived),
// and the draft is still there because it was never part of the cache.
//
// The write path (internal/answer) defines its own DraftStore interface; deskd
// adapts these methods to it. Keeping the record type here (rather than
// importing internal/answer) keeps the dependency pointing one way: the write
// path may know the store, the store need not know the write path.

// DraftRecord is one stored in-progress answer.
type DraftRecord struct {
	EscalationID domain.EscalationID
	Option       string
	Text         string
	SavedAt      time.Time
}

// SaveDraft upserts the draft for an escalation, overwriting any prior text.
func (d *DB) SaveDraft(r DraftRecord) error {
	saved := r.SavedAt
	if saved.IsZero() {
		saved = time.Now().UTC()
	}
	_, err := d.sql.Exec(`INSERT INTO local_drafts (escalation_id, option, text, saved_at)
	    VALUES (?,?,?,?)
	    ON CONFLICT(escalation_id) DO UPDATE SET option=excluded.option, text=excluded.text, saved_at=excluded.saved_at`,
		string(r.EscalationID), r.Option, r.Text, saved.UTC().Format(time.RFC3339Nano))
	return err
}

// LoadDraft returns the stored draft and whether one exists.
func (d *DB) LoadDraft(id domain.EscalationID) (DraftRecord, bool, error) {
	var opt, text, saved string
	err := d.sql.QueryRow(`SELECT option, text, saved_at FROM local_drafts WHERE escalation_id = ?`,
		string(id)).Scan(&opt, &text, &saved)
	if err == sql.ErrNoRows {
		return DraftRecord{}, false, nil
	}
	if err != nil {
		return DraftRecord{}, false, err
	}
	rec := DraftRecord{EscalationID: id, Option: opt, Text: text}
	if t, perr := time.Parse(time.RFC3339Nano, saved); perr == nil {
		rec.SavedAt = t
	}
	return rec, true, nil
}

// ClearDraft removes the draft once its answer has committed. A missing draft
// is already-clear, not an error.
func (d *DB) ClearDraft(id domain.EscalationID) error {
	_, err := d.sql.Exec(`DELETE FROM local_drafts WHERE escalation_id = ?`, string(id))
	return err
}
