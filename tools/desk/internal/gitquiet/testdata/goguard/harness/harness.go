package harness

import (
	"os/exec"
	"testing"
)

// Build is shared test support: a non-test file that imports "testing".
func Build(t testing.TB, dir string) error {
	cmd := exec.Command("go", "build", ".")
	cmd.Dir = dir
	return cmd.Run()
}
