package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/regression"
)

func TestMaintenanceHookPolicy(t *testing.T) {
	for _, kind := range []string{"default", "local-path", "global-path", "relative-worktree", "worktree-config", "configured-local", "configured-global", "configured-disabled", "configured-spoof", "configured-reset", "unrelated"} {
		t.Run(kind, func(t *testing.T) {
			work, calls := maintenanceFixture(t)
			target := work
			if kind == "relative-worktree" || kind == "worktree-config" {
				target = filepath.Join(t.TempDir(), "linked")
				mustGit(t, work, "worktree", "add", "--detach", target)
			}
			hookDir := filepath.Join(work, ".git", "hooks")
			switch kind {
			case "local-path", "global-path":
				hookDir = t.TempDir()
				scope := "--local"
				if kind == "global-path" {
					scope = "--global"
				}
				mustGit(t, work, "config", scope, "core.hooksPath", hookDir)
			case "relative-worktree", "worktree-config":
				hookDir = filepath.Join(target, "local-hooks")
				if kind == "worktree-config" {
					mustGit(t, work, "config", "extensions.worktreeConfig", "true")
					mustGit(t, target, "config", "--worktree", "core.hooksPath", "local-hooks")
				} else {
					mustGit(t, target, "config", "core.hooksPath", "local-hooks")
				}
			}
			if err := os.MkdirAll(hookDir, 0700); err != nil {
				t.Fatal(err)
			}
			event := "reference-transaction"
			if kind == "unrelated" {
				event = "pre-commit"
			}
			hook := filepath.Join(hookDir, event)
			marker := filepath.Join(t.TempDir(), "marker")
			// The fixture writes a local marker only. It never reads credentials or uses a network.
			body := "#!/bin/sh\nprintf ran > '" + filepath.ToSlash(marker) + "'\n"
			if strings.HasPrefix(kind, "configured-") {
				hook = filepath.Join(hookDir, "fixture-command")
				scope, name := "--local", "maintenance-fixture"
				if kind == "configured-global" {
					scope = "--global"
				}
				if kind == "configured-spoof" {
					name = "disabled\tfixture"
				}
				mustGit(t, work, "config", scope, "hook."+name+".event", event)
				mustGit(t, work, "config", scope, "hook."+name+".command", "sh '"+filepath.ToSlash(hook)+"'")
				if kind == "configured-reset" {
					mustGit(t, work, "config", "--add", "hook."+name+".event", "")
				}
				if kind == "configured-disabled" {
					mustGit(t, work, "config", scope, "hook."+name+".enabled", "false")
				}
			}
			writeFile(t, hook, body)
			if err := os.Chmod(hook, 0755); err != nil {
				t.Fatal(err)
			}
			config := mustGit(t, target, "config", "--list", "--show-origin")
			report, err := maintenanceRun(t, "--repo", target, "--apply")
			if kind == "unrelated" || kind == "configured-reset" {
				if err != nil || report.Status != "completed" {
					t.Fatalf("unrelated hook held maintenance: %v %+v", err, report)
				}
			} else {
				if err == nil || maintenanceWrites(*calls) != 0 {
					t.Fatalf("relevant hook accepted: %v writes=%d", err, maintenanceWrites(*calls))
				}
				if _, err := os.Stat(filepath.Join(work, ".git", "assay-maintenance.lock")); !os.IsNotExist(err) {
					t.Fatal("refusal wrote attempt stamp")
				}
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("marker hook ran")
			}
			got, err := os.ReadFile(hook)
			if err != nil || string(got) != body {
				t.Fatal("hook changed")
			}
			if mustGit(t, target, "config", "--list", "--show-origin") != config {
				t.Fatal("hook configuration changed")
			}
		})
	}
}

