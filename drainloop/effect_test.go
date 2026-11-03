package drainloop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// providerFake deliberately DOES NOT dedupe Apply: the reconciliation tests must
// catch a second physical write even without provider-side protection.
type providerFake struct {
	key         string
	records     []Receipt
	lookupErr   error
	applyErr    error
	lostAck     bool
	beforeApply func()
}

func (p *providerFake) Key() string { return p.key }
func (p *providerFake) Receipt(_ context.Context, key string) (Receipt, bool, error) {
	if p.lookupErr != nil {
		return Receipt{}, false, p.lookupErr
	}
	for _, r := range p.records {
		if r.Key == key {
			return r, true, nil
		}
	}
	return Receipt{}, false, nil
}
func (p *providerFake) Apply(context.Context) (Receipt, error) {
	if p.beforeApply != nil {
		p.beforeApply()
	}
	if p.applyErr != nil {
		return Receipt{}, p.applyErr
	}
	r := Receipt{Key: p.key, Ref: "example-org/record", At: time.Now()}
	p.records = append(p.records, r)
	if p.lostAck {
		return Receipt{}, errors.New("ack lost after external commit")
	}
	return r, nil
}

type recoveryJournal struct {
	sliceJournal
	fail     string
	dropDone bool
}

func (j *recoveryJournal) Record(e Event) error {
	if e.Kind == j.fail {
		return errors.New("journal unavailable")
	}
	if e.Kind == "EFFECT-DONE" && j.dropDone {
		return nil
	}
	return j.sliceJournal.Record(e)
}
func effectItem() Item { return Item{ID: "artifact", RunID: "run", AttemptID: "attempt-1"} }
func effectConfig(t *testing.T, items []Item, p *providerFake, j Journal) (Config, *MemoryQueue) {
	t.Helper()
	q := NewMemoryQueue("effect", items)
	return Config{Loop: q, Claimer: mustFileClaim(t, t.TempDir()), PoolSize: 1, StopWhenIdle: true, Journal: j, Effects: func(Item) ([]Effect, error) { return []Effect{p}, nil }}, q
}
func runEffect(t *testing.T, c Config, q *MemoryQueue, want Verdict) {
	t.Helper()
	if err := Run(c); err != nil {
		t.Fatal(err)
	}
	r, ok := q.Landed("artifact")
	if !ok || r.Verdict != want {
		t.Fatalf("landed=%v verdict=%v want=%v rows=%v", ok, r.Verdict, want, r.Rows)
	}
}

// regression: F-ge04-recovery
func TestLostAckIsReconciledNotRepeated(t *testing.T) {
	p := &providerFake{key: "operation", lostAck: true}
	j := &recoveryJournal{}
	c, q := effectConfig(t, []Item{effectItem()}, p, j)
	runEffect(t, c, q, VerdictHold)
	if j.kinds()["EFFECT-INTENT"] != 1 || j.kinds()["EFFECT-DONE"] != 0 {
		t.Fatal(j.kinds())
	}
	it := effectItem()
	it.AttemptID = "attempt-2" // Retry deliberately stays zero: journal history was lost.
	p.lostAck = false
	c, q = effectConfig(t, []Item{it}, p, j)
	runEffect(t, c, q, VerdictPass)
	if len(p.records) != 1 || j.kinds()["RECONCILE"] != 1 {
		t.Fatalf("records=%d kinds=%v", len(p.records), j.kinds())
	}
	t.Log("RECONCILE records=1")
}

// regression: F-ge04-recovery
func TestDuplicateDeliveryAppliesOnce(t *testing.T) {
	p := &providerFake{key: "operation"}
	c, q := effectConfig(t, []Item{effectItem(), effectItem()}, p, &recoveryJournal{})
	runEffect(t, c, q, VerdictPass)
	if len(p.records) != 1 {
		t.Fatalf("records=%d", len(p.records))
	}
}

// regression: F-ge04-recovery
func TestLostEventTriggersReconcile(t *testing.T) {
	p := &providerFake{key: "operation"}
	j := &recoveryJournal{dropDone: true}
	c, q := effectConfig(t, []Item{effectItem()}, p, j)
	runEffect(t, c, q, VerdictPass)
	it := effectItem()
	it.AttemptID = "attempt-2"
	it.Retry = 1
	c, q = effectConfig(t, []Item{it}, p, j)
	runEffect(t, c, q, VerdictPass)
	if len(p.records) != 1 || j.kinds()["RECONCILE"] != 1 {
		t.Fatalf("records=%d kinds=%v", len(p.records), j.kinds())
	}
}

// regression: F-ge04-recovery
func TestJournalRequiredForEffects(t *testing.T) {
	p := &providerFake{key: "operation"}
	c, q := effectConfig(t, []Item{{ID: "ordinary"}, effectItem()}, p, nil)
	c.Effects = func(it Item) ([]Effect, error) {
		if it.ID == "ordinary" {
			return nil, nil
		}
		return []Effect{p}, nil
	}
	if err := Run(c); err == nil {
		t.Fatal("missing journal accepted")
	}
	if _, ok := q.Landed("ordinary"); ok || len(p.records) != 0 {
		t.Fatal("pass started before validation")
	}
}

// regression: F-ge04-recovery
func TestIntentRecordFailureAbortsEffect(t *testing.T) {
	p := &providerFake{key: "operation"}
	c, q := effectConfig(t, []Item{effectItem()}, p, &recoveryJournal{fail: "EFFECT-INTENT"})
	runEffect(t, c, q, VerdictError)
	r, _ := q.Landed("artifact")
	if len(p.records) != 0 || !strings.Contains(r.Rows[0].Name, "journal") {
		t.Fatal(r)
	}
}

