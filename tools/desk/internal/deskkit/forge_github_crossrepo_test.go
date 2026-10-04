package deskkit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// forge_github_crossrepo_test.go — the forge-side pin for PullRequest.CrossRepo on GitHub.
//
// CrossRepo is what a merge tool's fork refusal reads, and on GitHub it is derived HERE from the
// head and base repositories the pull object reports (it used to be the forge's own
// isCrossRepository flag). A derivation nothing exercises can invert unnoticed, so this serves
// real pull objects through GetPullRequest and holds each shape to its answer:
//
//   - a head repository that differs from the base is a fork;
//   - the same repository under different letter case is the same (GitHub names are
//     case-insensitive);
//   - a head repository that is absent (a deleted fork is null on the wire), a base that is
//     absent, or a repository with no full_name is UNREPORTED — empty, never "same", because a
//     consumer that reads empty as could-not-check must not be handed a branch it cannot push to
//     as though it were in the base repository.
//
// FAIL-FIRST: with the final `return CrossRepoFork` in ghCrossRepo changed to CrossRepoSame, the
// fork case fails (got "same", want "fork").

func pullObject(headRepo, baseRepo string) string {
	return `{"number":7,"state":"open","draft":true,"merged":false,` +
		`"head":{"ref":"feat/x","sha":"1111111111111111111111111111111111111111","repo":` + headRepo + `},` +
		`"base":{"ref":"main","sha":"2222222222222222222222222222222222222222","repo":` + baseRepo + `},` +
		`"user":{"login":"someone","id":42}}`
}

func TestGitHubPullCrossRepo(t *testing.T) {
	repo := func(name string) string { return `{"full_name":"` + name + `"}` }
	cases := []struct {
		name       string
		head, base string
		want       string
	}{
		{"same repository", repo("o/r"), repo("o/r"), CrossRepoSame},
		{"same repository, different letter case", repo("O/R"), repo("o/r"), CrossRepoSame},
		{"head in another owner's fork", repo("forker/r"), repo("o/r"), CrossRepoFork},
		{"head in a differently named repository", repo("o/r-fork"), repo("o/r"), CrossRepoFork},
		{"head repository deleted (null)", `null`, repo("o/r"), ""},
		{"head repository with no full_name", `{}`, repo("o/r"), ""},
		{"base repository absent (null)", repo("o/r"), `null`, ""},
		{"base repository with no full_name", repo("o/r"), `{"full_name":""}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(pullObject(tc.head, tc.base)))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			pr, err := gh.GetPullRequest(ForgeRepo{Owner: "o", Name: "r"}, 7)
			if err != nil {
				t.Fatalf("GetPullRequest: %v", err)
			}
			if pr.CrossRepo != tc.want {
				t.Fatalf("CrossRepo = %q, want %q", pr.CrossRepo, tc.want)
			}
		})
	}
}

// TestGitHubPullMergedFlag — State is the forge's open/closed word and Merged is its own flag;
// the mapping must carry the flag (and the timestamp), because a consumer names the refusal
// reason from them.
func TestGitHubPullMergedFlag(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		merged      bool
		mergedAt    string
	}{
		{"merged flag", `"merged":true,"merged_at":"2026-10-01T00:00:00Z"`, true, "2026-10-01T00:00:00Z"},
		{"closed unmerged", `"merged":false`, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"number":7,"state":"closed",` + tc.extra + `}`))
			}))
			defer srv.Close()
			gh := &GitHubForge{Token: "stub", BaseURL: srv.URL}
			pr, err := gh.GetPullRequest(ForgeRepo{Owner: "o", Name: "r"}, 7)
			if err != nil {
				t.Fatalf("GetPullRequest: %v", err)
			}
			if pr.Merged != tc.merged || pr.MergedAt != tc.mergedAt {
				t.Fatalf("Merged=%v MergedAt=%q, want %v %q", pr.Merged, pr.MergedAt, tc.merged, tc.mergedAt)
			}
		})
	}
}
