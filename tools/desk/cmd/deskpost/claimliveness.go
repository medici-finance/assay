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
	"errors"
	"fmt"
	"net/http"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// refExists reports whether one ref is present in this client's repo.
//
// The ref is put through deskkit.ValidateRefPath first, so the only thing this read can
// address is a ref inside the named repo — the same bound DeleteRef carries, for the same
// reason: a path-shaped argument that reaches a URL is an arbitrary-endpoint reach unless
// something refuses the paths that are not refs.
//
// A 404 is the ANSWER "absent", not a failure — that is the whole point of the read. Every
// other non-2xx is an error, so a 403 from a token that cannot see refs can never be mistaken
// for "the claim was released".
func (c *ghClient) refExists(refPath string) (bool, error) {
	clean, err := deskkit.ValidateRefPath(refPath)
	if err != nil {
		return false, err
	}
	path := fmt.Sprintf("/repos/%s/%s/git/ref/%s", c.owner, c.repo, clean)
	if err := c.doJSON(http.MethodGet, path, nil, nil); err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.status == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// claimLiveness answers whether the dispatch claim behind this PR's stamp is still held, for
// the model-capability floor. repo is "<owner>/<name>" (the short-label resolution needs it);
// body is the PR body the trailer is read from.
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
	present, rerr := c.refExists(refPath)
	return deskkit.ClaimLivenessFromRefPresence(present, rerr)
}
