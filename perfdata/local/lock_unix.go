//go:build unix

package local

import (
	"errors"
	"os"
	"syscall"
	"time"
)

// lockFile takes an exclusive advisory lock on f, waiting at most wait. The lock is per
// open file description, so two Store values in one process also exclude each other.
func lockFile(f *os.File, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			return err
		}
		if time.Now().After(deadline) {
			return errLockTimeout
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// tryLockFile takes the lock without waiting; ok=false if another holder has it.
func tryLockFile(f *os.File) bool {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

func unlockFile(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

// liveMarkersSupported reports whether an abandoned live marker can be told apart from a
// running writer (it can when the marker carries a held lock).
const liveMarkersSupported = true
