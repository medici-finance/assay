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

// upCleanLines returns the `up cache clean:` lines in stderr; each is one
// Check that up ran.
func upCleanLines(stderr string) []string {
	var out []string
	for _, l := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(l, "up cache clean: ") {
			out = append(out, l)
		}
	}
	return out
}

// nearBudget records 900000 of the cell's 1048576 logical bytes in a clean
// pass: over three quarters of the budget, and young enough that no trim
// takes it, so a further clean cannot bring it under.
func nearBudget(t *testing.T, p cellcache.Policy) {
	t.Helper()
	must(t, os.MkdirAll(filepath.Join(p.Root, "build"), 0700))
	must(t, os.WriteFile(filepath.Join(p.Root, "build", "near"), make([]byte, 900000), 0600))
	time.Sleep(cellcache.AdmitMtimeSlack)
	if _, err := cellcache.Check(p, false); err != nil {
		t.Fatal(err)
	}
}

// When bounded admission defers on the recorded size, `up` runs one clean,
// names the bytes before and after on its one line, and prepares again; a
// clean that clears the pressure admits.
//
// regression: #6117
func TestCacheUpCleansSizeDeferralOnce(t *testing.T) {
	c, p := admissionCell(t)
	nearBudget(t, p)
	// The cache grew past what the cell now allows: the next admission defers
	// on size, and a clean is under pressure with no consumer, so it removes
	// the compiler cache whole.
	c.Env.Put("CELL_GO_CACHE_BYTES", "800000")
	var err error
	stderr := captureStderrOf(t, func() { _, err = c.upPrepareLaunch() })
	if err != nil {
		t.Fatalf("up did not admit after its clean: %v\n%s", err, stderr)
	}
	lines := upCleanLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("want exactly one clean line, got %d:\n%s", len(lines), stderr)
	}
	for _, want := range []string{"recorded size 900000 is within a quarter of budget 800000", "logical bytes before 900000, after 0", "outcome checked-clean"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("clean line does not name %q: %s", want, lines[0])
		}
	}
	if strings.Contains(lines[0], "run cellctl cache") {
		t.Fatalf("clean line repeats the operator instruction: %s", lines[0])
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "near")); !os.IsNotExist(err) {
		t.Fatalf("clean left the over-budget entry: %v", err)
	}
}

// A stale measurement is reclaimable too: one clean re-measures and admits.
//
// regression: #6117
func TestCacheUpCleansStaleDeferral(t *testing.T) {
	c, p := admissionCell(t)
	raw, err := os.ReadFile(p.ReportPath())
	must(t, err)
	var r cellcache.Report
	must(t, json.Unmarshal(raw, &r))
	r.After.At = time.Now().Add(-time.Hour)
	raw, err = json.Marshal(r)
	must(t, err)
	must(t, os.WriteFile(p.ReportPath(), raw, 0600))
	stderr := captureStderrOf(t, func() { _, err = c.upPrepareLaunch() })
	if err != nil {
		t.Fatalf("up did not admit after re-measuring: %v\n%s", err, stderr)
	}
	lines := upCleanLines(stderr)
	if len(lines) != 1 || !strings.Contains(lines[0], "recorded measurement is stale") || !strings.Contains(lines[0], "logical bytes before 0, after 0") {
		t.Fatalf("want one clean line naming the stale measurement:\n%s", stderr)
	}
}

// Still over budget after the clean, `up` refuses as before, with the clean
// run exactly once. The bounded launch path desk windows take is unchanged:
// it refuses the same cache without walking it.
//
// regression: #6117
func TestCacheUpStillOverBudgetRefuses(t *testing.T) {
	c, p := admissionCell(t)
	nearBudget(t, p)
	before, err := os.ReadFile(p.ReportPath())
	must(t, err)
	if _, err := c.prepareLaunch(); err == nil {
		t.Fatal("bounded launch admitted a near-budget cache")
	}
	after, err := os.ReadFile(p.ReportPath())
	must(t, err)
	if string(before) != string(after) {
		t.Fatal("bounded launch preparation walked the cache")
	}
	stderr := captureStderrOf(t, func() { _, err = c.upPrepareLaunch() })
	var d *cellcache.Deferred
	if !errors.As(err, &d) {
		t.Fatalf("up admitted a cache still over its margin: %v\n%s", err, stderr)
	}
	lines := upCleanLines(stderr)
	if len(lines) != 1 {
		t.Fatalf("want exactly one clean, got %d:\n%s", len(lines), stderr)
	}
	if !strings.Contains(lines[0], "logical bytes before 900000, after 900000") {
		t.Fatalf("clean line does not name the unchanged size: %s", lines[0])
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "near")); err != nil {
		t.Fatalf("clean removed data under the budget: %v", err)
	}
}

