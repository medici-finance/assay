package main

import (
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fetch_prrefspec_test.go — the --pr source ref follows the forge (#5448).
//
// GitHub serves a pull request's head at refs/pull/<N>/head; GitLab serves a merge request's
// head at refs/merge-requests/<iid>/head. The forge is the one servedByGitLab resolves from the
// gated origin, the same answer that picks the --as credential arm. These tests assert the
// refspec the transport was ASKED for, through the fetchFn seam, so nothing dials out.

const (
	githubPRSpec = "refs/pull/42/head:refs/heads/pr42"
	gitlabMRSpec = "refs/merge-requests/42/head:refs/heads/pr42"
)

func onlyRefspec(t *testing.T) string {
	t.Helper()
	got := onlyFetch(t)
	if len(got.RefSpecs) != 1 {
		t.Fatalf("fetch refspecs = %q, want exactly one", got.RefSpecs)
	}
	return got.RefSpecs[0]
}

// regression: #5448
func TestFetchPRGitLabAs(t *testing.T) {
	gitlabFetchEnv(t, gitlabOrigin, "https://gitlab.example.invalid:8443/api/v4")
	if code := run([]string{"fetch", "--pr", "42", "--as", "worker"}); code != deskkit.ExitOK {
		t.Fatalf("fetch --pr 42 --as worker on a GitLab origin exit = %d, want 0", code)
	}
	if got := onlyRefspec(t); got != gitlabMRSpec {
		t.Fatalf("GitLab --pr refspec = %q, want %q", got, gitlabMRSpec)
	}
}

// Without --as the fetch is unauthenticated, but the forge is resolved the same way, so a
// GitLab-served origin is still asked for the merge-request head.
//
// regression: #5448
func TestFetchPRGitLabPlain(t *testing.T) {
	gitlabFetchEnv(t, gitlabOrigin, "https://gitlab.example.invalid:8443/api/v4")
	if code := run([]string{"fetch", "--pr", "42"}); code != deskkit.ExitOK {
		t.Fatalf("fetch --pr 42 on a GitLab origin exit = %d, want 0", code)
	}
	got := onlyFetch(t)
	if len(got.RefSpecs) != 1 || got.RefSpecs[0] != gitlabMRSpec {
		t.Fatalf("GitLab --pr refspec = %q, want %q", got.RefSpecs, gitlabMRSpec)
	}
	if got.Auth != nil {
		t.Fatal("a fetch without --as carried a credential")
	}
}

// The GitHub arm is byte-identical to before, with and without --as.
//
// regression: #5448
func TestFetchPRGitHubArm(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"as", []string{"fetch", "--pr", "42", "--as", "worker"}},
		{"plain", []string{"fetch", "--pr", "42"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			mustGit(t, work, "remote", "set-url", "origin", githubOrigin)
			withEnv(t, work)
			asWorker(t)
			fetchFn = func(_ string, opts gitcore.FetchOpts) error {
				fetchRecorded = append(fetchRecorded, opts)
				return nil
			}
			if code := run(tc.args); code != deskkit.ExitOK {
				t.Fatalf("%v on a GitHub origin exit = %d, want 0", tc.args, code)
			}
			if got := onlyRefspec(t); got != githubPRSpec {
				t.Fatalf("GitHub --pr refspec = %q, want %q", got, githubPRSpec)
			}
		})
	}
}

// The audit detail names the ref actually fetched on each arm.
//
// regression: #5448
func TestPRRefspecDetail(t *testing.T) {
	for _, tc := range []struct {
		gitlab       bool
		spec, detail string
	}{
		{false, githubPRSpec, "fetched pull/42/head -> pr42"},
		{true, gitlabMRSpec, "fetched merge-requests/42/head -> pr42"},
	} {
		spec, detail := prRefspec("42", tc.gitlab)
		if spec != tc.spec || detail != tc.detail {
			t.Errorf("prRefspec(42, gitlab=%v) = (%q, %q), want (%q, %q)", tc.gitlab, spec, detail, tc.spec, tc.detail)
		}
	}
}
