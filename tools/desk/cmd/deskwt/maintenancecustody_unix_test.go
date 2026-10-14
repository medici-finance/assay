//go:build unix

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The old timeout/foreign-marker fixtures never killed an actual coordinator
// while its launched child remained alive after the OS locks were released.
// regression: #2513
func TestLegacyCrashHelper(t *testing.T) {
	mode := os.Getenv("ASSAY_LEGACY_CRASH_MODE")
	if len(os.Args) > 1 && os.Args[len(os.Args)-1] == "legacy-child" {
		if err := os.WriteFile(os.Getenv("ASSAY_LEGACY_CHILD_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Second)
		return
	}
	if mode != "coordinator" {
		return
	}
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "pack-refs") {
			return exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLegacyCrashHelper$", "--", "legacy-child")
		}
		return exec.CommandContext(ctx, name, args...)
	}
	if err := cmdMaintain([]string{"--repo", os.Getenv("ASSAY_LEGACY_REPO"), "--apply", "--timeout", "10s"}); err != nil {
		t.Fatal(err)
	}
}

// regression: #2513
func TestLegacyCoordinatorCrash(t *testing.T) {
	work, calls := maintenanceFixture(t)
	pidPath := filepath.Join(t.TempDir(), "child-pid")
	cmd := exec.Command(os.Args[0], "-test.run=^TestLegacyCrashHelper$")
	cmd.Env = append(os.Environ(), "ASSAY_LEGACY_CRASH_MODE=coordinator", "ASSAY_LEGACY_CHILD_PID="+pidPath, "ASSAY_LEGACY_REPO="+work)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	deadline := time.Now().Add(5 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(pidPath)
		if err == nil {
			pid, err = strconv.Atoi(string(b))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("child never started: %s", &output)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("fixture lost surviving child witness: %v", err)
	}
	marker := filepath.Join(work, ".git", "assay-maintenance-active.json")
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("crash lost child custody marker: %v", err)
	}
	before := maintenanceWrites(*calls)
	if _, err := maintenanceRun(t, "--repo", work, "--apply"); err == nil || !strings.Contains(err.Error(), "process cleanup is unconfirmed") {
		t.Fatalf("next coordinator accepted unconfirmed child: %v", err)
	}
	if maintenanceWrites(*calls) != before {
		t.Fatal("next coordinator launched mutating child")
	}
}

// regression: #2513
func TestLegacyDeadlineTree(t *testing.T) {
	work, _ := maintenanceFixture(t)
	dir := t.TempDir()
	started, escaped := filepath.Join(dir, "started"), filepath.Join(dir, "escaped")
	base := maintenanceCommand
	maintenanceCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "pack-refs") {
			body := "printf started > '" + started + "'; (sleep 2; printf escaped > '" + escaped + "') & wait"
			return exec.CommandContext(ctx, "sh", "-c", body)
		}
		return base(ctx, name, args...)
	}
	before := time.Now()
	if _, err := maintenanceRun(t, "--repo", work, "--apply", "--timeout", "1s"); err == nil {
		t.Fatal("deadline reported success")
	}
	if time.Since(before) > 4*time.Second {
		t.Fatal("deadline cleanup exceeded bounded budget")
	}
	if _, err := os.Stat(started); err != nil {
		t.Fatalf("child tree never started: %v", err)
	}
	time.Sleep(2200 * time.Millisecond)
	if _, err := os.Stat(escaped); !os.IsNotExist(err) {
		t.Fatal("descendant escaped coordinator deadline")
	}
	if _, err := os.Stat(filepath.Join(work, ".git", "assay-maintenance-active.json")); !os.IsNotExist(err) {
		t.Fatalf("confirmed deadline cleanup retained marker: %v", err)
	}
}
