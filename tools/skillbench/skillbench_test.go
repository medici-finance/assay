package main

import (
	"flag"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -update regenerates the golden report file rather than asserting against it.
var update = flag.Bool("update", false, "regenerate golden testdata files")

const (
	fxComplete = "fixtures/complete"
	fxDegraded = "fixtures/degraded"
	goldenPath = "testdata/complete.golden.md"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestParseDiff(t *testing.T) {
	// two files: file A 3 added / 1 removed, file B 1 added / 2 removed.
	diff := `diff --git a/x.go b/x.go
--- a/x.go
+++ b/x.go
@@ -1,2 +1,4 @@
 ctx
+a1
+a2
+a3
-r1
diff --git a/y.go b/y.go
--- a/y.go
+++ b/y.go
@@ -1,3 +1,2 @@
 ctx
+b1
-r1
-r2
`
	add, del, files := parseDiff([]byte(diff))
	if add != 4 {
		t.Errorf("added: got %d, want 4", add)
	}
	if del != 3 {
		t.Errorf("removed: got %d, want 3", del)
	}
	if files != 2 {
		t.Errorf("files: got %d, want 2", files)
	}
}

// A run with no usage.json must render its usage-derived metrics as
// could-not-check — never as a measured zero.
func TestLoadRunThreeState(t *testing.T) {
	r := loadRun(filepath.Join(fxDegraded, armWith, "run-1"), "run-1")

	if got := r.Metrics[mTokens]; got.State != cellCouldNotCheck {
		t.Errorf("tokens without a usage log: got state %v, want could-not-check", got.State)
	}
	if got := r.Metrics[mCost]; got.State != cellCouldNotCheck {
		t.Errorf("cost without a usage log: got state %v, want could-not-check", got.State)
	}
	// The metrics that DO have inputs stay measured.
	if got := r.Metrics[mDiffLines]; got.State != cellMeasured || !approx(got.Value, 6) {
		t.Errorf("diff_lines: got state %v value %v, want measured 6", got.State, got.Value)
	}
	if got := r.Metrics[mWall]; got.State != cellMeasured || !approx(got.Value, 110) {
		t.Errorf("wall_seconds: got state %v value %v, want measured 110", got.State, got.Value)
	}
	if got := r.Metrics[mCheck]; got.State != cellMeasured || !approx(got.Value, 1) {
		t.Errorf("check: got state %v value %v, want measured 1 (pass)", got.State, got.Value)
	}
}

func TestAggregateMath(t *testing.T) {
	arms, err := loadArms(fxComplete)
	if err != nil {
		t.Fatalf("loadArms: %v", err)
	}
	with := aggregateArm(arms[armWith])
	without := aggregateArm(arms[armWithout])

	cases := []struct {
		key                 string
		wantWith, wantWith0 float64 // wantWith mean, wantWith0 unused placeholder
		wantWithout         float64
		wantDeltaAbs        float64
		wantDeltaPct        float64
	}{
		{mDiffLines, 8, 0, 20, -12, -60},
		{mFilesTouched, 1, 0, 2, -1, -50},
		{mTokens, 12000, 0, 20000, -8000, -40},
		{mCost, 0.12, 0, 0.20, -0.08, -40},
		{mWall, 120, 0, 200, -80, -40},
		{mCheck, 1, 0, 1, 0, 0},
	}
	for _, c := range cases {
		w := with[c.key]
		o := without[c.key]
		if w.State != cellMeasured || !approx(w.Mean, c.wantWith) {
			t.Errorf("%s with-overlay: got state %v mean %v, want measured %v", c.key, w.State, w.Mean, c.wantWith)
		}
		if o.State != cellMeasured || !approx(o.Mean, c.wantWithout) {
			t.Errorf("%s without-overlay: got state %v mean %v, want measured %v", c.key, o.State, o.Mean, c.wantWithout)
		}
		if w.N != 2 || o.N != 2 {
			t.Errorf("%s n: got with=%d without=%d, want 2/2", c.key, w.N, o.N)
		}
		d := computeDelta(w, o)
		if d.State != cellMeasured || !approx(d.Abs, c.wantDeltaAbs) {
			t.Errorf("%s delta abs: got state %v abs %v, want measured %v", c.key, d.State, d.Abs, c.wantDeltaAbs)
		}
		if d.PctState == cellMeasured && !approx(d.Pct, c.wantDeltaPct) {
			t.Errorf("%s delta pct: got %v, want %v", c.key, d.Pct, c.wantDeltaPct)
		}
	}
}

// The degraded fixture (one arm with no usage logs) must produce could-not-check
// for tokens and cost — the positive control that a missing usage log never
// renders as a measured number.
func TestDegradedRendersCouldNotCheck(t *testing.T) {
	arms, err := loadArms(fxDegraded)
	if err != nil {
		t.Fatalf("loadArms: %v", err)
	}
	rep := reduce(arms, "degraded", "2026-08-21")
	md := renderReport(rep)

	if !strings.Contains(md, "could-not-check") {
		t.Fatalf("degraded report has no could-not-check cell:\n%s", md)
	}

	// Structurally: the with-overlay arm measured no tokens, so its aggregate
	// and the delta are both could-not-check — not a zero.
	with := aggregateArm(arms[armWith])
	if with[mTokens].State != cellCouldNotCheck {
		t.Errorf("with-overlay tokens: got state %v, want could-not-check", with[mTokens].State)
	}
	without := aggregateArm(arms[armWithout])
	d := computeDelta(with[mTokens], without[mTokens])
	if d.State != cellCouldNotCheck {
		t.Errorf("tokens delta: got state %v, want could-not-check", d.State)
	}

	// And the measured metrics still produce a real delta (the report is not
	// wholesale could-not-check).
	dd := computeDelta(with[mDiffLines], without[mDiffLines])
	if dd.State != cellMeasured {
		t.Errorf("diff_lines delta: got state %v, want measured", dd.State)
	}
}

func TestReportGolden(t *testing.T) {
	arms, err := loadArms(fxComplete)
	if err != nil {
		t.Fatalf("loadArms: %v", err)
	}
	rep := reduce(arms, "complete", "2026-08-21")
	got := renderReport(rep)

	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote golden %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run `go test -update` to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("report does not match golden %s.\n--- got ---\n%s\n--- want ---\n%s", goldenPath, got, want)
	}
}

// TestRunCLI exercises the executable entry point end to end: flag parsing,
// the reduce/render pipeline, and writing the report file.
func TestRunCLI(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "report.md")
	rc := runCLI([]string{
		"--arms", fxComplete,
		"--out", outPath,
		"--overlay-slug", "complete",
		"--date", "2026-08-21",
	}, os.Stderr)
	if rc != 0 {
		t.Fatalf("runCLI exit: got %d, want 0", rc)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if strings.Count(string(got), "delta:") < 3 {
		t.Errorf("report has fewer than 3 delta lines:\n%s", got)
	}
	// A missing arms directory is could-not-check the input: exit 2, no panic.
	if rc := runCLI([]string{"--arms", filepath.Join(t.TempDir(), "nope"), "--out", filepath.Join(t.TempDir(), "x.md")}, os.Stderr); rc != 2 {
		t.Errorf("missing arms dir: got exit %d, want 2", rc)
	}
	// A missing --arms flag is a usage error: exit 1.
	if rc := runCLI([]string{"--out", filepath.Join(t.TempDir(), "y.md")}, os.Stderr); rc != 1 {
		t.Errorf("no --arms: got exit %d, want 1", rc)
	}
}

// A delta must never be emitted when one arm did not measure the metric.
func TestDeltaNeedsBothArms(t *testing.T) {
	with := armMetric{State: cellMeasured, Mean: 5, N: 1, Total: 1}
	without := armMetric{State: cellCouldNotCheck, Total: 1}
	if d := computeDelta(with, without); d.State != cellCouldNotCheck {
		t.Errorf("one-arm delta: got state %v, want could-not-check", d.State)
	}
	// A zero baseline leaves the percentage undefined but the absolute delta
	// measured.
	with2 := armMetric{State: cellMeasured, Mean: 3, N: 1, Total: 1}
	without2 := armMetric{State: cellMeasured, Mean: 0, N: 1, Total: 1}
	d := computeDelta(with2, without2)
	if d.State != cellMeasured || !approx(d.Abs, 3) {
		t.Errorf("zero-baseline delta: got state %v abs %v, want measured 3", d.State, d.Abs)
	}
	if d.PctState != cellCouldNotCheck {
		t.Errorf("zero-baseline pct: got state %v, want could-not-check", d.PctState)
	}
}
