package deskkit

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// forge_github_typedwalk_test.go — the forge-side CLASS guard for "a typed comment-thread read
// hands back the head of a thread as the whole thread".
//
// ListCommentsTyped is what an authorization lookup reads a thread through: cmd/deskmerge's R-5
// and cmd/deskclose's R-1 gates match the sign-off comment by id on the permalink's thread and
// read "not found" as absence. On GitHub the change half used to be the single first-100
// request, so a sign-off past comment 100 of a pull-request thread was refused as deleted.
// This test enumerates EVERY kind ListCommentsTyped accepts and holds each to the same three
// properties: it follows the cursor to the end, it refuses (never truncates) at the page cap,
// and it refuses a next page advertised with no cursor. A kind added to the switch without a
// walk fails here, not at a sign-off.
//
// FAIL-FIRST: on the single-request change read, the TargetChange sub-tests return page 1
// only (want 2 comments, got 1) and the two refusal sub-tests get a nil error.

var typedWalkKinds = []TargetKind{TargetIssue, TargetChange}

func typedCommentsPage(kind TargetKind, hasNext bool, cursor string, id int) string {
	noteable := "issue"
	if kind == TargetChange {
		noteable = "pullRequest"
	}
	return fmt.Sprintf(`{"data":{"repository":{%q:{"comments":{`+
		`"pageInfo":{"hasNextPage":%t,"endCursor":%q},`+
		`"nodes":[{"id":"IC_%d","databaseId":%d,"body":"b","isMinimized":false,`+
		`"createdAt":"2026-07-02T00:00:00Z","url":"https://example.test/c",`+
		`"author":{"login":"someone","__typename":"User","databaseId":7}}]}}}}}`,
		noteable, hasNext, cursor, id, id)
}

var pageInfoRe = regexp.MustCompile(`"pageInfo":\{[^}]*\},`)

// answerAsAsked serves page as the GitHub API would for the query body it was sent: a query
// that does not select pageInfo gets the nodes only, so a read that sends the single first-100
// query sees one page and no continuation. Without this the fixture hands every query the
// continuation it asks for, and a typed read that quietly stopped walking would still pass here.
func answerAsAsked(query, page string) string {
	if strings.Contains(query, "pageInfo") {
		return page
	}
	return pageInfoRe.ReplaceAllString(page, "")
}

func TestTypedCommentsWalkEveryKind(t *testing.T) {
	for _, kind := range typedWalkKinds {
		t.Run(string(kind)+"/walks", func(t *testing.T) {
			var seen []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), `"after":"CURSOR1"`) {
					seen = append(seen, "CURSOR1")
					io.WriteString(w, answerAsAsked(string(body), typedCommentsPage(kind, false, "CURSOR2", 2)))
					return
				}
				seen = append(seen, "<none>")
				io.WriteString(w, answerAsAsked(string(body), typedCommentsPage(kind, true, "CURSOR1", 1)))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, kind)
			if err != nil {
				t.Fatalf("ListCommentsTyped(%s): %v", kind, err)
			}
			if len(got) != 2 || got[1].DatabaseID != 2 {
				t.Fatalf("want 2 comments across 2 pages, got %d: %+v — the %s read stopped at its first page", len(got), got, kind)
			}
			if len(seen) != 2 || seen[1] != "CURSOR1" {
				t.Errorf("want page 1 then page 2 (CURSOR1); got %v", seen)
			}
		})
		t.Run(string(kind)+"/cap", func(t *testing.T) {
			pages := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				pages++
				io.WriteString(w, answerAsAsked(string(body), typedCommentsPage(kind, true, "MORE", pages)))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, kind)
			if err == nil || !IsUnverifiable(err) {
				t.Fatalf("a %s thread still advertising a next page at the cap must be could-not-check; got %d comments, err %v", kind, len(got), err)
			}
			if pages != forgeMaxEventPages {
				t.Errorf("want %d pages walked before the cap, got %d", forgeMaxEventPages, pages)
			}
		})
		t.Run(string(kind)+"/cursorless", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				io.WriteString(w, answerAsAsked(string(body), typedCommentsPage(kind, true, "", 1)))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			if got, err := gh.ListCommentsTyped(ForgeRepo{Owner: "o", Name: "r"}, 20, kind); err == nil || !IsUnverifiable(err) {
				t.Fatalf("a %s next page with no cursor must be could-not-check; got %+v, err %v", kind, got, err)
			}
		})
	}
}
