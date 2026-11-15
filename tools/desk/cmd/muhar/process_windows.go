//go:build windows

package main

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op on Windows: there is no process group to join,
// so a deadline or an interrupt kills the suite's top process only.
func setProcessGroup(cmd *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// Windows process exits do not expose Unix resource-signal classifications.
func resourceSignal(*os.ProcessState) bool { return false }
