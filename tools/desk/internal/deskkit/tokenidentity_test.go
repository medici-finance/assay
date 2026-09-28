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
}

// probeTransport stands in for http.DefaultTransport, which the resolver-built probe backend
// dials through: it records every request, and answers the viewer read as a role App would.
type probeTransport struct{ reqs []*http.Request }

func (p *probeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	p.reqs = append(p.reqs, r)
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":{"viewer":{"login":"example-worker-app[bot]","databaseId":300000006}}}`)),
		Request:    r,
	}, nil
}

func installProbeTransport(t *testing.T) *probeTransport {
	t.Helper()
	p := &probeTransport{}
	old := http.DefaultTransport
	http.DefaultTransport = p
	t.Cleanup(func() { http.DefaultTransport = old })
	return p
}

// Issue 1631, the resolver-built probe: the inherited token goes only where the role's own
// GitHub credential would — the forge must resolve to GitHub, a given origin must be exactly
// github.com — and the request lands on GitHubAPIBase, never a caller-chosen host. Every refusal
// happens before ANY request, so the token is offered to nothing.
func TestGitHubTokenIdentityForRepoIsBoundToTheResolvedGitHubHost(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "example-repo"}

	t.Run("github.com origin: one viewer read at the default API base", func(t *testing.T) {
		withRoster(t, goldenRoster())
		p := installProbeTransport(t)
		id, err := GitHubTokenIdentityForRepo(repo, "git@github.com:example-org/example-repo.git", "example-token")
		if err != nil {
			t.Fatal(err)
		}
		if id.Login != "example-worker-app[bot]" || id.ID != 300000006 {
			t.Fatalf("identity = %+v", id)
		}
		if len(p.reqs) != 1 {
			t.Fatalf("%d request(s), want exactly one viewer read", len(p.reqs))
		}
		r := p.reqs[0]
		if r.URL.Host != "api.github.com" || r.URL.Path != "/graphql" || r.Method != http.MethodPost {
			t.Fatalf("probe went to %s %s, want POST https://api.github.com/graphql", r.Method, r.URL)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "example-token") {
			t.Errorf("the probe did not bind the token it was handed")
		}
	})

	for _, c := range []struct {
		name, origin, forges string
	}{
		{"empty token", "git@github.com:example-org/example-repo.git", ""},
		{"roster maps the repo to gitlab", "git@gitlab.com:example-org/example-repo.git", "example-org/example-repo=gitlab"},
		{"roster says github, origin is another host", "git@github.example.test:example-org/example-repo.git", "example-org/example-repo=github"},
		{"origin maps to no known forge", "https://forge.example.test/example-org/example-repo.git", ""},
		{"origin does not parse", "not a remote", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			roster := goldenRoster()
			if c.forges != "" {
				roster[EnvRepoForges] = c.forges
			}
			withRoster(t, roster)
			p := installProbeTransport(t)
			tok := "example-token"
			if c.name == "empty token" {
				tok = ""
			}
			if id, err := GitHubTokenIdentityForRepo(repo, c.origin, tok); err == nil {
				t.Fatalf("probe answered %+v; want a refusal", id)
			}
			if len(p.reqs) != 0 {
				t.Fatalf("the token was offered to %s before the refusal — it must reach no host", p.reqs[0].URL)
			}
		})
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
