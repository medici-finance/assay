//go:build unix

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// errPayloadIsLink marks an open refused because the entry is now a link.
var errPayloadIsLink = errors.New("payload path is a link")

// checkPayloadDir is the part of host contract H5 the signer can check on unix:
// the payload's immediate parent is a real directory (not a link), has no group
// or other write bit, and is owned by the signer's effective uid — so only the
// host's user can change the payload or its .out sibling. Directories above it
// are the host's to keep.
//
// The directory is OPENED (O_DIRECTORY|O_NOFOLLOW, so a link or any non-directory
// entry fails the open) and the checks run on that open handle, which is returned
// for the caller to close. openPayloadFile looks the payload up relative to this
// handle, so a directory swapped for a link after the check cannot redirect the
// read to a directory that was never checked.
func checkPayloadDir(dir string) (*os.File, error) {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
			return nil, fmt.Errorf("its directory %s is not a directory (a link or other entry)", dir)
		}
		return nil, fmt.Errorf("cannot inspect its directory %s: %v", dir, err)
	}
	d := os.NewFile(uintptr(fd), dir)
	if err := checkOpenedPayloadDir(d, dir); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

func checkOpenedPayloadDir(d *os.File, dir string) error {
	fi, err := d.Stat()
	if err != nil {
		return fmt.Errorf("cannot inspect its directory %s: %v", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("its directory %s is not a directory (a link or other entry)", dir)
	}
	if perm := fi.Mode().Perm(); perm&0o022 != 0 {
		return fmt.Errorf("its directory %s is writable by group or other (mode %#o)", dir, perm)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot read the owner of its directory %s", dir)
	}
	if int64(st.Uid) != int64(os.Geteuid()) {
		return fmt.Errorf("its directory %s is owned by uid %d, not the signer's uid %d", dir, st.Uid, os.Geteuid())
	}
	return nil
}

// openPayloadFile opens the payload relative to the checked directory handle
// (openat), without following a final-component link (O_NOFOLLOW: a link swapped
// in after the Lstat fails the open) and without blocking (O_NONBLOCK: a FIFO
// swapped in opens at once, and the caller's regular-file check refuses it).
func openPayloadFile(dir *os.File, path string) (*os.File, error) {
	fd, err := unix.Openat(int(dir.Fd()), filepath.Base(path), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		perr := &os.PathError{Op: "open", Path: path, Err: err}
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("%w: %v", errPayloadIsLink, perr)
		}
		return nil, perr
	}
	return os.NewFile(uintptr(fd), path), nil
}
