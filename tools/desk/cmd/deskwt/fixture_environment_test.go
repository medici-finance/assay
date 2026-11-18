package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/gitcore/testenv"
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// Capture only declared build inputs before TestMain relocates HOME for its
// synthetic roster. Windows home/config paths are query-only inputs: only the
// resolved Go paths below reach the fixture. No credential or desk policy is retained.
var worktreeBuildInputs = func() map[string]string {
	out := map[string]string{}
	for _, key := range []string{"HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "PATH", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT", "SystemRoot"} {
		if value, ok := os.LookupEnv(key); ok {
			out[key] = value
		}
	}
	return out
}()

// Resolve optional system tools before a fixture replaces PATH. A minimal caller
// PATH may omit sbin even though the standard Unix utility is installed there.
var worktreeStartupLsof = func() string {
	if path, err := exec.LookPath("lsof"); err == nil {
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute
		}
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{"/usr/sbin/lsof", "/usr/bin/lsof", "/sbin/lsof"} {
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				return path
			}
		}
	}
	return ""
}()
var worktreeFixtureOnce sync.Once
var worktreeFixtureTools map[string]string
var worktreeFixtureVars map[string]string
var worktreeFixtureError error

func resolveWorktreeFixtureInputs() {
	worktreeFixtureTools = map[string]string{}
	names := []string{"git", "go"}
	if runtime.GOOS != "windows" {
		// Git submodule sources git-sh-setup, which invokes these POSIX tools.
		names = append(names, "sh", "cat", "sleep", "basename", "sed", "uname")
	}
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err != nil {
			worktreeFixtureError = err
			return
		}
		path, err = filepath.Abs(path)
		if err != nil {
			worktreeFixtureError = err
			return
		}
		worktreeFixtureTools[name] = path
	}
	// Availability was captured before any scoped fixture could narrow PATH.
	if worktreeStartupLsof != "" {
		worktreeFixtureTools["lsof"] = worktreeStartupLsof
	}
	cmd := exec.Command(worktreeFixtureTools["go"], "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT")
	cmd.Env = []string{"GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"}
	for key, value := range worktreeBuildInputs {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	raw, err := goquiet.Cmd(cmd).Output()
	if err != nil {
		worktreeFixtureError = fmt.Errorf("resolve fixture build inputs: %w", err)
		return
	}
	worktreeFixtureVars = map[string]string{}
	if err := json.Unmarshal(raw, &worktreeFixtureVars); err != nil {
		worktreeFixtureError = err
		return
	}
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT"} {
		if worktreeFixtureVars[key] == "" || !filepath.IsAbs(filepath.SplitList(worktreeFixtureVars[key])[0]) {
			worktreeFixtureError = fmt.Errorf("fixture build input %s is not absolute", key)
			return
		}
	}
	for key, value := range map[string]string{"GOENV": "off", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "CGO_ENABLED": "0"} {
		worktreeFixtureVars[key] = value
	}
}

// Install at the top-level test boundary, before fixture construction. Nested
// repositories never reset intentional hostile inputs planted by the test.
func installWorktreeFixture(t *testing.T) *testenv.Fixture {
	t.Helper()
	worktreeFixtureOnce.Do(resolveWorktreeFixtureInputs)
	if worktreeFixtureError != nil {
		t.Fatal(worktreeFixtureError)
	}
	f := testenv.Install(t, testenv.Options{Tools: worktreeFixtureTools, Vars: worktreeFixtureVars})
	plantFixtureRoster(t, f.Home)
	return f
}

// regression: #2715, #6300
func TestFixtureSiblingParentIsolation06(t *testing.T) {
	worktreeFixtureOnce.Do(resolveWorktreeFixtureInputs)
	if worktreeFixtureError != nil {
		t.Fatal(worktreeFixtureError)
	}
	victim := t.TempDir()
	home := filepath.Join(victim, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(global, []byte("[fixture]\n owner = caller\n"), 0600); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(victim, "repo")
	// Literal setup is independent of the helper being tested.
	for _, args := range [][]string{
		{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "init", "-b", "main", repo},
		{"-C", repo, "remote", "add", "origin", "https://github.com/example-org/tracker.git"},
		{"-C", repo, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "victim"},
	} {
		cmd := exec.Command(worktreeFixtureTools["git"], args...)
		cmd.Env = []string{"PATH=" + worktreeBuildInputs["PATH"], "HOME=" + home, "GIT_CONFIG_GLOBAL=" + global, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
		if value := worktreeBuildInputs["SystemRoot"]; value != "" {
			cmd.Env = append(cmd.Env, "SystemRoot="+value)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("victim setup: %v: %s", err, out)
		}
	}
	before := regression.SnapshotTree(t, victim)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/foreign/project.git")
	if _, err := deskkit.OriginRepoSlug(repo); err == nil {
		t.Fatal("inherited positive control did not refuse command-priority configuration")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_DIR", filepath.Join(repo, ".git"))
	t.Setenv("GIT_WORK_TREE", repo)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repo, ".git", "index"))
	installWorktreeFixture(t)
	if os.Getenv("USERPROFILE") != os.Getenv("HOME") {
		t.Fatal("fixture profile does not belong to its owned home")
	}
	for _, key := range []string{"APPDATA", "LOCALAPPDATA"} {
		if _, present := os.LookupEnv(key); present {
			t.Fatalf("query-only startup input %s leaked into fixture environment", key)
		}
	}
	work := newRepo(t)
	withEnv(t, work)
	if got, err := deskkit.OriginRepoSlug(work); err != nil || got != "example-org/tracker" {
		t.Fatalf("isolated authority: %q %v", got, err)
	}
	mustGit(t, work, "config", "--global", "fixture.owner", "isolated")
	if after := regression.SnapshotTree(t, victim); !reflect.DeepEqual(before, after) {
		t.Fatal("isolated fixture modified parent config or repository")
	}
	// The real production guard remains mandatory for deliberately rejected input.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/foreign/project.git")
	if _, err := deskkit.OriginRepoSlug(work); err == nil || strings.Contains(err.Error(), "foreign/project") {
		t.Fatal("deliberate command-priority authority was not safely refused")
	}
}
