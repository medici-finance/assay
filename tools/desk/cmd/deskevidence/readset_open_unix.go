//go:build unix

package main

import (
	"os"
	"syscall"
)

// openStateFile opens p read-only with O_NOFOLLOW, so a link at the name is
// refused by the open itself, and O_NONBLOCK, so a FIFO swapped in at the
// name cannot block the open. readStateFile then checks the descriptor.
// O_NOFOLLOW's errno for a link differs by kernel (ELOOP, EMLINK, EFTYPE), so
// a failed open is classified by an Lstat that only names the refusal: nothing
// is read through it.
func openStateFile(p string) (*os.File, error) {
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err == nil {
		return f, nil
	}
	if fi, lerr := os.Lstat(p); lerr == nil && !fi.Mode().IsRegular() {
		return nil, errNotRegularStateFile
	}
	return nil, err
}
