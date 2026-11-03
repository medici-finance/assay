package main

import (
	"github.com/medici-finance/assay/statusgen/evaluation"
)

// checkSelectedClosure owns acceptance for an explicitly selected verified/done
// outcome. Both brief --check-verified and verifyclosure call it. Corpus lint
// retains its transition scope and calls the same failFirstClosureChecks below.
// This check never grandfathers a selected closure and requires at least one
// applicable Verify row. Attribution and landing authorization remain separate.
func checkSelectedClosure(root, key, status, verified string, art briefArtifacts) ([]checkFinding, error) {
	if err := evaluation.ValidateSelectedStamp(status, verified); err != nil {
		return nil, err
	}
	findings, refusal := closureWitnesses(art.Verify, art.Evidence, art.Path)
	observation := evaluation.SelectedClosure{Status: status, Stamp: verified, Malformed: refusal}
	for _, r := range briefVerifyRows(art.Verify) {
		if r.Class == classGateHuman {
			observation.HumanRows = append(observation.HumanRows, r.ID)
		}
	}
	for _, f := range findings {
		observation.Claims = append(observation.Claims, evaluation.ClosureClaim{ID: f.ID, State: f.State, Detail: f.Detail})
	}
	if refusal == "" {
		plan, why := protectedAcceptance(art.Verify, art.Evidence, art.Path)
		observation.Malformed = why
		if why == "" {
			observation.FailFirstProblems, _ = failFirstClosureChecks(root, key, status, art, plan)
		}
	}
	if err := evaluation.ValidateSelectedClosure(observation); err != nil {
		return nil, err
	}
	return findings, nil
}
