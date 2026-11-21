package main

import (
	"fmt"
	"os"
	"strings"
)

// acceptanceRun identifies a selected verifier run in a source-bound acceptance
// record. The issuer and close adapter must authenticate that record and check
// selection, original-row coverage, and floor separately.
type acceptanceRun struct {
	Run    string `json:"run"`
	Source string `json:"source"`
}

// creditAcceptanceRuns rechecks the exact selected runs against the landed
// witness. It is read-only. A successful read does not authorize a card or close.
func creditAcceptanceRuns(witness string, selected []acceptanceRun, ciWorkflowToken bool) error {
	if len(selected) == 0 {
		return fmt.Errorf("acceptance credit: no verifier runs selected")
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
		source  string
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
		runs[run] = witnessRun{bracket: bracket, source: a.source}
	}

	seen := map[string]bool{}
	for _, wanted := range selected {
		observed, exists := runs[wanted.Run]
		if !exists || observed.source != wanted.Source {
			return fmt.Errorf("acceptance credit: selected run %q and source do not match the landed witness", wanted.Run)
		}
		if seen[wanted.Run] {
			return fmt.Errorf("acceptance credit: selected run %s more than once", wanted.Run)
		}
		seen[wanted.Run] = true
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
