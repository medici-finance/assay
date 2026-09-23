package main

// claimliveness.go — deskpost's TRANSPORT for the model floor's stamp age-out.
//
// The DECISION lives once, in deskkit (stampage.go + modelfloor.go): a stamp whose dispatch
// claim has been released ages out and the PR reads unstamped. What each verb owns is only its
// own forge read of that one fact, exactly as each verb owns its own read of the PR's label
// events. This file is deskpost's half.
//
// WHAT THIS AGE-OUT IS FOR, AND WHY IT READS THE REVIEW-CLAIM FAMILY. deskpost's two floor
// consumers — the review VERDICT (review.go) and the ready-FLIP (ready.go) — are both
// REVIEW-LANE authority writes on a PR under review, and the stamp each validates is the
// REVIEWER's dispatch stamp, applied by the reviewer App via `deskdispatch --kit review --pr N`.
// The dispatch behind that stamp is the review-dispatch claim, whose key is "<short>--pr-<N>"
// (or, for a re-dispatch, "…--<suffix>"). So the age-out asks: is ANY claim in that PR's
// review-dispatch family still held? — NOT whether the PR's own WORKER dispatch (the Brief:
// trailer's "<short>--<stream>--<NN>" claim) is held. The worker claim is released the moment
// the worker finishes, long before any review, so keying the age-out on it (the prior behavior)
// aged out every reviewer stamp on every reviewed PR and refused every risk-classed verdict/flip.
//
// WHERE THE FAMILY LIVES. It is matched by prefix in DispatchClaimActiveRefsPrefix
// ("refs/dispatch/*") — the namespace claims are ACTUALLY acquired in today — not ClaimRefsPrefix
// ("refs/heads/dispatch/*") that the fleet's other Go readers list against. That divergence is
// issue 708; this reads where the claim actually is (the correct reader→writer direction), leaves
// the acquire/writer path untouched, and collapses back onto one namespace when 708 lands.
//
// EVERY UNCERTAIN PATH IS Unknown, WHICH CHANGES NOTHING. No derivable family prefix, a
// resolver/custody failure, and any read error all return ClaimLivenessUnknown, leaving the stamp
// standing. Only a successful listing that positively reports the family EMPTY ages a stamp out.
// This stays deskpost's GitHub-only precondition path: newGHClient refuses a GitLab-resolved repo
// before any verb reaches claimLiveness, so ForgeFor resolves GitHub here.

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimLiveness answers whether the REVIEW-dispatch claim behind this PR's reviewer stamp is
// still held, for the model-capability floor. repo is "<owner>/<name>" (the short-label
// resolution needs it); pr is the PR number the review-claim family is keyed on.
func (c *ghClient) claimLiveness(repo string, pr int) deskkit.ClaimLiveness {
	prefix, ok := deskkit.ReviewClaimFamilyRefPrefix(repo, pr)
	if !ok {
		// No derivable family prefix (no short label, or a non-positive PR number). There is no
		// family to look for, so there is nothing to age out.
		return deskkit.ClaimLivenessUnknown
	}
	fr := deskkit.ForgeRepo{Owner: c.owner, Name: c.repo}
	fg, ferr := deskkit.ForgeFor(fr, "reviewer")
	if ferr != nil {
		// Could not even construct the backend (resolver/custody failure). Could-not-check —
		// never a release.
		return deskkit.ClaimLivenessUnknown
	}
	refs, rerr := fg.MatchingRefs(fr, prefix)
	return deskkit.ReviewClaimLivenessFromMatchingRefs(refs, prefix, rerr)
}
