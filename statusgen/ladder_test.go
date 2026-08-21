package main

import (
	"strings"
	"testing"
	"time"
)

// mkRung is a tiny fixture builder for a measured or unmeasured rung.
func mkRung(step int, key, name string, measured, pass bool) ladderRung {
	r := ladderRung{Step: step, Key: key, Name: name, Measured: measured, Pass: pass}
	if !measured {
		r.Reason = "no-opmetrics-dayfile"
	}
	return r
}

// TestLadderStepMappingExact: the monotone prefix ladder — fixture axis inputs →
// expected step. With every axis measured, the position is the highest prefix of
// passing rungs; the first failing rung is the binding constraint.
func TestLadderStepMappingExact(t *testing.T) {
	cases := []struct {
		name       string
		passes     []bool // per rung 1..4, all measured
		wantStep   int
		wantConstr string // substring the constraint line must contain
	}{
		{"all fail", []bool{false, false, false, false}, 0, "held at 0 by: autonomy ratio (CI proxy)"},
		{"first only", []bool{true, false, false, false}, 1, "held at 1 by: deterministic-gate share"},
		{"first two", []bool{true, true, false, false}, 2, "held at 2 by: dispatch autonomy"},
		{"three", []bool{true, true, true, false}, 3, "held at 3 by: token efficiency"},
		{"topped out", []bool{true, true, true, true}, 4, "top rung"},
		// Monotone: a later pass does NOT count if an earlier rung fails.
		{"gap ignored", []bool{true, false, true, true}, 1, "held at 1 by: deterministic-gate share"},
	}
	names := []string{
		"autonomy ratio (CI proxy)", "deterministic-gate share",
		"dispatch autonomy", "token efficiency",
	}
	keys := []string{"autonomy_ci_proxy", "deterministic_gate_share", "autonomy_dispatch", "token_efficiency"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rungs []ladderRung
			for i, p := range c.passes {
				rungs = append(rungs, mkRung(i+1, keys[i], names[i], true, p))
			}
			rep := computeLadder(rungs)
			if !rep.Exact {
				t.Fatalf("all-measured ladder must be exact; got range %d-%d", rep.StepLow, rep.StepHigh)
			}
			if rep.StepLow != c.wantStep {
				t.Errorf("step = %d, want %d (rungs %v)", rep.StepLow, c.wantStep, c.passes)
			}
			if !strings.Contains(rep.Constraint, c.wantConstr) {
				t.Errorf("constraint = %q, want substring %q", rep.Constraint, c.wantConstr)
			}
		})
	}
}

// TestLadderRangeWhenAxisUnmeasured: the missing-axis RANGE render. Rungs 1–2
// pass and are measured; rungs 3–4 (the mm/40 day-file axes) are unmeasured, so
// the position is the range [2,4] — never a fabricated point value — and the
// constraint line names the unmeasured axes with the literal word "unmeasured".
func TestLadderRangeWhenAxisUnmeasured(t *testing.T) {
	rungs := []ladderRung{
		mkRung(1, "autonomy_ci_proxy", "autonomy ratio (CI proxy)", true, true),
		mkRung(2, "deterministic_gate_share", "deterministic-gate share", true, true),
		mkRung(3, "autonomy_dispatch", "dispatch autonomy", false, false),
		mkRung(4, "token_efficiency", "token efficiency", false, false),
	}
	rep := computeLadder(rungs)
	if rep.Exact {
		t.Fatalf("an unmeasured higher rung must produce a RANGE, not an exact step: %+v", rep)
	}
	if rep.StepLow != 2 || rep.StepHigh != 4 {
		t.Errorf("range = %d-%d, want 2-4", rep.StepLow, rep.StepHigh)
	}
	if !strings.Contains(rep.Constraint, "unmeasured") {
		t.Errorf("range constraint must say 'unmeasured'; got %q", rep.Constraint)
	}
	if !strings.Contains(rep.Constraint, "dispatch autonomy") || !strings.Contains(rep.Constraint, "token efficiency") {
		t.Errorf("range constraint must NAME the unmeasured axes; got %q", rep.Constraint)
	}
	// The rendered text must carry "step" (a consumer greps for it) and the
	// range label, never a silent zero.
	txt := renderLadderText(rep)
	if !strings.Contains(txt, "step: 2–4") {
		t.Errorf("render must show the range label 'step: 2–4'; got:\n%s", txt)
	}
	if !strings.Contains(txt, "unmeasured") {
		t.Errorf("render must say 'unmeasured'; got:\n%s", txt)
	}
}

