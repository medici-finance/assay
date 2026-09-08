package deskkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReadOpsBothBackends runs the four read ops the read-verbs-on-the-seam migration added — ListOpenChanges,
// ListOpenIssues, PRTrustEvents, IssueTrustEvents — under the SAME scenario name against BOTH
// backends: the GitHub backend returns the typed result off a recorded response, and the
// GitLab backend returns could-not-check-with-gap (the ops are not 1:1 there per the ruling).
// It is the read-ops twin of TestLabelOpBothBackends / TestWriteFileOpBothBackends, and it is
// the ONLY direct exercise of the GitHub implementations' happy path (the command suites drive
// them through fakes), so a decode regression surfaces here.
func TestReadOpsBothBackends(t *testing.T) {
	repo := ForgeRepo{Owner: "o", Name: "r"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/graphql") {
			body, _ := io.ReadAll(r.Body)
			q := string(body)
			switch {
			case strings.Contains(q, "pullRequests(states:OPEN"):
				io.WriteString(w, `{"data":{"repository":{"pullRequests":{"nodes":[`+
					`{"number":7,"title":"t","body":"","state":"OPEN","isDraft":true,"createdAt":"2026-01-01T00:00:00Z",`+
					`"lastEditedAt":null,"author":{"login":"botslug","__typename":"Bot"},"mergeStateStatus":"CLEAN",`+
					`"headRefOid":"abc123","headRefName":"feat/x","baseRefName":"main","labels":{"nodes":[{"name":"size:s"}]},`+
					`"commits":{"nodes":[{"commit":{"statusCheckRollup":{"contexts":{"nodes":[`+
					`{"__typename":"CheckRun","name":"ci","status":"COMPLETED","conclusion":"SUCCESS"}]}}}}]}}]}}}}`)
			case strings.Contains(q, "pullRequest(number"):
				io.WriteString(w, `{"data":{"repository":{"pullRequest":{"lastEditedAt":null,`+
					`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]},`+
					`"reviews":{"pageInfo":{"hasNextPage":false},"nodes":[]},`+
					`"reviewThreads":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`)
			default:
				io.WriteString(w, `{"data":{"repository":{"issue":{"lastEditedAt":null,`+
					`"comments":{"pageInfo":{"hasNextPage":false},"nodes":[]}}}}}`)
			}
			return
		}
		p := r.URL.Path
		// GitLab commit reads (real on GitLab per the 1:1 ruling) — checked FIRST because a
		// GitLab commit path also contains "/commits/" and would otherwise match a GitHub arm.
		if r.Method == http.MethodGet && strings.HasPrefix(p, "/api/v4/") && strings.Contains(p, "/repository/commits") {
			if strings.HasSuffix(p, "/repository/commits") {
				io.WriteString(w, `[{"id":"aaa111","committed_date":"2026-09-01T10:00:00Z"}]`)
			} else {
				io.WriteString(w, `{"id":"abc123","committed_date":"2026-09-01T10:00:00Z","author_name":"A","committer_name":"A"}`)
			}
			return
		}
		switch {
		// GitHub single-commit read (GetCommit).
		case strings.Contains(p, "/commits/"):
			io.WriteString(w, `{"sha":"abc123","author":{"login":"pusher"},"committer":{"login":"pusher"},`+
				`"commit":{"committer":{"date":"2026-09-01T10:00:00Z"}}}`)
			return
		// GitHub commit-history listing (ListRecentCommits).
		case strings.HasSuffix(p, "/commits"):
			io.WriteString(w, `[{"sha":"aaa111","commit":{"committer":{"date":"2026-09-01T10:00:00Z"}}},`+
				`{"sha":"bbb222","commit":{"committer":{"date":"2026-08-31T09:00:00Z"}}}]`)
			return
		// GitHub compare (CompareRefs).
		case strings.Contains(p, "/compare/"):
			io.WriteString(w, `{"status":"behind","ahead_by":0,"behind_by":2,"files":[{"filename":"a.go","status":"modified"}]}`)
			return
		// GitHub owner-wide search (SearchOpenChanges).
		case strings.HasSuffix(p, "/search/issues"):
			io.WriteString(w, `{"total_count":1,"incomplete_results":false,"items":[`+
				`{"number":9,"title":"open pr","created_at":"2026-01-01T00:00:00Z","repository_url":"https://api.github.com/repos/o/otherrepo"}]}`)
			return
		// GitHub workflow-directory listing (ListWorkflowFiles).
		case strings.Contains(p, "/contents/.github/workflows"):
			io.WriteString(w, `[{"name":"ci.yml","type":"file"},{"name":"README","type":"file"},{"name":"sub","type":"dir"}]`)
			return
		// GitHub raw diff (ChangeDiff) — the pulls endpoint with the diff media type.
		case strings.Contains(p, "/pulls/"):
			io.WriteString(w, "diff --git a/a.go b/a.go\n")
			return
		}
		// REST issues list (ListOpenIssues) — one issue and one PR entry; the PR is dropped.
		if r.Method == http.MethodGet && strings.Contains(p, "/issues") {
			io.WriteString(w, `[`+
				`{"number":3,"title":"an issue","user":{"login":"u","id":5},"labels":[{"name":"question"}],"created_at":"2026-01-01T00:00:00Z"},`+
				`{"number":4,"title":"a PR","user":{"login":"u","id":5},"labels":[],"created_at":"2026-01-02T00:00:00Z","pull_request":{"url":"x"}}]`)
			return
		}
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	defer srv.Close()

	gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
	gl := &GitLabForge{Token: "stub", BaseURL: srv.URL}

	cases := []struct {
		name string
		// ghCheck runs the GitHub op and asserts the typed result decoded.
		ghCheck func(t *testing.T)
		// glRun runs the GitLab op; when glCheck is nil it must return a could-not-check error
		// (the op is not 1:1 on GitLab).
		glRun func() error
		// glCheck, when set, asserts the GitLab op returns a REAL typed result — for the ops
		// this brief found genuinely 1:1 (the commit reads). Exactly one of glRun/glCheck is set.
		glCheck func(t *testing.T)
	}{
		{
			name: "ListOpenChanges",
			ghCheck: func(t *testing.T) {
				oc, err := gh.ListOpenChanges(repo)
				if err != nil {
					t.Fatalf("github ListOpenChanges: %v", err)
				}
				if len(oc.Changes) != 1 || oc.Changes[0].Number != 7 {
					t.Fatalf("github ListOpenChanges = %+v", oc)
				}
				// A GraphQL Bot actor is re-suffixed to the REST rendering the trust set expects.
				if got := oc.Changes[0].Author.Login; got != "botslug[bot]" {
					t.Errorf("bot login rendering = %q, want botslug[bot]", got)
				}
				if len(oc.Changes[0].Rollup) != 1 || oc.Changes[0].Rollup[0].Conclusion != "SUCCESS" {
					t.Errorf("rollup not decoded: %+v", oc.Changes[0].Rollup)
				}
			},
			glRun: func() error { _, err := gl.ListOpenChanges(repo); return err },
		},
		{
			name: "ListOpenIssues",
			ghCheck: func(t *testing.T) {
				iss, err := gh.ListOpenIssues(repo)
				if err != nil {
					t.Fatalf("github ListOpenIssues: %v", err)
				}
				if len(iss) != 1 || iss[0].Number != 3 { // the PR entry (#4) is dropped
					t.Fatalf("github ListOpenIssues = %+v (PRs must be filtered out)", iss)
				}
				if iss[0].Author.ID != 5 || len(iss[0].Labels) != 1 {
					t.Errorf("issue summary not decoded: %+v", iss[0])
				}
			},
			glRun: func() error { _, err := gl.ListOpenIssues(repo); return err },
		},
		{
			name: "PRTrustEvents",
			ghCheck: func(t *testing.T) {
				tp, err := gh.PRTrustEvents(repo, 7)
				if err != nil {
					t.Fatalf("github PRTrustEvents: %v", err)
				}
				if !tp.Complete {
					t.Errorf("an unoverflowed trust read must be complete: %+v", tp)
				}
			},
			glRun: func() error { _, err := gl.PRTrustEvents(repo, 7); return err },
		},
		{
			name: "IssueTrustEvents",
			ghCheck: func(t *testing.T) {
				tp, err := gh.IssueTrustEvents(repo, 3)
				if err != nil {
					t.Fatalf("github IssueTrustEvents: %v", err)
				}
				if !tp.Complete {
					t.Errorf("an unoverflowed trust read must be complete: %+v", tp)
				}
			},
			glRun: func() error { _, err := gl.IssueTrustEvents(repo, 3); return err },
		},
		{
			name: "ListRecentCommits",
			ghCheck: func(t *testing.T) {
				cs, err := gh.ListRecentCommits(repo, 5)
				if err != nil {
					t.Fatalf("github ListRecentCommits: %v", err)
				}
				if len(cs) != 2 || cs[0].SHA != "aaa111" || cs[0].CommittedDate == "" {
					t.Fatalf("github ListRecentCommits = %+v", cs)
				}
			},
			// 1:1 on GitLab — a REAL result, not could-not-check.
			glCheck: func(t *testing.T) {
				cs, err := gl.ListRecentCommits(repo, 5)
				if err != nil {
					t.Fatalf("gitlab ListRecentCommits: %v", err)
				}
				if len(cs) != 1 || cs[0].SHA != "aaa111" {
					t.Fatalf("gitlab ListRecentCommits = %+v", cs)
				}
			},
		},
		{
			name: "GetCommit",
			ghCheck: func(t *testing.T) {
				c, err := gh.GetCommit(repo, "abc123")
				if err != nil {
					t.Fatalf("github GetCommit: %v", err)
				}
				if c.SHA != "abc123" || c.CommittedDate == "" || c.CommitterLogin != "pusher" {
					t.Fatalf("github GetCommit = %+v", c)
				}
			},
			// 1:1 on GitLab for the date; the account-login field is a per-field could-not-check
			// (EMPTY) because GitLab commits carry only raw git identity.
			glCheck: func(t *testing.T) {
				c, err := gl.GetCommit(repo, "abc123")
				if err != nil {
					t.Fatalf("gitlab GetCommit: %v", err)
				}
				if c.SHA != "abc123" || c.CommittedDate == "" {
					t.Fatalf("gitlab GetCommit = %+v", c)
				}
				if c.AuthorLogin != "" || c.CommitterLogin != "" {
					t.Errorf("gitlab GetCommit must leave account logins EMPTY (no resolved account), got %+v", c)
				}
			},
		},
		{
			name: "CompareRefs",
			ghCheck: func(t *testing.T) {
				rc, err := gh.CompareRefs(repo, "main", "abc123")
				if err != nil {
					t.Fatalf("github CompareRefs: %v", err)
				}
				if rc.BehindBy != 2 || rc.Status != "behind" || len(rc.Files) != 1 {
					t.Fatalf("github CompareRefs = %+v", rc)
				}
			},
			glRun: func() error { _, err := gl.CompareRefs(repo, "main", "abc123"); return err },
		},
		{
			name: "SearchOpenChanges",
			ghCheck: func(t *testing.T) {
				res, err := gh.SearchOpenChanges("o")
				if err != nil {
					t.Fatalf("github SearchOpenChanges: %v", err)
				}
				if len(res.Results) != 1 || res.Results[0].Repo != "o/otherrepo" || res.Results[0].Number != 9 {
					t.Fatalf("github SearchOpenChanges = %+v", res)
				}
			},
			glRun: func() error { _, err := gl.SearchOpenChanges("o"); return err },
		},
		{
			name: "ListWorkflowFiles",
			ghCheck: func(t *testing.T) {
				names, err := gh.ListWorkflowFiles(repo, "abc123")
				if err != nil {
					t.Fatalf("github ListWorkflowFiles: %v", err)
				}
				if len(names) != 1 || names[0] != "ci.yml" { // README + dir dropped
					t.Fatalf("github ListWorkflowFiles = %+v", names)
				}
			},
			glRun: func() error { _, err := gl.ListWorkflowFiles(repo, "abc123"); return err },
		},
		{
			name: "ChangeDiff",
			ghCheck: func(t *testing.T) {
				d, err := gh.ChangeDiff(repo, 7)
				if err != nil {
					t.Fatalf("github ChangeDiff: %v", err)
				}
				if !strings.HasPrefix(d, "diff --git") {
					t.Fatalf("github ChangeDiff = %q", d)
				}
			},
			glRun: func() error { _, err := gl.ChangeDiff(repo, 7); return err },
		},
	}

	for _, c := range cases {
		t.Run(c.name+"/github", c.ghCheck)
		t.Run(c.name+"/gitlab", func(t *testing.T) {
			if c.glCheck != nil {
				c.glCheck(t)
				return
			}
			err := c.glRun()
			if err == nil {
				t.Fatalf("gitlab %s must return could-not-check, got nil", c.name)
			}
			if !strings.Contains(err.Error(), "could-not-check") {
				t.Errorf("gitlab %s error must name could-not-check (the gap), got: %v", c.name, err)
			}
		})
	}
}
