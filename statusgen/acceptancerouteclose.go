package main

import (
	"fmt"
	"reflect"
	"time"
)

// verifiedAcceptanceAdjudication is the result of a live readiness recheck.
// No marker, CLI flag or stored field can instantiate it.
type verifiedAcceptanceAdjudication struct {
	source acceptanceSource
}

func closeVerifyAdjudicatedHold(root, briefID, status string, bf *BriefFile, adjudication *verifiedAcceptanceAdjudication) error {
	if adjudication == nil {
		return closeVerifyHeldRefusal(briefID, status, bf.Evidence)
	}
	source, err := readAcceptanceSourceAt(root, bf.Path, adjudication.source.Revision)
	if err != nil {
		return err
	}
	if len(source.Refusals) == 0 || !reflect.DeepEqual(source, adjudication.source) {
		return fmt.Errorf("acceptance adjudication: close no longer matches the adjudicated hold scan")
	}
	return nil
}

// planAdjudicatedClose re-runs all route evidence and authority checks. The
// ordinary decision gate still sees the exact README transition. This function
// plans only. The workflow must separately authenticate a canonical card and
// its final human closer before applying the plan.
func planAdjudicatedClose(root, ref string, m acceptanceManifest, preparation int, ruling string, now time.Time,
	forge *ghClient, humanLogins map[string]string, ciWorkflowToken bool) (readme string, updated []byte, local string, err error) {
	bf, err := locateAcceptanceBrief(root, ref)
	if err != nil {
		return "", nil, "", err
	}
	if err := verifyAcceptanceReadiness(root, bf.Path, m, preparation, ruling, forge, humanLogins, ciWorkflowToken); err != nil {
		return "", nil, "", err
	}
	return closeVerifyPlanFor(root, ref, now, &verifiedAcceptanceAdjudication{source: m.Source})
}
