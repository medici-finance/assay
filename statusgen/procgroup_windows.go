//go:build windows

package main

import "os/exec"

// killWholeProcessGroup is the windows variant of the process-group kill helper.
//
// DEGRADATION — stated out loud at the site that loses it. Windows has no POSIX
// process groups and no negative-pid group kill, so this leaves cmd.Cancel at the
// exec default, which kills only the direct `git` child. A hung remote-helper
// GRANDCHILD (git-remote-<scheme> and the transport under it) can therefore ORPHAN
// itself and outlive the timeout — exactly the orphan pile-up the unix group kill
// exists to prevent.
//
// What still makes the deadline REAL for the caller is cmd.WaitDelay, which is set
// by the shared caller in gitinfo.go (unchanged, and not touched by either
// variant): after it expires the pipes are closed and Wait returns, so
// listRemoteBranches still fails fast even though a grandchild may linger. The
// follow-up that would CLOSE this gap on windows is a job object
// (CreateJobObject + AssignProcessToJobObject with
// JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE), which kills the whole tree on handle close;
// that is out of scope for this compile-target split.
func killWholeProcessGroup(cmd *exec.Cmd) {
	// Intentionally a no-op: leaves cmd.SysProcAttr and cmd.Cancel at the exec
	// defaults. See the caveat above — the group kill cannot be expressed here.
	_ = cmd
}
