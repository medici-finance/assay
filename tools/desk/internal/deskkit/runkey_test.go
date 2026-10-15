package deskkit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runKeyGit runs git in dir with an inline identity, failing the test on error.
func runKeyGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// runKeyRepo builds a repository with the worktree-config extension on and n linked
// worktrees, stamps one of them the way deskdispatch does, and plants assay.runKey in the
// repository's LOCAL config, where every worktree's plain `git config` lookup would see it.
// It returns the main worktree, the linked worktrees, and the stamped one's key.
func runKeyRepo(t *testing.T, n int) (main string, linked []string, key string) {
	t.Helper()
	root := t.TempDir()
	main = filepath.Join(root, "main")
	runKeyGit(t, root, "init", "-q", "-b", "main", main)
	runKeyGit(t, main, "commit", "-q", "--allow-empty", "-m", "init")
	runKeyGit(t, main, "config", "extensions.worktreeConfig", "true")
	for i := 0; i < n; i++ {
		wt := filepath.Join(root, fmt.Sprintf("wt-%02d", i))
		runKeyGit(t, main, "worktree", "add", "-q", "--detach", wt)
		linked = append(linked, wt)
	}
	key = "example-org--example-stream--07"
	runKeyGit(t, linked[n/2], "config", "--worktree", "assay.runKey", key)
	runKeyGit(t, main, "config", "--local", "assay.runKey", "planted-local-key")
	return main, linked, key
}

// isolateRunKeyConfig points git's global config at a file that plants assay.runKey, turns
// off the system config, and plants it again through the environment (`git -c` form), so a
// reader that consulted anything but the worktree's own file would see a key.
func isolateRunKeyConfig(t *testing.T) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[assay]\n\trunKey = planted-global-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "assay.runKey")
	t.Setenv("GIT_CONFIG_VALUE_0", "planted-env-key")
}

// TestRunKeyManyWorktrees — the resolver in a repository with 50 linked worktrees returns
// the stamped worktree's own key, "" for every unstamped worktree (linked or main), and
// never the key planted in the local, global or environment config. The last is the
// security property: a caller must not be able to name another run's key.
func TestRunKeyManyWorktrees(t *testing.T) {
	isolateRunKeyConfig(t)
	main, linked, key := runKeyRepo(t, 50)
	stamped := linked[len(linked)/2]
	if got := worktreeRunKeyIn(stamped); got != key {
		t.Fatalf("stamped worktree: run key %q, want %q", got, key)
	}
	for _, wt := range append([]string{main}, linked...) {
		if wt == stamped {
			continue
		}
		if got := worktreeRunKeyIn(wt); got != "" {
			t.Fatalf("unstamped worktree %s: run key %q, want \"\" (a planted local/global/env key leaked)",
				filepath.Base(wt), got)
		}
	}
	if got := worktreeRunKeyIn(t.TempDir()); got != "" {
		t.Fatalf("outside any repository: run key %q, want \"\"", got)
	}
}

// TestRunKeySingleNoExtension pins the one documented change of meaning: a single-worktree
// repository WITHOUT extensions.worktreeConfig. `git config --worktree` fell back to the
// repository config there; the direct read finds no per-worktree file and returns "", the
// same answer as a worktree that was never stamped.
func TestRunKeySingleNoExtension(t *testing.T) {
	isolateRunKeyConfig(t)
	repo := filepath.Join(t.TempDir(), "repo")
	runKeyGit(t, filepath.Dir(repo), "init", "-q", "-b", "main", repo)
	runKeyGit(t, repo, "config", "--local", "assay.runKey", "planted-local-key")
	if got := worktreeRunKeyIn(repo); got != "" {
		t.Fatalf("single worktree, no extension: run key %q, want \"\"", got)
	}
}

// gitTraceCommands returns the git commands a GIT_TRACE file recorded, one per line.
func gitTraceCommands(t *testing.T, trace string) []string {
	t.Helper()
	b, err := os.ReadFile(trace)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var cmds []string
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "trace: built-in: "); i >= 0 {
			cmds = append(cmds, line[i+len("trace: built-in: "):])
		}
	}
	return cmds
}

// regression: #2864
// TestRunKeyNoWorktreeWalk — the production read in a stamped linked worktree returns its
// key without running `git config --worktree`, the call that walks every registered
// worktree before it reads anything.
func TestRunKeyNoWorktreeWalk(t *testing.T) {
	isolateRunKeyConfig(t)
	_, linked, key := runKeyRepo(t, 3)
	t.Chdir(linked[len(linked)/2])
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE", trace)
	if got := gitWorktreeRunKey(); got != key {
		t.Fatalf("stamped worktree: run key %q, want %q", got, key)
	}
	cmds := gitTraceCommands(t, trace)
	if len(cmds) == 0 {
		t.Fatal("GIT_TRACE recorded no git command; the trace no longer observes the read")
	}
	for _, c := range cmds {
		if strings.Contains(c, "--worktree") {
			t.Fatalf("the run-key read ran %q, which walks every registered worktree", c)
		}
	}
}

// TestRunKeyStampedMainRelativeDir — a stamped MAIN worktree, read through a relative dir.
// There `rev-parse --git-path` prints a path relative to dir, and the second git call must
// resolve it against dir too, not against the process's own directory.
func TestRunKeyStampedMainRelativeDir(t *testing.T) {
	isolateRunKeyConfig(t)
	main, _, _ := runKeyRepo(t, 1)
	runKeyGit(t, main, "config", "--worktree", "assay.runKey", "main-key")
	t.Chdir(filepath.Dir(main))
	if got := worktreeRunKeyIn(filepath.Base(main)); got != "main-key" {
		t.Fatalf("stamped main worktree via relative dir: run key %q, want %q", got, "main-key")
	}
}

// regression: #2864
// TestRunKeyTestBinaryInert — inside a test binary the default resolver reads no worktree:
// run from a stamped worktree it returns "" and runs no git at all, so a command package's
// tests that drive Guard() never exec git against the developer's own worktree. It builds a
// FRESH default resolver: the shared runKeyResolver caches its first answer, which an
// earlier Guard() test has usually already filled, so reading it here could not fail.
func TestRunKeyTestBinaryInert(t *testing.T) {
	isolateRunKeyConfig(t)
	_, linked, _ := runKeyRepo(t, 3)
	t.Chdir(linked[len(linked)/2])
	trace := filepath.Join(t.TempDir(), "trace")
	t.Setenv("GIT_TRACE", trace)
	if got := defaultRunKeyResolver()(); got != "" {
		t.Fatalf("test binary in a stamped worktree: run key %q, want \"\"", got)
	}
	if cmds := gitTraceCommands(t, trace); len(cmds) != 0 {
		t.Fatalf("the default resolver ran git inside a test binary: %q", cmds)
	}
}

// regression: #2864
// TestRunKeyResolvedOnce — the memoised resolver runs its read once per process however
// many times a verb guards.
func TestRunKeyResolvedOnce(t *testing.T) {
	reads := 0
	resolve := onceRunKey(func() string { reads++; return "example-key" })
	for i := 0; i < 3; i++ {
		if got := resolve(); got != "example-key" {
			t.Fatalf("call %d: run key %q, want %q", i, got, "example-key")
		}
	}
	if reads != 1 {
		t.Fatalf("the read ran %d times for 3 resolutions, want 1", reads)
	}
}
