package deskkit

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// forge_gitlab_crossrepo_test.go — the forge-side pin for PullRequest.CrossRepo on GitLab, the
// twin of forge_github_crossrepo_test.go.
//
// CrossRepo is what a merge tool's fork refusal reads. On GitLab it is derived in GetPullRequest
// from the merge request's own source_project_id and target_project_id: a fork merge request's
// source project is not its target. A derivation nothing exercises can invert unnoticed, so this
// serves real merge-request objects through GetPullRequest and holds each shape to its answer:
//
//   - differing source and target project ids are a fork;
//   - equal ids are the same project;
//   - a zero id on either side is UNREPORTED — empty, never "same", because a consumer that reads
//     empty as could-not-check must not be handed a branch it cannot push to as though it were in
//     the target project.
//
// FAIL-FIRST: with `if mr.SourceProjectID != mr.TargetProjectID {` in GetPullRequest changed to
// `if false {`, the fork case fails (got "same", want "fork"). forge-gitlab-mutations.json carries
// that inversion as a mutation entry.

func mergeRequestObject(sourceID, targetID int) string {
	return fmt.Sprintf(`{"id":900,"iid":7,"state":"opened","draft":true,`+
		`"title":"t","description":"","changes_count":"1",`+
		`"sha":"1111111111111111111111111111111111111111",`+
		`"source_branch":"feat/x","target_branch":"main",`+
		`"source_project_id":%d,"target_project_id":%d,`+
		`"author":{"id":42,"username":"someone"}}`, sourceID, targetID)
}

func TestGitLabPullCrossRepo(t *testing.T) {
	cases := []struct {
		name           string
		source, target int
		want           string
	}{
		{"same project", 11, 11, CrossRepoSame},
		{"source project is a fork of the target", 12, 11, CrossRepoFork},
		{"source project id unreported (zero)", 0, 11, ""},
		{"target project id unreported (zero)", 11, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(mergeRequestObject(tc.source, tc.target)))
			}))
			defer srv.Close()
			f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
			pr, err := f.GetPullRequest(glRepo, 7)
			if err != nil {
				t.Fatalf("GetPullRequest: %v", err)
			}
			if pr.CrossRepo != tc.want {
				t.Fatalf("source %d, target %d: CrossRepo = %q, want %q",
					tc.source, tc.target, pr.CrossRepo, tc.want)
			}
		})
	}
}
