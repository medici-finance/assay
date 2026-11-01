//go:build !unix

package local

import (
	"os"
	"time"
)

// On platforms without flock the store lock is an exclusive-create sibling file. A
// crashed holder leaves it behind; a lock older than staleLock is taken over.
const staleLock = 30 * time.Second

func lockFile(f *os.File, wait time.Duration) error {
	path := f.Name() + ".excl"
	deadline := time.Now().Add(wait)
	for {
		g, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return g.Close()
		}
		if st, serr := os.Stat(path); serr == nil && time.Since(st.ModTime()) > staleLock {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return errLockTimeout
		}
		time.Sleep(time.Millisecond)
	}
}

func tryLockFile(f *os.File) bool { return false }

func unlockFile(f *os.File) { _ = os.Remove(f.Name() + ".excl") }

// Without a held lock an abandoned live marker cannot be told from a running writer, so
// unclean-termination detection is not available here; the reader reports live markers.
const liveMarkersSupported = false
