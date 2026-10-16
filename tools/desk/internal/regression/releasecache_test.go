package regression

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// regression: #138
// Includes the release-owned image producer/consumer contract.
func TestReleaseGoCache(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release workflow executes on Linux; fixtures run in build-test")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "-B", "-m", "unittest", "discover", "-s", "tools/release", "-p", "*_test.py", "-v")
	cmd.Dir = root
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "PYTHONDONTWRITEBYTECODE=1")
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("release cache fixtures: %v\n%s", err, out)
	}
	t.Log(string(out))
}
