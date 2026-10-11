//go:build unix

package gitmaintenance

import (
	"os"

	"golang.org/x/sys/unix"
)

// Require a directory in the same kernel operation that opens it. A prior Stat
// followed by an ordinary open would admit replacement by a blocking special file.
func openMaintenanceDirectory(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open maintenance directory", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}
