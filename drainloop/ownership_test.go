package drainloop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

// diskCell is a TEST-ONLY durable adapter, never an alternative runtime backend.
// Exclusive directory locks serialize independent processes; write/sync/rename
// plus directory sync model the acknowledged durability boundary. It proves the
// store-neutral protocol on this local test filesystem, not PostgreSQL or fencing.
type diskCell struct {
	path                   string
	nonDurable, failCommit bool
}

func (d *diskCell) RequiredDurability(context.Context) error {
	if d.nonDurable {
		return errors.New("non-durable test adapter")
	}
	return nil
}
func (d *diskCell) locked(ctx context.Context, f func() error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := os.Mkdir(d.path+".lock", 0700)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		time.Sleep(time.Millisecond)
	}
	defer os.Remove(d.path + ".lock")
	return f()
}
func (d *diskCell) read() (CellState, error) {
	var s CellState
	b, e := os.ReadFile(d.path)
	if e != nil {
		return s, e
	}
	e = json.Unmarshal(b, &s)
	return s, e
}
func (d *diskCell) save(s CellState) error {
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(d.path), "cell-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	if e = os.Rename(f.Name(), d.path); e != nil {
		return e
	}
	dir, e := os.Open(filepath.Dir(d.path))
	if e != nil {
		return e
	}
	defer dir.Close()
	return dir.Sync()
}
func (d *diskCell) ReadCell(ctx context.Context, cell string) (s CellState, e error) {
	e = d.locked(ctx, func() error {
		var err error
		s, err = d.read()
		if err == nil && s.Checkpoint.Ownership.CellID != cell {
			return errors.New("wrong cell")
		}
		return err
	})
	return
}
func (d *diskCell) Transaction(ctx context.Context, cell string, v uint64, f func(*CellState) error) error {
	return d.locked(ctx, func() error {
		s, e := d.read()
		if e != nil {
			return e
		}
		if s.Version != v {
			return ErrCellConflict
		}
		if s.Checkpoint.Ownership.CellID != cell {
			return errors.New("wrong cell")
		}
		if e = f(&s); e != nil {
			return e
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		if d.failCommit {
			return errors.New("injected durability failure")
		}
		s.Version++
		return d.save(s)
	})
}

type cellControl struct {
	mu            sync.Mutex
	current       OperatorIncarnation
	fail, cutover bool
	epoch         uint64
	issued        map[CellPermit]bool
}

func (c *cellControl) CurrentIncarnation(context.Context, string) (OperatorIncarnation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail {
		return OperatorIncarnation{}, errors.New("protected current unavailable")
	}
	return c.current, nil
}
func (c *cellControl) CheckCellCapability(_ context.Context, p CellPermit) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cutover || !c.issued[p] {
		return errors.New("scope/signature/cutover unqualified")
	}
	return nil
}
func (c *cellControl) PacingEpoch(context.Context, string, string, string, string) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail || c.epoch > 1 {
		return 0, errors.New("reset anchor absent/exhausted")
	}
	return c.epoch, nil
}
func (c *cellControl) issue(r CellAdmission) CellAdmission {
	r.Permit.CapabilityRef = "ref:authorized-" + r.Permit.Key
	r.Permit.AdmissionDigest = CellAdmissionDigest(r)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.issued == nil {
		c.issued = map[CellPermit]bool{}
	}
	c.issued[r.Permit] = true
	return r
}
func cellFixture(t *testing.T) (*diskCell, *cellControl, CellAuthority) {
	t.Helper()
	d := &diskCell{path: filepath.Join(t.TempDir(), "cell.json")}
	n := uint64(0)
	s := CellState{Version: 1, Checkpoint: CellCheckpoint{Schema: CellCheckpointSchema, Ownership: CellOwnership{CellID: "cell", IncarnationID: "inc-1", OwnerID: "owner", Generation: "generation-1", Backend: "test-provider"}, Activation: CellActivation{IncarnationID: "inc-1", KeyRef: "ref:key-1", StopReceipt: "ref:group-stopped"}, Budget: CellBudget{Balances: []BudgetBalance{{Ref: "ref:input", Account: "account", Provider: "provider", Unit: "reported-input-tokens", WindowStart: "2026-11-03T00:00:00Z", WindowEnd: "2026-11-04T00:00:00Z", Limit: 100, Headroom: 10, Consumed: &n}}}}}
	if e := d.save(s); e != nil {
		t.Fatal(e)
	}
	c := &cellControl{current: OperatorIncarnation{Schema: IncarnationSchema, CellID: "cell", ID: "inc-1", KeyRef: "ref:key-1"}, cutover: true}
	return d, c, CellAuthority{Store: d, Current: c, Capabilities: c, Pacing: c}
}
func cellRequest(t *testing.T, d *diskCell, c *cellControl, key string, amount uint64) CellAdmission {
	t.Helper()
	s, e := d.ReadCell(context.Background(), "cell")
	if e != nil {
		t.Fatal(e)
	}
	p := CellPermit{Ownership: s.Checkpoint.Ownership, WorkID: "work", AttemptID: "attempt-" + key, RequestID: "request-" + key, Key: key}
	return c.issue(CellAdmission{Permit: p, Intent: Intent{Event: Event{Kind: "EFFECT-INTENT", RunID: p.WorkID, AttemptID: p.AttemptID, Key: key, Generation: p.Ownership.Generation, ItemID: "item", Time: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}}, Reservations: []BudgetReservation{{ID: "reservation-" + key, WorkID: p.WorkID, AttemptID: p.AttemptID, RequestID: p.RequestID, BalanceRef: "ref:input", Amount: amount}}, ControlBoundary: "request"})
}
func cellEdit(t *testing.T, d *diskCell, f func(*CellState)) {
	t.Helper()
	s, e := d.ReadCell(context.Background(), "cell")
	if e != nil {
		t.Fatal(e)
	}
	if e = d.Transaction(context.Background(), "cell", s.Version, func(s *CellState) error { f(s); return nil }); e != nil {
		t.Fatal(e)
	}
}
func cellRead(t *testing.T, d *diskCell) CellState {
	t.Helper()
	s, e := d.ReadCell(context.Background(), "cell")
	if e != nil {
		t.Fatal(e)
	}
	return s
}

