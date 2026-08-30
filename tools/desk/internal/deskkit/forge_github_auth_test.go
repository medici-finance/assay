package deskkit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// forge_github_auth_test.go — the auth-binding and error-tier guards for the go-gh-backed
// GitHub Forge. These are the second layer under the golden corpus: the
// corpus proves the transport swap changed no observable REQUEST, and these prove the two
// properties the corpus cannot — that the backend authenticates FROM the injected token and
// REFUSES rather than falling back to an ambient identity when it is unset, and that a
// permission/credential/visibility failure surfaces as could-not-check (a non-nil error),
// never as a clean empty result.

// authCapture is an httptest server that records the Authorization header of the last request
// it served and counts how many requests it saw. A test points a GitHubForge at it.
type authCapture struct {
	srv      *httptest.Server
	gotAuth  string
	hitCount int
}

func newAuthCapture(t *testing.T, status int) *authCapture {
	t.Helper()
	a := &authCapture{}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.hitCount++
		a.gotAuth = r.Header.Get("Authorization")
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		// A minimal, valid issue payload so the 200 path decodes cleanly.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"number":1,"state":"open","user":{"login":"x","id":1}}`))
	}))
	t.Cleanup(a.srv.Close)
	return a
}

// TestForgeGithubAuth proves the injected token is the authenticating identity, and that an
// unset token is refused rather than silently resolved to an ambient gh-CLI identity.
func TestForgeGithubAuth(t *testing.T) {
	// Low-entropy placeholder — NOT a real credential.
	const injected = "test-injected-token-0000"

	t.Run("authenticates_from_injected_token", func(t *testing.T) {
		a := newAuthCapture(t, http.StatusOK)
		f := &GitHubForge{Token: injected, BaseURL: a.srv.URL, Client: a.srv.Client()}
		if _, err := f.GetIssue(forgeTestRepo, 1); err != nil {
			t.Fatalf("GetIssue with a valid token: unexpected error %v", err)
		}
		if a.hitCount != 1 {
			t.Fatalf("expected exactly one request to reach the server, got %d", a.hitCount)
		}
		want := "token " + injected
		if a.gotAuth != want {
			t.Fatalf("backend did not authenticate from the injected token\n got Authorization: %q\nwant: %q", a.gotAuth, want)
		}
	})

	t.Run("refuses_unset_token_no_ambient_fallback", func(t *testing.T) {
		a := newAuthCapture(t, http.StatusOK)
		f := &GitHubForge{Token: "", BaseURL: a.srv.URL, Client: a.srv.Client()}
		_, err := f.GetIssue(forgeTestRepo, 1)
		if err == nil {
			t.Fatalf("GetIssue with an unset token must refuse, not fall back to an ambient identity")
		}
		// The refusal is a fail-closed could-not-check (exit 6), not a network attempt.
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Fatalf("unset-token refusal should be ExitUnverifiable (%d), got %d (%v)", ExitUnverifiable, code, err)
		}
		if a.hitCount != 0 {
			t.Fatalf("an unset token must never reach the network (no ambient fallback), but the server saw %d request(s)", a.hitCount)
		}
	})
}

// TestForgeGithubTierErrors proves the error TIERS are surfaced as could-not-check, distinct
// from a clean empty result: a 403 (permission) — the tier the brief names — plus 401
// (credential) and 404 (visibility) each come back as a non-nil *ForgeAPIError carrying the
// status, mapping to ExitUnverifiable, with a nil result. A backend that swallowed a non-2xx
// into an empty-but-nil-error result would fail this.
func TestForgeGithubTierErrors(t *testing.T) {
	const tok = "test-injected-token-0000"
	cases := []struct {
		name       string
		status     int
		wantNotFnd bool
	}{
		{"forbidden_403_permission", http.StatusForbidden, false},
		{"unauthorized_401_credential", http.StatusUnauthorized, false},
		{"not_found_404_visibility", http.StatusNotFound, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a := newAuthCapture(t, tc.status)
			f := &GitHubForge{Token: tok, BaseURL: a.srv.URL, Client: a.srv.Client()}

			pr, err := f.GetPullRequest(forgeTestRepo, 7)
			if err == nil {
				t.Fatalf("HTTP %d must surface an error (could-not-check), not a clean result", tc.status)
			}
			if pr != nil {
				t.Fatalf("HTTP %d must yield a nil result distinct from an empty PullRequest, got %+v", tc.status, pr)
			}
			var fae *ForgeAPIError
			if !errors.As(err, &fae) {
				t.Fatalf("HTTP %d should surface a *ForgeAPIError, got %T (%v)", tc.status, err, err)
			}
			if fae.Status != tc.status {
				t.Fatalf("ForgeAPIError.Status = %d, want %d", fae.Status, tc.status)
			}
			if code := ExitCodeOf(err); code != ExitUnverifiable {
				t.Fatalf("HTTP %d should map to ExitUnverifiable (%d), got %d", tc.status, ExitUnverifiable, code)
			}
			if got := IsForgeNotFound(err); got != tc.wantNotFnd {
				t.Fatalf("IsForgeNotFound = %v, want %v (only 404 licenses a not-found re-resolution)", got, tc.wantNotFnd)
			}
		})
	}
}
