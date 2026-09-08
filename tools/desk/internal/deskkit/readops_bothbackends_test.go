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
		// REST issues list (ListOpenIssues) — one issue and one PR entry; the PR is dropped.
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues") {
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
		// glRun runs the GitLab op; it must return a could-not-check error.
		glRun func() error
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
	}

	for _, c := range cases {
		t.Run(c.name+"/github", c.ghCheck)
		t.Run(c.name+"/gitlab", func(t *testing.T) {
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
