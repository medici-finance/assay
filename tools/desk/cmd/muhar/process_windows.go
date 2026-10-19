//go:build windows

package main

import "os/exec"

// setProcessGroup is a no-op on Windows: there is no process group to join,
// so a deadline or an interrupt kills the suite's top process only.
func setProcessGroup(cmd *exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