// fakeCellProvider is a separately invoked lower boundary. Its own apply lock,
// current/capability checks and receipt map enforce the fake provider's operation.
// No scheduler decision or recovered SQL tuple can bypass these checks.
type fakeCellProvider struct {
	mu        sync.Mutex
	authority CellAuthority
	receipts  map[string]Receipt
	applied   int
}

func (p *fakeCellProvider) apply(ctx context.Context, permit CellPermit) (Receipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.authority.CheckEffect(ctx, permit); err != nil {
		return Receipt{}, err
	}
	if r, ok := p.receipts[permit.Key]; ok {
		return r, nil
	}
	r := Receipt{Key: permit.Key, Ref: "ref:effect-" + permit.Key}
	p.receipts[permit.Key] = r
	p.applied++
	return r, nil
}

type cellEffect struct {
	p      *fakeCellProvider
	permit *CellPermit
}

func (e cellEffect) Key() string { return e.permit.Key }
func (e cellEffect) Receipt(context.Context, string) (Receipt, bool, error) {
	e.p.mu.Lock()
	defer e.p.mu.Unlock()
	r, ok := e.p.receipts[e.permit.Key]
	return r, ok, nil
}
func (e cellEffect) Apply(ctx context.Context) (Receipt, error) { return e.p.apply(ctx, *e.permit) }
func cellProvider(a CellAuthority) *fakeCellProvider {
	return &fakeCellProvider{authority: a, receipts: map[string]Receipt{}}
}

