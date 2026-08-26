package deskkit

import (
	"encoding/json"
	"os"
	"testing"
)

// risktrigger_coupling_test.go — the deskkit half of the risk×files cross-read
// cross-tree binding. The SAME vector file the statusgen lint reads in-module
// (TestTriggerCoupling in statusgen/risktrigger_coupling_test.go) is read HERE
// cross-module and run through the desk classifier's POLICY half —
// RiskPathTriggersFor + matchTrigger, the exact reading patternClassifier.Classify
// dispatches to. If statusgen's duplicate (riskpathtriggers.go) drifts from this
// original — the base list, the glob rule, or the per-repo topology reading — one
// of the two TriggerCoupling halves goes red.
//
// This reads statusgen/testdata/risk_trigger_coupling.json, OUTSIDE this Go
// module, so it is registered in citrigger_test.go's ciCrossModuleRegistry — the
// guard that proves an edit to what a test READS actually triggers the job that
// RUNS it. The path is ../../../../statusgen/ because statusgen lives at the repo
// root, mirroring scancoupling_test.go's roster vector.
//
// POLICY HALF, deliberately. The cases run through matchTrigger over
// RiskPathTriggersFor, NOT through RiskPathTriggered (the exported accessor whose
// MECHANISM half fail-closes an unknown/public repo to true). The unknown-repo
// cases in the vector are the discriminator: they assert the base-list answer, not
// the fail-closed one, which is exactly the half the statusgen lint duplicates.
//
// DO NOT relax an assertion to green it: the refusal cases and the unknown-repo
// discriminators are the binding.

const riskTriggerVectorPath = "../../../../statusgen/testdata/risk_trigger_coupling.json"

type riskTriggerVectors struct {
	Cases []struct {
		Why       string `json:"why"`
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Triggered bool   `json:"triggered"`
	} `json:"cases"`
	EnvExtra string `json:"envExtra"`
	EnvCases []struct {
		Why       string `json:"why"`
		Repo      string `json:"repo"`
		Path      string `json:"path"`
		Triggered bool   `json:"triggered"`
	} `json:"envCases"`
}

func loadRiskTriggerVectors(t *testing.T) riskTriggerVectors {
	t.Helper()
	raw, err := os.ReadFile(riskTriggerVectorPath)
	if err != nil {
		t.Fatalf("cannot read the shared cross-tree trigger vectors at %s: %v — if statusgen moved, "+
			"re-point this coupling test, do NOT delete it", riskTriggerVectorPath, err)
	}
	var v riskTriggerVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s does not parse: %v", riskTriggerVectorPath, err)
	}
	if len(v.Cases) == 0 {
		t.Fatalf("%s carries no cases — an empty vector file is a coupling guard that cannot fail", riskTriggerVectorPath)
	}
	return v
}

// triggerHit is the POLICY-half decision this module contributes to the coupling:
// does any trigger in repo's set match path. It is the exact reading
// patternClassifier.Classify performs (RiskPathTriggersFor + matchTrigger), so a
// drift in either reddens here.
func triggerHit(repo, path string) bool {
	for _, pat := range RiskPathTriggersFor(repo) {
		if matchTrigger(pat, path) {
			return true
		}
	}
	return false
}

// TestTriggerCoupling runs the shared vector through THIS module's policy-half
// reader. The name matches statusgen's twin so `go test -run TriggerCoupling`
// exercises both.
func TestTriggerCoupling(t *testing.T) {
	vec := loadRiskTriggerVectors(t)

	// Base + per-repo cases: the default fixture roster sets no
	// ASSAY_RISK_PATH_TRIGGERS_EXTRA, so RiskExtra is empty here.
	for _, c := range vec.Cases {
		if got := triggerHit(c.Repo, c.Path); got != c.Triggered {
			t.Errorf("triggerHit(%q, %q) = %t, want %t — %s", c.Repo, c.Path, got, c.Triggered, c.Why)
		}
	}

	// Env leg: prove the additive ASSAY_RISK_PATH_TRIGGERS_EXTRA value is the cause.
	if len(vec.EnvCases) == 0 || vec.EnvExtra == "" {
		t.Fatalf("%s carries no envExtra/envCases — the additive-env leg of the binding is unbound", riskTriggerVectorPath)
	}
	// First, with it unset (default fixture roster), every triggered=true env case
	// must NOT match.
	for _, c := range vec.EnvCases {
		if !c.Triggered {
			continue
		}
		if triggerHit(c.Repo, c.Path) {
			t.Errorf("with ASSAY_RISK_PATH_TRIGGERS_EXTRA UNSET, %q on %q already matched — the env case cannot prove additivity: %s", c.Path, c.Repo, c.Why)
		}
	}
	// Now SET it (through the real config file loader) and assert each case.
	withRoster(t, map[string]string{"ASSAY_RISK_PATH_TRIGGERS_EXTRA": vec.EnvExtra})
	for _, c := range vec.EnvCases {
		if got := triggerHit(c.Repo, c.Path); got != c.Triggered {
			t.Errorf("with ASSAY_RISK_PATH_TRIGGERS_EXTRA=%q, triggerHit(%q, %q) = %t, want %t — %s",
				vec.EnvExtra, c.Repo, c.Path, got, c.Triggered, c.Why)
		}
	}
}
