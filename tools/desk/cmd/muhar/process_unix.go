//go:build !windows

package main

import (
	"os"
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

// terminationSignals are the signals a host, scheduler, terminal or resource
// limit sends to stop a process. Fault signals (SIGABRT, SIGSEGV, ...) are not
// listed: a mutant that crashes the suite is an ordinary failure.
var terminationSignals = map[syscall.Signal]bool{
	syscall.SIGKILL: true,
	syscall.SIGTERM: true,
	syscall.SIGINT:  true,
	syscall.SIGHUP:  true,
	syscall.SIGXCPU: true,
	syscall.SIGXFSZ: true,
}

// Only directly observed termination/resource signals are non-definitive.
// A wrapper that converts a child's death to a regular exit hides that signal.
func resourceSignal(state *os.ProcessState) bool {
	if state == nil {
		return false
	}
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	return status.Signaled() && terminationSignals[status.Signal()]
}
