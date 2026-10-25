package setenv

import (
	"os/exec"
	"testing"
)

func TestBuild(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_ = exec.Command("go", "build", ".").Run()
}
