package evaluation

import "testing"

func TestSelectedClosureProfiles(t *testing.T) {
	good := SelectedClosure{Status: "verified", Stamp: "2026-10-25 human:fixture", Claims: []ClosureClaim{{ID: "1", State: "pass"}}}
	if e := ValidateSelectedClosure(good); e != nil {
		t.Fatal(e)
	}
	bad := good
	bad.Claims = []ClosureClaim{{ID: "1", State: "author-added"}}
	if ValidateSelectedClosure(bad) == nil {
		t.Fatal("supplemental-only rows granted acceptance")
	}
	bad = good
	bad.Stamp = "2026-10-25 model:fixture"
	bad.HumanRows = []string{"1"}
	if ValidateSelectedClosure(bad) == nil {
		t.Fatal("human row admitted under model stamp")
	}
	bad = good
	bad.FailFirstProblems = []string{"missing Base"}
	if ValidateSelectedClosure(bad) == nil {
		t.Fatal("fail-first refusal erased")
	}
	if FailFirstProblem(FailFirst{Required: true, Found: true, State: "unsupported"}) == "" {
		t.Fatal("unsupported Base state admitted")
	}
}
