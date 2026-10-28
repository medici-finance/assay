package main

// Reconcile uses the same diff classifier as the approval/provenance resolver.
// Evidence and status bookkeeping cannot establish an implementation. Neither
// the PR's author, title nor a VERIFY FAIL string decides this question.

import "fmt"

type shapeResult struct {
	shape prShape
	err   error
}

// notImplementation excludes authoring and Evidence-only carriers. Declared
// partials stay in the lifecycle input (as in-progress); nonDelivery adds that
// exclusion for the historical backfill reader.
func (a *authoringClassifier) notImplementation(number int, briefID string) (bool, error) {
	only, err := a.authoringOnly(number, briefID)
	if err != nil || only {
		return only, err
	}
	files := a.files(number).files
	shape := prShape{}
	for _, f := range files {
		shape.Files = append(shape.Files, f.Filename)
		if f.PreviousFilename != "" {
			shape.RenamedFrom = append(shape.RenamedFrom, f.PreviousFilename)
		}
	}
	if !evidencePathsOnly(shape) {
		return false, nil
	}
	// Outcome/status-only paths need no hunk read. Brief/README edits require
	// immutable head content and patches from the existing complete-shape reader.
	if only, _ = evidenceOnlyReason(shape); !only {
		r, ok := a.shapes[number]
		if !ok {
			r.shape, r.err = reconcileFlipSource(modelReviewer().Forge).PRShape(a.repo, number)
			a.shapes[number] = r
		}
		if r.err != nil {
			return false, r.err
		}
		shape = r.shape
		if len(shape.Files) == 0 {
			return false, fmt.Errorf("PR #%d shape has no changed files", number)
		}
		if evidencePathsOnly(shape) {
			for _, p := range shape.Files {
				if !isStreamReadmePath(p) && !isStreamBriefPath(p) {
					continue
				}
				patch, _, why := patchAndHead(shape, p)
				if why != "" {
					return false, fmt.Errorf("%s", why)
				}
				if _, err := parsePatchChanges(patch); err != nil {
					return false, fmt.Errorf("unreadable patch for %s: %w", p, err)
				}
			}
		}
		only, _ = evidenceOnlyReason(shape)
	}
	if only && a.stderr != nil {
		fmt.Fprintf(a.stderr, "reconcile: NOTICE — PR #%d names %s but only changes Evidence or status bookkeeping, so it is not an implementation witness\n", number, briefID)
	}
	return only, nil
}

// This is only a read-cost filter. evidenceOnlyReason owns the actual decision;
// these are its shared path predicates, not a second hunk classifier.
func evidencePathsOnly(shape prShape) bool {
	if len(shape.Files) == 0 || len(shape.RenamedFrom) != 0 {
		return false
	}
	for _, p := range shape.Files {
		if !isCanonicalRepoPath(p) || !(p == "STATUS.md" || isVerifyOutcomePath(p) || isStreamReadmePath(p) || isStreamBriefPath(p)) {
			return false
		}
	}
	return true
}
