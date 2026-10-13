//go:build unix && !linux && !darwin

package deskkit

import (
	"fmt"
	"os"
	"syscall"
)

// Other platforms cannot verify a holder's identity, so every holder diagnoses
// as unverified and is never eligible for recovery.
func procStart(pid int) (string, error) {
	if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
		return "", errProcessGone
	}
	return "", fmt.Errorf("%w: process start time", errCouldNotCheck)
}

func procCommand(pid int) (string, error) {
	return "", fmt.Errorf("%w: process command", errCouldNotCheck)
}

func procHoldsFile(pid int, f *os.File) (bool, error) {
	return false, fmt.Errorf("%w: open-file listing", errCouldNotCheck)
}

func procTerminate(pid int) error {
	return fmt.Errorf("%w: ending a lock holder", errCouldNotCheck)
}
