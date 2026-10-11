package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// regression: #2647
// TestRunScriptOffline runs tools/gremlins/run_test.sh, the offline suite for the
// sweep script (stub Gremlins, canned build info for the pin check), so the desk
// module's ordinary test run covers the script as well as this tool.
func TestRunScriptOffline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX script")
	}
	if _, err := exec.LookPath("timeout"); err != nil {
		if runtime.GOOS == "linux" {
			t.Fatal("GNU timeout is missing; the sweep script cannot be checked")
		}
		t.Skip("GNU timeout not installed on this host; the Linux CI run checks the script")
	}
	script, err := filepath.Abs("../../../gremlins/run_test.sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", script)
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null", "TMPDIR="+t.TempDir())
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run_test.sh: %v\n%s", err, out)
	}
	t.Log(string(out))
}
