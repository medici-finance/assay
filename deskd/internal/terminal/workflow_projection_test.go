package terminal

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// the service contract, Verify row 103 (the task-workflow amendment of
// 2026-10-02): workflow controller / node / attempt projections are READ
// records. They are not registry entries, so observing, reconnecting to or
// refreshing one never starts, restarts or re-attaches anything on the attach
// side, and a waiting workflow with no session is not an attach failure.
//
// What "makes zero spawn or exec calls" is measured by: a spawn or exec on the
// attach side happens through a driver's Attach (a driver is the only thing
// that execs), and attach-side code reaches a driver by one of these routes:
//
//   - a DriverLookup handed to the Supervisor, which attaches configured desks;
//   - a Target handed to Registry.Attach by its caller;
//   - a resolver handed to Registry.OpenSession by its caller (deskd hands it
//     RegisteredDriver; OpenSession refuses a workflow id before resolving a
//     driver, and serve's TestNamedActionsRejectWorkflowControl pins that);
//   - the package's own driver table (RegisterDriver / DriverFor /
//     RegisteredDriver), which registry code needs no caller to reach.
//
// The Supervisor's lookup hands out recording fakes, the lower-layer subtest
// hands Registry.Attach a counting probe, and this test binary registers a
// recording fake in the driver table for every kind, so a read surface that
// reaches a driver by itself is seen. The test counts, per kind and per route,
// driver constructions and driver Attach calls, plus the supervisor's backoff
// clock (a re-attach is always preceded by an After() wait), and asserts every
// count is exactly what it was before any workflow record was touched. A direct
// os/exec call from registry code would bypass every driver; no driver fake can
// see that, and this test does not claim to.
//
// The "workflow records" are the ids a cockpit viewer holds for a workflow
// stage: a work item, a node, an attempt, and the desk-shaped ids that embed a
// real desk (the shape that would tempt a lazy lookup). None is a configured
// desk. The attach side has no workflow type to import, which is the point: the
// only way a workflow record can reach it is as an id, so an id is what is
// thrown at every read surface.

// projectionIDs are workflow-stage record ids. None names a configured desk.
var projectionIDs = []string{
	"assay/workflow/wf-7f3c9a",                     // a controller record
	"assay/workflow/wf-7f3c9a/node/review",         // a node
	"assay/workflow/wf-7f3c9a/attempt/2",           // an attempt
	"assay/review/attempt/7f3c9a",                  // embeds a real desk id
	"assay/dispatch/attempt/7f3c9a",                // embeds the pod desk id
	"assay/waiting-workflow",                       // waiting, no session ever
	"wf-7f3c9a",                                    // a bare id
	"assay/custom/wf-7f3c9a",                       // mimics a custom session
	"",                                             // an empty id
	"assay/review/../workflow/wf-7f3c9a/attempt/2", // a path-shaped id
}

// countingTarget records Attach calls without ever opening a session.
type countingTarget struct {
	mu sync.Mutex
	n  int
}

func (c *countingTarget) Attach(context.Context, *TargetSpec, Size) (Session, error) {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil, ErrUnreachable
}

func (c *countingTarget) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// tableTarget is this test binary's registration in the package driver table
// for one kind. It counts constructions and Attach calls and never opens a
// session.
type tableTarget struct {
	mu       sync.Mutex
	built    int
	attached int
}

func (c *tableTarget) factory() (Target, error) {
	c.mu.Lock()
	c.built++
	c.mu.Unlock()
	return c, nil
}

func (c *tableTarget) Attach(context.Context, *TargetSpec, Size) (Session, error) {
	c.mu.Lock()
	c.attached++
	c.mu.Unlock()
	return nil, ErrUnreachable
}

func (c *tableTarget) counts() (built, attached int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.built, c.attached
}

var tableKinds = []schema.TargetKind{schema.TargetContainer, schema.TargetLocal, schema.TargetPod}

// The driver-table registration is once per test binary: RegisterDriver panics
// on a second registration for a kind, and -count=N reruns tests in-process.
var (
	tableOnce  sync.Once
	tableFakes map[schema.TargetKind]*tableTarget
	tableErr   string
)

