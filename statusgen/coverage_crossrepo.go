package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// crossCoverage binds one brief's execution history to the configured source
// checkout. The accepted question is still read in its tracking repository.
// This is an applicability observation, not attestation authentication or credit.
type crossCoverage struct {
	observation evaluation.BriefObservation
	revision    string
	repo        evaluation.Repo
}

// coverageCrossSource reuses the selected-run reader's binding checks. An
// unbound cross-repository record never falls back to a same-repository pass.
// Runner cell applicability is still judged by the evaluation library.
func coverageCrossSource(root, path, evidence string, obs *evaluation.BriefObservation) *crossCoverage {
	if !unboundCrossAttestation(evidence) {
		return nil
	}
	hold := func(err error) *crossCoverage {
		obs.AcceptanceProblem = "cross-repository coverage: " + err.Error()
		return nil
	}
	pass, err := flipLatestPass(evidence)
	if err != nil {
		return hold(err)
	}
	rows, _ := flipDateRunnerRows(strings.Split(pass.Run, "\n"))
	for _, row := range rows {
		if runner := flipRunnerRe.FindStringSubmatch(row[1]); runner != nil && strings.HasPrefix(runner[3], "+") {
			return hold(fmt.Errorf("source witness records uncommitted or unknown source bytes"))
		}
	}
	if err := crossCoverageSpan(evidence, pass.Run, obs.VerifyRows); err != nil {
		return hold(err)
	}
	if _, err := briefVersionForPass(path, evidence, pass); err != nil {
		return hold(err)
	}
	// briefVersionForPass has established that every literal selected v2
	// marker agrees. Read that same selected run, never another run's line.
	var binding containerAttestation
	for _, marker := range protectedMarkersNamed(normalizedProtectedEvidence(pass.Run), strings.TrimSpace(attestationPrefix)) {
		parsed, ok := parseContainerAttestation(marker.text)
		if ok && parsed.version == 2 {
			binding = parsed
		}
	}
	source := effectiveSiblingRootOverrides(siblingRootFlagValues)[binding.sourceRepo]
	if source == "" {
		return hold(fmt.Errorf("no configured checkout for source repository %s", binding.sourceRepo))
	}
	if ok, why := checkSiblingCheckout(source); !ok {
		return hold(fmt.Errorf("%s", why))
	}
	repo, err := attestationTrackingRepo(source)
	if err != nil {
		return hold(err)
	}
	if !strings.EqualFold(repo, binding.sourceRepo) || pathIsRoot(root, source) {
		return hold(fmt.Errorf("configured checkout does not identify the attested source repository"))
	}
	revision, err := integrationTip(source)
	if err != nil {
		return hold(err)
	}
	// Ordinary cross-repository rows use the definition held by the admitted
	// tracking snapshot. Explicit protected definition revisions stay intact.
	if obs.AcceptanceRevision == "" {
		obs.AcceptanceRevision = binding.trackingSource
	}
	return &crossCoverage{
		observation: *obs,
		revision:    revision,
		repo: newMemoRepo(crossCoverageRepo{
			Repo:           newMemoRepo(newCoverageRepo(source)),
			tracking:       newMemoRepo(newCoverageRepo(root)),
			trackingSource: binding.trackingSource,
		}),
	}
}

// Source history answers code applicability; tracking history answers the
// question and its declared scope. The source commit never names a definition
// in a different object store. This adapter deliberately has no WitnessRepo
// extension: cross-repository witness-v2 records need their own storage binding
// and remain could-not-check here.
type crossCoverageRepo struct {
	evaluation.Repo
	tracking       evaluation.Repo
	trackingSource string
}

func (r crossCoverageRepo) VerifyExpectAt(rev, briefRel, rowID string) (string, bool) {
	return r.tracking.VerifyExpectAt(rev, briefRel, rowID)
}

func (r crossCoverageRepo) DeclaredFilesAt(_ string, briefRel string) ([]string, bool, bool) {
	return r.tracking.DeclaredFilesAt(r.trackingSource, briefRel)
}

// crossCoverageSpan establishes the existing source-only container scope for
// every latest row. Host or mixed-root runs need the separate tracking-tree
// binding and cannot acquire that scope from a neighboring bracket.
func crossCoverageSpan(evidence, run string, rows []evaluation.VerifyRow) error {
	checks := bracketChecks(run)
	if len(checks) == 0 {
		return fmt.Errorf("source-only coverage requires a closed container bracket")
	}
	for _, check := range checks {
		if check.verdict != "match" {
			return fmt.Errorf("source-only coverage requires matching container brackets")
		}
	}
	if len(containerRowsOutsideSpans(run, checks)) != 0 {
		return fmt.Errorf("source witness has rows outside its container brackets")
	}
	all, selected := parseEvidenceRows(evidence), parseEvidenceRows(run)
	for _, row := range rows {
		got := lastCoverageExecution(selected[row.ID])
		if got == "" || got != lastCoverageExecution(all[row.ID]) || witnessSandboxOf(got) != sandboxContainerNetns {
			return fmt.Errorf("Verify row %s has no latest source-only container witness in the selected run", row.ID)
		}
	}
	return nil
}

func lastCoverageExecution(rows []evidenceRow) string {
	for i := len(rows) - 1; i >= 0; i-- {
		if isWitnessRow(rows[i].Text) {
			return rows[i].Text
		}
	}
	return ""
}
