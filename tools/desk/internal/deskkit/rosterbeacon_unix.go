//go:build unix

package deskkit

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

// No-follow is enforced by the open, not a path precheck. NONBLOCK prevents a
// substituted FIFO from hanging before the handle's regular-file check.
func openRosterBeaconFile(path string, lock bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if lock {
		flags = unix.O_RDWR | unix.O_CREAT | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	}
	fd, err := unix.Open(path, flags, 0600)
	if err != nil {
		return nil, &os.PathError{Op: "open roster file", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = fmt.Errorf("roster file must be regular")
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func replaceRosterBeacon(from, to string) error {
	return os.Rename(from, to)
}
