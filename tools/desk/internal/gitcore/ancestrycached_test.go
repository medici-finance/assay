package gitcore

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// regression: #3187
func TestCachedAncestryMatchesGit(t *testing.T) {
	cf := newContainsFixture(t)
	for _, mode := range []string{"no-graph", "graph", "split-graph"} {
		t.Run(mode, func(t *testing.T) {
			if mode != "no-graph" {
				args := []string{"commit-graph", "write", "--reachable"}
				if mode == "split-graph" {
					args = append(args, "--split")
				}
				if _, err := cf.f.Git(args...); err != nil {
					t.Fatal(err)
				}
			}
			repo, err := Open(cf.f.Dir)
			if err != nil {
				t.Fatal(err)
			}
			for aName, a := range cf.commitNames {
				for dName, d := range cf.commitNames {
					cmd := exec.Command("git", "merge-base", "--is-ancestor", a, d)
					cmd.Dir = cf.f.Dir
					err := cmd.Run()
					var exit *exec.ExitError
					if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
						t.Fatal(err)
					}
					want := err == nil
					got, err := repo.IsAncestorCached(a, d)
					if err != nil || got != want {
						t.Fatalf("%s -> %s: got %v, %v; git says %v", aName, dName, got, err, want)
					}
				}
			}
			// Revisions are resolved on each call, even with a warm node index.
			if _, err := cf.f.Git("update-ref", "refs/remotes/origin/moving", cf.m); err != nil {
				t.Fatal(err)
			}
			if got, err := repo.IsAncestorCached(cf.f2, "refs/remotes/origin/moving"); err != nil || !got {
				t.Fatalf("initial tip: %v, %v", got, err)
			}
			if _, err := cf.f.Git("update-ref", "refs/remotes/origin/moving", cf.c2); err != nil {
				t.Fatal(err)
			}
			if got, err := repo.IsAncestorCached(cf.f2, "refs/remotes/origin/moving"); err != nil || got {
				t.Fatalf("moved tip: %v, %v", got, err)
			}
			for _, pair := range [][2]string{{"missing-ref", cf.m}, {cf.f2, "missing-ref"}, {strings.Repeat("f", 40), cf.m}, {cf.f2, strings.Repeat("f", 40)}} {
				if _, err := repo.IsAncestorCached(pair[0], pair[1]); err == nil {
					t.Fatalf("unreadable endpoint accepted: %v", pair)
				}
			}
		})
	}
}

// regression: #3187
func TestCachedAncestryDecodesSharedHistoryOnce(t *testing.T) {
	const commits = 80
	f, shas := linearFixture(t, commits, 4)
	newTip := f.CommitFile(t, "new.txt", "new\n", "unpublished")
	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// Different descendant tips share history. Reusing only final boolean answers
	// cannot satisfy this bound; the decoded immutable graph must also be shared.
	for _, tip := range shas {
		if got, err := repo.IsAncestorCached(newTip, tip); err != nil || got {
			t.Fatalf("unpublished ancestor: %v, %v", got, err)
		}
	}
	stats := repo.reachStats()
	if stats.odbLoads == 0 || stats.odbLoads > commits+1 {
		t.Fatalf("decoded %d commits for %d distinct ancestry queries, want 1..%d", stats.odbLoads, len(shas), commits+1)
	}
}
