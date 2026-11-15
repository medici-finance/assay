//go:build linux

package main

import "golang.org/x/sys/unix"

// platformMountReadOnly reads the mount flags of the file system holding path.
func platformMountReadOnly(path string) (bool, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false, err
	}
	return st.Flags&unix.ST_RDONLY != 0, nil
}
