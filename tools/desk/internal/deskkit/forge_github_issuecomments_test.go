package deskkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forge_github_issuecomments_test.go — the ISSUE thread read walks every page.
//
// ListCommentsTyped(…, TargetIssue) is what statusgen's un-block lane reads an issue thread
// through (via the `deskread comments` kind) in place of a `gh api --paginate` shell-out. That
// lane keys on the NEWEST blessing-authority comment, and a first-100 read drops exactly the
// newest comments on a long thread — so the GitHub backend must follow the connection's cursor
// to the end, and must refuse (never truncate) when it cannot.
//
// FAIL-FIRST: on the single-request read (no `pageInfo`, no `after`) the two-page test below
// returns only page 1's comment and the cursor assertion sees one request, and the two refusal
// tests get a nil error back with a truncated thread.

func issueCommentsPage(hasNext bool, cursor, login, created string) string {
	next := "false"
	if hasNext {
		next = "true"
	}
	return `{"data":{"repository":{"issue":{"comments":{` +
		`"pageInfo":{"hasNextPage":` + next + `,"endCursor":"` + cursor + `"},` +
		`"nodes":[{"id":"IC_` + login + `","databaseId":5,"body":"b","isMinimized":false,` +
		`"createdAt":"` + created + `","url":"https://example.test/c",` +
		`"author":{"login":"` + login + `","__typename":"User","databaseId":7}}]}}}}}`
}

func TestGitHubIssueCommentsWalkEveryPage(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		if !strings.Contains(q, "comments(first: 100, after: $after)") {
			http.Error(w, "unexpected query: "+q, http.StatusBadRequest)
			return
		}
		if strings.Contains(q, `"after":"CURSOR1"`) {
			seen = append(seen, "CURSOR1")
			io.WriteString(w, issueCommentsPage(false, "CURSOR2", "newest-answer", "2026-07-03T00:00:00Z"))
			return
		}
		if strings.Contains(q, `"after"`) {
			http.Error(w, "unexpected cursor: "+q, http.StatusBadRequest)
			return
		}
		seen = append(seen, "<none>")
		io.WriteString(w, issueCommentsPage(true, "CURSOR1", "oldest", "2026-07-02T00:00:00Z"))
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, TargetIssue)
	if err != nil {
		t.Fatalf("ListCommentsTyped(issue): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 comments across 2 pages, got %d: %+v — the newest page was dropped", len(got), got)
	}
	if got[1].Author.Login != "newest-answer" {
		t.Errorf("last comment author = %q, want newest-answer (oldest-first order across pages)", got[1].Author.Login)
	}
	if len(seen) != 2 || seen[0] != "<none>" || seen[1] != "CURSOR1" {
		t.Errorf("want page 1 (no cursor) then page 2 (CURSOR1); got %v", seen)
	}
}

func TestGitHubIssueCommentsRefuseAtCap(t *testing.T) {
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		pages++
		io.WriteString(w, issueCommentsPage(true, "MORE", "someone", "2026-07-02T00:00:00Z"))
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, TargetIssue)
	if err == nil {
		t.Fatalf("an issue thread still advertising a next page at the cap must be could-not-check; got %d comments and no error", len(got))
	}
	if pages != forgeMaxEventPages {
		t.Errorf("want exactly %d pages walked before the cap, got %d", forgeMaxEventPages, pages)
	}
}

func TestGitHubIssueCommentsRefuseCursorlessNextPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		io.WriteString(w, issueCommentsPage(true, "", "someone", "2026-07-02T00:00:00Z"))
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	if got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, TargetIssue); err == nil {
		t.Fatalf("a next page with no cursor must be could-not-check, never the head of the thread; got %+v", got)
	}
}

// TestGitHubChangeCommentsStaySingleRequest pins that the walk is ISSUE-only: the change read
// is still the one first-100 request the golden corpus pins, with no cursor variable.
func TestGitHubChangeCommentsStaySingleRequest(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests++
		if strings.Contains(string(body), "after") {
			http.Error(w, "change read must not carry a cursor", http.StatusBadRequest)
			return
		}
		io.WriteString(w, `{"data":{"repository":{"pullRequest":{"comments":{"nodes":[]}}}}}`)
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	if _, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 21, TargetChange); err != nil {
		t.Fatalf("ListCommentsTyped(change): %v", err)
	}
	if requests != 1 {
		t.Errorf("change read made %d requests, want 1", requests)
	}
}
