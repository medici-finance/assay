package main

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// github.go — the READ half. deskmerge makes no mutating forge call on any path; the
// only write it can perform is a `git push` of the PR's own branch. Every read here goes
// through the resolved deskkit.Forge under this session's minted App token (forge.go) —
// never a `gh` subprocess under an ambient identity (desktools-v2/03).

// prInfo is the subset of a PR's state deskmerge's decisions depend on.
type prInfo struct {
	Number      int
	State       string // OPEN | MERGED | CLOSED (deskkit.ParsePRKind's vocabulary)
	IsDraft     bool
	HeadRefName string
	HeadRefOid  string
	BaseRefName string
	// CrossRepo is deskkit.PullRequest.CrossRepo verbatim: CrossRepoSame, CrossRepoFork, or
	// EMPTY when the forge did not establish where the head branch lives — which
	// eligibleForMerge treats as could-not-check, never as "same".
	CrossRepo string
}

// prStateWord maps the forge-neutral change read onto ParsePRKind's vocabulary. A merged
// change reports State "closed" on GitHub, so merged-ness is read from the forge's own merged
// flag / timestamp, never inferred from State — a merged PR misread as merely closed is still
// refused, but the refusal must name the right reason.
func prStateWord(pr *deskkit.PullRequest) string {
	if pr.Merged || pr.MergedAt != "" {
		return "MERGED"
	}
	return strings.ToUpper(strings.TrimSpace(pr.State))
}

// fetchPR reads the PR's state.
//
// A failed read is Unverifiable (6) — could-not-check — never an assumed open draft.
// deskkit.ParsePRKind exists because "not stated" reading as "open" is the #247 shape, and
// a merge tool that guesses "open" would push to the branch of a PR that has already landed.
func fetchPR(repo string, number int) (prInfo, error) {
	fg, fr, ferr := forgeFor(repo)
	if ferr != nil {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d's state — deskmerge will not act on a PR whose "+
				"open/merged/draft status it could not establish", deskkit.StripControl(repo), number), ferr)
	}
	pr, err := fg.GetPullRequest(fr, number)
	if err != nil || pr == nil {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: cannot read %s#%d's state — deskmerge will not act on a PR whose "+
				"open/merged/draft status it could not establish", deskkit.StripControl(repo), number), err)
	}
	p := prInfo{
		Number:      pr.Number,
		State:       prStateWord(pr),
		IsDraft:     pr.Draft,
		HeadRefName: pr.HeadRef,
		HeadRefOid:  pr.HeadSHA,
		BaseRefName: pr.BaseRef,
		CrossRepo:   pr.CrossRepo,
	}
	if p.Number == 0 {
		p.Number = number
	}
	if p.HeadRefOid == "" {
		return prInfo{}, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d reported no head oid — every currency verdict is relative to a "+
				"head, and an unknown head makes every one of them unverifiable",
			deskkit.StripControl(repo), number), nil)
	}
	return p, nil
}

// eligibleForMerge applies the state preconditions R-5 binds `deskmerge merge` to. It
// is separate from the read so `check` can report on any PR while `merge` refuses most
// of them.
//
// Each refusal is a POSITIVE determination, hence exit 5 rather than 6.
func eligibleForMerge(repo string, p prInfo) error {
	switch kind := deskkit.ParsePRKind(p.State); kind {
	case deskkit.PROpen:
		// the only eligible state
	case deskkit.PRUnknown:
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d reported state %q, which is neither OPEN, MERGED nor CLOSED — "+
				"an unrecognised state is not an open PR", deskkit.StripControl(repo), p.Number,
			deskkit.StripControl(p.State)), nil)
	default:
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is %s — deskmerge never pushes to the branch of a PR that is no longer "+
				"open (the deskpushguard shape)", deskkit.StripControl(repo), p.Number, kind))
	}

	// R-5's flip bound, in code. The ready-flip criterion is "reviewer App APPROVED at
	// the current head". A desk merge REPLACES the head. So a desk merge on an
	// already-flipped PR would let the desk supply the very commit its own flip check
	// is evaluated against — the tool would be manufacturing its own gate's input.
	if !p.IsDraft {
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d is already flipped ready-for-review — R-5's flip bound forbids a desk "+
				"merge here, because the merge would replace the head that the desk's own "+
				"approval-at-head check is evaluated against. Merge-currency belongs BEFORE the "+
				"review, not after the flip (tracker#1544).",
			deskkit.StripControl(repo), p.Number))
	}

	// A cross-repository PR's head branch lives in a fork the desk has no push
	// authority over. Refusing here is honest; attempting the push and reporting the
	// remote's rejection would burn a charged write to learn something already known.
	switch p.CrossRepo {
	case deskkit.CrossRepoSame:
	case deskkit.CrossRepoFork:
		return deskkit.Refused(fmt.Sprintf(
			"refused: %s#%d's head branch lives in a fork — deskmerge pushes only to the PR's own "+
				"branch in the base repository", deskkit.StripControl(repo), p.Number))
	default:
		return deskkit.Unverifiable(fmt.Sprintf(
			"could-not-check: %s#%d did not report which repository its head branch lives in — "+
				"deskmerge pushes only to a branch it has established is in the base repository",
			deskkit.StripControl(repo), p.Number), nil)
	}
	return nil
}
