package main

import (
	"fmt"
	"strings"
)

// checkSelectedClosure owns acceptance for an explicitly selected verified/done
// outcome. Both brief --check-verified and verifyclosure call it. Corpus lint
// retains its transition scope and calls the same failFirstClosureChecks below.
// This check never grandfathers a selected closure and requires at least one
// applicable Verify row. Attribution and landing authorization remain separate.
func checkSelectedClosure(root, key, status, verified string, art briefArtifacts) ([]checkFinding, error) {
	if status != "verified" && status != "done" {
		return nil, fmt.Errorf("verified outcome requires status verified or done, got %q", status)
	}
	if !verifiedCellRe.MatchString(verified) {
		return nil, fmt.Errorf("verified outcome requires a dated runner in the Verified cell (YYYY-MM-DD <runner>)")
	}
	// This verb does not run the lint, so it refuses the #1939 case itself: a
	// closure is never authorized over a section the parser and the rendered
	// page may read differently.
	findings, refusal := closureWitnesses(art.Verify, art.Evidence, art.Path)
	if refusal != "" {
		return nil, fmt.Errorf("verified outcome requires well-formed Verify and Evidence sections: %s", refusal)
	}
	if len(findings) == 0 {
		return nil, fmt.Errorf("verified outcome requires Verify rows with passing execution witnesses")
	}
	// A Verify row classed gate:human is decided by a person: its closure is
	// valid only when the Verified cell names one. Read here from the written
	// board row, after whatever wrote it — a flip path that skipped its own
	// class check still stops at this one.
	if !hasHumanReviewer(verified) {
		for _, r := range briefVerifyRows(art.Verify) {
			if r.Class == classGateHuman {
				return nil, fmt.Errorf("verified outcome: Verify row %s is classed gate:human but the Verified cell %q names no human:<name>", r.ID, verified)
			}
		}
	}
	var failures []string
	for _, finding := range findings {
		if !closureSatisfied(finding) {
			failures = append(failures, fmt.Sprintf("row %s: %s — %s", finding.ID, finding.State, finding.Detail))
		}
	}
	if len(failures) > 0 {
		return nil, fmt.Errorf("verified outcome requires passing execution witnesses: %s", strings.Join(failures, "; "))
	}
	plan, refusal := protectedAcceptance(art.Verify, art.Evidence, art.Path)
	if refusal != "" {
		return nil, fmt.Errorf("verified outcome requires well-formed Verify and Evidence sections: %s", refusal)
	}
	problems, _ := failFirstClosureChecks(root, key, status, art, plan)
	if len(problems) > 0 {
		return nil, fmt.Errorf("verified outcome requires fail-first witnesses: %s", strings.Join(problems, "; "))
	}
	return findings, nil
}
