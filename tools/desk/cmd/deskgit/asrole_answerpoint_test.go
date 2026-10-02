package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// asrole_answerpoint_test.go — sec-1587-S1, round 3: the credential must be bound where it is
// ANSWERED, not only where destinations are listed.
//
// The destination gate (asrole_destination_test.go) checks the list `git remote get-url` prints.
// Git also connects to hosts that are on no such list: a submodule's own remote when push
// recursion is on in any config scope, and an http.proxy that asks the credential machinery for
// its password. An answer channel that replies to ANY prompt hands the GitHub App token to each
// of them. Every test here serves those hosts from a LOOPBACK recorder that never tunnels, and
// asserts on the credential the recorder actually RECEIVED, not only on the exit code.
//
// The token resolver is the fixture seam (asWorker), which ignores the destination list — so
// these tests also show the lower layer holding with the upper destination gate's host binding
// bypassed.

// credRecorder is a loopback HTTP server that answers every request with an auth challenge and
// records each Authorization / Proxy-Authorization header it is sent.
type credRecorder struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests int
	creds    []string // decoded Basic credentials, "user:password"
}

func newCredRecorder(t *testing.T, proxy bool) *credRecorder {
	t.Helper()
	r := &credRecorder{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.requests++
		for _, h := range []string{"Authorization", "Proxy-Authorization"} {
			for _, v := range req.Header.Values(h) {
				if enc, ok := strings.CutPrefix(v, "Basic "); ok {
					if dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(enc)); err == nil {
						r.creds = append(r.creds, string(dec))
						continue
					}
				}
				r.creds = append(r.creds, v)
			}
		}
		r.mu.Unlock()
		if proxy {
			w.Header().Set("Proxy-Authenticate", `Basic realm="loopback-proxy"`)
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="loopback"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

// hostPort is the recorder's 127.0.0.1:<port>.
func (r *credRecorder) hostPort() string { return strings.TrimPrefix(r.srv.URL, "http://") }

// assertNoToken fails when the recorder was ever sent the fixture token, in any header.
func (r *credRecorder) assertNoToken(t *testing.T, what string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.creds {
		if strings.Contains(c, fixtureToken) {
			t.Fatalf("%s: the loopback host received the GitHub App token (credential %q)", what, c)
		}
	}
}

func (r *credRecorder) requestCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests
}

// (1) Submodule push recursion. The superproject's origin passes every gate (a configured local
// root, so nothing leaves the machine); the submodule's own remote is the loopback recorder. With
// push recursion switched on in the repo or global scope, `git push` would also push the
// submodule to that remote, and the child inherits the credential channel. The push must neither
// recurse (no request reaches the recorder) nor, therefore, offer the token to it.
func TestPushAs_SubmoduleRecursion_NoCredentialToSubmoduleRemote(t *testing.T) {
	cases := []struct {
		name         string
		repoConfig   [][]string
		globalConfig string
	}{
		{name: "repo-push.recurseSubmodules-on-demand",
			repoConfig: [][]string{{"push.recurseSubmodules", "on-demand"}}},
		{name: "global-push.recurseSubmodules-on-demand",
			globalConfig: "[push]\n\trecurseSubmodules = on-demand\n"},
		{name: "global-submodule.recurse-true",
			globalConfig: "[submodule]\n\trecurse = true\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := newCredRecorder(t, false)
			work := newRepo(t, allowedSlug)
			withEnv(t, work) // fixture HOME before any further git setup
			asWorker(t)
			const branch = "feature-sm"
			onBranch(t, work, branch)

			// A submodule whose source is a local repo, added under the file protocol.
			subSrc := filepath.Join(t.TempDir(), "sub-src")
			mustGit(t, "", "init", "-b", "main", subSrc)
			for _, kv := range [][2]string{{"user.email", "t@e.st"}, {"user.name", "Test"}, {"commit.gpgsign", "false"}} {
				mustGit(t, subSrc, "config", kv[0], kv[1])
			}
			mustGit(t, subSrc, "commit", "--allow-empty", "-m", "sub init")
			mustGit(t, work, "-c", "protocol.file.allow=always", "submodule", "add", subSrc, "sub")
			mustGit(t, work, "commit", "-m", "add sub")

			// An unpushed submodule commit on the same branch name, whose remote is the recorder.
			sub := filepath.Join(work, "sub")
			for _, kv := range [][2]string{{"user.email", "t@e.st"}, {"user.name", "Test"}, {"commit.gpgsign", "false"}} {
				mustGit(t, sub, "config", kv[0], kv[1])
			}
			mustGit(t, sub, "checkout", "-b", branch)
			mustGit(t, sub, "commit", "--allow-empty", "-m", "unpushed sub commit")
			mustGit(t, sub, "remote", "set-url", "origin", rec.srv.URL+"/other-org/sub.git")
			mustGit(t, work, "add", "sub")
			mustGit(t, work, "commit", "-m", "bump sub")

			for _, args := range c.repoConfig {
				mustGit(t, work, append([]string{"config"}, args...)...)
			}
			if c.globalConfig != "" {
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(c.globalConfig), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			var code int
			out := captureStdoutStderr(t, func() { code = run([]string{"push", "--as", "worker"}) })
			rec.assertNoToken(t, c.name)
			if n := rec.requestCount(); n != 0 {
				t.Fatalf("%s: push recursed into the submodule — the loopback submodule remote got %d request(s)\n%s",
					c.name, n, out)
			}
			if code != deskkit.ExitOK {
				t.Fatalf("%s: push --as worker exit = %d, want %d (ok)\n%s", c.name, code, deskkit.ExitOK, out)
			}
		})
	}
}

// (2) The answer point itself. The origin is an http:// URL on the loopback recorder that names
// the gated slug; the fixture resolver hands back a token whatever the host, so this is the lower
// layer with the destination gate's host binding bypassed. Git connects, is challenged, and asks
// for a credential — which must not be the GitHub App token, because the host is not github.com.
func TestAsRole_NonGitHubHost_CredentialNeverAnswered(t *testing.T) {
	for _, verb := range []string{"push", "fetch"} {
		t.Run(verb, func(t *testing.T) {
			rec := newCredRecorder(t, false)
			work := newRepo(t, allowedSlug)
			withEnv(t, work)
			asWorker(t)
			onBranch(t, work, "feature-ap")
			mustGit(t, work, "remote", "set-url", "origin", rec.srv.URL+"/"+allowedSlug+".git")

			var code int
			out := captureStdoutStderr(t, func() { code = run([]string{verb, "--as", "worker"}) })
			rec.assertNoToken(t, verb+" to a loopback origin")
			if rec.requestCount() == 0 {
				t.Fatalf("%s: the loopback origin was never contacted — the test did not exercise the answer point\n%s", verb, out)
			}
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("%s --as worker to a host that gets no credential: exit = %d, want %d\n%s",
					verb, code, deskkit.ExitUnverifiable, out)
			}
		})
	}
}

