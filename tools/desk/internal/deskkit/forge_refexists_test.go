package deskkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// RefExists must tell "the ref is gone" (false, nil) apart from "I could not look" (error) — the
// distinction the model-capability floor's stamp age-out stands on. Only a clean 404 is absent;
// a 403 from a token that cannot see refs, or a 500 from a bad minute at the forge, is
// could-not-check, never a release. This is the logic that used to live in cmd/deskpost's
// refExists, now pinned at the backend where it moved. The golden corpus pins the present/absent/
// namespace-escape SHAPES; this pins the ERROR tiers those goldens do not drive.
func TestGitHubForgeRefExistsSeparatesAbsentFromCouldNotLook(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		wantPresent bool
		wantErr     bool
	}{
		{"present", http.StatusOK, true, false},
		{"absent", http.StatusNotFound, false, false},
		{"forbidden is could-not-look", http.StatusForbidden, false, true},
		{"server error is could-not-look", http.StatusInternalServerError, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(c.status)
				if c.status == http.StatusOK {
					_, _ = w.Write([]byte(`{"ref":"refs/heads/dispatch/item--01"}`))
				}
			}))
			defer srv.Close()
			f := &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}

			present, err := f.RefExists(forgeTestRepo, "heads/dispatch/item--01")
			if present != c.wantPresent || (err != nil) != c.wantErr {
				t.Fatalf("RefExists = (%v, %v), want (present=%v, err=%v)", present, err, c.wantPresent, c.wantErr)
			}
			// SINGULAR git/ref — the single-reference read, not the plural git/refs delete path.
			if want := "/repos/medici-finance/assay/git/ref/heads/dispatch/item--01"; gotPath != want {
				t.Errorf("read %s, want %s", gotPath, want)
			}
		})
	}
}

// A ref path that is not a ref path never reaches a URL — the same ValidateRefPath bound
// DeleteRef carries, so RefExists cannot be an arbitrary-endpoint reach in a typed coat.
func TestGitHubForgeRefExistsRefusesANonRefPath(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	f := &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}

	if _, err := f.RefExists(forgeTestRepo, "heads/../../branches/main/protection"); err == nil {
		t.Fatal("a traversing ref path was accepted")
	}
	if reached {
		t.Fatal("the refused path still reached the server — validation must happen before the request")
	}
}

// The GitLab backend answers the ref-existence read only for the heads/ namespace (Branches
// API); a 403 there is could-not-check, and a ref OUTSIDE heads/ is a REFUSAL with no request —
// never a guessed "absent" that would report a held claim as released.
func TestGitLabForgeRefExistsErrorTiers(t *testing.T) {
	t.Run("forbidden is could-not-look, not absent", func(t *testing.T) {
		s := newGLServer(t)
		s.forceStatus["/repository/branches/dispatch%2Fitem--01"] = http.StatusForbidden
		present, err := s.forge().RefExists(glRepo, "heads/dispatch/item--01")
		if err == nil {
			t.Fatal("a 403 on the Branches read must be could-not-check, got no error")
		}
		if present {
			t.Fatal("a could-not-check must not report the ref present")
		}
	})

	t.Run("a ref outside heads/ is a refusal with no request", func(t *testing.T) {
		s := newGLServer(t)
		present, err := s.forge().RefExists(glRepo, "tags/v1.2.3")
		if err == nil {
			t.Fatal("a non-heads ref must be a could-not-check refusal on GitLab, got no error")
		}
		if present {
			t.Fatal("a refused namespace must not report the ref present")
		}
		if len(s.requests) != 0 {
			t.Fatalf("a refused namespace emitted %d request(s); it must refuse before any request", len(s.requests))
		}
	})
}
