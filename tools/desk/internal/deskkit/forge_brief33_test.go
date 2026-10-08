package deskkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forge_brief33_test.go — the negative paths of forge-neutral brief 33's four reads (ops 55-58)
// and its widened fields, on BOTH backends. Every test here pins one rule the brief states as
// "absent is never a value": a population past the ceiling is Incomplete, an unreadable field
// stays EMPTY, and nothing is filled from a neighbouring field the forge did assert.

// b33GitHub serves GitHub requests through handle and records each request's method and path.
type b33GitHub struct {
	srv  *httptest.Server
	seen []string
}

func newB33GitHub(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, body string)) *b33GitHub {
	t.Helper()
	s := &b33GitHub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.seen = append(s.seen, r.Method+" "+r.URL.Path)
		handle(w, r, string(b))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *b33GitHub) forge() *GitHubForge {
	return &GitHubForge{Token: "test-token", BaseURL: s.srv.URL, Client: s.srv.Client()}
}

func b33Enc(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }

// b33After reads the GraphQL `after` variable out of a request body ("" when null).
func b33After(t *testing.T, body string) string {
	t.Helper()
	var in struct {
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		t.Fatalf("decode GraphQL body: %v", err)
	}
	a, _ := in.Variables["after"].(string)
	return a
}

// b33IssueNodes builds n GraphQL issue nodes numbered from first.
func b33IssueNodes(first, n int) []map[string]any {
	out := make([]map[string]any, 0, n)
	for i := first; i < first+n; i++ {
		node := map[string]any{"number": i, "title": fmt.Sprintf("issue %d", i), "state": "OPEN",
			"createdAt": "2026-09-01T09:00:00Z", "closedAt": nil, "url": fmt.Sprintf("https://example/issues/%d", i),
			"labels": map[string]any{"nodes": []map[string]any{{"name": "area:x"}}, "pageInfo": map[string]any{"hasNextPage": false}},
			"author": map[string]any{"login": "someone", "__typename": "User", "databaseId": 5}}
		if i%2 == 0 {
			node["state"] = "CLOSED"
			node["closedAt"] = "2026-09-02T09:00:00Z"
		}
		out = append(out, node)
	}
	return out
}

func b33IssuesPage(nodes []map[string]any, hasNext bool, cursor string) map[string]any {
	return map[string]any{"data": map[string]any{"repository": map[string]any{"issues": map[string]any{
		"pageInfo": map[string]any{"hasNextPage": hasNext, "endCursor": cursor}, "nodes": nodes}}}}
}

func TestListIssuesIncompleteIsNotAbsence(t *testing.T) {
	t.Run("github_page_ceiling", func(t *testing.T) {
		pages := 0
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			pages++
			b33Enc(w, b33IssuesPage(b33IssueNodes(pages*1000, 100), true, fmt.Sprintf("c%d", pages)))
		})
		got, err := gh.forge().ListIssues(forgeTestRepo, IssueListQuery{State: IssueStateAll})
		if err != nil {
			t.Fatalf("ListIssues: %v", err)
		}
		if !got.Incomplete || got.PageCap != 100 || pages != 100 || len(got.Issues) != 10000 {
			t.Fatalf("a population past 100 pages of 100 must read Incomplete with PageCap 100: "+
				"Incomplete=%v PageCap=%d pages=%d issues=%d", got.Incomplete, got.PageCap, pages, len(got.Issues))
		}
	})
	t.Run("github_label_connection_overflow", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			nodes := b33IssueNodes(1, 1)
			nodes[0]["labels"] = map[string]any{"nodes": []map[string]any{{"name": "a"}},
				"pageInfo": map[string]any{"hasNextPage": true}}
			b33Enc(w, b33IssuesPage(nodes, false, ""))
		})
		got, err := gh.forge().ListIssues(forgeTestRepo, IssueListQuery{State: IssueStateOpen})
		if err != nil {
			t.Fatalf("ListIssues: %v", err)
		}
		if !got.Incomplete {
			t.Fatal("an issue whose label connection still paginates must make the list Incomplete")
		}
	})
	t.Run("gitlab_page_ceiling", func(t *testing.T) {
		s := newGLServer(t)
		s.issuePages = -1
		got, err := s.forge().ListIssues(glRepo, IssueListQuery{State: IssueStateAll})
		if err != nil {
			t.Fatalf("ListIssues: %v", err)
		}
		if !got.Incomplete || got.PageCap != 25 || len(s.requests) != 25 {
			t.Fatalf("a GitLab population past 25 pages must read Incomplete with PageCap 25: "+
				"Incomplete=%v PageCap=%d requests=%d", got.Incomplete, got.PageCap, len(s.requests))
		}
	})
	t.Run("comma_label_refused_with_zero_calls", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) { b33Enc(w, map[string]any{}) })
		if _, err := gh.forge().ListIssues(forgeTestRepo, IssueListQuery{State: IssueStateOpen, Label: "a,b"}); err == nil || len(gh.seen) != 0 {
			t.Fatalf("GitHub: a comma label must be refused with zero calls: err=%v calls=%d", err, len(gh.seen))
		}
		s := newGLServer(t)
		if _, err := s.forge().ListIssues(glRepo, IssueListQuery{State: IssueStateOpen, Label: "a,b"}); err == nil || len(s.requests) != 0 {
			t.Fatalf("GitLab: a comma label must be refused with zero calls: err=%v calls=%d", err, len(s.requests))
		}
	})
}

