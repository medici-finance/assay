package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/gitcore/testenv"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

// Resolve the declared build paths before any fixture relocates HOME. The real
// allocator then inherits the same explicit environment as its parent fixture.
func realAllocatorFixtureEnvironment(t *testing.T, extraTools ...string) {
	t.Helper()
	tools := map[string]string{}
	for _, name := range append([]string{"git", "go", "sh", "cat"}, extraTools...) {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal(err)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		tools[name] = path
	}
	cmd := exec.Command(tools["go"], "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT")
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve fixture Go paths: %v", err)
	}
	vars := map[string]string{}
	if err := json.Unmarshal(raw, &vars); err != nil {
		t.Fatal(err)
	}
	for key, value := range vars {
		if value == "" || !filepath.IsAbs(filepath.SplitList(value)[0]) {
			t.Fatalf("invalid fixture build path %s", key)
		}
	}
	for key, value := range map[string]string{"GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "GOENV": "off", "GOFLAGS": "-buildvcs=false"} {
		vars[key] = value
	}
	t.Cleanup(deskkit.ReloadConfig)
	testenv.Install(t, testenv.Options{Tools: tools, Vars: vars})
	deskkit.ReloadConfig()
}

func TestFixtureSiblingParentIsolation07(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	git, err = filepath.Abs(git)
	if err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	global := filepath.Join(parent, "global")
	victim := filepath.Join(parent, "victim")
	const globalBody = "[fixture]\n\tvalue = parent\n"
	if err := os.WriteFile(global, []byte(globalBody), 0600); err != nil {
		t.Fatal(err)
	}
	fixed := []string{"PATH=" + filepath.Dir(git), "HOME=" + parent, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + global, "GIT_CONFIG_SYSTEM=" + os.DevNull}
	if value := os.Getenv("SystemRoot"); value != "" {
		fixed = append(fixed, "SystemRoot="+value)
	}
	run := func(env []string, args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("victim git: %v: %s", err, out)
		}
	}
	run(fixed, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "init", "-q", victim)
	run(fixed, "-C", victim, "remote", "add", "origin", "https://github.com/example-org/victim.git")
	config := filepath.Join(victim, ".git", "config")
	body, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(victim, ".git", "index"))
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "basic")
	if _, err := gitcore.OriginAuthority(victim); err == nil {
		t.Fatal("inherited authority control did not refuse injected configuration")
	}
	beforeControl := regression.TreeDigest(t, parent)
	run(os.Environ(), "config", "fixture.control", "reached")
	run(os.Environ(), "config", "--global", "fixture.control", "reached")
	if regression.TreeDigest(t, parent) == beforeControl {
		t.Fatal("inherited control did not reach the parent victim")
	}
	if err := os.WriteFile(config, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte(globalBody), 0600); err != nil {
		t.Fatal(err)
	}
	before := regression.SnapshotTree(t, parent)
	t.Run("real-allocator", func(t *testing.T) {
		realAllocatorFixtureEnvironment(t)
		r := newRealResume(t)
		result := createDispatchWorktree(r.o, r.plan)
		if result.err != nil {
			t.Fatalf("isolated real allocator: %v: %s", result.err, result.stderr)
		}
		target := strings.TrimSpace(result.stdout)
		t.Cleanup(func() { r.git(r.root, "worktree", "remove", target) })
		if got := r.git(target, "rev-parse", "HEAD"); got != r.head {
			t.Fatalf("allocated HEAD = %s, want %s", got, r.head)
		}
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
		t.Setenv("GIT_CONFIG_VALUE_0", "basic")
		if _, err := gitcore.OriginAuthority(r.root); err == nil {
			t.Fatal("fixture erased intentional authority rejection")
		}
	})
	if changes := before.Changes(regression.SnapshotTree(t, parent)); changes != "" {
		t.Fatalf("real allocator fixture changed parent victim:\n%s", changes)
	}
}
