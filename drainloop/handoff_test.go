package drainloop

import (
	"context"
	"errors"
	"testing"
)

// transactionalFake models a single coordination-store transaction: the generation
// compare and intent insertion happen in one CommitIntent, not two engine writes.
type transactionalFake struct {
	recoveryJournal
	current     string
	pending     map[string]Intent
	failCommit  bool
	failPending bool
	race        bool
}

func newOutbox() *transactionalFake {
	return &transactionalFake{current: "generation-1", pending: map[string]Intent{}}
}
func (j *transactionalFake) Generation(context.Context, string) (string, error) {
	return j.current, nil
}
func (j *transactionalFake) Pending(_ context.Context, run string) ([]Intent, error) {
	if j.failPending {
		return nil, errors.New("outbox unavailable")
	}
	var entries []Intent
	for _, e := range j.pending {
		if e.Event.RunID == run {
			entries = append(entries, e)
		}
	}
	return entries, nil
}
func (j *transactionalFake) CommitIntent(_ context.Context, in Intent) error {
	if j.race {
		j.current = "superseding-generation"
	}
	if j.failCommit || in.Event.Generation != j.current {
		return errors.New("transaction refused")
	}
	if old, ok := j.pending[in.Event.Key]; ok && old.Event.Generation != in.Event.Generation {
		return errors.New("pending generation changed")
	}
	j.pending[in.Event.Key] = in
	return j.recoveryJournal.Record(in.Event)
}
func (j *transactionalFake) Record(e Event) error {
	if err := j.recoveryJournal.Record(e); err != nil {
		return err
	}
	if e.Kind == "EFFECT-DONE" && !j.dropDone {
		if old, ok := j.pending[e.Key]; ok && old.Event.Generation == e.Generation {
			delete(j.pending, e.Key)
		}
	}
	return nil
}

// regression: F-ge04-outbox
func TestEffectIntentOutboxHandoff(t *testing.T) {
	t.Run("crash-after-intent-before-write", func(t *testing.T) {
		j := newOutbox()
		p := &providerFake{key: "operation"}
		p.beforeApply = func() { panic("process stopped after durable intent") }
		c, _ := effectConfig(t, []Item{effectItem()}, p, j)
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("crash not injected")
				}
			}()
			_ = Run(c)
		}()
		if len(j.pending) != 1 || len(p.records) != 0 {
			t.Fatal("intent or external state lost")
		}
		p.beforeApply = nil
		it := effectItem()
		it.AttemptID = "attempt-2"
		c, q := effectConfig(t, []Item{it}, p, j)
		runEffect(t, c, q, VerdictPass)
		if len(p.records) != 1 || len(j.pending) != 0 {
			t.Fatal("pending operation did not resume once")
		}
	})
	t.Run("commit-before-apply-and-resume", func(t *testing.T) {
		j := newOutbox()
		p := &providerFake{key: "operation", lostAck: true}
		p.beforeApply = func() {
			in, ok := j.pending[p.key]
			if !ok || in.Event.Generation != j.current || in.Event.Kind != "EFFECT-INTENT" {
				t.Fatal("Apply preceded atomic intent/generation commit")
			}
		}
		c, q := effectConfig(t, []Item{effectItem()}, p, j)
		c.RequireHandoff = true
		runEffect(t, c, q, VerdictHold)
		if len(j.pending) != 1 {
			t.Fatal("crash lost pending intent")
		}
		p.lostAck = false
		it := effectItem()
		it.AttemptID = "attempt-2"
		c, q = effectConfig(t, []Item{it}, p, j)
		runEffect(t, c, q, VerdictPass)
		if len(p.records) != 1 || len(j.pending) != 0 || j.kinds()["RECONCILE"] != 1 {
			t.Fatalf("records=%d pending=%d events=%v", len(p.records), len(j.pending), j.kinds())
		}
	})
	for _, found := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale-absence-holds", true: "stale-found-reconciles"}[found], func(t *testing.T) {
			j := newOutbox()
			it := effectItem()
			p := &providerFake{key: "operation"}
			j.pending[p.key] = Intent{Event: Event{Kind: "EFFECT-INTENT", ItemID: it.ID, RunID: it.RunID, Key: p.key, Generation: "old-generation"}}
			want := VerdictHold
			if found {
				p.records = []Receipt{{Key: p.key, Ref: "external-record"}}
				want = VerdictPass
			}
			c, q := effectConfig(t, []Item{it}, p, j)
			runEffect(t, c, q, want)
			count := 0
			if found {
				count = 1
			}
			if len(p.records) != count {
				t.Fatal("stale pending entry replayed")
			}
			if found && len(j.pending) != 0 {
				t.Fatalf("a found authoritative receipt must complete the older entry, pending=%d", len(j.pending))
			}
			if !found && len(j.pending) != 1 {
				t.Fatalf("a held stale entry must stay pending, pending=%d", len(j.pending))
			}
		})
	}
	t.Run("transaction-fails-closed", func(t *testing.T) {
		for _, race := range []bool{false, true} {
			j := newOutbox()
			j.failCommit = !race
			j.race = race
			p := &providerFake{key: "operation"}
			c, q := effectConfig(t, []Item{effectItem()}, p, j)
			runEffect(t, c, q, VerdictError)
			if len(p.records) != 0 {
				t.Fatal("failed transaction applied")
			}
		}
	})
	t.Run("unreadable-outbox-holds", func(t *testing.T) {
		j := newOutbox()
		j.failPending = true
		p := &providerFake{key: "operation"}
		c, q := effectConfig(t, []Item{effectItem()}, p, j)
		runEffect(t, c, q, VerdictHold)
		if len(p.records) != 0 {
			t.Fatal("unreadable outbox applied")
		}
	})
}

// regression: F-ge04-outbox
func TestOutboxRequiredNoFallback(t *testing.T) {
	p := &providerFake{key: "operation"}
	c, q := effectConfig(t, []Item{effectItem()}, p, &recoveryJournal{})
	c.RequireHandoff = true
	c.Loop = &noSelect{MemoryQueue: q, t: t}
	if err := Run(c); err == nil {
		t.Fatal("required handoff fell back")
	}
	if len(p.records) > 0 {
		t.Fatal("fallback applied")
	}
}

type noSelect struct {
	*MemoryQueue
	t *testing.T
}

func (q *noSelect) SelectQueue() ([]Item, error) {
	q.t.Fatal("selection ran before configuration refusal")
	return nil, nil
}
