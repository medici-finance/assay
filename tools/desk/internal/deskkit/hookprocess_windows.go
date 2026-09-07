//go:build windows

package deskkit

import "os/exec"

// isolateHookProcess — see hookprocess_unix.go for the contract. Windows has no process
// groups in the POSIX sense and no syscall.Kill, so there is nothing to set here: the
// default os/exec cancellation (terminate cmd.Process) stands, and the WaitDelay hooks.go
// sets is what keeps Wait from blocking on a pipe a surviving child still holds.
//
// This is a deliberate no-op rather than an unimplemented stub: the hook still times out and
// is still reported as timed out on Windows; only the reach of the kill is narrower. A
// job-object implementation would be the full fix and is not required by any hook call site
// today, none of which runs on Windows.
func isolateHookProcess(cmd *exec.Cmd) {}