// (3) A user-bearing proxy. The origin is github.com over https, but http.proxy routes the
// connection through a loopback proxy that answers every request — the CONNECT included — with
// 407 and never tunnels, so nothing reaches github.com. Because the proxy URL names a user, git
// asks the credential machinery for that user's password before it connects; that answer must
// not be the GitHub App token. With nothing answering, git stops at the prompt and never connects
// at all — so the check that the prompt was exercised reads git's own refusal, not the recorder.
func TestAsRole_UserBearingProxy_NeverGetsToken(t *testing.T) {
	for _, verb := range []string{"push", "fetch"} {
		t.Run(verb, func(t *testing.T) {
			rec := newCredRecorder(t, true)
			work := newRepo(t, allowedSlug)
			withEnv(t, work)
			asWorker(t)
			onBranch(t, work, "feature-px")
			mustGit(t, work, "remote", "set-url", "origin", githubOrigin)
			proxyURL := "http://corpuser@" + rec.hostPort()
			mustGit(t, work, "config", "http.proxy", proxyURL)

			var code int
			out := captureStdoutStderr(t, func() { code = run([]string{verb, "--as", "worker"}) })
			rec.assertNoToken(t, verb+" through a user-bearing proxy")
			if rec.requestCount() == 0 && !strings.Contains(out, "Password for '"+proxyURL+"'") {
				t.Fatalf("%s: neither the proxy nor its password prompt was reached — the test did not exercise the proxy credential\n%s", verb, out)
			}
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("%s --as worker through a 407 proxy: exit = %d, want %d\n%s", verb, code, deskkit.ExitUnverifiable, out)
			}
		})
	}
}
