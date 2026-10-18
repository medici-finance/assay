package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// The fake GitLab every test in this package talks to: an httptest server on loopback that
// records each request (method, API path, body, PRIVATE-TOKEN header) and answers from a
// per-test responder, the shared account answers, then a 500 naming the request as unstubbed.
// It also serves the public role icons under /assets, so the default-avatar path never leaves
// the machine. Nothing here reaches a real network.

// Fixture values. Never a real group, prefix or credential.
const (
	fakeGroup      = "example"
	fakePrefix     = "exorg"
	fakeProject    = "proj"
	fakeOwnerToken = "TOKEN-PLACEHOLDER-owner"
	fakeMinted     = "TOKEN-PLACEHOLDER-minted"
)

type fakeRequest struct {
	Method, Path, Body, Token string
}

func (r fakeRequest) String() string { return r.Method + " " + r.Path }

// responder answers one API request: ok=false passes it on to the shared answers.
type responder func(f *fakeGitLab, method, path, body string) (status int, resp string, ok bool)

type fakeGitLab struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	reqs     []fakeRequest
	icons    int
	counters map[string]int

	// plan is the group answer's "plan" field fragment, e.g. `,"plan":"free"`; "" omits it.
	plan string
	// respond is the per-test responder, consulted first.
	respond responder
	// iconFail makes every icon fetch answer 404.
	iconFail bool
	// dropPath makes every API request whose path contains it a transport failure (the
	// connection is closed with no answer).
	dropPath string
}

func newFakeGitLab(t *testing.T) *fakeGitLab {
	t.Helper()
	f := &fakeGitLab{t: t, counters: map[string]int{}, plan: `,"plan":"free"`}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// URL is the instance base URL, passed as --gitlab-url.
func (f *fakeGitLab) URL() string { return f.srv.URL }

// bump returns 1, 2, 3 … on successive calls for name.
func (f *fakeGitLab) bump(name string) int {
	f.counters[name]++
	return f.counters[name]
}

func (f *fakeGitLab) requests() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.reqs...)
}

// calls is every request the server received, API and icon alike.
func (f *fakeGitLab) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs) + f.icons
}

// count is the number of API requests whose "METHOD /path" starts with prefix.
func (f *fakeGitLab) count(prefix string) int {
	n := 0
	for _, r := range f.requests() {
		if strings.HasPrefix(r.String(), prefix) {
			n++
		}
	}
	return n
}

// bodies is every body sent with "METHOD /path" exactly.
func (f *fakeGitLab) bodies(method, path string) []string {
	var out []string
	for _, r := range f.requests() {
		if r.Method == method && r.Path == path {
			out = append(out, r.Body)
		}
	}
	return out
}

func (f *fakeGitLab) serve(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	p := r.URL.EscapedPath()
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.HasPrefix(p, "/assets/") {
		f.icons++
		if r.Header.Get("PRIVATE-TOKEN") != "" {
			f.t.Errorf("an icon fetch carried a PRIVATE-TOKEN header")
		}
		if f.iconFail {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("PNGFAKE"))
		return
	}
	api := strings.TrimPrefix(p, "/api/v4")
	body := string(b)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		body = "multipart"
	}
	f.reqs = append(f.reqs, fakeRequest{r.Method, api, body, r.Header.Get("PRIVATE-TOKEN")})
	if f.dropPath != "" && strings.Contains(api, f.dropPath) {
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				_ = c.Close()
				return
			}
		}
	}
	status, resp := f.answer(r.Method, api, string(b))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(resp))
}

func (f *fakeGitLab) answer(m, p, body string) (int, string) {
	if f.respond != nil {
		if st, resp, ok := f.respond(f, m, p, body); ok {
			return st, resp
		}
	}
	// The shared account answers: accounts, memberships and PAT minting all succeed.
	switch m + " " + p {
	case "GET /groups/" + fakeGroup:
		return 200, `{"id":1` + f.plan + `}`
	case "GET /groups/1/service_accounts":
		return 200, `[]`
	case "POST /groups/1/service_accounts":
		return 201, fmt.Sprintf(`{"id":%d}`, 100+f.bump("sa"))
	case "POST /groups/1/members":
		return 201, `{}`
	case "GET /projects/" + fakeProject:
		return 200, `{"id":7}`
	case "PUT /projects/7":
		return 200, `{}`
	case "GET /projects/7":
		return 200, `{"only_allow_merge_if_pipeline_succeeds":true,"only_allow_merge_if_all_discussions_are_resolved":true}`
	case "GET /projects/7/protected_tags":
		return 200, `[]`
	case "POST /projects/7/protected_tags", "POST /projects/7/labels":
		return 201, `{}`
	case "PUT /user/avatar":
		return 200, `{"avatar_url":"/uploads/-/system/user/avatar/1/x.png"}`
	}
	switch {
	case m == "GET" && strings.HasPrefix(p, "/groups/1/members/"):
		return 404, `{}`
	case m == "POST" && strings.HasPrefix(p, "/groups/1/service_accounts/") && strings.HasSuffix(p, "/personal_access_tokens"):
		return 201, `{"token":"` + fakeMinted + `"}`
	}
	return 500, `{"unstubbed":true}`
}

// mainProtectedAsIntended answers the protected-main read with the intended rule, and approvals
// as enforced — the answers most project cases share.
func mainProtectedAsIntended(f *fakeGitLab, m, p, _ string) (int, string, bool) {
	switch m + " " + p {
	case "GET /projects/7/protected_branches/main":
		return 200, `{"name":"main","push_access_levels":[{"access_level":0}],"merge_access_levels":[{"access_level":40}],"allow_force_push":false}`, true
	case "POST /projects/7/approvals":
		return 201, `{}`, true
	case "GET /projects/7/approvals":
		return 200, `{"merge_requests_author_approval":false,"merge_requests_disable_committers_approval":true,"merge_request_approvers_available":true}`, true
	}
	return 0, "", false
}

// chain consults each responder in turn.
func chain(rs ...responder) responder {
	return func(f *fakeGitLab, m, p, body string) (int, string, bool) {
		for _, r := range rs {
			if st, resp, ok := r(f, m, p, body); ok {
				return st, resp, ok
			}
		}
		return 0, "", false
	}
}

// runOutput is one in-process invocation's result; out is stdout and stderr interleaved, as
// the script's tests read `2>&1`.
type runOutput struct {
	code int
	out  string
}

func (r runOutput) has(s string) bool { return strings.Contains(r.out, s) }

// runAgainst runs `deskgitlab create <args>` in process against f, with GITLAB_TOKEN set to the
// owner placeholder (token="" leaves it unset) and the default icons served by f.
func runAgainst(t *testing.T, f *fakeGitLab, token string, args ...string) runOutput {
	t.Helper()
	prevL, prevI := lookupEnv, iconBase
	lookupEnv = func(k string) (string, bool) {
		if k == "GITLAB_TOKEN" && token != "" {
			return token, true
		}
		return "", false
	}
	iconBase = f.URL() + "/assets"
	t.Cleanup(func() { lookupEnv, iconBase = prevL, prevI })
	var buf bytes.Buffer
	full := append([]string{"create", "--gitlab-url", f.URL()}, args...)
	code := execute(full, &buf, &buf)
	if testing.Verbose() && t.Failed() {
		t.Logf("run %v (exit %d):\n%s", full, code, buf.String())
	}
	return runOutput{code, buf.String()}
}
