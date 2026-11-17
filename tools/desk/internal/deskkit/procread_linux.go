//go:build linux

package deskkit

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// readProcFile reads a /proc file only when the opened file is on procfs, so a file
// bind-mounted over /proc/self/uid_map in a caller's own mount namespace cannot pose as the
// kernel's answer.
func readProcFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var st unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &st); err != nil {
		return nil, fmt.Errorf("%s: filesystem type unreadable: %v", path, err)
	}
	if int64(st.Type) != unix.PROC_SUPER_MAGIC {
		return nil, fmt.Errorf("%s is not on procfs (filesystem type %#x)", path, st.Type)
	}
	return io.ReadAll(f)
}
