//go:build !linux

package main

import (
	"fmt"
	"runtime"
)

// platformMountReadOnly off Linux: the container runner is Linux, and here the
// mount flags are not read, so the check refuses rather than passes.
func platformMountReadOnly(path string) (bool, error) {
	return false, fmt.Errorf("mount flags are read only on Linux, and this host is %s", runtime.GOOS)
}
