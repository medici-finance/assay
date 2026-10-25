//go:build darwin || linux

package cellcache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #3353
func TestCacheScanAdmission(t *testing.T) {
	p := fixture(t)
	_, err := Prepare(p)
	if err != nil {
		t.Fatal(err)
	}
	// Warm immutable top-level directories make the existing recorded admission
	// usable without waiving the filesystem timestamp margin.
	old := time.Now().Add(-time.Hour)
	for _, name := range []string{"build", "mod"} {
		if err = os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = Check(p, false); err != nil {
		t.Fatal(err)
	}
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.close()
	paused, resume := make(chan struct{}), make(chan struct{})
	calls := 0
	m.free = func(string) (uint64, error) {
		calls++
		if calls == 1 {
			close(paused)
			<-resume
		}
		return bigFree, nil
	}
	done := make(chan Report, 1)
	go func() { done <- m.check(false) }()
	<-paused
	if m.lock != nil {
		// Prove the fixture detects the old global lock before doing a
		// second enrollment. Release only the deliberately broken fixture's
		// lock so its worker can finish rather than timing out the test.
		m.lock.Release()
		close(resume)
		<-done
		t.Fatal("maintenance retained admission custody during inventory")
	}
	lease, err := Acquire(p.Env())
	if err != nil {
		close(resume)
		<-done
		t.Fatalf("maintenance held admission lock: %v", err)
	}
	// A consumer admitted during the scan changes data and exits before the
	// maintenance transaction resumes. Active-record listing alone misses this.
	material(t, p, "build/new-output", 10)
	if err = lease.Finish(true); err != nil {
		t.Fatal(err)
	}
	close(resume)
	r := <-done
	if r.Outcome != "could-not-check" || r.Reclaimed != 0 || !strings.Contains(strings.Join(r.Failures, " "), "enrollment changed") {
		t.Fatalf("stale inventory gained authority: %+v", r)
	}
	b, err := os.ReadFile(filepath.Join(p.Root, "build/new-output"))
	if err != nil || !bytes.Equal(b, bytes.Repeat([]byte("x"), 10)) {
		t.Fatal("new consumer data removed", err)
	}
}

// regression: #3353
func TestCacheAdmissionFences(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "stale", "dirty", "space", "domain"} {
		t.Run(kind, func(t *testing.T) {
			p := fixture(t)
			_, err := Prepare(p)
			if err != nil {
				t.Fatal(err)
			}
			var report Report
			b, err := os.ReadFile(p.ReportPath())
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(b, &report); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "missing":
				if err = os.Remove(p.ReportPath()); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err = os.WriteFile(p.ReportPath(), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "stale":
				report.After.At = time.Now().Add(-time.Hour)
				b, _ = json.Marshal(report)
				if err = os.WriteFile(p.ReportPath(), b, 0600); err != nil {
					t.Fatal(err)
				}
			case "dirty":
				material(t, p, "build/new", 1)
			case "space":
				p.Floor = ^uint64(0) - 1
			case "domain":
				p.Domain = strings.Repeat("a", 64)
			}
			if l, e := Acquire(p.Env()); e == nil {
				l.Finish(true)
				t.Fatal("unproven accounting admitted a child")
			}
		})
	}
}

// regression: #3353
func TestMaintenanceScanBound(t *testing.T) {
	p := fixture(t)
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 65; i++ {
		if err := os.WriteFile(filepath.Join(p.Root, "mod", fmt.Sprintf("wide-%03d", i)), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.close()
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "32")
	rep := m.check(false)
	if rep.Outcome != "could-not-check" || rep.Before.LogicalBytes != nil || rep.Reclaimed != 0 {
		t.Fatalf("incomplete inventory gained authority: %+v", rep)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "mod", "wide-000")); err != nil {
		t.Fatal(err)
	}
	m.lock.Release()
	m.lock = nil
	if _, err := Acquire(p.Env()); err == nil {
		t.Fatal("incomplete accounting admitted consumer")
	}
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "256")
	time.Sleep(AdmitMtimeSlack)
	rep, err = Check(p, false)
	if err != nil || rep.Outcome != "checked-clean" || rep.Before.LogicalBytes == nil {
		t.Fatalf("finite raised-budget recovery failed: %+v %v", rep, err)
	}
	lease, err := Acquire(p.Env())
	if err != nil {
		t.Fatalf("recovered accounting refused: %v", err)
	}
	if err := lease.Finish(true); err != nil {
		t.Fatal(err)
	}

}

// regression: #3353
func TestCacheMaintenanceRecovery(t *testing.T) {
	p := fixture(t)
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(p.Env())
	if err != nil {
		t.Fatal(err)
	}
	material(t, p, "build/consumer-result", 1)
	if err := lease.Finish(true); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(p.Env()); err == nil || !strings.Contains(err.Error(), "changed after the recorded measurement") {
		t.Fatalf("dirty accounting did not explicitly refuse: %v", err)
	}
	time.Sleep(AdmitMtimeSlack)
	rep, err := Check(p, false)
	if err != nil {
		t.Fatal(err)
	}
	lease, err = Acquire(p.Env())
	if err != nil {
		t.Fatalf("explicit maintenance did not recover admission: %v", err)
	}
	if err := lease.Finish(true); err != nil {
		t.Fatal(err)
	}
	// Cross the actual age boundary through the persisted production report,
	// rather than sleeping five minutes or introducing a cached success seam.
	rep.After.At = time.Now().Add(-AdmitMaxAge - time.Second)
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Root, "report.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(p.Env()); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("aged accounting did not explicitly refuse: %v", err)
	}
	if _, err := Check(p, false); err != nil {
		t.Fatal(err)
	}
	lease, err = Acquire(p.Env())
	if err != nil {
		t.Fatalf("aged-report explicit maintenance did not recover: %v", err)
	}
	if err := lease.Finish(true); err != nil {
		t.Fatal(err)
	}
}

// regression: #3353
func TestCacheMaintenanceRealWidth(t *testing.T) {
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "")
	p := fixture(t)
	if _, err := Prepare(p); err != nil {
		t.Fatal(err)
	}
	// Fixed representative widths from review evidence, not a host-cache probe.
	// Empty regular files isolate directory-width accounting without allocating
	// a compiler cache's payload bytes; existing budget tests own byte pressure.
	for _, row := range []struct {
		tree string
		n    int
	}{{"build", 150000}, {"mod", 75000}} {
		for i := 0; i < row.n; i++ {
			if err := os.WriteFile(filepath.Join(p.Root, row.tree, fmt.Sprintf("entry-%06d", i)), nil, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	time.Sleep(AdmitMtimeSlack)
	rep, err := Check(p, false)
	if err != nil || rep.Outcome != "checked-clean" || rep.Before.LogicalBytes == nil || rep.Reclaimed != 0 {
		t.Fatalf("representative width exceeded default: %+v %v", rep, err)
	}
	lease, err := Acquire(p.Env())
	if err != nil {
		t.Fatalf("representative width did not admit after maintenance: %v", err)
	}
	if err := lease.Finish(true); err != nil {
		t.Fatal(err)
	}
}
