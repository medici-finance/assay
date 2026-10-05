package deskkit

// heldchange.go — the ONE path a desk verb opens a change through.
//
// On a forge with a merge-hold (GitLab today) a change is only flippable to ready if it carries
// the desk's merge-hold marker thread: deskflip's reviewer-approved condition RELEASES that hold
// at the reviewer's approve verdict, and refuses a change that carries none (MergeHoldAbsent).
// deskpr create opened the hold right after its create call, but deskevidence's draft lanes
// (the verifier's Evidence row and outcome record on a default branch that takes no direct
// write) called CreateDraftChange on their own and never opened one — so every verifier-authored
// Evidence draft came out permanently unflippable, approve or no approve.
//
// The defect class is "a CreateDraftChange call not paired with OpenMergeHold". This file is the
// pairing, and TestDraftChangeOnlyViaHeld fails on any CreateDraftChange call outside it (the
// checking decorator's own delegation excepted), so the next verb that opens a change cannot
// repeat the defect by calling the raw seam.

import "fmt"

// CreateHeldDraftChange opens in as a DRAFT change on repo and then opens the desk's merge-hold
// marker thread on it. The typed not-applicable (GitHub — its twin control is server-side branch
// protection) counts as success: there is nothing to open there.
//
// A create failure returns (nil, err) and opens no hold. A hold failure returns the created ref
// TOGETHER WITH a could-not-check error: the change already exists, WITHOUT its server-side merge
// gate armed, and the caller must name it and fail loud rather than report a clean landing — a
// change missing its hold is not discovered until a ready-flip refuses for a reason that reads
// like a different problem.
func CreateHeldDraftChange(fg Forge, repo ForgeRepo, in DraftChangeInput) (*PullRef, error) {
	ref, err := fg.CreateDraftChange(repo, in)
	if err != nil {
		return nil, err
	}
	if ref == nil || ref.Number <= 0 {
		return ref, Unverifiable(fmt.Sprintf(
			"could-not-check: the draft change opened on %s from %s came back with no number, so its "+
				"merge-hold marker thread could not be opened — the change may exist WITHOUT its "+
				"server-side merge gate armed", repo.Slug(), in.Head), nil)
	}
	if _, herr := fg.OpenMergeHold(repo, ref.Number); herr != nil && !IsMergeHoldNotApplicable(herr) {
		loc := ref.URL
		if loc == "" {
			loc = fmt.Sprintf("change #%d in %s", ref.Number, repo.Slug())
		}
		return ref, Unverifiable(fmt.Sprintf(
			"%s was created, but opening its merge-hold marker thread failed: %v — the change exists "+
				"WITHOUT its server-side merge gate armed, and a ready-flip will refuse it. Open one by "+
				"hand before the change is reviewed.", loc, herr), herr)
	}
	return ref, nil
}
