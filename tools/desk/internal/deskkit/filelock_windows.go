//go:build windows

package deskkit

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// ErrLockBusy — see filelock_unix.go for the contract. Declared here too under the
// windows build tag; exactly one filelock_*.go is compiled per build.
var ErrLockBusy = errors.New("deskkit: advisory lock is held by another process")

// TryLockExclusive takes a NON-BLOCKING exclusive lock on the first byte of f via
// LockFileEx with LOCKFILE_FAIL_IMMEDIATELY. It returns nil on success, ErrLockBusy
// when another process holds it (ERROR_LOCK_VIOLATION), and the raw error
// otherwise.
//
// It FAILS CLOSED — it NEVER returns nil when the lock was not taken. A silently
// granted lock on a contended claim path is a double-dispatch, the exact fault the
// deskkit lock exists to close; so a windows helper that returned nil on failure
// would defeat the whole primitive.
func TryLockExclusive(f *os.File) error {
	err := windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, new(windows.Overlapped),
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrLockBusy
	}
	return err
}

// UnlockFile releases the lock over the SAME one-byte range TryLockExclusive took —
// a lock and an unlock over different ranges is a leak.
func UnlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}