func TestListChangeCommitsOverCapIsIncomplete(t *testing.T) {
	t.Run("github_250_of_300", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			if strings.HasSuffix(r.URL.Path, "/commits") {
				n := map[string]int{"1": 100, "2": 100, "3": 50}[r.URL.Query().Get("page")]
				list := make([]map[string]any, 0, n)
				for i := 0; i < n; i++ {
					list = append(list, map[string]any{"sha": fmt.Sprintf("%s-%03d", r.URL.Query().Get("page"), i)})
				}
				b33Enc(w, list)
				return
			}
			b33Enc(w, map[string]any{"number": 7, "commits": 300})
		})
		got, err := gh.forge().ListChangeCommits(forgeTestRepo, 7)
		if err != nil {
			t.Fatalf("ListChangeCommits: %v", err)
		}
		if got.Complete || len(got.SHAs) != 250 {
			t.Fatalf("the 250-of-300 case: a change whose own commit count (300) exceeds the 250 listed "+
				"must read Complete=false; got Complete=%v with %d SHAs", got.Complete, len(got.SHAs))
		}
	})
	t.Run("github_count_matches", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			if strings.HasSuffix(r.URL.Path, "/commits") {
				b33Enc(w, []map[string]any{{"sha": "a"}, {"sha": "b"}})
				return
			}
			b33Enc(w, map[string]any{"number": 7, "commits": 2})
		})
		got, err := gh.forge().ListChangeCommits(forgeTestRepo, 7)
		if err != nil || !got.Complete {
			t.Fatalf("a listed count equal to the change's own count is Complete: %+v err=%v", got, err)
		}
	})
	t.Run("gitlab_page_ceiling", func(t *testing.T) {
		s := newGLServer(t)
		s.mrCommitPages = -1
		got, err := s.forge().ListChangeCommits(glRepo, 7)
		if err != nil {
			t.Fatalf("ListChangeCommits: %v", err)
		}
		if got.Complete || len(s.requests) != 25 {
			t.Fatalf("GitLab at the page ceiling must read Complete=false: Complete=%v requests=%d",
				got.Complete, len(s.requests))
		}
	})
}

