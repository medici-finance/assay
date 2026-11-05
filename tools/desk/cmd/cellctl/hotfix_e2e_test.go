//go:build hotfixseam && unix

package main

// Driver for testdata/hotfix-e2e.sh. It exists only in a build carrying the test-only
// hotfixseam tag and is skipped unless the script sets HOTFIX_E2E_SHIMGEN: shim generation is
// part of `cellctl desk`'s launch, which needs a worktree and a roster, so the script runs the
// same genShims against a throwaway cell instead.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestHotfixE2EShimGen(t *testing.T) {
	dir := os.Getenv("HOTFIX_E2E_SHIMGEN")
	if dir == "" {
		t.Skip("driven by testdata/hotfix-e2e.sh")
	}
	env := newEnvFromProcess()
	env.Put(deskkit.DeskToolsBinEnv, os.Getenv(deskkit.DeskToolsBinEnv))
	c := &Cell{
		Name: "e2e", Dir: dir, Home: filepath.Join(dir, "home"),
		Repo: os.Getenv("HOTFIX_E2E_REPO"), Env: env,
	}
	c.genShims()
}
