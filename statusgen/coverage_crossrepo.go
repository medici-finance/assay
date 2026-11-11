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
	// Host rows can read the admitted tracking tree as well as the source.
	// Establish that second input before allowing them beside a sealed span.
	hostBound := crossTrackingUnchanged(root, binding.trackingSource)
	if err := crossCoverageSpan(evidence, pass.Run, obs.VerifyRows, hostBound == nil); err != nil {
		if hostBound != nil {
			return hold(fmt.Errorf("%w; tracking inputs: %v", err, hostBound))
		}
		return hold(err)
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

// crossCoverageSpan requires a closed source bracket and a latest witness in
// this run for every row. Host rows additionally require unchanged tracking
// inputs; they never inherit source-only scope from the neighboring bracket.
func crossCoverageSpan(evidence, run string, rows []evaluation.VerifyRow, trackingUnchanged bool) error {
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
		if got == "" || got != lastCoverageExecution(all[row.ID]) {
			return fmt.Errorf("Verify row %s has no latest execution witness in the selected run", row.ID)
		}
		if witnessSandboxOf(got) != sandboxContainerNetns {
			execClass, _ := splitRowClassCell(row.ClassCell)
			class, _ := resolveRowClass(execClass)
			if class == classCheckCI {
				return fmt.Errorf("Verify row %s requires a sealed container witness", row.ID)
			}
			if !trackingUnchanged {
				return fmt.Errorf("Verify row %s needs unchanged tracking inputs for its host witness", row.ID)
			}
		}
	}
	return nil
}

// crossTrackingUnchanged permits only the bookkeeping edits already recognized
// by the delivery classifier. A changed policy, definition or other input stays
// held, including changes under docs/streams. The admitted commit and current
// tracking tree are separate from the deliverable's object store.
func crossTrackingUnchanged(root, admitted string) error {
	repo := newCoverageRepo(root)
	current, ok := repo.ResolveCommit("HEAD")
	if !ok || repo.MergeBase(admitted, current) != evaluation.HistoryRelated {
		return fmt.Errorf("cannot establish tracking history from %s", admitted)
	}
	files, ok := repo.DiffNames(admitted, current)
	if !ok {
		return fmt.Errorf("cannot compare the admitted tracking tree")
	}
	if len(files) == 0 {
		return nil
	}
	read := gitCoreForCommits(root, admitted, current)
	if read.err != nil {
		return fmt.Errorf("cannot read tracking objects: %w", read.err)
	}
	if read.native != "" {
		return fmt.Errorf("tracking objects need an unsupported read profile: %s", read.native)
	}
	shape := prShape{Files: files, Patches: map[string]string{}, Heads: map[string]string{}}
	for _, file := range files {
		if !isStreamBriefPath(file) && !isStreamReadmePath(file) {
			continue
		}
		patch, err := read.repo.FileDiff(admitted, current, file)
		if err != nil {
			return fmt.Errorf("cannot read tracking patch for %s", file)
		}
		head, err := read.repo.FileAt(current, file)
		if err != nil {
			return fmt.Errorf("cannot read current tracking path %s", file)
		}
		// The shared classifier consumes hunk lines, as supplied by a forge.
		if start := strings.Index(patch, "@@ "); start >= 0 {
			shape.Patches[file] = patch[start:]
		}
		shape.Heads[file] = head
	}
	if only, why := evidenceOnlyReason(shape); !only {
		return fmt.Errorf("tracking inputs changed since admission: %s", why)
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
