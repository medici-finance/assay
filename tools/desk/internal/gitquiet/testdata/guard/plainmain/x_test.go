package plainmain

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(m.Run()) }

func TestRepo(t *testing.T) { _ = exec.Command("git", "init", t.TempDir()) }
