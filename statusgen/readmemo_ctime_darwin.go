//go:build darwin

package main

import (
	"os"
	"syscall"
	"time"
)

// statCtime reads the change time, inode and device from a darwin stat.
func statCtime(fi os.FileInfo) (ctime time.Time, ino, dev uint64, ok bool) {
	st, isStat := fi.Sys().(*syscall.Stat_t)
	if !isStat {
		return time.Time{}, 0, 0, false
	}
	return time.Unix(st.Ctimespec.Sec, st.Ctimespec.Nsec), st.Ino, uint64(st.Dev), true
}
