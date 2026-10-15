//go:build darwin || linux

package cellcache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	admitBudget = 1 << 20
	bigFree     = uint64(1) << 40
)

// warmed returns a recorded, clean cache holding 100 logical bytes under a
// 1 MiB budget and a zero floor.
func warmed(t *testing.T) Policy {
	t.Helper()
	p := fixture(t)
	p.Budget = admitBudget
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/a", 100)
	if r, err := Check(p, false); err != nil || *r.After.LogicalBytes != 100 {
		t.Fatalf("fixture pass: %+v %v", r, err)
	}
	return p
}

// holdCacheLock takes the root's custody lock the way a sibling dispatch
// would, so any path that tries to take it blocks or reports busy.
func holdCacheLock(t *testing.T, p Policy) {
	t.Helper()
	r, err := os.OpenRoot(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	f, err := r.OpenFile("lock", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	h, err := deskkit.AcquireHeld(r, "lock", f, deskkit.LockSpec{Name: "cell Go cache", Wait: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Release(); r.Close() })
}

func shortLockWait(t *testing.T) {
	t.Helper()
	old := LockWait
	LockWait = 50 * time.Millisecond
	t.Cleanup(func() { LockWait = old })
}

// lockedProbe is the locked-path seam: it counts calls and returns a marker.
func lockedProbe(calls *int) func(Policy, bool) (Report, error) {
	return func(p Policy, dry bool) (Report, error) {
		*calls++
		return Report{Schema: schema, Policy: p, Outcome: "checked-clean", DryRun: dry}, nil
	}
}

func envWith(free uint64, calls *int) admitEnv {
	return admitEnv{
		free:   func(string) (uint64, error) { return free, nil },
		now:    time.Now,
		locked: lockedProbe(calls),
	}
}

// regression: #3045
func TestCacheAdmitCheapNoLock(t *testing.T) {
	p := warmed(t)
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	// A sibling holds the lock for the whole call: a path that takes it would
	// wait out LockWait and come back busy.
	holdCacheLock(t, p)
	shortLockWait(t)
	calls := 0
	for _, dry := range []bool{false, true} {
		r, err := admit(p, dry, envWith(bigFree, &calls))
		if err != nil || r.Outcome != "checked-clean" || r.Admission != "precheck" || r.DryRun != dry ||
			r.After.LogicalBytes == nil || *r.After.LogicalBytes != 100 || r.After.AvailableBytes == nil || *r.After.AvailableBytes != bigFree {
			t.Fatalf("cheap-clean cache not admitted lock-free: %+v %v", r, err)
		}
	}
	if calls != 0 {
		t.Fatalf("locked path ran %d times for a cheap-clean cache", calls)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("cheap admission rewrote the report")
	}
	// The public entry point agrees, with the real statfs and the real Check
	// behind it: a busy lock would surface as a deferral.
	r, err := Admit(p, false)
	if err != nil || r.Admission != "precheck" {
		t.Fatalf("Admit took the lock or failed: %+v %v", r, err)
	}
}

// regression: #3045
func TestCacheAdmitTakesLock(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, p *Policy)
		env   func(*admitEnv)
		free  uint64
		want  string
	}{
		{name: "over budget", want: "within a quarter of budget", setup: func(t *testing.T, p *Policy) {
			// A recorded size over budget cannot come from a clean pass (the
			// pass reclaims it), so doctor one: the check must not lean on it.
			recordedPolicy(t, *p, func(r *Report) { n := int64(admitBudget + 10); r.After.LogicalBytes = &n })
		}},
		{name: "near budget", want: "within a quarter of budget", setup: func(t *testing.T, p *Policy) {
			material(t, *p, "build/near", admitBudget/4*3)
			if _, err := Check(*p, false); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "stale estimate", want: "stale", env: func(e *admitEnv) {
			e.now = func() time.Time { return time.Now().Add(AdmitMaxAge + time.Minute) }
		}},
		{name: "future estimate", want: "stale or future", env: func(e *admitEnv) {
			e.now = func() time.Time { return time.Now().Add(-time.Hour) }
		}},
		{name: "missing report", want: "no readable recorded measurement", setup: func(t *testing.T, p *Policy) {
			if err := os.Remove(p.ReportPath()); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unparseable report", want: "not parseable", setup: func(t *testing.T, p *Policy) {
			if err := os.WriteFile(p.ReportPath(), []byte("{not json"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unreadable report", want: "no readable recorded measurement", setup: func(t *testing.T, p *Policy) {
			if err := os.Chmod(p.ReportPath(), 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "linked report", want: "no readable recorded measurement", setup: func(t *testing.T, p *Policy) {
			if err := os.Remove(p.ReportPath()); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(p.Root, "owner"), p.ReportPath()); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "last pass not clean", want: "not a clean full measurement", setup: func(t *testing.T, p *Policy) {
			recordedPolicy(t, *p, func(r *Report) { r.Outcome = "storage-deferred" })
		}},
		{name: "last pass had failures", want: "not a clean full measurement", setup: func(t *testing.T, p *Policy) {
			recordedPolicy(t, *p, func(r *Report) { r.Failures = []string{"measure build: denied"} })
		}},
		{name: "report without size", want: "no size", setup: func(t *testing.T, p *Policy) {
			recordedPolicy(t, *p, func(r *Report) { r.After.LogicalBytes = nil })
		}},
		{name: "report for another root", want: "another root or domain", setup: func(t *testing.T, p *Policy) {
			recordedPolicy(t, *p, func(r *Report) { r.Policy.Root = filepath.Join(filepath.Dir(p.Root), "elsewhere") })
		}},
		{name: "ownership marker foreign", want: "ownership not proven", setup: func(t *testing.T, p *Policy) {
			if err := os.WriteFile(filepath.Join(p.Root, "owner"), []byte("other\n"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "build replaced by link", want: "not an owned directory", setup: func(t *testing.T, p *Policy) {
			if err := os.RemoveAll(filepath.Join(p.Root, "build")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), filepath.Join(p.Root, "build")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "top level changed since report", want: "changed after the recorded measurement", setup: func(t *testing.T, p *Policy) {
			time.Sleep(10 * time.Millisecond)
			material(t, *p, "build/late", 10)
		}},
		{name: "free space at floor", want: "free space", free: 1 << 30, setup: func(t *testing.T, p *Policy) {
			p.Floor = 1 << 30
		}},
		{name: "free space inside margin", want: "free space", free: 1<<30 + admitBudget/4 - 1, setup: func(t *testing.T, p *Policy) {
			p.Floor = 1 << 30
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := warmed(t)
			if c.setup != nil {
				c.setup(t, &p)
			}
			free := c.free
			if free == 0 {
				free = bigFree
			}
			calls := 0
			env := envWith(free, &calls)
			if c.env != nil {
				c.env(&env)
			}
			if _, why := cheapClean(p, false, env); !strings.Contains(why, c.want) {
				t.Fatalf("cheap test verdict %q, want it to say %q", why, c.want)
			}
			r, err := admit(p, false, env)
			if calls != 1 || err != nil || !strings.HasPrefix(r.Admission, "locked: ") || !strings.Contains(r.Admission, c.want) {
				t.Fatalf("calls=%d: not routed to the locked path: %+v %v", calls, r, err)
			}
		})
	}
	// The boundary itself is cheap-clean: floor plus a quarter of the budget.
	p := warmed(t)
	p.Floor = 1 << 30
	calls := 0
	if r, why := cheapClean(p, false, envWith(1<<30+admitBudget/4, &calls)); why != "" || r.Admission != "precheck" {
		t.Fatalf("exact margin not admitted: %q", why)
	}
}

// regression: #3045
func TestCacheAdmitStatfsFails(t *testing.T) {
	p := warmed(t)
	failing := func(string) (uint64, error) { return 0, errors.New("statfs unavailable") }
	calls := 0
	env := admitEnv{free: failing, now: time.Now, locked: lockedProbe(&calls)}
	r, err := admit(p, false, env)
	if calls != 1 || err != nil || !strings.Contains(r.Admission, "available bytes") {
		t.Fatalf("statfs failure did not take the locked path: calls=%d %+v %v", calls, r, err)
	}
	// With the real locked path behind it and the lock busy, the failure ends
	// as a deferral: admission is never granted without the lock.
	holdCacheLock(t, p)
	shortLockWait(t)
	env.locked = Check
	r, err = admit(p, false, env)
	var d *Deferred
	if !errors.As(err, &d) || r.Outcome != "could-not-check" || d.Report.Admission == "" {
		t.Fatalf("admitted without the lock on a statfs failure: %+v %v", r, err)
	}
}

// regression: #3045
func TestCacheAdmitLockedEnforces(t *testing.T) {
	p := fixture(t)
	p.Budget = 1000
	// Cold, unrecorded root: nothing to lean on, so the full check runs and
	// still reclaims over-budget material with no consumer active.
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/a", 5000)
	if err := os.Remove(p.ReportPath()); err != nil {
		t.Fatal(err)
	}
	r, err := Admit(p, false)
	if err != nil || r.Reclaimed != 5000 || !strings.HasPrefix(r.Admission, "locked: ") {
		t.Fatalf("locked path changed behaviour: %+v %v", r, err)
	}
	// A live consumer protects mod/ exactly as before: still deferred.
	l, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Finish(true)
	material(t, p, "mod/m", 5000)
	if err := os.Remove(p.ReportPath()); err != nil {
		t.Fatal(err)
	}
	r, err = Admit(p, false)
	var d *Deferred
	if !errors.As(err, &d) || r.Outcome != "storage-deferred" || len(r.SkippedActive) != 1 {
		t.Fatalf("active consumer not protected: %+v %v", r, err)
	}
}

// regression: #2791
func TestCacheSkipsSecondWalk(t *testing.T) {
	p := fixture(t)
	p.Budget = 1 << 20
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/a", 100)
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.close()
	// measure reads free space once per walk, so the count is the walk count.
	walks := 0
	m.free = func(string) (uint64, error) { walks++; return bigFree, nil }
	r := m.check(false)
	if walks != 1 || r.Outcome != "checked-clean" || *r.After.LogicalBytes != 100 || *r.Before.LogicalBytes != 100 {
		t.Fatalf("clean pass walked %d times: %+v", walks, r)
	}
	// A pass that removes something still re-measures, so After is real.
	m.p.Budget = 10
	walks = 0
	r = m.check(false)
	if walks != 2 || r.Reclaimed != 100 || *r.After.LogicalBytes != 0 {
		t.Fatalf("removal pass did not re-measure: walks=%d %+v", walks, r)
	}
	// A removal that fails part way still re-measures: its effect is unknown.
	material(t, p, "build/b", 100)
	m.remove = func(string) error { return errors.New("removal denied") }
	walks = 0
	r = m.check(false)
	if walks != 2 || r.Outcome != "could-not-check" {
		t.Fatalf("failed removal skipped the re-measure: walks=%d %+v", walks, r)
	}
}
