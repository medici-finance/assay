package gitcore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

const trackingSpec = "+refs/heads/*:refs/remotes/origin/*"

// skipIfDirModeIgnored skips a test that makes a ref directory read-only to force a write or
// remove to fail: root ignores the mode, and Windows has no such directory mode.
func skipIfDirModeIgnored(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only directory does not stop writes here")
	}
}

func openFixture(t *testing.T, f *gittest.Fixture) *Repo {
	t.Helper()
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// A non-forced refspec never moves an existing tag, even to a descendant: git refuses
// ("would clobber existing tag") unless the refspec is forced.
func TestFetchRefusesToMoveTagWithoutForce(t *testing.T) {
	origin := gittest.NewFixture(t)
	mustFixtureGit(t, origin, "tag", "v1")
	work := cloneFixture(t, origin.Dir)
	was := mustFixtureGit(t, work, "rev-parse", "refs/tags/v1")
	moved := origin.CommitFile(t, "b.txt", "b\n", "descendant")
	mustFixtureGit(t, origin, "tag", "-f", "v1")
	repo := openFixture(t, work)

	err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{"refs/tags/v1:refs/tags/v1"}})
	if !errors.Is(err, ErrRefsNotUpdated) {
		t.Fatalf("non-forced Fetch over a moved tag = %v, want an error wrapping ErrRefsNotUpdated", err)
	}
	if got := mustFixtureGit(t, work, "rev-parse", "refs/tags/v1"); got != was {
		t.Fatalf("refs/tags/v1 moved %s -> %s without force", was, got)
	}
	if err := repo.Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{"+refs/tags/v1:refs/tags/v1"}}); err != nil {
		t.Fatalf("forced Fetch: %v", err)
	}
	if got := mustFixtureGit(t, work, "rev-parse", "refs/tags/v1"); got != moved {
		t.Fatalf("refs/tags/v1 = %s after a forced fetch, want %s", got, moved)
	}
}

// One destination that cannot be written does not stop the others: it is reported, they land.
func TestFetchUnwritableDestinationDoesNotStopTheRest(t *testing.T) {
	skipIfDirModeIgnored(t)
	origin := gittest.NewFixture(t)
	work := cloneFixture(t, origin.Dir)
	ahead := origin.CommitFile(t, "b.txt", "b\n", "ahead")
	locked := filepath.Join(work.Dir, ".git", "refs", "aaa")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	// refs/aaa/main sorts first and cannot be written; refs/remotes/origin/main must still land.
	err := openFixture(t, work).Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{
		"+refs/heads/main:refs/aaa/main", trackingSpec,
	}})
	if !errors.Is(err, ErrRefsNotUpdated) || !strings.Contains(err.Error(), "refs/aaa/main") {
		t.Fatalf("Fetch with an unwritable destination = %v, want ErrRefsNotUpdated naming refs/aaa/main", err)
	}
	if got := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != ahead {
		t.Fatalf("refs/remotes/origin/main = %s, want %s: the unwritable destination stopped the rest", got, ahead)
	}
	noStagedRefs(t, work, "after the partial fetch")
}

// A stale ref that cannot be pruned is reported, and the updates still land.
func TestFetchUnprunableRefDoesNotStopTheUpdates(t *testing.T) {
	skipIfDirModeIgnored(t)
	origin := gittest.NewFixture(t)
	work := cloneFixture(t, origin.Dir)
	mustFixtureGit(t, work, "update-ref", "refs/remotes/origin/gone/x", mustFixtureGit(t, work, "rev-parse", "HEAD"))
	ahead := origin.CommitFile(t, "b.txt", "b\n", "ahead")
	locked := filepath.Join(work.Dir, ".git", "refs", "remotes", "origin", "gone")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	err := openFixture(t, work).Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{trackingSpec}, Prune: true})
	if !errors.Is(err, ErrRefsNotUpdated) || !strings.Contains(err.Error(), "refs/remotes/origin/gone/x") {
		t.Fatalf("Fetch(prune) with an unprunable ref = %v, want ErrRefsNotUpdated naming it", err)
	}
	if got := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/main"); got != ahead {
		t.Fatalf("refs/remotes/origin/main = %s, want %s: the failed prune stopped the updates", got, ahead)
	}
}