func TestMergeCommitSHAEmptyUnlessMerged(t *testing.T) {
	t.Run("github_open_test_merge_sha", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			b33Enc(w, map[string]any{"number": 7, "state": "open", "merged": false,
				"merge_commit_sha": "eee888", "head": map[string]any{"sha": "abc"}})
		})
		pr, err := gh.forge().GetPullRequest(forgeTestRepo, 7)
		if err != nil {
			t.Fatalf("GetPullRequest: %v", err)
		}
		if pr.MergeCommitSHA != "" {
			t.Fatalf("an OPEN change's test-merge sha must not read as a merge commit: got %q", pr.MergeCommitSHA)
		}
	})
	t.Run("github_merged", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			b33Enc(w, map[string]any{"number": 7, "state": "closed", "merged": true,
				"merge_commit_sha": "fff999", "head": map[string]any{"sha": "abc"}})
		})
		pr, err := gh.forge().GetPullRequest(forgeTestRepo, 7)
		if err != nil || pr.MergeCommitSHA != "fff999" {
			t.Fatalf("a merged change carries its merge commit: %q err=%v", pr.MergeCommitSHA, err)
		}
	})
	t.Run("gitlab_fast_forward_no_sha", func(t *testing.T) {
		s := newGLServer(t)
		s.mr = glMR(map[string]any{"state": "merged", "merge_commit_sha": nil, "squash_commit_sha": nil})
		pr, err := s.forge().GetPullRequest(glRepo, 7)
		if err != nil {
			t.Fatalf("GetPullRequest: %v", err)
		}
		if pr.MergeCommitSHA != "" {
			t.Fatalf("a fast-forward merge with no merge or squash sha must read empty: got %q", pr.MergeCommitSHA)
		}
	})
	t.Run("gitlab_open_carries_sha", func(t *testing.T) {
		s := newGLServer(t)
		s.mr = glMR(map[string]any{"merge_commit_sha": "eee888"})
		pr, err := s.forge().GetPullRequest(glRepo, 7)
		if err != nil || pr.MergeCommitSHA != "" {
			t.Fatalf("an open merge request reads no merge commit: %q err=%v", pr.MergeCommitSHA, err)
		}
	})
}

func TestAccountTypeUnresolvedStaysEmpty(t *testing.T) {
	failUsers := func(s *glServer) {
		s.forceStatus["/users/5"] = http.StatusInternalServerError
		s.forceStatus["/users/99"] = http.StatusInternalServerError
	}
	t.Run("gitlab_get_issue", func(t *testing.T) {
		s := newGLServer(t)
		failUsers(s)
		s.issue = glIssue(map[string]any{"state": "closed", "closed_by": map[string]any{"id": 99, "username": "worker-bot"}})
		s.mrMissing = true
		iss, err := s.forge().GetIssue(glRepo, 12)
		if err != nil {
			t.Fatalf("GetIssue: %v", err)
		}
		if iss.Author.Type != "" || iss.ClosedBy.Type != "" {
			t.Fatalf("an unresolvable users read must leave Type empty, never \"User\": author=%q closer=%q",
				iss.Author.Type, iss.ClosedBy.Type)
		}
		if iss.Author.ID != 5 || iss.ClosedBy.ID != 99 {
			t.Fatalf("the ids are still carried: %+v %+v", iss.Author, iss.ClosedBy)
		}
	})
	t.Run("gitlab_change_ref_author", func(t *testing.T) {
		s := newGLServer(t)
		failUsers(s)
		s.mrList = []map[string]any{glMR(nil)}
		got, err := s.forge().ListChanges(glRepo, OpenAndMerged())
		if err != nil {
			t.Fatalf("ListChanges: %v", err)
		}
		if len(got.Changes) != 1 || got.Changes[0].Author.Type != "" || got.Changes[0].Author.ID != 99 {
			t.Fatalf("ChangeRef.Author must keep Type empty when unresolved: %+v", got.Changes)
		}
	})
	t.Run("gitlab_issue_state_events", func(t *testing.T) {
		s := newGLServer(t)
		failUsers(s)
		s.stateEvents = []map[string]any{{"id": 1, "state": "closed", "created_at": "2026-09-01T10:00:00Z",
			"user": map[string]any{"id": 99, "username": "worker-bot"}}}
		s.closedBy = []map[string]any{glMR(map[string]any{"state": "merged"})}
		got, err := s.forge().IssueStateEvents(glRepo, 12)
		if err != nil {
			t.Fatalf("IssueStateEvents: %v", err)
		}
		if got.Events[0].Actor.Type != "" || got.ClosingChanges[0].Author.Type != "" {
			t.Fatalf("op 56 must keep Type empty when unresolved: %+v", got)
		}
	})
}

func TestRepoDefaultBranchEmptyRefuses(t *testing.T) {
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
		b33Enc(w, map[string]any{"default_branch": ""})
	})
	if b, err := gh.forge().RepoDefaultBranch(forgeTestRepo); !IsUnverifiable(err) || b != "" {
		t.Fatalf("GitHub: an empty default_branch must be Unverifiable, never a value: %q err=%v", b, err)
	}
	s := newGLServer(t)
	s.project = map[string]any{"id": 1, "path_with_namespace": "medici-finance/assay"}
	if b, err := s.forge().RepoDefaultBranch(glRepo); !IsUnverifiable(err) || b != "" {
		t.Fatalf("GitLab: an empty default_branch must be Unverifiable, never a value: %q err=%v", b, err)
	}
}

