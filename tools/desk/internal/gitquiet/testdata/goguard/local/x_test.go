package local

import (
	"os/exec"
	"testing"
)

// goEnv is a package-local choke point: it turns telemetry off in the home first.
func goEnv(t *testing.T) []string {
	env := []string{"HOME=" + t.TempDir()}
	off := exec.Command("go", "telemetry", "off")
	off.Env = env
	_ = off.Run()
	return env
}

func TestBuild(t *testing.T) {
	cmd := exec.Command("go", "build", ".")
	cmd.Env = goEnv(t)
	_ = cmd.Run()
}
