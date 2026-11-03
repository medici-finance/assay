package index_test

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// TestReadAckSetsLocalStateZeroPlatformWrites is the attention layer Verify row 6 (D7):
// read and ack set local state, and NOTHING on the platform changes. The
// platform half of the proof is structural — internal/index holds no GitHub
// client and imports no transport, so there is no reachable write — and this
// test proves the local half: the timestamps land in local_attention, the
// derived cache is byte-for-byte untouched, and the read/ack survive a cache
// rebuild (they are the FIRST non-derivable deskd state, ruling D7).
func TestReadAckSetsLocalStateZeroPlatformWrites(t *testing.T) {
	db := newIndex(t)
	repo, _ := adapter.ParseRepo("owner/alpha")

	opts := []domain.Option{{ID: "a", Label: "A", Consequence: "c"}}
	e, ri := escFixture("01ESCREADACK00000000000001", 500, time.Now().UTC().Add(-time.Hour), opts)
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Baseline: unread, unacked, and a snapshot of the derived cache size.
	before := mustInboxOne(t, db)
	if before.Read != nil || before.Acked != nil {
		t.Fatalf("fresh escalation should be unread and unacked, got read=%v acked=%v", before.Read, before.Acked)
	}
	derivedBefore := derivedRowCount(t, db)

	// The write: set read then ack. No network, no GitHub client anywhere in
	// this call graph.
	now := time.Now().UTC()
	if err := db.SetRead(e.ID, index.DefaultActor(), now); err != nil {
		t.Fatalf("SetRead: %v", err)
	}
	if err := db.SetAck(e.ID, index.DefaultActor(), now); err != nil {
		t.Fatalf("SetAck: %v", err)
	}

	// Local state took.
	after := mustInboxOne(t, db)
	if after.Read == nil || after.Acked == nil {
		t.Fatalf("after read+ack: read=%v acked=%v, want both non-null", after.Read, after.Acked)
	}

	// Zero platform effect: the derived cache is unchanged in size, because
	// the setters cannot reach a derived table.
	if got := derivedRowCount(t, db); got != derivedBefore {
		t.Fatalf("read/ack changed the derived cache row count %d → %d; setters must touch local state ONLY", derivedBefore, got)
	}

	// First-write-wins: a later ack does not move the timestamp — the ack
	// clock measures the FIRST park, never the latest.
	firstAck := *after.Acked
	if err := db.SetAck(e.ID, index.DefaultActor(), now.Add(time.Hour)); err != nil {
		t.Fatalf("second SetAck: %v", err)
	}
	again := mustInboxOne(t, db)
	if !again.Acked.Equal(firstAck) {
		t.Fatalf("a second ack moved the timestamp %v → %v; the first ack is the one that counts", firstAck, *again.Acked)
	}

	// D7 durability: the read/ack survive a derived-cache rebuild.
	if err := db.DropDerived(); err != nil {
		t.Fatalf("DropDerived: %v", err)
	}
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{e.ID: ri}); err != nil {
		t.Fatalf("re-upsert after rebuild: %v", err)
	}
	survived := mustInboxOne(t, db)
	if survived.Read == nil || survived.Acked == nil {
		t.Fatalf("read/ack did not survive a cache rebuild; they must, being local not derived (ruling D7)")
	}
}

func mustInboxOne(t *testing.T, db *index.DB) index.Row {
	t.Helper()
	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want exactly one row, got %d", len(rows))
	}
	return rows[0]
}

func derivedRowCount(t *testing.T, db *index.DB) int {
	t.Helper()
	n, err := db.QueryIntForTest(`SELECT COUNT(*) FROM derived_escalations`)
	if err != nil {
		t.Fatalf("count derived_escalations: %v", err)
	}
	return n
}
