package main

import (
	"reflect"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestBootWritesOneStepRecordPerStep: a booted run with ASSAY_TELEMETRY=1 writes exactly one
// record per boot step, in boot order, each spelled "step:" + the step's own name constant
// and carrying the booted role; the wrapped run adds one verb-level record; the step roster
// deskkit validates against is pinned equal to this package's own list; and the boot
// aggregate (what `deskperf boot` renders) groups the steps under the role. A red step still
// records itself, and no step after it does.
func TestBootWritesOneStepRecordPerStep(t *testing.T) {
	if got := deskkit.PerfBootSteps(); !reflect.DeepEqual(got, bootSteps) {
		t.Fatalf("deskkit's boot-step roster %v drifted from deskboot's own step list %v", got, bootSteps)
	}

	s := &stub{}
	home, root := s.install(t)
	s.replies = happyStub(t, writeToken(t, home))
	t.Setenv("ASSAY_TELEMETRY", "1")

	if rc := deskkit.PerfRun("deskboot", perfVerbs(), []string{"the-desk", "--root", root}, run); rc != deskkit.ExitOK {
		t.Fatalf("boot rc = %d, want 0", rc)
	}
	recs, skipped, err := deskkit.LoadPerfDays(1)
	if err != nil || skipped != 0 {
		t.Fatalf("LoadPerfDays: %v (skipped %d)", err, skipped)
	}
	var steps []string
	verbLevel := 0
	for _, r := range recs {
		if r.Tool != "deskboot" || r.Role != "the-desk" {
			t.Errorf("record %+v: want tool deskboot, role the-desk", r)
		}
		if deskkit.IsPerfStep(r) {
			steps = append(steps, r.Verb)
			if r.Exit != deskkit.ExitOK || r.Result != deskkit.ResultOK || r.CPUMs != nil {
				t.Errorf("step record %+v: want exit 0 / ok and no cpuMs", r)
			}
			continue
		}
		verbLevel++
		if r.Verb != "the-desk" {
			t.Errorf("verb-level record verb = %q, want the booted role", r.Verb)
		}
	}
	var want []string
	for _, st := range bootSteps {
		want = append(want, "step:"+st)
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("step records:\n got  %v\n want %v (one per step, in boot order)", steps, want)
	}
	if verbLevel != 1 {
		t.Fatalf("verb-level records = %d, want 1", verbLevel)
	}

	sum := deskkit.SummariseBootSteps(recs)
	if sum.Records != len(bootSteps) || len(sum.Roles) != 1 || sum.Roles[0].Role != "the-desk" ||
		len(sum.Roles[0].Steps) != len(bootSteps) {
		t.Fatalf("boot aggregate does not group the %d steps under the-desk: %+v", len(bootSteps), sum)
	}
	for i, st := range sum.Roles[0].Steps {
		if st.Step != bootSteps[i] || st.Count != 1 {
			t.Errorf("aggregate step %d = %+v, want %s count 1", i, st, bootSteps[i])
		}
	}

	// A red preflight: the failing step records itself (exit 6), and the steps after it don't.
	s2 := &stub{}
	home2, root2 := s2.install(t)
	s2.replies = append(happyStub(t, writeToken(t, home2)), reply{match: "deskroster preflight", fail: true})
	t.Setenv("ASSAY_TELEMETRY", "1")
	if rc := run([]string{"the-desk", "--root", root2}); rc != deskkit.ExitUnverifiable {
		t.Fatalf("red preflight rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	recs, _, err = deskkit.LoadPerfDays(1)
	if err != nil {
		t.Fatal(err)
	}
	steps = steps[:0]
	for _, r := range recs {
		steps = append(steps, r.Verb)
	}
	wantRed := []string{"step:loop-identity", "step:worktree-prune", "step:worktree-lock", "step:roster-set", "step:roster-preflight"}
	if !reflect.DeepEqual(steps, wantRed) {
		t.Fatalf("red boot step records:\n got  %v\n want %v", steps, wantRed)
	}
	if last := recs[len(recs)-1]; last.Exit != deskkit.ExitUnverifiable || last.Result != deskkit.ResultUnverifiable {
		t.Errorf("the red step recorded %+v, want exit 6 / unverifiable", last)
	}
}
