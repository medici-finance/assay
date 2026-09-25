package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// monitor_forge_test.go — the poller's read identity on a mixed-forge scope (#1573 follow-up).
//
// The poller reads GitHub. ResolveMonitorIdentity used to mint a GitHub App token for EVERY owner
// in scope with no forge question in front of it, so a GitLab-served owner asked for a GitHub
// App. With the production mint binding (no stub), a GitLab-mapped owner must become a NAMED
// keyring fallback that says which forge serves it — with no mint and no token file handed to a
// GitHub poller — while a GitHub owner keeps its token file.

func TestMonitorIdentity_GitLabOwner_NamedFallbackNoMint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(deskkit.EnvConfigHome, "")
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	roster := fixtureRoster + "ASSAY_REPO_FORGES=example-gl/pilot=gitlab\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0o600); err != nil {
		t.Fatal(err)
	}
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)

	appToken := filepath.Join(t.TempDir(), "intake-loop-app-token")
	if err := os.WriteFile(appToken, []byte("x-example-installation-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var minted []string
	t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		minted = append(minted, owner)
		if owner == "example-gh" {
			return appToken, "", nil
		}
		return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
	}))
	// Production mint binding; only the role is stubbed (no DESK_LOOP in a test process).
	stubIdentity(t, func(string) (string, string, error) { return "intake-loop", "intake-desk", nil }, mintTokenFn)

	id := ResolveMonitorIdentity([]string{"example-gh/tracker", "example-gl/pilot"})
	for _, o := range minted {
		if o == "example-gl" {
			t.Fatalf("the GitHub App minter ran for the GitLab-served owner (minted for %q)", minted)
		}
	}
	if _, ok := id.TokenFiles["example-gl"]; ok {
		t.Fatalf("a token file was handed to the GitHub poller for a GitLab-served owner: %v", id.TokenFiles)
	}
	if why := id.Fallbacks["example-gl"]; !strings.Contains(why, "gitlab") {
		t.Fatalf("the GitLab-served owner's fallback does not name its forge: %q", why)
	}
	if id.TokenFiles["example-gh"] != appToken {
		t.Fatalf("the GitHub owner lost its token file: %v (fallbacks %v)", id.TokenFiles, id.Fallbacks)
	}
}
