package viahelper

import (
	"os"
	"os/exec"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

func TestMain(m *testing.M) { os.Exit(runTests(m)) }

func runTests(m *testing.M) int { return gitquiet.Run(m) }

func TestRepo(t *testing.T) { _ = exec.Command("git", "init", t.TempDir()) }
