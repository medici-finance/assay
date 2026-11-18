package main

import (
	"debug/buildinfo"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Exercise the actual compiler, not just an environment-slice assertion. Build a
// fresh binary rather than reading the sync.Once cache populated by other tests.
func TestCompiledFixtureParentEnvironment(t *testing.T) {
	f, _ := compilerFixture(t)
	gitName := "git"
	if runtime.GOOS == "windows" {
		gitName += ".exe"
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(filepath.Join(f.Bin, gitName), args...)
		cmd.Dir, cmd.Env = dir, f.Env()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleRoot := filepath.Join(wd, "..", "..")
	revision := git(moduleRoot, "rev-parse", "HEAD")
	victim := t.TempDir()
	git(victim, "init", "-b", "main")
	git(victim, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	global, system := filepath.Join(victim, "global"), filepath.Join(victim, "system")
	for _, path := range []string{global, system} {
		if err := os.WriteFile(path, []byte("[fixture]\n\tunchanged = true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	victimFiles := []string{global, system, filepath.Join(victim, ".git", "config"), filepath.Join(victim, ".git", "HEAD"), filepath.Join(victim, ".git", "refs", "heads", "main")}
	before := map[string]string{}
	for _, path := range victimFiles {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(b)
	}
	for key, value := range map[string]string{
		"GOFLAGS": "-buildvcs=false", "GIT_DIR": filepath.Join(victim, ".git"), "GIT_WORK_TREE": victim,
		"GIT_CONFIG_GLOBAL": global, "GIT_CONFIG_SYSTEM": system,
		"GIT_CONFIG_COUNT": "2", "GIT_CONFIG_KEY_0": "grep.patternType", "GIT_CONFIG_VALUE_0": "default",
		"GIT_CONFIG_KEY_1": "grep.extendedRegexp", "GIT_CONFIG_VALUE_1": "false",
		"GIT_NO_REPLACE_OBJECTS": "1", "GIT_ATTR_NOSYSTEM": "1", "GIT_CONFIG_NOSYSTEM": "1",
		"GH_TOKEN": "fixture-not-a-credential", "GITHUB_TOKEN": "fixture-not-a-credential",
	} {
		t.Setenv(key, value)
	}
	parent := os.Environ()
	sort.Strings(parent)
	out := filepath.Join(t.TempDir(), "cellctl")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	if err := compileCellctl(t, moduleRoot, out); err != nil {
		t.Fatal(err)
	}
	info, err := buildinfo.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if got := settings["vcs.revision"]; got != revision {
		t.Fatalf("compiled fixture revision = %q, want source revision %q", got, revision)
	}
	cmd := exec.Command(out, "--version")
	cmd.Env = f.Env()
	version, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled --version: %v\n%s", err, version)
	}
	want := "dev-" + revision[:shortRevisionLen]
	if settings["vcs.modified"] == "true" {
		want += "-dirty"
	}
	if got := strings.TrimSpace(string(version)); got != want {
		t.Fatalf("compiled --version = %q, want %q", got, want)
	}
	after := os.Environ()
	sort.Strings(after)
	if !reflect.DeepEqual(parent, after) {
		t.Fatal("compiler changed its parent environment")
	}
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("compiler changed victim file %s: %v", filepath.Base(path), err)
		}
	}
}
