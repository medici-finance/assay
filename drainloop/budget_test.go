package drainloop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestCellBudgetConcurrentReservations(t *testing.T) {
	d, c, a := cellFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for n := 0; n < 24; n++ {
		r := cellRequest(t, d, c, fmt.Sprint(n), 30)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tries := 0; tries < 100; tries++ {
				err := a.Commit(ctx, r)
				if errors.Is(err, ErrCellConflict) {
					continue
				}
				if err == nil {
					mu.Lock()
					admitted++
					mu.Unlock()
				}
				return
			}
		}()
	}
	wg.Wait()
	s := cellRead(t, d)
	total := *s.Checkpoint.Budget.Balances[0].Consumed + s.Checkpoint.Budget.Balances[0].Headroom
	for _, q := range s.Checkpoint.Budget.Reservations {
		total += q.Amount
	}
	if admitted != 3 || total > 100 || len(s.Pending) != admitted {
		t.Fatalf("concurrent reservations: admitted=%d total=%d intents=%d", admitted, total, len(s.Pending))
	}
	t.Run("rollback-multi-balance", func(t *testing.T) {
		d, c, a := cellFixture(t)
		r := cellRequest(t, d, c, "multi", 1)
		q := r.Reservations[0]
		q.ID += "-other"
		q.BalanceRef = "missing"
		r.Reservations = append(r.Reservations, q)
		r = c.issue(r)
		before := cellRead(t, d)
		if e := a.Commit(ctx, r); e == nil {
			t.Fatal("missing balance admitted")
		}
		if !reflect.DeepEqual(before, cellRead(t, d)) {
			t.Fatal("partial multi-balance reservation published")
		}
	})
	t.Run("overflow", func(t *testing.T) {
		d, c, a := cellFixture(t)
		cellEdit(t, d, func(s *CellState) {
			s.Checkpoint.Budget.Balances[0].Consumed = uintRef(^uint64(0))
			s.Checkpoint.Budget.Balances[0].Limit = ^uint64(0)
		})
		if e := a.Commit(ctx, cellRequest(t, d, c, "overflow", 1)); e == nil {
			t.Fatal("overflow created fresh allowance")
		}
	})
}
func uintRef(n uint64) *uint64 { return &n }

type usageFixture struct {
	rows []CellSettlement
	err  error
}

