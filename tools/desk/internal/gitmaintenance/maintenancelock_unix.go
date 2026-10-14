//go:build unix

package gitmaintenance

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

// Refuse symlinks/FIFOs at open time, before reading or truncating any stamp.
func openMaintenanceLock(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, &os.PathError{Op: "open maintenance lock", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("maintenance lock must be a regular file")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