func TestCellOwnership(t *testing.T) {
	ctx := context.Background()
	d, c, a := cellFixture(t)
	r := cellRequest(t, d, c, "one", 30)
	if e := a.Commit(ctx, r); e != nil {
		t.Fatal(e)
	}
	s := cellRead(t, d)
	if len(s.Pending) != 1 || len(s.Checkpoint.Budget.Reservations) != 1 || len(s.Checkpoint.PendingEffects) != 1 {
		t.Fatal("intent/reservation not committed together")
	}
	p := cellProvider(a)
	if _, e := p.apply(ctx, r.Permit); e != nil {
		t.Fatal(e)
	}
	t.Run("lower-no-intent", func(t *testing.T) {
		d, c, a := cellFixture(t)
		r := cellRequest(t, d, c, "unadmitted", 1)
		p := cellProvider(a)
		if _, e := p.apply(ctx, r.Permit); e == nil || p.applied != 0 {
			t.Fatal("lower boundary accepted absent intent")
		}
	})
	for _, name := range []string{"non-durable", "commit-failed", "unknown-current", "no-cutover", "changed-reservation", "unbounded-launch", "cancelled", "stopped", "bad-schema", "absent-source"} {
		t.Run(name, func(t *testing.T) {
			d, c, a := cellFixture(t)
			r := cellRequest(t, d, c, name, 1)
			before := cellRead(t, d)
			switch name {
			case "non-durable":
				d.nonDurable = true
			case "commit-failed":
				d.failCommit = true
			case "unknown-current":
				c.fail = true
			case "no-cutover":
				c.cutover = false
			case "changed-reservation":
				r.Reservations[0].Amount = 2
			case "unbounded-launch":
				r.ControlBoundary = "launch"
				r = c.issue(r)
			case "cancelled":
				cellEdit(t, d, func(s *CellState) { s.Checkpoint.CancelRequested = true })
			case "stopped":
				cellEdit(t, d, func(s *CellState) { s.Checkpoint.Stopped = true })
			case "bad-schema":
				cellEdit(t, d, func(s *CellState) { s.Checkpoint.Schema = "unsupported" })
			case "absent-source":
				a.Current = nil
			}
			before = cellRead(t, d)
			if e := a.Commit(ctx, r); e == nil {
				t.Fatal("unsafe admission succeeded")
			}
			after := cellRead(t, d)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("refusal published partial state")
			}
		})
	}
	t.Run("real-outbox-flow", func(t *testing.T) {
		d, c, a := cellFixture(t)
		r := cellRequest(t, d, c, "flow", 1)
		p := cellProvider(a)
		j := CellJournal{Authority: a, CellID: "cell", Request: func(i Intent) (CellAdmission, error) { r.Intent = i; r = c.issue(r); return r, nil }}
		q := NewMemoryQueue("cell", []Item{{ID: "item", RunID: "work", AttemptID: r.Permit.AttemptID}})
		cl := mustFileClaim(t, t.TempDir())
		e := Run(Config{Loop: q, Claimer: cl, PoolSize: 1, StopWhenIdle: true, Journal: j, RequireHandoff: true, Effects: func(Item) ([]Effect, error) { return []Effect{cellEffect{p: p, permit: &r.Permit}}, nil }})
		if e != nil {
			t.Fatal(e)
		}
		if p.applied != 1 {
			t.Fatalf("real outbox path applied %d effects", p.applied)
		}
		s := cellRead(t, d)
		if len(s.Pending) != 0 || len(s.Completed) != 1 {
			t.Fatal("outbox completion not durable")
		}
	})
}
func TestCellEffectRestoredRecords(t *testing.T) {
	for _, name := range []string{"changed-intent", "missing-reservation", "unknown-brake"} {
		t.Run(name, func(t *testing.T) {
			d, c, a := cellFixture(t)
			r := cellRequest(t, d, c, name, 1)
			if e := a.Commit(context.Background(), r); e != nil {
				t.Fatal(e)
			}
			cellEdit(t, d, func(s *CellState) {
				switch name {
				case "changed-intent":
					s.Pending[0].Reservations[0].Amount = 2
				case "missing-reservation":
					s.Checkpoint.Budget.Reservations = nil
				case "unknown-brake":
					s.Checkpoint.Budget.Balances[0].Consumed = nil
				}
			})
			p := cellProvider(a)
			if _, e := p.apply(context.Background(), r.Permit); e == nil || p.applied != 0 {
				t.Fatal("restored invalid records authorized effect")
			}
		})
	}
}

func TestCellOwnershipStaleWriterDenied(t *testing.T) {
	d, c, a := cellFixture(t)
	ctx := context.Background()
	r := cellRequest(t, d, c, "old", 1)
	if e := a.Commit(ctx, r); e != nil {
		t.Fatal(e)
	}
	p := cellProvider(a)
	cellEdit(t, d, func(s *CellState) { s.Checkpoint.Ownership.Generation = "new" })
	if _, e := p.apply(ctx, r.Permit); e == nil || p.applied != 0 {
		t.Fatal("scheduler bypass applied stale writer")
	}
	r = cellRequest(t, d, c, "new", 1)
	if e := a.Commit(ctx, r); e != nil {
		t.Fatal(e)
	}
	if _, e := p.apply(ctx, r.Permit); e != nil || p.applied != 1 {
		t.Fatal("current writer baseline failed", e)
	}
}
func TestCellOwnershipRestoreBudgetPause(t *testing.T) {
	d, c, a := cellFixture(t)
	r := cellRequest(t, d, c, "restore", 30)
	if e := a.Commit(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	cellEdit(t, d, func(s *CellState) {
		s.Checkpoint.Stopped = true
		s.Checkpoint.CancelRequested = true
		s.Checkpoint.Activation.Paused = true
		s.Checkpoint.OverrideRefs = []string{"ref:override"}
		s.Checkpoint.Budget.Reservations[0].Unknown = true
	})
	before := cellRead(t, d)
	reopened := &diskCell{path: d.path}
	a.Store = reopened
	after := cellRead(t, reopened)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restart lost holds/usage")
	}
	if e := a.CheckEffect(context.Background(), r.Permit); e == nil {
		t.Fatal("restored hold allowed effect")
	}
}

type recoveryFixture struct {
	c                                     *cellControl
	fail, keepID, keepKey, keepGeneration bool
}

