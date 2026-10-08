package deskkit

// forge_redirect_test.go — review finding SEC-1 (class credential-forwarded-on-redirect).
//
// GitLab authenticates with PRIVATE-TOKEN, a CUSTOM header. net/http strips Authorization and
// cookies when it follows a redirect to another host, but it forwards every other header — so a
// GitLab client that follows a redirect hands the role's credential to whatever host the
// Location names. A pipeline trigger token travels in the request BODY, which a 307/308 re-sends.
// These tests drive the real backends over the wire against two loopback servers with DIFFERENT
// host names (127.0.0.1 answers the API, localhost is the "other host"), so the redirect is
// cross-host in net/http's own sense, and they assert the other host never receives the
// credential. The GitHub test pins the opposite half: GitHub's run-log redirect to its storage
// host is followed (that is how the archive arrives), and the bearer token does not go with it.
// gitlabcredsites_test.go carries the structural half: every site in the tree that sends a
// GitLab credential is listed against the test here that pins it.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// otherHost records what a redirect target received.
type otherHost struct {
	mu     sync.Mutex
	hits   int
	tokens []string // every credential-bearing value seen: PRIVATE-TOKEN, Authorization, a body
	srv    *httptest.Server
}

// url is the target's base URL spelled with host name `localhost`, so that a redirect from the
// 127.0.0.1 API server is to a DIFFERENT host as net/http compares them.
func (o *otherHost) url() string { return strings.Replace(o.srv.URL, "127.0.0.1", "localhost", 1) }

func (o *otherHost) seen() (int, []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.hits, append([]string(nil), o.tokens...)
}

func newOtherHost(t *testing.T, body string) *otherHost {
	t.Helper()
	o := &otherHost{}
	o.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		o.mu.Lock()
		o.hits++
		for _, v := range []string{r.Header.Get("PRIVATE-TOKEN"), r.Header.Get("Authorization")} {
			if v != "" {
				o.tokens = append(o.tokens, v)
			}
		}
		if strings.Contains(string(b), "placeholder-trigger") {
			o.tokens = append(o.tokens, "body:"+string(b))
		}
		o.mu.Unlock()
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(o.srv.Close)
	return o
}

// redirectingGitLab answers every API request with a redirect of the given status to the same
// path on `to`, and counts the requests that reached it.
func redirectingGitLab(t *testing.T, status int, to *otherHost) (*httptest.Server, *int) {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		// The run-log path needs one job before it reads a trace: answer the job list, redirect
		// everything else.
		if glStubJobs.MatchString(r.URL.EscapedPath()) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":7,"name":"build","status":"failed"}]`)
			return
		}
		http.Redirect(w, r, to.url()+r.URL.RequestURI(), status)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func wantRedirectRefused(t *testing.T, what string, err error, to *otherHost) {
	t.Helper()
	hits, toks := to.seen()
	if len(toks) > 0 {
		t.Fatalf("%s: the credential reached the redirect target (%d request(s)): %q", what, hits, toks)
	}
	if hits > 0 {
		t.Fatalf("%s: the client followed the redirect to another host (%d request(s) reached it)", what, hits)
	}
	if err == nil {
		t.Fatalf("%s: a refused redirect returned no error — it must be a could-not-check", what)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("%s: exit %d (%v), want could-not-check (%d)", what, ExitCodeOf(err), err, ExitUnverifiable)
	}
	if !strings.Contains(err.Error(), "could-not-check") {
		t.Fatalf("%s: error does not say could-not-check: %v", what, err)
	}
}

// TestGitLabRunLogRedirectKeepsToken is SEC-1's reported instance: the per-job trace read.
func TestGitLabRunLogRedirectKeepsToken(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect} {
		other := newOtherHost(t, "a log served by another host\n")
		api, _ := redirectingGitLab(t, status, other)
		g := &GitLabForge{Token: "example-placeholder-worker-0000", BaseURL: api.URL, Client: api.Client()}
		parts, err := g.RunLog(rlRepo, RunRef{ID: "501"})
		wantRedirectRefused(t, "GitLab RunLog", err, other)
		if parts != nil {
			t.Fatalf("GitLab RunLog returned parts %v from a refused redirect", parts)
		}
	}
}

