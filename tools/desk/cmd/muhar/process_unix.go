//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// setProcessGroup runs cmd in its own process group, so a deadline or an
// interrupt can kill the whole suite, grandchildren included.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killProcessGroup(cmd) }
}

// killProcessGroup kills cmd's process group. With Setpgid the group id is
// the leader's pid, and the leader is not reaped until Wait returns, so the
// id cannot have been reused by an unrelated group.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
