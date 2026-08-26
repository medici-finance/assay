package main

import (
	"encoding/json"
	"os"
	"testing"
)

// risktrigger_coupling_test.go — the statusgen half of the mistake-proofing/01
// cross-tree binding. The SAME vector file is read by deskkit's
// TestTriggerCoupling (tools/desk/internal/deskkit/risktrigger_coupling_test.go)
// through the desk classifier's POLICY half, and here through statusgen's
// duplicate (riskpathtriggers.go). One table in, identical verdicts out; a drift
// in either copy of the base list, the glob rule, or the per-repo topology reading
// reddens one of the two halves.
//
// It lives in-module (statusgen reads testdata/ directly), so — unlike the deskkit
// half — no CI cross-module-reader registry row is owed on this side.
//
// DO NOT relax an assertion to green it. The vector's refusal cases and the
// unknown-repo policy-half discriminators are the binding; deleting one, or
// widening it so an empty set satisfies it, removes the very drift protection this
// file exists to provide.

const riskTriggerVectorPath = "testdata/risk_trigger_coupling.json"

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
		t.Fatalf("cannot read the shared cross-tree trigger vectors at %s: %v", riskTriggerVectorPath, err)
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

// TestTriggerCoupling runs the shared vector through statusgen's duplicated
// policy-half reader. The name matches deskkit's twin so `go test -run
// TriggerCoupling` exercises both.
func TestTriggerCoupling(t *testing.T) {
	vec := loadRiskTriggerVectors(t)

	// Base + per-repo cases: no adopter env, so RiskExtra is empty. scanWithNoRoster
	// points the config home at an empty dir, so a roster.env on the running
	// machine cannot pollute the trigger set.
	scanWithNoRoster(t)
	for _, c := range vec.Cases {
		_, hit := lintRiskTriggerHit(c.Repo, c.Path)
		if hit != c.Triggered {
			t.Errorf("lintRiskTriggerHit(%q, %q) = %t, want %t — %s", c.Repo, c.Path, hit, c.Triggered, c.Why)
		}
	}

	// Env leg: prove the additive ASSAY_RISK_PATH_TRIGGERS_EXTRA value is the cause.
	// First, with it UNSET, every triggered=true env case must NOT match.
	if len(vec.EnvCases) == 0 || vec.EnvExtra == "" {
		t.Fatalf("%s carries no envExtra/envCases — the additive-env leg of the binding is unbound", riskTriggerVectorPath)
	}
	for _, c := range vec.EnvCases {
		if !c.Triggered {
			continue
		}
		if _, hit := lintRiskTriggerHit(c.Repo, c.Path); hit {
			t.Errorf("with ASSAY_RISK_PATH_TRIGGERS_EXTRA UNSET, %q on %q already matched — the env case cannot prove additivity: %s", c.Path, c.Repo, c.Why)
		}
	}
	// Now SET it (through the real write-class file loader) and assert each case.
	scanWithRoster(t, map[string]string{scanEnvRiskPathTriggersExtra: vec.EnvExtra})
	for _, c := range vec.EnvCases {
		_, hit := lintRiskTriggerHit(c.Repo, c.Path)
		if hit != c.Triggered {
			t.Errorf("with ASSAY_RISK_PATH_TRIGGERS_EXTRA=%q, lintRiskTriggerHit(%q, %q) = %t, want %t — %s",
				vec.EnvExtra, c.Repo, c.Path, hit, c.Triggered, c.Why)
		}
	}
}

// TestTriggerCouplingVectorHasRefusals guards the fixture itself: a coupling
// vector of only acceptance cases proves the two readers agree on a good path but
// never binds the refusal side, where a rewritten matcher silently diverges.
func TestTriggerCouplingVectorHasRefusals(t *testing.T) {
	vec := loadRiskTriggerVectors(t)
	accept, refuse := 0, 0
	for _, c := range vec.Cases {
		if c.Triggered {
			accept++
		} else {
			refuse++
		}
	}
	if accept == 0 {
		t.Error("the trigger vector carries no acceptance case — it cannot prove a trigger fires")
	}
	if refuse == 0 {
		t.Error("the trigger vector carries no REFUSAL case — an acceptance-only vector cannot bind the glob rule; a rewritten matcher that over-matches would stay green")
	}
}
