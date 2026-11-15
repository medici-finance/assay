// Package planted is a fixture for TestDesktokenExecGuardPlanted: every shape a caller takes
// to fork the desktoken program instead of minting in process. It is never compiled (testdata).
package planted

import (
	"fmt"
	"os/exec"
)

// A package-level program name a later exec reads.
var plantedProgram = "desktoken"

var execCommand = exec.Command

func plantedDirectFork(repo string) error {
	return execCommand("desktoken", "worker", "--repo", repo).Run()
}

func forkViaVariable(repo string) error {
	name := "desktoken"
	return exec.Command(name, "desk", "--repo", repo).Run()
}

func forkViaLookPath() (string, error) {
	return exec.LookPath("desktoken")
}

func forkWindowsName(repo string) error {
	return exec.Command("desktoken.exe", "worker", "--repo", repo).Run()
}

func plantedInstallPath(repo string) error {
	return exec.Command("/opt/example/bin/desktoken", "worker", "--repo", repo).Run()
}

type minter struct{}

func (minter) forkFromMethod(repo string) error {
	return exec.Command("desktoken", "verifier", "--repo", repo).Run()
}

// Shapes that must NOT count: a message that mentions the program, a different program whose
// name merely starts with it, and a comment naming "desktoken".
func notAFork(repo string) error {
	_ = "desktokens"
	_ = "desktoken-helper"
	return fmt.Errorf("desktoken worker --repo %s failed", repo)
}
