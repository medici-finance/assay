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

// Prepare marks a bounded refusal reclaimable only when the recorded size or
// free-space margin, an aged measurement, or a top-level change since it is the
// reason. Missing, corrupt, dirty, future-dated, foreign and empty-cache-short
// refusals are never reclaimable: those stay with the operator.
//
// regression: #6117
func TestCacheReclaimableRefusals(t *testing.T) {
	cases := []struct {
		kind string
		want bool
	}{
		{"size", true}, {"stale", true}, {"changed", true}, {"space", true},
		{"missing", false}, {"corrupt", false}, {"future", false}, {"dirty-pass", false},
		{"domain", false}, {"empty-space", false},
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
			case "stale":
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
			// The mark is in-process only: the report's form is unchanged.
			b, _ := json.Marshal(r)
			if strings.Contains(string(b), `"reclaimable"`) {
				t.Fatalf("reclaimable leaked into the report: %s", b)
			}
		})
	}
	// A clean admission is not a refusal of any kind.
	p := fixture(t)
	if r, err := Prepare(p); err != nil || r.Reclaimable() {
		t.Fatalf("clean admission: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build")); err != nil {
		t.Fatal(err)
	}
}
