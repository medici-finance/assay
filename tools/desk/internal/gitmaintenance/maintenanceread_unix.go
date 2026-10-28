//go:build unix

package gitmaintenance

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Open a receipt/stamp without admitting a blocking special file or a symbolic
// link between a preceding inventory check and the read. This is read-only:
// inventory cannot create a missing state file as a side effect of looking.
func openMaintenanceRegular(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open maintenance state", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !st.Mode().IsRegular() {
		_ = f.Close()
		return nil, fmt.Errorf("maintenance state is not a regular file")
	}
	return f, nil
}
