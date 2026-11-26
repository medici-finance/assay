package main

import (
	"fmt"
	"strings"
)

// verifyAcceptanceReadiness is the shared read-only verification before either
// canonical effect. Lifecycle eligibility, decision hold, and actor checks
// specific to the issue publisher or human closer remain at those boundaries.
func verifyAcceptanceReadiness(root, path string, m acceptanceManifest, preparation int, ruling string,
	forge *ghClient, humanLogins map[string]string, ciWorkflowToken bool) error {
	if err := checkAcceptanceBriefBinding(root, path, m); err != nil {
		return err
	}
	source, err := readAcceptanceSourceAt(root, path, m.Source.Revision)
	if err != nil {
		return err
	}
	if err := m.checkDispositions(source, true); err != nil {
		return err
	}
	if err := m.checkResolvedEvidence(root, scanEffectiveConfig()); err != nil {
		return err
	}
	definition, err := readAcceptanceDefinition(root, source)
	if err != nil {
		return err
	}
	if err := m.checkDefinition(definition); err != nil {
		return err
	}
	_, raw, err := readAcceptanceBlob(root, source.Revision, source.BriefPath)
	if err != nil {
		return err
	}
	bf, ok, err := parseBriefFileBytes(path, raw)
	if err != nil || !ok {
		return fmt.Errorf("acceptance adjudication: could not read canonical brief: %v", err)
	}
	local, _, ok := expectedBriefID(path)
	if !ok {
		return fmt.Errorf("acceptance adjudication: cannot bind the canonical brief id")
	}
	p, err := flipLatestPass(bf.Evidence)
	if err != nil {
		return fmt.Errorf("acceptance adjudication: latest verification is not a strict passing run: %w", err)
	}
	if err := closeVerifyFailRefusal(local, "verified", bf.Evidence); err != nil {
		return err
	}
	date, runner := evidenceVerifierInfo(bf.Evidence)
	if date == "" || runner == "" {
		return fmt.Errorf("acceptance adjudication: latest verification has no coherent date and runner")
	}
	if err := closeVerifyFloorRefusal(local, bf, date+" "+runner); err != nil {
		return err
	}
	if err := checkAcceptanceLatestRun(p.Run, m.Runs); err != nil {
		return err
	}
	if err := verifyAcceptancePreparation(m, preparation, ciWorkflowToken); err != nil {
		return err
	}
	if err := corroborateAcceptanceAdjudication(forge, m, preparation, ruling, humanLogins); err != nil {
		return err
	}
	return creditAcceptanceRuns(path, definition.verify, m.Runs, ciWorkflowToken)
}

// checkAcceptanceBriefBinding refuses a preparation, and so any adjudication
// of it, that names a brief other than the one being issued or closed. The
// route applies to any gate: human brief, so a record made for one brief must
// never stand in for another. The full source comparison that follows also
// binds the path; this check names the cause before any other read.
func checkAcceptanceBriefBinding(root, path string, m acceptanceManifest) error {
	rel, inside := relWithin(root, path)
	if !inside || rel == "" || m.Source.BriefPath != rel {
		return fmt.Errorf("acceptance adjudication: the preparation binds brief %q, not %q", m.Source.BriefPath, rel)
	}
	return nil
}

func checkAcceptanceLatestRun(run string, selected []acceptanceRun) error {
	var latest acceptanceRun
	for _, line := range strings.Split(run, "\n") {
		if a, ok := parseContainerAttestation(line); ok {
			if latest.Run != "" {
				return fmt.Errorf("acceptance adjudication: latest run contains multiple attestations")
			}
			latest = acceptanceRun{Run: a.run, Source: a.source}
		}
	}
	if latest.Run != "" {
		for _, s := range selected {
			if s == latest {
				return nil
			}
		}
	}
	return fmt.Errorf("acceptance adjudication: selected runs omit the latest passing attestation")
}
