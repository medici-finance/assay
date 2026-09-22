package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A collision is judged against the branch's OWN remote counterpart, never the mainline.
//
// THE DEFECT. A branch created by an earlier `add` tracks the ref it was cut FROM — the
// mainline — so its configured upstream is `refs/remotes/origin/main` even after its work
// was pushed to `refs/remotes/origin/<branch>`. Measuring "is this a leftover?" against
// that upstream measures it against main, and EVERY real feature branch is ahead of main
// by construction: a branch byte-identical to its own pushed counterpart read as
// "unfinished work, not a leftover" and the collision was refused, permanently, for the
// one case where reclaiming is provably safe (nothing is lost — every commit is already on
// the remote counterpart).
//
// The comparison ref is therefore the branch's own remote counterpart when one exists. A
// branch carrying commits BEYOND that counterpart is genuinely unpushed work and is still
// refused, naming that counterpart rather than the mainline.

// commitOnto returns a new commit on top of parent, made without checking anything out, so
// the fixture's own HEAD (and thus the "no worktree holds it" property) stays untouched.
func commitOnto(t *testing.T, work, parent, msg string) string {
	t.Helper()
	tree := mustGit(t, work, "rev-parse", parent+"^{tree}")
	return mustGit(t, work, "-c", "user.email=t@e.st", "-c", "user.name=Test",
		"commit-tree", tree, "-p", parent, "-m", msg)
}

// plantPushedBranch plants a branch that exists BOTH on the remote (pushed, so
// refs/remotes/origin/<br> resolves) and locally. Its local tip is `extra` commits beyond
// the pushed tip; `upstream` is what the local branch's configured upstream is set to
// ("" leaves it unset). It returns the pushed tip.
func plantPushedBranch(t *testing.T, work, br, upstream string, extra int) string {
	t.Helper()
	pushed := commitOnto(t, work, "refs/remotes/origin/main", "work on "+br)
	mustGit(t, work, "push", "--quiet", "origin", pushed+":refs/heads/"+br)
	mustGit(t, work, "update-ref", "refs/remotes/origin/"+br, pushed)

	local := pushed
	for i := 0; i < extra; i++ {
		local = commitOnto(t, work, local, "unpushed work")
	}
	mustGit(t, work, "update-ref", "refs/heads/"+br, local)
	if upstream != "" {
		mustGit(t, work, "branch", "--set-upstream-to="+upstream, br)
	}
	return pushed
}

// (b) A local branch that EQUALS its own remote counterpart is a leftover ref, however far
// ahead of the mainline it is — it is reclaimed, never refused.
func TestCollisionAgainstTheBranchesOwnRemoteCounterpart(t *testing.T) {
	cases := []struct {
		name     string
		upstream string
		// base is the --base the caller passes ("" = the verb's default, the mainline — the
		// shape the loud refusal was observed in; the branch's own ref = the shape a resume
		// dispatch passes once it cuts from the change's branch).
		base string
	}{
		// The defect's own residue: the branch tracks the ref it was cut FROM (the mainline).
		{"mainline base, upstream points at the mainline", "origin/main", ""},
		// No upstream configured at all — its remote counterpart is still its own.
		{"mainline base, no upstream configured", "", ""},
		{"resume base, upstream points at the mainline", "origin/main", "own"},
		{"resume base, no upstream configured", "", "own"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			work := newRepo(t)
			withEnv(t, work)
			const name = "collide"
			pushed := plantPushedBranch(t, work, name, c.upstream, 0)

			args := []string{"add", name}
			if c.base == "own" {
				args = append(args, "--base", "refs/remotes/origin/"+name)
			}
			rc, stderr := runCapErr(t, args)
			if rc != deskkit.ExitOK {
				t.Fatalf("add rc = %d, want %d — a branch equal to its own remote counterpart is not "+
					"unfinished work, however far ahead of the mainline it is; stderr:\n%s", rc, deskkit.ExitOK, stderr)
			}
			if !strings.Contains(stderr, "reclaimed stale local branch") {
				t.Errorf("the collision was not reported as a reclaim:\n%s", stderr)
			}
			if !strings.Contains(stderr, "0 commits ahead of refs/remotes/origin/"+name) {
				t.Errorf("the reclaim measured the branch against something other than its own remote "+
					"counterpart:\n%s", stderr)
			}
			if c.base == "own" {
				// The resume shape: the worktree lands at the branch's remote TIP, not the mainline's.
				target := filepath.Join(tmpBaseDir, "tracker-"+name)
				if head := mustGit(t, target, "rev-parse", "HEAD"); head != pushed {
					mainSHA := mustGit(t, work, "rev-parse", "refs/remotes/origin/main")
					t.Errorf("worktree HEAD = %s, want the branch's remote tip %s (mainline is %s)", head, pushed, mainSHA)
				}
			}
		})
	}
}

// (c) UNCHANGED: a local branch carrying commits beyond its own remote counterpart is real
// unpushed work and is still refused — now naming that counterpart, not the mainline.
func TestCollisionStillRefusesWorkBeyondTheBranchesOwnRemoteCounterpart(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	const name = "collide"
	plantPushedBranch(t, work, name, "origin/main", 1)

	rc, stderr := runCapErr(t, []string{"add", name})
	if rc != deskkit.ExitRefused {
		t.Fatalf("add rc = %d, want %d — a branch with commits its own remote counterpart does not "+
			"carry is unfinished work; stderr:\n%s", rc, deskkit.ExitRefused, stderr)
	}
	for _, want := range []string{"1 commit(s) not in refs/remotes/origin/" + name, "unfinished work"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the refusal does not name the cause: missing %q in:\n%s", want, stderr)
		}
	}
	if out := mustGit(t, work, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); out == "" {
		t.Error("a refusal deleted the colliding branch")
	}
}
