package main

import (
	"errors"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// regression: #3207
func TestFetchWithPushURL(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "checkout", true: "linked"}[linked], func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			wantURL := originURL(t, work)
			pushURL := newDeniedUpstream(t)
			mustGit(t, work, "config", "remote.origin.pushurl", pushURL)
			if linked {
				work = newLinkedWorktree(t, work)
			}
			mustGit(t, work, "update-ref", "-d", "refs/remotes/origin/main")
			withEnv(t, work)
			if code := run([]string{"fetch"}); code != deskkit.ExitOK {
				t.Fatalf("fetch exit = %d, want 0", code)
			}
			if got := onlyFetch(t); got.URL != wantURL {
				t.Fatalf("fetch URL = %q, want %q", got.URL, wantURL)
			}
			if got := mustGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != mustGit(t, work, "rev-parse", "HEAD") {
				t.Fatal("fetch did not restore the upstream ref")
			}
			if got := mustGit(t, work, "config", "--get-all", "remote.origin.pushurl"); got != pushURL {
				t.Fatal("fetch changed pushurl")
			}
		})
	}
}

// regression: #3207
func TestFetchPushOnly(t *testing.T) {
	work := newRepo(t, allowedSlug)
	pushURL := originURL(t, work)
	mustGit(t, work, "config", "--unset-all", "remote.origin.url")
	mustGit(t, work, "config", "remote.origin.pushurl", pushURL)
	calls := withEnv(t, work)
	if code := run([]string{"fetch"}); code == deskkit.ExitOK {
		t.Fatal("push-only origin was accepted as a fetch URL")
	}
	if fetchArgv(*calls) != nil {
		t.Fatal("push-only origin reached fetch transport")
	}
}

// regression: #3207
func TestFetchRoleWithPushURL(t *testing.T) {
	work := newRepo(t, allowedSlug)
	mustGit(t, work, "config", "remote.origin.url", githubOrigin)
	mustGit(t, work, "config", "remote.origin.pushurl", "https://elsewhere.invalid/other/repo.git")
	withEnv(t, work)
	asWorker(t)
	roleTokenForRepo = func(role, repo string, _ []string) (string, string, error) {
		if role != "worker" || repo != allowedSlug {
			t.Fatalf("token bound to %s/%s", role, repo)
		}
		return fixtureToken, "", nil
	}
	fetchFn = func(_ string, opts gitcore.FetchOpts) error {
		fetchRecorded = append(fetchRecorded, opts)
		return errors.New("offline")
	}
	if code := run([]string{"fetch", "--as", "worker"}); code != deskkit.ExitUnverifiable {
		t.Fatalf("fetch exit = %d, want offline transport error", code)
	}
	got := onlyFetch(t)
	if got.URL != githubOrigin {
		t.Fatalf("fetch URL = %q, want fetch destination", got.URL)
	}
	auth, ok := got.Auth.(*githttp.BasicAuth)
	if !ok || auth.Password != fixtureToken {
		t.Fatal("fetch did not receive the role credential")
	}
}

// regression: #3281
func TestFetchWithPushInsteadOf(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "checkout", true: "linked"}[linked], func(t *testing.T) {
			work := newRepo(t, allowedSlug)
			wantURL := originURL(t, work)
			mustGit(t, work, "config", "url."+wantURL+".pushInsteadOf", wantURL)
			if linked {
				work = newLinkedWorktree(t, work)
			}
			mustGit(t, work, "update-ref", "-d", "refs/remotes/origin/main")
			withEnv(t, work)
			if code := run([]string{"fetch"}); code != deskkit.ExitOK {
				t.Fatalf("fetch exit = %d, want 0", code)
			}
			if got := onlyFetch(t); got.URL != wantURL {
				t.Fatalf("fetch URL = %q, want %q", got.URL, wantURL)
			}
			if got := mustGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != mustGit(t, work, "rev-parse", "HEAD") {
				t.Fatal("fetch did not restore the upstream ref")
			}
		})
	}
}

