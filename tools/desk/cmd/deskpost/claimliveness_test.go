package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The transport's whole job is to tell "the claim ref is gone" apart from "I could not look".
// Only the first ages a stamp out, so a status that is NOT a clean 404 must never reduce to a
// release: a 403 from a token that cannot read refs, or a 500 from a bad minute at the forge,
// would otherwise throw away the attestation of a LIVE dispatch.
func TestRefExistsSeparatesAbsentFromCouldNotLook(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		wantPresent bool
		wantErr     bool
		wantLive    deskkit.ClaimLiveness
	}{
		{"ref present", http.StatusOK, true, false, deskkit.ClaimHeld},
		{"ref absent", http.StatusNotFound, false, false, deskkit.ClaimReleased},
		{"forbidden", http.StatusForbidden, false, true, deskkit.ClaimLivenessUnknown},
		{"server error", http.StatusInternalServerError, false, true, deskkit.ClaimLivenessUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(c.status)
				if c.status == http.StatusOK {
					_, _ = w.Write([]byte(`{"ref":"refs/heads/dispatch/one--st--07"}`))
				}
			}))
			defer srv.Close()
			setAPIBase(t, srv.URL)

			cl := &ghClient{owner: "example-org", repo: "one", token: "t", http: srv.Client()}
			present, err := cl.refExists("heads/dispatch/one--st--07")
			if present != c.wantPresent || (err != nil) != c.wantErr {
				t.Fatalf("refExists = (%v, %v), want (%v, err=%v)", present, err, c.wantPresent, c.wantErr)
			}
			if got := deskkit.ClaimLivenessFromRefPresence(present, err); got != c.wantLive {
				t.Fatalf("liveness = %v, want %v — a %d must not reduce to %v",
					got, c.wantLive, c.status, deskkit.ClaimReleased)
			}
			if want := "/repos/example-org/one/git/ref/heads/dispatch/one--st--07"; gotPath != want {
				t.Errorf("read %s, want %s", gotPath, want)
			}
		})
	}
}

// A ref path that is not a ref path never reaches a URL: the same bound DeleteRef carries, for
// the same reason — a path-shaped argument that is interpolated into an endpoint is an
// arbitrary-endpoint reach unless something refuses the paths that are not refs.
func TestRefExistsRefusesANonRefPath(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	cl := &ghClient{owner: "example-org", repo: "one", token: "t", http: srv.Client()}
	if _, err := cl.refExists("heads/../../branches/main/protection"); err == nil {
		t.Fatal("a traversing ref path was accepted")
	}
	if reached {
		t.Fatal("the refused path still reached the server — validation must happen before the request")
	}
}

// claimLiveness is Unknown for every PR whose dispatch claim cannot be identified. A body with
// no link trailer names no dispatch, and inventing a key for it would look up a claim nobody
// ever took — whose absence would then read as a release and age out a stamp on no evidence.
func TestClaimLivenessIsUnknownWithoutADerivableKey(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	cl := &ghClient{owner: "example-org", repo: "one", token: "t", http: srv.Client()}
	if got := cl.claimLiveness("example-org/one", "a human opened this PR by hand\n"); got != deskkit.ClaimLivenessUnknown {
		t.Fatalf("liveness = %v, want unknown for a body with no link trailer", got)
	}
	if reached {
		t.Error("a PR with no derivable claim key still triggered a claim-ref read")
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
