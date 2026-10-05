package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The forge is offline, but dispatch's allocation child and every Git command
// are real. An argv-only stub cannot catch a producer/allocator contract mismatch.
func TestResumeRealAllocator(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "deskwt")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// Use the allocator's existing test-only prefix seam for portable isolation.
	build := exec.Command("go", "build", "-ldflags", fmt.Sprintf("-X %q", "main.tmpBaseDir="+t.TempDir()), "-o", binary, "../deskwt")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build allocator: %v %s", err, out)
	}
	s := &stub{}
	_, root := s.install(t)
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-b", "main")
	git(root, "-c", "user.name=Example", "-c", "user.email=example@example.test", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "main")
	main := git(root, "rev-parse", "HEAD")
	const branch = "fix/resume-source"
	git(root, "checkout", "-b", branch)
	git(root, "-c", "user.name=Example", "-c", "user.email=example@example.test", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "source")
	head := git(root, "rev-parse", "HEAD")
	git(root, "checkout", "main")
	git(root, "branch", "-D", branch)
	remote := filepath.Join(t.TempDir(), "medici-finance", "assay.git")
	git(root, "init", "--bare", remote)
	git(root, "remote", "add", "origin", remote)
	git(root, "push", "origin", main+":refs/heads/main", head+":refs/heads/"+branch)
	// All allocator calls execute the shipped command, never a substituted success.
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "deskwt" {
			return exec.Command(binary, args...)
		}
		if name != "git" {
			t.Fatalf("unexpected child %s", name)
		}
		return exec.Command(name, args...)
	}
	readResumeChange = func(dispatchOpts, string) (deskkit.PullRequest, error) {
		return deskkit.PullRequest{State: "open", HeadRef: branch, HeadSHA: head, CrossRepo: deskkit.CrossRepoSame}, nil
	}
	o := dispatchOpts{root: root, pr: 42, kit: "worker"}
	plan := dispatchPlan{repo: allowedRepo, wtName: fmt.Sprintf("resume-%d", time.Now().UnixNano()), identityRole: "worker"}
	if err := resolveResume(o, &plan); err != nil {
		t.Fatal(err)
	}
	result := createDispatchWorktree(o, plan)
	if result.err != nil {
		t.Fatalf("real allocator rejected verified resume: %v; %s", result.err, result.stderr)
	}
	target := strings.TrimSpace(result.stdout)
	t.Cleanup(func() { git(root, "worktree", "remove", target) })
	if got := git(target, "rev-parse", "HEAD"); got != head || got == main {
		t.Fatalf("HEAD=%s want source=%s main=%s", got, head, main)
	}
	if got := git(target, "rev-parse", "--abbrev-ref", "HEAD"); got != branch {
		t.Fatalf("branch=%s", got)
	}
	if got := git(target, "rev-parse", "--symbolic-full-name", "@{upstream}"); got != "refs/remotes/origin/"+branch {
		t.Fatalf("upstream=%s", got)
	}
}
