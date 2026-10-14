package gitmaintenance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// regression: #2513
func TestHooksRefuseBeforeLock(t *testing.T) {
	for _, kind := range []string{"default", "local-path", "global-path", "relative-worktree", "worktree-config", "configured", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			target := repo
			hookDir := filepath.Join(repo, ".git", "hooks")
			if kind == "relative-worktree" || kind == "worktree-config" {
				target = filepath.Join(t.TempDir(), "linked")
				fixtureGit(t, repo, "worktree", "add", "--detach", target)
				hookDir = filepath.Join(target, "local-hooks")
				if kind == "worktree-config" {
					fixtureGit(t, repo, "config", "extensions.worktreeConfig", "true")
					fixtureGit(t, target, "config", "--worktree", "core.hooksPath", "local-hooks")
				} else {
					fixtureGit(t, repo, "config", "core.hooksPath", "local-hooks")
				}
			}
			if kind == "local-path" || kind == "global-path" {
				hookDir = t.TempDir()
				if kind == "local-path" {
					fixtureGit(t, repo, "config", "core.hooksPath", hookDir)
				} else {
					text := fmt.Sprintf("[core]\n hooksPath = %q\n", hookDir)
					if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".gitconfig"), []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := os.MkdirAll(hookDir, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "marker")
			script := "#!/bin/sh\nprintf fired > '" + filepath.ToSlash(marker) + "'\n"
			path := filepath.Join(hookDir, "reference-transaction")
			if kind == "configured" || kind == "disabled" {
				path = filepath.Join(hookDir, "configured-script")
				fixtureGit(t, repo, "config", "hook.fixture.event", "reference-transaction")
				fixtureGit(t, repo, "config", "hook.fixture.command", "sh '"+filepath.ToSlash(path)+"'")
				if kind == "disabled" {
					fixtureGit(t, repo, "config", "hook.fixture.enabled", "false")
				}
			}
			if err := os.WriteFile(path, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			before := diskImage(t, repo)
			linkedBefore := diskImage(t, target)
			homeBefore := diskImage(t, os.Getenv("HOME"))
			for _, dry := range []bool{true, false} {
				if _, err := Run(context.Background(), target, Options{DryRun: dry}); err == nil {
					t.Fatal("reference-transaction hook accepted; hooks must remain enforced")
				}
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) || !reflect.DeepEqual(linkedBefore, diskImage(t, target)) {
				t.Fatal("hook refusal changed repository or attempt bookkeeping")
			}
			if !reflect.DeepEqual(homeBefore, diskImage(t, os.Getenv("HOME"))) {
				t.Fatal("hook refusal rewrote global configuration")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("maintenance executed a configured hook")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != script {
				t.Fatal("hook was changed")
			}
		})
	}
}

// regression: #2513
func TestBadRefInventoryStops(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	ref := filepath.Join(repo, ".git", "refs", "heads", "corrupt")
	if err := os.WriteFile(ref, []byte("not-an-object-id\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Ref inventory follows both cooperative locks; seed each known bookkeeping file.
	for _, name := range []string{"assay-maintenance.lock", "assay-maintenance-temprefs.lock"} {
		if err := os.WriteFile(filepath.Join(repo, ".git", name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := diskImage(t, repo)
	for _, dry := range []bool{true, false} {
		if _, err := Run(context.Background(), repo, Options{DryRun: dry}); err == nil {
			t.Fatal("Git warning omitted a corrupt ref but inventory reported complete")
		}
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatalf("incomplete ref inventory permitted mutation: %v", snapshotChanges(before, diskImage(t, repo)))
	}
}

// regression: #2513
func TestHookChangeStopsTasks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX diagnostic shim")
	}
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shim := t.TempDir()
	hook := filepath.Join(repo, ".git", "hooks", "reference-transaction")
	script := "#!/bin/sh\ncase \" $* \" in *' count-objects '*) printf '#!/bin/sh\\nexit 0\\n' > '" + hook + "'; chmod 755 '" + hook + "';; esac\nexec '" + gitPath + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shim+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := Run(context.Background(), repo, Options{}); err == nil {
		t.Fatal("hook added during inventory permitted mutation")
	}
	if _, err := os.Stat(hook); err != nil {
		t.Fatal("positive control: no hook was planted")
	}
	for _, path := range []string{"packed-refs", "assay-maintenance.json", "objects/info/commit-graph"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("hook change permitted task or success stamp %s", path)
		}
	}
}

// regression: #2513
func TestHookFailureHelper(t *testing.T) {
	switch os.Getenv("ASSAY_FIXTURE_HOOK_FAILURE") {
	case "unsupported":
		fmt.Fprintln(os.Stderr, "error: unknown subcommand: list")
		os.Exit(129)
	case "empty-success":
		os.Exit(0)
	case "empty-failure":
		os.Exit(1)
	}
}

// regression: #2513
func TestHookInventoryFailClosed(t *testing.T) {
	for _, kind := range []string{"unsupported", "empty-success", "empty-failure"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			before := diskImage(t, repo)
			t.Setenv("ASSAY_FIXTURE_HOOK_FAILURE", kind)
			start := func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if strings.Contains(" "+strings.Join(args, " ")+" ", " hook list ") {
					return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHookFailureHelper$")
				}
				return exec.CommandContext(ctx, name, args...)
			}
			if err := CheckHooksWithStart(context.Background(), repo, start); err == nil {
				t.Fatal("uncertain native hook inventory accepted")
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) {
				t.Fatal("hook inventory failure wrote repository state")
			}
		})
	}
}
