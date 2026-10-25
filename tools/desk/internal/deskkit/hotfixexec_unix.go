//go:build unix

package deskkit

import (
	"fmt"
	"os"
	"syscall"
)

// hotfixExecFallback replaces this process with the pinned binary; it returns only when the
// exec itself failed, with the could-not-check exit code.
func hotfixExecFallback(path string, args, env []string) int {
	err := syscall.Exec(path, append([]string{path}, args...), env)
	fmt.Fprintf(os.Stderr, "desk-tools: could not exec pinned %s: %v\n", path, err)
	return ExitUnverifiable
}
