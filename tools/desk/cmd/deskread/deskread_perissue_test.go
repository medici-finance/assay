package main

// deskread_perissue_test.go — the `trust` and `comments` kinds (the statusgen --scan-issues
// trust gate and un-block lane, moved off `gh api graphql` / `gh api --paginate`).
//
// Each read goes through the REAL GitHubForge against GitHub's own GraphQL wire shape, so what is
// proven is verb → backend → envelope, not a fake that agrees with itself. Before these kinds
// existed the verb REFUSED both (unknown kind, exit 5), which is the fail-first state.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// graphqlServer answers every POST /graphql with body, and records each request body.
func graphqlServer(t *testing.T, body string, seen *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if seen != nil {
			*seen = append(*seen, string(b))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runItemEnvelope(t *testing.T, args ...string) (ItemEnvelope, int, string) {
	t.Helper()
	var out, errb strings.Builder
	code := run(args, &out, &errb)
	var env ItemEnvelope
	if s := strings.TrimSpace(out.String()); s != "" {
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatalf("stdout is not the JSON envelope: %v\n%s", err, s)
		}
	}
	return env, code, errb.String()
}

func TestDeskreadTrustRoundTrip(t *testing.T) {
	var seen []string
	srv := graphqlServer(t, `{"data":{"repository":{"issue":{
	  "lastEditedAt":"2026-09-01T10:00:00.5Z",
	  "comments":{"pageInfo":{"hasNextPage":false},"nodes":[
	    {"createdAt":"2026-09-02T00:00:00Z","lastEditedAt":null,
	     "author":{"login":"authority","__typename":"User","databaseId":100001}},
	    {"createdAt":"2026-09-03T00:00:00Z","lastEditedAt":"2026-09-04T00:00:00Z",
	     "author":{"login":"example-app","__typename":"Bot","databaseId":300000001}},
	    {"createdAt":"2026-09-05T00:00:00Z","lastEditedAt":null,"author":null}
	  ]}}}}}`, &seen)
	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()},
	}, nil)
	defer restore()

	env, code, errs := runItemEnvelope(t, "trust", "--issue", "example-org/alpha#7")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d, want %d; stderr=%s", code, deskkit.ExitOK, errs)
	}
	if env.Schema != envelopeSchema || env.Kind != "trust" {
		t.Errorf("schema/kind = %d/%q, want %d/trust", env.Schema, env.Kind, envelopeSchema)
	}
	if len(env.Partial) != 0 || len(env.Items) != 1 {
		t.Fatalf("items=%+v partial=%+v, want exactly one item", env.Items, env.Partial)
	}
	it := env.Items[0]
	if it.Repo != "example-org/alpha" || it.Number != 7 || it.Trust == nil || it.Comments != nil {
		t.Fatalf("item = %+v, want example-org/alpha#7 carrying trust only", it)
	}
	tr := it.Trust
	if !tr.Complete {
		t.Error("complete=false, want true (hasNextPage=false)")
	}
	// Sub-second precision survives: a tie-sensitive rule downstream must not see a rounded stamp.
	if tr.BodyEditedAt != "2026-09-01T10:00:00.5Z" {
		t.Errorf("bodyEditedAt = %q, want 2026-09-01T10:00:00.5Z", tr.BodyEditedAt)
	}
	if len(tr.Events) != 3 {
		t.Fatalf("events = %+v, want 3", tr.Events)
	}
	if e := tr.Events[0]; e.AuthorLogin != "authority" || e.AuthorID != 100001 || e.CreatedAt != "2026-09-02T00:00:00Z" || e.EditedAt != "" {
		t.Errorf("event 0 = %+v", e)
	}
	// A GraphQL Bot actor is re-suffixed to the rendering the trust set expects.
	if e := tr.Events[1]; e.AuthorLogin != "example-app[bot]" || e.EditedAt != "2026-09-04T00:00:00Z" {
		t.Errorf("event 1 = %+v, want example-app[bot] edited 2026-09-04", e)
	}
	// A deleted account stays an EMPTY login — untrusted downstream, never invented.
	if e := tr.Events[2]; e.AuthorLogin != "" || e.AuthorID != 0 {
		t.Errorf("event 2 = %+v, want an empty (deleted) author", e)
	}
	if len(seen) != 1 || !strings.Contains(seen[0], "lastEditedAt comments(first:100)") {
		t.Errorf("want ONE bounded trust query; got %v", seen)
	}
}

func TestDeskreadTrustOverflowIsAnAnswerNotPartial(t *testing.T) {
	srv := graphqlServer(t, `{"data":{"repository":{"issue":{"lastEditedAt":null,
	  "comments":{"pageInfo":{"hasNextPage":true},"nodes":[]}}}}}`, nil)
	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()},
	}, nil)
	defer restore()

	env, code, _ := runItemEnvelope(t, "trust", "--issue", "example-org/alpha#8")
	if code != deskkit.ExitOK || len(env.Items) != 1 {
		t.Fatalf("exit=%d items=%+v — an overflowed thread was READ; it is an answer (complete=false)", code, env.Items)
	}
	if env.Items[0].Trust.Complete {
		t.Error("complete=true on an overflowed thread — the consumer would evaluate a blessing off a partial thread")
	}
}

