package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// claimRepoURL swaps only origin's trailing owner/name: scheme, host or SSH alias, port,
// userinfo and a ".git" suffix stay, so git applies origin's rewrite rules and credentials.
// regression: #7490
func TestClaimRepoURL(t *testing.T) {
	for _, tc := range []struct{ origin, want string }{
		{"https://forge.example/acme/deliver.git", "https://forge.example/other-org/tracking.git"},
		{"https://forge.example/acme/deliver", "https://forge.example/other-org/tracking"},
		{"https://forge.example/acme/deliver/", "https://forge.example/other-org/tracking"},
		{"git@github.com:acme/deliver.git", "git@github.com:other-org/tracking.git"},
		{"github.com:acme/deliver.git", "github.com:other-org/tracking.git"}, // scp form with no user, as an alias is written
		{"ssh://git@ssh.github.com:443/acme/deliver.git", "ssh://git@ssh.github.com:443/other-org/tracking.git"},
		{"/srv/git/acme/deliver.git", "/srv/git/other-org/tracking.git"},
	} {
		got, err := claimRepoURL(tc.origin, "other-org/tracking")
		if err != nil || got != tc.want {
			t.Errorf("claimRepoURL(%q) = %q, %v; want %q", tc.origin, got, err, tc.want)
		}
	}
	for _, tc := range []struct{ origin, repo string }{
		{"https://forge.example/acme/deliver.git", "../tracking"},
		{"https://forge.example/acme/deliver.git", "other-org"},
		{"https://forge.example/acme/deliver.git", "a/b/c"},
		{"acme/deliver", "other-org/tracking"}, // no host part to keep
		{"https://forge.example/", "other-org/tracking"},
	} {
		if got, err := claimRepoURL(tc.origin, tc.repo); err == nil {
			t.Errorf("claimRepoURL(%q, %q) = %q, want an error", tc.origin, tc.repo, got)
		}
	}
	// A credential in origin's URL never reaches the error text.
	if _, err := claimRepoURL("https://user:<token>@forge.example/", "other-org/tracking"); err == nil || strings.Contains(err.Error(), "<token>") {
		t.Errorf("err = %v; want an error that does not echo the userinfo", err)
	}
}

// Both reads of a cross-repository claim, the verdict and the re-read just before the delete,
// list the tracking repository, never this checkout's origin.
// regression: #7490
func TestCrossRepoRereadsClaimRepo(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	target := crossRepoHome(t, work, "xreread", "item-x5")
	old := listDispatchClaims
	var remotes []string
	listDispatchClaims = func(_, remote string) (map[string]bool, error) {
		remotes = append(remotes, remote)
		return map[string]bool{}, nil
	}
	t.Cleanup(func() { listDispatchClaims = old })

	rc, errout := runCapErr(t, []string{"remove", target})
	if rc != deskkit.ExitOK {
		t.Fatalf("remove rc = %d, want 0; stderr:\n%s", rc, errout)
	}
	if len(remotes) != 2 {
		t.Fatalf("listings = %q, want the verdict and the re-read", remotes)
	}
	for _, r := range remotes {
		if !strings.HasSuffix(r, "/other-org/tracking.git") {
			t.Fatalf("listings = %q, want every read in other-org/tracking", remotes)
		}
	}
}
