package deskkit

// rolecredential_test.go — the forge-aware role-credential entry points (#1573 follow-up):
// ResolveRoleCredential selects the custody by forge, and GitHubRoleToken /
// GitHubRoleTokenForRemote refuse a repo another forge serves instead of minting a GitHub App
// token for it. Each test installs a minter DETECTOR in the real token seam, so "the GitHub
// minter never ran" is a counted fact, not an inference.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minterDetector resets the per-process memo (so an earlier test's mint cannot hide a call as
// a memo hit) and installs a fake minter that counts forks. When path is empty the fake fails
// the way a GitLab-only deployment's minter does (no App ID).
func minterDetector(t *testing.T, path string) *int {
	t.Helper()
	resetRoleTokenMemo()
	t.Cleanup(resetRoleTokenMemo)
	n := 0
	t.Cleanup(SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		n++
		if path == "" {
			return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
		}
		return path, "", nil
	}))
	return &n
}

// plantCustody writes a role's GitLab PAT custody file (0600) in a private credential dir and
// points the App-credential search path at it.
func plantCustody(t *testing.T, role, value string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gitlab-"+role+".token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigHome, dir)
	return p
}

func plantAppToken(t *testing.T, value string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worker-app-token")
	if err := os.WriteFile(p, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func rosterWithForge(t *testing.T, slug, kind string) {
	t.Helper()
	roster := goldenRoster()
	if slug != "" {
		roster[EnvRepoForges] = slug + "=" + kind
	}
	withRoster(t, roster)
}

func TestRoleCredential_GitLabReadsCustody_NoMint(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	custody := plantCustody(t, "worker", "gitlab-pat-stub")
	mints := minterDetector(t, "")

	cred, err := ResolveRoleCredential("worker", repo, "")
	if err != nil {
		t.Fatalf("ResolveRoleCredential on a GitLab-mapped repo: %v", err)
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter forked %d time(s) for a GitLab-mapped repo", *mints)
	}
	if cred.Resolution.Kind != ForgeGitLab || cred.Path != custody || cred.Token != "gitlab-pat-stub" {
		t.Fatalf("got kind=%q path=%q; want gitlab custody %s", cred.Resolution.Kind, cred.Path, custody)
	}
}

func TestRoleCredential_GitLabMissingCustody_Refused(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "gitlab-pilot"}
	rosterWithForge(t, repo.Slug(), "gitlab")
	t.Setenv(EnvConfigHome, t.TempDir()) // an empty search path: nothing provisioned
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

	_, err := ResolveRoleCredential("worker", repo, "")
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(errString(err), "gitlab-worker.token") {
		t.Fatalf("missing custody: err = %v (exit %d); want a refusal naming gitlab-worker.token", err, ExitCodeOf(err))
	}
	if *mints != 0 {
		t.Fatalf("missing GitLab custody fell through to the GitHub App minter (%d fork(s))", *mints)
	}
}

func TestRoleCredential_GitHubAndUnresolved_MintAppToken(t *testing.T) {
	for _, tc := range []struct{ name, slugForge, origin string }{
		{"roster says github", "github", ""},
		{"roster says github, origin on github.com", "github", "git@github.com:example-org/tracker.git"},
		{"origin host says github", "", "https://github.com/example-org/tracker.git"},
		{"origin host says github (scp form)", "", "git@github.com:example-org/tracker.git"},
		{"origin host says github (ssh scheme)", "", "ssh://git@github.com/example-org/tracker.git"},
		{"origin host says github (upper-case, explicit port)", "", "https://GitHub.com:443/example-org/tracker.git"},
		// With NO origin in hand (a roster-only caller), an unresolved forge keeps the historical
		// GitHub default. With an origin in hand it is refused instead — see
		// TestRoleCredential_NonGitHubOriginHost_RefusedBeforeMint.
		{"unresolved, no origin: historical GitHub default", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
			if tc.slugForge != "" {
				rosterWithForge(t, repo.Slug(), tc.slugForge)
			} else {
				rosterWithForge(t, "", "")
			}
			plantCustody(t, "worker", "gitlab-pat-stub") // present, and must NOT be read
			app := plantAppToken(t, "app-token-stub")
			mints := minterDetector(t, app)

			cred, err := ResolveRoleCredential("worker", repo, tc.origin)
			if err != nil {
				t.Fatalf("ResolveRoleCredential: %v", err)
			}
			if *mints != 1 || cred.Resolution.Kind != ForgeGitHub || cred.Path != app || cred.Token != "app-token-stub" {
				t.Fatalf("got kind=%q path=%q mints=%d; want the GitHub App token %s from one mint",
					cred.Resolution.Kind, cred.Path, *mints, app)
			}
		})
	}
}

