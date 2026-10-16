//go:build darwin || linux

package cellcache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recordedEnv is envWith with the locked seam pointed at CheckRecorded's
// stand-in: a probe that counts calls.
func recordedEnv(calls *int) admitEnv { return envWith(bigFree, calls) }

// regression: #3102
func TestAdmitRecordedNoLock(t *testing.T) {
	p := warmed(t)
	before, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	// A sibling launch holds the cache lock for the whole call: a path that
	// takes it waits out LockWait and comes back deferred.
	holdCacheLock(t, p)
	shortLockWait(t)
	calls := 0
	for _, dry := range []bool{false, true} {
		r, err := admitRecorded(p.Root, dry, recordedEnv(&calls))
		if err != nil || r.Outcome != "checked-clean" || r.Admission != "precheck" || r.DryRun != dry {
			t.Fatalf("cheap-clean root not admitted lock-free: %+v %v", r, err)
		}
	}
	if calls != 0 {
		t.Fatalf("locked path ran %d times for a cheap-clean root", calls)
	}
	// The public entry point agrees, with the real statfs and the real
	// CheckRecorded behind it.
	r, err := AdmitRecorded(p.Root, false)
	if err != nil || r.Admission != "precheck" {
		t.Fatalf("AdmitRecorded took the lock or failed: %+v %v", r, err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil || string(before) != string(after) {
		t.Fatal("cheap admission rewrote the report")
	}
}

// regression: #3102
func TestAdmitRecordedLockedPath(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, p *Policy)
		env   func(*admitEnv)
		want  string
	}{
		{name: "near high water", want: "within a quarter of budget", setup: func(t *testing.T, p *Policy) {
			material(t, *p, "build/near", admitBudget/4*3)
			quiet(t, *p)
			if _, err := Check(*p, false); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "stale estimate", want: "stale", env: func(e *admitEnv) {
			e.now = func() time.Time { return time.Now().Add(AdmitMaxAge + time.Minute) }
		}},
		{name: "unparseable report", want: "policy not readable", setup: func(t *testing.T, p *Policy) {
			if err := os.WriteFile(p.ReportPath(), []byte("{not json"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing report", want: "policy not readable", setup: func(t *testing.T, p *Policy) {
			if err := os.Remove(p.ReportPath()); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "unreadable report", want: "policy not readable", setup: func(t *testing.T, p *Policy) {
			if err := os.Chmod(p.ReportPath(), 0); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "top level changed", want: "changed after the recorded measurement", setup: func(t *testing.T, p *Policy) {
			time.Sleep(10 * time.Millisecond)
			material(t, *p, "build/late", 10)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := warmed(t)
			if c.setup != nil {
				c.setup(t, &p)
			}
			calls := 0
			env := recordedEnv(&calls)
			if c.env != nil {
				c.env(&env)
			}
			r, err := admitRecorded(p.Root, false, env)
			if calls != 1 || err != nil || !strings.HasPrefix(r.Admission, "locked: ") || !strings.Contains(r.Admission, c.want) {
				t.Fatalf("calls=%d: not routed to the locked path: %+v %v", calls, r, err)
			}
		})
	}
	// An unmarked root has nothing to lean on either.
	calls := 0
	r, err := admitRecorded(filepath.Join(t.TempDir(), "go-cache"), false, recordedEnv(&calls))
	if calls != 1 || err != nil || !strings.HasPrefix(r.Admission, "locked: ") {
		t.Fatalf("unmarked root not routed to the locked path: calls=%d %+v %v", calls, r, err)
	}
}

// regression: #3102
func TestAdmitRecordedNeverUnlocked(t *testing.T) {
	// With the real locked path behind it and the lock busy, a root whose
	// pre-check cannot be proven ends as a deferral: it is never admitted
	// without the lock.
	p := warmed(t)
	if err := os.WriteFile(p.ReportPath(), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	holdCacheLock(t, p)
	shortLockWait(t)
	r, err := AdmitRecorded(p.Root, false)
	var d *Deferred
	if !errors.As(err, &d) || r.Outcome != "could-not-check" || !strings.HasPrefix(d.Report.Admission, "locked: ") {
		t.Fatalf("admitted without the lock on an unreadable pre-check: %+v %v", r, err)
	}
}