// b33StateEvents is a GitHub op 56 response with the given connection overflow flags and
// closing-change nodes.
func b33StateEvents(timelineNext, closedNext bool, closers []map[string]any) map[string]any {
	return map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
		"timelineItems": map[string]any{"pageInfo": map[string]any{"hasNextPage": timelineNext},
			"nodes": []map[string]any{{"__typename": "ClosedEvent", "createdAt": "2026-09-01T10:00:00Z",
				"actor": map[string]any{"login": "someone", "__typename": "User", "databaseId": 5}}}},
		"closedByPullRequestsReferences": map[string]any{"pageInfo": map[string]any{"hasNextPage": closedNext},
			"nodes": closers},
	}}}}
}

func TestIssueStateEventsOverflowIsIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		timelineNext, closedNext bool
	}{{"timeline_overflow", true, false}, {"closing_refs_overflow", false, true}} {
		t.Run("github_"+tc.name, func(t *testing.T) {
			gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
				b33Enc(w, b33StateEvents(tc.timelineNext, tc.closedNext, []map[string]any{}))
			})
			got, err := gh.forge().IssueStateEvents(forgeTestRepo, 12)
			if err != nil {
				t.Fatalf("IssueStateEvents: %v", err)
			}
			if got.Complete {
				t.Fatalf("%s: a connection with hasNextPage must read Complete=false", tc.name)
			}
		})
	}
	t.Run("gitlab_state_events_ceiling", func(t *testing.T) {
		s := newGLServer(t)
		s.stateEventPages = -1
		s.users["5"] = map[string]any{"id": 5, "username": "someone", "bot": false}
		got, err := s.forge().IssueStateEvents(glRepo, 12)
		if err != nil {
			t.Fatalf("IssueStateEvents: %v", err)
		}
		if got.Complete {
			t.Fatal("GitLab state events at the page ceiling must read Complete=false")
		}
	})
}

func TestChangedFilePatchAbsentIsStated(t *testing.T) {
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
		if r.URL.Query().Get("page") != "1" {
			b33Enc(w, []map[string]any{})
			return
		}
		b33Enc(w, []map[string]any{
			{"filename": "a.go", "status": "modified", "patch": "@@ -1 +1 @@"},
			{"filename": "big.bin", "status": "modified"},
			{"filename": "empty.txt", "status": "added", "patch": ""},
		})
	})
	files, err := gh.forge().ListChangedFiles(forgeTestRepo, 7)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}
	if len(files) != 3 || files[0].PatchAbsent || !files[1].PatchAbsent || files[1].Patch != "" || files[2].PatchAbsent {
		t.Fatalf("GitHub: only the entry with NO patch key reads PatchAbsent: %+v", files)
	}

	s := newGLServer(t)
	s.diffs = []map[string]any{
		{"old_path": "a.go", "new_path": "a.go", "diff": "@@ -1 +1 @@"},
		{"old_path": "big.bin", "new_path": "big.bin", "too_large": true, "diff": ""},
		{"old_path": "gen.go", "new_path": "gen.go", "collapsed": true, "diff": ""},
	}
	gl, err := s.forge().ListChangedFiles(glRepo, 7)
	if err != nil {
		t.Fatalf("ListChangedFiles: %v", err)
	}
	if len(gl) != 3 || gl[0].PatchAbsent || !gl[1].PatchAbsent || !gl[2].PatchAbsent {
		t.Fatalf("GitLab: too_large and collapsed entries read PatchAbsent: %+v", gl)
	}
}

