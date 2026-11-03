//go:build darwin || linux

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// oNoFollow refuses to open a symlink as the final path element.
const oNoFollow = unix.O_NOFOLLOW

// fileIdentity is the (inode, change time) pair pause records for a flag it created, so resume
// lifts only that same file: a flag removed and re-created by anyone else since has another pair.
// The path itself is not followed if it is a symlink.
func fileIdentity(path string) (uint64, string, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, "", err // a syscall.Errno: errors.Is(err, os.ErrNotExist) holds for ENOENT
	}
	return uint64(st.Ino), fmt.Sprintf("%d.%09d", st.Ctim.Sec, st.Ctim.Nsec), nil
}

