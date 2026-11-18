package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/gitcore/testenv"
)

// Install only at the top-level test boundary. Repository constructors must not
// erase worktree/include/environment inputs planted by authority controls.
func installDeskkitOriginFixture(t *testing.T) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	testenv.Install(t, testenv.Options{Tools: map[string]string{"git": git}})
}

// Both the native Git constructor and the production origin reader must ignore
// caller authority. Observe independent victim files as well as returned values.
func TestDeskkitFixtureParentEnvironment(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]string{"git": git}
	clean := testenv.New(t, testenv.Options{Tools: tools})
	caller := t.TempDir()
	victim := filepath.Join(caller, "victim")
	control := filepath.Join(caller, "control")
	for _, dir := range []string{victim, control} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"remote", "add", "origin", "https://github.com/owner/repo.git"}} {
			cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
			cmd.Env = clean.Env()
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("clean Git setup: %s: %v", out, err)
			}
		}
	}
	decoyDir := filepath.Join(caller, "bin")
	decoyCalls := filepath.Join(caller, "git-calls")
	if err := os.Mkdir(decoyDir, 0700); err != nil {
		t.Fatal(err)
	}
	// The caller's executable is a sentinel, never an alternate source of Git
	// answers. The fixture must use the approved tool captured above.
	decoy := filepath.Join(decoyDir, "git")
	if err := os.WriteFile(decoy, []byte("#!/bin/sh\nprintf 'called\\n' >> '"+decoyCalls+"'\nexit 97\n"), 0700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(caller, "global")
	system := filepath.Join(caller, "system")
	for path, body := range map[string]string{
		global: "[user]\n name = Caller Victim\n", system: "[user]\n name = System Victim\n",
		filepath.Join(victim, "keep"): "victim payload\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() map[string]string {
		t.Helper()
		files := map[string]string{}
		err := filepath.WalkDir(caller, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			b, err := os.ReadFile(path)
			if err == nil {
				files[path] = string(b)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := snapshot()
	t.Setenv("HOME", caller)
	t.Setenv("USERPROFILE", caller)
	t.Setenv("XDG_CONFIG_HOME", caller)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
	t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/other/steer.git")
	t.Setenv("GH_TOKEN", "fixture-parent-token")
	if runtime.GOOS != "windows" {
		t.Setenv("PATH", decoyDir)
	}
	if slug, err := OriginRepoSlug(control); !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) || slug != "" {
		t.Fatalf("inherited origin control did not fail closed: %q %v", slug, err)
	}
	// Execute the same bare-command boundary the repository constructor uses.
	// Inheritance must demonstrably hit the sentinel before isolation is tested.
	t.Run("inherited-command-control", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX command sentinel; origin and victim controls still execute")
		}
		cmd := exec.Command("git", "-C", control, "status", "--porcelain")
		if err := cmd.Run(); err == nil {
			t.Fatal("inherited Git control unexpectedly succeeded")
		}
		if calls, err := os.ReadFile(decoyCalls); err != nil || string(calls) != "called\n" {
			t.Fatalf("inherited Git did not hit sentinel: %q %v", calls, err)
		}
		if err := os.Remove(decoyCalls); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("isolated", func(t *testing.T) {
		testenv.Install(t, testenv.Options{Tools: tools})
		root, linked, _ := workSourceFixture(t, true)
		for _, dir := range []string{root, linked} {
			if slug, err := OriginRepoSlug(dir); err != nil || slug != "owner/repo" {
				t.Fatalf("isolated origin: %q %v", slug, err)
			}
			catalog, err := LocalWorkCatalog(dir, "")
			if err != nil {
				t.Fatal(err)
			}
			selected, err := catalog.Select("stream:b-08@2")
			if err != nil || selected.Subject.Repository != "owner/repo" {
				t.Fatalf("isolated work source: %+v %v", selected, err)
			}
		}
		if os.Getenv("GH_TOKEN") != "" {
			t.Fatal("parent token survived installation")
		}
		// A native global write must land only in the fixture-owned config.
		cmd := exec.Command("git", "config", "--global", "user.name", "Owned Fixture")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("owned config write: %s %v", out, err)
		}
		gitAll(t, linked, [][]string{{"config", "--worktree", "include.path", "steer.cfg"}})
		if _, err := OriginRepoSlug(linked); !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) {
			t.Fatalf("post-install include not rejected: %v", err)
		}
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "remote.origin.url")
		t.Setenv("GIT_CONFIG_VALUE_0", "https://github.com/other/steer.git")
		if _, err := OriginRepoSlug(root); !errors.Is(err, gitcore.ErrRemoteScopeUnmerged) {
			t.Fatalf("post-install injected config not rejected: %v", err)
		}
	})
	if after := snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("caller config or repository victim changed")
	}
}
