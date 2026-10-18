package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// claimdecay_github_test.go — the in-process GitHub branch read behind dead-claim
// decay (#3372). The board regen runs on a runner with no `gh`; these tests run
// the decay with an EMPTY PATH, so any process start fails, and serve the GraphQL
// API through http.DefaultTransport, which the reader resolves through.

// graphqlCall is one request the fake API saw.
type graphqlCall struct {
	Method, URL, Auth string
	Vars              map[string]any
}

// fakeGraphQL replaces http.DefaultTransport for one test. serve answers each
// request (1-based) with a status and a body.
func fakeGraphQL(t *testing.T, serve func(n int, vars map[string]any) (int, string)) *[]graphqlCall {
	t.Helper()
	var mu sync.Mutex
	calls := &[]graphqlCall{}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &req)
		}
		mu.Lock()
		*calls = append(*calls, graphqlCall{r.Method, r.URL.String(), r.Header.Get("Authorization"), req.Variables})
		n := len(*calls)
		mu.Unlock()
		status, body := serve(n, req.Variables)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)),
			Header: http.Header{"Content-Type": {"application/json"}}, Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	return calls
}

// offlineGitHubEnv empties PATH (no `gh`, no anything), pins the credential and
// the origin remote, and clears the Actions endpoint override.
func offlineGitHubEnv(t *testing.T, token string) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GH_TOKEN", token)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_GRAPHQL_URL", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	prev := remoteOriginURL
	remoteOriginURL = func(string) (string, error) { return "https://github.com/acme/widgets.git", nil }
	t.Cleanup(func() { remoteOriginURL = prev })
}

// branchPage renders one GraphQL page: each branch's tip is "tip-<name>", and
// its one same-repository pull request is in the given state with that tip as
// its head.
func branchPage(hasNext bool, cursor string, states map[string]string) string {
	var nodes []string
	for name, state := range states {
		nodes = append(nodes, `{"name":"`+name+`","target":{"oid":"tip-`+name+`"},`+
			`"associatedPullRequests":{"nodes":[{"number":1,"state":"`+state+`","headRefName":"`+name+`",`+
			`"headRefOid":"tip-`+name+`","isCrossRepository":false,"headRepository":{"name":"widgets"},`+
			`"headRepositoryOwner":{"login":"acme"}}]}}`)
	}
	next := "false"
	if hasNext {
		next = "true"
	}
	return `{"data":{"repository":{"refs":{"pageInfo":{"hasNextPage":` + next + `,"endCursor":"` + cursor + `"},` +
		`"nodes":[` + strings.Join(nodes, ",") + `]}}}}`
}

// TestDecayRunsWithoutGhOnPath is the #3372 regression: with no `gh` anywhere on
// PATH and a token in the environment, decay reads both pages of the branch
// listing over HTTP and drops the closed pull request's branch, keeping the open
// one — and reports no could-not-check.
//
// regression: #3372
func TestDecayRunsWithoutGhOnPath(t *testing.T) {
	offlineGitHubEnv(t, "test-token")
	calls := fakeGraphQL(t, func(n int, _ map[string]any) (int, string) {
		if n == 1 {
			return 200, branchPage(true, "cursor-1", map[string]string{"fix/alpha-01-dead": "CLOSED"})
		}
		return 200, branchPage(false, "cursor-2", map[string]string{"fix/alpha-02-live": "OPEN"})
	})

	var got []string
	var reason string
	stderr := captureStderr(t, func() {
		got, reason = decayDeadClaims("/repo", []string{"fix/alpha-01-dead", "fix/alpha-02-live"})
	})
	if reason != "" {
		t.Fatalf("decay must run without `gh` on PATH; could-not-check reason %q\nstderr:\n%s", reason, stderr)
	}
	if want := []string{"fix/alpha-02-live"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("decayDeadClaims = %v, want %v (closed-PR branch dropped, open kept)", got, want)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 GraphQL requests (two pages), got %d: %+v", len(*calls), *calls)
	}
	for i, c := range *calls {
		if c.Method != http.MethodPost || c.URL != "https://api.github.com/graphql" {
			t.Errorf("request %d = %s %s, want POST https://api.github.com/graphql", i+1, c.Method, c.URL)
		}
		if c.Auth != "Bearer test-token" {
			t.Errorf("request %d Authorization = %q, want the GH_TOKEN bearer", i+1, c.Auth)
		}
		if c.Vars["owner"] != "acme" || c.Vars["name"] != "widgets" {
			t.Errorf("request %d variables = %v, want owner acme, name widgets", i+1, c.Vars)
		}
	}
	if (*calls)[0].Vars["endCursor"] != nil || (*calls)[1].Vars["endCursor"] != "cursor-1" {
		t.Errorf("cursor not threaded: first %v, second %v", (*calls)[0].Vars["endCursor"], (*calls)[1].Vars["endCursor"])
	}
}

