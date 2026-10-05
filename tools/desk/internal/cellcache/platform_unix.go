//go:build darwin || linux

package cellcache

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"syscall"
)

const noFollow = unix.O_NOFOLLOW

// Ancestors may be shared only when sticky (e.g. /tmp). No symlink component
// is accepted; Resolve canonicalizes only the trusted cell location beforehand.
func validatePath(path string) error {
	for cur := path; ; cur = filepath.Dir(cur) {
		st, err := os.Lstat(cur)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("cache path contains a non-directory or symlink: %s", cur)
			}
			sys, ok := st.Sys().(*syscall.Stat_t)
			if !ok || (sys.Uid != uint32(os.Getuid()) && sys.Uid != 0) {
				return fmt.Errorf("foreign cache ancestor: %s", cur)
			}
			if st.Mode().Perm()&0022 != 0 && st.Mode()&os.ModeSticky == 0 {
				return fmt.Errorf("writable cache ancestor: %s", cur)
			}
			if cur == path && (sys.Uid != uint32(os.Getuid()) || st.Mode().Perm()&0077 != 0) {
				return fmt.Errorf("cache root must be owned by this user and mode 0700")
			}
		}
		if filepath.Dir(cur) == cur {
			return nil
		}
	}
}

func identity(st os.FileInfo) (device uint64, inode uint64, err error) {
	s, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("filesystem identity unavailable")
	}
	return uint64(s.Dev), s.Ino, nil
}
func checkEntry(st os.FileInfo, dev uint64) error {
	d, _, err := identity(st)
	if err != nil {
		return err
	}
	s := st.Sys().(*syscall.Stat_t)
	if d != dev || s.Uid != uint32(os.Getuid()) || st.Mode()&os.ModeSymlink != 0 || (!st.IsDir() && !st.Mode().IsRegular()) || (!st.IsDir() && s.Nlink != 1) {
		return fmt.Errorf("foreign, linked or unsupported cache entry %s", st.Name())
	}
	if st.Mode().Perm()&0022 != 0 {
		return fmt.Errorf("cache entry writable by another principal")
	}
	return nil
}
func diskFree(path string) (uint64, error) {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return 0, err
	}
	if s.Bsize <= 0 {
		return 0, fmt.Errorf("filesystem block size unavailable")
	}
	return uint64(s.Bavail) * uint64(s.Bsize), nil
}
func filesystemName(dev uint64) string { return fmt.Sprintf("device:%d", dev) }
