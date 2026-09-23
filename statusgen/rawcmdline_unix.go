//go:build !windows

package main

import "os/exec"

// applyRowCmdLine is the NON-WINDOWS variant of the per-row command-line fixup
// for issue #1424: a no-op. SysProcAttr.CmdLine is a Windows-only field, and the
// per-argument escaping the fix works around exists only in os/exec's Windows
// command-line construction, so off Windows every row — including a `cmd` row
// exercised through a fake interpreter on PATH — is dispatched with the argv
// unchanged, exactly as before.
func applyRowCmdLine(cmd *exec.Cmd, shell string, argv []string) {
	_, _, _ = cmd, shell, argv
}
