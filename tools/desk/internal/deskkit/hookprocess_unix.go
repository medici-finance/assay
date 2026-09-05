//go:build unix

package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// isolateHookProcess makes the hook's timeout kill the whole PROCESS TREE the hook started,
// not just the /bin/sh we spawned. See hooks.go's Run for why that distinction is the
// difference between a 200ms timeout and a 30s one.
//
// Two settings, both required:
//
//   - Setpgid puts the shell in a NEW process group of its own, with the shell as leader
//     (so pgid == the shell's pid). Every process the hook starts — foreground, background,
//     subshell — inherits that group unless it deliberately leaves it.
//   - Cancel replaces os/exec's default (which signals ONLY cmd.Process) with a kill aimed
//     at the negated pid, i.e. at the whole group.
//
// A hook is operator-authored shell from the state directory, and its whole purpose is to
// run other commands; a timeout that reaped only the shell would leave those commands
// running forever and, because they hold the inherited stdout/stderr pipe open, would leave
// the desk verb that started them blocked too.
func isolateHookProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		// Negative pid ⇒ "the process group whose id is |pid|". Setpgid above made that
		// group the shell's own, so this reaches the shell and everything under it, and
		// nothing else. SIGKILL, not SIGTERM: the budget has already been spent, and a
		// hook that ignores or traps a polite signal must not be able to extend it.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == nil {
			return nil
		}
		// The group is already gone (the hook exited in the race between the deadline
		// firing and the signal landing). os/exec treats ErrProcessDone as "nothing to
		// cancel" and does not surface it, which is exactly right here.
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