func TestGitHubRoleToken_RefusesOtherForge_NoMint(t *testing.T) {
	// Roster-mapped GitLab repo, via the roster-only form.
	rosterWithForge(t, "example-org/gitlab-pilot", "gitlab")
	plantCustody(t, "worker", "gitlab-pat-stub")
	mints := minterDetector(t, plantAppToken(t, "app-token-stub"))
	tok, path, err := GitHubRoleToken("worker", "example-org/gitlab-pilot")
	if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
		t.Fatalf("GitHubRoleToken on a GitLab-mapped repo: tok=%q path=%q err=%v; want a refusal and nothing handed back", tok, path, err)
	}
	if strings.Contains(errString(err), "gitlab-pat-stub") {
		t.Fatalf("the refusal carries a token value: %v", err)
	}
	// Roster silent, but the origin host is unambiguously GitLab.
	tok, path, err = GitHubRoleTokenForRemote("worker", "example-org/other", "https://gitlab.com/example-org/other.git")
	if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
		t.Fatalf("GitHubRoleTokenForRemote on a gitlab.com origin: tok=%q path=%q err=%v; want a refusal", tok, path, err)
	}
	if *mints != 0 {
		t.Fatalf("the GitHub App minter forked %d time(s) for a repo another forge serves", *mints)
	}
	// And a GitHub repo still gets its App token through the same entry point.
	tok, _, err = GitHubRoleToken("worker", "example-org/tracker")
	if err != nil || tok != "app-token-stub" || *mints != 1 {
		t.Fatalf("GitHubRoleToken on an unmapped repo: tok=%q err=%v mints=%d; want the App token from one mint", tok, err, *mints)
	}
}

// nonGitHubOrigins is the host class sec-1587-S1 names: every origin whose host is not EXACTLY
// github.com, including the ones that only resemble it. A GitHub App token authenticates only to
// github.com, so each of these must be refused before the minter runs — whether the roster is
// silent (the unresolved default) or names the forge "github" (software, not instance: S2).
var nonGitHubOrigins = []struct{ name, origin string }{
	{"suffix lookalike", "https://github.com.evil.test/example-org/tracker.git"},
	{"prefix lookalike", "https://evilgithub.com/example-org/tracker.git"},
	{"subdomain", "https://git.github.com/example-org/tracker.git"},
	{"trailing dot", "https://github.com./example-org/tracker.git"},
	{"internationalized lookalike", "https://g\u0456thub.com/example-org/tracker.git"},
	{"userinfo-shaped", "https://github.com@evil.test/example-org/tracker.git"},
	{"userinfo carrying a secret", "https://someone:secret-in-origin-url@evil.test/example-org/tracker.git"},
	{"scp double-at", "git@github.com@evil.test:example-org/tracker.git"},
	{"self-hosted GitLab (https)", "https://gitlab.example.com/example-org/tracker.git"},
	{"self-hosted GitLab (scp)", "git@gitlab.example.com:example-org/tracker.git"},
	{"self-hosted GitHub", "https://github.example.com/example-org/tracker.git"},
	{"unparseable: local path", "/srv/git/example-org/tracker.git"},
	{"unparseable: scp with no user", "github.com:example-org/tracker.git"},
}

// TestRoleCredential_NonGitHubOriginHost_RefusedBeforeMint is sec-1587-S1 (and S2): with an
// origin URL in hand, the GitHub App arm of BOTH entry points is taken only for an origin whose
// parsed host is exactly github.com. Anything else is Refused (exit 5) before any mint, names
// the roster key that would map it, hands back no token or path, and never echoes the raw URL.
func TestRoleCredential_NonGitHubOriginHost_RefusedBeforeMint(t *testing.T) {
	for _, roster := range []string{"", "github"} {
		for _, tc := range nonGitHubOrigins {
			t.Run("roster="+roster+"/"+tc.name, func(t *testing.T) {
				repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
				slug := ""
				if roster != "" {
					slug = repo.Slug()
				}
				rosterWithForge(t, slug, roster)
				mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

				cred, err := ResolveRoleCredential("worker", repo, tc.origin)
				if ExitCodeOf(err) != ExitRefused || cred.Token != "" || cred.Path != "" {
					t.Fatalf("ResolveRoleCredential(origin %q): token=%q path=%q err=%v (exit %d); want a refusal (exit 5) and nothing handed back",
						tc.origin, cred.Token, cred.Path, err, ExitCodeOf(err))
				}
				checkHostRefusal(t, err)

				tok, path, err := GitHubRoleTokenForRemote("worker", repo.Slug(), tc.origin)
				if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
					t.Fatalf("GitHubRoleTokenForRemote(origin %q): token=%q path=%q err=%v (exit %d); want a refusal (exit 5) and nothing handed back",
						tc.origin, tok, path, err, ExitCodeOf(err))
				}
				checkHostRefusal(t, err)

				if *mints != 0 {
					t.Fatalf("the GitHub App minter forked %d time(s) for origin %q", *mints, tc.origin)
				}
			})
		}
	}
}

