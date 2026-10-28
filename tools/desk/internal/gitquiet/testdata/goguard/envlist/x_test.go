package envlist

import (
	"os/exec"
	"testing"
)

func TestBuild(t *testing.T) {
	cmd := exec.Command("go", "build", ".")
	cmd.Env = []string{"HOME=" + t.TempDir()}
	_ = cmd.Run()
}
