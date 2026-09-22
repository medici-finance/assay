//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// applyRowCmdLine is the WINDOWS variant of the per-row command-line fixup for
// issue #1424. For a `cmd` row it replaces os/exec's default per-argument
// escaping with a RAW command line built by winCmdLine, so `cmd /d /s /c`
// receives the row wrapped in exactly one outer quote pair and nothing else
// re-quoted.
//
// WHY ONLY `cmd`. os/exec's default construction (syscall.EscapeArg per element)
// is exactly the quoting a program that parses its command line with
// CommandLineToArgvW expects — that includes PowerShell, whose `-Command`
// argument is delivered and re-parsed by the standard argv rules. `cmd.exe` is
// the outlier: it does its OWN quote handling and `/s /c` strips only the outer
// quote pair, so EscapeArg's backslash-escaped inner quotes leak through. So the
// fix is scoped to `cmd`; `pwsh` and `sh` rows keep the default escaping, which
// is correct for them.
//
// SysProcAttr.CmdLine, when set, is used verbatim as the process command line
// (cmd.Path — resolved by exec.Command's LookPath — remains the executable). We
// keep cmd.Args aligned with the argv we built so the logical arg list matches
// the raw line.
func applyRowCmdLine(cmd *exec.Cmd, shell string, argv []string) {
	if shell != rowShellCmd {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CmdLine = winCmdLine(argv)
	cmd.Args = argv
}
