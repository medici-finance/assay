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