// TestDecayNoTokenIsCouldNotCheck: no credential and no `gh` is a named
// could-not-check before any request — never an anonymous call, never a
// silently undecayed board.
//
// regression: #3372
func TestDecayNoTokenIsCouldNotCheck(t *testing.T) {
	offlineGitHubEnv(t, "")
	calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
	branches := []string{"fix/alpha-01-dead"}
	var got []string
	var reason string
	_ = captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("an unread decay must keep every branch; got %v", got)
	}
	if !strings.Contains(reason, "GH_TOKEN") || !strings.Contains(reason, "GITHUB_TOKEN") {
		t.Errorf("reason must name the credential to set; got %q", reason)
	}
	if len(*calls) != 0 {
		t.Errorf("no request may be made without a credential; saw %+v", *calls)
	}
}

// TestDecayHTTP401CouldNotCheck: a refused read keeps every branch and carries
// the status and GitHub's message onto the board's could-not-check.
func TestDecayHTTP401CouldNotCheck(t *testing.T) {
	offlineGitHubEnv(t, "bad-token")
	fakeGraphQL(t, func(int, map[string]any) (int, string) { return 401, `{"message":"Bad credentials"}` })
	branches := []string{"fix/alpha-01-dead"}
	var got []string
	var reason string
	_ = captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })
	if !reflect.DeepEqual(got, branches) {
		t.Fatalf("a refused read must keep every branch; got %v", got)
	}
	if !strings.Contains(reason, "HTTP 401: Bad credentials") || !strings.Contains(reason, "GH_TOKEN") {
		t.Errorf("reason must carry the status, message and credential source; got %q", reason)
	}
}

// TestDecayPagingNeverShort: a listing that cannot be completed — a cursor that
// does not advance, or still paging at the safety bound — is an error, never a
// short listing read as "these are all the branches".
func TestDecayPagingNeverShort(t *testing.T) {
	t.Run("stuck-cursor", func(t *testing.T) {
		offlineGitHubEnv(t, "test-token")
		fakeGraphQL(t, func(int, map[string]any) (int, string) {
			return 200, branchPage(true, "same", map[string]string{"fix/alpha-01-dead": "CLOSED"})
		})
		if _, err := githubBranchPRPages("/repo"); err == nil || !strings.Contains(err.Error(), "no new cursor") {
			t.Fatalf("a cursor that does not advance must be an error; got %v", err)
		}
	})
	t.Run("page-bound", func(t *testing.T) {
		offlineGitHubEnv(t, "test-token")
		prev := githubDecayMaxPages
		githubDecayMaxPages = 2
		t.Cleanup(func() { githubDecayMaxPages = prev })
		fakeGraphQL(t, func(n int, _ map[string]any) (int, string) {
			return 200, branchPage(true, "c"+strings.Repeat("x", n), map[string]string{"fix/alpha-01-dead": "CLOSED"})
		})
		if _, err := githubBranchPRPages("/repo"); err == nil || !strings.Contains(err.Error(), "still paging") {
			t.Fatalf("a walk still paging at the bound must be an error; got %v", err)
		}
	})
	t.Run("graphql-error-page", func(t *testing.T) {
		offlineGitHubEnv(t, "test-token")
		fakeGraphQL(t, func(int, map[string]any) (int, string) {
			return 200, `{"data":{"repository":null},"errors":[{"message":"Resource not accessible by integration"}]}`
		})
		branches := []string{"fix/alpha-01-dead"}
		var got []string
		var reason string
		_ = captureStderr(t, func() { got, reason = decayDeadClaims("/repo", branches) })
		if !reflect.DeepEqual(got, branches) || !strings.Contains(reason, "Resource not accessible by integration") {
			t.Fatalf("a GraphQL error page must be a could-not-check keeping every branch; got %v, %q", got, reason)
		}
	})
}

