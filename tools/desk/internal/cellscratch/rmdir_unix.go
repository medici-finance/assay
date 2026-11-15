//go:build unix

package cellscratch

import (
	"os"

	"golang.org/x/sys/unix"
)

// removeEmptyDir removes name from the open directory parent only while name is an
// empty directory: rmdir semantics, so a file or a non-empty directory renamed onto
// name meanwhile fails the call instead of being removed.
func removeEmptyDir(parent *os.Root, name string) error {
	f, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	conn, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var rmErr error
	if err = conn.Control(func(fd uintptr) {
		rmErr = unix.Unlinkat(int(fd), name, unix.AT_REMOVEDIR)
	}); err != nil {
		return err
	}
	if rmErr != nil {
		return &os.PathError{Op: "rmdir", Path: name, Err: rmErr}
	}
	return nil
}
