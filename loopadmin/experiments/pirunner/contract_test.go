//go:build pirunner

package pirunner

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
)

// TestPiPilotContract runs the entire exported conformance kit against the
// Pi adapter through the real runner.Client, then the pilot's own refusals.
// The adapter declares no runner.ProfileDeclarer, so the two profile cases
// pass without exercising it; the report says so.
func TestPiPilotContract(t *testing.T) {
	t.Run("Kit", func(t *testing.T) { conformance.RunAll(t, subject) })

	t.Run("NoProfileDecl", func(t *testing.T) {
		var a runner.Adapter = &Adapter{}
		if _, ok := a.(runner.ProfileDeclarer); ok {
			t.Fatal("the pilot adapter now declares profiles; update the report's profile note")
		}
	})

	t.Run("UnsupportedRefused", func(t *testing.T) {
		s := subject(t)
		a := s.Adapter.(*Adapter)
		c := runner.NewClient(a, conformance.NewClaims())
		for name, mut := range map[string]func(*runner.LaunchRequest){
			"request-scope": func(r *runner.LaunchRequest) { r.Budget.Scope = runner.BudgetRequest },
			"needs-resume":  func(r *runner.LaunchRequest) { r.Require = []string{string(runner.CapResume)} },
			"needs-cancel":  func(r *runner.LaunchRequest) { r.Require = []string{string(runner.CapCancelAck)} },
			"unknown-cap":   func(r *runner.LaunchRequest) { r.Require = []string{"teleport"} },
		} {
			req := pilotReq("desk-worker", "unsupported-"+name, 1)
			mut(&req)
			if _, _, err := c.Start(ctxT(t), req); err == nil {
				t.Errorf("%s: started", name)
			}
		}
		if n := a.H.Counters.Snapshot().Submits; n != 0 {
			t.Fatalf("refused launches reached Pi: %d submissions", n)
		}
	})

	t.Run("PinnedModelAndTools", func(t *testing.T) {
		s := subject(t)
		a := s.Adapter.(*Adapter)
		req := pilotReq("desk-worker", "pinned", 1)
		rec, err := a.Start(ctxT(t), req)
		if err != nil {
			t.Fatal(err)
		}
		if rec.ActualModel != req.Profile.Model {
			t.Fatalf("actual model %q, pinned %q", rec.ActualModel, req.Profile.Model)
		}
		a.RunModelInstead("model-b")
		rec2, err := a.Start(ctxT(t), pilotReq("desk-worker", "fallback", 1))
		if err != nil {
			t.Fatal(err)
		}
		if rec2.ActualModel != "model-b" {
			t.Fatalf("model override not reported: %q", rec2.ActualModel)
		}
		_, _, err = runner.NewClient(a, conformance.NewClaims()).Start(ctxT(t), pilotReq("desk-worker", "fallback-2", 1))
		if !errors.Is(err, runner.ErrModelFallback) {
			t.Fatalf("silent fallback accepted: %v", err)
		}
	})
}
