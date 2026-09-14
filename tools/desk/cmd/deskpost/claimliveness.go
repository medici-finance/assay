package main

// claimliveness.go — deskpost's TRANSPORT for the model floor's stamp age-out.
//
// The DECISION lives once, in deskkit (stampage.go + modelfloor.go): a stamp whose dispatch
// claim has been released ages out and the PR reads unstamped. What each verb owns is only
// its own forge read of that one fact, exactly as each verb already owns its own read of the
// PR's label events. This file is deskpost's half: derive the claim key the PR's dispatch
// would have been claimed under, ask GitHub whether that claim ref is still there, and reduce
// the answer through deskkit's ONE reducer so a read failure cannot become a release here and
// a no-op somewhere else.
//
// EVERY UNCERTAIN PATH IS Unknown, WHICH CHANGES NOTHING. A body with no derivable claim key,
// a key that will not build a legal ref path, and any API error other than a clean 404 all
// return ClaimLivenessUnknown, and Unknown leaves the stamp standing exactly as it stood
// before this mechanism existed. Only a successful read that positively reports the ref ABSENT
// ages a stamp out.

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimLiveness answers whether the dispatch claim behind this PR's stamp is still held, for
// the model-capability floor. repo is "<owner>/<name>" (the short-label resolution needs it);
// body is the PR body the trailer is read from.
//
// The ref-existence read is now the typed Forge op (deskkit.Forge.RefExists), not a hand-rolled
// REST call: the logic that was this file's own `refExists` moved onto the seam (the GitHub
// backend's RefExists) so a second forge implements the read rather than deskpost forking it.
// The backend is reached through deskkit.ForgeFor — the ONE sanctioned construction site — the
// same proof-of-reachability wiring runComment uses for PostComment, authenticated via the SAME
// custody path (github.go's init minter). This stays deskpost's GitHub-only precondition path:
// newGHClient has already refused a GitLab-resolved repo before any verb reaches claimLiveness,
// so ForgeFor resolves GitHub here.
//
// EVERY UNCERTAIN PATH IS Unknown, WHICH CHANGES NOTHING. A body with no derivable claim key, a
// key that will not build a legal ref path, a resolver/custody failure, and any read error other
// than a clean ABSENT all return ClaimLivenessUnknown, and Unknown leaves the stamp standing
// exactly as it stood before this mechanism existed. Only a successful read that positively
// reports the ref ABSENT ages a stamp out.
func (c *ghClient) claimLiveness(repo, body string) deskkit.ClaimLiveness {
	key, ok := deskkit.ClaimKeyForPR(repo, body)
	if !ok {
		// No derivable claim key — a human-opened PR, or a body whose trailer set does not
		// name one. There is no claim to look for, so there is nothing to age out.
		return deskkit.ClaimLivenessUnknown
	}
	refPath, err := deskkit.ClaimRefPath(key)
	if err != nil {
		return deskkit.ClaimLivenessUnknown
	}
	fr := deskkit.ForgeRepo{Owner: c.owner, Name: c.repo}
	fg, ferr := deskkit.ForgeFor(fr, "reviewer")
	if ferr != nil {
		// Could not even construct the backend (resolver/custody failure). Could-not-check —
		// never a release.
		return deskkit.ClaimLivenessUnknown
	}
	present, rerr := fg.RefExists(fr, refPath)
	return deskkit.ClaimLivenessFromRefPresence(present, rerr)
}
