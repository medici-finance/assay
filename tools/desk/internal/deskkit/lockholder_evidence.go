package deskkit

import (
	"errors"
	"os"
)

// ReadLockHolder returns the owner record written beside root/name without
// taking the lock. A record that outlived its process (a crash skips Release)
// is the start-time evidence a reconcile classifies a dead owner by. It is
// evidence only: every field must be re-checked against the live system.
func ReadLockHolder(root *os.Root, name string) (*LockHolder, error) {
	return readLockHolder(root, name)
}

// ProcessStart reports pid's start identity as the lock-holder record stores
// it. gone is true only when the pid names no running process; any other
// failure is could-not-check and is returned as err, never as gone.
func ProcessStart(pid int) (start string, gone bool, err error) {
	if pid <= 0 {
		return "", false, errors.New("invalid pid")
	}
	start, err = procStart(pid)
	if errors.Is(err, errProcessGone) {
		return "", true, nil
	}
	return start, false, err
}
