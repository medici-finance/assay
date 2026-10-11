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

// The existing live build-test job runs this package on every push and PR.
// These suites use API stubs and scratch fixtures; none contacts GitHub.
func TestEvidenceAutomergeRefusal(t *testing.T) {
	shellFloor(t, "tools/evidence-automerge/automerge-refusal_test.sh")
}

func TestEvidenceAutomergeStep(t *testing.T) {
	shellFloor(t, "tools/evidence-automerge/automerge-step_test.sh")
}

func TestEvidenceAutomergePoll(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX workflow fixtures run in the Linux build-test job")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), shellBudget)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(root, "tools/evidence-automerge/automerge-poll_test.py"))
	cmd.Env = FixtureEnv("KUBECONFIG=/dev/null", "TMPDIR="+t.TempDir(),
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("offline Evidence poll suite: %v (deadline: %v)\n%s", err, ctx.Err(), out)
	}
	t.Log(string(out))
}