// TestGitLabClientRedirectKeepsToken is the class over the backend's two clients: a read through
// the access-token client (RunStatus) and the trigger call (RunWorkflow, whose token is in the
// body, so a 307 would re-send it).
func TestGitLabClientRedirectKeepsToken(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprintf("status/%d", status), func(t *testing.T) {
			other := newOtherHost(t, `{"id":1,"status":"success"}`)
			api, _ := redirectingGitLab(t, status, other)
			g := &GitLabForge{Token: "example-placeholder-trigger-0000", BaseURL: api.URL, Client: api.Client()}
			_, err := g.RunStatus(rlRepo, RunRef{ID: "501"})
			wantRedirectRefused(t, "GitLab RunStatus", err, other)
		})
		t.Run(fmt.Sprintf("trigger/%d", status), func(t *testing.T) {
			other := newOtherHost(t, `{"id":1,"web_url":"x"}`)
			api, _ := redirectingGitLab(t, status, other)
			g := &GitLabForge{Token: "example-placeholder-trigger-0000", BaseURL: api.URL, Client: api.Client()}
			_, err := g.RunWorkflow(rlRepo, RunWorkflowInput{Ref: "main"})
			wantRedirectRefused(t, "GitLab RunWorkflow (trigger)", err, other)
		})
	}
}

// TestGitLabNilClientRefusesRedirect — production leaves Client nil. The refusal must not depend on
// the caller injecting a client.
func TestGitLabNilClientRefusesRedirect(t *testing.T) {
	other := newOtherHost(t, `{"id":1,"status":"success"}`)
	api, _ := redirectingGitLab(t, http.StatusFound, other)
	g := &GitLabForge{Token: "example-placeholder-worker-0000", BaseURL: api.URL}
	_, err := g.RunStatus(rlRepo, RunRef{ID: "501"})
	wantRedirectRefused(t, "GitLab RunStatus (nil Client)", err, other)
}

// TestGitHubRunLogRedirectDropsToken pins the GitHub half, which is a property of the go-gh
// transport this backend builds on: the logs route's redirect to another host IS followed (the
// archive arrives and is read), and the bearer token does not travel with it.
func TestGitHubRunLogRedirectDropsToken(t *testing.T) {
	archive := ghJobsArchive(t, 1)
	other := &otherHost{}
	other.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other.mu.Lock()
		other.hits++
		if v := r.Header.Get("Authorization"); v != "" {
			other.tokens = append(other.tokens, v)
		}
		other.mu.Unlock()
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive)
	}))
	t.Cleanup(other.srv.Close)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/actions/runs/501/logs") {
			if r.Header.Get("Authorization") == "" {
				t.Errorf("the API host itself received no bearer token")
			}
			http.Redirect(w, r, other.url()+"/_run_archive/501.zip", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(api.Close)
	g := &GitHubForge{Token: "test-token", BaseURL: api.URL, Client: api.Client()}
	parts, err := g.RunLog(rlRepo, RunRef{ID: "501"})
	if err != nil {
		t.Fatalf("GitHub RunLog through a cross-host redirect: %v", err)
	}
	if len(parts) == 0 {
		t.Fatal("GitHub RunLog returned no parts")
	}
	hits, toks := other.seen()
	if hits == 0 {
		t.Fatal("the archive host was never reached — this test did not exercise the redirect")
	}
	if len(toks) > 0 {
		t.Fatalf("the bearer token reached the archive host: %q", toks)
	}
}

// TestGitLabAccountFetcherInjectedClientRefusesRedirect — the liveness fetcher refused redirects
// only on its nil-Client default; outboundforge.go hands it the binding's Client, and an injected
// client followed redirects with PRIVATE-TOKEN on. The refusal must hold whatever client is given.
func TestGitLabAccountFetcherInjectedClientRefusesRedirect(t *testing.T) {
	other := newOtherHost(t, `[]`)
	api, _ := redirectingGitLab(t, http.StatusFound, other)
	f := &HTTPGitLabAccountFetcher{Token: "example-placeholder-worker-0000", BaseURL: api.URL, Client: api.Client()}
	_, err := f.GetAccount("desk-worker")
	if hits, toks := other.seen(); hits > 0 {
		t.Fatalf("the liveness read followed the redirect to another host (%d request(s), credentials %q)", hits, toks)
	}
	if err == nil || err == ErrAccountNotFound {
		t.Fatalf("a refused redirect must be a distinct error, never nil or not-found: %v", err)
	}
}
