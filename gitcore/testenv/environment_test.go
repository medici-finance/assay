package testenv

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/gitquiet"
)

func TestMain(m *testing.M) { os.Exit(gitquiet.Run(m)) }

func gitTool(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFixtureEnvironment(t *testing.T) {
	git := gitTool(t)
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "basic")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "1")
	t.Setenv("GH_TOKEN", "synthetic-parent-value")
	t.Setenv("GITLAB_TOKEN", "synthetic-parent-value")
	t.Setenv("BASH_ENV", "unowned-startup")
	t.Setenv("ASSAY_CONFIG_HOME", "unowned-config")
	t.Setenv("FIXTURE_UNDECLARED", "parent")
	before := os.Environ()
	t.Run("scoped", func(t *testing.T) {
		f := Install(t, Options{Tools: map[string]string{"git": git}, Vars: map[string]string{"FIXTURE_EXPLICIT": "yes"}})
		for _, key := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_NO_REPLACE_OBJECTS", "GH_TOKEN", "GITLAB_TOKEN", "BASH_ENV", "ASSAY_CONFIG_HOME", "FIXTURE_UNDECLARED"} {
			if _, exists := os.LookupEnv(key); exists {
				t.Fatalf("undeclared parent key survived: %s", key)
			}
		}
		if os.Getenv("HOME") != f.Home || os.Getenv("PATH") != f.Bin {
			t.Fatal("fixture does not own HOME/PATH")
		}
		if !reflect.DeepEqual(asMap(os.Environ()), asMap(f.Env())) {
			t.Fatal("process and child base environments differ")
		}
		first := f.Env("FIXTURE_EXPLICIT=first", "FIXTURE_EXPLICIT=last", "GH_TOKEN=synthetic-fixture-value")
		if got := asMap(first); got["FIXTURE_EXPLICIT"] != "last" || got["GH_TOKEN"] != "synthetic-fixture-value" {
			t.Fatal("explicit overlays lost")
		}
		t.Setenv("FIXTURE_LATE", "not-in-snapshot")
		if _, exists := asMap(f.Env())["FIXTURE_LATE"]; exists {
			t.Fatal("Env read ambient state after installation")
		}
		first[0] = "MUTATED=caller"
		if _, exists := asMap(f.Env())["MUTATED"]; exists {
			t.Fatal("returned environment aliases stored base")
		}
		// A security test may deliberately replant rejected input after setup.
		t.Setenv("GIT_CONFIG_COUNT", "1")
		if os.Getenv("GIT_CONFIG_COUNT") != "1" || asMap(f.Env("GIT_CONFIG_COUNT=1"))["GIT_CONFIG_COUNT"] != "1" {
			t.Fatal("deliberate hostile controls were suppressed")
		}
		cmd := exec.Command("git", "init", "-q", t.TempDir())
		cmd.Env = f.Env()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("explicit child git: %v: %s", err, out)
		}
		if _, err := exec.LookPath("undeclared-fixture-tool"); err == nil {
			t.Fatal("undeclared executable found")
		}
	})
	if !reflect.DeepEqual(asMap(before), asMap(os.Environ())) {
		t.Fatal("cleanup did not restore parent presence and values")
	}
	t.Run("platform-keys", func(t *testing.T) {
		if keyFor("Path", "windows") != keyFor("PATH", "windows") || keyFor("Path", "linux") == keyFor("PATH", "linux") {
			t.Fatal("platform environment comparison changed")
		}
		for _, key := range []string{"path", "HOME", "Temp", "XDG_CONFIG_HOME"} {
			if !reserved(key) {
				t.Fatalf("owned key %s accepted", key)
			}
		}
	})
}

func asMap(env []string) map[string]string {
	m := map[string]string{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		m[key] = value
	}
	return m
}

func TestFixtureVictimUnchanged(t *testing.T) {
	git := gitTool(t)
	root := t.TempDir()
	victim, global := filepath.Join(root, "victim"), filepath.Join(root, "parent-global")
	parentEnv := []string{"PATH=" + filepath.Dir(git), "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + global}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		parentEnv = append(parentEnv, "SystemRoot="+systemRoot)
	}
	if err := os.WriteFile(global, []byte("[fixture]\n\tvalue = parent\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(env []string, args ...string) []byte {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("victim control git %v: %v: %s", args, err, out)
		}
		return out
	}
	run(parentEnv, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "init", "-q", victim)
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(victim, ".git", "index"))
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "fixture.injected")
	t.Setenv("GIT_CONFIG_VALUE_0", "parent")
	// The unsanitized positive control really can reach the disposable victim.
	run(os.Environ(), "config", "fixture.control", "reached")
	run(os.Environ(), "config", "--global", "fixture.control", "reached")
	beforeRepo, err := os.ReadFile(filepath.Join(victim, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	beforeGlobal, err := os.ReadFile(global)
	if err != nil {
		t.Fatal(err)
	}
	beforeTree := snapshot(t, victim)
	fixtureRepo := filepath.Join(root, "fixture")
	t.Run("isolated", func(t *testing.T) {
		f := Install(t, Options{Tools: map[string]string{"git": git}})
		run(f.Env(), "init", "-q", fixtureRepo)
		run(f.Env(), "-C", fixtureRepo, "config", "fixture.changed", "fixture-only")
		run(f.Env(), "config", "--global", "fixture.changed", "fixture-only")
		for _, setting := range []string{"maintenance.auto", "gc.auto"} {
			got := strings.TrimSpace(string(run(f.Env(), "-C", fixtureRepo, "config", "--local", setting)))
			if got != "false" && got != "0" {
				t.Fatalf("owned template lost %s", setting)
			}
		}
	})
	afterRepo, err := os.ReadFile(filepath.Join(victim, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	afterGlobal, err := os.ReadFile(global)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeRepo, afterRepo) || !bytes.Equal(beforeGlobal, afterGlobal) || !reflect.DeepEqual(beforeTree, snapshot(t, victim)) {
		t.Fatal("fixture changed parent repository or global config")
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var body []byte
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			body = []byte(target)
		} else if info.Mode().IsRegular() {
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("%v:%x", info.Mode(), sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFixtureRejectedOptions(t *testing.T) {
	if option := os.Getenv("TESTENV_BAD_OPTION"); option != "" {
		opts := Options{}
		switch option {
		case "relative":
			opts.Tools = map[string]string{"git": "relative/git"}
		case "missing":
			opts.Tools = map[string]string{"git": filepath.Join(t.TempDir(), "absent")}
		case "directory":
			opts.Tools = map[string]string{"git": t.TempDir()}
		case "escape":
			opts.Tools = map[string]string{"../git": os.Args[0]}
		case "home":
			opts.Vars = map[string]string{"HOME": t.TempDir()}
		}
		f := Install(t, opts)
		if option == "path-extra" {
			f.Env("PATH=ambient")
		}
		if option == "malformed" {
			f.Env("not-an-assignment")
		}
		return
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"relative", "missing", "directory", "escape", "home", "path-extra", "malformed"} {
		t.Run(option, func(t *testing.T) {
			cmd := exec.Command(binary, "-test.run=^TestFixtureRejectedOptions$")
			cmd.Env = append(os.Environ(), "TESTENV_BAD_OPTION="+option)
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte("testenv:")) {
				t.Fatalf("invalid option accepted or failed for wrong reason: %v: %s", err, out)
			}
		})
	}
}