func TestListIssuesServesIssuePopulation(t *testing.T) {
	t.Run("github_1200_issues_beside_1500_changes", func(t *testing.T) {
		restIssues := 0
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			if r.URL.Path != "/graphql" {
				// The REST issues list, which on this repository would serve 1,500 changes
				// alongside the issues and spend the ceiling on them.
				restIssues++
				b33Enc(w, []map[string]any{})
				return
			}
			page := 1
			if a := b33After(t, body); a != "" {
				_, _ = fmt.Sscanf(a, "c%d", &page)
				page++
			}
			b33Enc(w, b33IssuesPage(b33IssueNodes((page-1)*100+1, 100), page < 12, fmt.Sprintf("c%d", page)))
		})
		got, err := gh.forge().ListIssues(forgeTestRepo, IssueListQuery{State: IssueStateAll})
		if err != nil {
			t.Fatalf("ListIssues: %v", err)
		}
		if got.Incomplete || len(got.Issues) != 1200 {
			t.Fatalf("all 1,200 issues must come back complete: Incomplete=%v issues=%d", got.Incomplete, len(got.Issues))
		}
		for _, req := range gh.seen {
			if req != "POST /graphql" {
				t.Fatalf("every request must be the GraphQL issues connection, saw %q", req)
			}
		}
		if restIssues != 0 {
			t.Fatalf("REST /issues was read %d time(s)", restIssues)
		}
		first, second := got.Issues[0], got.Issues[1]
		if first.State != "open" || first.ClosedAt != "" || second.State != "closed" || second.ClosedAt == "" ||
			first.Author.Login != "someone" || first.Author.ID != 5 || len(first.Labels) != 1 || first.Title == "" ||
			first.CreatedAt == "" {
			t.Fatalf("each issue carries state, times, author login and id, labels and title: %+v %+v", first, second)
		}
	})
	t.Run("gitlab_1200_issues", func(t *testing.T) {
		s := newGLServer(t)
		s.issueTotal = 1200
		got, err := s.forge().ListIssues(glRepo, IssueListQuery{State: IssueStateAll})
		if err != nil {
			t.Fatalf("ListIssues: %v", err)
		}
		if got.Incomplete || len(got.Issues) != 1200 {
			t.Fatalf("all 1,200 GitLab issues must come back complete: Incomplete=%v issues=%d", got.Incomplete, len(got.Issues))
		}
		i := got.Issues[0]
		if i.State != "open" || i.ClosedAt != "" || i.Author.ID != 5 || i.Author.Login == "" || len(i.Labels) != 1 ||
			i.Title == "" || i.CreatedAt == "" {
			t.Fatalf("each GitLab issue carries the fields the issue metrics read: %+v", i)
		}
	})
}

// b33Changes is a GitHub ListChanges response with one node carrying the given fork facts.
func b33Changes(cross any, headRepo any) map[string]any {
	return map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequests": map[string]any{
		"pageInfo": map[string]any{"hasNextPage": false},
		"nodes": []map[string]any{{"number": 7, "state": "OPEN", "headRefOid": "a", "headRefName": "feat/a",
			"baseRefName": "main", "title": "t", "body": "", "isCrossRepository": cross, "headRepository": headRepo,
			"author": map[string]any{"login": "someone", "__typename": "User", "databaseId": 5}}},
	}}}}
}

func TestChangeRefCrossRepoUnreadableStaysEmpty(t *testing.T) {
	read := func(t *testing.T, payload map[string]any) ChangeRef {
		t.Helper()
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) { b33Enc(w, payload) })
		got, err := gh.forge().ListChanges(forgeTestRepo, OpenAndMerged())
		if err != nil || len(got.Changes) != 1 {
			t.Fatalf("ListChanges: %v %+v", err, got)
		}
		return got.Changes[0]
	}
	t.Run("github_null_fork", func(t *testing.T) {
		c := read(t, b33Changes(nil, nil))
		if c.CrossRepo != "" || c.HeadRepo != "" {
			t.Fatalf("the null-fork case: isCrossRepository null and headRepository null must read both "+
				"EMPTY, never %q or the base repository; got CrossRepo=%q HeadRepo=%q", CrossRepoSame, c.CrossRepo, c.HeadRepo)
		}
	})
	t.Run("github_deleted_fork", func(t *testing.T) {
		c := read(t, b33Changes(true, nil))
		if c.CrossRepo != CrossRepoFork || c.HeadRepo != "" {
			t.Fatalf("the null-fork case (deleted fork): want CrossRepo=%q HeadRepo=\"\"; got %q %q",
				CrossRepoFork, c.CrossRepo, c.HeadRepo)
		}
	})
	t.Run("github_same_repo", func(t *testing.T) {
		c := read(t, b33Changes(false, map[string]any{"nameWithOwner": "medici-finance/assay"}))
		if c.CrossRepo != CrossRepoSame || c.HeadRepo != "medici-finance/assay" {
			t.Fatalf("a same-repository change reads same + its slug: %q %q", c.CrossRepo, c.HeadRepo)
		}
	})
	gl := func(t *testing.T, src, tgt any) ChangeRef {
		t.Helper()
		s := newGLServer(t)
		mr := glMR(nil)
		if src != nil {
			mr["source_project_id"] = src
		}
		if tgt != nil {
			mr["target_project_id"] = tgt
		}
		s.mrList = []map[string]any{mr}
		got, err := s.forge().ListChanges(glRepo, OpenAndMerged())
		if err != nil || len(got.Changes) != 1 {
			t.Fatalf("ListChanges: %v %+v", err, got)
		}
		return got.Changes[0]
	}
	t.Run("gitlab_fork", func(t *testing.T) {
		c := gl(t, 2, 1)
		if c.CrossRepo != CrossRepoFork || c.HeadRepo != "" {
			t.Fatalf("the null-fork case (GitLab fork): want fork with no head repo; got %q %q", c.CrossRepo, c.HeadRepo)
		}
	})
	t.Run("gitlab_missing_id", func(t *testing.T) {
		for _, ids := range [][2]any{{nil, 1}, {1, nil}, {nil, nil}} {
			c := gl(t, ids[0], ids[1])
			if c.CrossRepo != "" || c.HeadRepo != "" {
				t.Fatalf("the null-fork case (GitLab id missing %v): want both empty; got %q %q", ids, c.CrossRepo, c.HeadRepo)
			}
		}
	})
}

