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
// fixture runtime. The underlying shell assertions are unchanged. The wrapped
// suites run git, so the child never inherits the caller's GIT_* variables, and
// null global and system config keep a caller's hooks out of their repositories.
func runShellFixture(ctx context.Context, path, tmp string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "bash", path)
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "TMPDIR="+tmp,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
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

// TestShellGitIsolation runs a planted shell fixture that creates and commits to its
// own repository under TMPDIR while GIT_DIR names a second repository. The wrapper
// must keep that repository byte-unchanged, as it must for the wrapped suites, which
// also run git.
func TestShellGitIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	victim := HostileGitDir(t)
	before := TreeDigest(t, victim)
	dir := t.TempDir()
	path := filepath.Join(dir, "plant.sh")
	script := "#!/usr/bin/env bash\nset -eu\nexport GIT_CONFIG_NOSYSTEM=1\n" +
		"git init -q \"$TMPDIR/plant\"\n" +
		"git -C \"$TMPDIR/plant\" -c user.name=Plant -c user.email=plant@example.invalid " +
		"-c commit.gpgsign=false commit -q --allow-empty -m plant\n"
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := runShellFixture(ctx, path, dir)
	if after := TreeDigest(t, victim); after != before {
		t.Fatalf("shell fixture wrote to the GIT_DIR-named repository %s\n%s", victim, out)
	}
	if err != nil {
		t.Fatalf("planted fixture failed in its own repository: %v\n%s", err, out)
	}
}
