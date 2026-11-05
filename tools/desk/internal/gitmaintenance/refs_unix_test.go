//go:build unix

package gitmaintenance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

// regression: #2525
func TestTempUnreadableHolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind root")
	}
	root, old := refWorld(t)
	a := seedTemp(t, root, old)
	wt := filepath.Join(t.TempDir(), "held")
	refGit(t, root, "worktree", "add", "-q", "-b", "held", wt)
	common, err := gitcore.CommonDir(root)
	if err != nil {
		t.Fatal(err)
	}
	// A branch that exists but cannot be read is not an unborn branch.
	branch := filepath.Join(common, "refs", "heads", "held")
	if err := os.Chmod(branch, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(branch, 0o600) })
	if row, err := a.Complete(context.Background()); err == nil || row.State != "held" {
		t.Fatalf("unreadable branch holder=%+v,%v", row, err)
	}
	assertRef(t, root, a.Name, old)
}
