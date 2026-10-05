package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinnedBaseRemainsImmutable(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	head := mustGit(t, work, "rev-parse", "HEAD")
	next := commitOnto(t, work, head, "advance remote")
	upstream := "refs/remotes/origin/source"
	mustGit(t, work, "update-ref", upstream, head)
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if strings.Join(args[:min(2, len(args))], " ") == "worktree add" {
			mustGit(t, work, "update-ref", upstream, next)
		}
		return old(name, args...)
	}
	if rc, errOut := runCapErr(t, []string{"add", "pinned", "--branch", "source", "--base", head, "--upstream", upstream}); rc != 0 {
		t.Fatalf("add=%d %s", rc, errOut)
	}
	target := filepath.Join(tmpBaseDir, "tracker-pinned")
	if got := mustGit(t, target, "rev-parse", "HEAD"); got != head {
		t.Fatalf("checkout moved with upstream: got %s want %s", got, head)
	}
	if got := mustGit(t, target, "rev-parse", "--symbolic-full-name", "@{upstream}"); got != upstream {
		t.Fatalf("upstream=%s", got)
	}
}

func TestPinnedUpstreamRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"missing", []string{"--upstream", "refs/remotes/origin/missing"}},
		{"different", []string{"--upstream", "refs/remotes/origin/other"}},
		{"short", []string{"--upstream", "origin/main"}},
		{"option", []string{"--upstream", "--upload-pack=bad"}},
		{"noncommit", []string{"--base", "origin/main", "--upstream", "refs/remotes/origin/main"}},
		{"detached", []string{"--detach", "--upstream", "refs/remotes/origin/main"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := newRepo(t)
			calls := withEnv(t, work)
			head := mustGit(t, work, "rev-parse", "HEAD")
			next := commitOnto(t, work, head, "different source")
			mustGit(t, work, "update-ref", "refs/remotes/origin/other", next)
			args := append([]string{"add", "refused", "--base", head}, tc.args...)
			if rc, _ := runCapErr(t, args); rc == 0 {
				t.Fatal("invalid pinned source admitted")
			}
			if hasWorktreeVerb(*calls, "add") {
				t.Fatal("refused source allocated a worktree")
			}
		})
	}
}

func TestPinnedTrackingRollback(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	head := mustGit(t, work, "rev-parse", "HEAD")
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		if len(args) > 1 && args[0] == "branch" && args[1] == "--set-upstream-to" {
			return exec.Command("git", "fixture-tracking-failure")
		}
		return old(name, args...)
	}
	rc, errOut := runCapErr(t, []string{"add", "rollback", "--base", head, "--upstream", "refs/remotes/origin/main"})
	if rc == 0 || !strings.Contains(errOut, "worktree rolled back") {
		t.Fatalf("tracking failure=%d %s", rc, errOut)
	}
	if _, err := os.Stat(filepath.Join(tmpBaseDir, "tracker-rollback")); !os.IsNotExist(err) {
		t.Fatalf("failed allocation remains: %v", err)
	}
}