func TestDeskreadCommentsRoundTrip(t *testing.T) {
	var seen []string
	srv := graphqlServer(t, `{"data":{"repository":{"issue":{"comments":{
	  "pageInfo":{"hasNextPage":false,"endCursor":"END"},
	  "nodes":[
	    {"id":"IC_1","databaseId":11,"body":"<!-- desk-automation --> status","isMinimized":false,
	     "createdAt":"2026-09-02T00:00:00Z","url":"https://example.test/c/11",
	     "author":{"login":"example-app","__typename":"Bot","databaseId":300000001}},
	    {"id":"IC_2","databaseId":12,"body":"go ahead","isMinimized":false,
	     "createdAt":"2026-09-03T00:00:00Z","url":"https://example.test/c/12",
	     "author":{"login":"authority","__typename":"User","databaseId":100001}}
	  ]}}}}}`, &seen)
	emptySrv := graphqlServer(t, `{"data":{"repository":{"issue":{"comments":{
	  "pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[]}}}}}`, nil)
	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()},
		"example-org/empty": &deskkit.GitHubForge{Token: testToken, BaseURL: emptySrv.URL, Client: emptySrv.Client()},
	}, nil)
	defer restore()

	env, code, errs := runItemEnvelope(t, "comments",
		"--issue", "example-org/alpha#9", "--issue", "example-org/empty#1")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, errs)
	}
	if len(env.Items) != 2 || len(env.Partial) != 0 {
		t.Fatalf("items=%+v partial=%+v, want two items", env.Items, env.Partial)
	}
	it := env.Items[0]
	if it.Repo != "example-org/alpha" || it.Number != 9 || it.Comments == nil || it.Trust != nil {
		t.Fatalf("item 0 = %+v, want example-org/alpha#9 carrying comments only", it)
	}
	cs := *it.Comments
	if len(cs) != 2 {
		t.Fatalf("comments = %+v, want 2", cs)
	}
	if cs[0].AuthorLogin != "example-app[bot]" || !strings.Contains(cs[0].Body, "desk-automation") {
		t.Errorf("comment 0 = %+v, want the bot-rendered author and its body (the marker check reads it)", cs[0])
	}
	if cs[1].AuthorLogin != "authority" || cs[1].AuthorID != 100001 || cs[1].CreatedAt != "2026-09-03T00:00:00Z" || cs[1].Body != "go ahead" {
		t.Errorf("comment 1 = %+v", cs[1])
	}
	// The read addressed the ISSUE's thread (issue selection), not a change's.
	if len(seen) != 1 || !strings.Contains(seen[0], "issue(number: $number)") {
		t.Errorf("want one issue-thread request; got %v", seen)
	}
	// An issue with no comments is an ANSWER: present, with an empty (not absent) list.
	e := env.Items[1]
	if e.Comments == nil || len(*e.Comments) != 0 {
		t.Errorf("example-org/empty#1 comments = %v, want a present empty list", e.Comments)
	}
}

func TestDeskreadPerIssuePartialAndAllUnreadable(t *testing.T) {
	srv := graphqlServer(t, `{"data":{"repository":{"issue":{"comments":{
	  "pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`, nil)
	restore := stubForges(t, map[string]deskkit.Forge{
		"example-org/alpha": &deskkit.GitHubForge{Token: testToken, BaseURL: srv.URL, Client: srv.Client()},
	}, map[string]error{
		"example-org/locked": deskkit.Unverifiable("the App installation cannot read example-org/locked", nil),
	})
	defer restore()

	env, code, _ := runItemEnvelope(t, "comments", "--issue", "example-org/alpha#1", "--issue", "example-org/locked#2")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d — one unreadable issue in a set is a PARTIAL result", code)
	}
	if len(env.Partial) != 1 || env.Partial[0].Repo != "example-org/locked" || env.Partial[0].Number != 2 || env.Partial[0].Reason == "" {
		t.Fatalf("partial = %+v, want example-org/locked#2 with a reason", env.Partial)
	}
	for _, it := range env.Items {
		if it.Repo == "example-org/locked" {
			t.Fatal("the unreadable issue appears in items — could-not-check rendered as an empty thread")
		}
	}

	_, code2, _ := runItemEnvelope(t, "trust", "--issue", "example-org/locked#2")
	if code2 != deskkit.ExitUnverifiable {
		t.Fatalf("all-unreadable exit=%d, want %d", code2, deskkit.ExitUnverifiable)
	}
}

func TestDeskreadPerIssueAddressingRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"trust with --repo", []string{"trust", "--repo", "example-org/alpha"}},
		{"comments with no --issue", []string{"comments"}},
		{"issues with --issue", []string{"issues", "--issue", "example-org/alpha#1"}},
		{"issue with no number", []string{"trust", "--issue", "example-org/alpha"}},
		{"issue number zero", []string{"trust", "--issue", "example-org/alpha#0"}},
		{"issue number not decimal", []string{"comments", "--issue", "example-org/alpha#+7"}},
		{"issue with a bad slug", []string{"comments", "--issue", "not-a-slug#7"}},
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