func TestClosedByEmptyUnlessClosed(t *testing.T) {
	ghIssue := func(t *testing.T, payload map[string]any) *Issue {
		t.Helper()
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) { b33Enc(w, payload) })
		iss, err := gh.forge().GetIssue(forgeTestRepo, 12)
		if err != nil {
			t.Fatalf("GetIssue: %v", err)
		}
		return iss
	}
	author := map[string]any{"login": "someone", "id": 5, "type": "User"}
	closer := map[string]any{"login": "worker[bot]", "id": 99, "type": "Bot"}
	t.Run("github_reopened_issue", func(t *testing.T) {
		iss := ghIssue(t, map[string]any{"number": 12, "state": "open", "user": author, "closed_by": closer})
		if iss.ClosedBy != (Account{}) {
			t.Fatalf("the reopened-issue case: an OPEN issue's stale closed_by must read zero, got %+v", iss.ClosedBy)
		}
	})
	t.Run("github_closed_null_closer", func(t *testing.T) {
		iss := ghIssue(t, map[string]any{"number": 12, "state": "closed", "user": author, "closed_by": nil})
		if iss.ClosedBy != (Account{}) {
			t.Fatalf("a closed issue with a null closer reads zero, never its author: %+v", iss.ClosedBy)
		}
	})
	t.Run("github_closed", func(t *testing.T) {
		iss := ghIssue(t, map[string]any{"number": 12, "state": "closed", "user": author, "closed_by": closer})
		if iss.ClosedBy.ID != 99 || iss.ClosedBy.Type != "Bot" {
			t.Fatalf("a closed issue carries its closer: %+v", iss.ClosedBy)
		}
	})
	glIss := func(t *testing.T, over map[string]any) *Issue {
		t.Helper()
		s := newGLServer(t)
		s.issue = glIssue(over)
		s.mrMissing = true
		iss, err := s.forge().GetIssue(glRepo, 12)
		if err != nil {
			t.Fatalf("GetIssue: %v", err)
		}
		return iss
	}
	t.Run("gitlab_reopened_issue", func(t *testing.T) {
		iss := glIss(t, map[string]any{"state": "opened", "closed_by": map[string]any{"id": 99, "username": "worker-bot"}})
		if iss.ClosedBy != (Account{}) {
			t.Fatalf("the reopened-issue case (GitLab): an OPEN issue's stale closed_by must read zero, got %+v", iss.ClosedBy)
		}
	})
	t.Run("gitlab_closed_null_closer", func(t *testing.T) {
		iss := glIss(t, map[string]any{"state": "closed", "closed_by": nil})
		if iss.ClosedBy != (Account{}) {
			t.Fatalf("GitLab: a closed issue with a null closer reads zero, never its author: %+v", iss.ClosedBy)
		}
	})
	t.Run("gitlab_closed", func(t *testing.T) {
		iss := glIss(t, map[string]any{"state": "closed", "closed_by": map[string]any{"id": 99, "username": "worker-bot"}})
		if iss.ClosedBy.ID != 99 || iss.ClosedBy.Login != "worker-bot" {
			t.Fatalf("GitLab: a closed issue carries its closer: %+v", iss.ClosedBy)
		}
	})
}

