//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"
)

// TestApplyRowCmdLine_SetsRawCmdLine_Windows asserts, on a real Windows build,
// that applyRowCmdLine installs the raw command line for a `cmd` row and that it
// is NOT the string os/exec would otherwise launch (syscall.EscapeArg per
// element). This is the build-tagged half of the #1424 evidence; the portable
// half (rawcmdline_test.go) proves the same string shape without a Windows host.
func TestApplyRowCmdLine_SetsRawCmdLine_Windows(t *testing.T) {
	argv := []string{"cmd", "/d", "/s", "/c", cmdRowSample}
	cmd := exec.Command(argv[0], argv[1:]...)

	applyRowCmdLine(cmd, rowShellCmd, argv)

	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CmdLine == "" {
		t.Fatalf("applyRowCmdLine did not set SysProcAttr.CmdLine for a cmd row")
	}
	want := `cmd /d /s /c "` + cmdRowSample + `"`
	if cmd.SysProcAttr.CmdLine != want {
		t.Fatalf("CmdLine =\n  %q\nwant\n  %q", cmd.SysProcAttr.CmdLine, want)
	}

	// The default os/exec construction — what runs when CmdLine is unset — for
	// the SAME argv. It backslash-escapes the row's quotes; the raw line does not.
	defaultLine := "cmd /d /s /c " + syscall.EscapeArg(cmdRowSample)
	if cmd.SysProcAttr.CmdLine == defaultLine {
		t.Fatalf("raw CmdLine must differ from os/exec's default escaping (%q)", defaultLine)
	}
	if !strings.Contains(defaultLine, `\"`) {
		t.Fatalf("expected the default line to backslash-escape the row's quotes; got %q", defaultLine)
	}
	if strings.Contains(cmd.SysProcAttr.CmdLine, `\"`) {
		t.Fatalf("raw CmdLine still carries a backslash-escaped quote: %q", cmd.SysProcAttr.CmdLine)
	}
}

// TestApplyRowCmdLine_LeavesPwshAndShAlone_Windows guards the scoping: only a
// `cmd` row gets a raw CmdLine. `pwsh` and `sh` rows keep os/exec's default
// escaping, which is correct for a program parsed via CommandLineToArgvW.
func TestApplyRowCmdLine_LeavesPwshAndShAlone_Windows(t *testing.T) {
	for _, shell := range []string{rowShellPwsh, rowShellSh} {
		cmd := exec.Command("x")
		applyRowCmdLine(cmd, shell, []string{"x", "y"})
		if cmd.SysProcAttr != nil && cmd.SysProcAttr.CmdLine != "" {
			t.Errorf("shell %q: CmdLine was set (%q); only cmd rows are raw-built", shell, cmd.SysProcAttr.CmdLine)
		}
	}
}