func TestMaintenanceHookInventoryFailure(t *testing.T) {
	for _, kind := range []string{"unsupported", "missing-command", "missing-directory", "broken-link", "unreadable-directory"} {
		t.Run(kind, func(t *testing.T) {
			work, calls := maintenanceFixture(t)
			switch kind {
			case "unsupported":
				old := maintenanceCommand
				maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
					if strings.Contains(strings.Join(args, " "), "hook list") {
						return exec.CommandContext(ctx, "git", "unsupported-fixture-command")
					}
					return old(ctx, name, args...)
				}
			case "missing-command":
				mustGit(t, work, "config", "hook.fixture.event", "reference-transaction")
			case "missing-directory":
				mustGit(t, work, "config", "core.hooksPath", filepath.Join(t.TempDir(), "missing"))
			case "broken-link":
				if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), filepath.Join(work, ".git", "hooks", "reference-transaction")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "unreadable-directory":
				if runtime.GOOS == "windows" {
					t.Skip("POSIX directory permissions")
				}
				p := filepath.Join(work, ".git", "hooks")
				if err := os.Chmod(p, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(p, 0755) })
				f, err := os.Open(p)
				if err == nil {
					f.Close()
					t.Skip("process can read mode-000 directory")
				}
			}
			_, err := maintenanceRun(t, "--repo", work, "--apply")
			if err == nil || maintenanceWrites(*calls) != 0 {
				t.Fatalf("uncertain hook inventory accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(work, ".git", "assay-maintenance.lock")); !os.IsNotExist(err) {
				t.Fatal("uncertain inventory wrote stamp")
			}
		})
	}
}

func TestMaintenanceHookChangeAfterInventory(t *testing.T) {
	work, calls := maintenanceFixture(t)
	old := maintenanceCommand
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "count-objects") {
			p := filepath.Join(work, ".git", "hooks", "reference-transaction")
			writeFile(t, p, "#!/bin/sh\nexit 0\n")
			if err := os.Chmod(p, 0755); err != nil {
				t.Fatal(err)
			}
		}
		return old(ctx, name, args...)
	}
	_, err := maintenanceRun(t, "--repo", work, "--apply")
	if err == nil || maintenanceWrites(*calls) != 0 {
		t.Fatalf("changed hook inventory accepted: %v", err)
	}
}

// regression: #6389, #6300
func TestMaintenanceFixtureGlobalVictim(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	victim := t.TempDir()
	home := filepath.Join(victim, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(home, ".gitconfig")
	const original = "[fixture]\n\towner = caller\n"
	if err := os.WriteFile(global, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	// Build the victim independently, with a literal environment rather than
	// the isolation helper whose adoption this control is meant to detect.
	victimGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home,
			"GIT_CONFIG_GLOBAL=" + global, "GIT_CONFIG_SYSTEM=" + os.DevNull,
			"GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0"}
		if root := os.Getenv("SYSTEMROOT"); root != "" {
			cmd.Env = append(cmd.Env, "SYSTEMROOT="+root)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("victim setup: %v: %s", err, out)
		}
	}
	repo := filepath.Join(victim, "repo")
	victimGit("-c", "maintenance.auto=false", "-c", "gc.auto=0", "init", "-b", "main", repo)
	victimGit("-C", repo, "-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "victim")
	victimGit("config", "--global", "fixture.probe", "inherited-control")
	if got, err := os.ReadFile(global); err != nil || string(got) == original {
		t.Fatal("positive control did not modify the disposable inherited config")
	}
	if err := os.WriteFile(global, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	before := regression.SnapshotTree(t, victim)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "2")
	t.Setenv("GIT_CONFIG_KEY_0", "grep.patternType")
	t.Setenv("GIT_CONFIG_VALUE_0", "basic")
	t.Setenv("GIT_CONFIG_KEY_1", "grep.extendedRegexp")
	t.Setenv("GIT_CONFIG_VALUE_1", "false")
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "1")
	work, _ := maintenanceFixture(t)
	mustGit(t, work, "config", "--global", "fixture.probe", "fixture-owned")
	if got := mustGit(t, work, "config", "--global", "fixture.probe"); got != "fixture-owned" {
		t.Fatalf("fixture global config was not writable: %q", got)
	}
	if after := regression.SnapshotTree(t, victim); !reflect.DeepEqual(before, after) {
		t.Fatal("fixture modified the caller config or victim repository")
	}
}
