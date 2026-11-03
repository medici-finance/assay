package answer

import (
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// Draft is an in-progress answer the human has typed but not committed. It
// survives a deskd restart (Verify 9, §7.6 "draft survives failure"): a draft
// is saved as the human edits and cleared only on a successful commit, so a
// crash mid-answer — or a failed/raced commit — leaves the text to reopen to.
type Draft struct {
	EscalationID domain.EscalationID `json:"escalation_id"`
	// Option is the staged option id, if the human staged one before typing.
	Option string `json:"option,omitempty"`
	// Text is the free-text half of the answer.
	Text string `json:"text"`
	// SavedAt is when the draft was last written.
	SavedAt time.Time `json:"saved_at"`
}

// DraftStore persists answer drafts durably. It is an interface so the Service
// stays testable with an in-memory fake; deskd backs it with the index's local
// (non-derived, survives-rebuild) store, the same place read/ack timestamps
// live (ruling D7). Nothing here is derived from GitHub — a draft is the
// human's own state, so it lives beside the local_attention table, not in the
// derived cache.
type DraftStore interface {
	// SaveDraft writes (or overwrites) the draft for an escalation.
	SaveDraft(d Draft) error
	// LoadDraft returns the stored draft and whether one exists.
	LoadDraft(id domain.EscalationID) (Draft, bool, error)
	// ClearDraft removes the draft once its answer has committed.
	ClearDraft(id domain.EscalationID) error
}

// AckRecorder records the LOCAL ack timestamp (spec §1.1.2, ruling D7). The
// ack write-through posts a operator 👀 reaction AND records the local timestamp;
// the two are separate stores because the reaction is platform state and the
// timestamp is deskd's own. deskd backs this with the index.
type AckRecorder interface {
	SetAck(id domain.EscalationID, actor string, at time.Time) error
}

// MemoryDraftStore is an in-memory DraftStore for tests. It is NOT durable, so
// it is never used in deskd — the real store is the index's local table.
type MemoryDraftStore struct {
	drafts map[domain.EscalationID]Draft
}

// NewMemoryDraftStore builds an empty in-memory store.
func NewMemoryDraftStore() *MemoryDraftStore {
	return &MemoryDraftStore{drafts: map[domain.EscalationID]Draft{}}
}

func (m *MemoryDraftStore) SaveDraft(d Draft) error {
	m.drafts[d.EscalationID] = d
	return nil
}

func (m *MemoryDraftStore) LoadDraft(id domain.EscalationID) (Draft, bool, error) {
	d, ok := m.drafts[id]
	return d, ok, nil
}

func (m *MemoryDraftStore) ClearDraft(id domain.EscalationID) error {
	delete(m.drafts, id)
	return nil
}
