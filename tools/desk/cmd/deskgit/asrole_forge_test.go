package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// asrole_forge_test.go — `--as` on a repo another forge serves (#1573 follow-up).
//
// The authenticated transport answers git's Username prompt with the GitHub App-token username,
// so it speaks only GitHub. Before the follow-up its token seam was bound straight to the GitHub
// App minter with no forge question in front of it: on a GitLab-served origin it asked for a
// GitHub App (or, with one installed on a same-named account, minted a GitHub token and offered
// it to the GitLab host). The PRODUCTION binding is exercised here — no asWorker stub — with a
// minter DETECTOR in deskkit's own seam, so "nothing was minted" is a counted fact.

func TestAsRole_GitLabServedOrigin_RefusedBeforeAnyMint(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-gl")
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	roster := fixtureRoster + "ASSAY_REPO_FORGES=" + allowedSlug + "=gitlab\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()

	mints := 0
	t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		mints++
		return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
	}))

	for _, verb := range []string{"push", "fetch"} {
		if code := run([]string{verb, "--as", "worker"}); code != deskkit.ExitRefused {
			t.Fatalf("%s --as worker on a GitLab-served origin exit = %d, want %d (refused before any token)",
				verb, code, deskkit.ExitRefused)
		}
		if gitCallWith(*calls, verb) != nil {
			t.Fatalf("git %s ran although no GitHub credential applies to a GitLab-served origin", verb)
		}
	}
	if mints != 0 {
		t.Fatalf("the GitHub App minter forked %d time(s) for a GitLab-served origin", mints)
	}
}

// TestAsRole_NonGitHubOriginHost_RefusedBeforeAnyMint is sec-1587-S1/S2 at the deskgit binding:
// the token is offered for the ORIGIN, and parseRepo gates only its owner/repo path, never its
// host — so an origin whose host is not exactly github.com (a lookalike, a userinfo-shaped URL,
// a self-hosted instance) must be refused before any token is minted or offered to it, with the
// roster silent AND with the roster naming the forge "github" (software, not instance). The
// PRODUCTION binding runs, with a minter detector in deskkit's own seam.
func TestAsRole_NonGitHubOriginHost_RefusedBeforeAnyMint(t *testing.T) {
	origins := []string{
		"https://github.com.evil.test/" + allowedSlug + ".git",
		"https://github.com@evil.test/" + allowedSlug + ".git",
		"https://gitlab.example.com/" + allowedSlug + ".git",
		"git@github.example.com:" + allowedSlug + ".git",
	}
	for _, rosterForge := range []string{"", "github"} {
		for _, origin := range origins {
			t.Run("roster="+rosterForge+"/"+origin, func(t *testing.T) {
				work := newRepo(t, allowedSlug)
				onBranch(t, work, "feature-host")
				calls := withEnv(t, work)
				t.Setenv("DESK_LOOP", "worker-desk")
				roster := fixtureRoster
				if rosterForge != "" {
					roster += "ASSAY_REPO_FORGES=" + allowedSlug + "=" + rosterForge + "\n"
				}
				if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".config", "assay", "roster.env"), []byte(roster), 0o600); err != nil {
					t.Fatal(err)
				}
				deskkit.ReloadConfig()
				mustGit(t, work, "remote", "set-url", "origin", origin)

				mints := 0
				t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
					mints++
					return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
				}))

				for _, verb := range []string{"push", "fetch"} {
					if code := run([]string{verb, "--as", "worker"}); code != deskkit.ExitRefused {
						t.Fatalf("%s --as worker on origin %s exit = %d, want %d (refused before any token)",
							verb, origin, code, deskkit.ExitRefused)
					}
					if gitCallWith(*calls, verb) != nil {
						t.Fatalf("git %s ran against %s although a GitHub App token serves only github.com", verb, origin)
					}
				}
				if mints != 0 {
					t.Fatalf("the GitHub App minter forked %d time(s) for origin %s", mints, origin)
				}
			})
		}
	}
}