func (r recoveryFixture) ConfirmCellRecovery(_ context.Context, old CellOwnership) (OperatorIncarnation, CellActivation, CellOwnership, error) {
	if r.fail {
		return OperatorIncarnation{}, CellActivation{}, CellOwnership{}, errors.New("group stop/cutover unconfirmed")
	}
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	cur := r.c.current
	cur.ID = "inc-2"
	cur.KeyRef = "ref:key-2"
	if r.keepID {
		cur.ID = old.IncarnationID
	}
	if r.keepKey {
		cur.KeyRef = r.c.current.KeyRef
	}
	owned := old
	owned.OwnerID = "next"
	owned.Generation = "generation-2"
	owned.IncarnationID = cur.ID
	if r.keepGeneration {
		owned.Generation = old.Generation
	}
	r.c.current = cur
	return cur, CellActivation{IncarnationID: cur.ID, KeyRef: cur.KeyRef, StopReceipt: "ref:confirmed-new-group-stop"}, owned, nil
}
func TestOwnershipBackendHandbackUnknown(t *testing.T) {
	for _, name := range []string{"unknown-effect", "unknown-usage", "unconfirmed-stop", "old-id", "old-key", "old-generation", "clean"} {
		t.Run(name, func(t *testing.T) {
			d, c, a := cellFixture(t)
			op := recoveryFixture{c: c}
			cellEdit(t, d, func(s *CellState) {
				s.Checkpoint.OverrideRefs = []string{"ref:override"}
				switch name {
				case "unknown-effect":
					s.Checkpoint.PendingEffects = []string{"unknown"}
				case "unknown-usage":
					s.Checkpoint.Budget.Reservations = []BudgetReservation{{ID: "unknown", Unknown: true, Amount: 8}}
				case "unconfirmed-stop":
					op.fail = true
				case "old-id":
					op.keepID = true
				case "old-key":
					op.keepKey = true
				case "old-generation":
					op.keepGeneration = true
				}
			})
			before := cellRead(t, d)
			e := a.Transfer(context.Background(), "cell", op)
			after := cellRead(t, d)
			if name == "clean" {
				if e != nil {
					t.Fatal(e)
				}
				if after.Checkpoint.Ownership.IncarnationID == before.Checkpoint.Ownership.IncarnationID {
					t.Fatal("no rotation")
				}
			} else {
				if e == nil {
					t.Fatal("unsafe handback succeeded")
				}
				if after.Checkpoint.Ownership != before.Checkpoint.Ownership {
					t.Fatal("held transfer changed owner")
				}
			}
			if !reflect.DeepEqual(before.Checkpoint.Budget, after.Checkpoint.Budget) || !reflect.DeepEqual(before.Checkpoint.OverrideRefs, after.Checkpoint.OverrideRefs) {
				t.Fatal("handback discarded brakes/override")
			}
		})
	}
}
func TestIncarnationStaleRefused(t *testing.T) {
	d, c, a := cellFixture(t)
	r := cellRequest(t, d, c, "rollback", 1)
	if e := a.Commit(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	p := cellProvider(a)
	snapshot, e := os.ReadFile(d.path)
	if e != nil {
		t.Fatal(e)
	}
	c.current.ID = "inc-current"
	c.current.KeyRef = "ref:key-current"
	if e = os.WriteFile(d.path, snapshot, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = p.apply(context.Background(), r.Permit); e == nil || p.applied != 0 {
		t.Fatal("restored SQL tuple revived old incarnation")
	}
	c.current.ID = "inc-1"
	c.current.KeyRef = "ref:key-1"
	if _, e = p.apply(context.Background(), r.Permit); e != nil || p.applied != 1 {
		t.Fatal("current positive failed", e)
	}
}
func TestIncarnationMismatchPaused(t *testing.T) {
	for _, name := range []string{"missing", "mismatched", "unconfirmed", "clean"} {
		t.Run(name, func(t *testing.T) {
			d, c, a := cellFixture(t)
			r := cellRequest(t, d, c, name, 1)
			switch name {
			case "missing":
				c.fail = true
			case "mismatched":
				c.current.ID = "protected-current"
			}
			if name == "missing" || name == "mismatched" {
				if e := a.Commit(context.Background(), r); e == nil && name != "unconfirmed" {
					t.Fatal("restored activation selected current")
				}
			}
			op := recoveryFixture{c: c, fail: name == "unconfirmed"}
			e := a.Transfer(context.Background(), "cell", op)
			if name == "missing" || name == "unconfirmed" {
				if e == nil {
					t.Fatal("reopened without protected stop/source")
				}
				if !cellRead(t, d).Checkpoint.Activation.Paused {
					t.Fatal("failed reopen did not retain pause")
				}
			} else if e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestIncarnationSpecMatchesFields(t *testing.T) { TestIncarnationContractFields(t) }

var _ TransactionalJournal = CellJournal{}
