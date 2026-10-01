package regression

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func shellFloor(t *testing.T, relative string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fleet/shim fixture: exercised on Linux/macOS; native Windows behavior has its own suite")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := runShellFixture(ctx, filepath.Join(root, filepath.FromSlash(relative)), t.TempDir())
	if err != nil {
		t.Fatalf("fixture suite %s: %v\n%s", relative, err, out)
	}
	t.Log(string(out))
}

// TestReg786FleetHardening pins #786, fixed by 643637114: the
// existing fixture suite checks argv redaction AND transport-error retention.
func TestReg786FleetHardening(t *testing.T) { shellFloor(t, "tools/create-fleet-gitlab_test.sh") }

// TestReg1145ShimCredential pins #1145, fixed by f84dde307. The
// shell oracle is the preserved implementation; its fixtures include the
// later role-token isolation correction, so this never grants ambient auth.
func TestReg1145ShimCredential(t *testing.T) {
	shellFloor(t, "tools/cellctl/tests/gen-shims-gh-token.test.sh")
}

// Keep the deadline finite while allowing headroom above the observed 23s
// fixture runtime. The underlying shell assertions are unchanged.
func runShellFixture(ctx context.Context, path, tmp string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null", "TMPDIR="+tmp)
	cmd.WaitDelay = time.Second
	return cmd.CombinedOutput()
}

func TestShellDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "deadline.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nexec sleep 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := runShellFixture(ctx, path, dir)
	if err == nil || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("deadline fixture completed without cancellation: err=%v context=%v", err, ctx.Err())
	}
}
