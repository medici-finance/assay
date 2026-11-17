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

// remeasureLines returns the `launch cache re-measure:` lines in stderr; each
// is one Check a desk or scratch launch ran.
func remeasureLines(stderr string) []string {
	var out []string
	for _, l := range strings.Split(stderr, "\n") {
		if strings.HasPrefix(l, "launch cache re-measure: ") {
			out = append(out, l)
		}
	}
	return out
}

// staleReport rewrites the recorded measurement so the bounded pre-check
// cannot lean on it: aged, unstamped, future-dated, or gone.
func staleReport(t *testing.T, p cellcache.Policy, kind string) {
	t.Helper()
	if kind == "missing" {
		must(t, os.Remove(p.ReportPath()))
		return
	}
	raw, err := os.ReadFile(p.ReportPath())
	must(t, err)
	var r cellcache.Report
	must(t, json.Unmarshal(raw, &r))
	switch kind {
	case "stale":
		r.After.At = time.Now().Add(-time.Hour)
	case "future":
		r.After.At = time.Now().Add(time.Hour)
	case "zero":
		r.After.At = time.Time{}
	}
	raw, err = json.Marshal(r)
	must(t, err)
	must(t, os.WriteFile(p.ReportPath(), raw, 0600))
}

// A desk or scratch launch whose only refusal is an unstamped, future-dated or
// missing measurement re-measures once and admits on the fresh numbers. A
// measurement that is only stale is not refused at all (#6723): the launch
// admits on it with no re-measure. The cache is well under budget throughout.
//
// regression: #6593
func TestLaunchRemeasuresStale(t *testing.T) {
	for _, kind := range []string{"stale", "future", "zero", "missing"} {
		t.Run(kind, func(t *testing.T) {
			c, p := admissionCell(t)
			staleReport(t, p, kind)
			before, _ := os.ReadFile(p.ReportPath())
			var err error
			stderr := captureStderrOf(t, func() { _, err = c.launchPrepareLaunch() })
			if err != nil {
				t.Fatalf("%s: launch did not admit after re-measuring: %v\n%s", kind, err, stderr)
			}
			lines := remeasureLines(stderr)
			if kind == "stale" {
				after, _ := os.ReadFile(p.ReportPath())
				if len(lines) != 0 || string(before) != string(after) {
					t.Fatalf("stale: launch re-measured a stale-only measurement:\n%s", stderr)
				}
				return
			}
			if len(lines) != 1 || !strings.Contains(lines[0], "outcome checked-clean") {
				t.Fatalf("%s: want exactly one re-measure line, got:\n%s", kind, stderr)
			}
			raw, err := os.ReadFile(p.ReportPath())
			must(t, err)
			var r cellcache.Report
			must(t, json.Unmarshal(raw, &r))
			if age := time.Since(r.After.At); age < 0 || age > time.Minute {
				t.Fatalf("%s: re-measure did not record a fresh measurement: %v", kind, r.After.At)
			}
		})
	}
}

// After the one re-measure the fresh numbers decide: an unstamped measurement
// over a cache that really is near its budget refuses, with exactly one Check
// run and no second attempt.
//
// regression: #6593
func TestLaunchRemeasureRefusesOnce(t *testing.T) {
	c, p := admissionCell(t)
	nearBudget(t, p)
	staleReport(t, p, "zero")
	var err error
	stderr := captureStderrOf(t, func() { _, err = c.launchPrepareLaunch() })
	var d *cellcache.Deferred
	if !errors.As(err, &d) {
		t.Fatalf("launch admitted a cache still over its margin: %v\n%s", err, stderr)
	}
	if lines := remeasureLines(stderr); len(lines) != 1 {
		t.Fatalf("want exactly one re-measure, got %d:\n%s", len(lines), stderr)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "build", "near")); err != nil {
		t.Fatalf("re-measure removed data under the budget: %v", err)
	}
}

// A desk or scratch launch re-measures only a measurement it cannot lean on.
// Size pressure on a fresh measurement, and every operator-only refusal, are
// returned untouched: no Check runs and the recorded report is not rewritten.
//
// regression: #6593
func TestLaunchKeepsOtherRefusals(t *testing.T) {
	for _, kind := range []string{"size", "corrupt", "dirty-pass"} {
		t.Run(kind, func(t *testing.T) {
			c, p := admissionCell(t)
			switch kind {
			case "size":
				nearBudget(t, p)
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
			stderr := captureStderrOf(t, func() { _, err = c.launchPrepareLaunch() })
			if err == nil || !strings.Contains(err.Error(), "run cellctl cache <cell> clean") {
				t.Fatalf("%s: want the operator refusal, got %v", kind, err)
			}
			if lines := remeasureLines(stderr); len(lines) != 0 {
				t.Fatalf("%s: launch walked a cache it must leave alone: %v", kind, lines)
			}
			after, _ := os.ReadFile(p.ReportPath())
			if string(before) != string(after) {
				t.Fatalf("%s: launch rewrote the recorded measurement", kind)
			}
		})
	}
}
