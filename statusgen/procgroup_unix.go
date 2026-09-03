//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// killWholeProcessGroup runs cmd in its OWN process group and, on context
// cancellation, kills the WHOLE GROUP rather than just the direct child.
//
// The default Cancel kills only `git ls-remote`; the remote-helper chain it
// spawned (git-remote-<scheme> and any transport under it) survives as an
// orphan holding the hung connection forever — hundreds of them piled up
// from repeated runs and drove the machine's load average into the hundreds.
//
// This is the unix behaviour, verbatim from the inline block it replaced. The
// windows variant (procgroup_windows.go) cannot do this and says so.
func killWholeProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
