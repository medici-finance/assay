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

	"github.com/medici-finance/assay/gitcore/gittest"
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The forge is offline, but dispatch's allocation child and every Git command
// are real. An argv-only stub cannot catch a producer/allocator contract mismatch.
// regression: #3327
// regression: desktools-v2/58 — the allocator child mints in process, so a PATH minter
// fixture is never run; the child must be served from the provider's own cache.
func TestResumeRealAllocator(t *testing.T) {
	r := newRealResume(t)
	result := createDispatchWorktree(r.o, r.plan)
	if result.err != nil {
		t.Fatalf("real allocator rejected verified resume: %v; %s", result.err, result.stderr)
	}
	if got, err := os.ReadFile(r.adopted); err != nil || strings.TrimSpace(string(got)) != "worker-app medici-finance" {
		t.Fatalf("allocator child did not resolve its worker custody for medici-finance: %q %v", got, err)
	}
	target := strings.TrimSpace(result.stdout)
	t.Cleanup(func() { r.git(r.root, "worktree", "remove", target) })
	if got := r.git(target, "rev-parse", "HEAD"); got != r.head || got == r.main {
		t.Fatalf("HEAD=%s want source=%s main=%s", got, r.head, r.main)
	}
	if got := r.git(target, "rev-parse", "--abbrev-ref", "HEAD"); got != r.branch {
		t.Fatalf("branch=%s", got)
	}
	if got := r.git(target, "rev-parse", "--symbolic-full-name", "@{upstream}"); got != "refs/remotes/origin/"+r.branch {
		t.Fatalf("upstream=%s", got)
	}
}

// realResume is a verified worker resume wired to the SHIPPED allocator binary.
type realResume struct {
	o                                 dispatchOpts
	plan                              dispatchPlan
	git                               func(dir string, args ...string) string
	root, adopted, branch, head, main string
}

func newRealResume(t *testing.T) realResume {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "deskwt")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// Use the allocator's existing test-only prefix seam for portable isolation.
	build := exec.Command("go", "build", "-ldflags", fmt.Sprintf("-X %q", "main.tmpBaseDir="+t.TempDir()), "-o", binary, "../deskwt")
	if out, err := goquiet.Cmd(build).CombinedOutput(); err != nil {
		t.Fatalf("build allocator: %v %s", err, out)
	}
	// The allocator is another process: the parent's in-memory minter seam cannot reach it.
	// Since desktools-v2/58 it resolves its worker custody IN PROCESS through the credential
	// provider, never a desktoken found on PATH, so the fixture is that provider's own on-disk
	// state: an App ID, the (App, account) install-id cache and a fresh 0600 token cache. A
	// reuse needs no key and no network, and it adopts the cache for the account it served.
	s := &stub{}
	home, root := s.install(t)
	cfg := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.EnvConfigHome, "")
	for _, suffix := range []string{"_APP_ID", "_PEM", "_TOKEN", "_INSTALL_ID", "_APP"} {
		t.Setenv("WORKER"+suffix, "")
	}
	t.Setenv("WORKER_APP_ID", "12345")
	for name, body := range map[string]string{
		"worker-app-install-medici-finance": "4242\n",
		"worker-token-4242":                 "allocator-role-fixture\n",
	} {
		if err := os.WriteFile(filepath.Join(cfg, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	adopted := filepath.Join(cfg, "worker-token-4242.owner")
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
	t.Cleanup(restore)
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
	plan := dispatchPlan{repo: allowedRepo, wtName: fmt.Sprintf("resume-%d", time.Now().UnixNano()), identityRole: "worker",
		claimKey: "assay--issue-42"}
	if err := resolveResume(o, &plan); err != nil {
		t.Fatal(err)
	}
	return realResume{o: o, plan: plan, git: git, root: root, adopted: adopted, branch: branch, head: head, main: main}
}
