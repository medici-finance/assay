package deskkit

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// errProcessGone means the pid names no running process. Only this error proves
// absence; any other process-inspection error is could-not-check.
var errProcessGone = errors.New("process is not running")

// errCouldNotCheck marks a process fact this platform cannot establish.
var errCouldNotCheck = errors.New("could not check on this platform")

// commandName normalises an executable path to the name the platform reports.
func commandName(exe string) string {
	base := filepath.Base(exe)
	if runtime.GOOS == "windows" {
		base = strings.TrimSuffix(strings.ToLower(base), ".exe")
	}
	return base
}