// b33Comments is a GitHub comments-connection response on the given noteable.
func b33Comments(noteable string, nodes []map[string]any) map[string]any {
	return map[string]any{"data": map[string]any{"repository": map[string]any{noteable: map[string]any{
		"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": nodes}}}}}
}

func TestCommentUpdatedAtUnreadableStaysEmpty(t *testing.T) {
	nodes := []map[string]any{
		{"id": "IC_a", "databaseId": 1, "body": "never edited", "createdAt": "2026-09-01T10:00:00Z",
			"url": "https://example/c/1", "author": map[string]any{"login": "a", "__typename": "User", "databaseId": 5}},
		{"id": "IC_b", "databaseId": 2, "body": "edited", "createdAt": "2026-09-01T10:00:00Z",
			"updatedAt": "2026-09-02T10:00:00Z", "url": "https://example/c/2",
			"author": map[string]any{"login": "a", "__typename": "User", "databaseId": 5}},
	}
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) { b33Enc(w, b33Comments("issue", nodes)) })
	got, err := gh.forge().ListCommentsTyped(forgeTestRepo, 12, TargetIssue)
	if err != nil || len(got) != 2 {
		t.Fatalf("ListCommentsTyped: %v %+v", err, got)
	}
	if got[0].UpdatedAt != "" || got[0].CreatedAt == "" {
		t.Fatalf("the absent-update-time case (GitHub): no updatedAt must read EMPTY with CreatedAt set, "+
			"never CreatedAt; got UpdatedAt=%q CreatedAt=%q", got[0].UpdatedAt, got[0].CreatedAt)
	}
	if got[1].UpdatedAt != "2026-09-02T10:00:00Z" || got[1].UpdatedAt == got[1].CreatedAt {
		t.Fatalf("an edited comment carries its two times as they differ: %+v", got[1])
	}

	s := newGLServer(t)
	s.issueNotes = []map[string]any{
		{"id": 950, "body": "no update time", "system": false, "created_at": "2026-08-30T12:00:00Z",
			"author": map[string]any{"id": 42, "username": "worker-bot"}},
		{"id": 951, "body": "edited", "system": false, "created_at": "2026-08-30T12:00:00Z",
			"updated_at": "2026-08-31T12:00:00Z", "author": map[string]any{"id": 42, "username": "worker-bot"}},
	}
	gl, err := s.forge().ListCommentsTyped(glRepo, 12, TargetIssue)
	if err != nil || len(gl) != 2 {
		t.Fatalf("ListCommentsTyped: %v %+v", err, gl)
	}
	if gl[0].UpdatedAt != "" || gl[0].CreatedAt == "" {
		t.Fatalf("the absent-update-time case (GitLab): no updated_at must read EMPTY: %+v", gl[0])
	}
	if gl[1].UpdatedAt != "2026-08-31T12:00:00Z" {
		t.Fatalf("GitLab: an edited note carries its update time: %+v", gl[1])
	}
}