// registerTableFakes puts a recording fake in the package driver table for every
// kind, and proves the table resolves to it (a positive control: a registration
// that did not take would leave the instrument blind and the test green). The
// package links no driver, so every kind must be free; one that is not is
// could-not-check, never a silent reuse of whatever registered it.
func registerTableFakes(t *testing.T) map[schema.TargetKind]*tableTarget {
	t.Helper()
	tableOnce.Do(func() {
		for _, kind := range tableKinds {
			if _, taken := DriverFor(kind); taken {
				tableErr = fmt.Sprintf("could-not-check: a driver for kind %q is already registered in this test binary", kind)
				return
			}
		}
		fakes := map[schema.TargetKind]*tableTarget{}
		for _, kind := range tableKinds {
			f := &tableTarget{}
			RegisterDriver(kind, f.factory)
			fakes[kind] = f
		}
		for _, kind := range tableKinds {
			got, err := RegisteredDriver(kind)
			if err != nil || got != Target(fakes[kind]) {
				tableErr = fmt.Sprintf("could-not-check: the driver table resolves kind %q to %v (err %v), not this test's recording fake", kind, got, err)
				return
			}
		}
		tableFakes = fakes
	})
	if tableErr != "" {
		t.Fatal(tableErr)
	}
	return tableFakes
}

// projectionFixture is a started supervisor over four configured desks (one
// per kind plus a desk with no target), settled, with a snapshot of every
// attach-side observable taken after the settle.
type projectionFixture struct {
	reg      *Registry
	cell     *schema.Cell
	clk      *stepClock
	cont     *recTarget
	loc      *recTarget
	pod      *recTarget
	table    map[schema.TargetKind]*tableTarget
	mu       sync.Mutex // guards factory
	factory  map[schema.TargetKind]int
	baseline projectionSnapshot
}

type projectionSnapshot struct {
	attaches    map[schema.TargetKind]int // driver Attach calls, supervisor lookup
	factory     map[schema.TargetKind]int // driver constructions, supervisor lookup
	tableAttach map[schema.TargetKind]int // driver Attach calls, package driver table
	tableBuilt  map[schema.TargetKind]int // driver constructions, package driver table
	rows        int
	views       map[string]deskSnapshot
}

type deskSnapshot struct {
	connected bool
	writable  bool
	epoch     any
	cursor    uint64
	attach    any
}

func newProjectionFixture(t *testing.T) *projectionFixture {
	t.Helper()
	cfg := cfgWithLoops(
		containerLoop(schema.RoleReview, "c-review"),
		localLoop(schema.RoleVerify),
		podLoop(schema.RoleDispatch),
		schema.Loop{Role: schema.RoleMetrics},
	)
	clk := newStepClock()
	reg := newRegistryWithClock(cfg, clk.Now)
	f := &projectionFixture{
		reg:     reg,
		cell:    &cfg.Cells[0],
		clk:     clk,
		cont:    newRecTarget(),
		loc:     newRecTarget(),
		pod:     newRecTarget(),
		table:   registerTableFakes(t),
		factory: map[schema.TargetKind]int{},
	}
	byKind := map[schema.TargetKind]Target{
		schema.TargetContainer: f.cont, schema.TargetLocal: f.loc, schema.TargetPod: f.pod,
	}
	s := NewSupervisor(reg, clk, SupervisorOptions{
		Host: Host{OS: "linux", Arch: "amd64"},
		Drivers: func(kind schema.TargetKind) (func() (Target, error), bool) {
			tgt, ok := byKind[kind]
			if !ok {
				return nil, false
			}
			return func() (Target, error) {
				f.mu.Lock()
				f.factory[kind]++
				f.mu.Unlock()
				return tgt, nil
			}, true
		},
		Jitter: identityJitter,
	})
	s.Start(context.Background())
	t.Cleanup(s.Stop)

	waitStatus(t, reg, "assay/review", AttachAttached, ReasonNone)
	waitStatus(t, reg, "assay/verify", AttachAttached, ReasonNone)
	waitStatus(t, reg, "assay/dispatch", AttachFailed, ReasonUnauthorized)
	waitStatus(t, reg, "assay/metrics", AttachFailed, ReasonNoTarget)
	clk.noWait(t, "no desk may be waiting on a retry before any workflow record is touched")
	f.baseline = f.snapshot()
	return f
}

func (f *projectionFixture) snapshot() projectionSnapshot {
	s := projectionSnapshot{
		attaches: map[schema.TargetKind]int{
			schema.TargetContainer: f.cont.calls(), schema.TargetLocal: f.loc.calls(), schema.TargetPod: f.pod.calls(),
		},
		factory:     map[schema.TargetKind]int{},
		tableAttach: map[schema.TargetKind]int{},
		tableBuilt:  map[schema.TargetKind]int{},
		rows:        f.reg.Len(),
		views:       map[string]deskSnapshot{},
	}
	f.mu.Lock()
	for k, n := range f.factory {
		s.factory[k] = n
	}
	f.mu.Unlock()
	for k, c := range f.table {
		s.tableBuilt[k], s.tableAttach[k] = c.counts()
	}
	for _, h := range f.reg.Handles() {
		s.views[h.DeskID] = deskSnapshot{
			connected: h.Connected, writable: h.Writable, epoch: h.Epoch, cursor: h.Cursor, attach: h.Attach,
		}
	}
	return s
}