// TestGitHubDecayTarget pins where the read goes and which repository it asks
// about, for the remote shapes a desk or a runner actually has.
func TestGitHubDecayTarget(t *testing.T) {
	cases := []struct {
		name, origin, graphqlEnv, repoEnv string
		wantURL, wantRepo                 string
	}{
		{"github.com https", "https://github.com/acme/widgets.git", "", "", "https://api.github.com/graphql", "acme/widgets"},
		{"ssh host alias", "git@github-work:acme/widgets.git", "", "", "https://api.github.com/graphql", "acme/widgets"},
		{"ssh over 443", "ssh://git@ssh.github.com:443/acme/widgets.git", "", "", "https://api.github.com/graphql", "acme/widgets"},
		{"enterprise host", "git@github.example.com:acme/widgets.git", "", "", "https://github.example.com/api/graphql", "acme/widgets"},
		{"actions override", "https://github.com/acme/widgets.git", "https://ghe.example/api/graphql", "", "https://ghe.example/api/graphql", "acme/widgets"},
		{"local origin, actions repo", "/srv/mirror/widgets", "", "acme/widgets", "https://api.github.com/graphql", "acme/widgets"},
		{"nothing", "/srv/mirror/widgets", "", "", "https://api.github.com/graphql", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GITHUB_GRAPHQL_URL", c.graphqlEnv)
			t.Setenv("GITHUB_REPOSITORY", c.repoEnv)
			prev := remoteOriginURL
			remoteOriginURL = func(string) (string, error) { return c.origin, nil }
			t.Cleanup(func() { remoteOriginURL = prev })
			if got := githubGraphQLEndpoint("/repo"); got != c.wantURL {
				t.Errorf("endpoint = %q, want %q", got, c.wantURL)
			}
			o, n, err := githubDecayRepo("/repo")
			got := ""
			if err == nil {
				got = o + "/" + n
			}
			if got != c.wantRepo {
				t.Errorf("repo = %q (err %v), want %q", got, err, c.wantRepo)
			}
		})
	}
}

