//go:build windows

package deskkit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// hotfixExecFallback runs the pinned binary as a child and returns its exit code (Windows has
// no exec). The overlay is unsupported on Windows; this keeps the package building there.
func hotfixExecFallback(path string, args, env []string) int {
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = env, os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "desk-tools: could not run pinned %s: %v\n", path, err)
		return ExitUnverifiable
	}
	return 0
}
