package main

// identity_test.go — the two identity refusals, and the negative-path proof that neither of
// them leaves a forge call behind.
//
// SUCCESSORS, 1:1, for the argv-era assertions this file used to make (see deskflip_test.go's
// header for the full map):
//
//	"the refusal made no `gh` call"         → the recorder saw ZERO requests
//	"a gh call with no minted token does
//	 not run"                               → TestForgeCallRefusedWithNoMintedToken: the
//	                                          backend refuses to build a client without an
//	                                          explicitly minted token, and issues no request
//	"every gh child gets GH_TOKEN"          → TestMintedTokenAuthenticatesEveryRequest: every
//	                                          recorded request carries the minted token in its
//	                                          Authorization header
//
// The last pair is where the successor is STRICTER than what it replaces. Asserting that a
// child process received an environment variable proved the value was handed over; asserting
// the Authorization header proves it was actually USED to authenticate, on every request.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// With the role's App token unavailable, deskflip refuses — it does NOT proceed on the
// operator's ambient credential. This is the whole defect: a flip made on the ambient login
// writes under a human identity and reads afterwards as a human decision.
func TestNoAppTokenRefusesAndNeverTouchesTheForge(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "", "/config/home/" + role + "-token-1", errors.New("private key not found")
	}

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused) — a missing App credential is a settled state, not a "+
			"licence to write as somebody else", rc, deskkit.ExitRefused)
	}
	if len(s.requests) != 0 {
		t.Fatalf("the refusal still made %d forge call(s): %v", len(s.requests), s.requests)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("the refusal still mutated: %v", m)
	}
}

// The refusal has to be actionable: it names the ROLE whose token is missing and the PATH the
// token was looked for at, and it never prints a token value.
func TestNoAppTokenRefusalNamesTheRoleAndThePath(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	const tokenPath = "/config/home/reviewer-token-1"
	mintTokenFn = func(role, repo string) (string, string, error) {
		return "", tokenPath, errors.New("private key not found")
	}

	err := cmdFlip([]string{"7", "--repo", privateCIRepo})
	if err == nil {
		t.Fatal("no error from a flip with no App token")
	}
	msg := err.Error()
	wantRole, ok := deskkit.TokenRoleForLoop(flipRole)
	if !ok {
		t.Fatalf("loop %s carries no App role in the shared table", flipRole)
	}
	for _, want := range []string{condAppToken, wantRole, tokenPath} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal does not name %q: %s", want, msg)
		}
	}
}

// The backstop BELOW the condition: even reached directly, a forge call with no minted token
// does not happen. This is what makes "never the ambient credential" a property of the code
// rather than of the current call order — and it lives one layer deeper than the old runCmd
// guard did, in the backend that constructs the client.
func TestForgeCallRefusedWithNoMintedToken(t *testing.T) {
	var served int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	fg := &deskkit.GitHubForge{Token: "", BaseURL: srv.URL, Client: srv.Client()}
	if _, err := fg.GetPullRequest(deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}, 7); err == nil {
		t.Fatal("the backend reached the forge with no minted token")
	}
	if served != 0 {
		t.Fatalf("the refusal still issued %d request(s) — it must refuse BEFORE the wire", served)
	}
}

// And the positive half: when a token IS minted, every request the verb makes carries it. This
// is the assertion that would have caught the original bug, where the value was resolved and
// then never actually used to authenticate.
func TestMintedTokenAuthenticatesEveryRequest(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)

	if rc := run([]string{"7", "--repo", privateCIRepo}); rc != deskkit.ExitOK {
		t.Fatalf("flip rc = %d, want 0", rc)
	}
	if len(s.requests) == 0 {
		t.Fatal("no requests were recorded, so this assertion measured nothing")
	}
	for _, r := range s.requests {
		if !strings.Contains(r.Auth, stubToken) {
			t.Fatalf("%s carried Authorization %q — the minted token did not authenticate it, so the "+
				"read would fall through to whatever identity the transport defaults to", r, r.Auth)
		}
	}
}

// $DESK_LOOP unset means a STOP.<loop> flag a human is holding can never match this session, so
// the verb refuses before it does anything at all.
func TestDeskLoopUnsetRefusesBeforeAnyWork(t *testing.T) {
	s := newStub()
	s.install(t)
	s.reviews = approvalAtHead(t, headSHA)
	t.Setenv("DESK_LOOP", "")

	rc := run([]string{"7", "--repo", privateCIRepo})
	if rc != deskkit.ExitRefused {
		t.Fatalf("rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	if len(s.requests) != 0 {
		t.Fatalf("a session with no loop identity still made %d forge call(s): %v", len(s.requests), s.requests)
	}
	if m := s.mutated(); len(m) != 0 {
		t.Fatalf("a session with no loop identity still mutated: %v", m)
	}
}