func TestIssueStateEventsMergedUnreadableIsIncomplete(t *testing.T) {
	author := map[string]any{"login": "worker", "__typename": "Bot", "databaseId": 99}
	repo := map[string]any{"nameWithOwner": "medici-finance/assay"}
	for _, tc := range []struct {
		name   string
		closer map[string]any
	}{
		{"null_merged", map[string]any{"number": 7, "state": "MERGED", "merged": nil, "repository": repo, "author": author}},
		{"merged_false_state_merged", map[string]any{"number": 7, "state": "MERGED", "merged": false, "repository": repo, "author": author}},
		{"no_state", map[string]any{"number": 7, "merged": false, "repository": repo, "author": author}},
	} {
		t.Run("github_"+tc.name, func(t *testing.T) {
			gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
				b33Enc(w, b33StateEvents(false, false, []map[string]any{tc.closer}))
			})
			got, err := gh.forge().IssueStateEvents(forgeTestRepo, 12)
			if err != nil {
				t.Fatalf("IssueStateEvents: %v", err)
			}
			if got.Complete {
				t.Fatalf("the null-merged case (%s): an unreadable merged state must clear Complete, "+
					"never read as Merged=false with Complete=true", tc.name)
			}
		})
	}
	t.Run("github_merged_closer", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			b33Enc(w, b33StateEvents(false, false, []map[string]any{
				{"number": 7, "state": "MERGED", "merged": true, "repository": repo, "author": author}}))
		})
		got, err := gh.forge().IssueStateEvents(forgeTestRepo, 12)
		if err != nil || !got.Complete || len(got.ClosingChanges) != 1 {
			t.Fatalf("IssueStateEvents: %v %+v", err, got)
		}
		c := got.ClosingChanges[0]
		if !c.Merged || c.Number != 7 || c.Repo != "medici-finance/assay" || c.Author.ID != 99 {
			t.Fatalf("a merged closer reads back with its number, repo and author: %+v", c)
		}
	})
	for _, state := range []any{nil, "weird"} {
		t.Run(fmt.Sprintf("gitlab_state_%v", state), func(t *testing.T) {
			s := newGLServer(t)
			mr := glMR(nil)
			if state == nil {
				delete(mr, "state")
			} else {
				mr["state"] = state
			}
			s.closedBy = []map[string]any{mr}
			got, err := s.forge().IssueStateEvents(glRepo, 12)
			if err != nil {
				t.Fatalf("IssueStateEvents: %v", err)
			}
			if got.Complete {
				t.Fatalf("the null-merged case (GitLab state %v): must clear Complete", state)
			}
		})
	}
}

func TestCommentFieldsOnChangeTarget(t *testing.T) {
	t.Run("github_pull_request_thread", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			if !strings.Contains(body, "pullRequest(number: $number)") {
				http.Error(w, "expected the pull-request thread query", http.StatusBadRequest)
				return
			}
			b33Enc(w, b33Comments("pullRequest", []map[string]any{
				{"id": "IC_a", "databaseId": 501, "body": "b", "createdAt": "2026-09-01T10:00:00Z",
					"updatedAt": "2026-09-02T10:00:00Z", "url": "https://example/pull/7#issuecomment-501",
					"author": map[string]any{"login": "worker", "__typename": "Bot", "databaseId": 99}}}))
		})
		got, err := gh.forge().ListCommentsTyped(forgeTestRepo, 7, TargetChange)
		if err != nil || len(got) != 1 {
			t.Fatalf("ListCommentsTyped(change): %v %+v", err, got)
		}
		c := got[0]
		if c.DatabaseID != 501 || c.URL == "" || c.UpdatedAt == "" || c.Author.Type != "Bot" {
			t.Fatalf("a change-thread comment carries DatabaseID, URL, UpdatedAt and author Type: %+v", c)
		}
	})
	t.Run("github_change_target_at_issue_number", func(t *testing.T) {
		gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, body string) {
			b33Enc(w, map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": nil}}})
		})
		got, err := gh.forge().ListCommentsTyped(forgeTestRepo, 12, TargetChange)
		if !IsUnverifiable(err) || got != nil {
			t.Fatalf("TargetChange at an issue number must be Unverifiable, never an empty thread: %v %+v", err, got)
		}
	})
	t.Run("gitlab_merge_request_thread", func(t *testing.T) {
		s := newGLServer(t)
		s.notes = []map[string]any{
			{"id": 900, "body": "note on the merge request", "system": false,
				"created_at": "2026-08-30T12:00:00Z", "updated_at": "2026-08-31T12:00:00Z",
				"author": map[string]any{"id": 42, "username": "worker-bot"}},
		}
		got, err := s.forge().ListCommentsTyped(glRepo, 7, TargetChange)
		if err != nil || len(got) != 1 {
			t.Fatalf("ListCommentsTyped(change): %v %+v", err, got)
		}
		c := got[0]
		// GitLab publishes no per-note permalink, so URL is EMPTY by the Comment.URL contract, and
		// the author's Type is not resolved on this read, so it stays EMPTY (could-not-check).
		if c.DatabaseID != 900 || c.UpdatedAt != "2026-08-31T12:00:00Z" || c.URL != "" || c.Author.Type != "" {
			t.Fatalf("a merge-request note carries DatabaseID and UpdatedAt, with URL and Type empty: %+v", c)
		}
		for _, req := range s.requests {
			if strings.Contains(req.Path, "/issues/") {
				t.Fatalf("the change thread must not read an issue: %+v", s.requests)
			}
		}
	})
}
