package gittest

import (
	"fmt"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type roleRoundTrip func(*http.Request) (*http.Response, error)

func (f roleRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// RoleHTTP serves real Git advertisements and pack data, requiring a synthetic
// credential. Only the dial is redirected: go-git's real HTTP layer sets BasicAuth.
// Every request is confined to loopback, including unexpected destinations.
type RoleWire struct {
	Requests atomic.Int32
	Broken   atomic.Bool
}

func RoleHTTP(t *testing.T, projectRoot, token string) *RoleWire {
	t.Helper()
	path, err := exec.Command("git", "--exec-path").Output()
	if err != nil {
		t.Fatal(err)
	}
	backend := &cgi.Handler{Path: filepath.Join(strings.TrimSpace(string(path)), "git-http-backend"), Env: []string{"GIT_PROJECT_ROOT=" + projectRoot, "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}}
	count := &RoleWire{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "x-access-token" || pass != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		count.Requests.Add(1)
		if count.Broken.Load() {
			w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			if _, err := fmt.Fprint(w, "001e# service=git-upload-pack\n00000040truncated"); err != nil {
				t.Error("fixture failed to write truncated upload-pack advertisement")
			}
			return
		}
		backend.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	old := client.Protocols["https"]
	rt := roleRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() != "github.com" {
			return nil, fmt.Errorf("fixture refuses non-GitHub request")
		}
		c := r.Clone(r.Context())
		u := *r.URL
		c.URL = &u
		c.URL.Scheme, c.URL.Host = target.Scheme, target.Host
		response, err := http.DefaultTransport.RoundTrip(c)
		if response != nil {
			response.Request = r
		}
		return response, err
	})
	client.InstallProtocol("https", githttp.NewClient(&http.Client{Transport: rt}))
	t.Cleanup(func() { client.InstallProtocol("https", old) })
	return count
}
