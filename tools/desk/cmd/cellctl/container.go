package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// containerRun delegates to the operator-owned container launcher.
//
// The contract is EXECUTABLE ARGV, never eval. A clean environment carries registry metadata,
// not the operator's forge/model credentials or SSH agent. The launcher is trusted host code; it
// owns container mounts, credentials, state validation and the runtime. This boundary is not a
// sandbox around the launcher, and the port does not make it one.
func (c *Cell) containerRun(args ...string) {
	if c.Env.Get("DRY_RUN") == "1" {
		var b strings.Builder
		for _, a := range args {
			b.WriteString(bashQuote(a))
			b.WriteString(" ")
		}
		fmt.Printf("[dry-run] container cell=%s launcher=%s argv=%s\n",
			c.Name, bashQuote(c.Env.Get("CELL_CONTAINER_LAUNCHER")), b.String())
		return
	}
	cmd := exec.Command(c.Env.Get("CELL_CONTAINER_LAUNCHER"), args...)
	cmd.Env = []string{
		"HOME=" + c.Env.Get("HOME"),
		"PATH=" + c.Env.Get("PATH"),
		"TERM=" + c.Env.GetOr("TERM", "xterm-256color"),
		"CELL=" + c.Name,
		"CELL_KIND=container",
		"CELL_DIR=" + c.Dir,
		"CELL_REPO=" + c.Repo,
		"CELL_ROOTS=" + c.Env.Get("CELL_ROOTS"),
		"CELL_HARNESS=" + c.Harness,
		"ROLES=" + c.Env.Get("ROLES"),
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		exitWith(exitStatus(err))
	}
}

func exitStatus(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}