// TestDecayTokenStaysOnItsHost: the github.com credential (GH_TOKEN,
// GITHUB_TOKEN, gh's github.com login) is never sent to a host the endpoint was
// merely GUESSED from — an origin on any host with a "github" label, a lookalike
// included. That host gets only an enterprise credential, and the enterprise
// credential never goes to github.com. An endpoint the operator set explicitly
// (GITHUB_GRAPHQL_URL) is an opt-in and gets the ordinary credential.
//
// regression: SEC-2 on #3384
func TestDecayTokenStaysOnItsHost(t *testing.T) {
	const lookalike = "https://github.lookalike.example/acme/widgets.git"
	setOrigin := func(t *testing.T, origin string) {
		prev := remoteOriginURL
		remoteOriginURL = func(string) (string, error) { return origin, nil }
		t.Cleanup(func() { remoteOriginURL = prev })
	}
	sawToken := func(calls []graphqlCall, token string) bool {
		for _, c := range calls {
			if strings.Contains(c.Auth, token) {
				return true
			}
		}
		return false
	}

	t.Run("lookalike-origin-never-gets-github-token", func(t *testing.T) {
		offlineGitHubEnv(t, "com-token")
		t.Setenv("GITHUB_TOKEN", "com-token-2")
		t.Setenv("GH_ENTERPRISE_TOKEN", "")
		t.Setenv("GITHUB_ENTERPRISE_TOKEN", "")
		setOrigin(t, lookalike)
		calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
		_, err := githubBranchPRPages("/repo")
		if err == nil || !strings.Contains(err.Error(), "GH_ENTERPRISE_TOKEN") || !strings.Contains(err.Error(), "github.lookalike.example") {
			t.Fatalf("a guessed enterprise host with no enterprise credential must be a could-not-check naming it; got %v", err)
		}
		if len(*calls) != 0 || sawToken(*calls, "com-token") {
			t.Fatalf("the github.com credential reached a guessed host: %+v", *calls)
		}
	})

	t.Run("lookalike-origin-gets-enterprise-token-only", func(t *testing.T) {
		offlineGitHubEnv(t, "com-token")
		t.Setenv("GH_ENTERPRISE_TOKEN", "ent-token")
		setOrigin(t, lookalike)
		calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
		if _, err := githubBranchPRPages("/repo"); err != nil {
			t.Fatalf("read with an enterprise credential: %v", err)
		}
		if len(*calls) != 1 || (*calls)[0].URL != "https://github.lookalike.example/api/graphql" || (*calls)[0].Auth != "Bearer ent-token" {
			t.Fatalf("want one POST to the enterprise endpoint with the enterprise bearer; got %+v", *calls)
		}
	})

	t.Run("github-com-never-gets-enterprise-token", func(t *testing.T) {
		offlineGitHubEnv(t, "")
		t.Setenv("GH_ENTERPRISE_TOKEN", "ent-token")
		calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
		if _, err := githubBranchPRPages("/repo"); err == nil || !strings.Contains(err.Error(), "GH_TOKEN") {
			t.Fatalf("github.com with only an enterprise credential must be a could-not-check; got %v", err)
		}
		if len(*calls) != 0 {
			t.Fatalf("the enterprise credential reached github.com: %+v", *calls)
		}
	})

	t.Run("operator-endpoint-is-an-opt-in", func(t *testing.T) {
		offlineGitHubEnv(t, "com-token")
		t.Setenv("GITHUB_GRAPHQL_URL", "https://ghe.example/api/graphql")
		calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
		if _, err := githubBranchPRPages("/repo"); err != nil {
			t.Fatalf("read at the operator's endpoint: %v", err)
		}
		if len(*calls) != 1 || (*calls)[0].URL != "https://ghe.example/api/graphql" || (*calls)[0].Auth != "Bearer com-token" {
			t.Fatalf("want one POST to GITHUB_GRAPHQL_URL with GH_TOKEN; got %+v", *calls)
		}
	})

	t.Run("gh-is-asked-for-the-guessed-host", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the gh stand-in is a POSIX shell script")
		}
		offlineGitHubEnv(t, "com-token")
		t.Setenv("GH_ENTERPRISE_TOKEN", "")
		t.Setenv("GITHUB_ENTERPRISE_TOKEN", "")
		setOrigin(t, lookalike)
		dir := t.TempDir()
		argLog := filepath.Join(dir, "gh-args")
		// The stand-in answers only a host-scoped ask, the way gh does for a
		// host it holds a login for; a bare `gh auth token` gets nothing.
		script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + argLog + "\"\n" +
			"[ \"$*\" = \"auth token --hostname github.lookalike.example\" ] && echo host-token\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		calls := fakeGraphQL(t, func(int, map[string]any) (int, string) { return 200, branchPage(false, "", nil) })
		if _, err := githubBranchPRPages("/repo"); err != nil {
			t.Fatalf("read with gh's host-scoped login: %v", err)
		}
		args, _ := os.ReadFile(argLog)
		if got := strings.TrimSpace(string(args)); got != "auth token --hostname github.lookalike.example" {
			t.Fatalf("gh must be asked for the guessed host's login only; asked %q", got)
		}
		if len(*calls) != 1 || (*calls)[0].Auth != "Bearer host-token" || sawToken(*calls, "com-token") {
			t.Fatalf("want one request carrying gh's host-scoped token; got %+v", *calls)
		}
	})
}