// assertUntouched fails unless the attach side is exactly what it was before
// the workflow records were observed: no driver was built or called, no row was
// added or changed, no retry was scheduled, and no desk's attach status moved.
func (f *projectionFixture) assertUntouched(t *testing.T, what string) {
	t.Helper()
	f.clk.noWait(t, what)
	got := f.snapshot()
	if !reflect.DeepEqual(got.attaches, f.baseline.attaches) {
		t.Errorf("%s: driver Attach calls %v, want %v (a spawn or exec on the attach side)", what, got.attaches, f.baseline.attaches)
	}
	if !reflect.DeepEqual(got.factory, f.baseline.factory) {
		t.Errorf("%s: driver constructions %v, want %v", what, got.factory, f.baseline.factory)
	}
	if !reflect.DeepEqual(got.tableAttach, f.baseline.tableAttach) {
		t.Errorf("%s: driver-table Attach calls %v, want %v (a spawn or exec through the package driver table)", what, got.tableAttach, f.baseline.tableAttach)
	}
	if !reflect.DeepEqual(got.tableBuilt, f.baseline.tableBuilt) {
		t.Errorf("%s: driver-table constructions %v, want %v", what, got.tableBuilt, f.baseline.tableBuilt)
	}
	if got.rows != f.baseline.rows {
		t.Errorf("%s: %d desk rows, want %d (a workflow record became a registry entry)", what, got.rows, f.baseline.rows)
	}
	if !reflect.DeepEqual(got.views, f.baseline.views) {
		t.Errorf("%s: desk views changed\n got  %+v\n want %+v", what, got.views, f.baseline.views)
	}
}

// observeProjection reads one workflow record id through every per-id surface
// the registry offers a reader. Every one must miss. The listing surfaces
// (Handles, DeskRows) take no id; observeListing reads those.
func observeProjection(t *testing.T, r *Registry, id string) {
	t.Helper()
	if _, ok := r.Resolve(id); ok {
		t.Errorf("Resolve(%q) found a desk: a workflow record is not a registry entry", id)
	}
	if f, ok := r.Follow(id); ok {
		f.Close()
		t.Errorf("Follow(%q) yielded a follower: a workflow record is not a registry entry", id)
	}
	if _, err := r.Since(id, 0); err == nil {
		t.Errorf("Since(%q) read a scrollback", id)
	}
	if r.Alive(id) {
		t.Errorf("Alive(%q) = true", id)
	}
	if n := r.Followers(id); n != 0 {
		t.Errorf("Followers(%q) = %d", id, n)
	}
	if e := r.Epoch(id); e != "" {
		t.Errorf("Epoch(%q) = %q, want \"\"", id, e)
	}
}

// observeListing reads the two listings a viewer reads beside its workflow
// records, Handles (the terminal listing) and DeskRows (the desk registry read
// serve and mcp return). Each must carry exactly the four configured desks.
func (f *projectionFixture) observeListing(t *testing.T, round int) {
	t.Helper()
	if n := len(f.reg.Handles()); n != 4 {
		t.Fatalf("round %d: Handles() = %d rows, want the 4 configured desks", round, n)
	}
	rows := f.reg.DeskRows(f.cell)
	if len(rows) != 4 {
		t.Fatalf("round %d: DeskRows() = %d rows, want the 4 configured desks", round, len(rows))
	}
	for _, row := range rows {
		for _, id := range projectionIDs {
			if row.ID == id {
				t.Fatalf("round %d: DeskRows() carries a workflow record %q", round, id)
			}
		}
	}
}

