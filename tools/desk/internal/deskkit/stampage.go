package deskkit

// stampage.go — a dispatch stamp AGES OUT when the cycle that applied it is dead.
//
// WHAT THIS CLOSES. The model-capability floor reads the dispatched-* stamp a PR carries and
// fails closed on one it cannot trust. That is right while the dispatch is LIVE. It is wrong
// once the dispatching cycle is over: a stamp left behind by a cycle that never posted a
// verdict then blocks every authority-bearing write on that PR forever, and it blocks it
// HARDER than no stamp at all — an unstamped PR takes the proceed-with-NOTICE branch, while a
// dead stamp takes the refuse branch. Two verified non-risk PRs sat bricked behind stamps a
// days-dead cycle had left, when the very same PRs with no stamp would have posted fine. That
// asymmetry — dead attestation treated as WORSE than absent attestation — is the defect.
//
// THE RULE. A stamp whose dispatch claim has been RELEASED is IGNORED: the PR reads as
// unstamped and takes the same branch an unstamped PR takes. The stamp is not trusted, not
// distrusted, not repaired — it is simply not an attestation about the session performing
// this write, because the session it attested for is gone.
//
// WHY THE CLAIM, NOT A CLOCK. The obvious alternative is a wall-clock threshold ("a stamp
// older than N hours is stale"). It fails in both directions at once: any threshold short
// enough to release a dead stamp promptly also expires a LIVE long-running dispatch out from
// under itself, and any threshold long enough to be safe for those leaves a recently-dead
// stamp bricking the PR for the whole window. The dispatch claim is the fact that actually
// carries the meaning: the claim is taken when the cycle starts and released when it ends, so
// "the claim is gone" IS "the cycle is dead", with no threshold to tune and no clock to skew.
//
// WHICH DIRECTION THIS FAILS. Age-out only ever moves a decision toward the UNSTAMPED branch,
// and it happens only on POSITIVE evidence of release: the claim key had to be derivable from
// the PR and the presence read had to SUCCEED and report the ref absent. A read that failed,
// a PR whose claim key cannot be derived, and a verb with no presence read at all are all
// ClaimLivenessUnknown, and Unknown leaves the stamp exactly as it stands. Could-not-check is
// never rounded into a release.
//
// THE COST, STATED. A stamp applied on a lane that never took a forge claim reads RELEASED,
// because a released claim and a claim never taken are the same absence on an append-only
// forge — the ref is deleted on release, so there is nothing left to tell them apart. On
// today's semantics that costs nothing a caller could not already have: the branch it moves
// such a PR to is the one an UNSTAMPED PR takes, and anyone able to apply a stamp was equally
// able to apply none. It is recorded here because it stops being free the moment an unstamped
// branch is made to refuse for some class of PR — at which point the age-out tightens rather
// than loosens, and the lanes that take no claim have to take one.

import "strings"

// ClaimLiveness is the three-state answer to "is the dispatch claim that produced this PR's
// stamp still held?". The ZERO VALUE is the non-answer, so a consumer that forgets to resolve
// it leaves the stamp standing rather than silently ageing it out.
type ClaimLiveness int

const (
	// ClaimLivenessUnknown is the ZERO VALUE: the claim's state could not be established —
	// no presence read was wired, the read failed, or the PR names no derivable claim key.
	// The stamp stands exactly as it would without this mechanism.
	ClaimLivenessUnknown ClaimLiveness = iota
	// ClaimHeld means the dispatch claim is still standing: the cycle is alive and its stamp
	// is a live attestation.
	ClaimHeld
	// ClaimReleased means the dispatch claim is gone: the cycle is dead and its stamp ages
	// out. This is the ONLY value that changes a floor decision.
	ClaimReleased
)

func (c ClaimLiveness) String() string {
	switch c {
	case ClaimHeld:
		return "held"
	case ClaimReleased:
		return "released"
	default:
		return "unknown"
	}
}

// AgesOutStamp reports whether this liveness makes a stamp read as unstamped. It exists so
// consumers write the question rather than compare against one of three values and pick the
// wrong side of the fail-closed line.
func (c ClaimLiveness) AgesOutStamp() bool { return c == ClaimReleased }

// ClaimLivenessFromRefPresence reduces ONE verb's claim-ref presence read to a liveness. Every
// verb owns its own transport (the forge surfaces differ) but none of them owns the REDUCTION,
// so a read error cannot become a release in one verb and a no-op in another.
//
// An error is Unknown, never Released: a claim we could not look for is not a claim we saw
// released, and treating a failed read as a release is exactly how a LIVE dispatch would get
// its attestation thrown away.
func ClaimLivenessFromRefPresence(present bool, err error) ClaimLiveness {
	if err != nil {
		return ClaimLivenessUnknown
	}
	if present {
		return ClaimHeld
	}
	return ClaimReleased
}

// ClaimKeyForPR derives the dispatch claim key a PR's own dispatch would have been claimed
// under, from the repo and the PR body's link trailer. It returns ok=false when the body names
// no derivable key — the caller's answer for that is ClaimLivenessUnknown, never a release.
//
// It MIRRORS deskdispatch's claimKeyFor: the repo's short label, then the item, with every
// "/" written as "--". The two must agree or the reader looks for a claim in a place the
// writer never took one, which reads as released on every PR — so the shapes are derived from
// the same two facts (RepoShortLabel and the trailer) rather than each spelled independently:
//
//	Brief: <stream>/<NN>  ->  <short>--<stream>--<NN>
//	Issue: #<N>           ->  <short>--issue-<N>
//
// A malformed trailer SET is not a key. So is a body with neither trailer — a human-opened PR
// names no dispatch, and inventing a key for it would look up a claim nobody ever took.
func ClaimKeyForPR(repo, body string) (string, bool) {
	short := strings.TrimSpace(RepoShortLabel(repo))
	if short == "" {
		return "", false
	}
	trs, err := ParseTrailers([]byte(body))
	if err != nil {
		return "", false
	}
	for _, t := range trs {
		if t.Kind != TrailerBrief {
			continue
		}
		stream, nn, ok := SplitBriefTrailer(t.Value)
		if !ok {
			return "", false
		}
		return short + "--" + stream + "--" + nn, true
	}
	for _, t := range trs {
		if t.Kind != TrailerIssue {
			continue
		}
		n := strings.TrimSpace(t.Value)
		if n == "" {
			return "", false
		}
		return short + "--issue-" + n, true
	}
	return "", false
}
