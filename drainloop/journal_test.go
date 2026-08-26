package drainloop

import "testing"

// A sliceJournal records every scheduling event, for tests and demos.
type sliceJournal struct{ events []Event }

func (j *sliceJournal) Record(e Event) error {
	j.events = append(j.events, e)
	return nil
}

func (j *sliceJournal) kinds() map[string]int {
	m := map[string]int{}
	for _, e := range j.events {
		m[e.Kind]++
	}
	return m
}

// When a Journal is configured, the engine emits the scheduling events (CLAIM, DISPATCH,
// LAND, IDLE) — the attribution record derived from the run, not narrated after it.
func TestJournalRecordsScheduling(t *testing.T) {
	q := NewMemoryQueue("t", []Item{{ID: "a"}, {ID: "b"}})
	c := mustFileClaim(t, t.TempDir())
	j := &sliceJournal{}
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true, Journal: j}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got := j.kinds()
	for _, kind := range []string{"CLAIM", "DISPATCH", "LAND", "IDLE"} {
		if got[kind] == 0 {
			t.Fatalf("journal missing %s events; recorded kinds=%v", kind, got)
		}
	}
	if got["CLAIM"] != 2 || got["DISPATCH"] != 2 || got["LAND"] != 2 {
		t.Fatalf("expected 2 each of CLAIM/DISPATCH/LAND for 2 items, got %v", got)
	}
}

// A nil Journal is the default and must be a no-op (no panic, drain unchanged).
func TestNilJournalIsNoOp(t *testing.T) {
	q := NewMemoryQueue("t", []Item{{ID: "a"}})
	c := mustFileClaim(t, t.TempDir())
	if err := Run(Config{Loop: q, Claimer: c, PoolSize: 1, StopWhenIdle: true}); err != nil {
		t.Fatalf("Run with nil Journal: %v", err)
	}
	if _, ok := q.Landed("a"); !ok {
		t.Fatal("item not landed with nil Journal")
	}
}
