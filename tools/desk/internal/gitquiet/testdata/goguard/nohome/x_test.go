package nohome

import (
	"os/exec"
	"testing"
)

func TestBuild(t *testing.T) { _ = exec.Command("go", "build", ".").Run() }
