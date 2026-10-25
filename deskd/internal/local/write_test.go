package local_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
	"github.com/medici-finance/assay/deskd/internal/local"
)

func openIndex(t *testing.T) *index.DB {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "deskd-index.db"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedOne(t *testing.T, db *index.DB) domain.EscalationID {
	t.Helper()
	repo, _ := adapter.ParseRepo("owner/alpha")
	id := domain.EscalationID("01ESCLOCAL0000000000000001")
	e := domain.Escalation{
		ID: id, Cell: "assay",
		Automation: domain.AutomationRef{Cell: "assay", Role: "review"},
		Kind:       domain.KindNeedsDecision, Provenance: domain.AgentRaised, Urgency: domain.UrgencyUrgent,
		Title: "seed", Situation: "s", FiledAt: time.Now().UTC().Add(-time.Hour), Repeat: 1,
		Fields: domain.FieldConfidence{},
	}
	ri := adapter.RawIssue{Number: 700, URL: "https://example.invalid/700"}
	if err := db.UpsertEscalations(repo, []domain.Escalation{e}, map[domain.EscalationID]adapter.RawIssue{id: ri}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return id
}

// TestReadAckEndpointsWriteLocalOnly proves the deskd write endpoints
// (the answer flow's target) set local read/ack state and touch nothing on the
// platform — the package holds no GitHub client, so the only effect a POST can
// have is on local_attention.
func TestReadAckEndpointsWriteLocalOnly(t *testing.T) {
	db := openIndex(t)
	id := seedOne(t, db)
	h := local.New(db).Handler()

	for _, verb := range []string{"read", "ack"} {
		req := httptest.NewRequest(http.MethodPost, "/escalations/"+string(id)+"/"+verb, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s → HTTP %d, want 200; body: %s", verb, rec.Code, rec.Body.String())
		}
	}

	read, acked, err := db.Attention(id)
	if err != nil {
		t.Fatalf("Attention: %v", err)
	}
	if read == nil || acked == nil {
		t.Fatalf("after read+ack POSTs: read=%v acked=%v, want both set", read, acked)
	}
}

// TestGetIsNotAllowed proves the endpoints are POST-only: a GET is a 405, so
// the local write surface is not a second read path and cannot be confused for
// one.
func TestGetIsNotAllowed(t *testing.T) {
	db := openIndex(t)
	id := seedOne(t, db)
	h := local.New(db).Handler()

	req := httptest.NewRequest(http.MethodGet, "/escalations/"+string(id)+"/read", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET on a write endpoint → HTTP %d, want 405", rec.Code)
	}
}

// TestMissingIDIsRejected guards the path-value plumbing.
func TestMissingIDIsRejected(t *testing.T) {
	db := openIndex(t)
	h := local.New(db).Handler()
	// An empty id segment does not match the {id} pattern, so the mux 404s;
	// the point is only that no panic and no write occurs.
	req := httptest.NewRequest(http.MethodPost, "/escalations//read", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("a missing id must not yield a 200 write")
	}
}
