//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// sgExecFallback replaces this process with the pinned statusgen; it returns only when the
// exec itself failed, with the could-not-check exit code.
func sgExecFallback(path string, args, env []string) int {
	err := syscall.Exec(path, append([]string{path}, args...), env)
	fmt.Fprintf(os.Stderr, "statusgen: could not exec pinned %s: %v\n", path, err)
	return sgExitUnverifiable
}