// TestLadderMeasuredFailCapsRangeBelowUnmeasured: a MEASURED failure is a hard
// ceiling — an unmeasured rung ABOVE it cannot lift the ceiling, so the position
// stays exact at the failure, never optimistically ranging past a known fail.
func TestLadderMeasuredFailCapsRangeBelowUnmeasured(t *testing.T) {
	rungs := []ladderRung{
		mkRung(1, "autonomy_ci_proxy", "autonomy ratio (CI proxy)", true, true),
		mkRung(2, "deterministic_gate_share", "deterministic-gate share", true, false), // measured FAIL
		mkRung(3, "autonomy_dispatch", "dispatch autonomy", false, false),              // unmeasured, above the fail
		mkRung(4, "token_efficiency", "token efficiency", false, false),
	}
	rep := computeLadder(rungs)
	if !rep.Exact || rep.StepLow != 1 {
		t.Errorf("a measured fail at rung 2 must cap the position exact at step 1; got %+v", rep)
	}
	if !strings.Contains(rep.Constraint, "held at 1 by: deterministic-gate share") {
		t.Errorf("constraint must name the measured-failed rung; got %q", rep.Constraint)
	}
}

// TestLadderRungsFromAutonomyDegradesWithoutDayFile: the adapter that feeds
// runLadder inherits autonomy.go's honest degrade — with gh readable but NO
// mm/40 opmetrics day-file, rungs 1–2 are measured and rungs 3–4 (dispatch +
// token) are unmeasured with the no-opmetrics reason. This is the exact
// public-checkout degrade mm/42 requires.
func TestLadderRungsFromAutonomyDegradesWithoutDayFile(t *testing.T) {
	since := mustTime(t, "2026-07-01T00:00:00Z")
	until := mustTime(t, "2026-07-15T00:00:00Z")
	in := autonomyInputs{
		Since: since, Until: until, Now: until,
		AuthorsOK:   true,
		HumanLogins: map[string]bool{"alice": true},
		MergedAuthors: []autonomyAuthor{
			{Login: "assay-worker-app[bot]", IsBot: true}, // loop
			{Login: "assay-worker-app[bot]", IsBot: true}, // loop
			{Login: "alice", IsBot: false},                // human
		},
		GateOK: true,
		GateData: []autonomyGatePR{
			{Number: 1, CheckNames: []string{"go-test"}}, // deterministic
			{Number: 2, CheckNames: []string{"go-lint"}}, // deterministic
		},
		DayFile: nil, // the private collector did not run — public checkout
	}
	rungs := ladderRungsFromAutonomy(in)
	if len(rungs) != 4 {
		t.Fatalf("want 4 rungs, got %d", len(rungs))
	}
	if !rungs[0].Measured || !rungs[1].Measured {
		t.Errorf("rungs 1-2 must be measured from gh; got %+v %+v", rungs[0], rungs[1])
	}
	for _, i := range []int{2, 3} {
		if rungs[i].Measured {
			t.Errorf("rung %d must be UNMEASURED without a day-file; got %+v", i+1, rungs[i])
		}
		if rungs[i].Reason != "no-opmetrics-dayfile" {
			t.Errorf("rung %d reason = %q, want no-opmetrics-dayfile", i+1, rungs[i].Reason)
		}
	}
	// End-to-end: this must produce a range, and the render must say unmeasured.
	rep := computeLadder(rungs)
	if rep.Exact {
		t.Errorf("with two unmeasured higher rungs the position must be a range; got %+v", rep)
	}
	if !strings.Contains(renderLadderText(rep), "unmeasured") {
		t.Error("render must degrade to 'unmeasured', never a silent zero")
	}
}

// TestRunLadderPublicDegradeNoDayFile: the --ladder entrypoint on a root with no
// opmetrics day-file exits 0 (never errors on the missing private input) and its
// output carries the literal 'unmeasured' — the mm/42 Verify contract (public
// ship + degrade). gh reads are stubbed so the test is deterministic offline.
func TestRunLadderPublicDegradeNoDayFile(t *testing.T) {
	// Stub the gh-backed data gatherers so the test never touches the network.
	origAuthors, origGates := autonomyMergedAuthors, autonomyGates
	origNow := nowFunc
	t.Cleanup(func() { autonomyMergedAuthors, autonomyGates, nowFunc = origAuthors, origGates, origNow })
	autonomyMergedAuthors = func(root string, since, until time.Time) ([]autonomyAuthor, bool) {
		return []autonomyAuthor{{Login: "assay-worker-app[bot]", IsBot: true}}, true
	}
	autonomyGates = func(root string, since, until time.Time) ([]autonomyGatePR, bool) {
		return []autonomyGatePR{{Number: 1, CheckNames: []string{"go-test"}}}, true
	}
	nowFunc = func() time.Time { return mustTime(t, "2026-07-15T00:00:00Z") }

	root := t.TempDir() // no docs/reports/daily/ → no opmetrics day-file

	var code int
	out := captureStdout(t, func() { code = runLadder(root, "", false) })
	if code != 0 {
		t.Fatalf("runLadder exited %d, want 0 — a missing private day-file must not be an error", code)
	}
	if !strings.Contains(out, "unmeasured") {
		t.Errorf("--ladder output must contain 'unmeasured' when the opmetrics day-file is absent; got:\n%s", out)
	}
	if !strings.Contains(out, "step") {
		t.Errorf("--ladder output must contain 'step'; got:\n%s", out)
	}
}
