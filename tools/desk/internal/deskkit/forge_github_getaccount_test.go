package deskkit

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// forge_github_getaccount_test.go — hermetic httptest coverage for
// HTTPAccountFetcher.GetAccount, modeled on repovis_http_test.go's fetcherAgainst pattern
// (real request/response cycle, no network).
//
// NOTE ON FILE/TYPE NAMING: the brief that authored this coverage named the production type
// as a *GitHubForge method; trustliveness.go's file-level comment records why that placement
// does not compile clean here (it trips forge_surface_test.go's closed-surface invariant)
// and why HTTPAccountFetcher — a standalone struct, same shape as HTTPRepoInfoFetcher — is
// the corrected placement. This test file keeps its originally-briefed name so the Verify
// table's `go test ./internal/deskkit/ -run '^TestGetAccount...'` commands still resolve.

func accountFetcherAgainst(t *testing.T, h http.HandlerFunc) *HTTPAccountFetcher {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &HTTPAccountFetcher{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}
}

// TestGetAccountDeletedIs404 — a login GitHub no longer resolves to any account (404) must
// come back as the canonical ErrAccountNotFound sentinel, never a bare "some error".
func TestGetAccountDeletedIs404(t *testing.T) {
	var gotPath, gotAuth string
	f := accountFetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})

	acct, err := f.GetAccount("ghost-login")
	if acct != nil {
		t.Fatalf("GetAccount returned a non-nil account alongside a 404: %+v", acct)
	}
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("GetAccount err = %v, want ErrAccountNotFound", err)
	}
	if gotPath != "/users/ghost-login" {
		t.Fatalf("path = %q, want /users/ghost-login", gotPath)
	}
	if gotAuth != "token test-token" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

// TestGetAccountRenameKeepsIDMatchesDifferentLogin — the same account (same id) answering
// under a login GitHub reports differently from what was requested: a rename, not a
// deletion or a reclaim. GetAccount itself does no classification (that is
// trustliveness.go's job) — it just needs to hand back the WIRE id/login it was given.
func TestGetAccountRenameKeepsIDMatchesDifferentLogin(t *testing.T) {
	f := accountFetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":1002,"login":"renamed-human-new-name"}`))
	})

	acct, err := f.GetAccount("renamed-human")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.ID != 1002 || acct.Login != "renamed-human-new-name" {
		t.Fatalf("GetAccount = %+v, want {ID:1002 Login:renamed-human-new-name}", acct)
	}
}

// TestGetAccountReclaimDifferentID — the requested login resolves, but to a DIFFERENT id
// than the caller pinned. GetAccount reports the wire truth; the pinned-id comparison is
// the caller's (trustliveness.go's) job, not this method's.
func TestGetAccountReclaimDifferentID(t *testing.T) {
	f := accountFetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":99999,"login":"squatted-login"}`))
	})

	acct, err := f.GetAccount("squatted-login")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acct.ID != 99999 {
		t.Fatalf("GetAccount.ID = %d, want 99999 (a reclaimed account carries a NEW id)", acct.ID)
	}
}

// TestGetAccountTransportFailureIsNotDeleted — a 500 (or any non-404 failure) must NEVER be
// classified the same as ErrAccountNotFound: the caller's whole "did it not exist, or did
// we just fail to ask?" distinction depends on this method keeping the two apart.
func TestGetAccountTransportFailureIsNotDeleted(t *testing.T) {
	cases := []struct {
		name   string
		status int
	}{
		{"server_error", http.StatusInternalServerError},
		{"forbidden", http.StatusForbidden},
		{"unauthorized", http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := accountFetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
			})

			acct, err := f.GetAccount("some-login")
			if acct != nil {
				t.Fatalf("GetAccount returned a non-nil account alongside a %d: %+v", c.status, acct)
			}
			if err == nil {
				t.Fatal("GetAccount returned nil error on a non-2xx response")
			}
			if errors.Is(err, ErrAccountNotFound) {
				t.Fatalf("status %d misclassified as ErrAccountNotFound — a transport/auth failure must "+
					"stay distinguishable from a genuine 404", c.status)
			}
		})
	}
}

// TestGetAccountUnreadableBodyIsNeverDeleted pins the same "unreadable != deleted" property
// against a 200 that carries no usable body, mirroring repovis_http_test.go's fails-closed
// table for RepoVisibility.
func TestGetAccountUnreadableBodyIsNeverDeleted(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"truncated_body", `{"id":1,"log`},
		{"array_instead_of_object", `[{"id":1,"login":"x"}]`},
		{"login_field_absent", `{"id":1}`},
		{"empty_body", ``},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := accountFetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(c.body))
			})
			acct, err := f.GetAccount("some-login")
			if err == nil {
				t.Fatalf("GetAccount returned (%+v, nil) on an unreadable 200 body — must be an error", acct)
			}
			if errors.Is(err, ErrAccountNotFound) {
				t.Fatal("an unreadable 200 body must never be classified ErrAccountNotFound")
			}
		})
	}
}
