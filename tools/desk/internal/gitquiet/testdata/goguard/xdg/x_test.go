package xdg

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "xdg-")
	_ = os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestBuild(t *testing.T) { _ = exec.Command("go", "vet", ".").Run() }
