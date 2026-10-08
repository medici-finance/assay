//go:build unix

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// errPayloadIsLink marks an open refused because the entry is now a link.
var errPayloadIsLink = errors.New("payload path is a link")

// checkPayloadDir is the part of host contract H5 the signer can check on unix:
// the payload's immediate parent is a real directory (not a link), has no group
// or other write bit, and is owned by the signer's effective uid — so only the
// host's user can change the payload or its .out sibling. Directories above it
// are the host's to keep.
func checkPayloadDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("cannot inspect its directory %s: %v", dir, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
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

// openPayloadFile opens the payload without following a final-component link
// (O_NOFOLLOW: a link swapped in after the Lstat fails the open) and without
// blocking (O_NONBLOCK: a FIFO swapped in opens at once, and the caller's
// regular-file check refuses it).
func openPayloadFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil && errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%w: %v", errPayloadIsLink, err)
	}
	return f, err
}
