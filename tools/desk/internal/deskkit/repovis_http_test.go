package deskkit

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	err := PublicRepoGate(f, "example-org", "example-k8s", 1)
	if err == nil {
		t.Fatal("gate passed with an unreadable visibility — must fail closed")
	}
	if !IsUnverifiable(err) {
		t.Fatalf("gate error is %T (%v), want Unverifiable/exit 6", err, err)
	}
}

func TestHTTPIssueReactionsFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"forbidden_rate_limit", http.StatusForbidden, `{"message":"API rate limit exceeded"}`},
		{"server_error", http.StatusInternalServerError, `{}`},
		// A 200 carrying a NON-ARRAY payload. GitHub returns an object here on some
		// error shapes, and decoding it into []Reaction must error rather than yield an
		// empty slice — an empty slice reads as "no +1 present", which is a REFUSAL and
		// therefore still safe, but it would report the wrong reason to a human.
		{"object_not_array", http.StatusOK, `{"message":"Not Found"}`},
		{"truncated_array", http.StatusOK, `[{"content":"+1"`},
		{"html_error_page", http.StatusOK, `<html>nope</html>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.body))
			})
			if _, err := f.IssueReactions("example-org", "example-k8s", 7); err == nil {
				t.Fatal("IssueReactions returned nil error on an unusable response")
			}
		})
	}
}

// TestHTTPIssueReactionsParsesTheNumericID pins that the id the strict blessing-authority
// check depends on actually survives the JSON decode. The `id` tag was added late;
// a struct-tag typo would silently zero it, and IsBlessAuthorityIDStrict would then
// refuse a genuine blessing-authority +1 — a fail-closed break, but a total outage of the
// gate's only success path, which no stub-driven test would catch.
func TestHTTPIssueReactionsParsesTheNumericID(t *testing.T) {
	var gotPath, gotAccept string
	f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAccept = r.URL.Path, r.Header.Get("Accept")
		w.Write([]byte(`[{"content":"+1","user":{"login":"ada","type":"User","id":2001}}]`))
	})
	rs, err := f.IssueReactions("example-org", "example-k8s", 7)
	if err != nil {
		t.Fatalf("IssueReactions: %v", err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d reactions, want 1", len(rs))
	}
	if rs[0].User.ID != fixtureBlessID {
		t.Fatalf("user id decoded as %d, want %d — the json:\"id\" tag is not doing its job",
			rs[0].User.ID, fixtureBlessID)
	}
	if gotPath != "/repos/example-org/example-k8s/issues/7/reactions" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(gotAccept, "squirrel-girl") {
		t.Fatalf("Accept header = %q — the reactions API needs the squirrel-girl preview", gotAccept)
	}
}

// TestHTTPGateEndToEndOverHTTP drives the whole gate against an httptest server: a real
// request/response cycle for BOTH calls, refusing without a qualifying +1 and passing
// with one. This is the closest hermetic analogue of the production path.
func TestHTTPGateEndToEndOverHTTP(t *testing.T) {
	reactions := `[]`
	f := fetcherAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/reactions") {
			w.Write([]byte(reactions))
			return
		}
		w.Write([]byte(`{"visibility":"public"}`))
	})

	if err := PublicRepoGate(f, "example-org", "example-k8s", 7); !IsRefused(err) {
		t.Fatalf("public repo with no reactions: got %v, want Refused/exit 5", err)
	}

	reactions = `[{"content":"eyes","user":{"login":"ada","type":"User","id":2001}}]`
	if err := PublicRepoGate(f, "example-org", "example-k8s", 7); !IsRefused(err) {
		t.Fatalf("non-+1 reaction from ada: got %v, want Refused/exit 5", err)
	}

	reactions = `[{"content":"+1","user":{"login":"example-org","type":"User","id":12345}}]`
	if err := PublicRepoGate(f, "example-org", "example-k8s", 7); !IsRefused(err) {
		t.Fatalf("+1 from the agent account: got %v, want Refused/exit 5", err)
	}

	reactions = `[{"content":"+1","user":{"login":"ada","type":"User","id":2001}}]`
	if err := PublicRepoGate(f, "example-org", "example-k8s", 7); err != nil {
		t.Fatalf("genuine ada +1 over HTTP was refused: %v — the gate's only success "+
			"path does not work against a real response", err)
	}
}
