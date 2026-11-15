package gitcore

import (
	"os/exec"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// regression: #6510
func TestPushUpdatedReportsNoop(t *testing.T) {
	local := gittest.NewFixture(t)
	remoteDir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remoteDir).CombinedOutput(); err != nil {
		t.Fatalf("init bare remote: %v: %s", err, out)
	}
	repo, err := Open(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	opts := PushOpts{URL: remoteDir, RefSpecs: []string{"refs/heads/main:refs/heads/main"}}
	push := func(step string, want bool) {
		t.Helper()
		updated, err := repo.PushUpdated(opts)
		if err != nil {
			t.Fatalf("%s: PushUpdated: %v", step, err)
		}
		if updated != want {
			t.Fatalf("%s: updated = %v, want %v", step, updated, want)
		}
	}
	push("first push (creates the ref)", true)
	push("second push (remote already holds it)", false)
	local.CommitFile(t, "next.txt", "next\n", "next commit")
	push("push after a new commit", true)
	if err := repo.Push(opts); err != nil {
		t.Fatalf("Push of an up-to-date ref must stay success: %v", err)
	}
}
