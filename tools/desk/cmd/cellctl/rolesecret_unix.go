//go:build darwin || linux

package main

import "golang.org/x/sys/unix"

// fileOwnerUID is the owner of path itself (a symlink is not followed).
func fileOwnerUID(path string) (int, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return 0, err
	}
	return int(st.Uid), nil
}
