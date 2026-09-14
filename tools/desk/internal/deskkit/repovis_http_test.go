package deskkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// HTTP-layer coverage for HTTPRepoInfoFetcher — the production fetcher the gate runs on.
//
// Closing a stated "could not check" from the #310 review: every previous test drove the
// gate through a stub RepoInfoFetcher or an in-process command fake, so the real
// request-building, status handling and JSON decoding had never been executed. The cases
// that matter are the UNHAPPY ones, because the gate's whole claim is that an unreadable
// answer refuses rather than passes — a decode path that returned a zero value on garbage
// would hand PublicRepoGate an empty visibility and, before the empty-string guard, a
// clean pass-through.
//
// httptest.Server, so this is hermetic and runs in ordinary CI.

func fetcherAgainst(t *testing.T, h http.HandlerFunc) *HTTPRepoInfoFetcher {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &HTTPRepoInfoFetcher{Token: "t", BaseURL: srv.URL, Client: srv.Client()}
}

func TestHTTPRepoVisibilityHappyPath(t *testing.T) {
	var gotPath, gotAuth, gotAPIVersion string
	f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotAPIVersion = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-GitHub-Api-Version")
		w.Write([]byte(`{"visibility":"public","private":false}`))
	})
	v, err := f.RepoVisibility("medici-finance", "assay")
	if err != nil {
		t.Fatalf("RepoVisibility: %v", err)
	}
	if v != "public" {
		t.Fatalf("visibility = %q, want public", v)
	}
	if gotPath != "/repos/medici-finance/assay" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "token t" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotAPIVersion == "" {
		t.Fatal("X-GitHub-Api-Version header not sent")
	}
}

func TestHTTPRepoVisibilityFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"not_found", http.StatusNotFound, `{"message":"Not Found"}`},
		{"server_error", http.StatusInternalServerError, `{"message":"boom"}`},
		{"rate_limited", http.StatusForbidden, `{"message":"API rate limit exceeded"}`},
		{"unauthorized", http.StatusUnauthorized, `{"message":"Bad credentials"}`},
		// 200 responses that carry nothing usable. Each must be an ERROR, not "".
		{"truncated_body", http.StatusOK, `{"visibility":"pub`},
		{"array_instead_of_object", http.StatusOK, `[{"visibility":"public"}]`},
		{"visibility_field_absent", http.StatusOK, `{"private":false}`},
		{"visibility_empty_string", http.StatusOK, `{"visibility":""}`},
		{"empty_body", http.StatusOK, ``},
		{"html_error_page", http.StatusOK, `<html><body>nope</body></html>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			})
			v, err := f.RepoVisibility("example-org", "example-k8s")
			if err == nil {
				t.Fatalf("RepoVisibility returned (%q, nil) — an unreadable answer must be an "+
					"error, never a value the gate can act on", v)
			}
			if v != "" {
				t.Fatalf("RepoVisibility returned a value %q alongside an error", v)
			}
		})
	}
}

// TestHTTPRepoVisibilityErrorReachesTheGateAsExit6 is the composition test: the fetcher's
// error must surface through PublicRepoGate as Unverifiable (exit 6), never as a
// pass-through. Without this the two halves could each be right and the seam still wrong.
func TestHTTPRepoVisibilityErrorReachesTheGateAsExit6(t *testing.T) {
	f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	err := PublicRepoGate(f, "example-org", "example-k8s")
	if err == nil {
		t.Fatal("gate passed with an unreadable visibility — must fail closed")
	}
	if !IsUnverifiable(err) {
		t.Fatalf("gate error is %T (%v), want Unverifiable/exit 6", err, err)
	}
}

// TestHTTPGateEndToEndOverHTTP drives the whole gate against an httptest server: a real
// request/response cycle for the live-visibility read, then the repository-scoped
// authorization decision. gateRoster lists example-org/pubrepo as :public and
// example-org/privrepo as :private.
func TestHTTPGateEndToEndOverHTTP(t *testing.T) {
	installRoster(t, gateRoster)

	visibility := `public`
	f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"visibility":"` + visibility + `"}`))
	})

	// Listed :public repo, live public → pass.
	if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
		t.Fatalf("listed :public repo, live public over HTTP: got %v, want pass", err)
	}

	// Listed :public repo, live internal → pass.
	visibility = "internal"
	if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
		t.Fatalf("listed :public repo, live internal over HTTP: got %v, want pass", err)
	}

	// A repo NOT listed :public, live public → refused.
	visibility = "public"
	if err := PublicRepoGate(f, "example-org", "not-listed-anywhere"); !IsRefused(err) {
		t.Fatalf("unlisted live-public repo over HTTP: got %v, want Refused/exit 5", err)
	}

	// Roster drift: configured :private, live public → refused (the live read refuses on the
	// stale roster claim).
	if err := PublicRepoGate(f, "example-org", "privrepo"); !IsRefused(err) {
		t.Fatalf("configured :private but live public over HTTP: got %v, want Refused/exit 5", err)
	}

	// Live private → pass, whatever the configured entry.
	visibility = "private"
	if err := PublicRepoGate(f, "example-org", "pubrepo"); err != nil {
		t.Fatalf("live private over HTTP: got %v, want pass", err)
	}
}
