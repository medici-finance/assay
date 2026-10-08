package helpernorun

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(runTests(m)) }

func runTests(m *testing.M) int { return m.Run() }

func TestRepo(t *testing.T) { _ = exec.Command("git", "init", t.TempDir()) }
