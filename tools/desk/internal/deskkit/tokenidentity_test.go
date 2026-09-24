package deskkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// viewerServer answers the one GraphQL `viewer` read with body and status, recording the
// Authorization header it was sent so a test can see the token was bound explicitly.
func viewerServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" || !strings.Contains(string(b), "viewer") {
			t.Errorf("unexpected request %s %s %s", r.Method, r.URL.Path, b)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &auth
}

func TestViewerIdentityReadsLoginAndBotUserID(t *testing.T) {
	srv, auth := viewerServer(t, 200, `{"data":{"viewer":{"login":"example-worker-app[bot]","databaseId":300000006}}}`)
	g := &GitHubForge{Token: "example-token", BaseURL: srv.URL, Client: srv.Client()}
	id, err := g.viewerIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if id.Login != "example-worker-app[bot]" || id.ID != 300000006 {
		t.Fatalf("identity = %+v", id)
	}
	if !strings.Contains(*auth, "example-token") {
		t.Errorf("the probe did not authenticate with the token it was handed (Authorization %q)", *auth)
	}
}

// Every way the read can fail is an error — never an empty identity a caller could compare.
func TestViewerIdentityFailuresAreErrorsNotIdentities(t *testing.T) {
	for _, c := range []struct {
		name, body string
		status     int
	}{
		{"bad credentials", `{"message":"Bad credentials"}`, 401},
		{"graphql error", `{"errors":[{"message":"example failure"}]}`, 200},
		{"no login", `{"data":{"viewer":{"login":"","databaseId":0}}}`, 200},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv, _ := viewerServer(t, c.status, c.body)
			g := &GitHubForge{Token: "example-token", BaseURL: srv.URL, Client: srv.Client()}
			if id, err := g.viewerIdentity(); err == nil {
				t.Fatalf("a failed read returned identity %+v with no error", id)
			}
		})
	}
	if _, err := GitHubTokenIdentity(""); err == nil {
		t.Fatal("an empty token was probed instead of refused — that is the ambient-login fallback")
	}
}

func TestActsAsRoleIsRosterBound(t *testing.T) {
	plantRoster(t, raisedByFixtureRoster)
	for _, c := range []struct {
		name         string
		id           TokenIdentity
		match, bound bool
	}{
		{"the role's own App", TokenIdentity{"example-worker-app[bot]", 300000006}, true, true},
		{"the gh-CLI rendering of it", TokenIdentity{"app/example-worker-app", 300000006}, true, true},
		{"a human login", TokenIdentity{"ada", 2001}, false, true},
		{"another role's App", TokenIdentity{"example-desk-app[bot]", 300000001}, false, true},
		{"the right login under the wrong pinned id", TokenIdentity{"example-worker-app[bot]", 999}, false, true},
		{"the bare slug (a user could squat it)", TokenIdentity{"example-worker-app", 300000006}, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, b := c.id.ActsAsRole("worker")
			if m != c.match || b != c.bound {
				t.Fatalf("ActsAsRole = (%v,%v), want (%v,%v)", m, b, c.match, c.bound)
			}
		})
	}
	if m, b := (TokenIdentity{"example-worker-app[bot]", 300000006}).ActsAsRole("no-such-role"); m || b {
		t.Fatalf("an unbound role answered (%v,%v); want (false,false) — nothing to check against", m, b)
	}
}
