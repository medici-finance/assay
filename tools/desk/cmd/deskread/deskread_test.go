package main

// deskread_test.go — the Verify rows for the statusgen-off-the-forge-CLI brief, slice 1.
//
// TestDeskreadIssuesRoundTrip drives the read through the REAL GitHubForge and the REAL
// GitLabForge against each forge's own wire shape, so what is proven is the cross-component
// path (verb → backend → envelope) on both backends, not a fake that agrees with itself.
//
// TestDeskreadPartialIsNotAnError is the negative-path row: a repo that could not be read must
// land in `partial` with its reason and be ABSENT from `repos`, with the exit code still 0 — and
// the all-unreadable case must NOT be exit 0. Without this row the verb could ship rendering an
// unreadable repo as a repo with no open issues, which is the could-not-check-as-a-clean failure
// the whole shape exists to prevent.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// testToken is a low-entropy placeholder — NOT a real credential.
const testToken = "test-injected-token-0000"

// --- backend fixtures ---------------------------------------------------------------------

// githubIssuesServer serves the GitHub REST open-issue list for exactly one repo coordinate.
// Page 2 is empty, which is how the backend's walk terminates.
func githubIssuesServer(t *testing.T, owner, name string, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := fmt.Sprintf("/repos/%s/%s/issues", owner, name)
		if !strings.HasPrefix(r.URL.Path, want) {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") != "1" {
			_, _ = io.WriteString(w, `[]`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// gitlabIssuesServer serves the GitLab project-issue list. NextPage is absent, which is
// GitLab's authoritative end-of-walk signal.
func gitlabIssuesServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/issues") || !strings.HasPrefix(r.URL.Path, "/api/v4/projects/") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// --- row 7: the both-backends round trip ---------------------------------------------------

func TestDeskreadIssuesRoundTrip(t *testing.T) {
	ghSrv := githubIssuesServer(t, "example-org", "alpha", `[
	  {"number": 11, "title": "first", "state": "open",
	   "user": {"login": "someone", "id": 42},
	   "labels": [{"name": "question"}],
	   "created_at": "2026-09-01T00:00:00Z",
	   "html_url": "https://example.test/example-org/alpha/issues/11"},
	  {"number": 12, "title": "a change, not an issue", "state": "open",
	   "user": {"login": "someone", "id": 42},
	   "created_at": "2026-09-02T00:00:00Z",
	   "pull_request": {}}
	]`)
	// The instance-global "id" is present on purpose: GitLab always sends it and the client
	// library's Issue.UnmarshalJSON reflects on it unconditionally, PANICking without one — a
	// fixture that omitted it would be pinning a response shape the real API never produces.
	glSrv := gitlabIssuesServer(t, `[
	  {"id": 1007, "iid": 7, "title": "gl one", "state": "opened",
	   "labels": ["help wanted"],
	   "created_at": "2026-09-03T00:00:00Z",
	   "web_url": "https://gl.example.test/example-org/beta/-/issues/7",
	   "author": {"id": 99, "username": "glbot"}}
	]`)

	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: ghSrv.URL, Client: ghSrv.Client()},
		"example-org/beta":  &deskkit.GitLabForge{Token: testToken, BaseURL: glSrv.URL, Client: glSrv.Client()},
	}, nil)
	defer restore()

	env, code, _ := runEnvelope(t, "issues", "--repo", "example-org/alpha", "--repo", "example-org/beta")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d, want %d", code, deskkit.ExitOK)
	}
	if env.Schema != envelopeSchema {
		t.Errorf("schema=%d, want %d — a consumer pins this and refuses an unrecognised value", env.Schema, envelopeSchema)
	}
	if env.Kind != "issues" {
		t.Errorf("kind=%q, want %q", env.Kind, "issues")
	}
	if len(env.Partial) != 0 {
		t.Errorf("partial=%v, want empty — both backends served the read", env.Partial)
	}
	if len(env.Repos) != 2 {
		t.Fatalf("repos=%d, want 2 — ONE invocation must serve the whole set", len(env.Repos))
	}

	// Order follows the caller's --repo order, not goroutine completion order.
	if env.Repos[0].Repo != "example-org/alpha" || env.Repos[1].Repo != "example-org/beta" {
		t.Errorf("repo order = %q,%q — the envelope must be deterministic", env.Repos[0].Repo, env.Repos[1].Repo)
	}

	// GitHub: the change (pull_request member) is dropped; the issue survives with its fields.
	gh := env.Repos[0].Issues
	if len(gh) != 1 || gh[0].Number != 11 {
		t.Fatalf("github issues = %+v, want exactly issue 11 (the change must be dropped)", gh)
	}
	if gh[0].State != "open" || gh[0].AuthorLogin != "someone" || gh[0].AuthorID != 42 {
		t.Errorf("github issue = %+v, want state=open author=someone/42", gh[0])
	}
	if len(gh[0].Labels) != 1 || gh[0].Labels[0] != "question" {
		t.Errorf("github labels = %v, want [question]", gh[0].Labels)
	}
	if gh[0].CreatedAt == "" {
		t.Error("github createdAt is empty — the stale-issue alarm has nothing to age against")
	}

	// GitLab: the IID is the number, and the author is the forge's own account shape.
	gl := env.Repos[1].Issues
	if len(gl) != 1 || gl[0].Number != 7 {
		t.Fatalf("gitlab issues = %+v, want exactly issue 7", gl)
	}
	if gl[0].AuthorLogin != "glbot" || gl[0].AuthorID != 99 {
		t.Errorf("gitlab author = %s/%d, want glbot/99", gl[0].AuthorLogin, gl[0].AuthorID)
	}
	if gl[0].CreatedAt == "" {
		t.Error("gitlab createdAt is empty")
	}
}

