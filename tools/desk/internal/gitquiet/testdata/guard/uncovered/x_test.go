package uncovered

import (
	"os/exec"
	"testing"
)

func TestRepo(t *testing.T) { _ = exec.Command("git", "init", t.TempDir()) }
