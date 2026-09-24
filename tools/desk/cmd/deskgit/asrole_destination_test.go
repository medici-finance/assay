package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// asrole_destination_test.go — sec-1587-S1, round 2: the host binding must cover every URL git
// will actually CONNECT to, not the one URL the repo gate read.
//
// `git push origin` sends to every remote.origin.pushurl value (or, with none, to every
// remote.origin.url value, pushInsteadOf applied), and both fetch and push apply
// url.<base>.insteadOf from EVERY config scope — global and worktree included. The askpass
// answers whichever host git connects to. A binding that checks only the first
// remote.origin.url as the repo's own config spells it therefore hands the GitHub App token to
// any other destination. These tests run the PRODUCTION binding with a minter detector in
// deskkit's own seam, so "refused before any mint" is a counted fact, and every git config
// shape is set with real git so git itself resolves it.

// destinationCase is one origin configuration: the git config writes that shape it.
type destinationCase struct {
	name string
	// repoConfig are `git config` argument lists applied in the checkout.
	repoConfig [][]string
	// globalConfig is written to $HOME/.gitconfig (the scope a checkout's own config never shows).
	globalConfig string
	// verbs are the deskgit verbs whose destination this shape moves.
	verbs []string
}

const githubOrigin = "https://github.com/" + allowedSlug + ".git"

// minterDetector installs a counting minter in deskkit's seam that always fails, so a case that
// passes every gate stops at the mint (exit 6) instead of contacting any host.
func minterDetector(t *testing.T) *int {
	t.Helper()
	mints := new(int)
	t.Cleanup(deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		*mints++
		return "", `no App ID for App "` + role + `-app"`, errors.New("exit status 6")
	}))
	return mints
}

// setupDestination builds a checkout on a feature branch whose origin url is on github.com and
// then applies c's config, bound to the worker role with the production token resolver.
func setupDestination(t *testing.T, c destinationCase) (string, *[][]string, *int) {
	t.Helper()
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-dest")
	calls := withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")
	mustGit(t, work, "remote", "set-url", "origin", githubOrigin)
	for _, args := range c.repoConfig {
		mustGit(t, work, append([]string{"config"}, args...)...)
	}
	if c.globalConfig != "" {
		if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(c.globalConfig), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return work, calls, minterDetector(t)
}

func TestAsRole_DestinationOffGitHub_RefusedBeforeAnyMint(t *testing.T) {
	cases := []destinationCase{
		{name: "pushurl-lookalike-host", verbs: []string{"push"},
			repoConfig: [][]string{{"remote.origin.pushurl", "https://github.com.evil.test/" + allowedSlug + ".git"}}},
		{name: "pushurl-self-hosted", verbs: []string{"push"},
			repoConfig: [][]string{{"remote.origin.pushurl", "https://gitlab.example.com/" + allowedSlug + ".git"}}},
		{name: "pushurl-multi-valued-one-bad", verbs: []string{"push"},
			repoConfig: [][]string{
				{"--add", "remote.origin.pushurl", githubOrigin},
				{"--add", "remote.origin.pushurl", "https://gitlab.example.com/" + allowedSlug + ".git"},
			}},
		{name: "pushurl-other-repo-on-github", verbs: []string{"push"},
			repoConfig: [][]string{{"remote.origin.pushurl", "https://github.com/" + deniedSlug + ".git"}}},
		{name: "pushurl-cleartext-http", verbs: []string{"push"},
			repoConfig: [][]string{{"remote.origin.pushurl", "http://github.com/" + allowedSlug + ".git"}}},
		{name: "second-url-value", verbs: []string{"push", "fetch"},
			repoConfig: [][]string{{"--add", "remote.origin.url", "https://gitlab.example.com/" + allowedSlug + ".git"}}},
		{name: "pushInsteadOf-self-hosted", verbs: []string{"push"},
			repoConfig: [][]string{{"url.https://gitlab.example.com/.pushInsteadOf", "https://github.com/"}}},
		{name: "global-insteadOf-self-hosted", verbs: []string{"push", "fetch"},
			globalConfig: "[url \"https://gitlab.example.com/\"]\n\tinsteadOf = https://github.com/\n"},
		{name: "second-insteadOf-value", verbs: []string{"push", "fetch"},
			repoConfig: [][]string{
				// git honours every insteadOf value of a url section; a reader that keeps only
				// the last one misses the rewrite that matches.
				{"--add", "url.https://gitlab.example.com/.insteadOf", "https://github.com/"},
				{"--add", "url.https://gitlab.example.com/.insteadOf", "https://nowhere.example/"},
			}},
	}
	for _, c := range cases {
		for _, verb := range c.verbs {
			t.Run(c.name+"/"+verb, func(t *testing.T) {
				_, calls, mints := setupDestination(t, c)
				if code := run([]string{verb, "--as", "worker"}); code != deskkit.ExitRefused {
					t.Fatalf("%s --as worker with %s: exit = %d, want %d (refused before any mint)",
						verb, c.name, code, deskkit.ExitRefused)
				}
				if *mints != 0 {
					t.Fatalf("%s --as worker with %s: the GitHub App minter forked %d time(s)", verb, c.name, *mints)
				}
				if gitCallWith(*calls, verb) != nil {
					t.Fatalf("git %s ran with %s although a destination is not exactly github.com", verb, c.name)
				}
			})
		}
	}
}

// The control: every destination on github.com, the repo's own slug, over https — the gate
// lets the verb through to the mint (the detector then fails it, so nothing is contacted).
func TestAsRole_DestinationsAllGitHub_ReachTheMinter(t *testing.T) {
	cases := []destinationCase{
		{name: "url-only", verbs: []string{"push", "fetch"}},
		{name: "pushurl-multi-valued-all-github", verbs: []string{"push"},
			repoConfig: [][]string{
				{"--add", "remote.origin.pushurl", githubOrigin},
				{"--add", "remote.origin.pushurl", "https://github.com:443/" + allowedSlug + ".git"},
				{"--add", "remote.origin.pushurl", "git@github.com:" + allowedSlug + ".git"},
			}},
		{name: "insteadOf-to-github-ssh", verbs: []string{"push", "fetch"},
			globalConfig: "[url \"ssh://git@github.com/\"]\n\tinsteadOf = https://github.com/\n"},
	}
	for _, c := range cases {
		for _, verb := range c.verbs {
			t.Run(c.name+"/"+verb, func(t *testing.T) {
				_, calls, mints := setupDestination(t, c)
				if code := run([]string{verb, "--as", "worker"}); code != deskkit.ExitUnverifiable {
					t.Fatalf("%s --as worker with %s: exit = %d, want %d (through the gate to the failing mint)",
						verb, c.name, code, deskkit.ExitUnverifiable)
				}
				if *mints != 1 {
					t.Fatalf("%s --as worker with %s: minter forked %d time(s), want 1", verb, c.name, *mints)
				}
				if gitCallWith(*calls, verb) != nil {
					t.Fatalf("git %s ran although the mint failed", verb)
				}
			})
		}
	}
}
