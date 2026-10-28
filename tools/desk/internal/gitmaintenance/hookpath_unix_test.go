//go:build unix

package gitmaintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// regression: #2513
func TestSpecialPathHelper(t *testing.T) {
	repo := os.Getenv("ASSAY_FIXTURE_HOOK_REPO")
	if repo == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Run(ctx, repo, Options{DryRun: os.Getenv("ASSAY_FIXTURE_HOOK_DRY") == "1"}); err == nil {
		t.Fatal("uncertain filesystem hook state accepted")
	}
}

// regression: #2513
func TestHookPathsStayBounded(t *testing.T) {
	for _, kind := range []string{"fifo", "fifo-link", "broken-link", "live-link", "unreadable-directory"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			path := filepath.Join(t.TempDir(), "hooks")
			switch kind {
			case "fifo", "fifo-link":
				if err := unix.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "fifo-link" {
					link := path + "-link"
					if err := os.Symlink(path, link); err != nil {
						t.Fatal(err)
					}
					path = link
				}
			case "broken-link":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+"-missing", filepath.Join(path, "reference-transaction")); err != nil {
					t.Fatal(err)
				}
			case "live-link":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(t.TempDir(), "hook-script")
				if err := os.WriteFile(source, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(source, filepath.Join(path, "reference-transaction")); err != nil {
					t.Fatal(err)
				}
			case "unreadable-directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0700) })
				if f, err := os.Open(path); err == nil {
					f.Close()
					t.Skip("process can read mode-000 directory")
				}
			}
			fixtureGit(t, repo, "config", "core.hooksPath", path)
			before := diskImage(t, repo)
			t.Setenv("ASSAY_FIXTURE_HOOK_REPO", repo)
			for _, dry := range []string{"0", "1"} {
				t.Setenv("ASSAY_FIXTURE_HOOK_DRY", dry)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSpecialPathHelper$")
				out, err := cmd.CombinedOutput()
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					t.Fatal("hook-path inspection exceeded its work bound")
				}
				if err != nil {
					t.Fatalf("bounded child failed: %v\n%s", err, out)
				}
			}
			if !reflect.DeepEqual(before, diskImage(t, repo)) {
				t.Fatal("uncertain hook inventory changed repository bookkeeping")
			}
		})
	}
}

// regression: #2513
func TestSpecialLockHelper(t *testing.T) {
	repo := os.Getenv("ASSAY_FIXTURE_LOCK_REPO")
	if repo == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Run(ctx, repo, Options{}); err == nil {
		t.Fatal("special lock path accepted")
	}
}

// regression: #2513
func TestLockPathsStayBounded(t *testing.T) {
	for _, kind := range []string{"symlink", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			repo := fixtureRepo(t)
			requireNativeHooks(t, repo)
			lock := filepath.Join(repo, ".git", "assay-maintenance.lock")
			target := filepath.Join(t.TempDir(), "external")
			if kind == "symlink" {
				if err := os.WriteFile(target, []byte("held external state\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, lock); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := unix.Mkfifo(lock, 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("ASSAY_FIXTURE_LOCK_REPO", repo)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSpecialLockHelper$")
			out, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("special lock open exceeded bound")
			}
			if err != nil {
				t.Fatalf("lock helper failed: %v\n%s", err, out)
			}
			if kind == "symlink" {
				got, err := os.ReadFile(target)
				if err != nil || string(got) != "held external state\n" {
					t.Fatal("lock followed symlink and touched external state")
				}
			}
			if _, err := os.Stat(filepath.Join(repo, ".git", "assay-maintenance.json")); !os.IsNotExist(err) {
				t.Fatal("special lock produced success stamp")
			}
		})
	}
}