func checkHostRefusal(t *testing.T, err error) {
	t.Helper()
	msg := errString(err)
	if !strings.Contains(msg, EnvRepoForges) || !strings.Contains(msg, "github.com") {
		t.Fatalf("the refusal must name %s and the one host the token serves: %v", EnvRepoForges, err)
	}
	for _, leak := range []string{"secret-in-origin-url", "app-token-stub"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("the refusal carries %q: %v", leak, err)
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestGitHubRoleTokenForDestinations_EveryDestinationBound is sec-1587-S1 round 2 at the resolver:
// a transport that connects to SEVERAL URLs (every pushurl value, every url value) gets the
// GitHub App token only when EVERY one passes the host binding — one bad destination anywhere
// in the list, an empty list or entry, or a cleartext http:// destination is refused before any
// mint, and nothing is handed back.
func TestGitHubRoleTokenForDestinations_EveryDestinationBound(t *testing.T) {
	const good = "https://github.com/example-org/tracker.git"
	type refusal struct {
		name  string
		dests []string
	}
	var cases []refusal
	for _, tc := range nonGitHubOrigins {
		cases = append(cases,
			refusal{"bad last/" + tc.name, []string{good, tc.origin}},
			refusal{"bad first/" + tc.name, []string{tc.origin, good}})
	}
	cases = append(cases,
		refusal{"no destinations", nil},
		refusal{"empty entry", []string{good, ""}},
		refusal{"blank entry", []string{" ", good}},
		refusal{"cleartext http on github.com", []string{good, "http://github.com/example-org/tracker.git"}},
		refusal{"cleartext HTTP upper-case", []string{"HTTP://github.com/example-org/tracker.git"}},
	)
	for _, roster := range []string{"", "github"} {
		for _, tc := range cases {
			t.Run("roster="+roster+"/"+tc.name, func(t *testing.T) {
				slug := ""
				if roster != "" {
					slug = "example-org/tracker"
				}
				rosterWithForge(t, slug, roster)
				mints := minterDetector(t, plantAppToken(t, "app-token-stub"))

				tok, path, err := GitHubRoleTokenForDestinations("worker", "example-org/tracker", tc.dests)
				if ExitCodeOf(err) != ExitRefused || tok != "" || path != "" {
					t.Fatalf("GitHubRoleTokenForDestinations(%q): token=%q path=%q err=%v (exit %d); want a refusal (exit 5) and nothing handed back",
						tc.dests, tok, path, err, ExitCodeOf(err))
				}
				for _, leak := range []string{"secret-in-origin-url", "app-token-stub"} {
					if strings.Contains(errString(err), leak) {
						t.Fatalf("the refusal carries %q: %v", leak, err)
					}
				}
				if *mints != 0 {
					t.Fatalf("the GitHub App minter forked %d time(s) for destinations %q", *mints, tc.dests)
				}
			})
		}
	}

	t.Run("roster=gitlab/all destinations on github.com", func(t *testing.T) {
		rosterWithForge(t, "example-org/tracker", "gitlab")
		mints := minterDetector(t, plantAppToken(t, "app-token-stub"))
		tok, _, err := GitHubRoleTokenForDestinations("worker", "example-org/tracker", []string{good})
		if ExitCodeOf(err) != ExitRefused || tok != "" || *mints != 0 {
			t.Fatalf("a GitLab-mapped repo: token=%q err=%v mints=%d; want a refusal before any mint", tok, err, *mints)
		}
	})

	t.Run("control/every destination on github.com mints once", func(t *testing.T) {
		rosterWithForge(t, "", "")
		app := plantAppToken(t, "app-token-stub")
		mints := minterDetector(t, app)
		dests := []string{
			good,
			"https://GitHub.com:443/example-org/tracker.git",
			"git@github.com:example-org/tracker.git",
			"ssh://git@github.com/example-org/tracker.git",
		}
		tok, path, err := GitHubRoleTokenForDestinations("worker", "example-org/tracker", dests)
		if err != nil || tok != "app-token-stub" || path != app || *mints != 1 {
			t.Fatalf("all-github destinations: token=%q path=%q err=%v mints=%d; want the App token from one mint",
				tok, path, err, *mints)
		}
	})
}