// TestAttachIgnoresWorkflowProjection is Verify row 103.
func TestAttachIgnoresWorkflowProjection(t *testing.T) {
	configured := []string{"assay/review", "assay/verify", "assay/dispatch", "assay/metrics"}

	t.Run("driver_table_is_observed", func(t *testing.T) {
		// Positive control for the driver-table instrument: one construction and
		// one Attach through the package driver table must move the snapshot by
		// exactly that much, so a blind instrument fails here instead of reading
		// clean in every subtest below.
		f := newProjectionFixture(t)
		tgt, err := RegisteredDriver(schema.TargetLocal)
		if err != nil {
			t.Fatalf("could-not-check: RegisteredDriver(local): %v", err)
		}
		if _, err := tgt.Attach(context.Background(), &TargetSpec{Kind: schema.TargetLocal}, DefaultAttachSize); err == nil {
			t.Fatalf("the driver-table fake opened a session")
		}
		got := f.snapshot()
		if d := got.tableBuilt[schema.TargetLocal] - f.baseline.tableBuilt[schema.TargetLocal]; d != 1 {
			t.Errorf("driver-table constructions moved by %d, want 1", d)
		}
		if d := got.tableAttach[schema.TargetLocal] - f.baseline.tableAttach[schema.TargetLocal]; d != 1 {
			t.Errorf("driver-table Attach calls moved by %d, want 1", d)
		}
	})

	t.Run("observing_workflow_records", func(t *testing.T) {
		f := newProjectionFixture(t)
		for round := 0; round < 3; round++ {
			for _, id := range projectionIDs {
				observeProjection(t, f.reg, id)
			}
			f.observeListing(t, round)
		}
		f.assertUntouched(t, "after observing workflow records")
	})

	t.Run("waiting_workflow_is_not_a_failure", func(t *testing.T) {
		// A workflow stage that is waiting has no session. It must add no row, so
		// there is nothing to read as failed / no-target / unreachable for it, and
		// the configured desks' own statuses are untouched.
		f := newProjectionFixture(t)
		observeProjection(t, f.reg, "assay/waiting-workflow")
		for _, h := range f.reg.Handles() {
			if h.DeskID == "assay/waiting-workflow" {
				t.Fatalf("a waiting workflow became a desk row: %+v", h.Attach)
			}
		}
		f.assertUntouched(t, "after a waiting workflow with no session")
	})

	t.Run("reconnecting_viewers", func(t *testing.T) {
		// A cockpit viewer connecting, dropping and resuming from its cursor, for
		// workflow-stage records and for the configured desks beside them, never
		// touches the session deskd holds and never causes a re-attach.
		f := newProjectionFixture(t)
		for round := 0; round < 5; round++ {
			for _, id := range projectionIDs {
				observeProjection(t, f.reg, id)
			}
			for _, id := range configured {
				fw, ok := f.reg.Follow(id)
				if !ok {
					t.Fatalf("round %d: Follow(%s) missed a configured desk", round, id)
				}
				fw.ReadFrom(0)
				fw.State()
				fw.Close()
			}
		}
		for _, id := range configured {
			if n := f.reg.Followers(id); n != 0 {
				t.Errorf("Followers(%s) = %d after every viewer disconnected, want 0", id, n)
			}
		}
		// The attached sessions are still the ones deskd opened: neither was closed.
		for name, tgt := range map[string]*recTarget{"container": f.cont, "local": f.loc} {
			tgt.mu.Lock()
			for i, s := range tgt.sessions {
				s.mu.Lock()
				if s.closed {
					t.Errorf("%s session %d was closed by a viewer connecting or disconnecting", name, i)
				}
				s.mu.Unlock()
			}
			tgt.mu.Unlock()
		}
		f.assertUntouched(t, "after viewers reconnected")
	})

	t.Run("refreshing_workflow_records", func(t *testing.T) {
		// A refresh re-reads on a later clock: the idle window elapses and the
		// prompt signal is recomputed. None of it is an attach event.
		f := newProjectionFixture(t)
		for round := 0; round < 3; round++ {
			f.clk.advance(10 * time.Minute)
			for _, id := range projectionIDs {
				observeProjection(t, f.reg, id)
			}
			for _, id := range configured {
				f.reg.handle(id).Refresh()
				f.reg.Resolve(id)
			}
			f.observeListing(t, round)
		}
		f.assertUntouched(t, "after refreshing on an advanced clock")
	})

	t.Run("lower_layer_with_the_supervisor_bypassed", func(t *testing.T) {
		// The upper layer is the supervisor, whose only input is the configured
		// registry. This calls the registry's own attach and detach directly with a
		// workflow id and a recording target: with no supervisor in the path, a
		// workflow record is still refused before any driver is touched.
		f := newProjectionFixture(t)
		probe := &countingTarget{}
		for _, id := range projectionIDs {
			if err := f.reg.Attach(context.Background(), id, probe, DefaultAttachSize); err == nil {
				t.Errorf("Attach(%q) succeeded for a workflow record", id)
			}
			if err := f.reg.Detach(id); err == nil {
				t.Errorf("Detach(%q) succeeded for a workflow record", id)
			}
		}
		if n := probe.calls(); n != 0 {
			t.Errorf("the target was called %d time(s) for workflow records, want 0", n)
		}
		f.assertUntouched(t, "after attaching workflow ids directly")
	})
}
