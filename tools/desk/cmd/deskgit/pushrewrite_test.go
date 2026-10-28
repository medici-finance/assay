package main

import (
	"errors"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #3281
func TestFetchPushRewrite(t *testing.T) {
	for _, role := range []bool{false, true} {
		t.Run(map[bool]string{false: "fetch", true: "role-fetch"}[role], func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			mustGit(t, work, "config", "remote.origin.url", githubOrigin)
			mustGit(t, work, "config", "url.https://github.com/.pushInsteadOf", "https://github.com/")
			withEnv(t, work)
			args := []string{"fetch"}
			if role {
				asWorker(t)
				args = append(args, "--as", "worker")
				roleTokenForRepo = func(gotRole, repo string, _ []string) (string, string, error) {
					if gotRole != "worker" || repo != allowedSlug {
						t.Fatalf("token bound to %s/%s", gotRole, repo)
					}
					return fixtureToken, "", nil
				}
			}
			fetchFn = func(_ string, opts gitcore.FetchOpts) error {
				fetchRecorded = append(fetchRecorded, opts)
				return errors.New("offline fixture")
			}
			if code := run(args); code != deskkit.ExitUnverifiable {
				t.Fatalf("fetch exit = %d, want offline transport failure", code)
			}
			got := onlyFetch(t)
			if got.URL != githubOrigin {
				t.Fatalf("transport URL = %q, want gated URL %q", got.URL, githubOrigin)
			}
			if role {
				auth, ok := got.Auth.(*githttp.BasicAuth)
				if !ok || auth.Password != fixtureToken {
					t.Fatal("fetch did not receive bound role token")
				}
			}
		})
	}
}