// A refusal no clean can clear is returned untouched: no Check runs, no
// report is written, and the operator instruction stands.
//
// regression: #6117
func TestCacheUpLeavesOtherRefusalsToTheOperator(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "dirty-pass"} {
		t.Run(kind, func(t *testing.T) {
			c, p := admissionCell(t)
			switch kind {
			case "missing":
				must(t, os.Remove(p.ReportPath()))
			case "corrupt":
				must(t, os.WriteFile(p.ReportPath(), []byte("broken"), 0600))
			case "dirty-pass":
				raw, err := os.ReadFile(p.ReportPath())
				must(t, err)
				var r cellcache.Report
				must(t, json.Unmarshal(raw, &r))
				r.Outcome = "storage-deferred"
				raw, err = json.Marshal(r)
				must(t, err)
				must(t, os.WriteFile(p.ReportPath(), raw, 0600))
			}
			before, _ := os.ReadFile(p.ReportPath())
			var err error
			stderr := captureStderrOf(t, func() { _, err = c.upPrepareLaunch() })
			if err == nil || !strings.Contains(err.Error(), "run cellctl cache <cell> clean") {
				t.Fatalf("%s: want the operator refusal, got %v", kind, err)
			}
			if lines := upCleanLines(stderr); len(lines) != 0 {
				t.Fatalf("%s: up cleaned a cache it must leave to the operator: %v", kind, lines)
			}
			after, _ := os.ReadFile(p.ReportPath())
			if string(before) != string(after) {
				t.Fatalf("%s: up rewrote the recorded measurement", kind)
			}
		})
	}
}

// The real `cellctl up` binary takes the clean: a cell whose recorded size has
// outgrown its budget gets one clean and opens its window; a cell still over
// its margin after the clean opens none and exits as before.
//
// regression: #6117
func TestCacheUpBinaryCleansOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		budget string
		admit  bool
		bytes  string
	}{
		{"cleared", "800000", true, "logical bytes before 900000, after 0"},
		{"still over", "1048576", false, "logical bytes before 900000, after 900000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newUpWorld(t, "worker-desk", 10*time.Millisecond)
			root := filepath.Join(w.root, "gocache")
			cache := "\nCELL_GO_CACHE=on\nCELL_GO_CACHE_ROOT=" + root + "\nCELL_GO_CACHE_MIN_FREE=0\nCELL_GO_CACHE_BYTES="
			w.writeHouseEnv(t, "ROLES=\"worker-desk\""+cache+"1048576\n")
			if r := w.run(t, nil, "cache", "example", "clean"); r.Code != 0 {
				t.Fatalf("first clean exit=%d stderr=%s", r.Code, r.Stderr)
			}
			must(t, os.MkdirAll(filepath.Join(root, "build"), 0700))
			must(t, os.WriteFile(filepath.Join(root, "build", "near"), make([]byte, 900000), 0600))
			time.Sleep(cellcache.AdmitMtimeSlack)
			if r := w.run(t, nil, "cache", "example", "clean"); r.Code != 0 {
				t.Fatalf("recording clean exit=%d stderr=%s", r.Code, r.Stderr)
			}
			w.writeHouseEnv(t, "ROLES=\"worker-desk\""+cache+tc.budget+"\n")
			r := w.up(t, "--role", "worker-desk")
			lines := upCleanLines(r.Stderr)
			if len(lines) != 1 || !strings.Contains(lines[0], tc.bytes) {
				t.Fatalf("want one clean line naming %q, got exit=%d stderr=%s", tc.bytes, r.Code, r.Stderr)
			}
			started := strings.Contains(strings.Join(w.timeline(t), "\n"), "start worker-desk")
			if tc.admit && (r.Code != 0 || !started) {
				t.Fatalf("up did not open its window after the clean: exit=%d started=%v stderr=%s", r.Code, started, r.Stderr)
			}
			if !tc.admit && (r.Code == 0 || started || !strings.Contains(r.Stderr, "no windows launched")) {
				t.Fatalf("up launched past a cache still over its margin: exit=%d started=%v stderr=%s", r.Code, started, r.Stderr)
			}
		})
	}
}
