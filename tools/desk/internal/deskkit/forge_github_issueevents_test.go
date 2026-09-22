package deskkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGitHubIssueContentEventsPaginates proves the escalation-clock read walks the comment
// connection to exhaustion (the whole thread, across pages), re-suffixes a Bot author to the
// "<slug>[bot]" rendering the trust set expects, and reports Complete=true when it reaches the
// end. It is the forge-level counterpart of the board-level whole-board regression test: the
// escalation clock needs the WHOLE thread, unlike the single-page trust gate.
func TestGitHubIssueContentEventsPaginates(t *testing.T) {
	var seenCursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		q := string(body)
		if !strings.Contains(q, "comments(first:100,after:$after)") {
			http.Error(w, "unexpected query: "+q, http.StatusBadRequest)
			return
		}
		// Page 1 has no cursor; page 2 is fetched with the cursor page 1 returned.
		if strings.Contains(q, `"after":"CURSOR1"`) {
			seenCursors = append(seenCursors, "CURSOR1")
			io.WriteString(w, `{"data":{"repository":{"issue":{"comments":{`+
				`"pageInfo":{"hasNextPage":false,"endCursor":"CURSOR2"},`+
				`"nodes":[{"createdAt":"2026-07-03T00:00:00Z","author":{"login":"botslug","__typename":"Bot","databaseId":42}}]}}}}}`)
			return
		}
		seenCursors = append(seenCursors, "<none>")
		io.WriteString(w, `{"data":{"repository":{"issue":{"comments":{`+
			`"pageInfo":{"hasNextPage":true,"endCursor":"CURSOR1"},`+
			`"nodes":[{"createdAt":"2026-07-02T00:00:00Z","author":{"login":"human-user","__typename":"User","databaseId":7}}]}}}}}`)
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	tp, err := gh.IssueContentEvents(ForgeRepo{Owner: "o", Name: "r"}, 20)
	if err != nil {
		t.Fatalf("IssueContentEvents: %v", err)
	}
	if !tp.Complete {
		t.Errorf("reached the end of the thread; Complete = false, want true")
	}
	if len(tp.Events) != 2 {
		t.Fatalf("expected 2 events across 2 pages, got %d: %+v", len(tp.Events), tp.Events)
	}
	if got := tp.Events[1].Author; got != "botslug[bot]" {
		t.Errorf("bot author rendering = %q, want botslug[bot]", got)
	}
	if len(seenCursors) != 2 || seenCursors[0] != "<none>" || seenCursors[1] != "CURSOR1" {
		t.Errorf("expected page 1 (no cursor) then page 2 (CURSOR1); got %v", seenCursors)
	}
}

// TestGitHubIssueContentEventsCapReportsIncomplete proves the read stays BOUNDED: a thread
// that never stops advertising a next page is walked exactly forgeMaxEventPages times and then
// reported Complete=false — the signal the board degrades that ONE row on (escalate,
// could-not-check) rather than looping forever or failing the whole board.
func TestGitHubIssueContentEventsCapReportsIncomplete(t *testing.T) {
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.ReadAll(r.Body)
		pages++
		// Always advertise another page, with a fresh cursor, so the walk only stops at the cap.
		io.WriteString(w, `{"data":{"repository":{"issue":{"comments":{`+
			`"pageInfo":{"hasNextPage":true,"endCursor":"MORE"},`+
			`"nodes":[{"createdAt":"2026-07-02T00:00:00Z","author":{"login":"human-user","__typename":"User","databaseId":7}}]}}}}}`)
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	tp, err := gh.IssueContentEvents(ForgeRepo{Owner: "o", Name: "r"}, 20)
	if err != nil {
		t.Fatalf("IssueContentEvents: %v", err)
	}
	if tp.Complete {
		t.Errorf("an unbounded thread must report Complete=false at the cap; got true")
	}
	if pages != forgeMaxEventPages {
		t.Errorf("expected exactly %d pages walked before the cap, got %d", forgeMaxEventPages, pages)
	}
}