func (u usageFixture) CellUsage(context.Context, CellPermit) ([]CellSettlement, error) {
	return u.rows, u.err
}
func TestCellBudgetRestoreAllAttempts(t *testing.T) {
	t.Run("signed-request-phase", TestCellUsagePhaseBound)
	d, c, a := cellFixture(t)
	ctx := context.Background()
	phases := []string{"preparation", "summary", "execution", "fallback", "verification"}
	var permits []CellPermit
	for n, phase := range phases {
		r := cellRequest(t, d, c, phase, 10)
		r.Phase = phase
		r = c.issue(r)
		if e := a.Commit(ctx, r); e != nil {
			t.Fatal(e)
		}
		charge := uintRef(uint64(n + 1))
		output := uintRef(3)
		outcome := "complete"
		if phase == "execution" {
			outcome = "failed"
		}
		if phase == "verification" {
			charge = nil
			output = nil
			outcome = "unknown"
		}
		u := AttemptUsage{WorkID: r.Permit.WorkID, AttemptID: r.Permit.AttemptID, ParentAttemptID: "original-attempt", RequestID: r.Permit.RequestID, Phase: phase, Outcome: outcome, Account: "account", Provider: "provider", Model: "example-model", PricingRef: "ref:policy", SourceRef: "ref:trust-gated-usage", StartedAt: "2026-11-03T12:00:00Z", FinishedAt: "2026-11-03T12:01:00Z", ControlBoundary: "request", Input: uintRef(17), Output: output, CacheRead: uintRef(2), CacheWrite: uintRef(0)}
		a.Usage = usageFixture{rows: []CellSettlement{{ReservationID: r.Reservations[0].ID, Charged: charge, Usage: u}}}
		if e := a.Settle(ctx, r.Permit); e != nil {
			t.Fatal(e)
		}
		if e := a.Settle(ctx, r.Permit); e != nil {
			t.Fatal("idempotent usage", e)
		}
		permits = append(permits, r.Permit)
	}
	cellEdit(t, d, func(s *CellState) {
		s.Checkpoint.Stopped = true
		s.Checkpoint.CancelRequested = true
		s.Checkpoint.OverrideRefs = []string{"ref:override"}
	})
	before := cellRead(t, d)
	a.Store = &diskCell{path: d.path}
	after := cellRead(t, a.Store.(*diskCell))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("restart changed exact usage/unknown/stop state")
	}
	b := after.Checkpoint.Budget
	if len(b.Usage) != 5 || *b.Balances[0].Consumed != 10 || len(b.Reservations) != 1 || !b.Reservations[0].Unknown {
		t.Fatalf("attempt charges lost: %+v", b)
	}
	for _, u := range b.Usage {
		if u.WorkID != "work" || *u.Input != 17 {
			t.Fatal("original work or numeric usage corrupted")
		}
	}
	if b.Usage[4].Output != nil {
		t.Fatal("unknown telemetry became zero")
	}
	if e := a.CheckEffect(ctx, permits[0]); e == nil {
		t.Fatal("restore cleared stop")
	}
	t.Run("unreadable-usage", func(t *testing.T) {
		d, c, a := cellFixture(t)
		r := cellRequest(t, d, c, "missing-usage", 10)
		if e := a.Commit(ctx, r); e != nil {
			t.Fatal(e)
		}
		a.Usage = usageFixture{err: errors.New("could-not-check")}
		before := cellRead(t, d)
		if e := a.Settle(ctx, r.Permit); e == nil {
			t.Fatal("unknown usage released")
		}
		if !reflect.DeepEqual(before, cellRead(t, d)) {
			t.Fatal("unknown usage changed reservations")
		}
	})
}
func TestCellUsageReconcileUnknown(t *testing.T) {
	d, c, a := cellFixture(t)
	ctx := context.Background()
	r := cellRequest(t, d, c, "reconcile", 10)
	if e := a.Commit(ctx, r); e != nil {
		t.Fatal(e)
	}
	u := AttemptUsage{WorkID: r.Permit.WorkID, AttemptID: r.Permit.AttemptID, RequestID: r.Permit.RequestID, Phase: "execution", Outcome: "unknown", Account: "account", Provider: "provider", Model: "model", PricingRef: "ref:policy", SourceRef: "ref:unknown", Input: uintRef(17)}
	x := CellSettlement{ReservationID: r.Reservations[0].ID, Usage: u}
	a.Usage = usageFixture{rows: []CellSettlement{x}}
	if e := a.Settle(ctx, r.Permit); e != nil {
		t.Fatal(e)
	}
	x.PreviousSourceRef = "ref:unknown"
	x.Usage.SourceRef = "ref:confirmed"
	x.Usage.Output = uintRef(3)
	x.Usage.Outcome = "complete"
	x.Charged = uintRef(4)
	a.Usage = usageFixture{rows: []CellSettlement{x}}
	if e := a.Settle(ctx, r.Permit); e != nil {
		t.Fatal(e)
	}
	if e := a.Settle(ctx, r.Permit); e != nil {
		t.Fatal(e)
	}
	s := cellRead(t, d)
	if len(s.Checkpoint.Budget.Usage) != 2 || len(s.Checkpoint.Budget.Reservations) != 0 || *s.Checkpoint.Budget.Balances[0].Consumed != 4 || s.Checkpoint.Budget.Usage[0].Output != nil {
		t.Fatal("unknown reconciliation lost provenance, duplicated charge or retained reservation")
	}
}

