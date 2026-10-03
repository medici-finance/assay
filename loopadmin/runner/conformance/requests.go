package conformance

import "github.com/medici-finance/assay/loopadmin/runner"

// StandingRequest builds a valid standing-desk request: a desk binding and no
// workflow reference of any kind.
func StandingRequest(id string) runner.LaunchRequest {
	return runner.LaunchRequest{
		Version: runner.Version, Caller: "desk-worker", ID: id,
		Mode:      runner.ModeStandingDesk,
		Desk:      &runner.DeskRef{BindingID: "binding-1"},
		Packet:    runner.RolePacket{Role: "worker", Ref: "packets/worker-v1", Trust: runner.TrustOperator},
		Profile:   runner.Profile{ID: "profile-a", Model: "model-a", Tools: []string{"read", "write"}},
		Workspace: "workspaces/w1",
		Authority: runner.Authority{Key: "claim/desk-1", Generation: 1, ValidatedBy: "claim-store"},
		Budget:    runner.Reservation{ID: "res-1", Scope: runner.BudgetLaunch, MaxTokens: 1000},
	}
}

// WorkflowRequest builds a valid workflow-stage request: canonical work, node
// and attempt references and no desk binding.
func WorkflowRequest(id string) runner.LaunchRequest {
	r := StandingRequest(id)
	r.Caller = "controller"
	r.Mode = runner.ModeWorkflowStage
	r.Desk = nil
	r.Work = &runner.WorkRef{WorkID: "work-7", NodeID: "node-3", AttemptID: "attempt-1"}
	r.Authority.Key = "attempt/work-7/node-3"
	return r
}

// GoodResult builds a well-formed success result for a request.
func GoodResult(req runner.LaunchRequest) runner.Result {
	return runner.Result{
		Caller: req.Caller, ID: req.ID,
		Generation: req.Authority.Generation, Outcome: runner.OutcomeSuccess,
		Summary: "done",
		Artifacts: []runner.Artifact{{
			Name: "out", Ref: "artifacts/out", Trust: runner.TrustUntrusted,
			Hash: "0000000000000000000000000000000000000000000000000000000000000000",
		}},
	}
}

// FullUsage is a usage with every figure measured.
func FullUsage() runner.Usage {
	return runner.Usage{InputTokens: runner.Int64(10), OutputTokens: runner.Int64(5), CostMicros: runner.Int64(100)}
}
