//go:build !unix

package local

import (
	"os"
	"time"
)

// On platforms without flock the store lock is an exclusive-create file beside the lock
// file, made through the store's directory handle. A crashed holder leaves it behind; a
// lock older than staleLock is taken over.
const staleLock = 30 * time.Second

const exclName = "lock.excl"

func lockStore(r *os.Root, _ *os.File, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		g, err := r.OpenFile(exclName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return g.Close()
		}
		if st, serr := r.Lstat(exclName); serr == nil && time.Since(st.ModTime()) > staleLock {
			_ = r.Remove(exclName)
			continue
		}
		if time.Now().After(deadline) {
			return errLockTimeout
		}
		time.Sleep(time.Millisecond)
	}
}

func unlockStore(r *os.Root, _ *os.File) { _ = r.Remove(exclName) }

func tryLockFile(*os.File) bool { return false }

func unlockFile(*os.File) {}

// Without a held lock an abandoned live marker cannot be told from a running writer, so
// unclean-termination detection is not available here; the reader reports live markers.
const liveMarkersSupported = false