func TestCellBrakeCountUnavailableHolds(t *testing.T) {
	t.Run("authority-clock", TestCellAuthorityWindow)
	d, c, a := cellFixture(t)
	r := cellRequest(t, d, c, "brake", 1)
	cellEdit(t, d, func(s *CellState) { s.Checkpoint.Budget.Balances[0].Consumed = nil })
	p := cellProvider(a)
	if e := a.Commit(context.Background(), r); e == nil {
		t.Fatal("unknown brake admitted")
	}
	if _, e := p.apply(context.Background(), r.Permit); e == nil || p.applied != 0 {
		t.Fatal("unknown brake applied with pacing bypassed")
	}
	cellEdit(t, d, func(s *CellState) { s.Checkpoint.Budget.Balances[0].Consumed = uintRef(0) })
	if e := a.Commit(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	if _, e := p.apply(context.Background(), r.Permit); e != nil {
		t.Fatal("known-brake positive failed", e)
	}
}

// resetAnchor is deliberately distinct from the database snapshot. It models the
// qualified operator reset record and returns error if missing, unknown or reused.
type resetAnchor struct {
	*cellControl
	path string
}

func (a resetAnchor) PacingEpoch(context.Context, string, string, string, string) (uint64, error) {
	b, e := os.ReadFile(a.path)
	if e != nil {
		return 0, e
	}
	n, e := strconv.ParseUint(string(b), 10, 64)
	if e != nil || n > 1 {
		return 0, errors.New("reset missing/exhausted")
	}
	return n, nil
}

// Real child processes contend on diskCell. This helper is never a runtime path.
func TestCellPacingChild(t *testing.T) {
	if os.Getenv("ASSAY_CELL_CHILD") != "1" {
		return
	}
	d := &diskCell{path: os.Getenv("ASSAY_CELL_STATE")}
	s, e := d.ReadCell(context.Background(), "cell")
	if e != nil {
		t.Fatal(e)
	}
	c := &cellControl{current: OperatorIncarnation{Schema: IncarnationSchema, CellID: "cell", ID: "inc-1", KeyRef: "ref:key-1"}, cutover: true}
	a := CellAuthority{Store: d, Current: c, Capabilities: c, Pacing: resetAnchor{cellControl: c, path: os.Getenv("ASSAY_CELL_ANCHOR")}, Now: cellNow}
	_ = s
	for n := 0; n < 12; n++ {
		r := cellRequest(t, d, c, fmt.Sprintf("pid-%d-%d", os.Getpid(), n), 1)
		r.PacingScope = "scope"
		r.PacingWindow = "window"
		r = c.issue(r)
		for k := 0; k < 100; k++ {
			e := a.Commit(context.Background(), r)
			if errors.Is(e, ErrCellConflict) {
				continue
			}
			break
		}
	}
}
func pacingProcesses(t *testing.T, d *diskCell, anchor string) {
	t.Helper()
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	var cmds []*exec.Cmd
	var outputs []*os.File
	for n := 0; n < 2; n++ {
		out, e := os.CreateTemp(t.TempDir(), "process-log-*")
		if e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command(exe, "-test.run=^TestCellPacingChild$", "-test.timeout=20s")
		cmd.Env = append(os.Environ(), "ASSAY_CELL_CHILD=1", "ASSAY_CELL_STATE="+d.path, "ASSAY_CELL_ANCHOR="+anchor)
		cmd.Stdout = out
		cmd.Stderr = out
		if e = cmd.Start(); e != nil {
			t.Fatal(e)
		}
		cmds = append(cmds, cmd)
		outputs = append(outputs, out)
	}
	for n, cmd := range cmds {
		e := cmd.Wait()
		outputs[n].Close()
		if e != nil {
			b, _ := os.ReadFile(outputs[n].Name())
			t.Fatalf("real process failed: %v\n%s", e, b)
		}
	}
}
func TestCellPacingRestartNoMultiply(t *testing.T) {
	d, _, _ := cellFixture(t)
	anchor := filepath.Join(t.TempDir(), "protected-reset")
	if e := os.WriteFile(anchor, []byte("0"), 0600); e != nil {
		t.Fatal(e)
	}
	cellEdit(t, d, func(s *CellState) {
		s.Checkpoint.Budget.Balances[0].Limit = 10000
		s.Pacing = []CellPacing{{Scope: "scope", Window: "window", Incarnation: "inc-1", Class: "effect", Cap: 10, C: 2, HistoryKnown: true, Seed: uintRef(3), Cached: 0, ResetKnown: true}}
	})
	// Four actual worker processes over two launches, with every cache update absent.
	// An intact seed at less than half the cap and shared intents survive restarts.
	pacingProcesses(t, d, anchor)
	cellEdit(t, d, func(s *CellState) { s.Pacing[0].Cached = 0 })
	pacingProcesses(t, d, anchor)
	s := cellRead(t, d)
	initial := len(s.Pending) + len(s.Completed)
	if initial != 7 {
		t.Fatalf("shared history/seed ignored after cache loss/restarts: intents=%d want7", initial)
	}
	// Missing history is never measured zero, even if the cached row says zero.
	cellEdit(t, d, func(s *CellState) { s.Pacing[0].HistoryKnown = false; s.Pacing[0].Seed = nil })
	pacingProcesses(t, d, anchor)
	if len(cellRead(t, d).Pending) != initial {
		t.Fatal("missing history created fresh allowance")
	}
	// Exactly one recorded reset outside the lost database allows one fresh epoch.
	if e := os.WriteFile(anchor, []byte("1"), 0600); e != nil {
		t.Fatal(e)
	}
	cellEdit(t, d, func(s *CellState) {
		s.Pending = nil
		s.Completed = nil
		s.Pacing[0].Epoch = 1
		s.Pacing[0].HistoryKnown = true
		s.Pacing[0].Seed = uintRef(0)
	})
	pacingProcesses(t, d, anchor)
	resetCount := len(cellRead(t, d).Pending)
	if resetCount != 10 || initial+3+resetCount > 2*(10+2) {
		t.Fatalf("reset multiplied allowance: before=%d reset=%d", initial, resetCount)
	}
	for _, mode := range []string{"2", "missing"} {
		if mode == "missing" {
			if e := os.Remove(anchor); e != nil {
				t.Fatal(e)
			}
		} else if e := os.WriteFile(anchor, []byte(mode), 0600); e != nil {
			t.Fatal(e)
		}
		cellEdit(t, d, func(s *CellState) { s.Pending = nil; s.Completed = nil; s.Pacing[0].Seed = uintRef(0) })
		pacingProcesses(t, d, anchor)
		if len(cellRead(t, d).Pending) != 0 {
			t.Fatal("second or absent reset record admitted")
		}
	}
	t.Run("read-estimate-unknown", func(t *testing.T) {
		s := CellState{Pacing: []CellPacing{{Scope: "scope", Window: "window", Incarnation: "inc", Class: "read", Cap: 10, HistoryKnown: true, ResetKnown: true}}}
		if e := cellPacing(s, "scope", "window", "inc", 0); e == nil {
			t.Fatal("missing retained read count became zero")
		}
		s.Pacing[0].Seed = uintRef(3)
		if e := cellPacing(s, "scope", "window", "inc", 0); e != nil {
			t.Fatal(e)
		}
	})
}

func TestCellUsagePhaseBound(t *testing.T) {
	d, c, a := cellFixture(t)
	r := cellRequest(t, d, c, "phase", 5)
	if err := a.Commit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	u := usageFor(r.Permit, "ref:usage", "fallback")
	a.Usage = usageFixture{rows: []CellSettlement{{ReservationID: r.Reservations[0].ID, Charged: uintRef(2), Usage: u}}}
	before := cellRead(t, d)
	if err := a.Settle(context.Background(), r.Permit); err == nil {
		t.Fatal("different phase reused signed provider request")
	}
	if !reflect.DeepEqual(before, cellRead(t, d)) {
		t.Fatal("wrong phase partially charged")
	}
	u.Phase = "execution"
	a.Usage = usageFixture{rows: []CellSettlement{{ReservationID: r.Reservations[0].ID, Charged: uintRef(2), Usage: u}}}
	if err := a.Settle(context.Background(), r.Permit); err != nil {
		t.Fatal(err)
	}
}
func TestCellAuthorityWindow(t *testing.T) {
	for _, name := range []string{"closed-current", "zero-current", "missing-clock", "old-provenance"} {
		t.Run(name, func(t *testing.T) {
			d, c, a := cellFixture(t)
			r := cellRequest(t, d, c, name, 1)
			switch name {
			case "closed-current":
				a.Now = func() time.Time { return cellNow().Add(48 * time.Hour) }
			case "zero-current":
				a.Now = func() time.Time { return time.Time{} }
			case "missing-clock":
				a.Now = nil
			case "old-provenance":
				r.Intent.Event.Time = cellNow().Add(-48 * time.Hour)
				r = c.issue(r)
			}
			err := a.Commit(context.Background(), r)
			if name == "old-provenance" {
				if err != nil {
					t.Fatal("provenance time chosen as current", err)
				}
			} else if err == nil {
				t.Fatal("closed/unavailable authority clock admitted")
			}
		})
	}
	d, c, a := cellFixture(t)
	now := cellNow()
	a.Now = func() time.Time { return now }
	r := cellRequest(t, d, c, "window-boundary", 1)
	if err := a.Commit(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	now = now.Add(48 * time.Hour)
	provider := cellProvider(a)
	if _, err := provider.apply(context.Background(), r.Permit); err == nil || provider.applied != 0 {
		t.Fatal("provider bypassed closed authority window")
	}
}