// Prune removes the directories it empties, as git does (never refs/remotes/ itself), and an
// empty directory left where a new ref goes is cleared before the write, as git does.
func TestFetchPruneAndWriteClearEmptyDirectories(t *testing.T) {
	origin := gittest.NewFixture(t)
	mustFixtureGit(t, origin, "branch", "feat")
	work := cloneFixture(t, origin.Dir)
	refs := filepath.Join(work.Dir, ".git", "refs")
	mustFixtureGit(t, work, "update-ref", "-d", "refs/remotes/origin/feat")
	mustFixtureGit(t, work, "pack-refs", "--all")
	mustFixtureGit(t, work, "update-ref", "refs/remotes/stale/gone/x", mustFixtureGit(t, work, "rev-parse", "HEAD"))
	if err := os.MkdirAll(filepath.Join(refs, "remotes", "origin", "feat", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := openFixture(t, work).Fetch(FetchOpts{URL: origin.Dir, Prune: true, RefSpecs: []string{
		trackingSpec, "+refs/heads/*:refs/remotes/stale/*",
	}})
	if err != nil {
		t.Fatalf("Fetch(prune): %v", err)
	}
	if got, want := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/feat"), mustFixtureGit(t, origin, "rev-parse", "feat"); got != want {
		t.Fatalf("refs/remotes/origin/feat = %s, want %s: the empty directory in its place was not cleared", got, want)
	}
	if _, err := os.Stat(filepath.Join(refs, "remotes", "stale", "gone")); !os.IsNotExist(err) {
		t.Fatalf("prune left the emptied directory refs/remotes/stale/gone (%v)", err)
	}
	if _, err := os.Stat(filepath.Join(refs, "remotes")); err != nil {
		t.Fatalf("refs/remotes is gone: %v", err)
	}
}

// Prune removes a pruned ref's reflog and the log directories that leaves empty, as git does.
// A reflog left behind conflicts with the reflog of a later ref named X/a (or X), and the git
// binary's own fetch in that checkout then fails to update that ref on every run.
func TestFetchPruneRemovesReflog(t *testing.T) {
	for _, names := range [][2]string{{"x", "x/a"}, {"y/a", "y"}} {
		old, nu := names[0], names[1]
		t.Run(strings.ReplaceAll(old+"->"+nu, "/", "_"), func(t *testing.T) {
			origin := gittest.NewFixture(t)
			work := cloneFixture(t, origin.Dir)
			mustFixtureGit(t, origin, "branch", old)
			mustFixtureGit(t, work, "fetch", "origin") // a fetch logs the tracking ref it creates
			logs := filepath.Join(work.Dir, ".git", "logs", "refs", "remotes", "origin")
			oldLog := filepath.Join(logs, filepath.FromSlash(old))
			if _, err := os.Stat(oldLog); err != nil {
				t.Fatalf("fixture: git fetch wrote no reflog for origin/%s: %v", old, err)
			}
			mustFixtureGit(t, origin, "branch", "-D", old)

			err := openFixture(t, work).Fetch(FetchOpts{URL: origin.Dir, Prune: true, RefSpecs: []string{trackingSpec}})
			if err != nil {
				t.Fatalf("Fetch(prune): %v", err)
			}
			if _, err := os.Lstat(oldLog); !os.IsNotExist(err) {
				t.Fatalf("prune left the reflog of refs/remotes/origin/%s (%v)", old, err)
			}
			if dir, _, nested := strings.Cut(old, "/"); nested {
				if _, err := os.Lstat(filepath.Join(logs, dir)); !os.IsNotExist(err) {
					t.Fatalf("prune left the emptied log directory %s (%v)", dir, err)
				}
			}

			mustFixtureGit(t, origin, "branch", nu)
			if out, err := work.Git("fetch", "origin"); err != nil {
				t.Fatalf("git fetch origin after the prune: %v: %s", err, out)
			}
			if got, want := mustFixtureGit(t, work, "rev-parse", "refs/remotes/origin/"+nu), mustFixtureGit(t, origin, "rev-parse", nu); got != want {
				t.Fatalf("refs/remotes/origin/%s = %s, want %s", nu, got, want)
			}
		})
	}
}

// Two destinations of one fetch whose names conflict: the first lands, the second is refused
// as a name conflict (git: "'…' exists; cannot create '…'"), not attempted as a write.
func TestFetchRefusesConflictingNewNames(t *testing.T) {
	origin := gittest.NewFixture(t)
	work := cloneFixture(t, origin.Dir)
	err := openFixture(t, work).Fetch(FetchOpts{URL: origin.Dir, RefSpecs: []string{
		"+refs/heads/main:refs/zz/m", "+refs/heads/main:refs/zz/m/n",
	}})
	if !errors.Is(err, ErrRefsNotUpdated) || !strings.Contains(err.Error(), "refs/zz/m/n (cannot create it: refs/zz/m exists)") {
		t.Fatalf("Fetch into conflicting names = %v, want ErrRefsNotUpdated naming the conflict on refs/zz/m/n", err)
	}
	if got, want := mustFixtureGit(t, work, "rev-parse", "refs/zz/m"), mustFixtureGit(t, origin, "rev-parse", "main"); got != want {
		t.Fatalf("refs/zz/m = %s, want %s", got, want)
	}
}

// The directory helpers touch only empty directories strictly below refs/<kind>/, and only
// for a plain refs/ name.
func TestRefDirectoryHelpers(t *testing.T) {
	refsDir := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(refsDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	exists := func(rel string) bool {
		_, err := os.Lstat(filepath.Join(refsDir, filepath.FromSlash(rel)))
		return err == nil
	}

	for _, name := range []string{"HEAD", "refs/", "refs//x", "refs/./x", "refs/../x", "refs/a/../../x", "heads/x"} {
		if got := looseRefPath(refsDir, name); got != "" {
			t.Errorf("looseRefPath(%q) = %q, want \"\"", name, got)
		}
	}
	if got := looseRefPath("", "refs/heads/x"); got != "" {
		t.Errorf("looseRefPath with no refs dir = %q, want \"\"", got)
	}

	mk("kind/only/a/b")
	removeEmptyParents(refsDir, "refs/kind/only/a/b/x")
	if exists("kind/only") || !exists("kind") {
		t.Errorf("removeEmptyParents: kind/only exists = %v, kind exists = %v; want the emptied dirs gone and refs/kind kept", exists("kind/only"), exists("kind"))
	}
	mk("kind/two/a")
	if err := os.WriteFile(filepath.Join(refsDir, "kind", "two", "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	removeEmptyParents(refsDir, "refs/kind/two/a/x")
	if exists("kind/two/a") || !exists("kind/two/keep") {
		t.Errorf("removeEmptyParents went past a non-empty directory or left an empty one")
	}

	mk("kind/empty/x/y")
	removeEmptyTree(refsDir, "refs/kind/empty")
	if exists("kind/empty") {
		t.Error("removeEmptyTree left a tree that holds no file")
	}
	mk("kind/full/x")
	if err := os.WriteFile(filepath.Join(refsDir, "kind", "full", "x", "f"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	removeEmptyTree(refsDir, "refs/kind/full")
	if !exists("kind/full/x/f") {
		t.Error("removeEmptyTree removed a tree that holds a file")
	}
	mk("kind/link")
	if err := os.Symlink(t.TempDir(), filepath.Join(refsDir, "kind", "link", "l")); err == nil {
		removeEmptyTree(refsDir, "refs/kind/link")
		if !exists("kind/link/l") {
			t.Error("removeEmptyTree followed or removed a symlink")
		}
	}
}
