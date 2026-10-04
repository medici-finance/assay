package main

// namedpr_test.go — `deskpr update --pr N / --branch B` and `deskpr edit --pr N` from a
// worktree whose local branch name differs from the PR's head branch (issue #2085).
//
// Git lets one worktree hold a given branch, so a worker whose PR head branch is checked out
// elsewhere had no sanctioned verb to push to that PR (update resolved the PR from the cwd
// branch name alone) or to correct its body from a desk worktree (edit's checkout guard).
//
// FAIL-FIRST: before the change `update --pr` / `--branch` were unknown flags (rc 5, "bad
// flags") so every success case here was RED, and `edit --pr` from a checkout at a foreign
// commit or with no commits ahead was refused (rc 5). The refusal cases below assert on
// "nothing pushed" so they cannot pass on a flag-parse refusal alone: each also reads the
// bare remote.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const heldBranch = "feature/held-elsewhere"

// namedPRFixture returns a work tree sitting on `neutral-rework`, whose HEAD is one commit on
// top of the PR head commit (C1) that the bare remote already holds as refs/heads/heldBranch.
// It returns the work dir, the recorded calls, the bare remote's path and C1.
func namedPRFixture(t *testing.T) (work string, calls *[][]string, bare, c1 string) {
	t.Helper()
	work = newBaseFixture(t)
	calls = withEnv(t, work)
	mustGit(t, work, "checkout", "-b", "neutral-rework")
	c1 = mustGit(t, work, "rev-parse", "HEAD")
	mustGit(t, work, "push", "origin", "HEAD:refs/heads/"+heldBranch)
	writeFile(t, filepath.Join(work, "next.txt"), "follow-up\n")
	mustGit(t, work, "add", "next.txt")
	mustGit(t, work, "commit", "-m", "follow-up on the PR")
	bare = filepath.Join(filepath.Dir(work), "origin.git")
	t.Setenv("FAKEGH_PR_HEAD", heldBranch)
	t.Setenv("FAKEGH_PR_OID", c1)
	return work, calls, bare, c1
}

func remoteRef(t *testing.T, bare, ref string) string {
	t.Helper()
	return mustGit(t, bare, "rev-parse", "--verify", "--quiet", ref)
}

