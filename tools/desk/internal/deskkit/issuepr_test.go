package deskkit

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #6908
func TestDispatchIssueLinks(t *testing.T) {
	const repo = "example-org/target"
	prs := []PRRef{
		{1, "OPEN", "Issue: #42"},
		{2, "MERGED", "Fixes: example-org/target#43"},
		{3, "OPEN", "Closes: example-org/other#44"},
		{4, "CLOSED", "Issue: #45"},
		{5, "OPEN", "Example:\n~~~\nIssue: #46\n~~~"},
		{6, "OPEN", "Brief: example/02\nIssue: #47"},
		{7, "OPEN", "Issue: #480"},
		{8, "MERGED", "Issue: #49"},
		{9, "OPEN", "Issue: #49"},
	}
	calls := 0
	idx := IssuePRIndex{List: func(got string) ([]PRRef, bool, error) {
		calls++
		if got != repo {
			t.Fatal(got)
		}
		return prs, false, nil
	}}
	for _, tt := range []struct {
		n, pr  int
		merged bool
	}{{42, 1, false}, {43, 2, true}, {44, 0, false}, {45, 0, false}, {46, 0, false}, {47, 0, false}, {48, 0, false}, {49, 9, false}} {
		got, err := idx.Match(DispatchIssue{Repo: repo, Number: tt.n})
		if err != nil || got.Number != tt.pr || got.Merged != tt.merged {
			t.Errorf("issue %d: %+v, %v", tt.n, got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("list read %d times", calls)
	}
}

func TestDispatchIssuePastWindow(t *testing.T) {
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		if got := r.URL.Query().Get("q"); got != `repo:example-org/target is:pr "42" in:body` {
			t.Errorf("query %q", got)
		}
		fmt.Fprint(w, `{"total_count":2,"incomplete_results":false,"items":[{"number":81,"state":"closed","body":"Fixes: #42","pull_request":{"merged_at":"2026-01-02T00:00:00Z"}},{"number":82,"state":"open","body":"Issue: #420","pull_request":{}}]}`)
	})
	calls := 0
	idx := IssuePRIndex{Lookup: func(_ string, id string) ([]PRRef, error) {
		calls++
		return ChangesByBriefReader(gh.forge())(ForgeRepo{Owner: "example-org", Name: "target"}, id)
	}}
	idx.RecordWindow("example-org/target", []PRRef{{79, "MERGED", "Issue: #42"}}, true, nil)
	issue := DispatchIssue{Repo: "example-org/target", Number: 42, partial: map[int]bool{79: true}}
	for i := 0; i < 2; i++ {
		got, err := idx.Match(issue)
		if err != nil || got.Number != 81 || !got.Merged {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
	if calls != 1 {
		t.Fatalf("%d lookups", calls)
	}
	idx.Lookup = func(string, string) ([]PRRef, error) { return nil, errors.New("offline") }
	if _, err := idx.Match(DispatchIssue{Repo: issue.Repo, Number: 44}); err == nil {
		t.Fatal("failed read became fresh")
	}
}

func TestDispatchPlaceholderSource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "issue-loop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	withRoster(t, map[string]string{"ASSAY_ALLOWED_REPOS": "example-org/target:ci:public"})
	file := filepath.Join(dir, "target-issue-7.md")
	put := func(n int) {
		t.Helper()
		if err := os.WriteFile(file, []byte(fmt.Sprintf("---\nschema: placeholder-v1\nbrief: issue-loop/target-issue-7\nissue: %d\nrepo: example-org/target\n---\n", n)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put(7)
	issue, err := DispatchPlaceholder(root, "example-org/tracking", "issue-loop/target-issue-7")
	if err != nil || issue.Repo != "example-org/target" || issue.Number != 7 {
		t.Fatalf("%+v %v", issue, err)
	}
	put(8)
	if _, err := DispatchPlaceholder(root, "example-org/tracking", "issue-loop/target-issue-7"); err == nil {
		t.Fatal("mismatched file admitted")
	}
	if _, err := DispatchPlaceholder(root, "example-org/tracking", "issue-loop/missing-issue-7"); err == nil {
		t.Fatal("missing foreign identity guessed")
	}
	if _, err := DispatchPlaceholder(root, "example-org/tracking", "issue-loop/issue-7"); err == nil {
		t.Fatal("missing legacy source guessed")
	}
	if _, err := DispatchPlaceholder(root, "example-org/tracking", "../issue-7"); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatal(err)
	}
}
