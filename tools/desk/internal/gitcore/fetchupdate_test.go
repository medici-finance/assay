package gitcore

import (
	"errors"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// fetchupdate_test.go — how Fetch applies what it fetched (fetchstage.go).
//
// A non-forced refspec whose origin ref was rewritten to a commit that does not descend from
// the local one leaves the local ref where it is AND says so (git's "rejected (non-fast-
// forward)"); a forced one moves it. Prune never removes a symbolic ref, so a clone's
// refs/remotes/origin/HEAD survives it. And no staged ref outlives the call.

func noStagedRefs(t *testing.T, f *gittest.Fixture, when string) {
	t.Helper()
	if left := mustFixtureGit(t, f, "for-each-ref", "--format=%(refname)", stageNamespace); left != "" {
		t.Errorf("%s: staged refs left behind: %q", when, left)
	}
}

func TestFetchRefusesNonFastForward(t *testing.T) {
	for _, tc := range []struct {
		name, src, dst string
	}{
		{"branch", "refs/heads/side", "refs/heads/side"},
		{"pull", "refs/pull/7/head", "refs/heads/pr7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin := gittest.NewFixture(t)
			first := origin.CommitFile(t, "a.txt", "first\n", "first")
			mustFixtureGit(t, origin, "update-ref", tc.src, first)
			mustFixtureGit(t, origin, "reset", "-q", "--hard", "HEAD~1")
			work := cloneFixture(t, origin.Dir)
			spec := tc.src + ":" + tc.dst

			repo, err := Open(work.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{spec}}); err != nil {
				t.Fatalf("first Fetch: %v", err)
			}
			if got := mustFixtureGit(t, work, "rev-parse", tc.dst); got != first {
				t.Fatalf("%s = %s after the first fetch, want %s", tc.dst, got, first)
			}

			// The origin's ref is rewritten to a commit off main that does not descend from first.
			rewritten := origin.CommitFile(t, "b.txt", "rewritten\n", "rewritten")
			mustFixtureGit(t, origin, "update-ref", tc.src, rewritten)

			err = repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{spec}})
			if !errors.Is(err, ErrRefsNotUpdated) {
				t.Fatalf("non-forced Fetch over a rewritten origin ref = %v, want an error wrapping ErrRefsNotUpdated", err)
			}
			if got := mustFixtureGit(t, work, "rev-parse", tc.dst); got != first {
				t.Fatalf("%s moved %s -> %s on a refused non-fast-forward", tc.dst, first, got)
			}
			noStagedRefs(t, work, "after the refused fetch")

			if err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{spec}, Force: true}); err != nil {
				t.Fatalf("forced Fetch: %v", err)
			}
			if got := mustFixtureGit(t, work, "rev-parse", tc.dst); got != rewritten {
				t.Fatalf("%s = %s after a forced fetch, want %s", tc.dst, got, rewritten)
			}
			noStagedRefs(t, work, "after the forced fetch")
		})
	}
}

func TestFetchPruneKeepsSymref(t *testing.T) {
	origin := gittest.NewFixture(t)
	work := cloneFixture(t, origin.Dir)
	head := mustFixtureGit(t, work, "symbolic-ref", "refs/remotes/origin/HEAD")
	mustFixtureGit(t, work, "update-ref", "refs/remotes/origin/stale", mustFixtureGit(t, work, "rev-parse", "HEAD"))
	ahead := origin.CommitFile(t, "b.txt", "b\n", "ahead")

	repo, err := Open(work.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{"+refs/heads/*:refs/remotes/origin/*"}, Prune: true}); err != nil {
		t.Fatalf("Fetch(prune): %v", err)
	}
	if out, err := work.Git("rev-parse", "--verify", "-q", "refs/remotes/origin/stale"); err == nil {
		t.Fatalf("prune left the stale tracking ref (%s)", out)
	}
	if got := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != ahead {
		t.Fatalf("refs/remotes/origin/main = %s, want %s", got, ahead)
	}
	if got, err := work.Git("symbolic-ref", "refs/remotes/origin/HEAD"); err != nil || got != head {
		t.Fatalf("refs/remotes/origin/HEAD after prune = %q (%v), want the symbolic ref to %s kept", got, err, head)
	}
	noStagedRefs(t, work, "after the prune")
}
