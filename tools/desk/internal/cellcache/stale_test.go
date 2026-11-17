//go:build darwin || linux

package cellcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// measured is a cache with data in it, measured by explicit maintenance, whose
// top-level directories predate any timestamp a test later records: only the
// recorded time decides admission, never the empty-root or changed-since path.
func measured(t *testing.T) Policy {
	t.Helper()
	p := fixture(t)
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/example-output", 100)
	old := time.Now().Add(-2 * time.Hour)
	for _, name := range []string{"build", "mod"} {
		if err := os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	return p
}

// recordAt rewrites the recorded measurement's timestamp (zero: unstamped)
// and returns the report bytes as left on disk.
func recordAt(t *testing.T, p Policy, at time.Time) []byte {
	t.Helper()
	b, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	var r Report
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	r.After.At = at
	if b, err = json.Marshal(r); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p.ReportPath(), b, 0600); err != nil {
		t.Fatal(err)
	}
	return b
}

// regression: #6723
//
// A measurement whose only defect is age is stale, not invalid: bounded launch
// preparation (Prepare) and final enrollment (Acquire) both admit on it,
// report it stale with its age, and leave report.json as they found it.
func TestCacheStaleAdmits(t *testing.T) {
	p := measured(t)
	recorded := time.Now().Add(-time.Hour)
	before := recordAt(t, p, recorded)
	r, err := Prepare(p)
	if err != nil {
		t.Fatalf("preparation refused a stale-only measurement: %v", err)
	}
	if r.Admission != "precheck-stale" || r.Outcome != "checked-clean" {
		t.Fatalf("stale preparation = %s/%s, want precheck-stale/checked-clean", r.Admission, r.Outcome)
	}
	if !strings.Contains(r.Reason, "stale: 1h0m") {
		t.Fatalf("stale preparation does not report its age: %q", r.Reason)
	}
	if !r.After.At.Equal(recorded) {
		t.Fatalf("stale report claims a newer measurement: %v, recorded %v", r.After.At, recorded)
	}
	lease, err := Acquire(p.Env())
	if err != nil {
		t.Fatalf("enrollment refused a stale-only measurement: %v", err)
	}
	if !strings.Contains(lease.Stale(), "stale: 1h0m") {
		t.Fatalf("enrollment does not report the stale age: %q", lease.Stale())
	}
	if err = lease.Finish(true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(p.ReportPath())
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a stale admission re-measured: report.json changed (%v)", err)
	}
}

// regression: #6723
//
// An unstamped or future-dated measurement is invalid, not stale: both entry
// points still refuse it. Preparation marks the refusal as one fresh
// measurement decides (a cellctl launch re-measures once, #6593); enrollment
// never re-measures.
func TestCacheInvalidRefuses(t *testing.T) {
	for _, kind := range []string{"zero", "future"} {
		t.Run(kind, func(t *testing.T) {
			p := measured(t)
			at := time.Time{}
			if kind == "future" {
				at = time.Now().Add(time.Hour)
			}
			recordAt(t, p, at)
			_, err := Prepare(p)
			var d *Deferred
			if !errors.As(err, &d) || !d.Report.Stale() || !strings.Contains(err.Error(), "unstamped or future-dated") {
				t.Fatalf("preparation on a %s measurement: %v", kind, err)
			}
			if l, e := Acquire(p.Env()); e == nil {
				l.Finish(true)
				t.Fatalf("enrollment admitted a %s measurement", kind)
			}
		})
	}
}

// regression: #6723
//
// A caller that asks for a freshness bound gets one: a measurement older than
// the policy's bound refuses at both entry points, as a stale deferral (one
// fresh measurement decides; a cellctl launch re-measures once), and one
// inside it admits.
func TestCacheStaleBoundRefuses(t *testing.T) {
	p := measured(t)
	recordAt(t, p, time.Now().Add(-time.Hour))
	p.MeasurementMaxAge = int64((10 * time.Minute) / time.Second)
	_, err := Prepare(p)
	var d *Deferred
	if !errors.As(err, &d) || !d.Report.Stale() || !strings.Contains(err.Error(), "over the freshness bound 10m0s") {
		t.Fatalf("bounded preparation on an hour-old measurement: %v", err)
	}
	if l, e := Acquire(p.Env()); e == nil || !strings.Contains(e.Error(), "freshness bound") {
		l.Finish(true)
		t.Fatalf("bounded enrollment admitted an hour-old measurement: %v", e)
	}
	p.MeasurementMaxAge = int64((2 * time.Hour) / time.Second)
	if _, err = Prepare(p); err != nil {
		t.Fatalf("a measurement inside the bound refused: %v", err)
	}
}

// regression: #6723
//
// A stale measurement that also fails a reclaim check (a top-level change
// since it, or the size margin) is a refusal of the stale estimate, not of the
// cache: preparation marks it stale as well as reclaimable, so a cellctl
// launch re-measures once and the fresh numbers decide (#6593). A fresh
// measurement keeps the plain reclaim class (TestCacheReclaimableRefusals).
func TestCacheStaleReclaimIsStale(t *testing.T) {
	for _, kind := range []string{"changed", "size"} {
		t.Run(kind, func(t *testing.T) {
			p := measured(t)
			recordAt(t, p, time.Now().Add(-time.Hour))
			switch kind {
			case "changed":
				half := time.Now().Add(-30 * time.Minute)
				if err := os.Chtimes(filepath.Join(p.Root, "build"), half, half); err != nil {
					t.Fatal(err)
				}
			case "size":
				b, err := os.ReadFile(p.ReportPath())
				if err != nil {
					t.Fatal(err)
				}
				var r Report
				if err = json.Unmarshal(b, &r); err != nil {
					t.Fatal(err)
				}
				big := p.Budget
				r.After.LogicalBytes = &big
				if b, err = json.Marshal(r); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(p.ReportPath(), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Prepare(p)
			var d *Deferred
			if !errors.As(err, &d) || !d.Report.Stale() || !d.Report.Reclaimable() {
				t.Fatalf("stale measurement with a %s refusal is not stale-class: %v", kind, err)
			}
		})
	}
}

// regression: #6723
//
// A freshness bound under five minutes is applied by preparation, not only by
// enrollment: preparation refuses a measurement past it as stale-class, so a
// cellctl launch re-measures before any child reaches enrollment.
func TestCacheShortBoundPrepare(t *testing.T) {
	p := measured(t)
	recordAt(t, p, time.Now().Add(-2*time.Minute))
	p.MeasurementMaxAge = 60
	_, err := Prepare(p)
	var d *Deferred
	if !errors.As(err, &d) || !d.Report.Stale() || !strings.Contains(err.Error(), "over the freshness bound 1m0s") {
		t.Fatalf("preparation under a 60s bound on a 2m-old measurement: %v", err)
	}
}