// TestUpdateByPRPushesToTheHeadBranch — the issue's case: the worktree's branch name is not
// the PR's head branch, and update --pr 42 pushes HEAD onto the PR's own head branch, with an
// explicit refspec and without re-pointing upstream. Pre-fix: rc 5 (unknown flag).
func TestUpdateByPRPushesToTheHeadBranch(t *testing.T) {
	work, calls, bare, _ := namedPRFixture(t)
	head := mustGit(t, work, "rev-parse", "HEAD")
	if rc := run([]string{"update", "--pr", "42"}); rc != deskkit.ExitOK {
		t.Fatalf("update --pr 42 rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != head {
		t.Fatalf("PR head branch on the remote = %s, want the worktree HEAD %s", got, head)
	}
	if !anyCall(gitCalls(*calls), "push", "origin", "HEAD:refs/heads/"+heldBranch) {
		t.Fatalf("update did not push by explicit refspec: %v", gitCalls(*calls))
	}
	if anyCall(gitCalls(*calls), "push", "-u") {
		t.Fatalf("update --pr re-pointed the local branch's upstream: %v", gitCalls(*calls))
	}
	if got := mustGit(t, bare, "for-each-ref", "refs/heads/neutral-rework"); got != "" {
		t.Fatalf("the local branch name leaked to the remote: %s", got)
	}
}

// TestUpdateByPRFromDetachedHead — a detached HEAD has no branch name at all; --pr N still
// resolves the destination from the PR.
func TestUpdateByPRFromDetachedHead(t *testing.T) {
	work, calls, bare, _ := namedPRFixture(t)
	mustGit(t, work, "checkout", "--detach", "HEAD")
	head := mustGit(t, work, "rev-parse", "HEAD")
	if rc := run([]string{"update", "--pr", "42"}); rc != deskkit.ExitOK {
		t.Fatalf("detached update --pr 42 rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != head {
		t.Fatalf("remote head = %s, want %s", got, head)
	}
}

// TestUpdateByBranchPushesToTheHeadBranch — --branch B names the PR by its remote head branch.
func TestUpdateByBranchPushesToTheHeadBranch(t *testing.T) {
	work, calls, bare, _ := namedPRFixture(t)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	head := mustGit(t, work, "rev-parse", "HEAD")
	if rc := run([]string{"update", "--branch", heldBranch}); rc != deskkit.ExitOK {
		t.Fatalf("update --branch rc = %d, want 0; calls: %v", rc, *calls)
	}
	if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != head {
		t.Fatalf("remote head = %s, want %s", got, head)
	}
	if !curForgeLookedUp(heldBranch) {
		t.Fatalf("the PR was not looked up by the --branch name: %v", curForge.openBranches)
	}
}

func curForgeLookedUp(branch string) bool {
	for _, b := range curForge.openBranches {
		if b == branch {
			return true
		}
	}
	return false
}

// TestUpdateByPRRefusals — every refusal pushes nothing (the bare remote still holds C1).
func TestUpdateByPRRefusals(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, work string)
		args  []string
		want  int // 0 = ExitRefused
	}{
		{name: "closed PR", setup: func(t *testing.T, _ string) { t.Setenv("FAKEGH_PR_STATE", "closed") }, args: []string{"--pr", "42"}},
		{name: "head not reported", setup: func(t *testing.T, _ string) { t.Setenv("FAKEGH_PR_OID", "") }, args: []string{"--pr", "42"}},
		{name: "head ref not reported", setup: func(t *testing.T, _ string) { t.Setenv("FAKEGH_PR_HEAD", "") }, args: []string{"--pr", "42"}, want: deskkit.ExitUnverifiable},
		{
			name: "HEAD does not descend from the PR head",
			setup: func(t *testing.T, work string) {
				mustGit(t, work, "checkout", "-b", "unrelated", "refs/remotes/origin/main")
				writeFile(t, filepath.Join(work, "other.txt"), "other\n")
				mustGit(t, work, "add", "other.txt")
				mustGit(t, work, "commit", "-m", "unrelated work")
			},
			args: []string{"--pr", "42"},
		},
		{name: "PR head is the default branch", setup: func(t *testing.T, _ string) { t.Setenv("FAKEGH_PR_HEAD", "main") }, args: []string{"--pr", "42"}},
		{name: "--pr and --branch together", args: []string{"--pr", "42", "--branch", heldBranch}},
		{name: "negative --pr", args: []string{"--pr", "-3"}},
		{name: "--branch names the default branch", args: []string{"--branch", "main"}},
		{
			name: "--branch does not match the PR's head",
			setup: func(t *testing.T, _ string) {
				t.Setenv("FAKEGH_LIST_HAS_PR", "1")
				t.Setenv("FAKEGH_PR_HEAD", "feature/other")
			},
			args: []string{"--branch", heldBranch},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			work, calls, bare, c1 := namedPRFixture(t)
			if tc.setup != nil {
				tc.setup(t, work)
			}
			want := tc.want
			if want == 0 {
				want = deskkit.ExitRefused
			}
			if rc := run(append([]string{"update"}, tc.args...)); rc != want {
				t.Fatalf("rc = %d, want %d", rc, want)
			}
			if got := remoteRef(t, bare, "refs/heads/"+heldBranch); got != c1 {
				t.Fatalf("a refused update moved the PR head: %s, want %s", got, c1)
			}
			if anyCall(gitCalls(*calls), "push") {
				t.Fatalf("pushed: %v", gitCalls(*calls))
			}
		})
	}
}

// TestUpdateWithoutPRFlagUnchanged — the default path still pushes the local branch with -u
// (the explicit-refspec form is the named-PR path only).
func TestUpdateWithoutPRFlagUnchanged(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	t.Setenv("FAKEGH_LIST_HAS_PR", "1")
	if rc := run([]string{"update"}); rc != deskkit.ExitOK {
		t.Fatalf("update rc = %d", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("default update push changed: %v", gitCalls(*calls))
	}
}

// existingPRVerbsMissingPR is the CLASS GUARD for #2085. The defect class: a deskpr verb that
// acts on an EXISTING PR but can only find it from the worktree's branch name, so it is
// unusable from a worktree whose branch differs from the PR's head. Every such verb (all but
// create, which has no PR yet) must offer --pr N in its usage line.
func existingPRVerbsMissingPR(usageText string) []string {
	var missing []string
	for _, line := range strings.Split(usageText, "\n") {
		if !strings.HasPrefix(line, "  deskpr ") {
			continue // the verb lines are the indented USAGE block, not the prose below it
		}
		f := strings.Fields(line)
		if len(f) < 2 || strings.HasPrefix(f[1], "--") || f[1] == "create" {
			continue
		}
		if !strings.Contains(line, "--pr") {
			missing = append(missing, f[1])
		}
	}
	return missing
}

func TestExistingPRVerbsAllTakePR(t *testing.T) {
	if got := existingPRVerbsMissingPR(usage); len(got) != 0 {
		t.Fatalf("deskpr verbs acting on an existing PR with no --pr override: %v", got)
	}
	// Positive control: a planted second instance (a verb that resolves its PR from the cwd
	// branch only) must be flagged, so a matcher that silently stopped matching fails here.
	planted := usage + "\n  deskpr close [--check]\n"
	if got := existingPRVerbsMissingPR(planted); len(got) != 1 || got[0] != "close" {
		t.Fatalf("class guard did not flag the planted verb: %v", got)
	}
}
