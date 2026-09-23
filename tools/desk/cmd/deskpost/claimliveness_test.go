package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimLiveness reads the REVIEW-dispatch claim family behind a reviewed PR's reviewer stamp,
// through the typed Forge op (deskkit.Forge.MatchingRefs) reached via deskkit.ForgeFor — the ONE
// sanctioned backend-construction site. The prefix-match + "--" boundary logic itself lives on the
// seam and in deskkit (ReviewClaimFamilyRefPrefix / RefInReviewClaimFamily /
// ReviewClaimLivenessFromMatchingRefs). What these tests own is deskpost's HALF: it derives the
// family prefix from (repo, pr), lists it through the resolved GitHub backend at the forgeAPIBase
// override, and reduces the answer so a read failure cannot become a release here and a no-op
// somewhere else.
//
// WHY THE FAMILY, NOT THE WORKER CLAIM. deskpost's floor consumers (a review verdict, a ready-flip)
// validate the REVIEWER's stamp, whose dispatch is the "<short>--pr-<N>" review-claim family — not
// the PR body's worker Brief: claim, which is released the moment the worker finishes. Keying the
// age-out on the worker claim aged out every reviewer stamp on every reviewed PR (assay#2875).
//
// The transport's whole job is to tell "the review family is gone" apart from "I could not look".
// Only the first ages a stamp out, so a status that is NOT a clean empty listing must never reduce
// to a release: a 403 from a token that cannot list refs, or a 500 from a bad minute at the forge,
// would otherwise throw away the attestation of a LIVE review dispatch.
func TestClaimLivenessSeparatesReleasedFromCouldNotLook(t *testing.T) {
	// A disambiguated member of PR 547's review family (short label of example-org/tracker is
	// "tracker"): a re-dispatch's "--rr1-corr" suffix, which the "--" boundary accepts.
	familyMember := "refs/dispatch/tracker--pr-547--rr1-corr"
	cases := []struct {
		name     string
		refs     []string // explicit family listing; nil + !released → the fake echoes a live family
		released bool     // serve an EMPTY family → ClaimReleased
		status   int      // matching-refs status; 0 → 200
		wantLive deskkit.ClaimLiveness
	}{
		{name: "family held → held", refs: []string{familyMember}, wantLive: deskkit.ClaimHeld},
		{name: "family empty → released", released: true, wantLive: deskkit.ClaimReleased},
		{name: "forbidden → unknown", status: http.StatusForbidden, wantLive: deskkit.ClaimLivenessUnknown},
		{name: "server error → unknown", status: http.StatusInternalServerError, wantLive: deskkit.ClaimLivenessUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, _ := setupFake(t)
			f.matchingRefs = c.refs
			f.reviewClaimReleased = c.released
			f.matchingRefsStatus = c.status
			cl, err := newGHClient("example-org", "tracker")
			if err != nil {
				t.Fatalf("newGHClient: %v", err)
			}
			got := cl.claimLiveness(exampleRepo, 547)
			if got != c.wantLive {
				t.Fatalf("claimLiveness = %v, want %v — a %d must not reduce to %v",
					got, c.wantLive, c.status, deskkit.ClaimReleased)
			}
		})
	}
}

// A ref that only PREFIX-matches a NEIGHBOUR PR must not be read as this PR's family: the
// matching-refs endpoint prefix-matches, so a listing for PR 548 could contain PR 5489's claims,
// and the reducer's "--" boundary is what keeps them apart. Here the only live claim belongs to a
// different PR number that shares a string prefix, so PR 548's family reads RELEASED, not held.
func TestClaimLivenessRejectsAPrefixNeighbourPR(t *testing.T) {
	f, _ := setupFake(t)
	// PR 5489's claim string-prefixes "tracker--pr-548" but is not in PR 548's family.
	f.matchingRefs = []string{"refs/dispatch/tracker--pr-5489--rr1-corr"}
	cl, err := newGHClient("example-org", "tracker")
	if err != nil {
		t.Fatalf("newGHClient: %v", err)
	}
	if got := cl.claimLiveness(exampleRepo, 548); got != deskkit.ClaimReleased {
		t.Fatalf("claimLiveness = %v, want released — PR 5489's claim is not PR 548's family", got)
	}
}

// claimLiveness is Unknown for a PR whose review-claim family cannot be identified — a non-positive
// PR number names no family, and inventing one would list a claim nobody ever took, whose absence
// would then read as a release and age out a stamp on no evidence. The read must not even be
// ATTEMPTED: no derivable family returns before the forge is resolved.
func TestClaimLivenessIsUnknownWithoutADerivableFamily(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	cl := &ghClient{owner: "example-org", repo: "tracker", token: "t", http: srv.Client()}
	if got := cl.claimLiveness(exampleRepo, 0); got != deskkit.ClaimLivenessUnknown {
		t.Fatalf("liveness = %v, want unknown for a non-positive PR number", got)
	}
	if reached {
		t.Error("a PR with no derivable review-claim family still triggered a matching-refs read")
	}
}

// setAPIBase points deskpost's REST reads at a test server for the duration of one test. The
// override is the package var the harness already uses; nothing here constructs a forge host.
func setAPIBase(t *testing.T, url string) {
	t.Helper()
	old := forgeAPIBase
	forgeAPIBase = url
	t.Cleanup(func() { forgeAPIBase = old })
}
