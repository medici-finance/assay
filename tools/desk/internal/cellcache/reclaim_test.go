//go:build darwin || linux

package cellcache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// reclaimableRefusal reports whether err is a bounded launch deferral that one
// clean may clear.
func reclaimableRefusal(err error) bool {
	var d *Deferred
	return errors.As(err, &d) && d.Report.Reclaimable()
}

// staleRefusal reports whether err is a bounded launch deferral whose only
// reason is that the recorded measurement cannot be leaned on.
func staleRefusal(err error) bool {
	var d *Deferred
	return errors.As(err, &d) && d.Report.Stale()
}

// Prepare marks a bounded refusal reclaimable only when the recorded size or
// free-space margin, an unstamped, future-dated or missing measurement, one
// older than the policy's freshness bound, or a top-level change since it is
// the reason. Corrupt, dirty, foreign and empty-cache-short refusals are never
// reclaimable: those stay with the operator. Only the measurement-validity
// reasons (past the bound, unstamped, future-dated, missing) are also marked
// stale, the one mark a desk or scratch launch re-measures on. A measurement
// only older than five minutes, with no bound, admits (TestCacheStaleAdmits).
//
// regression: #6117
// regression: #6593
func TestCacheReclaimableRefusals(t *testing.T) {
	cases := []struct {
		kind  string
		want  bool
		stale bool
	}{
		{"size", true, false}, {"stale-bound", true, true}, {"changed", true, false}, {"space", true, false},
		{"missing", true, true}, {"future", true, true}, {"zero", true, true},
		{"corrupt", false, false}, {"dirty-pass", false, false},
		{"domain", false, false}, {"empty-space", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			// The size and space cases each settle the clock margin; run the
			// cases side by side so the cache gate's bounded run stays bounded.
			t.Parallel()
			p := fixture(t)
			p.Budget = 1 << 20
			if tc.kind == "empty-space" {
				// A cache this call creates is admitted empty or not at all.
				p.Floor = ^uint64(0) - uint64(p.Budget)
				_, err := Prepare(p)
				if err == nil || reclaimableRefusal(err) {
					t.Fatalf("empty cache short of space: err=%v reclaimable=%v", err, reclaimableRefusal(err))
				}
				return
			}
			if _, err := Prepare(p); err != nil {
				t.Fatal(err)
			}
			rewrite := func(edit func(*Report)) {
				t.Helper()
				b, err := os.ReadFile(p.ReportPath())
				if err != nil {
					t.Fatal(err)
				}
				var r Report
				if err = json.Unmarshal(b, &r); err != nil {
					t.Fatal(err)
				}
				edit(&r)
				if b, err = json.Marshal(r); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(p.ReportPath(), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch tc.kind {
			case "size":
				// 900000 of 1048576 logical bytes is over three quarters of the
				// budget; a young file is not trimmed, so the pass stays clean.
				material(t, p, "build/near", 900000)
				time.Sleep(AdmitMtimeSlack)
				if _, err := Check(p, false); err != nil {
					t.Fatal(err)
				}
			case "stale-bound":
				// Age alone admits; only a freshness bound refuses it.
				p.MeasurementMaxAge = 600
				rewrite(func(r *Report) { r.After.At = time.Now().Add(-time.Hour) })
			case "changed":
				material(t, p, "build/new", 1)
			case "space":
				// A measured, nonempty cache whose free-space margin fails.
				material(t, p, "build/small", 1000)
				time.Sleep(AdmitMtimeSlack)
				if _, err := Check(p, false); err != nil {
					t.Fatal(err)
				}
				p.Floor = ^uint64(0) - uint64(p.Budget)
			case "missing":
				if err := os.Remove(p.ReportPath()); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(p.ReportPath(), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "future":
				rewrite(func(r *Report) { r.After.At = time.Now().Add(time.Hour) })
			case "zero":
				rewrite(func(r *Report) { r.After.At = time.Time{} })
			case "dirty-pass":
				rewrite(func(r *Report) { r.Outcome = "storage-deferred" })
			case "domain":
				p.Domain = strings.Repeat("a", 64)
			}
			r, err := Prepare(p)
			if err == nil {
				t.Fatalf("%s: bounded admission admitted: %+v", tc.kind, r)
			}
			if got := reclaimableRefusal(err); got != tc.want {
				t.Fatalf("%s: reclaimable=%v, want %v (%v)", tc.kind, got, tc.want, err)
			}
			if tc.want && r.Reclaimable() != tc.want {
				t.Fatalf("%s: returned report disagrees with its deferral", tc.kind)
			}
			if got := staleRefusal(err); got != tc.stale || r.Stale() != tc.stale {
				t.Fatalf("%s: stale=%v report=%v, want %v (%v)", tc.kind, got, r.Stale(), tc.stale, err)
			}
			// The mark is in-process only: the report's form is unchanged.
			b, _ := json.Marshal(r)
			if strings.Contains(string(b), `"reclaimable"`) || strings.Contains(string(b), `"stale"`) {
				t.Fatalf("an in-process mark leaked into the report: %s", b)
			}
		})
	}
	// A clean admission is not a refusal of any kind.
	p := fixture(t)
	if r, err := Prepare(p); err != nil || r.Reclaimable() || r.Stale() {
		t.Fatalf("clean admission: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build")); err != nil {
		t.Fatal(err)
	}
}
