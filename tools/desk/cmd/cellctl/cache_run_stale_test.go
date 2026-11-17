//go:build darwin || linux

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

// cacheRunMeasured is an enabled cache holding data, measured by explicit
// maintenance, whose recorded measurement is then dated at; its top-level
// directories predate that time, so only the recorded time decides admission.
func cacheRunMeasured(t *testing.T, at time.Time) cellcache.Policy {
	t.Helper()
	cell, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := cellcache.Resolve(cell, func(k string) string {
		switch k {
		case "CELL_GO_CACHE":
			return "on"
		case "CELL_GO_CACHE_BYTES":
			return "1048576"
		case "CELL_GO_CACHE_MIN_FREE":
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cellcache.Prepare(*p); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Root, "build", "example-output"), []byte("example"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-3 * time.Hour)
	for _, name := range []string{"build", "mod"} {
		if err = os.Chtimes(filepath.Join(p.Root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = cellcache.Check(*p, false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ReportPath())
	if err != nil {
		t.Fatal(err)
	}
	var rep cellcache.Report
	if err = json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	rep.After.At = at
	if b, err = json.Marshal(rep); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(p.ReportPath(), b, 0600); err != nil {
		t.Fatal(err)
	}
	return *p
}

// regression: #6723
//
// `cellctl cache-run`, end to end through the binary: a stale-only measurement
// runs the child and says it is stale with its age; an unstamped or
// future-dated one still refuses with exit 6 and runs nothing.
func TestCacheRunStaleAdmits(t *testing.T) {
	w := newCLIWorld(t, "")
	for _, tc := range []struct {
		kind  string
		at    time.Time
		admit bool
	}{
		{"stale", time.Now().Add(-time.Hour), true},
		{"zero", time.Time{}, false},
		{"future", time.Now().Add(time.Hour), false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			p := cacheRunMeasured(t, tc.at)
			marker := filepath.Join(filepath.Dir(p.Root), "child-ran")
			r := w.run(t, p.Env(), "cache-run", "/bin/sh", "-c", `: > "$1"`, "sh", marker)
			_, statErr := os.Stat(marker)
			if !tc.admit {
				if r.Code != 6 || !errors.Is(statErr, os.ErrNotExist) || !strings.Contains(r.Stderr, "unstamped or future-dated") {
					t.Fatalf("cache-run on a %s measurement: exit %d, child ran=%v\n%s", tc.kind, r.Code, statErr == nil, r.Stderr)
				}
				return
			}
			if r.Code != 0 || statErr != nil {
				t.Fatalf("cache-run refused a stale-only measurement: exit %d (%v)\n%s", r.Code, statErr, r.Stderr)
			}
			if !strings.Contains(r.Stderr, "cache-enrollment: recorded measurement is stale: 1h0m") {
				t.Fatalf("cache-run did not report the stale age:\n%s", r.Stderr)
			}
		})
	}
}
