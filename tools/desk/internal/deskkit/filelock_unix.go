//go:build unix

package deskkit

import (
	"errors"
	"os"
	"syscall"
)

// ErrLockBusy is returned by TryLockExclusive when the advisory lock is held by
// another process. A caller MUST treat it as "another op is live", never as
// "free" — a lock this process cannot hold is never permission to proceed.
//
// The windows variant (filelock_windows.go) declares the SAME sentinel under its
// own build tag; exactly one file is compiled per build, so callers compare
// against deskkit.ErrLockBusy regardless of GOOS.
var ErrLockBusy = errors.New("deskkit: advisory lock is held by another process")

// TryLockExclusive takes a NON-BLOCKING exclusive advisory lock on f. It returns
// nil on success, ErrLockBusy when another process holds the lock, and the raw
// error for any other failure. It FAILS CLOSED: it never reports success when the
// lock was not taken. Unix variant: flock(LOCK_EX|LOCK_NB), mapping EWOULDBLOCK to
// ErrLockBusy — the exact behaviour of the syscall.Flock sites it replaces.
func TryLockExclusive(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return ErrLockBusy
	}
	return err
}

// UnlockFile releases the advisory lock held on f, over the same range
// TryLockExclusive took.
func UnlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
