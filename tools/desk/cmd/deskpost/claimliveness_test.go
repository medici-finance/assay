package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimLiveness now reads the claim ref through the typed Forge op (deskkit.Forge.RefExists),
// reached via deskkit.ForgeFor — the ONE sanctioned backend-construction site — not a hand-rolled
// REST call. The "absent vs could-not-look" logic itself moved onto the seam and is pinned there
// (deskkit.TestGitHubForgeRefExists* + the forge golden corpus). What these tests own is
// deskpost's HALF: it derives the claim key, reads the ref through the resolved GitHub backend at
// the forgeAPIBase override, and reduces the answer through deskkit's ONE reducer so a read
// failure cannot become a release here and a no-op somewhere else.
//
// The transport's whole job is to tell "the claim ref is gone" apart from "I could not look".
// Only the first ages a stamp out, so a status that is NOT a clean 404 must never reduce to a
// release: a 403 from a token that cannot read refs, or a 500 from a bad minute at the forge,
// would otherwise throw away the attestation of a LIVE dispatch.
func TestClaimLivenessSeparatesReleasedFromCouldNotLook(t *testing.T) {
	cases := []struct {
		name     string
		status   int // git/ref status; 0 → present (200)
		wantLive deskkit.ClaimLiveness
	}{
		{"ref present → held", 0, deskkit.ClaimHeld},
		{"ref absent → released", http.StatusNotFound, deskkit.ClaimReleased},
		{"forbidden → unknown", http.StatusForbidden, deskkit.ClaimLivenessUnknown},
		{"server error → unknown", http.StatusInternalServerError, deskkit.ClaimLivenessUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, _ := setupFake(t)
			f.claimRefStatus = c.status
			cl, err := newGHClient("example-org", "tracker")
			if err != nil {
				t.Fatalf("newGHClient: %v", err)
			}
			// The body carries a Brief trailer, so the claim key is derivable and the ref read
			// through ForgeFor actually fires against the fake's git/ref route.
			got := cl.claimLiveness(exampleRepo, "does the thing\n\nBrief: st/07\n")
			if got != c.wantLive {
				t.Fatalf("claimLiveness = %v, want %v — a %d must not reduce to %v",
					got, c.wantLive, c.status, deskkit.ClaimReleased)
			}
		})
	}
}

// claimLiveness is Unknown for every PR whose dispatch claim cannot be identified. A body with
// no link trailer names no dispatch, and inventing a key for it would look up a claim nobody
// ever took — whose absence would then read as a release and age out a stamp on no evidence.
// The read must not even be ATTEMPTED: no derivable key returns before the forge is resolved.
func TestClaimLivenessIsUnknownWithoutADerivableKey(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	cl := &ghClient{owner: "example-org", repo: "tracker", token: "t", http: srv.Client()}
	if got := cl.claimLiveness(exampleRepo, "a human opened this PR by hand\n"); got != deskkit.ClaimLivenessUnknown {
		t.Fatalf("liveness = %v, want unknown for a body with no link trailer", got)
	}
	if reached {
		t.Error("a PR with no derivable claim key still triggered a claim-ref read")
	}
}

// A malformed Brief trailer is also not a derivable key — Unknown, not a read (and never a
// release). This guards the reducer's fail-safe direction at the key-derivation boundary.
func TestClaimLivenessIsUnknownForAMalformedTrailer(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	cl := &ghClient{owner: "example-org", repo: "tracker", token: "t", http: srv.Client()}
	if got := cl.claimLiveness(exampleRepo, "Brief: not-a-valid-brief-shape\n"); got != deskkit.ClaimLivenessUnknown {
		t.Fatalf("liveness = %v, want unknown for a malformed trailer", got)
	}
	if reached {
		t.Error("a malformed trailer still triggered a claim-ref read")
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
