package regression

import (
	"context"
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
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(root, filepath.FromSlash(relative)))
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null", "TMPDIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
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
