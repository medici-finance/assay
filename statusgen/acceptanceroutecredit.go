package main

import (
	"fmt"
	"os"
	"strings"
)

// acceptanceRun identifies a selected verifier run in a source-bound acceptance
// record. The issuer and close adapter must authenticate that record and bind
// its selected runs to the current acceptance definition.
type acceptanceRun struct {
	Run    string `json:"run"`
	Source string `json:"source"`
}

// creditAcceptanceRuns rechecks the exact selected runs against the landed
// witness. It is read-only. A successful read does not authorize a card or close.
func creditAcceptanceRuns(witness, protectedVerify string, selected []acceptanceRun, ciWorkflowToken bool) error {
	if len(selected) < 2 {
		return fmt.Errorf("acceptance credit: two independent verifier runs are required")
	}
	if len(briefVerifyRows(protectedVerify)) == 0 {
		return fmt.Errorf("acceptance credit: protected Verify definition is empty")
	}

	contents, err := os.ReadFile(witness)
	if err != nil {
		return fmt.Errorf("acceptance credit: read witness: %w", err)
	}
	text := string(contents)
	cc := newCreditContext(witness, "", text)
	cc.ciWorkflowToken = ciWorkflowToken
	if cc.place.verdict != creditCredited {
		return fmt.Errorf("acceptance credit: %s: %s", cc.place.verdict, cc.place.why)
	}

	type witnessRun struct {
		bracket bracketCheck
		att     containerAttestation
	}
	runs := map[string]witnessRun{}
	for _, bracket := range bracketChecks(text) {
		if bracket.header == 0 {
			continue
		}
		_, line, problem := spanOwnLines(cc.lines[bracket.header-1 : bracket.bracket-1])
		a, ok := parseContainerAttestation(line)
		if problem != "" || !ok {
			continue
		}
		run := strings.ToLower(a.run)
		if _, duplicate := runs[run]; duplicate {
			return fmt.Errorf("acceptance credit: duplicate witness run %s", run)
		}
		runs[run] = witnessRun{bracket: bracket, att: a}
	}

	seen := map[string]bool{}
	models := map[string]bool{}
	strong := false
	for _, wanted := range selected {
		observed, exists := runs[wanted.Run]
		if !exists || observed.att.source != wanted.Source {
			return fmt.Errorf("acceptance credit: selected run %q and source do not match the landed witness", wanted.Run)
		}
		if seen[wanted.Run] {
			return fmt.Errorf("acceptance credit: selected run %s more than once", wanted.Run)
		}
		seen[wanted.Run] = true
		models[observed.att.model] = true
		strong = strong || observed.att.tier == "strong"
		b := observed.bracket
		if b.verdict != "match" {
			return fmt.Errorf("acceptance credit: selected run %s has no matching bracket", wanted.Run)
		}
		span := strings.Join(cc.lines[b.header-1:b.bracket-1], "\n")
		findings, refusal := closureWitnesses(protectedVerify, span)
		if refusal != "" {
			return fmt.Errorf("acceptance credit: run %s: %s", wanted.Run, refusal)
		}
		for _, finding := range findings {
			if finding.State != statePass {
				return fmt.Errorf("acceptance credit: run %s original row %s: %s", wanted.Run, finding.ID, finding.Detail)
			}
		}
	}
	if len(models) < 2 || !strong {
		return fmt.Errorf("acceptance credit: need two distinct verifier models and a strong tier")
	}
	for _, wanted := range selected {
		cc.source = wanted.Source
		v := cc.credit(runs[wanted.Run].bracket)
		if v.verdict != creditCredited {
			return fmt.Errorf("acceptance credit: run %s: %s: %s", wanted.Run, v.verdict, v.why)
		}
	}
	return nil
}