// --- row 8: partial is a result, not an error ----------------------------------------------

func TestDeskreadPartialIsNotAnError(t *testing.T) {
	ghSrv := githubIssuesServer(t, "example-org", "alpha", `[
	  {"number": 3, "title": "ok", "state": "open",
	   "user": {"login": "someone", "id": 42},
	   "created_at": "2026-09-01T00:00:00Z"}
	]`)
	// The second readable repo serves an EMPTY open-issue list. That is a real answer — "no
	// open issues" — and the test asserts it is rendered differently from the unreadable one.
	emptySrv := githubIssuesServer(t, "example-org", "empty", `[]`)

	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: ghSrv.URL, Client: ghSrv.Client()},
		"example-org/empty": &deskkit.GitHubForge{Token: testToken, BaseURL: emptySrv.URL, Client: emptySrv.Client()},
	}, map[string]error{
		"example-org/locked": deskkit.Unverifiable("the App installation cannot read example-org/locked", nil),
	})
	defer restore()

	env, code, _ := runEnvelope(t, "issues",
		"--repo", "example-org/alpha", "--repo", "example-org/locked", "--repo", "example-org/empty")

	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d, want %d — one unreadable repo in a set is a PARTIAL result, not a failed run", code, deskkit.ExitOK)
	}
	if len(env.Repos) != 2 {
		t.Fatalf("repos=%d, want 2 (alpha, empty)", len(env.Repos))
	}
	if len(env.Partial) != 1 || env.Partial[0].Repo != "example-org/locked" {
		t.Fatalf("partial=%+v, want exactly example-org/locked", env.Partial)
	}
	if env.Partial[0].Reason == "" {
		t.Error("the partial entry carries no reason — an unreadable repo with no stated reason is indistinguishable from a bug")
	}
	// The unreadable repo must be ABSENT from repos. If it appeared with an empty issue list it
	// would be indistinguishable from example-org/empty, which is the whole defect.
	for _, r := range env.Repos {
		if r.Repo == "example-org/locked" {
			t.Fatal("the unreadable repo appears in repos — could-not-check has been rendered as a clean empty answer")
		}
	}
	// And the genuinely empty repo must be present WITH an empty list, so a caller can tell the
	// two apart in the other direction too.
	var sawEmpty bool
	for _, r := range env.Repos {
		if r.Repo == "example-org/empty" {
			sawEmpty = true
			if len(r.Issues) != 0 {
				t.Errorf("example-org/empty issues = %+v, want none", r.Issues)
			}
		}
	}
	if !sawEmpty {
		t.Error("the empty-but-readable repo is missing from repos — 'no open issues' is an ANSWER and must be reported")
	}

	// ALL repos unreadable is the one case that is not a partial: there is nothing to be
	// partial about, so the run is unverifiable rather than a green empty envelope.
	restore()
	restore2 := stubForges(t, nil, map[string]error{
		"example-org/locked": deskkit.Unverifiable("cannot read", nil),
		"example-org/other":  deskkit.Unverifiable("cannot read", nil),
	})
	defer restore2()
	_, code2, _ := runEnvelope(t, "issues", "--repo", "example-org/locked", "--repo", "example-org/other")
	if code2 != deskkit.ExitUnverifiable {
		t.Fatalf("all-unreadable exit=%d, want %d — a set in which nothing could be read is could-not-check, not an empty result",
			code2, deskkit.ExitUnverifiable)
	}
}

// --- the closed kind set and flag refusals -------------------------------------------------

func TestDeskreadRefusesUnknownKindAndFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"unknown kind", []string{"raw", "--repo", "example-org/alpha"}},
		{"no repo", []string{"issues"}},
		{"bad repo", []string{"issues", "--repo", "not-a-slug"}},
		{"unknown flag", []string{"issues", "--repo", "example-org/alpha", "--query", "is:open"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb strings.Builder
			if code := run(tc.args, &out, &errb); code != deskkit.ExitRefused {
				t.Fatalf("exit=%d, want %d (refused); stderr=%s", code, deskkit.ExitRefused, errb.String())
			}
			if out.Len() != 0 {
				t.Errorf("a refused run wrote to stdout: %q", out.String())
			}
		})
	}
}

// --- helpers -------------------------------------------------------------------------------

// stubForges replaces the resolver with a lookup table, so a test proves the VERB's behaviour
// without a minted credential. `fail` maps a repo to the error its resolution returns — the
// unreadable case. It returns a restore function.
func stubForges(t *testing.T, forges map[string]deskkit.Forge, fail map[string]error) func() {
	t.Helper()
	prev := forgeFor
	forgeFor = func(repo string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		owner, name, _ := strings.Cut(repo, "/")
		fr := deskkit.ForgeRepo{Owner: owner, Name: name}
		if err, ok := fail[repo]; ok {
			return nil, fr, err
		}
		f, ok := forges[repo]
		if !ok {
			return nil, fr, deskkit.Unverifiable("no stub forge for "+repo, nil)
		}
		return f, fr, nil
	}
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		forgeFor = prev
	}
}

func runEnvelope(t *testing.T, args ...string) (Envelope, int, string) {
	t.Helper()
	var out, errb strings.Builder
	code := run(args, &out, &errb)
	var env Envelope
	if s := strings.TrimSpace(out.String()); s != "" {
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatalf("stdout is not the JSON envelope: %v\n%s", err, s)
		}
	}
	return env, code, errb.String()
}
