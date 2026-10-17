package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// The forge is offline, but dispatch's allocation child and every Git command
// are real. An argv-only stub cannot catch a producer/allocator contract mismatch.
// regression: #3327
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
	// The allocator is another process: the parent's in-memory minter seam
	// cannot reach it. Supply a native fixture minter, never an installed tool.
	minterDir := t.TempDir()
	minter := filepath.Join(minterDir, "desktoken")
	if runtime.GOOS == "windows" {
		minter += ".exe"
	}
	source := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(source, []byte(`package main
import ("fmt"; "os"; "strings")
func main() {
 if strings.Join(os.Args[1:], " ") != "worker --repo medici-finance" { os.Exit(2) }
 if err := os.WriteFile(os.Getenv("RESUME_MINTER_RECORD"), []byte(strings.Join(os.Args[1:], " ")), 0600); err != nil { os.Exit(3) }
 fmt.Println(os.Getenv("RESUME_MINTER_TOKEN_FILE"))
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-o", minter, source).CombinedOutput(); err != nil {
		t.Fatalf("build fixture minter: %v %s", err, out)
	}
	s := &stub{}
	home, root := s.install(t)
	t.Setenv("PATH", minterDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	record := filepath.Join(home, "minter.record")
	t.Setenv("RESUME_MINTER_RECORD", record)
	t.Setenv("RESUME_MINTER_TOKEN_FILE", filepath.Join(home, "allocator.token"))
	if err := os.WriteFile(os.Getenv("RESUME_MINTER_TOKEN_FILE"), []byte("allocator-role-fixture"), 0600); err != nil {
		t.Fatal(err)
	}
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
	git(root, "remote", "set-url", "origin", "https://github.com/"+allowedRepo+".git")
	gittest.RoleHTTP(t, filepath.Dir(filepath.Dir(remote)), "resume-role-fixture")
	restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
		path := filepath.Join(os.Getenv("HOME"), "resume.token")
		err := os.WriteFile(path, []byte("resume-role-fixture"), 0600)
		return path, "", err
	})
	defer restore()
	refreshResume = liveRefreshResume
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
	if got, err := os.ReadFile(record); err != nil || string(got) != "worker --repo medici-finance" {
		t.Fatalf("allocator child did not resolve its worker custody: %q %v", got, err)
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