// regression: F-ge04-recovery
func TestReceiptCouldNotCheckHolds(t *testing.T) {
	p := &providerFake{key: "operation", lookupErr: errors.New("offline")}
	c, q := effectConfig(t, []Item{effectItem()}, p, &recoveryJournal{})
	runEffect(t, c, q, VerdictHold)
	if len(p.records) != 0 {
		t.Fatal("applied on unreadable receipt")
	}
}

// regression: F-ge04-recovery
func TestNonEffectJournalStaysBestEffort(t *testing.T) {
	q := NewMemoryQueue("ordinary", []Item{{ID: "ordinary"}})
	c := Config{Loop: q, Claimer: mustFileClaim(t, t.TempDir()), PoolSize: 1, StopWhenIdle: true, Journal: &recoveryJournal{fail: "CLAIM"}}
	if err := Run(c); err != nil {
		t.Fatal(err)
	}
	if r, ok := q.Landed("ordinary"); !ok || r.Verdict != VerdictPass {
		t.Fatal("claim log aborted normal dispatch")
	}
}

// regression: F-ge04-recovery
func TestDeclaredEffectCannotBypassJournal(t *testing.T) {
	// Node kind is deliberately artifact, represented by an opaque payload: the
	// engine must route EVERY declaration through the same mandatory-intent seam.
	p := &providerFake{key: "operation"}
	it := effectItem()
	it.Payload = map[string]string{"kind": "artifact"}
	c, q := effectConfig(t, []Item{it}, p, &recoveryJournal{fail: "EFFECT-INTENT"})
	runEffect(t, c, q, VerdictError)
	if len(p.records) != 0 {
		t.Fatal("artifact bypassed intent")
	}
}

// regression: F-ge04-recovery
func TestEffectUnsupportedIdempotencyHolds(t *testing.T) {
	p := &providerFake{key: "operation", lostAck: true}
	j := &recoveryJournal{}
	c, q := effectConfig(t, []Item{effectItem()}, p, j)
	runEffect(t, c, q, VerdictHold)
	p.lostAck = false
	p.lookupErr = errors.New("provider cannot prove absence or dedupe")
	it := effectItem()
	it.AttemptID = "attempt-2"
	it.Retry = 1
	c, q = effectConfig(t, []Item{it}, p, j)
	runEffect(t, c, q, VerdictHold)
	if len(p.records) != 1 {
		t.Fatalf("records=%d: unsupported lookup reapplied", len(p.records))
	}
}

// regression: F-ge04-recovery
func TestEffectKeysAndIdentity(t *testing.T) {
	a, err := OperationKey("run", "publish", "target", "subject")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := OperationKey("run", "publish", "target", "subject")
	if a != b {
		t.Fatal("unstable key")
	}
	for _, v := range [][4]string{{"other", "publish", "target", "subject"}, {"run", "other", "target", "subject"}, {"run", "publish", "other", "subject"}, {"run", "publish", "target", "other"}} {
		k, _ := OperationKey(v[0], v[1], v[2], v[3])
		if a == k {
			t.Fatal("coordinate omitted")
		}
	}
	if _, err := OperationKey("", "publish", "target", "subject"); err == nil {
		t.Fatal("empty coordinate")
	}
	for _, it := range []Item{{ID: "artifact", AttemptID: "attempt"}, {ID: "artifact", RunID: "run"}} {
		p := &providerFake{key: a}
		c, q := effectConfig(t, []Item{it}, p, &recoveryJournal{})
		runEffect(t, c, q, VerdictError)
		if len(p.records) > 0 {
			t.Fatal("missing identity applied")
		}
	}
}

// regression: F-ge04-recovery
func TestEffectOutcomeAndDiscovery(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want Verdict
	}{{EffectFailure{errors.New("rejected before mutation")}, VerdictFail},
		{&EffectFailure{errors.New("rejected before mutation")}, VerdictFail}, {errors.New("unknown outcome"), VerdictHold}} {
		p := &providerFake{key: "operation", applyErr: tc.err}
		c, q := effectConfig(t, []Item{effectItem()}, p, &recoveryJournal{})
		runEffect(t, c, q, tc.want)
	}
	p := &providerFake{key: "operation"}
	c, q := effectConfig(t, []Item{effectItem()}, p, &recoveryJournal{})
	c.Effects = func(Item) ([]Effect, error) { return nil, errors.New("declarations unreadable") }
	if err := Run(c); err != nil {
		t.Fatal(err)
	}
	if _, ok := q.Landed("artifact"); ok || len(p.records) > 0 {
		t.Fatal("discovery failure was dispatched")
	}
}

// regression: F-ge04-recovery
func TestMultipleEffectsResume(t *testing.T) {
	first := &providerFake{key: "first"}
	second := &providerFake{key: "second", lostAck: true}
	j := &recoveryJournal{}
	c, q := effectConfig(t, []Item{effectItem()}, first, j)
	c.Effects = func(Item) ([]Effect, error) { return []Effect{first, second}, nil }
	runEffect(t, c, q, VerdictHold)
	second.lostAck = false
	it := effectItem()
	it.AttemptID = "attempt-2"
	c, q = effectConfig(t, []Item{it}, first, j)
	c.Effects = func(Item) ([]Effect, error) { return []Effect{first, second}, nil }
	runEffect(t, c, q, VerdictPass)
	if len(first.records) != 1 || len(second.records) != 1 || j.kinds()["RECONCILE"] != 2 {
		t.Fatal("partial completion repeated an operation")
	}
}
